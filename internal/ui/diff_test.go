package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

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

func TestDiffModel_LeftRightKeysDoNotHorizontallyScroll(t *testing.T) {
	// The diff pane never wraps or scrolls horizontally (lazygit clips
	// instead) - left/right must be inert here so lazylore's own h/l
	// panel-switch shortcuts (and stray arrow presses) can never leave a
	// nonzero scroll offset that silently clips the next file's diff.
	m := newDiffModel(80, 24)
	if m.vp.KeyMap.Left.Enabled() || m.vp.KeyMap.Right.Enabled() {
		t.Fatal("expected viewport's Left/Right keybindings to be disabled")
	}
}

func TestNewDiffModel_SetContentDoesNotPanic(t *testing.T) {
	m := newDiffModel(80, 24)
	m.SetContent("some diff text\n")
}

func TestDiffModel_ViewWithScrollbarPadsColoredLongLinesToExactWidth(t *testing.T) {
	// Truncation must be ANSI-aware - a raw rune-count slice through the
	// middle of an escape code corrupts the terminal's color state for
	// every row rendered after it.
	m := newDiffModel(20, 5)
	m.vp.Width = 20
	// More lines than the viewport height, so viewWithScrollbar actually
	// takes the scrollbar-drawing path (its early return for
	// totalLines <= h skips the pad/truncate loop entirely).
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "+"+strings.Repeat("x", 100))
	}
	m.SetContent(strings.Join(lines, "\n"))

	out := m.viewWithScrollbar()
	for i, line := range strings.Split(out, "\n") {
		w := lipgloss.Width(line)
		if w != m.vp.Width+1 { // +1 for the trailing scrollbar track/thumb column
			t.Fatalf("line %d visible width = %d, want %d (panel width + scrollbar column); line = %q", i, w, m.vp.Width+1, line)
		}
	}
}

func TestDiffModel_SetContentRawSkipsDiffColoring(t *testing.T) {
	// A branch log line like "abc123 - fix bug" starts with a literal "-" -
	// SetContentRaw must not style it as a diff deletion.
	m := newDiffModel(80, 24)
	m.SetContentRaw("abc123 dev - fix bug\nmore text")
	view := m.vp.View()
	if !containsSubstring(view, "fix bug") {
		t.Fatalf("SetContentRaw content missing from viewport view: %q", view)
	}
}
