package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newHelpInput() textinput.Model {
	input := textinput.New()
	input.Prompt = "Filter ('@' for keybindings): "
	input.TextStyle = lipgloss.NewStyle().Foreground(borderFocused)
	input.PromptStyle = input.TextStyle
	input.Cursor.Style = input.TextStyle
	input.Cursor.SetMode(cursor.CursorStatic)
	return input
}

func (m *Model) resizeHelp() {
	m.helpWidth = m.popupWidth(90)
	m.helpHeight = min(max(1, len(m.buildHelpRows())), max(3, 3*m.height/4-borderHeight))
	width := max(1, m.helpWidth-lipgloss.Width(m.helpInput.Prompt)-1)
	if m.helpInput.Width != width {
		value, position := m.helpInput.Value(), m.helpInput.Position()
		m.helpInput.Width = width
		// Changing Width alone leaves the text input's scroll offsets stale.
		m.helpInput.SetValue("")
		m.helpInput.SetValue(value)
		m.helpInput.SetCursor(position)
	}
}

func (m *Model) filterHelp() {
	rows := m.buildHelpRows()
	if query := m.helpInput.Value(); query != "" {
		keysOnly := strings.HasPrefix(query, "@")
		if keysOnly {
			query = strings.TrimPrefix(query, "@")
		}
		var bindings []helpRow
		var targets []string
		var sections []helpRow
		var bindingSections []int
		for _, row := range rows {
			if row.section {
				sections = append(sections, row)
			} else if !row.blank {
				bindings = append(bindings, row)
				bindingSections = append(bindingSections, len(sections)-1)
				target := row.desc
				if keysOnly {
					target = row.key
				}
				targets = append(targets, target)
			}
		}
		groups := make([][]helpRow, len(sections))
		if query == "" {
			for i, binding := range bindings {
				groups[bindingSections[i]] = append(groups[bindingSections[i]], binding)
			}
		} else {
			for _, match := range list.DefaultFilter(query, targets) {
				section := bindingSections[match.Index]
				groups[section] = append(groups[section], bindings[match.Index])
			}
		}
		rows = nil
		for i, section := range sections {
			if len(groups[i]) == 0 {
				continue
			}
			if len(rows) > 0 {
				rows = append(rows, helpRow{blank: true})
			}
			rows = append(rows, section)
			rows = append(rows, groups[i]...)
		}
	}
	m.helpRows = rows
	m.helpCursor = firstSelectable(rows)
	m.resizeHelp()
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.helpInput.Focused() {
			binding := ""
			if m.helpCursor >= 0 && m.helpCursor < len(m.helpRows) {
				binding = m.helpRows[m.helpCursor].binding
			}
			m.helpInput.SetValue("")
			m.helpInput.Blur()
			m.filterHelp()
			for i, row := range m.helpRows {
				if binding != "" && row.binding == binding {
					m.helpCursor = i
					break
				}
			}
		} else {
			m.showHelp = false
		}
		return m, nil
	case "up":
		m.helpCursor = prevSelectable(m.helpRows, m.helpCursor)
		return m, nil
	case "down":
		m.helpCursor = nextSelectable(m.helpRows, m.helpCursor)
		return m, nil
	case "enter":
		if m.helpCursor >= 0 && m.helpCursor < len(m.helpRows) {
			if binding := m.helpRows[m.helpCursor].binding; binding != "" {
				m.showHelp = false
				return m.handleKey(helpKeyMsg(binding))
			}
		}
		return m, nil
	}
	if !m.helpInput.Focused() {
		switch msg.String() {
		case "?":
			m.showHelp = false
			return m, nil
		case "j":
			m.helpCursor = nextSelectable(m.helpRows, m.helpCursor)
			return m, nil
		case "k":
			m.helpCursor = prevSelectable(m.helpRows, m.helpCursor)
			return m, nil
		case "/":
			m.helpInput.Focus()
			return m, nil
		}
		if msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace {
			return m, nil
		}
		m.helpInput.Focus()
	}
	return m.updateHelpInput(msg)
}

func (m Model) updateHelpInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Detach the rune slice before editing a copied Bubble Tea model.
	previous := m.helpInput.Value()
	m.helpInput.SetValue(previous)
	var cmd tea.Cmd
	m.helpInput, cmd = m.helpInput.Update(msg)
	if m.helpInput.Value() != previous {
		m.filterHelp()
	}
	return m, cmd
}
