package ui

import "github.com/charmbracelet/lipgloss"

var (
	focusedPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6"))
	unfocusedPanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
)

func panelStyle(focused bool) lipgloss.Style {
	if focused {
		return focusedPanelStyle
	}
	return unfocusedPanelStyle
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	left := lipgloss.JoinVertical(lipgloss.Left,
		panelStyle(m.focus == focusFiles).Render("Files\n"+m.files.View()),
		panelStyle(m.focus == focusBranches).Render("Branches\n"+m.branches.View()),
		panelStyle(m.focus == focusHistory).Render("History\n"+m.history.View()),
	)
	right := panelStyle(m.focus == focusDiff).Render("Diff\n" + m.diff.vp.View())
	main := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	footer := m.log.View()
	if m.prompt != promptNone {
		footer = m.input.View()
	} else if m.err != nil {
		footer = errorStyle.Render(m.err.Error()) + "\n" + footer
	}

	return lipgloss.JoinVertical(lipgloss.Left, main, footer)
}
