package ui

import "testing"

func TestCommandLog_AppendUnderMaxKeepsAllEntries(t *testing.T) {
	log := newCommandLogModel(3)
	log.Append("one")
	log.Append("two")
	if got := log.View(); got != "one\ntwo" {
		t.Fatalf("View() = %q, want %q", got, "one\ntwo")
	}
}

func TestCommandLog_AppendOverMaxTruncatesOldest(t *testing.T) {
	log := newCommandLogModel(2)
	log.Append("one")
	log.Append("two")
	log.Append("three")
	if got := log.View(); got != "two\nthree" {
		t.Fatalf("View() = %q, want %q", got, "two\nthree")
	}
}
