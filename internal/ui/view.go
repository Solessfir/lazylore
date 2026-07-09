package ui

import "github.com/charmbracelet/lipgloss"

// Colors match lazygit's actual default theme (pkg/config/user_config.go):
// ActiveBorderColor "green bold", InactiveBorderColor "default",
// OptionsTextColor "blue", UnstagedChangesColor "red" (reused here for
// errors). Basic 16-color ANSI codes only (0-15) - the one palette every
// terminal renders correctly, unlike 256-color/TrueColor codes which
// depend on terminal capability detection going right.
var (
	focusedPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("2")).Bold(true)
	unfocusedPanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	keybindBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
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

// renderPanel wraps title+content in a bordered box at an explicit
// width/height. list.Model.View() and viewport.Model.View() (and plain
// text, for the Status panel) only render their actual content lines -
// they don't pad to fill a configured size - so without an explicit
// Width/Height here, a panel with little content (e.g. one branch) shrinks
// its border to fit that content instead of matching its neighbors, and
// panels stop lining up with each other.
func renderPanel(focused bool, width, height int, title, content string) string {
	return panelStyle(focused).Width(width).Height(height).Render(title + "\n" + content)
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	statusText := m.repoName
	if m.status.Branch != "" {
		statusText += " → " + m.status.Branch
	}
	statusPanel := renderPanel(false, m.panelWidth, statusPanelHeight-borderHeight, "Status", statusText)

	left := lipgloss.JoinVertical(lipgloss.Left,
		statusPanel,
		renderPanel(m.focus == focusFiles, m.panelWidth, m.filesHeight, "Files", m.files.View()),
		renderPanel(m.focus == focusBranches, m.panelWidth, m.branchesHeight, "Branches", m.branches.View()),
		renderPanel(m.focus == focusHistory, m.panelWidth, m.historyHeight, "History", m.history.View()),
	)
	right := renderPanel(m.focus == focusDiff, m.diff.vp.Width, m.diffHeight, "Diff", m.diff.vp.View())
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
