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

func TestCurrentUserID_ErrorsWhenBothAuthAndConfigIdentityFail(t *testing.T) {
	// Not authenticated and no repo identity configured either - callers
	// must treat this as best-effort (same as LockStatus), not a fatal
	// error for the whole app.
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json auth info":                      {ExitCode: 1, Stdout: jsonCompleteFailure},
		"--json repository config get identity": {ExitCode: 1, Stdout: jsonCompleteFailure},
	}}
	_, err := lore.CurrentUserID(fake)
	if err == nil {
		t.Fatal("expected an error when both auth info and config identity fail")
	}
}

func TestCurrentUserID_FallsBackToConfigIdentityWhenAuthInfoFails(t *testing.T) {
	// On a no-auth server (see project_lazylore_lock_owner_todo memory),
	// `lore auth info` fails outright - lock acquire itself falls back to
	// the repo's configured identity as owner in that case, so
	// CurrentUserID must resolve the same value or every one of your own
	// locks would misreport as someone else's.
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json auth info": {ExitCode: 1, Stdout: jsonCompleteFailure},
		"--json repository config get identity": {ExitCode: 0, Stdout: `{"tagName":"repositoryConfigGet","data":{"key":"identity","value":"Solessfir"}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
	}}
	id, err := lore.CurrentUserID(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "Solessfir" {
		t.Fatalf("id = %q, want %q", id, "Solessfir")
	}
}
