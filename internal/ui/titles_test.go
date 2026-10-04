package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestWithBottomCount_PreservesBorderColor(t *testing.T) {
	// Force a real color profile - under `go test`'s non-tty default, color
	// output is stripped entirely and a missing style would pass invisibly.
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

func TestPanelLabelsAndContentStayWithinBounds(t *testing.T) {
	for _, width := range []int{3, 12, 25} {
		panels := []string{
			renderTitledPanel(false, width, 1, "5", "界界界界界界界界界界界界", strings.Repeat("界", 40)+"\nextra"),
			renderDualTitledPanel(false, width, 1, "3", "Local branches", "Remotes", true, "main"),
			withBottomCount(renderTitledPanel(false, width, 1, "2", "Files", "a"), "100000 of 100000", false),
		}
		for _, panel := range panels {
			if lipgloss.Width(panel) != width+borderWidth || lipgloss.Height(panel) != 1+borderHeight {
				t.Fatalf("inner width %d rendered %dx%d: %q", width, lipgloss.Width(panel), lipgloss.Height(panel), panel)
			}
		}
	}
}

func TestWithScrollbar_NoOpWhenNothingToScroll(t *testing.T) {
	p := renderTitledPanel(false, 20, 3, "2", "Files", "M a.go\nM b.go\nM c.go")
	got := withScrollbar(p, 0, 3, 3, false)
	if got != p {
		t.Fatal("expected no change when total <= height")
	}
}

func TestWithScrollbar_ThumbKeepsItsSizeAtBottom(t *testing.T) {
	panel := renderTitledPanel(false, 20, 10, "2", "Files", strings.Repeat("row\n", 9)+"row")
	for _, start := range []int{0, 1} {
		got := withScrollbar(panel, start, 11, 10, false)
		if cells := strings.Count(got, "▐"); cells != 10 {
			t.Fatalf("start %d drew %d thumb cells, want the full 10-cell thumb", start, cells)
		}
	}
}

func TestWithScrollbar_ThumbReplacesBorderCellInsteadOfAddingAColumn(t *testing.T) {
	p := renderTitledPanel(false, 20, 5, "2", "Files", "M a.go\nM b.go\nM c.go\nM d.go\nM e.go")
	origWidth := lipgloss.Width(strings.Split(p, "\n")[1])

	got := withScrollbar(p, 0, 10, 5, false)
	lines := strings.Split(got, "\n")

	thumbRows := map[int]bool{}
	for i, l := range lines {
		if strings.HasSuffix(l, "▐") {
			thumbRows[i] = true
		}
	}
	if len(thumbRows) == 0 {
		t.Fatal("expected at least one row to carry the thumb glyph ▐")
	}

	for i, l := range lines {
		if i == 0 || i == len(lines)-1 {
			continue // top/bottom border rows untouched
		}
		if w := lipgloss.Width(l); w != origWidth {
			t.Fatalf("row %d width = %d, want unchanged %d - the thumb must replace the border cell, not add a column", i, w, origWidth)
		}
		if !thumbRows[i] && !strings.HasSuffix(l, "│") {
			t.Fatalf("non-thumb row %d lost its normal border character: %q", i, l)
		}
	}
}
