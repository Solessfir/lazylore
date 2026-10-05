package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/solessfir/lazylore/internal/lore"
)

func TestCommandLogHeightAdaptsAndFocusFillsRightColumn(t *testing.T) {
	for _, height := range []int{24, 39, 40, 60} {
		m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: height})
		m = updated.(Model)
		wantLogHeight := 10
		if height < 40 {
			wantLogHeight = 3
		}
		l := m.computeMouseLayout()
		view := strings.Split(ansi.Strip(m.View()), "\n")
		if !strings.Contains(view[l.diffAreaEnd], "Command Log") || l.diffAreaEnd+wantLogHeight != height-keybindBarHeight {
			t.Fatalf("height %d log starts at %d, want %d outer rows", height, l.diffAreaEnd, wantLogHeight)
		}
		updated, _ = m.Update(tea.MouseMsg{X: l.diffBoxLeft + 2, Y: l.diffAreaEnd + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		m = updated.(Model)
		if m.focus != focusCommandLog {
			t.Fatalf("height %d click below Diff did not focus Command Log", height)
		}
		view = strings.Split(ansi.Strip(m.View()), "\n")
		if !strings.Contains(view[0], "Command Log") || len(view) != height {
			t.Fatalf("height %d focused log did not fill the right column", height)
		}
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
		m = updated.(Model)
		if m.computeMouseLayout().diffAreaEnd != l.diffAreaEnd {
			t.Fatalf("height %d leaving log changed Diff geometry", height)
		}
	}
}

func TestScrollablePanelsUseTheirRightBorderForThumbs(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	var branches []lore.Branch
	var revisions []lore.Revision
	for i := 0; i < 50; i++ {
		branches = append(branches, lore.Branch{Name: fmt.Sprintf("branch%02d", i)})
		revisions = append(revisions, lore.Revision{Hash: fmt.Sprintf("revision%02d", i)})
	}
	updated, _ = m.Update(branchesMsg{branches: branches})
	m = updated.(Model)
	updated, _ = m.Update(historyMsg{revisions: revisions})
	m = updated.(Model)
	m.diff.SetContentRaw(strings.Repeat(strings.Repeat("x", m.diff.vp.Width)+"\n", 50))
	l := m.computeMouseLayout()
	view := strings.Split(ansi.Strip(m.View()), "\n")
	for _, tc := range []struct {
		name                 string
		row, column          int
		precedingContentCell string
	}{
		{name: "Branches", row: l.branchesBoxTop + 1, column: l.leftW - 1},
		{name: "History", row: l.historyBoxTop + 1, column: l.leftW - 1},
		{name: "Diff", row: 1, column: l.leftW + m.diff.vp.Width + borderWidth - 1, precedingContentCell: "x"},
	} {
		if got := ansi.Cut(view[tc.row], tc.column, tc.column+1); got != "▐" {
			t.Fatalf("%s scrollbar cell = %q, want border thumb", tc.name, got)
		}
		if tc.precedingContentCell != "" && ansi.Cut(view[tc.row], tc.column-1, tc.column) != tc.precedingContentCell {
			t.Fatalf("%s scrollbar consumed an interior content cell", tc.name)
		}
	}
	if lipgloss.Width(m.View()) != m.width-1 {
		t.Fatal("border scrollbars changed the outer panel width")
	}
}
