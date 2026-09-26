package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/localipc"
	"atlasnote/internal/note"
	"atlasnote/internal/notespace"
	"atlasnote/internal/organize"
	"atlasnote/internal/readapi"
)

func setupChangeApp(t *testing.T) (*App, *localipc.Client, note.Notebook, note.Notebook, note.Note) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", root)
	a := newApp("test")
	a.startup(t.Context())
	t.Cleanup(func() { a.shutdown(context.Background()) })
	if !a.GetStartupStatus().Ready {
		t.Fatal("app is not ready")
	}
	public, err := a.CreateNotebook(note.NotebookCreateInput{Name: "Published"})
	if err != nil {
		t.Fatal(err)
	}
	private, err := a.CreateNotebook(note.NotebookCreateInput{Name: "Private"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := a.CreateNote(note.CreateInput{NotebookID: &public.ID, Title: "Original", Content: "old body"})
	if err != nil {
		t.Fatal(err)
	}
	client, err := localipc.Connect(root)
	if err != nil {
		t.Fatal(err)
	}
	return a, client, public, private, target
}

func changeID(t *testing.T, response readapi.Response) string {
	t.Helper()
	if response.Status != readapi.StatusPending {
		t.Fatalf("request was not pending: %+v", response)
	}
	id, _ := responseDataMap(t, response)["operationId"].(string)
	if id == "" {
		t.Fatalf("missing operation ID: %+v", response)
	}
	return id
}

func assertUnchanged(t *testing.T, a *App, original note.Note) {
	t.Helper()
	current, err := a.notes.Get(t.Context(), original.ID)
	if err != nil || current.Title != original.Title || current.Content != original.Content || current.Revision != original.Revision || current.IsTrashed != original.IsTrashed {
		t.Fatalf("note changed without approval: %+v, %v", current, err)
	}
}

func TestExternalChangeCapacityReclaimsCompletedOperations(t *testing.T) {
	for _, completedState := range []string{"applied", readapi.StatusRejected} {
		t.Run(completedState, func(t *testing.T) {
			a, root, _, _, target := setupChangeApp(t)
			oldestID := ""
			for index := 0; index < 64; index++ {
				current, err := a.notes.Get(t.Context(), target.ID)
				if err != nil {
					t.Fatal(err)
				}
				id := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestUpdate, map[string]any{
					"noteId": target.ID, "expectedRevision": current.Revision, "patch": map[string]any{"title": fmt.Sprintf("Updated %d", index)},
				}))
				if index == 0 {
					oldestID = id
				}
				var result readapi.ChangeResult
				if completedState == "applied" {
					result = a.ApproveExternalChange(id)
				} else {
					result = a.RejectExternalChange(id)
				}
				if result.State != completedState || a.ApproveExternalChange(id).State != "unavailable" {
					t.Fatalf("completed operation could be reapplied: %+v", result)
				}
			}
			current, err := a.notes.Get(t.Context(), target.ID)
			if err != nil {
				t.Fatal(err)
			}
			nextID := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestUpdate, map[string]any{
				"noteId": target.ID, "expectedRevision": current.Revision, "patch": map[string]any{"title": "65th request"},
			}))
			if response := callExternalOrganization(t, root, readapi.OperationOperationsGet, map[string]any{"operationId": oldestID}); response.Error == nil || response.Error.Code != "RESOURCE_UNAVAILABLE" {
				t.Fatalf("oldest completed operation was not reclaimed: %+v", response)
			}
			if response := callExternalOrganization(t, root, readapi.OperationOperationsGet, map[string]any{"operationId": nextID}); response.Status != readapi.StatusOK {
				t.Fatalf("owner cannot read new operation: %+v", response)
			}
			other, err := root.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NoteIDs: []string{target.ID}})
			if err != nil {
				t.Fatal(err)
			}
			if response := callExternalOrganization(t, other, readapi.OperationOperationsGet, map[string]any{"operationId": nextID}); response.Error == nil || response.Error.Code != "RESOURCE_UNAVAILABLE" {
				t.Fatalf("other client read new operation: %+v", response)
			}
			wrongSpace := a.changeService.Execute(t.Context(), readapi.Principal{ClientID: "other", StorageSpaceID: "other-space", Permissions: map[string]bool{readapi.PermissionMetadata: true}}, readapi.Request{
				APIVersion: readapi.APIVersion, RequestID: "wrong-space", ClientID: "other", Scope: readapi.Scope{StorageSpaceID: a.activeSpace.ID}, Operation: readapi.OperationOperationsGet,
				Params: json.RawMessage(`{"operationId":"` + nextID + `"}`),
			})
			if wrongSpace.Error == nil || wrongSpace.Error.Code != "SCOPE_MISMATCH" {
				t.Fatalf("other storage space read new operation: %+v", wrongSpace)
			}
		})
	}
}

