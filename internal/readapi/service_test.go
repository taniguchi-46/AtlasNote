package readapi

import (
	"context"
	"encoding/json"
	"testing"
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
