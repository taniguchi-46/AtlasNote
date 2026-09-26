package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/localipc"
	"atlasnote/internal/note"
	"atlasnote/internal/readapi"
)

func TestExternalOrganizationCLIAndMCPAreScopedAndNonDestructive(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	application := newApp("test")
	application.startup(t.Context())
	t.Cleanup(func() { application.shutdown(context.Background()) })
	if status := application.GetStartupStatus(); !status.Ready {
		t.Fatalf("app is not ready: %+v", status)
	}

	publicNotebook, err := application.CreateNotebook(note.NotebookCreateInput{Name: "Published organization"})
	if err != nil {
		t.Fatal(err)
	}
	privateNotebook, err := application.CreateNotebook(note.NotebookCreateInput{Name: "Private notebook marker"})
	if err != nil {
		t.Fatal(err)
	}
	publicNote, err := application.CreateNote(note.CreateInput{
		NotebookID: &publicNotebook.ID, Title: "Public old title", Content: "# Public proposed title\npublic body",
	})
	if err != nil {
		t.Fatal(err)
	}
	privateNote, err := application.CreateNote(note.CreateInput{
		NotebookID: &privateNotebook.ID, Title: "Private title marker", Content: "# Private proposal marker\nprivate body marker",
	})
	if err != nil {
		t.Fatal(err)
	}
	scopeNote, err := application.CreateNote(note.CreateInput{
		NotebookID: &publicNotebook.ID, Title: "Scoped old title", Content: "# Scoped proposed title",
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := application.notes.Get(t.Context(), publicNote.ID)
	if err != nil {
		t.Fatal(err)
	}

	cliAnalysis, code := runReadCommand(t, "organize", "analyze", "--scope", "note", "--note", publicNote.ID, "--limit", "1", "--json")
	if code != 0 || cliAnalysis.Status != readapi.StatusOK {
		t.Fatalf("CLI analysis failed: code=%d response=%+v", code, cliAnalysis)
	}
	cliData := responseDataMap(t, cliAnalysis)
	analysisID, _ := cliData["analysisId"].(string)
	if analysisID == "" {
		t.Fatalf("CLI analysis ID = %#v", cliData)
	}
	cliCandidates, code := runReadCommand(t, "organize", "candidates", analysisID, "--kind", "title", "--limit", "1", "--json")
	if code != 0 || cliCandidates.Status != readapi.StatusOK {
		t.Fatalf("CLI candidates failed: code=%d response=%+v", code, cliCandidates)
	}
	cliJSON, _ := json.Marshal(cliCandidates)
	if !strings.Contains(string(cliJSON), "Public proposed title") {
		t.Fatalf("CLI did not use organization engine: %s", cliJSON)
	}
	afterCLI, err := application.notes.Get(t.Context(), publicNote.ID)
	if err != nil || afterCLI.Revision != before.Revision || afterCLI.Content != before.Content || afterCLI.Title != before.Title {
		t.Fatalf("analysis changed note: before=%+v after=%+v err=%v", before, afterCLI, err)
	}

	rootClient, err := localipc.Connect(dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	mcpClient, err := rootClient.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NoteIDs: []string{publicNote.ID}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mcpClient.RevokeMCPSession(context.Background()) })
	mcpAnalysis := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeAnalyze, readapi.OrganizeAnalyzeInput{Scope: "space", Limit: 1})
	mcpJSON, _ := json.Marshal(mcpAnalysis)
	for _, secret := range []string{privateNote.ID, privateNotebook.ID, "Private title marker", "Private proposal marker", "private body marker"} {
		if strings.Contains(string(mcpJSON), secret) {
			t.Fatalf("MCP analysis exposed unpublished value %q: %s", secret, mcpJSON)
		}
	}
	if !strings.Contains(string(mcpJSON), publicNote.ID) {
		t.Fatalf("MCP analysis omitted published candidate: %s", mcpJSON)
	}
	mcpAnalysisID, _ := responseDataMap(t, mcpAnalysis)["analysisId"].(string)
	mcpCandidates := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeGetCandidates, readapi.OrganizeCandidatesInput{
		AnalysisID: mcpAnalysisID, Kind: "title", Limit: 1,
	})
	if mcpCandidates.Status != readapi.StatusOK {
		t.Fatalf("MCP candidates = %+v", mcpCandidates)
	}
	mcpCandidatesJSON, _ := json.Marshal(mcpCandidates)
	if !strings.Contains(string(mcpCandidatesJSON), "Public proposed title") {
		t.Fatalf("MCP candidates did not use organization engine: %s", mcpCandidatesJSON)
	}

	otherClient, err := rootClient.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NoteIDs: []string{publicNote.ID}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = otherClient.RevokeMCPSession(context.Background()) })
	otherResponse := callExternalOrganization(t, otherClient, readapi.OperationOrganizeGetCandidates, readapi.OrganizeCandidatesInput{AnalysisID: mcpAnalysisID})
	if otherResponse.Error == nil || otherResponse.Error.Code != "ANALYSIS_UNAVAILABLE" {
		t.Fatalf("other MCP session reused analysis: %+v", otherResponse)
	}
	forged := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeGetCandidates, readapi.OrganizeCandidatesInput{AnalysisID: strings.Repeat("f", 32)})
	if forged.Error == nil || forged.Error.Code != "ANALYSIS_UNAVAILABLE" {
		t.Fatalf("forged analysis ID response = %+v", forged)
	}
	badCursor := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeGetCandidates, readapi.OrganizeCandidatesInput{AnalysisID: mcpAnalysisID, Cursor: "not-a-cursor"})
	if badCursor.Error == nil || badCursor.Error.Code != "CURSOR_INVALID" {
		t.Fatalf("invalid cursor response = %+v", badCursor)
	}
	outOfScope := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeAnalyze, readapi.OrganizeAnalyzeInput{Scope: "note", NoteID: privateNote.ID})
	outOfScopeJSON, _ := json.Marshal(outOfScope)
	if outOfScope.Error == nil || outOfScope.Error.Code != "RESOURCE_UNAVAILABLE" || strings.Contains(string(outOfScopeJSON), "Private") {
		t.Fatalf("out-of-scope analysis response = %s", outOfScopeJSON)
	}
	changedTag, err := application.CreateTag(note.TagCreateInput{Name: "changed-after-analysis"})
	if err != nil || changedTag.Tag == nil {
		t.Fatalf("create changed tag: result=%+v err=%v", changedTag, err)
	}
	if result, err := application.SetNoteTags(publicNote.ID, note.SetNoteTagsInput{TagIDs: []string{changedTag.Tag.ID}}); err != nil || result.Error != nil {
		t.Fatalf("change analyzed tags: result=%+v err=%v", result, err)
	}
	staleAfterTagChange := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeGetCandidates, readapi.OrganizeCandidatesInput{AnalysisID: mcpAnalysisID})
	if staleAfterTagChange.Error == nil || staleAfterTagChange.Error.Code != "ANALYSIS_UNAVAILABLE" {
		t.Fatalf("analysis survived tag state change: %+v", staleAfterTagChange)
	}
	hiddenNote, err := application.CreateNote(note.CreateInput{
		NotebookID: &publicNotebook.ID, Title: "Protected child marker", Content: "# Protected proposal marker",
	})
	if err != nil {
		t.Fatal(err)
	}
	hiddenProtection := application.EnableContentLock(contentlock.EnableInput{
		TargetType: contentlock.TargetNote, TargetID: hiddenNote.ID, Passphrase: "another correct horse battery staple",
	})
	if hiddenProtection.Error != nil {
		t.Fatalf("protect hidden note: %+v", hiddenProtection.Error)
	}
	notebookClient, err := rootClient.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NotebookIDs: []string{publicNotebook.ID}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = notebookClient.RevokeMCPSession(context.Background()) })
	notebookAnalysis := callExternalOrganization(t, notebookClient, readapi.OperationOrganizeAnalyze, readapi.OrganizeAnalyzeInput{
		Scope: "notebook", NotebookID: publicNotebook.ID, Limit: 1,
	})
	notebookAnalysisID, _ := responseDataMap(t, notebookAnalysis)["analysisId"].(string)
	if notebookAnalysisID == "" {
		t.Fatalf("notebook analysis = %+v", notebookAnalysis)
	}
	notebookJSON, _ := json.Marshal(notebookAnalysis)
	if strings.Contains(string(notebookJSON), hiddenNote.ID) || strings.Contains(string(notebookJSON), "Protected child marker") || strings.Contains(string(notebookJSON), `"skippedLocked":1`) {
		t.Fatalf("protected child count or metadata leaked: %s", notebookJSON)
	}
	if _, err := application.notes.Update(t.Context(), scopeNote.ID, note.UpdateInput{NotebookID: &privateNotebook.ID, ExpectedRevision: &scopeNote.Revision}); err != nil {
		t.Fatal(err)
	}
	changedScope := callExternalOrganization(t, notebookClient, readapi.OperationOrganizeGetCandidates, readapi.OrganizeCandidatesInput{AnalysisID: notebookAnalysisID})
	changedScopeJSON, _ := json.Marshal(changedScope)
	if changedScope.Error == nil || changedScope.Error.Code != "ANALYSIS_UNAVAILABLE" || strings.Contains(string(changedScopeJSON), privateNotebook.ID) {
		t.Fatalf("changed notebook scope remained readable: %s", changedScopeJSON)
	}

	freshBeforeProtection := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeAnalyze, readapi.OrganizeAnalyzeInput{
		Scope: "note", NoteID: publicNote.ID, Limit: 1,
	})
	freshAnalysisID, _ := responseDataMap(t, freshBeforeProtection)["analysisId"].(string)
	if freshAnalysisID == "" {
		t.Fatalf("fresh analysis before protection = %+v", freshBeforeProtection)
	}
	protected := application.EnableContentLock(contentlock.EnableInput{
		TargetType: contentlock.TargetNote, TargetID: publicNote.ID, Passphrase: "correct horse battery staple",
	})
	if protected.Error != nil {
		t.Fatalf("protect note: %+v", protected.Error)
	}
	staleAfterProtection := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeGetCandidates, readapi.OrganizeCandidatesInput{AnalysisID: freshAnalysisID})
	if staleAfterProtection.Error == nil || staleAfterProtection.Error.Code != "ANALYSIS_UNAVAILABLE" {
		t.Fatalf("protected analysis remained readable: %+v", staleAfterProtection)
	}
	protectedAnalyze := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeAnalyze, readapi.OrganizeAnalyzeInput{Scope: "note", NoteID: publicNote.ID})
	if protectedAnalyze.Error == nil || protectedAnalyze.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("protected note was analyzed: %+v", protectedAnalyze)
	}
	locked := application.LockContentNow(contentlock.Target{Type: contentlock.TargetNote, ID: publicNote.ID})
	if locked.Error != nil {
		t.Fatalf("lock note: %+v", locked.Error)
	}
	lockedAnalyze := callExternalOrganization(t, mcpClient, readapi.OperationOrganizeAnalyze, readapi.OrganizeAnalyzeInput{Scope: "note", NoteID: publicNote.ID})
	if lockedAnalyze.Error == nil || lockedAnalyze.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("locked note was analyzed: %+v", lockedAnalyze)
	}

	trash := true
	if _, err := application.notes.Update(t.Context(), privateNote.ID, note.UpdateInput{IsTrashed: &trash, ExpectedRevision: &privateNote.Revision}); err != nil {
		t.Fatal(err)
	}
	trashClient, err := rootClient.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NoteIDs: []string{privateNote.ID}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = trashClient.RevokeMCPSession(context.Background()) })
	trashedAnalyze := callExternalOrganization(t, trashClient, readapi.OperationOrganizeAnalyze, readapi.OrganizeAnalyzeInput{Scope: "note", NoteID: privateNote.ID})
	trashedJSON, _ := json.Marshal(trashedAnalyze)
	if trashedAnalyze.Error == nil || trashedAnalyze.Error.Code != "RESOURCE_UNAVAILABLE" || strings.Contains(string(trashedJSON), "Private") {
		t.Fatalf("trashed note was exposed: %s", trashedJSON)
	}

	toolsOutput := runMCPTranscript(t, []string{"--note", privateNote.ID},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	if !strings.Contains(toolsOutput, readapi.OperationOrganizeAnalyze) || !strings.Contains(toolsOutput, readapi.OperationOrganizeGetCandidates) {
		t.Fatalf("Stage B MCP tools are missing: %s", toolsOutput)
	}
	defaultOutput := runMCPTranscript(t, nil,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"organize.analyze","arguments":{"scope":"space"}}}`,
	)
	if !strings.Contains(defaultOutput, `"code":"PERMISSION_DENIED"`) || strings.Contains(defaultOutput, privateNote.ID) {
		t.Fatalf("default MCP scope analyzed notes: %s", defaultOutput)
	}
}

func TestExternalOrganizationRequiresProposalAndContentPermissions(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	application := newApp("test")
	application.startup(t.Context())
	t.Cleanup(func() { application.shutdown(context.Background()) })
	created, err := application.CreateNote(note.CreateInput{Title: "Permission test", Content: "# Proposed title"})
	if err != nil {
		t.Fatal(err)
	}
	service := readapi.New(application.notes, application.contentLocks, application.activeSpace.ID, application.organizer)
	params, _ := json.Marshal(readapi.OrganizeAnalyzeInput{Scope: "note", NoteID: created.ID})
	for _, test := range []struct {
		name        string
		permissions map[string]bool
		wantStatus  string
		wantCode    string
	}{
		{name: "P only", permissions: map[string]bool{readapi.PermissionProposal: true}, wantStatus: readapi.StatusRejected, wantCode: "PERMISSION_DENIED"},
		{name: "R1 only", permissions: map[string]bool{readapi.PermissionContent: true}, wantStatus: readapi.StatusRejected, wantCode: "PERMISSION_DENIED"},
		{name: "P and R1", permissions: map[string]bool{readapi.PermissionProposal: true, readapi.PermissionContent: true}, wantStatus: readapi.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			clientID := "organization-permission-" + strings.ReplaceAll(test.name, " ", "-")
			response := service.Execute(t.Context(), readapi.Principal{
				ClientID: clientID, StorageSpaceID: application.activeSpace.ID, Permissions: test.permissions,
			}, readapi.Request{
				APIVersion: readapi.APIVersion, RequestID: clientID, ClientID: clientID,
				Scope: readapi.Scope{StorageSpaceID: application.activeSpace.ID}, Operation: readapi.OperationOrganizeAnalyze, Params: params,
			})
			if response.Status != test.wantStatus {
				t.Fatalf("status = %q, want %q: %+v", response.Status, test.wantStatus, response)
			}
			if test.wantCode != "" && (response.Error == nil || response.Error.Code != test.wantCode) {
				t.Fatalf("error = %+v, want %s", response.Error, test.wantCode)
			}
		})
	}

	ownerID := "organization-candidate-permission-owner"
	fullPermissions := map[string]bool{readapi.PermissionProposal: true, readapi.PermissionContent: true}
	analysisResponse := service.Execute(t.Context(), readapi.Principal{
		ClientID: ownerID, StorageSpaceID: application.activeSpace.ID, Permissions: fullPermissions,
	}, readapi.Request{
		APIVersion: readapi.APIVersion, RequestID: ownerID + "-analyze", ClientID: ownerID,
		Scope: readapi.Scope{StorageSpaceID: application.activeSpace.ID}, Operation: readapi.OperationOrganizeAnalyze, Params: params,
	})
	analysisData, ok := analysisResponse.Data.(readapi.OrganizeAnalyzeData)
	if !ok || analysisData.AnalysisID == "" {
		t.Fatalf("authorized analysis = %+v", analysisResponse)
	}
	for _, test := range []struct {
		name        string
		permissions map[string]bool
		wantStatus  string
		wantCode    string
	}{
		{name: "P only", permissions: map[string]bool{readapi.PermissionProposal: true}, wantStatus: readapi.StatusRejected, wantCode: "PERMISSION_DENIED"},
		{name: "R1 only", permissions: map[string]bool{readapi.PermissionContent: true}, wantStatus: readapi.StatusRejected, wantCode: "PERMISSION_DENIED"},
		{name: "P and R1", permissions: fullPermissions, wantStatus: readapi.StatusOK},
	} {
		t.Run("candidates "+test.name, func(t *testing.T) {
			candidateParams, _ := json.Marshal(readapi.OrganizeCandidatesInput{AnalysisID: analysisData.AnalysisID})
			response := service.Execute(t.Context(), readapi.Principal{
				ClientID: ownerID, StorageSpaceID: application.activeSpace.ID, Permissions: test.permissions,
			}, readapi.Request{
				APIVersion: readapi.APIVersion, RequestID: ownerID + "-" + test.name, ClientID: ownerID,
				Scope: readapi.Scope{StorageSpaceID: application.activeSpace.ID}, Operation: readapi.OperationOrganizeGetCandidates, Params: candidateParams,
			})
			if response.Status != test.wantStatus {
				t.Fatalf("status = %q, want %q: %+v", response.Status, test.wantStatus, response)
			}
			if test.wantCode != "" && (response.Error == nil || response.Error.Code != test.wantCode) {
				t.Fatalf("error = %+v, want %s", response.Error, test.wantCode)
			}
		})
	}
}

func TestExternalOrganizationAndProtectionChangeDoNotDeadlock(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataRoot)
	application := newApp("test")
	application.startup(t.Context())
	t.Cleanup(func() { application.shutdown(context.Background()) })
	created, err := application.CreateNote(note.CreateInput{Title: "Concurrent organization", Content: "# Proposed title"})
	if err != nil {
		t.Fatal(err)
	}

	protectionDone := make(chan contentlock.MutationResult, 1)
	reader := &organizationRaceReader{Service: application.notes}
	reader.afterBegin = func() {
		started := make(chan struct{})
		go func() {
			close(started)
			protectionDone <- application.EnableContentLock(contentlock.EnableInput{
				TargetType: contentlock.TargetNote, TargetID: created.ID, Passphrase: "correct horse battery staple",
			})
		}()
		<-started
		time.Sleep(100 * time.Millisecond)
	}
	service := readapi.New(reader, application.contentLocks, application.activeSpace.ID, application.organizer)
	params, _ := json.Marshal(readapi.OrganizeAnalyzeInput{Scope: "note", NoteID: created.ID})
	responseDone := make(chan readapi.Response, 1)
	go func() {
		responseDone <- service.Execute(context.Background(), readapi.Principal{
			ClientID: "organization-race", StorageSpaceID: application.activeSpace.ID,
			Permissions: map[string]bool{readapi.PermissionProposal: true, readapi.PermissionContent: true},
		}, readapi.Request{
			APIVersion: readapi.APIVersion, RequestID: "organization-race", ClientID: "organization-race",
			Scope: readapi.Scope{StorageSpaceID: application.activeSpace.ID}, Operation: readapi.OperationOrganizeAnalyze, Params: params,
		})
	}()

	select {
	case response := <-responseDone:
		if response.Status != readapi.StatusOK {
			t.Fatalf("organization response = %+v", response)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("external organization analysis deadlocked with protection change")
	}
	select {
	case result := <-protectionDone:
		if result.Error != nil {
			t.Fatalf("protection change failed: %+v", result.Error)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("protection change did not finish after external analysis")
	}
}

func callExternalOrganization(t *testing.T, client *localipc.Client, operation string, input any) readapi.Response {
	t.Helper()
	requestID, err := localipc.NewRequestID()
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Call(t.Context(), operation, input, requestID)
	if err != nil {
		t.Fatalf("call %s: %v", operation, err)
	}
	return response
}

func responseDataMap(t *testing.T, response readapi.Response) map[string]any {
	t.Helper()
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("response data type = %T (%+v)", response.Data, response.Data)
	}
	return data
}

type organizationRaceReader struct {
	*note.Service
	afterBegin func()
}

func (reader *organizationRaceReader) BeginExternalRead(ctx context.Context) (context.Context, func()) {
	ctx, release := reader.Service.BeginExternalRead(ctx)
	if reader.afterBegin != nil {
		reader.afterBegin()
		reader.afterBegin = nil
	}
	return ctx, release
}
