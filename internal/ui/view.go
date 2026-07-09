package ui

import "github.com/charmbracelet/lipgloss"

var (
	focusedPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6"))
	unfocusedPanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	keybindBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
)

// keybindBarText is the global keybinding legend pinned to the very
// bottom of the screen. Kept to keys handleKey/handlePromptKey actually
// implement - no promising a "?" help overlay or similar that doesn't exist.
const keybindBarText = "Panel: tab/h/l  Stage: space  Commit: c  Branch: n  Diff/Switch: enter  Discard: d  Quit: q"

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

	statusText := m.repoName
	if m.status.Branch != "" {
		statusText += " → " + m.status.Branch
	}
	statusPanel := unfocusedPanelStyle.Render("Status\n" + statusText)

	left := lipgloss.JoinVertical(lipgloss.Left,
		statusPanel,
		panelStyle(m.focus == focusFiles).Render("Files\n"+m.files.View()),
		panelStyle(m.focus == focusBranches).Render("Branches\n"+m.branches.View()),
		panelStyle(m.focus == focusHistory).Render("History\n"+m.history.View()),
	)
	right := panelStyle(m.focus == focusDiff).Render("Diff\n" + m.diff.vp.View())
	main := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	var footer string
	switch {
	case m.prompt == promptConfirmDiscard:
		footer = "Discard changes to " + m.pendingDiscardPath + "? (y/N)"
	case m.prompt != promptNone:
		footer = m.input.View()
	case m.err != nil:
		footer = errorStyle.Render(m.err.Error()) + "\n" + m.log.LastLines(footerHeight-1)
	default:
		footer = m.log.LastLines(footerHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, main, footer, keybindBarStyle.Render(keybindBarText))
}
