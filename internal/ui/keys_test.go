package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

func TestUpdateFocusedList_RoutesKeysToDiffViewport(t *testing.T) {
	m := NewModel(&lore.FakeRunner{})
	m.focus = focusDiff
	m.diff.vp.Width = 10
	m.diff.vp.Height = 2
	m.diff.SetContent("line1\nline2\nline3\nline4\nline5")

	if m.diff.vp.YOffset != 0 {
		t.Fatalf("precondition failed: YOffset = %d, want 0", m.diff.vp.YOffset)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := updated.(Model)
	if m2.diff.vp.YOffset == 0 {
		t.Fatalf("expected the down key to scroll the diff viewport, YOffset = %d", m2.diff.vp.YOffset)
	}
}

func TestHandleKey_FilterModeBypassesGlobalShortcuts(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"reset a.txt": {ExitCode: 0},
	}}
	m := NewModel(fake)
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	// Enter filter-typing mode on the focused (Files) list, as "/" would.
	var filterCmd tea.Cmd
	m2.files, filterCmd = m2.files.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_ = filterCmd
	if !m2.files.SettingFilter() {
		t.Fatal("expected files list to be in filter-typing state after \"/\"")
	}

	// "d" is normally the global reset shortcut. While filtering, it must be
	// typed into the filter box instead of firing resetCmd.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	if len(fake.Calls) != 0 {
		t.Fatalf("global shortcut fired while filtering: Calls = %+v, want none", fake.Calls)
	}
	if got := m3.files.FilterInput.Value(); got != "d" {
		t.Fatalf("filter input value = %q, want %q", got, "d")
	}
	if !m3.files.SettingFilter() {
		t.Fatal("expected files list to still be in filter-typing state")
	}
}
