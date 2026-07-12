package ui

import (
	"errors"
	"testing"
)

func TestCommandLog_AppendUnderMaxKeepsAllEntries(t *testing.T) {
	log := newCommandLogModel(3)
	log.AppendAction("Stage file", []string{"lore stage a.txt"}, nil)
	log.AppendAction("Unstage file", []string{"lore unstage a.txt"}, nil)
	want := "Stage file\n  lore stage a.txt\n\nUnstage file\n  lore unstage a.txt"
	if got := log.View(); got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestCommandLog_AppendOverMaxTruncatesOldest(t *testing.T) {
	log := newCommandLogModel(2)
	log.AppendAction("one", nil, nil)
	log.AppendAction("two", nil, nil)
	log.AppendAction("three", nil, nil)
	want := "two\n\nthree"
	if got := log.View(); got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestCommandLog_AppendActionShowsMultipleCommandLines(t *testing.T) {
	log := newCommandLogModel(3)
	log.AppendAction("Discard all changes", []string{"lore unstage a.txt b.txt", "lore reset --purge a.txt b.txt"}, nil)
	want := "Discard all changes\n  lore unstage a.txt b.txt\n  lore reset --purge a.txt b.txt"
	if got := log.View(); got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestCommandLog_AppendActionShowsErrorLine(t *testing.T) {
	log := newCommandLogModel(3)
	log.AppendAction("Stage file", []string{"lore stage a.txt"}, errors.New("boom"))
	if got := log.View(); got != "Stage file\n  lore stage a.txt\n  boom" {
		t.Fatalf("View() = %q, want a trailing indented error line", got)
	}
}
