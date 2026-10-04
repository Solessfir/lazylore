package lore_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"lazylore/internal/lore"
)

func TestLockStatus_ParsesLockedPaths(t *testing.T) {
	// Captured shape from lore-revision/src/lock/file/status.rs:
	// LoreLockFileStatusBeginEventData{count} + LoreLockFileStatusEventData
	// per locked path (unlocked paths simply don't appear).
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock status -- a.txt b.txt": {ExitCode: 0, Stdout: `{"tagName":"lockFileStatusBegin","data":{"count":1}}
{"tagName":"lockFileStatus","data":{"path":"a.txt","owner":"user-123","lockedAt":1750000000000}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
	}}
	locks, err := lore.LockStatus(fake, "a.txt", "b.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locks) != 1 {
		t.Fatalf("locks = %+v, want 1 entry (only a.txt is locked)", locks)
	}
	if locks[0].Path != "a.txt" || locks[0].Owner != "user-123" || locks[0].LockedAt != 1750000000000 {
		t.Fatalf("locks[0] = %+v, unexpected values", locks[0])
	}
}

func TestLockStatus_NoOpOnEmptyPaths(t *testing.T) {
	fake := &lore.FakeRunner{}
	locks, err := lore.LockStatus(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if locks != nil {
		t.Fatalf("locks = %+v, want nil for an empty path list", locks)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("Calls = %+v, want no runner calls for an empty path list", fake.Calls)
	}
}

func TestLockStatus_BatchesLargePathLists(t *testing.T) {
	runner := &recordingSuccessRunner{}
	paths := make([]string, 300)
	for i := range paths {
		paths[i] = fmt.Sprintf("Content/%03d-%s.uasset", i, strings.Repeat("x", 180))
	}

	locks, err := lore.LockStatus(runner, paths...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locks) != 0 {
		t.Fatalf("locks = %+v, want none from the success-only runner", locks)
	}
	if len(runner.Calls) <= 1 {
		t.Fatalf("Calls = %d, want multiple calls for a large path list", len(runner.Calls))
	}
	for _, call := range runner.Calls {
		if len(strings.Join(call, " ")) > 17*1024 {
			t.Fatalf("command is still too large: %d bytes", len(strings.Join(call, " ")))
		}
	}
}

func TestLockAcquire_BuildsArgs(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock acquire -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.LockAcquire(fake, "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLockRelease_BuildsArgs(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock release -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.LockRelease(fake, "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLockCommands_PreserveLiteralPaths(t *testing.T) {
	runner := &recordingSuccessRunner{}
	path := "--force 'quoted'.txt"
	if _, err := lore.LockStatus(runner, path); err != nil {
		t.Fatal(err)
	}
	for _, run := range []func(lore.Runner, string) (lore.Result, error){lore.LockAcquire, lore.LockRelease, lore.LockReleaseForce} {
		if _, err := run(runner, path); err != nil {
			t.Fatal(err)
		}
	}
	want := [][]string{
		{"--json", "lock", "status", "--", path},
		{"--json", "lock", "acquire", "--", path},
		{"--json", "lock", "release", "--", path},
		{"--json", "lock", "release", "--force", "--", path},
	}
	if !reflect.DeepEqual(runner.Calls, want) {
		t.Fatalf("Calls = %#v, want %#v", runner.Calls, want)
	}
}
