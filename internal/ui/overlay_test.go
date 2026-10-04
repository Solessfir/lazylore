package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestOverlayPreservesWideAndCombiningBackgroundCells(t *testing.T) {
	for _, tc := range []struct {
		background string
		x          int
		want       string
	}{
		{background: "界界界界zz", x: 4, want: "界界XX界zz"},
		{background: "界界界界zz", x: 3, want: "界 XX 界zz"},
		{background: "a\u0301bcdef", x: 2, want: "a\u0301bXXef"},
		{background: "\x1b[31m界界界界zz\x1b[0m", x: 4, want: "界界XX界zz"},
	} {
		got := spliceLine(tc.x, "XX", tc.background)
		if ansiStrip(got) != tc.want || lipgloss.Width(got) != lipgloss.Width(tc.background) {
			t.Fatalf("overlay %q at %d = %q, want %q", tc.background, tc.x, got, tc.want)
		}
	}
}
