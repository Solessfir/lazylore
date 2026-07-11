package lore

import "testing"

// Captured verbatim from `lore.exe --json branch list` against a real repo.
const jsonBranchListOutput = `{"tagName":"branchListBegin","data":{"location":"local"}}
{"tagName":"branchListEntry","data":{"location":"local","id":"e726318bbc3fd75ac8733a7e030cc35b","name":"main","category":"","latest":"b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195","stack":[],"creator":"","created":1783602421644,"isCurrent":false,"archived":false}}
{"tagName":"branchListEntry","data":{"location":"local","id":"019f46fd51147821aed79f28a839f591","name":"my-first-branch","category":"","latest":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","stack":[{"branch":"e726318bbc3fd75ac8733a7e030cc35b","revision":"b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195"}],"creator":"","created":1783602434325,"isCurrent":true,"archived":false}}
{"tagName":"branchListEnd","data":{"location":"local","count":2}}
{"tagName":"branchListBegin","data":{"location":"remote"}}
{"tagName":"branchListEnd","data":{"location":"remote","count":0}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

// Captured verbatim from `lore.exe --json stage nonexistent-file-xyz.txt`
// (a real failure - the process exited 255).
const jsonStageFailureOutput = `{"tagName":"fileStageBegin","data":{"pathCount":1}}
{"tagName":"fileStageProgress","data":{"count":{"directoryModifyCount":0,"directoryAddCount":0,"directoryDeleteCount":0,"directoryMoveCount":0,"fileModifyCount":0,"fileAddCount":0,"fileDeleteCount":0,"fileMoveCount":0,"totalCount":0}}}
{"tagName":"log","data":{"level":"error","category":0,"timestamp":1783770954340,"location":"lore_revision::relay","message":"Invalid path nonexistent-file-xyz.txt\n  at lore-revision\\src\\stage.rs:172:1"}}
{"tagName":"complete","data":{"status":-1,"error":{"errorCode":-1,"message":"Invalid path nonexistent-file-xyz.txt","traceLocations":[{"file":"lore-revision\\src\\stage.rs","line":172,"column":1,"context":""}]}}}
`

func TestParseEvents_SplitsOneEventPerLine(t *testing.T) {
	events, err := parseEvents(jsonBranchListOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 7 {
		t.Fatalf("events = %d, want 7", len(events))
	}
	if events[0].TagName != "branchListBegin" {
		t.Fatalf("events[0].TagName = %q, want %q", events[0].TagName, "branchListBegin")
	}
	if events[1].TagName != "branchListEntry" {
		t.Fatalf("events[1].TagName = %q, want %q", events[1].TagName, "branchListEntry")
	}
}

func TestParseEvents_SkipsBlankLines(t *testing.T) {
	events, err := parseEvents("\n" + jsonBranchListOutput + "\n\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 7 {
		t.Fatalf("events = %d, want 7 (blank lines skipped)", len(events))
	}
}

func TestParseEvents_ErrorsOnMalformedLine(t *testing.T) {
	_, err := parseEvents(`{"tagName":"complete","data":{`)
	if err == nil {
		t.Fatal("expected an error for a malformed JSON line")
	}
}

func TestFindComplete_SuccessStatus(t *testing.T) {
	events, err := parseEvents(jsonBranchListOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := findComplete(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Status != 0 {
		t.Fatalf("Status = %d, want 0", data.Status)
	}
}

func TestFindComplete_FailureStatusAndMessage(t *testing.T) {
	events, err := parseEvents(jsonStageFailureOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := findComplete(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Status == 0 {
		t.Fatal("Status = 0, want nonzero (this was a real failure)")
	}
	if data.Error.Message != "Invalid path nonexistent-file-xyz.txt" {
		t.Fatalf("Error.Message = %q, want the real error text", data.Error.Message)
	}
}

func TestFindComplete_ErrorsWhenNoCompleteEvent(t *testing.T) {
	events, err := parseEvents(`{"tagName":"branchListBegin","data":{"location":"local"}}` + "\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = findComplete(events)
	if err == nil {
		t.Fatal("expected an error when no complete event is present")
	}
}
