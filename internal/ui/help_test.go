package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazylore/internal/lore"
)

func helpTestModel() Model {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	m.width, m.height = 120, 40
	m.resize()
	m.openHelp()
	return m
}

func updateHelp(m Model, msg tea.Msg) Model {
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func typeHelp(m Model, text string) Model {
	return updateHelp(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
}

func TestHelpSearchMatchesKeysDescriptionsAndFuzzyTerms(t *testing.T) {
	for _, tc := range []struct{ query, binding string }{
		{"DISCARD", "D"}, {"@shift+tab", "shift+tab"}, {"@space", " "}, {"stg", " "},
	} {
		t.Run(tc.query, func(t *testing.T) {
			m := typeHelp(helpTestModel(), tc.query)
			found := false
			for _, row := range m.helpRows {
				if row.section || row.blank {
					t.Fatal("search results include a section or spacer")
				}
				found = found || row.binding == tc.binding
			}
			if !found {
				t.Fatalf("query %q did not match binding %q", tc.query, tc.binding)
			}
		})
	}
}

func TestHelpSearchKeyPrefixExcludesDescriptions(t *testing.T) {
	m := typeHelp(helpTestModel(), "@discard")
	if len(m.helpRows) != 0 {
		t.Fatal("key search matched an action description")
	}
	m = typeHelp(helpTestModel(), "@")
	_, total := selectableRank(m.buildHelpRows(), 0)
	if len(m.helpRows) != total {
		t.Fatal("bare key prefix did not show all bindings")
	}
	m = typeHelp(m, "shift+tab")
	if len(m.helpRows) != 1 || m.helpRows[0].binding != "shift+tab" {
		t.Fatal("key search did not match a navigation alias")
	}
	m = typeHelp(helpTestModel(), "shift+tab")
	if len(m.helpRows) != 0 {
		t.Fatal("description search matched a key name")
	}
}

func TestHelpSearchNavigationAndEditingStayInsideModal(t *testing.T) {
	m := helpTestModel()
	m = updateHelp(m, helpKeyMsg("/"))
	m = typeHelp(m, "jkq?")
	if !m.showHelp || m.helpInput.Value() != "jkq?" || m.focus != focusFiles {
		t.Fatal("printable keys executed bindings while searching")
	}
	m = updateHelp(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.helpInput.Value() != "" || !m.helpInput.Focused() {
		t.Fatal("clearing query left editing mode")
	}
	m = typeHelp(m, "discard")
	m = updateHelp(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.helpCursor != 1 {
		t.Fatal("down did not navigate search results")
	}
	binding := m.helpRows[m.helpCursor].binding
	m = updateHelp(m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.showHelp || m.helpInput.Focused() || m.helpInput.Value() != "" || m.helpRows[m.helpCursor].binding != binding {
		t.Fatal("escape did not clear search and preserve selected binding")
	}
	m = updateHelp(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.showHelp {
		t.Fatal("second escape did not close help")
	}
}

func TestHelpSearchZeroMatchesCannotExecuteStaleBinding(t *testing.T) {
	m := typeHelp(helpTestModel(), "zzzzzzzz")
	for _, msg := range []tea.Msg{
		tea.KeyMsg{Type: tea.KeyEnter}, tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyDown},
		tea.MouseMsg{Button: tea.MouseButtonWheelDown}, tea.MouseMsg{Button: tea.MouseButtonWheelUp},
	} {
		m = updateHelp(m, msg)
	}
	view := m.renderHelpModal()
	if len(m.helpRows) != 0 || !m.showHelp || m.prompt != promptNone ||
		!strings.Contains(view, "No matching keybindings") || !strings.Contains(view, "0 of 0") {
		t.Fatal("empty search did not remain safely visible")
	}
	m = updateHelp(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.helpInput.Value() != "zzzzzzz" {
		t.Fatal("backspace did not edit an empty result query")
	}
}

func TestHelpSearchResizeAndBackgroundCompletionPreserveState(t *testing.T) {
	m := typeHelp(helpTestModel(), "discard")
	m = updateHelp(m, tea.KeyMsg{Type: tea.KeyDown})
	selected := m.helpRows[m.helpCursor].binding
	start := startTestActivity(t, &m, m.activityCmd("Loading diff", func() tea.Msg {
		return diffMsg{request: mainContentRequest{id: 999}}
	}))
	for _, size := range []tea.WindowSizeMsg{{Width: 20, Height: 5}, {Width: 80, Height: 24}, {Width: 120, Height: 40}} {
		m = updateHelp(m, size)
		if m.helpInput.Value() != "discard" || !m.helpInput.Focused() || m.helpRows[m.helpCursor].binding != selected {
			t.Fatal("resize changed search or selection")
		}
		if m.layoutFits() && lipgloss.Width(m.renderHelpModal()) > m.width {
			t.Fatal("search popup overflowed resized terminal")
		}
	}
	m = updateHelp(m, start.cmd())
	if m.activityName() != "" || !m.showHelp || m.helpInput.Value() != "discard" || m.helpRows[m.helpCursor].binding != selected {
		t.Fatal("stale load completion lost modal state or remained busy")
	}
}

func TestFilteredHelpExecutionRetainsConfirmation(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := helpTestModel()
	m.runner = fake
	m = updateHelp(m, statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "known.txt"}}}})
	m = typeHelp(m, "discard all changes")
	if len(m.helpRows) != 1 || m.helpRows[0].binding != "D" {
		t.Fatal("search did not isolate discard-all action")
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.showHelp || m.prompt != promptConfirmDiscardAll || len(fake.Calls) != 0 || len(m.pendingDiscardPaths) != 1 {
		t.Fatal("executing filtered binding bypassed confirmation")
	}
}

func TestHelpSearchEditsDoNotMutatePreviousModel(t *testing.T) {
	m := typeHelp(helpTestModel(), "discard")
	m = updateHelp(m, tea.KeyMsg{Type: tea.KeyLeft})
	updated := typeHelp(m, "X")
	if m.helpInput.Value() != "discard" || updated.helpInput.Value() != "discarXd" {
		t.Fatal("editing search mutated the previous model's input")
	}
}

func TestHelpSearchLongUnicodeQueryFitsTitle(t *testing.T) {
	m := typeHelp(helpTestModel(), strings.Repeat("界🙂", 80))
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 120, Height: 40}} {
		m = updateHelp(m, size)
		box := m.renderHelpModal()
		if lipgloss.Width(box) != m.helpWidth+borderWidth || !strings.Contains(box, "Keybindings") {
			t.Fatal("long Unicode search overflowed or overwrote the title")
		}
	}
}
