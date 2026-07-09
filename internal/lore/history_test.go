package lore_test

import (
	"testing"

	"lazylore/internal/lore"
)

// Captured verbatim from `lore.exe history --oneline` with two revisions.
const historyOnelineOutput = `2 Second revision on branch
1 Initial revision
`

func TestParseHistoryOneline_TwoRevisions(t *testing.T) {
	revisions, err := lore.ParseHistoryOneline(historyOnelineOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []lore.Revision{
		{Number: 2, Message: "Second revision on branch"},
		{Number: 1, Message: "Initial revision"},
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

func TestParseHistoryOneline_Empty(t *testing.T) {
	revisions, err := lore.ParseHistoryOneline("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(revisions) != 0 {
		t.Fatalf("revisions = %+v, want empty", revisions)
	}
}

func TestParseHistoryOneline_ErrorsOnMalformedLine(t *testing.T) {
	_, err := lore.ParseHistoryOneline("not-a-number a message\n")
	if err == nil {
		t.Fatal("expected an error for a non-numeric revision")
	}
}
