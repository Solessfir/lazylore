package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

func TestCurrentFooter_EmptyWhenPromptOpen(t *testing.T) {
	// Prompts render as a centered popup (see renderPromptModal), not the
	// footer line, so the footer must stay blank while one is open -
	// otherwise the panels above it would shrink to leave room for text
	// that's no longer drawn there.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.prompt = promptConfirmDiscardAll
	if got := m.currentFooter(); got != "" {
		t.Fatalf("currentFooter() = %q, want empty while a prompt is open", got)
	}
}

func TestCurrentFooter_StillShowsError(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.err = errors.New("boom")
	if got := m.currentFooter(); !strings.Contains(got, "boom") {
		t.Fatalf("currentFooter() = %q, want it to contain the error message", got)
	}
}

func TestRenderPromptModal_CommitShowsInput(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updatedStatus, _ := m.Update(statusMsg{status: lore.Status{Staged: []lore.FileChange{{Status: 'A', Path: "a.txt"}}}})
	m = updatedStatus.(Model)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m2 := updated.(Model)
	got := m2.renderPromptModal()
	if !strings.Contains(got, "Commit") {
		t.Fatalf("renderPromptModal() = %q, want it to contain the title %q", got, "Commit")
	}
}

func TestRenderPromptModal_ConfirmShowsPendingLabel(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.prompt = promptConfirmBranchReset
	m.pendingResetLabel = "Reset current branch to main"
	got := m.renderPromptModal()
	if !strings.Contains(got, "Reset current branch to main") {
		t.Fatalf("renderPromptModal() = %q, want it to contain the pending label", got)
	}
}

func TestBuildHelpRows_IncludesFocusedPanelAndGlobalSections(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusHistory
	rows := m.buildHelpRows()

	var hasLocal, hasGlobal bool
	for _, r := range rows {
		if r.desc == "Drop (revert) selected revision" {
			hasLocal = true
		}
		if r.desc == "Close this help" {
			hasGlobal = true
		}
	}
	if !hasLocal {
		t.Fatalf("buildHelpRows() for focusHistory missing its local rows: %+v", rows)
	}
	if !hasGlobal {
		t.Fatalf("buildHelpRows() missing the Global section: %+v", rows)
	}
}
