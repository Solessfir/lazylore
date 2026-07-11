package lore_test

import (
	"errors"
	"strings"
	"testing"

	"lazylore/internal/lore"
)

// jsonCompleteSuccess/jsonCompleteFailure are the minimal real shape of a
// --json command's terminal event (see events_test.go's captured samples)
// for action wrappers that don't parse anything beyond success/failure.
const jsonCompleteSuccess = `{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}` + "\n"

const jsonCompleteFailure = `{"tagName":"complete","data":{"status":-1,"error":{"errorCode":-1,"message":"not a lore repository","traceLocations":[]}}}` + "\n"

func TestGetStatus_ParsesSuccessfulRun(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json status --scan": {ExitCode: 0, Stdout: statusCleanOutput},
	}}
	s, err := lore.GetStatus(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Repository == "" {
		t.Fatal("expected a parsed Status with a Repository set")
	}
	if len(fake.Calls) != 1 || fake.Calls[0][0] != "--json" || fake.Calls[0][1] != "status" {
		t.Fatalf("Calls = %+v, want a single --json status call", fake.Calls)
	}
}

func TestGetStatus_ErrorsOnFailureComplete(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json status --scan": {ExitCode: 1, Stdout: jsonCompleteFailure},
	}}
	_, err := lore.GetStatus(fake)
	if err == nil {
		t.Fatal("expected an error for a failed complete event")
	}
}

func TestBranchList_ParsesSuccessfulRun(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json branch list": {ExitCode: 0, Stdout: branchListSingleOutput},
	}}
	branches, err := lore.BranchList(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(branches) != 2 {
		t.Fatalf("branches = %+v, want 2 entries", branches)
	}
}

func TestHistory_BuildsLengthArg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json history 10": {ExitCode: 0, Stdout: historyTwoRevisionsOutput},
	}}
	revisions, err := lore.History(fake, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revisions = %+v, want 2 entries", revisions)
	}
}

func TestHistoryForBranch_BuildsArgs(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json history 10 --branch dev": {ExitCode: 0, Stdout: historyTwoRevisionsOutput},
	}}
	revisions, err := lore.HistoryForBranch(fake, "dev", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revisions = %+v, want 2 entries", revisions)
	}
}

func TestDiffRevision_ConcatenatesEveryFileDiffPatch(t *testing.T) {
	// Two files changed between source and target - both fileDiff events
	// must be concatenated into the combined "patch" (lore's equivalent of
	// `git show <commit>`, which lists every changed file in one output).
	out := `{"tagName":"fileDiff","data":{"path":"a.txt","patch":"--- a.txt@1\n+++ a.txt@2\n@@ -1 +1 @@\n-old\n+new\n","action":"keep"}}
{"tagName":"fileDiff","data":{"path":"b.txt","patch":"--- /dev/null\n+++ b.txt\n@@ -0,0 +1 @@\n+added\n","action":"add"}}
` + jsonCompleteSuccess
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json diff --source parenthash --target commithash": {ExitCode: 0, Stdout: out},
	}}
	patch, err := lore.DiffRevision(fake, "parenthash", "commithash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(patch, "a.txt") || !strings.Contains(patch, "b.txt") {
		t.Fatalf("patch missing one of the two files: %q", patch)
	}
}

func TestDiff_ReturnsPatchFromFileDiffEvent(t *testing.T) {
	// Captured verbatim from `lore.exe --json diff hello.txt`.
	const jsonDiffOutput = `{"tagName":"fileDiff","data":{"path":"hello.txt","patch":"--- hello.txt@2\n+++ hello.txt\n@@ -1,2 +1,3 @@\n Hello, Lore\n Second line added\n+more text\n","action":"keep"}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json diff hello.txt": {ExitCode: 0, Stdout: jsonDiffOutput},
	}}
	text, err := lore.Diff(fake, "hello.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "--- hello.txt@2\n+++ hello.txt\n@@ -1,2 +1,3 @@\n Hello, Lore\n Second line added\n+more text\n"
	if text != want {
		t.Fatalf("text = %q, want %q", text, want)
	}
}

func TestStage_BuildsArgsForMultiplePaths(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage a.txt b.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.Stage(fake, "a.txt", "b.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommit_PassesMessageAsSingleArg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json commit fix the thing": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.Commit(fake, "fix the thing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSwitchBranch_ErrorsOnRunnerFailure(t *testing.T) {
	fake := &lore.FakeRunner{Errs: map[string]error{
		"--json branch switch main": errors.New("boom"),
	}}
	_, err := lore.SwitchBranch(fake, "main")
	if err == nil {
		t.Fatal("expected an error when the Runner itself fails")
	}
}

func TestCreateBranch_BuildsArgs(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json branch create my-first-branch": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.CreateBranch(fake, "my-first-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscardChanges_UnstagesThenResetsInOrder(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset a.txt":   {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.DiscardChanges(fake, "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.Calls) != 2 {
		t.Fatalf("Calls = %+v, want 2 calls", fake.Calls)
	}
	if fake.Calls[0][0] != "--json" || fake.Calls[0][1] != "unstage" {
		t.Fatalf("Calls[0] = %+v, want --json unstage first", fake.Calls[0])
	}
	if fake.Calls[1][0] != "--json" || fake.Calls[1][1] != "reset" {
		t.Fatalf("Calls[1] = %+v, want --json reset second", fake.Calls[1])
	}
}

func TestDiscardAllChanges_UnstagesThenPurgeResetsAllPaths(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage a.txt b.txt":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge a.txt b.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.DiscardAllChanges(fake, []string{"a.txt", "b.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.Calls) != 2 {
		t.Fatalf("Calls = %+v, want 2 calls", fake.Calls)
	}
	if fake.Calls[1][1] != "reset" || fake.Calls[1][2] != "--purge" {
		t.Fatalf("Calls[1] = %+v, want reset --purge", fake.Calls[1])
	}
}

func TestDiscardAllChanges_NoOpOnEmptyPaths(t *testing.T) {
	fake := &lore.FakeRunner{}
	_, err := lore.DiscardAllChanges(fake, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("Calls = %+v, want no runner calls for an empty path list", fake.Calls)
	}
}

func TestResetBranchTo_BuildsArgs(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json branch reset abc123": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.ResetBranchTo(fake, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncTo_BuildsArgs(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json sync abc123": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.SyncTo(fake, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRevertRevision_BuildsArgs(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json revision revert abc123": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.RevertRevision(fake, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRevertRevision_SurfacesConflictAsError(t *testing.T) {
	// lazylore has no UI for lore's resolve/abort revert sub-flow, so a
	// conflicting revert must surface as a plain error, not be silently
	// left in an unresolved state.
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json revision revert abc123": {ExitCode: 1, Stdout: `{"tagName":"complete","data":{"status":-1,"error":{"errorCode":-1,"message":"revert conflict","traceLocations":[]}}}` + "\n"},
	}}
	_, err := lore.RevertRevision(fake, "abc123")
	if err == nil {
		t.Fatal("expected an error when the revert reports a failure")
	}
}

func TestDiscardChanges_ShortCircuitsWhenUnstageFails(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage a.txt": {ExitCode: 1, Stdout: jsonCompleteFailure},
		"--json reset a.txt":   {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.DiscardChanges(fake, "a.txt")
	if err == nil {
		t.Fatal("expected an error when unstage fails")
	}
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "unstage" {
		t.Fatalf("Calls = %+v, want only the unstage call (reset must not run)", fake.Calls)
	}
}
