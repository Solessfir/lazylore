package lore_test

import (
	"fmt"
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

func TestCurrentUserID_UsesAnonymousOwnerWithoutAuthEndpoint(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json auth info --identity=native-test": {ExitCode: 9, Stdout: `{"tagName":"complete","data":{"status":9,"error":{"errorCode":9,"message":"Operation not supported: authentication requires a configured auth endpoint","traceLocations":[]}}}`},
		"--json repository config get identity":   {Stdout: `{"tagName":"repositoryConfigGet","data":{"key":"identity","value":"native-test"}}` + "\n" + jsonCompleteSuccess},
	}}
	id, err := lore.CurrentUserID(fake)
	if err != nil || id != "<unknown>" {
		t.Fatalf("id = %q, error = %v, want anonymous server lock owner", id, err)
	}
	if len(fake.Calls) != 2 {
		t.Fatalf("Calls = %#v, want configured account lookup then auth info", fake.Calls)
	}
}

func TestCurrentUserID_RequiresSpecificMissingAuthEndpointCompletion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		code    int
		message string
	}{
		{"other status", 1, 9, "Operation not supported: authentication requires a configured auth endpoint"},
		{"other code", 9, 1, "Operation not supported: authentication requires a configured auth endpoint"},
		{"other unsupported operation", 9, 9, "Operation not supported: requested operation is unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &lore.FakeRunner{Results: map[string]lore.Result{
				"--json auth info --identity=native-test": {ExitCode: tc.status, Stdout: fmt.Sprintf(`{"tagName":"complete","data":{"status":%d,"error":{"errorCode":%d,"message":%q,"traceLocations":[]}}}`, tc.status, tc.code, tc.message)},
				"--json repository config get identity":   {Stdout: `{"tagName":"repositoryConfigGet","data":{"key":"identity","value":"native-test"}}` + "\n" + jsonCompleteSuccess},
			}}
			id, err := lore.CurrentUserID(fake)
			if err != nil || id != "native-test" {
				t.Fatalf("id = %q, error = %v, want configured identity fallback", id, err)
			}
		})
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
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json auth info --identity=Solessfir": {ExitCode: 1, Stdout: jsonCompleteFailure},
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

func TestCurrentUserID_SelectsConfiguredAccountAmongCachedAccounts(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json repository config get identity": {Stdout: `{"tagName":"repositoryConfigGet","data":{"key":"identity","value":"account-B"}}` + "\n" + jsonCompleteSuccess},
		"--json auth info":                      {Stdout: `{"tagName":"authUserInfo","data":{"id":"server-id-A","name":"Account A"}}` + "\n" + jsonCompleteSuccess},
		"--json auth info --identity=account-B": {Stdout: `{"tagName":"authUserInfo","data":{"id":"server-id-B","name":"Account B"}}` + "\n" + jsonCompleteSuccess},
	}}
	id, err := lore.CurrentUserID(fake)
	if err != nil || id != "server-id-B" {
		t.Fatalf("configured account's server identity = %q, error = %v", id, err)
	}
	if len(fake.Calls) != 2 || len(fake.Calls[1]) != 4 || fake.Calls[1][3] != "--identity=account-B" {
		t.Fatalf("identity query selected another cached account: %v", fake.Calls)
	}
}
