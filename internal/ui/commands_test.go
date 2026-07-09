package ui

import (
	"testing"

	"lazylore/internal/lore"
)

func TestLoadStatusCmd_ReturnsStatusMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"status --scan": {ExitCode: 0, Stdout: "Repository abc\nOn branch main revision 0 -> 0\n"},
	}}
	msg := loadStatusCmd(fake)()
	sm, ok := msg.(statusMsg)
	if !ok {
		t.Fatalf("msg = %#v, want statusMsg", msg)
	}
	if sm.err != nil {
		t.Fatalf("unexpected error: %v", sm.err)
	}
	if sm.status.Repository != "abc" {
		t.Fatalf("Repository = %q, want abc", sm.status.Repository)
	}
}

func TestLoadBranchesCmd_ReturnsBranchesMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"branch list": {ExitCode: 0, Stdout: "Local branches:\n* main\nRemote branches:\n  main\n"},
	}}
	msg := loadBranchesCmd(fake)()
	bm, ok := msg.(branchesMsg)
	if !ok {
		t.Fatalf("msg = %#v, want branchesMsg", msg)
	}
	if len(bm.branches) != 2 {
		t.Fatalf("branches = %+v, want 2 entries", bm.branches)
	}
}

func TestLoadHistoryCmd_ReturnsHistoryMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"history --oneline 50": {ExitCode: 0, Stdout: "1 Initial revision\n"},
	}}
	msg := loadHistoryCmd(fake)()
	hm, ok := msg.(historyMsg)
	if !ok {
		t.Fatalf("msg = %#v, want historyMsg", msg)
	}
	if len(hm.revisions) != 1 {
		t.Fatalf("revisions = %+v, want 1 entry", hm.revisions)
	}
}

func TestLoadDiffCmd_ReturnsDiffMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"diff hello.txt": {ExitCode: 0, Stdout: "+++ hello.txt\n"},
	}}
	msg := loadDiffCmd(fake, "hello.txt")()
	dm, ok := msg.(diffMsg)
	if !ok {
		t.Fatalf("msg = %#v, want diffMsg", msg)
	}
	if dm.text != "+++ hello.txt\n" {
		t.Fatalf("text = %q", dm.text)
	}
}

func TestStageCmd_CallsRunnerAndReturnsActionDoneMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"stage hello.txt": {ExitCode: 0},
	}}
	msg := stageCmd(fake, "hello.txt")()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err != nil {
		t.Fatalf("unexpected error: %v", am.err)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("Calls = %+v, want exactly one call", fake.Calls)
	}
}

func TestCommitCmd_ReturnsErrorOnFailure(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"commit oops": {ExitCode: 1, Stderr: "nothing staged"},
	}}
	msg := commitCmd(fake, "oops")()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err == nil {
		t.Fatal("expected an error to be carried on actionDoneMsg")
	}
}
