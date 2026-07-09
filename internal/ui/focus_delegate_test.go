package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
)

func TestNewListDelegate_FocusedGetsBackgroundFill(t *testing.T) {
	d, ok := newListDelegate(true).(list.DefaultDelegate)
	if !ok {
		t.Fatalf("newListDelegate(true) did not return a list.DefaultDelegate")
	}
	if d.Styles.SelectedTitle.GetBackground() != selectedBg {
		t.Fatalf("focused SelectedTitle background = %v, want %v", d.Styles.SelectedTitle.GetBackground(), selectedBg)
	}
	if !d.Styles.SelectedTitle.GetBold() {
		t.Fatal("focused SelectedTitle should be bold")
	}
}

func TestNewListDelegate_UnfocusedHasNoBackgroundFill(t *testing.T) {
	d, ok := newListDelegate(false).(list.DefaultDelegate)
	if !ok {
		t.Fatalf("newListDelegate(false) did not return a list.DefaultDelegate")
	}
	if bg := d.Styles.SelectedTitle.GetBackground(); bg == selectedBg {
		t.Fatalf("unfocused SelectedTitle has the focused background fill %v, want none", bg)
	}
}

func TestNewPanelList_PaginationHidden(t *testing.T) {
	l := newPanelList(true)
	if l.ShowPagination() {
		t.Fatal("panel lists should not show pagination dots - panels are too small to page through, and it wastes a row")
	}
}
