package lore_test

import (
	"testing"

	"lazylore/internal/lore"
)

func TestCurrentUserID_ParsesAuthUserInfoEvent(t *testing.T) {
	// Captured shape from lore-revision/src/auth/userinfo.rs:
	// LoreAuthUserInfoEventData{id, name}, tagName "authUserInfo".
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json auth info": {ExitCode: 0, Stdout: `{"tagName":"authUserInfo","data":{"id":"user-123","name":"Solessfir"}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
	}}
	id, err := lore.CurrentUserID(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "user-123" {
		t.Fatalf("id = %q, want %q", id, "user-123")
	}
}

func TestCurrentUserID_ErrorsOnFailureComplete(t *testing.T) {
	// Not authenticated - callers must treat this as best-effort (same as
	// LockStatus), not a fatal error for the whole app.
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json auth info": {ExitCode: 1, Stdout: jsonCompleteFailure},
	}}
	_, err := lore.CurrentUserID(fake)
	if err == nil {
		t.Fatal("expected an error when auth info reports a failure")
	}
}
