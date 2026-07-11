package ui

import "testing"

func TestColorizeDiff_PreservesLineContent(t *testing.T) {
	input := "hello.txt\n--- hello.txt@1\n+++ hello.txt\n@@ -1 +1,2 @@\n Hello, Lore\n+Second line added\n"
	out := colorizeDiff(input)
	for _, want := range []string{"hello.txt", "--- hello.txt@1", "+++ hello.txt", "@@ -1 +1,2 @@", "Hello, Lore", "Second line added"} {
		if !containsSubstring(out, want) {
			t.Fatalf("colorizeDiff output missing %q; got %q", want, out)
		}
	}
}

func containsSubstring(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestNewDiffModel_SetContentDoesNotPanic(t *testing.T) {
	m := newDiffModel(80, 24)
	m.SetContent("some diff text\n")
}

func TestDiffModel_SetContentRawSkipsDiffColoring(t *testing.T) {
	// Regression: a branch log line like "abc123 - fix bug" starts with a
	// literal "-" character. Routed through SetContent (colorizeDiff), that
	// would get wrongly styled as a diff deletion; SetContentRaw must leave
	// it untouched.
	m := newDiffModel(80, 24)
	m.SetContentRaw("abc123 dev - fix bug\nmore text")
	view := m.vp.View()
	if !containsSubstring(view, "fix bug") {
		t.Fatalf("SetContentRaw content missing from viewport view: %q", view)
	}
}
