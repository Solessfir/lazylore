package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestWithBottomCount_PreservesBorderColor(t *testing.T) {
	// Regression: withBottomCount used to rebuild the bottom border from
	// plain, unstyled runes, silently dropping the ANSI color/bold every
	// other border segment carries. Force a real color profile - under
	// `go test`'s non-tty default, color output is stripped entirely and
	// this bug would pass invisibly.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	p := renderTitledPanel(true, 20, 3, "1", "Files", "M a.go\nM b.go\nM c.go")
	p = withBottomCount(p, "3 of 3", true)
	lines := strings.Split(p, "\n")
	bottom := lines[len(lines)-1]

	if !strings.Contains(bottom, "\x1b[") {
		t.Fatalf("bottom border lost its color styling: %q", bottom)
	}
	if !strings.Contains(bottom, "3 of 3") {
		t.Fatalf("bottom border missing count: %q", bottom)
	}
}

func TestWithBottomCount_PadsBothSidesOfCount(t *testing.T) {
	p := renderTitledPanel(false, 20, 3, "1", "Files", "M a.go\nM b.go\nM c.go")
	p = withBottomCount(p, "1 of 34", false)
	lines := strings.Split(p, "\n")
	bottom := lines[len(lines)-1]

	if !strings.HasSuffix(bottom, "1 of 34─╯") {
		t.Fatalf("bottom border must have a ─ between the count and the corner; got %q", bottom)
	}
}
