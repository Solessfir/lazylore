package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
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
}

func TestNewPanelList_PaginationHidden(t *testing.T) {
	l := newPanelList(newListDelegate(true, 42))
	if l.ShowPagination() {
		t.Fatal("panel lists should not show pagination dots - panels are too small to page through, and it wastes a row")
	}
}
