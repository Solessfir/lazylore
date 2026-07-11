package lore_test

import (
	"testing"

	"lazylore/internal/lore"
)

// Captured verbatim from `lore.exe --json branch list` on a fresh repo
// with a single local branch and its remote counterpart.
const branchListSingleOutput = `{"tagName":"branchListBegin","data":{"location":"local"}}
{"tagName":"branchListEntry","data":{"location":"local","id":"e726318bbc3fd75ac8733a7e030cc35b","name":"main","category":"","latest":"0000000000000000000000000000000000000000000000000000000000000000","stack":[],"creator":"","created":1783771246964,"isCurrent":true,"archived":false}}
{"tagName":"branchListEnd","data":{"location":"local","count":1}}
{"tagName":"branchListBegin","data":{"location":"remote"}}
{"tagName":"branchListEntry","data":{"location":"remote","id":"e726318bbc3fd75ac8733a7e030cc35b","name":"main","category":"","latest":"0000000000000000000000000000000000000000000000000000000000000000","stack":[],"creator":"<unknown>","created":1783771246,"isCurrent":false,"archived":false}}
{"tagName":"branchListEnd","data":{"location":"remote","count":1}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

// Captured verbatim from `lore.exe --json branch list` after creating and
// switching to a second branch (local-only - the remote hadn't been pushed to).
const branchListMultiOutput = `{"tagName":"branchListBegin","data":{"location":"local"}}
{"tagName":"branchListEntry","data":{"location":"local","id":"e726318bbc3fd75ac8733a7e030cc35b","name":"main","category":"","latest":"b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195","stack":[],"creator":"","created":1783602421644,"isCurrent":false,"archived":false}}
{"tagName":"branchListEntry","data":{"location":"local","id":"019f46fd51147821aed79f28a839f591","name":"my-first-branch","category":"","latest":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","stack":[{"branch":"e726318bbc3fd75ac8733a7e030cc35b","revision":"b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195"}],"creator":"","created":1783602434325,"isCurrent":true,"archived":false}}
{"tagName":"branchListEnd","data":{"location":"local","count":2}}
{"tagName":"branchListBegin","data":{"location":"remote"}}
{"tagName":"branchListEnd","data":{"location":"remote","count":0}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

func TestParseBranchList_Single(t *testing.T) {
	branches, err := lore.ParseBranchList(branchListSingleOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []lore.Branch{
		{Name: "main", Current: true, Remote: false, Latest: "0000000000000000000000000000000000000000000000000000000000000000", Created: 1783771246964},
		{Name: "main", Current: false, Remote: true, Latest: "0000000000000000000000000000000000000000000000000000000000000000", Created: 1783771246},
	}
	if len(branches) != len(want) {
		t.Fatalf("branches = %+v, want %+v", branches, want)
	}
	for i := range want {
		if branches[i] != want[i] {
			t.Fatalf("branches[%d] = %+v, want %+v", i, branches[i], want[i])
		}
	}
}

func TestParseBranchList_Multi(t *testing.T) {
	branches, err := lore.ParseBranchList(branchListMultiOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []lore.Branch{
		{Name: "main", Current: false, Remote: false, Latest: "b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195", Created: 1783602421644},
		{Name: "my-first-branch", Current: true, Remote: false, Latest: "45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2", Created: 1783602434325},
	}
	if len(branches) != len(want) {
		t.Fatalf("branches = %+v, want %+v", branches, want)
	}
	for i := range want {
		if branches[i] != want[i] {
			t.Fatalf("branches[%d] = %+v, want %+v", i, branches[i], want[i])
		}
	}
}

func TestParseBranchList_EmptyWhenNoEntries(t *testing.T) {
	branches, err := lore.ParseBranchList(`{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}` + "\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(branches) != 0 {
		t.Fatalf("branches = %+v, want empty", branches)
	}
}

func TestParseBranchList_ErrorsOnMalformedInput(t *testing.T) {
	_, err := lore.ParseBranchList("garbage\n")
	if err == nil {
		t.Fatal("expected an error for malformed --json output")
	}
}
