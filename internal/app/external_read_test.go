package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/externalcmd"
	"atlasnote/internal/localipc"
	"atlasnote/internal/note"
	"atlasnote/internal/notespace"
	"atlasnote/internal/readapi"
)

func TestExternalReadCLIAndMCPUseAuthenticatedAppBoundary(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	application := newApp("test")
	application.startup(t.Context())
	t.Cleanup(func() { application.shutdown(context.Background()) })
	if status := application.GetStartupStatus(); !status.Ready {
		t.Fatalf("app is not ready: %+v", status)
	}

	notebook, err := application.CreateNotebook(note.NotebookCreateInput{Name: "External reads"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := application.CreateNote(note.CreateInput{NotebookID: &notebook.ID, Title: "Target", Content: "searchable atlas body"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := application.CreateNote(note.CreateInput{
		NotebookID: &notebook.ID,
		Title:      "Source",
		Content:    "[target](atlasnote://note/" + target.ID + ") searchable atlas relation",
	})
	if err != nil {
		t.Fatal(err)
	}
	tagResult, err := application.CreateTag(note.TagCreateInput{Name: "stage-a"})
	if err != nil || tagResult.Error != nil || tagResult.Tag == nil {
		t.Fatalf("create tag: result=%+v err=%v", tagResult, err)
	}
	if result, err := application.SetNoteTags(target.ID, note.SetNoteTagsInput{TagIDs: []string{tagResult.Tag.ID}}); err != nil || result.Error != nil {
		t.Fatalf("set note tags: result=%+v err=%v", result, err)
	}

	for _, command := range [][]string{
		{"notes", "list", "--notebook", notebook.ID, "--json"},
		{"notes", "get", target.ID, "--expected-revision", "1", "--json"},
		{"notes", "search", "searchable", "--json"},
		{"notes", "backlinks", target.ID, "--json"},
		{"notes", "related", target.ID, "--json"},
		{"notebooks", "list", "--json"},
		{"tags", "list", "--json"},
	} {
		response, code := runReadCommand(t, command...)
		if code != 0 || response.Status != readapi.StatusOK {
			t.Fatalf("command %v failed: code=%d response=%+v", command, code, response)
		}
	}

	var mcpOutput bytes.Buffer
	mcpInput := strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"notes.get","arguments":{"noteId":"` + target.ID + `"}}}`,
	}, "\n") + "\n")
	if code := externalcmd.RunMCP([]string{"--note", target.ID}, mcpInput, &mcpOutput, &bytes.Buffer{}); code != 0 {
		t.Fatalf("MCP server exited with %d: %s", code, mcpOutput.String())
	}
	lines := strings.Split(strings.TrimSpace(mcpOutput.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], `"structuredContent"`) || !strings.Contains(lines[1], target.ID) {
		t.Fatalf("unexpected MCP output: %s", mcpOutput.String())
	}
	if _, err := application.db.ExecContext(t.Context(), "UPDATE note_search_state SET indexed_revision = 999 WHERE note_id = ?", target.ID); err != nil {
		t.Fatal(err)
	}
	indexResponse, code := runReadCommand(t, "notes", "search", "atlas", "--json")
	if code != 4 || indexResponse.Error == nil || indexResponse.Error.Code != "INDEX_INCONSISTENT" {
		t.Fatalf("inconsistent search index was not rejected: code=%d response=%+v", code, indexResponse)
	}
	if _, err := application.db.ExecContext(t.Context(), "UPDATE note_link_state SET indexed_revision = 999 WHERE note_id = ?", source.ID); err != nil {
		t.Fatal(err)
	}
	linkResponse, code := runReadCommand(t, "notes", "backlinks", target.ID, "--json")
	if code != 4 || linkResponse.Error == nil || linkResponse.Error.Code != "INDEX_INCONSISTENT" {
		t.Fatalf("inconsistent backlink index was not rejected: code=%d response=%+v", code, linkResponse)
	}

	conflict, code := runReadCommand(t, "notes", "get", target.ID, "--expected-revision", "2", "--json")
	if code != 3 || conflict.Status != readapi.StatusConflict || conflict.Error == nil || conflict.Error.Code != "REVISION_MISMATCH" {
		t.Fatalf("revision mismatch was not preserved: code=%d response=%+v", code, conflict)
	}

	descriptor, err := localipc.LoadDescriptor(dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	badToken := descriptor
	badToken.Token = strings.Repeat("x", len(descriptor.Token))
	badClient, err := localipc.NewClient(badToken)
	if err != nil {
		t.Fatal(err)
	}
	requestID, _ := localipc.NewRequestID()
	if _, err := badClient.Call(context.Background(), readapi.OperationTagsList, readapi.TagListInput{}, requestID); !errors.Is(err, localipc.ErrAuthentication) {
		t.Fatalf("invalid IPC token result = %v", err)
	}

	badScope := descriptor
	badScope.StorageSpaceID = "different-space"
	scopeClient, err := localipc.NewClient(badScope)
	if err != nil {
		t.Fatal(err)
	}
	requestID, _ = localipc.NewRequestID()
	scopeResponse, err := scopeClient.Call(context.Background(), readapi.OperationTagsList, readapi.TagListInput{}, requestID)
	if err != nil || scopeResponse.Error == nil || scopeResponse.Error.Code != "SCOPE_MISMATCH" {
		t.Fatalf("scope mismatch was not rejected: response=%+v err=%v", scopeResponse, err)
	}

	protected := application.EnableContentLock(contentlock.EnableInput{
		TargetType: contentlock.TargetNote, TargetID: source.ID, Passphrase: "correct horse battery staple",
	})
	if protected.Error != nil {
		t.Fatalf("enable content lock: %+v", protected.Error)
	}
	protectedResponse, code := runReadCommand(t, "notes", "get", source.ID, "--json")
	if code != 2 || protectedResponse.Error == nil || protectedResponse.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("protected note was exposed: code=%d response=%+v", code, protectedResponse)
	}
	listedAfterProtection, code := runReadCommand(t, "notes", "list", "--json")
	listedJSON, _ := json.Marshal(listedAfterProtection)
	if code != 0 || strings.Contains(string(listedJSON), source.ID) {
		t.Fatalf("protected note remained in list: code=%d response=%s", code, listedJSON)
	}
	locked := application.LockContentNow(contentlock.Target{Type: contentlock.TargetNote, ID: source.ID})
	if locked.Error != nil {
		t.Fatalf("lock content now: %+v", locked.Error)
	}
	lockedResponse, code := runReadCommand(t, "notes", "get", source.ID, "--json")
	if code != 2 || lockedResponse.Error == nil || lockedResponse.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("locked note was exposed: code=%d response=%+v", code, lockedResponse)
	}

	client, err := localipc.NewClient(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	application.shutdown(t.Context())
	requestID, _ = localipc.NewRequestID()
	if _, err := client.Call(context.Background(), readapi.OperationTagsList, readapi.TagListInput{}, requestID); err == nil {
		t.Fatal("IPC call succeeded after app shutdown")
	}
}

func TestMCPRequiresExplicitPublicationScopeAndNegotiatesImplementedVersion(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	application := newApp("test")
	application.startup(t.Context())
	t.Cleanup(func() { application.shutdown(context.Background()) })
	if status := application.GetStartupStatus(); !status.Ready {
		t.Fatalf("app is not ready: %+v", status)
	}

	publicNotebook, err := application.CreateNotebook(note.NotebookCreateInput{Name: "Published notebook"})
	if err != nil {
		t.Fatal(err)
	}
	privateNotebook, err := application.CreateNotebook(note.NotebookCreateInput{Name: "Private notebook"})
	if err != nil {
		t.Fatal(err)
	}
	publicNote, err := application.CreateNote(note.CreateInput{NotebookID: &publicNotebook.ID, Title: "Published title", Content: "published-body-marker"})
	if err != nil {
		t.Fatal(err)
	}
	privateNote, err := application.CreateNote(note.CreateInput{NotebookID: &privateNotebook.ID, Title: "Private title marker", Content: "private-body-marker"})
	if err != nil {
		t.Fatal(err)
	}
	publicTag, err := application.CreateTag(note.TagCreateInput{Name: "published-tag"})
	if err != nil || publicTag.Tag == nil {
		t.Fatalf("create public tag: result=%+v err=%v", publicTag, err)
	}
	privateTag, err := application.CreateTag(note.TagCreateInput{Name: "private-tag-marker"})
	if err != nil || privateTag.Tag == nil {
		t.Fatalf("create private tag: result=%+v err=%v", privateTag, err)
	}
	if result, err := application.SetNoteTags(publicNote.ID, note.SetNoteTagsInput{TagIDs: []string{publicTag.Tag.ID}}); err != nil || result.Error != nil {
		t.Fatalf("set public note tags: result=%+v err=%v", result, err)
	}
	if result, err := application.SetNoteTags(privateNote.ID, note.SetNoteTagsInput{TagIDs: []string{privateTag.Tag.ID}}); err != nil || result.Error != nil {
		t.Fatalf("set private note tags: result=%+v err=%v", result, err)
	}

	cliResponse, code := runReadCommand(t, "notes", "get", privateNote.ID, "--json")
	if code != 0 || cliResponse.Status != readapi.StatusOK {
		t.Fatalf("CLI lost its explicit local read access: code=%d response=%+v", code, cliResponse)
	}

	defaultOutput := runMCPTranscript(t, nil,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"notes.list","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"notes.get","arguments":{"noteId":"`+publicNote.ID+`"}}}`,
	)
	if !strings.Contains(defaultOutput, `"protocolVersion":"2025-06-18"`) || strings.Contains(defaultOutput, "2026-07-28") {
		t.Fatalf("MCP advertised an unsupported protocol version: %s", defaultOutput)
	}
	if strings.Contains(defaultOutput, publicNote.ID) || !strings.Contains(defaultOutput, `"code":"PERMISSION_DENIED"`) {
		t.Fatalf("default MCP scope exposed R1 content or metadata: %s", defaultOutput)
	}

	noteOutput := runMCPTranscript(t, []string{"--note", publicNote.ID},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"notes.list","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"notes.get","arguments":{"noteId":"`+publicNote.ID+`"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"notes.get","arguments":{"noteId":"`+privateNote.ID+`"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"notes.search","arguments":{"query":"marker"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"tags.list","arguments":{}}}`,
	)
	if !strings.Contains(noteOutput, publicNote.ID) || !strings.Contains(noteOutput, "published-body-marker") || !strings.Contains(noteOutput, "published-tag") {
		t.Fatalf("explicitly published note was not available: %s", noteOutput)
	}
	for _, secret := range []string{privateNote.ID, "Private title marker", "private-body-marker", "private-tag-marker"} {
		if strings.Contains(noteOutput, secret) {
			t.Fatalf("note-scoped MCP exposed unpublished value %q: %s", secret, noteOutput)
		}
	}

	notebookOutput := runMCPTranscript(t, []string{"--notebook", publicNotebook.ID},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"notebooks.list","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"notes.list","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"notes.get","arguments":{"noteId":"`+publicNote.ID+`"}}}`,
	)
	if !strings.Contains(notebookOutput, `"protocolVersion":"2025-06-18"`) || !strings.Contains(notebookOutput, publicNotebook.ID) || !strings.Contains(notebookOutput, publicNote.ID) {
		t.Fatalf("notebook publication scope or version negotiation failed: %s", notebookOutput)
	}
	if strings.Contains(notebookOutput, privateNotebook.ID) || strings.Contains(notebookOutput, privateNote.ID) {
		t.Fatalf("notebook-scoped MCP exposed another notebook: %s", notebookOutput)
	}
}

func TestMCPDoesNotReconnectAfterAppRestart(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	first := newApp("test")
	first.startup(t.Context())
	firstStopped := false
	t.Cleanup(func() {
		if !firstStopped {
			first.shutdown(context.Background())
		}
	})
	noteItem, err := first.CreateNote(note.CreateInput{Title: "Bound session", Content: "bound-session-body"})
	if err != nil {
		t.Fatal(err)
	}

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	done := make(chan int, 1)
	go func() {
		code := externalcmd.RunMCP([]string{"--note", noteItem.ID}, inputReader, outputWriter, &bytes.Buffer{})
		_ = outputWriter.Close()
		done <- code
	}()
	scanner := bufio.NewScanner(outputReader)
	writeMCPLine(t, inputWriter, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("initialize response = %q", scanner.Text())
	}
	writeMCPLine(t, inputWriter, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	writeMCPLine(t, inputWriter, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"notes.get","arguments":{"noteId":"`+noteItem.ID+`"}}}`)
	if !scanner.Scan() || !strings.Contains(scanner.Text(), "bound-session-body") {
		t.Fatalf("initial bound read response = %q", scanner.Text())
	}

	createdSpace := first.CreateStorageSpace(notespace.CreateInput{Name: "Switched space"})
	if createdSpace.Error != nil || createdSpace.Space == nil {
		t.Fatalf("create switched space: %+v", createdSpace)
	}
	selection := first.SelectStorageSpace(notespace.SelectInput{ID: createdSpace.Space.ID})
	if selection.Error != nil || !selection.RestartRequired {
		t.Fatalf("select switched space: %+v", selection)
	}
	first.shutdown(t.Context())
	firstStopped = true
	second := newApp("test")
	second.startup(t.Context())
	t.Cleanup(func() { second.shutdown(context.Background()) })
	if status := second.GetStartupStatus(); !status.Ready || status.ActiveStorageSpace == nil || status.ActiveStorageSpace.ID != createdSpace.Space.ID {
		t.Fatalf("restarted app is not ready: %+v", status)
	}
	writeMCPLine(t, inputWriter, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"notes.get","arguments":{"noteId":"`+noteItem.ID+`"}}}`)
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"code":"MCP_RESTART_REQUIRED"`) {
		t.Fatalf("existing MCP process reconnected after app restart: %q", scanner.Text())
	}
	_ = inputWriter.Close()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("MCP process exit code = %d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("MCP process did not stop after stdin closed")
	}
}

