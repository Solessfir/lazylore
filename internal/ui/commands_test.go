package ui

import (
	"testing"

	"lazylore/internal/lore"
)

// jsonCompleteSuccess is the minimal real shape of a --json command's
// terminal event on success - shared by every test in this package that
// exercises a Runner call without needing to assert on the response body.
const jsonCompleteSuccess = `{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

func TestLoadStatusCmd_ReturnsStatusMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json status --scan": {ExitCode: 0, Stdout: `{"tagName":"repositoryStatusRevision","data":{"repository":"abc","branchName":"main"}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
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
		"--json branch list": {ExitCode: 0, Stdout: `{"tagName":"branchListEntry","data":{"location":"local","name":"main","isCurrent":true}}
{"tagName":"branchListEntry","data":{"location":"remote","name":"main","isCurrent":false}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
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
		"--json history 50": {ExitCode: 0, Stdout: `{"tagName":"revisionHistoryEntry","data":{"revision":"abc","revisionNumber":1,"parent":["0","0"]}}
{"tagName":"metadata","data":{"key":"message","value":{"tagName":"string","data":"Initial revision"}}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
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
		"--json diff hello.txt": {ExitCode: 0, Stdout: `{"tagName":"fileDiff","data":{"path":"hello.txt","patch":"+++ hello.txt\n","action":"keep"}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
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
		"--json stage hello.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
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
		"--json commit oops": {ExitCode: 1, Stdout: "{\"tagName\":\"complete\",\"data\":{\"status\":-1,\"error\":{\"errorCode\":-1,\"message\":\"nothing staged\",\"traceLocations\":[]}}}\n"},
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
