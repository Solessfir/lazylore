package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

func TestUpdateFocusedList_RoutesKeysToDiffViewport(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo")
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
	m := NewModel(fake, "test-repo")
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

func TestModel_DKeyOnFileOpensDiscardConfirmPrompt(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"unstage a.txt": {ExitCode: 0},
		"reset a.txt":   {ExitCode: 0},
	}}
	m := NewModel(fake, "test-repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)
	if m3.prompt != promptConfirmDiscard {
		t.Fatalf("prompt = %v, want promptConfirmDiscard", m3.prompt)
	}
	if m3.pendingDiscardPath != "a.txt" {
		t.Fatalf("pendingDiscardPath = %q, want %q", m3.pendingDiscardPath, "a.txt")
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd yet (confirmation pending), got %v", cmd)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("expected no runner calls before confirmation, got %+v", fake.Calls)
	}
}

func TestModel_YKeyConfirmsDiscardAndUnstagesThenResets(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"unstage a.txt": {ExitCode: 0},
		"reset a.txt":   {ExitCode: 0},
	}}
	m := NewModel(fake, "test-repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after y = %v, want promptNone", m4.prompt)
	}
	if m4.pendingDiscardPath != "" {
		t.Fatalf("pendingDiscardPath after y = %q, want empty", m4.pendingDiscardPath)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after confirming discard")
	}
	msg := cmd()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err != nil {
		t.Fatalf("unexpected error: %v", am.err)
	}
	if len(fake.Calls) != 2 || fake.Calls[0][0] != "unstage" || fake.Calls[1][0] != "reset" {
		t.Fatalf("Calls = %+v, want unstage then reset (via lore.DiscardChanges)", fake.Calls)
	}
}

func TestModel_EscCancelsDiscardPromptWithoutRunnerCalls(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"unstage a.txt": {ExitCode: 0},
		"reset a.txt":   {ExitCode: 0},
	}}
	m := NewModel(fake, "test-repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after esc = %v, want promptNone", m4.prompt)
	}
	if m4.pendingDiscardPath != "" {
		t.Fatalf("pendingDiscardPath after esc = %q, want empty", m4.pendingDiscardPath)
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd after cancelling, got %v", cmd)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("expected no runner calls after cancelling, got %+v", fake.Calls)
	}
}