func TestExternalChangeCapacityKeepsPendingOperations(t *testing.T) {
	a, root, _, _, target := setupChangeApp(t)
	firstID := ""
	for index := 0; index < 64; index++ {
		id := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestUpdate, map[string]any{
			"noteId": target.ID, "expectedRevision": target.Revision, "patch": map[string]any{"title": fmt.Sprintf("Pending %d", index)},
		}))
		if index == 0 {
			firstID = id
		}
	}
	denied := callExternalOrganization(t, root, readapi.OperationNotesRequestUpdate, map[string]any{
		"noteId": target.ID, "expectedRevision": target.Revision, "patch": map[string]any{"title": "65th pending"},
	})
	if denied.Error == nil || denied.Error.Code != "CHANGE_LIMIT" || len(a.ListExternalChangeReviews()) != 64 {
		t.Fatalf("pending capacity was not preserved: %+v", denied)
	}
	if response := callExternalOrganization(t, root, readapi.OperationOperationsGet, map[string]any{"operationId": firstID}); response.Status != readapi.StatusOK || responseDataMap(t, response)["state"] != readapi.StatusPending {
		t.Fatalf("oldest pending operation was evicted: %+v", response)
	}
}

func TestExternalChangesRequireGUIApprovalAndBindClient(t *testing.T) {
	a, root, public, private, target := setupChangeApp(t)
	input := map[string]any{"noteId": target.ID, "expectedRevision": target.Revision, "patch": map[string]any{"title": "Approved"}}
	response := callExternalOrganization(t, root, readapi.OperationNotesRequestUpdate, input)
	id := changeID(t, response)
	assertUnchanged(t, a, target)
	unsafePatch := callExternalOrganization(t, root, readapi.OperationNotesRequestUpdate, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision, "patch": map[string]any{"isTrashed": true}})
	if unsafePatch.Error == nil || unsafePatch.Error.Code != "INVALID_ARGUMENT" {
		t.Fatalf("arbitrary patch accepted: %+v", unsafePatch)
	}
	cli, code := runReadCommand(t, "notes", "request-update", "--input", mustJSON(t, input), "--json")
	if code != 0 || cli.Status != readapi.StatusPending {
		t.Fatalf("CLI request = %d %+v", code, cli)
	}
	cliID, _ := responseDataMap(t, cli)["operationId"].(string)
	cliState, stateCode := runReadCommand(t, "operations", "get", cliID, "--json")
	if stateCode != 0 || cliState.Status != readapi.StatusOK || responseDataMap(t, cliState)["state"] != readapi.StatusPending {
		t.Fatalf("one-shot CLI state = %d %+v", stateCode, cliState)
	}
	assertUnchanged(t, a, target)
	for _, operation := range []string{"notes.apply", "operations.approve", "operations.issue_token", "organize.apply"} {
		denied := callExternalOrganization(t, root, operation, map[string]any{"operationId": id})
		if denied.Error == nil || denied.Error.Code != "OPERATION_NOT_ALLOWED" {
			t.Fatalf("external apply exposed for %s: %+v", operation, denied)
		}
	}
	if _, code := runReadCommand(t, "operations", "approve", id, "--json"); code != 2 {
		t.Fatalf("CLI approval command returned %d", code)
	}
	tools := runMCPTranscript(t, []string{"--note", target.ID},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"operations.approve","arguments":{"operationId":"`+id+`"}}}`,
	)
	if !strings.Contains(tools, `"name":"notes.request_update"`) || strings.Contains(tools, `"name":"operations.approve"`) || !strings.Contains(tools, `"code":"OPERATION_NOT_ALLOWED"`) {
		t.Fatalf("MCP tool boundary: %s", tools)
	}
	other, err := root.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NoteIDs: []string{target.ID}})
	if err != nil {
		t.Fatal(err)
	}
	privateRead := callExternalOrganization(t, other, readapi.OperationOperationsGet, map[string]any{"operationId": id})
	if privateRead.Error == nil || privateRead.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("other client read operation: %+v", privateRead)
	}
	wrongScope := callExternalOrganization(t, other, readapi.OperationNotesRequestMove, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision, "targetNotebookId": private.ID})
	if wrongScope.Error == nil || wrongScope.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("move to unpublished notebook: %+v", wrongScope)
	}
	create := callExternalOrganization(t, other, readapi.OperationNotesRequestCreate, map[string]any{"title": "Created", "content": "body", "notebookId": private.ID})
	if create.Error == nil || create.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("create in unpublished notebook: %+v", create)
	}
	create = callExternalOrganization(t, other, readapi.OperationNotesRequestCreate, map[string]any{"title": "Created", "content": "body", "notebookId": public.ID})
	if create.Error == nil || create.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("note-only scope created note: %+v", create)
	}
	wrongNote := callExternalOrganization(t, other, readapi.OperationNotesRequestUpdate, map[string]any{"noteId": strings.Repeat("a", 32), "expectedRevision": 1, "patch": map[string]any{"title": "Hidden"}})
	if wrongNote.Error == nil || wrongNote.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("scope leak: %+v", wrongNote)
	}
	privateNote, err := a.CreateNote(note.CreateInput{NotebookID: &private.ID, Title: "Private marker", Content: "secret body"})
	if err != nil {
		t.Fatal(err)
	}
	privateRequest := callExternalOrganization(t, other, readapi.OperationNotesRequestUpdate, map[string]any{"noteId": privateNote.ID, "expectedRevision": privateNote.Revision, "patch": map[string]any{"title": "Hidden"}})
	encodedPrivate, _ := json.Marshal(privateRequest)
	if privateRequest.Error == nil || privateRequest.Error.Code != "RESOURCE_UNAVAILABLE" || strings.Contains(string(encodedPrivate), "Private marker") {
		t.Fatalf("existing scope leak: %s", encodedPrivate)
	}
	current := callExternalOrganization(t, root, readapi.OperationOperationsGet, map[string]any{"operationId": id})
	serialized, _ := json.Marshal(current)
	if strings.Contains(string(serialized), "Approved") || strings.Contains(string(serialized), "old body") {
		t.Fatalf("external operation leaked review payload: %s", serialized)
	}
	if len(a.ListExternalChangeReviews()) == 0 {
		t.Fatal("GUI review missing")
	}
	result := a.ApproveExternalChange(id)
	if result.State != "applied" {
		t.Fatalf("GUI approval: %+v", result)
	}
	updated, err := a.notes.Get(t.Context(), target.ID)
	if err != nil || updated.Title != "Approved" || updated.Revision != target.Revision+1 {
		t.Fatalf("approved change was not saved: %+v %v", updated, err)
	}
	if replay := a.ApproveExternalChange(id); replay.State != "unavailable" {
		t.Fatalf("approval token replay accepted: %+v", replay)
	}
}

func TestExternalChangeConflictsAndBeforeVerification(t *testing.T) {
	a, root, _, _, target := setupChangeApp(t)
	badBefore := callExternalOrganization(t, root, readapi.OperationNotesProposeEdit, map[string]any{"noteId": target.ID, "baseRevision": target.Revision, "before": "fake", "after": "new", "reason": "test"})
	if badBefore.Error == nil || badBefore.Error.Code != "BEFORE_MISMATCH" {
		t.Fatalf("fake before accepted: %+v", badBefore)
	}
	id := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesProposeEdit, map[string]any{"noteId": target.ID, "baseRevision": target.Revision, "before": "old body", "after": "new body", "reason": "test"}))
	assertUnchanged(t, a, target)
	newTitle := "Concurrent"
	_, err := a.notes.Update(t.Context(), target.ID, note.UpdateInput{Title: &newTitle, ExpectedRevision: &target.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if result := a.ApproveExternalChange(id); result.State != "conflict" {
		t.Fatalf("stale approval = %+v", result)
	}
	current, _ := a.notes.Get(t.Context(), target.ID)
	if current.Content != target.Content {
		t.Fatalf("stale proposal overwrote content: %+v", current)
	}
	views := a.ListExternalChangeReviews()
	if len(views) == 0 || views[0].Items[0].After == nil {
		t.Fatal("conflicted proposal was lost")
	}
	stale := callExternalOrganization(t, root, readapi.OperationNotesRequestTrash, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision})
	if stale.Status != readapi.StatusConflict {
		t.Fatalf("request time revision not checked: %+v", stale)
	}
}

func TestExternalChangeRestrictedScopeAndProtection(t *testing.T) {
	a, root, public, private, target := setupChangeApp(t)
	session, err := root.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NotebookIDs: []string{public.ID}})
	if err != nil {
		t.Fatal(err)
	}
	createID := changeID(t, callExternalOrganization(t, session, readapi.OperationNotesRequestCreate, map[string]any{"title": "Allowed", "content": "body", "notebookId": public.ID}))
	if result := a.RejectExternalChange(createID); result.State != readapi.StatusRejected {
		t.Fatalf("reject = %+v", result)
	}
	if result := a.ApproveExternalChange(createID); result.State != "unavailable" {
		t.Fatalf("rejected operation applied: %+v", result)
	}
	denied := callExternalOrganization(t, session, readapi.OperationNotesRequestMove, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision, "targetNotebookId": private.ID})
	if denied.Error == nil || denied.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("unpublished move = %+v", denied)
	}
	pendingID := changeID(t, callExternalOrganization(t, session, readapi.OperationNotesRequestUpdate, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision, "patch": map[string]any{"content": "sensitive proposal"}}))
	protected := a.EnableContentLock(contentlock.EnableInput{TargetType: contentlock.TargetNote, TargetID: target.ID, Passphrase: "correct horse battery staple"})
	if protected.Error != nil {
		t.Fatal(protected.Error)
	}
	denied = callExternalOrganization(t, session, readapi.OperationNotesRequestTrash, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision})
	if denied.Error == nil || denied.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("protected note request = %+v", denied)
	}
	locked := a.LockContentNow(contentlock.Target{Type: contentlock.TargetNote, ID: target.ID})
	if locked.Error != nil {
		t.Fatal(locked.Error)
	}
	denied = callExternalOrganization(t, session, readapi.OperationNotesRequestUpdate, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision, "patch": map[string]any{"title": "Bad"}})
	if denied.Error == nil || denied.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("locked note request = %+v", denied)
	}
	for _, review := range a.ListExternalChangeReviews() {
		if review.OperationID == pendingID {
			if review.State != "unavailable" || review.Items == nil || len(review.Items) != 0 || len(review.TargetIDs) != 0 {
				t.Fatalf("locked review was not safely redacted: %+v", review)
			}
		}
	}
}

func TestExternalOrganizationCandidateServerSnapshot(t *testing.T) {
	a, root, _, _, target := setupChangeApp(t)
	analysis := callExternalOrganization(t, root, readapi.OperationOrganizeAnalyze, map[string]any{"scope": "note", "noteId": target.ID})
	analysisID, _ := responseDataMap(t, analysis)["analysisId"].(string)
	forged := callExternalOrganization(t, root, readapi.OperationOrganizeRequestApply, map[string]any{"analysisId": analysisID, "candidateIds": []string{strings.Repeat("f", 32)}})
	if forged.Error == nil || forged.Error.Code != "ANALYSIS_UNAVAILABLE" {
		t.Fatalf("forged candidate accepted: %+v", forged)
	}
	if len(a.ListExternalChangeReviews()) != 0 {
		t.Fatal("forged candidate created operation")
	}
}

func TestExternalOrganizationApplyKeepsPartialResults(t *testing.T) {
	a, root, public, _, first := setupChangeApp(t)
	second, err := a.CreateNote(note.CreateInput{NotebookID: &public.ID, Title: "Second old", Content: "# Second proposed"})
	if err != nil {
		t.Fatal(err)
	}
	firstContent := "# First proposed"
	_, err = a.notes.Update(t.Context(), first.ID, note.UpdateInput{Content: &firstContent, ExpectedRevision: &first.Revision})
	if err != nil {
		t.Fatal(err)
	}
	first, err = a.notes.Get(t.Context(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	analysis := callExternalOrganization(t, root, readapi.OperationOrganizeAnalyze, map[string]any{"scope": "space"})
	if analysis.Status != readapi.StatusOK {
		t.Fatalf("analysis = %+v", analysis)
	}
	analysisID, _ := responseDataMap(t, analysis)["analysisId"].(string)
	candidates := callExternalOrganization(t, root, readapi.OperationOrganizeGetCandidates, map[string]any{"analysisId": analysisID, "kind": "title", "limit": 100})
	if candidates.Status != readapi.StatusOK {
		t.Fatalf("candidates = %+v", candidates)
	}
	var ids []string
	for _, raw := range responseDataMap(t, candidates)["candidates"].([]any) {
		candidate := raw.(map[string]any)
		if candidate["noteId"] == first.ID || candidate["noteId"] == second.ID {
			ids = append(ids, candidate["id"].(string))
		}
	}
	if len(ids) != 2 {
		t.Fatalf("title candidates = %#v", ids)
	}
	id := changeID(t, callExternalOrganization(t, root, readapi.OperationOrganizeRequestApply, map[string]any{"analysisId": analysisID, "candidateIds": ids}))
	if result := a.ApplyOrganizationCandidates(structuredApplyInput(analysisID, ids)); len(result) == 0 || result[0].Status != "stale" {
		t.Fatalf("external session reached legacy GUI apply: %+v", result)
	}
	newTitle := "Concurrent second"
	_, err = a.notes.Update(t.Context(), second.ID, note.UpdateInput{Title: &newTitle, ExpectedRevision: &second.Revision})
	if err != nil {
		t.Fatal(err)
	}
	result := a.ApproveExternalChange(id)
	if len(result.Items) != 2 {
		t.Fatalf("partial result missing: %+v", result)
	}
	statuses := map[string]int{}
	for _, item := range result.Items {
		statuses[item.Status]++
	}
	if statuses["applied"] != 1 || statuses["conflict"] != 1 {
		t.Fatalf("partial result = %+v", result)
	}
	firstAfter, _ := a.notes.Get(t.Context(), first.ID)
	secondAfter, _ := a.notes.Get(t.Context(), second.ID)
	if firstAfter.Title != "First proposed" || secondAfter.Title != newTitle {
		t.Fatalf("partial apply changed wrong content: %+v %+v", firstAfter, secondAfter)
	}
	staleRequest := callExternalOrganization(t, root, readapi.OperationOrganizeRequestApply, map[string]any{"analysisId": analysisID, "candidateIds": ids})
	if staleRequest.Error == nil || staleRequest.Error.Code != "ANALYSIS_UNAVAILABLE" {
		t.Fatalf("stale analysis was accepted: %+v", staleRequest)
	}
}

func TestExternalTagConflictAndTrashBoundary(t *testing.T) {
	a, root, _, _, target := setupChangeApp(t)
	one, err := a.CreateTag(note.TagCreateInput{Name: "one"})
	if err != nil || one.Tag == nil {
		t.Fatal(err)
	}
	two, err := a.CreateTag(note.TagCreateInput{Name: "two"})
	if err != nil || two.Tag == nil {
		t.Fatal(err)
	}
	id := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestTags, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision, "addTagIds": []string{one.Tag.ID}}))
	if result, err := a.SetNoteTags(target.ID, note.SetNoteTagsInput{TagIDs: []string{two.Tag.ID}}); err != nil || result.Error != nil {
		t.Fatal(err)
	}
	if result := a.ApproveExternalChange(id); result.State != "conflict" {
		t.Fatalf("tag conflict = %+v", result)
	}
	tags, err := a.notes.ListNoteTags(t.Context(), target.ID)
	if err != nil || len(tags.Tags) != 1 || tags.Tags[0].ID != two.Tag.ID {
		t.Fatalf("tag state overwritten: %+v %v", tags, err)
	}
	trash := true
	_, err = a.notes.Update(t.Context(), target.ID, note.UpdateInput{IsTrashed: &trash, ExpectedRevision: &target.Revision})
	if err != nil {
		t.Fatal(err)
	}
	denied := callExternalOrganization(t, root, readapi.OperationNotesRequestTrash, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision + 1})
	if denied.Error == nil || denied.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("trashed note accepted: %+v", denied)
	}
}

func TestRestrictedTagRequestRechecksVisibilityAtApproval(t *testing.T) {
	a, root, public, _, target := setupChangeApp(t)
	tagResult, err := a.CreateTag(note.TagCreateInput{Name: "scope-hidden-tag"})
	if err != nil || tagResult.Tag == nil {
		t.Fatal(err)
	}
	tagID := tagResult.Tag.ID
	source, err := a.CreateNote(note.CreateInput{NotebookID: &public.ID, Title: "Tag source", Content: "source"})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := a.SetNoteTags(source.ID, note.SetNoteTagsInput{TagIDs: []string{tagID}}); err != nil || result.Error != nil {
		t.Fatalf("publish tag: %+v %v", result, err)
	}
	session, err := root.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NotebookIDs: []string{public.ID}})
	if err != nil {
		t.Fatal(err)
	}
	id := changeID(t, callExternalOrganization(t, session, readapi.OperationNotesRequestTags, map[string]any{
		"noteId": target.ID, "expectedRevision": target.Revision, "addTagIds": []string{tagID},
	}))
	if result, err := a.SetNoteTags(source.ID, note.SetNoteTagsInput{TagIDs: []string{}}); err != nil || result.Error != nil {
		t.Fatalf("remove published tag: %+v %v", result, err)
	}
	result := a.ApproveExternalChange(id)
	if result.State != "conflict" || strings.Contains(result.Message, tagResult.Tag.Name) {
		t.Fatalf("hidden tag request was applied or leaked a name: %+v", result)
	}
	tags, err := a.notes.ListNoteTags(t.Context(), target.ID)
	if err != nil || len(tags.Tags) != 0 {
		t.Fatalf("hidden tag was assigned: %+v %v", tags, err)
	}
	external := callExternalOrganization(t, session, readapi.OperationOperationsGet, map[string]any{"operationId": id})
	serialized, _ := json.Marshal(external)
	if external.Status != readapi.StatusOK || strings.Contains(string(serialized), tagResult.Tag.Name) {
		t.Fatalf("operation state leaked hidden tag: %s", serialized)
	}
	cliID := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestTags, map[string]any{
		"noteId": target.ID, "expectedRevision": target.Revision, "addTagIds": []string{tagID},
	}))
	if result := a.ApproveExternalChange(cliID); result.State != "applied" {
		t.Fatalf("unrestricted CLI tag request changed: %+v", result)
	}
}

func TestRestrictedTagRequestAppliesWhileTagRemainsVisible(t *testing.T) {
	a, root, public, _, target := setupChangeApp(t)
	tagResult, err := a.CreateTag(note.TagCreateInput{Name: "visible-tag"})
	if err != nil || tagResult.Tag == nil {
		t.Fatal(err)
	}
	source, err := a.CreateNote(note.CreateInput{NotebookID: &public.ID, Title: "Tag source", Content: "source"})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := a.SetNoteTags(source.ID, note.SetNoteTagsInput{TagIDs: []string{tagResult.Tag.ID}}); err != nil || result.Error != nil {
		t.Fatalf("publish tag: %+v %v", result, err)
	}
	session, err := root.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NotebookIDs: []string{public.ID}})
	if err != nil {
		t.Fatal(err)
	}
	id := changeID(t, callExternalOrganization(t, session, readapi.OperationNotesRequestTags, map[string]any{
		"noteId": target.ID, "expectedRevision": target.Revision, "addTagIds": []string{tagResult.Tag.ID},
	}))
	if result := a.ApproveExternalChange(id); result.State != "applied" {
		t.Fatalf("visible tag request did not apply: %+v", result)
	}
	tags, err := a.notes.ListNoteTags(t.Context(), target.ID)
	if err != nil || len(tags.Tags) != 1 || tags.Tags[0].ID != tagResult.Tag.ID {
		t.Fatalf("visible tag assignment missing: %+v %v", tags, err)
	}
}

func TestRestrictedOrganizationTagCandidateRechecksVisibilityAtApproval(t *testing.T) {
	a, root, public, _, target := setupChangeApp(t)
	tagResult, err := a.CreateTag(note.TagCreateInput{Name: "candidate-scope-tag"})
	if err != nil || tagResult.Tag == nil {
		t.Fatal(err)
	}
	tagID := tagResult.Tag.ID
	source, err := a.CreateNote(note.CreateInput{NotebookID: &public.ID, Title: "Tag source", Content: "source"})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := a.SetNoteTags(source.ID, note.SetNoteTagsInput{TagIDs: []string{tagID}}); err != nil || result.Error != nil {
		t.Fatalf("publish tag: %+v %v", result, err)
	}
	content := "This note mentions candidate-scope-tag."
	if _, err := a.notes.Update(t.Context(), target.ID, note.UpdateInput{Content: &content, ExpectedRevision: &target.Revision}); err != nil {
		t.Fatal(err)
	}
	session, err := root.CreateMCPSession(t.Context(), localipc.MCPSessionScope{NotebookIDs: []string{public.ID}})
	if err != nil {
		t.Fatal(err)
	}
	analysis := callExternalOrganization(t, session, readapi.OperationOrganizeAnalyze, map[string]any{"scope": "space"})
	if analysis.Status != readapi.StatusOK {
		t.Fatalf("restricted analysis: %+v", analysis)
	}
	analysisID, _ := responseDataMap(t, analysis)["analysisId"].(string)
	candidates := callExternalOrganization(t, session, readapi.OperationOrganizeGetCandidates, map[string]any{"analysisId": analysisID, "kind": "tag-assignment", "limit": 100})
	if candidates.Status != readapi.StatusOK {
		t.Fatalf("tag candidates: %+v", candidates)
	}
	candidateID := ""
	for _, raw := range responseDataMap(t, candidates)["candidates"].([]any) {
		candidate := raw.(map[string]any)
		if candidate["noteId"] == target.ID && candidate["tagId"] == tagID {
			candidateID, _ = candidate["id"].(string)
		}
	}
	if candidateID == "" {
		t.Fatal("restricted tag-assignment candidate missing")
	}
	id := changeID(t, callExternalOrganization(t, session, readapi.OperationOrganizeRequestApply, map[string]any{"analysisId": analysisID, "candidateIds": []string{candidateID}}))
	if result, err := a.SetNoteTags(source.ID, note.SetNoteTagsInput{TagIDs: []string{}}); err != nil || result.Error != nil {
		t.Fatalf("remove published tag: %+v %v", result, err)
	}
	result := a.ApproveExternalChange(id)
	if result.State != "conflict" || strings.Contains(result.Message, tagResult.Tag.Name) {
		t.Fatalf("hidden candidate tag was applied or leaked a name: %+v", result)
	}
	tags, err := a.notes.ListNoteTags(t.Context(), target.ID)
	if err != nil || len(tags.Tags) != 0 {
		t.Fatalf("hidden candidate tag was assigned: %+v %v", tags, err)
	}
	external := callExternalOrganization(t, session, readapi.OperationOperationsGet, map[string]any{"operationId": id})
	serialized, _ := json.Marshal(external)
	if external.Status != readapi.StatusOK || strings.Contains(string(serialized), tagResult.Tag.Name) {
		t.Fatalf("operation state leaked hidden tag: %s", serialized)
	}
	if result, err := a.SetNoteTags(source.ID, note.SetNoteTagsInput{TagIDs: []string{tagID}}); err != nil || result.Error != nil {
		t.Fatalf("restore published tag: %+v %v", result, err)
	}
	visibleID := changeID(t, callExternalOrganization(t, session, readapi.OperationOrganizeRequestApply, map[string]any{"analysisId": analysisID, "candidateIds": []string{candidateID}}))
	if result := a.ApproveExternalChange(visibleID); result.State != "applied" || len(result.Items) != 1 || result.Items[0].Status != "applied" {
		t.Fatalf("visible tag candidate did not apply: %+v", result)
	}
}

func TestExternalCreateMoveTagsAndTrashUseNoteService(t *testing.T) {
	a, root, public, private, _ := setupChangeApp(t)
	createID := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestCreate, map[string]any{"title": "Created from CLI", "content": "created body", "notebookId": public.ID}))
	if result := a.ApproveExternalChange(createID); result.State != "applied" {
		t.Fatalf("create approval = %+v", result)
	}
	summaries, err := a.notes.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var created note.Summary
	for _, item := range summaries {
		if item.Title == "Created from CLI" {
			created = item
		}
	}
	if created.ID == "" || created.NotebookID == nil || *created.NotebookID != public.ID {
		t.Fatalf("created note missing: %+v", created)
	}
	moveID := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestMove, map[string]any{"noteId": created.ID, "expectedRevision": created.Revision, "targetNotebookId": private.ID}))
	if result := a.ApproveExternalChange(moveID); result.State != "applied" {
		t.Fatalf("move approval = %+v", result)
	}
	moved, err := a.notes.Get(t.Context(), created.ID)
	if err != nil || moved.NotebookID == nil || *moved.NotebookID != private.ID {
		t.Fatalf("move did not use Note Service: %+v %v", moved, err)
	}
	tag, err := a.CreateTag(note.TagCreateInput{Name: "external-tag"})
	if err != nil || tag.Tag == nil {
		t.Fatal(err)
	}
	tagsID := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestTags, map[string]any{"noteId": moved.ID, "expectedRevision": moved.Revision, "addTagIds": []string{tag.Tag.ID}}))
	if result := a.ApproveExternalChange(tagsID); result.State != "applied" {
		t.Fatalf("tags approval = %+v", result)
	}
	tags, err := a.notes.ListNoteTags(t.Context(), moved.ID)
	if err != nil || len(tags.Tags) != 1 || tags.Tags[0].ID != tag.Tag.ID {
		t.Fatalf("tag CAS did not apply: %+v %v", tags, err)
	}
	trashID := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestTrash, map[string]any{"noteId": moved.ID, "expectedRevision": moved.Revision}))
	if result := a.ApproveExternalChange(trashID); result.State != "applied" {
		t.Fatalf("trash approval = %+v", result)
	}
	trashed, err := a.notes.Get(t.Context(), moved.ID)
	if err != nil || !trashed.IsTrashed {
		t.Fatalf("trash did not apply: %+v %v", trashed, err)
	}
}

func TestMCPChildExitKeepsPendingGUIReviewUntilOperationExpiry(t *testing.T) {
	a, _, _, _, target := setupChangeApp(t)
	output := runMCPTranscript(t, []string{"--note", target.ID},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"notes.request_update","arguments":{"noteId":"`+target.ID+`","expectedRevision":1,"patch":{"title":"MCP approved"}}}}`,
	)
	if !strings.Contains(output, `"status":"pending_approval"`) {
		t.Fatalf("MCP request failed: %s", output)
	}
	assertUnchanged(t, a, target)
	var id string
	for _, review := range a.ListExternalChangeReviews() {
		if review.Kind == readapi.OperationNotesRequestUpdate {
			id = review.OperationID
		}
	}
	if id == "" {
		t.Fatal("MCP exit lost pending review")
	}
	if result := a.ApproveExternalChange(id); result.State != "applied" {
		t.Fatalf("MCP exit made review inapplicable: %+v", result)
	}
	current, err := a.notes.Get(t.Context(), target.ID)
	if err != nil || current.Title != "MCP approved" {
		t.Fatalf("MCP review not applied: %+v %v", current, err)
	}
}

