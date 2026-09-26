package readapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"atlasnote/internal/organize"
)

func TestExecuteRejectsPermissionAndScopeBeforeReading(t *testing.T) {
	service := New(nil, nil, "space-a")
	params, _ := json.Marshal(NoteGetInput{NoteID: "0123456789abcdef0123456789abcdef"})
	request := Request{
		APIVersion: APIVersion,
		RequestID:  "request-1",
		ClientID:   "client-a",
		Scope:      Scope{StorageSpaceID: "space-a"},
		Operation:  OperationNotesGet,
		Params:     params,
	}
	principal := Principal{
		ClientID:       "client-a",
		StorageSpaceID: "space-a",
		Permissions:    map[string]bool{PermissionMetadata: true},
	}
	response := service.Execute(context.Background(), principal, request)
	if response.Error == nil || response.Error.Code != "PERMISSION_DENIED" || response.Status != StatusRejected {
		t.Fatalf("permission response = %+v", response)
	}

	principal.Permissions[PermissionContent] = true
	request.Scope.StorageSpaceID = "space-b"
	response = service.Execute(context.Background(), principal, request)
	if response.Error == nil || response.Error.Code != "SCOPE_MISMATCH" || response.Status != StatusRejected {
		t.Fatalf("scope response = %+v", response)
	}
}

func TestOrganizationCandidatePageStaysBelowIPCResponseLimit(t *testing.T) {
	candidates := make([]OrganizationCandidate, 100)
	for index := range candidates {
		candidates[index] = OrganizationCandidate{
			ID: strings.Repeat("a", 32), Kind: "title", NoteID: strings.Repeat("b", 32),
			Reason: strings.Repeat("候補理由", 12_000), Before: map[string]any{"title": "before"},
		}
	}
	fingerprint := organizationCursorFingerprint(strings.Repeat("c", 32), "")
	page, cursor := pageOrganizationCandidates(candidates, 0, 100, OperationOrganizeGetCandidates, fingerprint)
	if len(page) == 0 || len(page) >= len(candidates) || cursor == "" {
		t.Fatalf("size-bounded page = %d candidates, cursor=%q", len(page), cursor)
	}
	response := success("size-test", OrganizeCandidatesData{AnalysisID: strings.Repeat("c", 32), Candidates: page, NextCursor: cursor})
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= 3<<20 {
		t.Fatalf("organization response size = %d bytes", len(encoded))
	}
	offset, err := decodeCursor(cursor, OperationOrganizeGetCandidates, fingerprint)
	if err != nil || offset != len(page) {
		t.Fatalf("page cursor offset=%d err=%v", offset, err)
	}
	if _, err := decodeCursor(cursor, OperationOrganizeGetCandidates, organizationCursorFingerprint(strings.Repeat("d", 32), "")); err == nil {
		t.Fatal("analysis cursor was accepted for another analysis")
	}
}

func TestOversizedOrganizationCandidateIsNotPublished(t *testing.T) {
	snapshot := organize.AnalysisSnapshot{Candidates: []organize.Candidate{{
		ID: strings.Repeat("a", 32), Kind: organize.KindTitle, Reason: "oversized",
		Before: map[string]any{"title": "before"}, Proposed: map[string]any{"title": strings.Repeat("\\", maxOrganizationCandidateBytes)},
	}}}
	if candidates := organizationCandidatesForPrincipal(Principal{}, snapshot); len(candidates) != 0 {
		t.Fatalf("oversized candidates published = %d", len(candidates))
	}
}
