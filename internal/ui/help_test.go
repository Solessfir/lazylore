package ui

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"

	"github.com/solessfir/lazylore/internal/lore"
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
					continue
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
	if _, filteredTotal := selectableRank(m.helpRows, m.helpCursor); filteredTotal != total {
		t.Fatal("bare key prefix did not show all bindings")
	}
	m = typeHelp(m, "shift+tab")
	if _, total := selectableRank(m.helpRows, m.helpCursor); total != 1 || m.helpRows[m.helpCursor].binding != "shift+tab" {
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
	if rank, _ := selectableRank(m.helpRows, m.helpCursor); rank != 2 {
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
	if _, total := selectableRank(m.helpRows, m.helpCursor); total != 1 || m.helpRows[m.helpCursor].binding != "D" {
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

func TestHelpSearchResizeReflowsInputViewport(t *testing.T) {
	for _, position := range []int{50, 100} {
		m := typeHelp(helpTestModel(), strings.Repeat("x", 100))
		m.helpInput.SetCursor(position)
		for _, width := range []int{60, 40, 120} {
			m = updateHelp(m, tea.WindowSizeMsg{Width: width, Height: 40})
			lines := strings.Split(m.renderHelpModal(), "\n")
			if !strings.Contains(lines[0], "Keybindings") || !strings.Contains(lines[len(lines)-2], m.helpInput.Prompt) {
				t.Fatalf("search input disappeared or overwrote title at width %d", width)
			}
			if lipgloss.Width(m.helpInput.View()) > m.helpInput.Width+lipgloss.Width(m.helpInput.Prompt)+1 {
				t.Fatalf("search viewport did not fit resized input at width %d", width)
			}
			if m.helpInput.Value() != strings.Repeat("x", 100) || m.helpInput.Position() != position || !m.helpInput.Focused() {
				t.Fatal("resize changed search text, cursor, or focus")
			}
		}
	}
}

func TestFilteredHelpKeepsSectionsPaddingAndBottomInput(t *testing.T) {
	m := helpTestModel()
	initial := m.renderHelpModal()
	if !strings.Contains(strings.Split(initial, "\n")[0], "──(Type to filter)") {
		t.Fatal("unfiltered help did not show its filter hint")
	}
	m = typeHelp(m, "discard")
	if len(m.helpRows) != 3 || !m.helpRows[0].section || m.helpRows[0].key != "Local" || m.helpCursor != 1 {
		t.Fatal("filtered help did not keep the matching section and select its first binding")
	}
	box := m.renderHelpModal()
	lines := strings.Split(box, "\n")
	if strings.Contains(lines[0], "(Type to filter)") || strings.Contains(lines[0], "@ for keys") {
		t.Fatal("filtered help showed a redundant header hint")
	}
	if lipgloss.Height(box) != lipgloss.Height(initial) || lipgloss.Width(box) != lipgloss.Width(initial) {
		t.Fatal("filtering resized the help panel")
	}
	if !strings.Contains(lines[1], "─── Local") || strings.Contains(box, "─── Global") || strings.Contains(box, "─── Navigation") {
		t.Fatal("filtered help showed missing or unmatched sections")
	}
	if !strings.Contains(lines[len(lines)-3], "├"+strings.Repeat("─", m.helpWidth)+"┤") ||
		!strings.Contains(lines[len(lines)-2], m.helpInput.Prompt+"discard") ||
		strings.Contains(lines[0], "discard") || !strings.Contains(lines[len(lines)-1], "1 of 2") {
		t.Fatal("filter input, divider, or binding count did not match the help layout")
	}
	if strings.Trim(lines[len(lines)-4], "│ ") != "" {
		t.Fatal("filtered help did not pad the body above its filter input")
	}
	m = updateHelp(m, tea.KeyMsg{Type: tea.KeyDown})
	if !strings.Contains(m.renderHelpModal(), "2 of 2") {
		t.Fatal("filtered binding count included its section heading")
	}
}

func TestFilteredHelpKeepsSectionColorAndSelectionBackground(t *testing.T) {
	previous := lipgloss.DefaultRenderer()
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	lipgloss.SetDefaultRenderer(renderer)
	t.Cleanup(func() { lipgloss.SetDefaultRenderer(previous) })
	m := typeHelp(helpTestModel(), "discard")
	box := m.renderHelpModal()
	cells := cellbuf.NewBuffer(lipgloss.Width(box), lipgloss.Height(box))
	cellbuf.SetContent(cells, box)
	plain := strings.Split(ansi.Strip(box), "\n")
	headingX := ansi.StringWidth(plain[1][:strings.Index(plain[1], "Local")])
	if !sameSelectionColor(cells.Cell(headingX, 1).Style.Fg, ansi.BasicColor(2)) {
		t.Fatal("filtered help lost its green section heading")
	}
	for x := 1; x < cells.Width()-1; x++ {
		cell := cells.Cell(x, m.helpCursor+1)
		if cell.Style.Attrs&cellbuf.BoldAttr == 0 || cell.Style.Bg == nil {
			t.Fatalf("filtered selection lost its highlight at column %d", x)
		}
	}
}
