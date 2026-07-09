package lore_test

import (
	"errors"
	"testing"

	"lazylore/internal/lore"
)

func TestGetStatus_ParsesSuccessfulRun(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"status --scan": {ExitCode: 0, Stdout: statusCleanOutput},
	}}
	s, err := lore.GetStatus(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Repository == "" {
		t.Fatal("expected a parsed Status with a Repository set")
	}
	if len(fake.Calls) != 1 || fake.Calls[0][0] != "status" {
		t.Fatalf("Calls = %+v, want a single status call", fake.Calls)
	}
}

func TestGetStatus_ErrorsOnNonZeroExit(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"status --scan": {ExitCode: 1, Stderr: "not a lore repository"},
	}}
	_, err := lore.GetStatus(fake)
	if err == nil {
		t.Fatal("expected an error for a nonzero exit code")
	}
}

func TestBranchList_ParsesSuccessfulRun(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"branch list": {ExitCode: 0, Stdout: branchListSingleOutput},
	}}
	branches, err := lore.BranchList(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(branches) != 2 {
		t.Fatalf("branches = %+v, want 2 entries", branches)
	}
}

func TestHistoryOneline_BuildsLengthArg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"history --oneline 10": {ExitCode: 0, Stdout: historyOnelineOutput},
	}}
	revisions, err := lore.HistoryOneline(fake, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revisions = %+v, want 2 entries", revisions)
	}
}

func TestDiff_ReturnsRawOutput(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"diff hello.txt": {ExitCode: 0, Stdout: "--- hello.txt@1\n+++ hello.txt\n"},
	}}
	text, err := lore.Diff(fake, "hello.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "--- hello.txt@1\n+++ hello.txt\n" {
		t.Fatalf("text = %q", text)
	}
}

func TestStage_BuildsArgsForMultiplePaths(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"stage a.txt b.txt": {ExitCode: 0, Stdout: "Staged repository state ...\n"},
	}}
	_, err := lore.Stage(fake, "a.txt", "b.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommit_PassesMessageAsSingleArg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"commit fix the thing": {ExitCode: 0, Stdout: "Commit succeeded\n"},
	}}
	_, err := lore.Commit(fake, "fix the thing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSwitchBranch_ErrorsOnRunnerFailure(t *testing.T) {
	fake := &lore.FakeRunner{Errs: map[string]error{
		"branch switch main": errors.New("boom"),
	}}
	_, err := lore.SwitchBranch(fake, "main")
	if err == nil {
		t.Fatal("expected an error when the Runner itself fails")
	}
}

func TestCreateBranch_BuildsArgs(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"branch create my-first-branch": {ExitCode: 0, Stdout: "Created branch my-first-branch ...\n"},
	}}
	_, err := lore.CreateBranch(fake, "my-first-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscardChanges_UnstagesThenResetsInOrder(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"unstage a.txt": {ExitCode: 0},
		"reset a.txt":   {ExitCode: 0},
	}}
	_, err := lore.DiscardChanges(fake, "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.Calls) != 2 {
		t.Fatalf("Calls = %+v, want 2 calls", fake.Calls)
	}
	if fake.Calls[0][0] != "unstage" {
		t.Fatalf("Calls[0] = %+v, want unstage first", fake.Calls[0])
	}
	if fake.Calls[1][0] != "reset" {
		t.Fatalf("Calls[1] = %+v, want reset second", fake.Calls[1])
	}
}

func TestDiscardChanges_ShortCircuitsWhenUnstageFails(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"unstage a.txt": {ExitCode: 1, Stderr: "boom"},
		"reset a.txt":   {ExitCode: 0},
	}}
	_, err := lore.DiscardChanges(fake, "a.txt")
	if err == nil {
		t.Fatal("expected an error when unstage fails")
	}
	if len(fake.Calls) != 1 || fake.Calls[0][0] != "unstage" {
		t.Fatalf("Calls = %+v, want only the unstage call (reset must not run)", fake.Calls)
	}
}
