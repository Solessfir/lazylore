package lore_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/solessfir/lazylore/internal/lore"
)

// jsonCompleteSuccess/jsonCompleteFailure are the minimal real shape of a
// --json command's terminal event (see events_test.go's captured samples)
// for action wrappers that don't parse anything beyond success/failure.
const jsonCompleteSuccess = `{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}` + "\n"

const jsonCompleteFailure = `{"tagName":"complete","data":{"status":-1,"error":{"errorCode":-1,"message":"not a lore repository","traceLocations":[]}}}` + "\n"

type recordingSuccessRunner struct {
	Calls [][]string
}

func (r *recordingSuccessRunner) Run(args ...string) (lore.Result, error) {
	r.Calls = append(r.Calls, append([]string(nil), args...))
	return lore.Result{Stdout: jsonCompleteSuccess}, nil
}

type streamingFakeRunner struct {
	lore.FakeRunner
}

func (r *streamingFakeRunner) RunStream(onLine func(string), args ...string) (lore.Result, error) {
	result, err := r.Run(args...)
	if onLine != nil {
		for _, line := range strings.Split(result.Stdout, "\n") {
			onLine(line)
		}
	}
	return result, err
}

func TestCommands_ReportProcessErrorsWithoutCompleteEvents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result lore.Result
		want   string
	}{
		{"missing output", lore.Result{ExitCode: 2, Stderr: "error: unexpected argument '-n' found\n"}, "error: unexpected argument '-n' found"},
		{"invalid JSON", lore.Result{ExitCode: 2, Stdout: "invalid JSON", Stderr: "error: invalid command\n"}, "error: invalid command"},
		{"missing completion", lore.Result{ExitCode: 2, Stdout: "{\"tagName\":\"fileStageBegin\",\"data\":{}}\n", Stderr: "error: command terminated\n"}, "error: command terminated"},
		{"no stderr", lore.Result{ExitCode: 2, Stdout: "invalid JSON"}, "parsing --json event line"},
		{"successful process with invalid JSON", lore.Result{Stdout: "invalid JSON", Stderr: "stderr warning"}, "parsing --json event line"},
		{"complete failure takes precedence", lore.Result{ExitCode: 1, Stdout: jsonCompleteFailure, Stderr: "stderr warning"}, "not a lore repository"},
		{"complete success takes precedence", lore.Result{ExitCode: 1, Stdout: jsonCompleteSuccess, Stderr: "stderr warning"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, streaming := range []bool{false, true} {
				runner := &streamingFakeRunner{FakeRunner: lore.FakeRunner{Results: map[string]lore.Result{
					"--json stage -- -notes.txt": tc.result,
					"--json push":                tc.result,
				}}}
				var err error
				if streaming {
					_, err = lore.PushStream(runner, nil)
				} else {
					_, err = lore.Stage(runner, "-notes.txt")
				}
				if tc.want == "" {
					if err != nil {
						t.Fatalf("streaming=%v: unexpected error: %v", streaming, err)
					}
				} else if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("streaming=%v: error = %v, want %q", streaming, err, tc.want)
				}
			}
		})
	}
}

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
		"--json history 10 --branch=dev": {ExitCode: 0, Stdout: historyTwoRevisionsOutput},
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
		"--json diff -- hello.txt": {ExitCode: 0, Stdout: jsonDiffOutput},
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
		"--json stage -- a.txt b.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.Stage(fake, "a.txt", "b.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFileCommands_PreserveLiteralPaths(t *testing.T) {
	paths := []string{"-notes.txt", "--purge", "move", "merge", "space 'quoted'.txt"}
	for _, tc := range []struct {
		name string
		run  func(lore.Runner, ...string) (lore.Result, error)
	}{
		{"stage", lore.Stage},
		{"unstage", lore.Unstage},
		{"reset", lore.Reset},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &recordingSuccessRunner{}
			if _, err := tc.run(runner, paths...); err != nil {
				t.Fatal(err)
			}
			want := [][]string{append([]string{"--json", tc.name, "--"}, paths...)}
			if !reflect.DeepEqual(runner.Calls, want) {
				t.Fatalf("Calls = %#v, want %#v", runner.Calls, want)
			}
		})
	}

	runner := &recordingSuccessRunner{}
	if _, err := lore.Diff(runner, paths[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := lore.DiscardChanges(runner, paths[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := lore.DiscardAllChanges(runner, paths); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"--json", "diff", "--", paths[0]},
		{"--json", "unstage", "--", paths[0]},
		{"--json", "reset", "--", paths[0]},
		append([]string{"--json", "unstage", "--"}, paths...),
		append([]string{"--json", "reset", "--purge", "--"}, paths...),
	}
	if !reflect.DeepEqual(runner.Calls, want) {
		t.Fatalf("Calls = %#v, want %#v", runner.Calls, want)
	}
}

func TestCommands_PreserveLiteralMessagesAndBranchNames(t *testing.T) {
	runner := &recordingSuccessRunner{}
	message := "--message 'quoted' \"double quoted\"\nsecond line"
	branch := "--branch"
	for _, tc := range []struct {
		name  string
		run   func(lore.Runner, string) (lore.Result, error)
		value string
	}{
		{"commit", lore.Commit, message},
		{"create", lore.CreateBranch, branch},
		{"switch", lore.SwitchBranch, branch},
		{"merge", lore.MergeBranch, branch},
	} {
		if _, err := tc.run(runner, tc.value); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if _, err := lore.HistoryForBranch(runner, branch, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := lore.RevertRevision(runner, "abc123", message); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"--json", "commit", "--", message},
		{"--json", "branch", "create", "--", branch},
		{"--json", "branch", "switch", "--", branch},
		{"--json", "branch", "merge", "--", branch},
		{"--json", "history", "10", "--branch=" + branch},
		{"--json", "revision", "revert", "abc123", "--message=" + message},
	}
	if !reflect.DeepEqual(runner.Calls, want) {
		t.Fatalf("Calls = %#v, want %#v", runner.Calls, want)
	}
}

func TestCommit_PassesMessageAsSingleArg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json commit -- fix the thing": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.Commit(fake, "fix the thing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSwitchBranch_ErrorsOnRunnerFailure(t *testing.T) {
	fake := &lore.FakeRunner{Errs: map[string]error{
		"--json branch switch -- main": errors.New("boom"),
	}}
	_, err := lore.SwitchBranch(fake, "main")
	if err == nil {
		t.Fatal("expected an error when the Runner itself fails")
	}
}

func TestCreateBranch_BuildsArgs(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json branch create -- my-first-branch": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.CreateBranch(fake, "my-first-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscardChanges_UnstagesThenResetsInOrder(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset -- a.txt":   {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.DiscardChanges(fake, "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.Calls) != 2 {
		t.Fatalf("Calls = %+v, want 2 calls", fake.Calls)
	}
	if fake.Calls[0][0] != "--json" || fake.Calls[0][1] != "unstage" {
		t.Fatalf("Calls[0] = %+v, want --json unstage -- first", fake.Calls[0])
	}
	if fake.Calls[1][0] != "--json" || fake.Calls[1][1] != "reset" {
		t.Fatalf("Calls[1] = %+v, want --json reset second", fake.Calls[1])
	}
}

func TestDiscardAllChanges_UnstagesThenPurgeResetsAllPaths(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- a.txt b.txt":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge -- a.txt b.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
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

func TestDiscardAllChanges_BatchesLargePathLists(t *testing.T) {
	runner := &recordingSuccessRunner{}
	paths := make([]string, 300)
	for i := range paths {
		paths[i] = fmt.Sprintf("Content/%03d-%s.uasset", i, strings.Repeat("x", 180))
	}

	_, err := lore.DiscardAllChanges(runner, paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.Calls) <= 2 {
		t.Fatalf("Calls = %d, want more than two calls for a large path list", len(runner.Calls))
	}

	resetStarted := false
	for _, call := range runner.Calls {
		if len(strings.Join(call, " ")) > 17*1024 {
			t.Fatalf("command is still too large: %d bytes", len(strings.Join(call, " ")))
		}
		if len(call) > 1 && call[1] == "reset" {
			resetStarted = true
		}
		if resetStarted && len(call) > 1 && call[1] == "unstage" {
			t.Fatalf("unstage call appeared after reset started: %+v", runner.Calls)
		}
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

func TestRevertRevision_BuildsArgsWithoutMessage(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json revision revert abc123": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.RevertRevision(fake, "abc123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRevertRevision_BuildsArgsWithMessageFlag(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		`--json revision revert abc123 --message=Revert "oops"`: {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.RevertRevision(fake, "abc123", `Revert "oops"`)
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
	_, err := lore.RevertRevision(fake, "abc123", "")
	if err == nil {
		t.Fatal("expected an error when the revert reports a failure")
	}
}

func TestDiscardChanges_ShortCircuitsWhenUnstageFails(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- a.txt": {ExitCode: 1, Stdout: jsonCompleteFailure},
		"--json reset -- a.txt":   {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	_, err := lore.DiscardChanges(fake, "a.txt")
	if err == nil {
		t.Fatal("expected an error when unstage fails")
	}
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "unstage" {
		t.Fatalf("Calls = %+v, want only the unstage call (reset must not run)", fake.Calls)
	}
}