func TestAppShutdownMakesPendingOperationInapplicable(t *testing.T) {
	a, root, _, _, target := setupChangeApp(t)
	id := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestTrash, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision}))
	a.shutdown(context.Background())
	if result := a.ApproveExternalChange(id); result.State != "unavailable" {
		t.Fatalf("shutdown operation applied: %+v", result)
	}
}

func TestStorageSelectionInvalidatesPendingOperation(t *testing.T) {
	a, root, _, _, target := setupChangeApp(t)
	id := changeID(t, callExternalOrganization(t, root, readapi.OperationNotesRequestTrash, map[string]any{"noteId": target.ID, "expectedRevision": target.Revision}))
	other := a.CreateStorageSpace(notespace.CreateInput{Name: "Other"})
	if other.Error != nil || other.Space == nil {
		t.Fatalf("create space: %+v", other)
	}
	selected := a.SelectStorageSpace(notespace.SelectInput{ID: other.Space.ID})
	if selected.Error != nil || !selected.RestartRequired {
		t.Fatalf("select space: %+v", selected)
	}
	if result := a.ApproveExternalChange(id); result.State != "unavailable" {
		t.Fatalf("old space operation applied: %+v", result)
	}
	response := callExternalOrganization(t, root, readapi.OperationOperationsGet, map[string]any{"operationId": id})
	if response.Error == nil || response.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("old operation remained available: %+v", response)
	}
}

func structuredApplyInput(analysisID string, ids []string) organize.ApplyCandidatesInput {
	return organize.ApplyCandidatesInput{SessionID: analysisID, CandidateIDs: ids}
}

func mustJSON(t *testing.T, input any) string {
	t.Helper()
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
