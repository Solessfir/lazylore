package lore_test

import (
	"testing"

	"lazylore/internal/lore"
)

// Captured verbatim from `lore.exe --json history` with two revisions -
// each revisionHistoryEntry followed by its metadata events (message is
// what Revision needs; branch/timestamp/created-by/committed-by are real
// but unused).
const historyTwoRevisionsOutput = `{"tagName":"revisionHistory","data":{"repository":"019f46fd1f7b7880a88e902a7b44074a","branch":"019f46fd51147821aed79f28a839f591"}}
{"tagName":"revisionHistoryEntry","data":{"revision":"45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2","revisionNumber":2,"parent":["b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195","0000000000000000000000000000000000000000000000000000000000000000"]}}
{"tagName":"metadata","data":{"key":"branch","value":{"tagName":"context","data":"019f46fd51147821aed79f28a839f591"}}}
{"tagName":"metadata","data":{"key":"timestamp","value":{"tagName":"numeric","data":1783602540320}}}
{"tagName":"metadata","data":{"key":"message","value":{"tagName":"string","data":"Second revision on branch"}}}
{"tagName":"metadata","data":{"key":"created-by","value":{"tagName":"string","data":"dev@example.com"}}}
{"tagName":"metadata","data":{"key":"committed-by","value":{"tagName":"string","data":"dev@example.com"}}}
{"tagName":"revisionHistoryEntry","data":{"revision":"b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195","revisionNumber":1,"parent":["0000000000000000000000000000000000000000000000000000000000000000","0000000000000000000000000000000000000000000000000000000000000000"]}}
{"tagName":"metadata","data":{"key":"branch","value":{"tagName":"context","data":"e726318bbc3fd75ac8733a7e030cc35b"}}}
{"tagName":"metadata","data":{"key":"timestamp","value":{"tagName":"numeric","data":1783602427622}}}
{"tagName":"metadata","data":{"key":"message","value":{"tagName":"string","data":"Initial revision"}}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

func TestParseHistory_TwoRevisions(t *testing.T) {
	revisions, err := lore.ParseHistory(historyTwoRevisionsOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []lore.Revision{
		{Number: 2, Message: "Second revision on branch", Hash: "45593a0083a67a79602235b4d6c39d9d2dc1fc89375543a1bd6bd43288fa60a2", Author: "dev@example.com", Parent: "b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195"},
		{Number: 1, Message: "Initial revision", Hash: "b3e648f10d02c6162433bfdb49c06012027d19bc586560647d0bf2ff6278c195", Author: "", Parent: "0000000000000000000000000000000000000000000000000000000000000000"},
	}
	if len(revisions) != len(want) {
		t.Fatalf("revisions = %+v, want %+v", revisions, want)
	}
	for i := range want {
		if revisions[i] != want[i] {
			t.Fatalf("revisions[%d] = %+v, want %+v", i, revisions[i], want[i])
		}
	}
}

func TestParseHistory_Empty(t *testing.T) {
	revisions, err := lore.ParseHistory(`{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}` + "\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(revisions) != 0 {
		t.Fatalf("revisions = %+v, want empty", revisions)
	}
}

func TestParseHistory_ErrorsOnMalformedInput(t *testing.T) {
	_, err := lore.ParseHistory("not-json-at-all")
	if err == nil {
		t.Fatal("expected an error for malformed --json output")
	}
}
