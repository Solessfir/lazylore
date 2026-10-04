package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

func TestNewListDelegate_FocusedGetsBackgroundFill(t *testing.T) {
	d, ok := newListDelegate(true, 42).(list.DefaultDelegate)
	if !ok {
		t.Fatalf("newListDelegate(true, _) did not return a list.DefaultDelegate")
	}
	if d.Styles.SelectedTitle.GetBackground() != selectedBg {
		t.Fatalf("focused SelectedTitle background = %v, want %v", d.Styles.SelectedTitle.GetBackground(), selectedBg)
	}
	if !d.Styles.SelectedTitle.GetBold() {
		t.Fatal("focused SelectedTitle should be bold")
	}
}

func TestNewListDelegate_FocusedBackgroundIsBoundedToPanelWidth(t *testing.T) {
	// This is the actual bug: without an explicit Width(), the background
	// fill isn't bounded to this panel at all, and bleeds across the rest
	// of the terminal row past the panel's own border.
	d, ok := newListDelegate(true, 42).(list.DefaultDelegate)
	if !ok {
		t.Fatalf("newListDelegate(true, _) did not return a list.DefaultDelegate")
	}
	if got := d.Styles.SelectedTitle.GetWidth(); got != 42 {
		t.Fatalf("focused SelectedTitle width = %d, want 42 (the panel's content width)", got)
	}
	if got := d.Styles.SelectedDesc.GetWidth(); got != 42 {
		t.Fatalf("focused SelectedDesc width = %d, want 42", got)
	}
}

func TestNewListDelegate_UnfocusedHasNoBackgroundFill(t *testing.T) {
	d, ok := newListDelegate(false, 42).(list.DefaultDelegate)
	if !ok {
		t.Fatalf("newListDelegate(false, _) did not return a list.DefaultDelegate")
	}
	if bg := d.Styles.SelectedTitle.GetBackground(); bg == selectedBg {
		t.Fatalf("unfocused SelectedTitle has the focused background fill %v, want none", bg)
	}
	if !d.Styles.SelectedTitle.GetBold() || !d.Styles.SelectedDesc.GetBold() {
		t.Fatal("unfocused cursor title and description should be bold")
	}
}

func TestListStylesUseTerminalForeground(t *testing.T) {
	if selectedBg != lipgloss.Color("#292a2e") {
		t.Fatalf("selected background = %v, want #292a2e", selectedBg)
	}
	for _, focused := range []bool{false, true} {
		d := newListDelegate(focused, 42).(list.DefaultDelegate)
		for _, style := range []lipgloss.Style{d.Styles.NormalTitle, d.Styles.NormalDesc, d.Styles.SelectedTitle, d.Styles.SelectedDesc, d.Styles.DimmedTitle, d.Styles.DimmedDesc} {
			if style.GetForeground() != (lipgloss.NoColor{}) {
				t.Fatalf("list text foreground = %v, want terminal default", style.GetForeground())
			}
		}
	}
	l := newPanelList(newListDelegate(true, 42))
	for _, style := range []lipgloss.Style{l.Styles.FilterPrompt, l.Styles.FilterCursor, l.Styles.NoItems, l.FilterInput.PromptStyle, l.FilterInput.Cursor.Style} {
		if style.GetForeground() != (lipgloss.NoColor{}) {
			t.Fatalf("list filter/empty text foreground = %v, want terminal default", style.GetForeground())
		}
	}
}

func TestNewPanelList_PaginationHidden(t *testing.T) {
	l := newPanelList(newListDelegate(true, 42))
	if l.ShowPagination() {
		t.Fatal("panel lists should not show pagination dots - panels are too small to page through, and it wastes a row")
	}
}
