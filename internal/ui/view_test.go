package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderPanel_MatchesRequestedSizeRegardlessOfContentLength(t *testing.T) {
	const width, height = 30, 5

	cases := map[string]string{
		"short content": "x",
		"long content":  "a very very very long line of content that is much wider than thirty columns",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			out := renderPanel(false, width, height, "Title", content)
			lines := strings.Split(out, "\n")

			wantLines := height + borderHeight
			if len(lines) != wantLines {
				t.Fatalf("line count = %d, want %d; output:\n%s", len(lines), wantLines, out)
			}
			for i, line := range lines {
				wantWidth := width + borderWidth
				if got := lipgloss.Width(line); got != wantWidth {
					t.Fatalf("line %d width = %d, want %d; line: %q", i, got, wantWidth, line)
				}
			}
		})
	}
}

func TestRenderPanel_DifferentContentProducesSameSize(t *testing.T) {
	// This is the exact bug: two panels with different content (e.g. Files
	// with two long entries vs Branches with one short one) rendering at
	// different widths/heights instead of the same requested size.
	a := renderPanel(false, 20, 4, "Files", "M a.go\nM b.txt")
	b := renderPanel(false, 20, 4, "Branches", "* main")

	linesA := strings.Split(a, "\n")
	linesB := strings.Split(b, "\n")
	if len(linesA) != len(linesB) {
		t.Fatalf("line counts differ: %d vs %d", len(linesA), len(linesB))
	}
	for i := range linesA {
		wA, wB := lipgloss.Width(linesA[i]), lipgloss.Width(linesB[i])
		if wA != wB {
			t.Fatalf("line %d width differs: %d vs %d", i, wA, wB)
		}
	}
}
