package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/solessfir/lazylore/internal/lore"
)

func TestDiscardMenuDescriptionFollowsSelectionAndFitsResize(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	m.prompt = promptDiscardMenu
	m.pendingDiscardPath = "notes.txt"
	m.width, m.height = 120, 40
	for cursor, want := range []string{
		"Discard both staged and unstaged changes in 'notes.txt'.",
		"Disabled: The selected items don't have both staged and unstaged changes.",
		"",
	} {
		m.discardCursor = cursor
		got := ansi.Strip(m.renderDiscardMenuModal())
		if !strings.Contains(got, want) || !strings.Contains(got, "Discard changes") || !strings.Contains(got, "Cancel") {
			t.Fatalf("selection %d lost its menu or description: %q", cursor, got)
		}
		if cursor == 2 && strings.Contains(got, "notes.txt") {
			t.Fatal("Cancel retained another option's description")
		}
	}
	m.discardCursor = 1
	m.pendingDiscardPath = strings.Repeat("界/folder/", 100)
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 40, Height: 16}, {Width: 120, Height: 40}} {
		m.width, m.height = size.Width, size.Height
		got := m.renderDiscardMenuModal()
		if lipgloss.Width(got) > m.width || lipgloss.Height(got) >= m.height || !strings.Contains(ansi.Strip(got), "Disabled:") {
			t.Fatalf("discard menu lost its explanation or exceeded %dx%d: %q", m.width, m.height, got)
		}
	}
}

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

func TestConfirmationFitsTerminalWithContextualFooter(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	m.prompt = promptConfirmForceUnlock
	m.pendingForceUnlockPath = strings.Repeat("界/folder/", 100) + "asset.bin"
	m.locks = map[string]lore.Lock{m.pendingForceUnlockPath: {Owner: "someone"}}
	got := m.renderPromptModal()
	if lipgloss.Width(got) > m.width || lipgloss.Height(got) > m.height {
		t.Fatalf("confirmation rendered %dx%d for %dx%d", lipgloss.Width(got), lipgloss.Height(got), m.width, m.height)
	}
	if !strings.Contains(got, "Force-unlock") || !strings.Contains(m.View(), "Confirm: <enter> | Close/Cancel: <esc>") {
		t.Fatalf("confirmation lost context or footer: %q", got)
	}
}

func TestInputPopupsUseBoundedTitledFrames(t *testing.T) {
	for _, prompt := range []promptKind{promptCommit, promptNewBranch} {
		m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
		m.openCommitPrompt()
		m.prompt = prompt
		m.input.SetValue(strings.Repeat("branch-or-summary ", 15) + "TAIL")
		m.input.CursorEnd()
		for _, width := range []int{120, 40, 180} {
			m.width, m.height = width, 40
			got := m.renderPromptModal()
			if lipgloss.Width(got) != min(80, width-2) || lipgloss.Height(got) != 3 || !strings.Contains(got, "TAIL") {
				t.Fatalf("prompt %v at width %d lost its input or bounded frame: %q", prompt, width, got)
			}
		}
	}
}

func TestCommitModalFitsResizeWithoutChangingText(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.openCommitPrompt()
	value := strings.Repeat("long commit summary ", 10) + "TAIL"
	m.input.SetValue(value)
	m.input.CursorEnd()
	for _, width := range []int{120, 40, 100} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		m = updated.(Model)
		got := m.renderPromptModal()
		if lipgloss.Width(got) > width || lipgloss.Height(got) != 3 {
			t.Fatalf("commit modal after resizing to %d rendered %dx%d", width, lipgloss.Width(got), lipgloss.Height(got))
		}
		if m.input.Value() != value || !strings.Contains(got, "TAIL") {
			t.Fatalf("resize lost text or the cursor's visible text: %q", got)
		}
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