func TestMCPNormalExitRevokesSessionsBeyondServerLimit(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	application := newApp("test")
	application.startup(t.Context())
	t.Cleanup(func() { application.shutdown(context.Background()) })
	if status := application.GetStartupStatus(); !status.Ready {
		t.Fatalf("app is not ready: %+v", status)
	}
	for attempt := 1; attempt <= 65; attempt++ {
		output := runMCPTranscript(t, nil,
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		)
		if !strings.Contains(output, `"protocolVersion":"2025-06-18"`) {
			t.Fatalf("MCP initialize %d failed: %s", attempt, output)
		}
	}
}

func TestIPCStartupFailureDoesNotDisableGUI(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	if err := os.Mkdir(filepath.Join(dataRoot, ".atlasnote-ipc.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	application := newApp("test")
	application.startup(t.Context())
	t.Cleanup(func() { application.shutdown(context.Background()) })
	if status := application.GetStartupStatus(); !status.Ready {
		t.Fatalf("optional IPC failure disabled the GUI: %+v", status)
	}
	if application.readIPC != nil {
		t.Fatal("IPC remained enabled after descriptor publication failed")
	}
	created, err := application.CreateNote(note.CreateInput{Title: "GUI remains available", Content: "body"})
	if err != nil || created.ID == "" {
		t.Fatalf("GUI note service is unavailable: note=%+v err=%v", created, err)
	}
}

func TestExternalReadGateKeepsTagsRevisionAndTrashStateConsistent(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	application := newApp("test")
	application.startup(t.Context())
	t.Cleanup(func() { application.shutdown(context.Background()) })
	created, err := application.CreateNote(note.CreateInput{Title: "Concurrent read", Content: "concurrent-search-marker"})
	if err != nil {
		t.Fatal(err)
	}
	firstTag, _ := application.CreateTag(note.TagCreateInput{Name: "before"})
	secondTag, _ := application.CreateTag(note.TagCreateInput{Name: "after"})
	if firstTag.Tag == nil || secondTag.Tag == nil {
		t.Fatal("create concurrency test tags")
	}
	if result, err := application.SetNoteTags(created.ID, note.SetNoteTagsInput{TagIDs: []string{firstTag.Tag.ID}}); err != nil || result.Error != nil {
		t.Fatalf("set initial tags: result=%+v err=%v", result, err)
	}

	reader := &pausingNoteReader{
		Service: application.notes, pauseGetID: created.ID,
		getReached: make(chan struct{}), continueGet: make(chan struct{}),
	}
	service := readapi.New(reader, application.contentLocks, application.activeSpace.ID)
	principal := readapi.Principal{
		ClientID: "concurrency-test", Kind: "cli", StorageSpaceID: application.activeSpace.ID,
		Permissions: map[string]bool{readapi.PermissionMetadata: true, readapi.PermissionContent: true},
	}
	getParams, _ := json.Marshal(readapi.NoteGetInput{NoteID: created.ID})
	getResponse := make(chan readapi.Response, 1)
	go func() {
		getResponse <- service.Execute(context.Background(), principal, readapi.Request{
			APIVersion: readapi.APIVersion, RequestID: "concurrent-get", ClientID: principal.ClientID,
			Scope: readapi.Scope{StorageSpaceID: application.activeSpace.ID}, Operation: readapi.OperationNotesGet, Params: getParams,
		})
	}()
	<-reader.getReached
	tagMutationDone := make(chan error, 1)
	go func() {
		result, mutationErr := application.notes.SetNoteTags(context.Background(), created.ID, note.SetNoteTagsInput{TagIDs: []string{secondTag.Tag.ID}})
		if mutationErr == nil && result.Error != nil {
			mutationErr = errors.New(result.Error.Message)
		}
		tagMutationDone <- mutationErr
	}()
	assertOperationBlocked(t, tagMutationDone, "tag mutation")
	close(reader.continueGet)
	response := <-getResponse
	data, ok := response.Data.(readapi.NoteGetData)
	if !ok || response.Status != readapi.StatusOK || data.Note.Revision != created.Revision || len(data.Note.Tags) != 1 || data.Note.Tags[0].ID != firstTag.Tag.ID {
		t.Fatalf("external read mixed note revision and tags: response=%+v", response)
	}
	if err := <-tagMutationDone; err != nil {
		t.Fatal(err)
	}

	reader.searchReached = make(chan struct{})
	reader.continueSearch = make(chan struct{})
	searchParams, _ := json.Marshal(readapi.SearchInput{Query: "concurrent-search-marker"})
	searchResponse := make(chan readapi.Response, 1)
	go func() {
		searchResponse <- service.Execute(context.Background(), principal, readapi.Request{
			APIVersion: readapi.APIVersion, RequestID: "concurrent-search", ClientID: principal.ClientID,
			Scope: readapi.Scope{StorageSpaceID: application.activeSpace.ID}, Operation: readapi.OperationNotesSearch, Params: searchParams,
		})
	}()
	<-reader.searchReached
	trash := true
	trashDone := make(chan error, 1)
	go func() {
		_, updateErr := application.notes.Update(context.Background(), created.ID, note.UpdateInput{IsTrashed: &trash, ExpectedRevision: &created.Revision})
		trashDone <- updateErr
	}()
	assertOperationBlocked(t, trashDone, "trash mutation")
	close(reader.continueSearch)
	response = <-searchResponse
	searchData, ok := response.Data.(readapi.SearchData)
	if !ok || response.Status != readapi.StatusOK || len(searchData.Matches) != 1 || searchData.Matches[0].NoteID != created.ID {
		t.Fatalf("external search leaked a mixed trash state: response=%+v", response)
	}
	if err := <-trashDone; err != nil {
		t.Fatal(err)
	}
}

func TestExternalNoteListStopsAfterEnoughVisibleItems(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	application := newApp("test")
	application.startup(t.Context())
	t.Cleanup(func() { application.shutdown(context.Background()) })
	for index := 0; index < note.MaxNoteListPageSize+1; index++ {
		if _, err := application.CreateNote(note.CreateInput{Title: fmt.Sprintf("note-%03d", index), Content: "body"}); err != nil {
			t.Fatal(err)
		}
	}
	reader := &countingNoteReader{Service: application.notes}
	service := readapi.New(reader, application.contentLocks, application.activeSpace.ID)
	principal := readapi.Principal{
		ClientID: "page-test", Kind: "cli", StorageSpaceID: application.activeSpace.ID,
		Permissions: map[string]bool{readapi.PermissionMetadata: true, readapi.PermissionContent: true},
	}
	params, _ := json.Marshal(readapi.NoteListInput{Limit: 1})
	response := service.Execute(context.Background(), principal, readapi.Request{
		APIVersion: readapi.APIVersion, RequestID: "page-test", ClientID: principal.ClientID,
		Scope: readapi.Scope{StorageSpaceID: application.activeSpace.ID}, Operation: readapi.OperationNotesList, Params: params,
	})
	data, ok := response.Data.(readapi.NoteListData)
	if !ok || response.Status != readapi.StatusOK || len(data.Notes) != 1 || data.NextCursor == "" {
		t.Fatalf("unexpected limited list response: %+v", response)
	}
	if reader.listCalls != 1 {
		t.Fatalf("limit=1 scanned %d repository pages", reader.listCalls)
	}
}

type pausingNoteReader struct {
	*note.Service
	pauseGetID     string
	getReached     chan struct{}
	continueGet    chan struct{}
	searchReached  chan struct{}
	continueSearch chan struct{}
}

type countingNoteReader struct {
	*note.Service
	listCalls int
}

func (reader *countingNoteReader) ListPage(ctx context.Context, input note.NoteListInput) (note.NoteListResult, error) {
	reader.listCalls++
	return reader.Service.ListPage(ctx, input)
}

func (reader *pausingNoteReader) Get(ctx context.Context, id string) (note.Note, error) {
	item, err := reader.Service.Get(ctx, id)
	if err == nil && id == reader.pauseGetID && reader.getReached != nil {
		close(reader.getReached)
		<-reader.continueGet
		reader.getReached = nil
	}
	return item, err
}

func (reader *pausingNoteReader) Search(ctx context.Context, input note.SearchInput) (note.SearchResult, error) {
	result, err := reader.Service.Search(ctx, input)
	if err == nil && reader.searchReached != nil {
		close(reader.searchReached)
		<-reader.continueSearch
		reader.searchReached = nil
	}
	return result, err
}

func runMCPTranscript(t *testing.T, args []string, lines ...string) string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	input := strings.NewReader(strings.Join(lines, "\n") + "\n")
	if code := externalcmd.RunMCP(args, input, &stdout, &stderr); code != 0 {
		t.Fatalf("MCP exited with %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func writeMCPLine(t *testing.T, writer io.Writer, line string) {
	t.Helper()
	if _, err := fmt.Fprintln(writer, line); err != nil {
		t.Fatal(err)
	}
}

func assertOperationBlocked(t *testing.T, done <-chan error, name string) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("%s completed during an external read: %v", name, err)
	case <-time.After(100 * time.Millisecond):
	}
}

func runReadCommand(t *testing.T, args ...string) (readapi.Response, int) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	handled, code := externalcmd.Run(args, &stdout, &stderr)
	if !handled {
		t.Fatalf("command was not handled: %v", args)
	}
	var response readapi.Response
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode command output %q: %v (stderr=%s)", stdout.String(), err, stderr.String())
	}
	return response, code
}
