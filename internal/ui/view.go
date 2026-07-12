package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors match lazygit's actual default theme (pkg/config/user_config.go):
// ActiveBorderColor "green bold", InactiveBorderColor "default",
// OptionsTextColor "blue", UnstagedChangesColor "red" (reused here for
// errors). Basic 16-color ANSI codes only (0-15) - the one palette every
// terminal renders correctly, unlike 256-color/TrueColor codes which
// depend on terminal capability detection going right.
var (
	focusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("2")).
				Padding(0) // no extra inner padding (reduces top padding vs lazygit)
	unfocusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("7")). // white like lazygit inactive
				Padding(0)
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	keybindBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Height(1).Padding(0)
)

// bracketedKey wraps a named (non-printable) key in angle brackets, matching
// lazygit's own key-label convention (docs/keybindings/Keybindings_en.md:
// "<space>", "<enter>" vs bare "c", "d", "D" for literal characters).
func bracketedKey(k string) string {
	switch k {
	case "space", "enter", "esc", "tab", "shift+tab":
		return "<" + k + ">"
	default:
		return k
	}
}

// keybindBarFor returns the keybinding legend for the currently focused
// panel, pinned to the bottom of the screen. This mirrors lazygit's own
// bottom option bar (pkg/gui/options_map.go: renderContextOptionsMap),
// which shows only the current context's "DisplayOnScreen" bindings - not
// a single static global list. Kept to keys handleKey/handlePromptKey
// actually implement for that panel.
func keybindBarFor(focus focusPanel) string {
	switch focus {
	case focusFiles:
		return "Stage: " + bracketedKey("space") + " | Commit: c | Edit: e | Discard: d | Reset: D | Lock: L"
	case focusBranches:
		return "Checkout: " + bracketedKey("space") + " | New branch: n | Reset: g"
	case focusHistory:
		return "Checkout: " + bracketedKey("space") + " | Drop: d | Reset: g"
	default:
		return ""
	}
}

// renderPanel is retained for tests that assert on explicit size behavior.
// It still puts the title inside as content (legacy path). New code uses
// renderTitledPanel (see titles.go) so titles live in the top border line.
func renderPanel(focused bool, width, height int, title, content string) string {
	s := unfocusedPanelStyle
	if focused {
		s = focusedPanelStyle
	}
	return s.Width(width).Height(height).Render(title + "\n" + content)
}

// renderTitledPanelForStatus is a convenience for the always-unfocused
// compact status panel.
func renderTitledPanelForStatus(width, height int, content string) string {
	// Status is informational and never the "focused" panel in this UI.
	// Jump key 1 (and 2) target the Files panel below it, so no [N] here.
	return renderTitledPanel(false, width, height, "", "Status", content)
}

// aheadBehindArrows renders lazygit's exact ahead/behind indicator
// (pkg/gui/presentation/branches.go's BranchStatus): "↓N↑N" when the branch
// diverges both ways, "↓N" behind only, "↑N" ahead only, "" when in sync or
// no remote comparison is available (offline, no remote, not pushed yet).
func aheadBehindArrows(ahead, behind int) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow, matches lazygit
	switch {
	case behind > 0 && ahead > 0:
		return style.Render(fmt.Sprintf("↓%d↑%d", behind, ahead)) + " "
	case behind > 0:
		return style.Render(fmt.Sprintf("↓%d", behind)) + " "
	case ahead > 0:
		return style.Render(fmt.Sprintf("↑%d", ahead)) + " "
	default:
		return ""
	}
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	statusText := aheadBehindArrows(m.status.AheadCount, m.status.BehindCount) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓ ") + m.repoName +
		" (" + lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(m.status.Branch) + ")"

	footer := m.currentFooter()

	// Compute actual footer lines so we can grow the panels to eliminate
	// artificial gap between panel bottoms and the keybind bar.
	footerLineCount := 0
	if footer != "" {
		footerLineCount = strings.Count(footer, "\n") + 1
	}
	actualBottom := footerLineCount + 1 // +1 for keybind
	reservedBottom := footerHeight + keybindBarHeight
	extra := reservedBottom - actualBottom
	if extra < 0 {
		extra = 0
	}

	baseBody := max(0, m.height-footerHeight-keybindBarHeight)

	// Grow using full distribute over (body + extra) so the fixed-size status
	// panel stays fixed and the weighted ones fairly split the extra. This
	// keeps exact sums so left stack height matches right column (diff + log)
	// height.
	mainAvail := baseBody + extra
	leftOuters := distributeSpace([]layoutBox{
		{Size: statusPanelHeight},
		{Weight: 1}, // Files
		{Weight: 1}, // Branches
		{Weight: 1}, // History
	}, mainAvail)
	effFilesH := max(0, leftOuters[1]-borderHeight)
	effBranchesH := max(0, leftOuters[2]-borderHeight)
	effHistoryH := max(0, leftOuters[3]-borderHeight)
	effDiffH := m.diffHeight + extra // all footer-saved growth to diff (log fixed size)

	// Re-size widgets for the effective (larger when prompt footer short) content area.
	// Command log height is fixed (see commandLogPanelHeight).
	m.files.SetSize(m.panelWidth, max(0, effFilesH))
	m.branches.SetSize(m.panelWidth, max(0, effBranchesH))
	m.history.SetSize(m.panelWidth, max(0, effHistoryH))
	m.diff.vp.Height = max(0, effDiffH)

	// Use titled-border rendering so "Status"/"Files" etc. appear in the top
	// border line itself (╭─[N]─Title────╮), matching lazygit.
	statusInnerH := max(0, statusPanelHeight-borderHeight)
	statusPanel := renderTitledPanelForStatus(m.panelWidth, statusInnerH, statusText)

	left := lipgloss.JoinVertical(lipgloss.Left,
		statusPanel,
		func() string {
			v := strings.TrimLeft(m.files.View(), "\n\r")
			p := renderTitledPanel(m.focus == focusFiles, m.panelWidth, effFilesH, "1", "Files", v)
			if m.filesTotal > 0 {
				cur := m.files.Index() + 1
				p = withBottomCount(p, fmt.Sprintf("%d of %d", cur, m.filesTotal), m.focus == focusFiles)
			}
			return p
		}(),
		func() string {
			v := strings.TrimLeft(m.branches.View(), "\n\r")
			p := renderDualTitledPanel(m.focus == focusBranches, m.panelWidth, effBranchesH, "3", "Local branches", "Remotes", !m.showRemoteBranches, v)
			if m.branchesTotal > 0 {
				cur := m.branches.Index() + 1
				p = withBottomCount(p, fmt.Sprintf("%d of %d", cur, m.branchesTotal), m.focus == focusBranches)
			}
			return p
		}(),
		func() string {
			v := strings.TrimLeft(m.history.View(), "\n\r")
			p := renderTitledPanel(m.focus == focusHistory, m.panelWidth, effHistoryH, "4", "History", v)
			if m.historyTotal > 0 {
				cur := m.history.Index() + 1
				p = withBottomCount(p, fmt.Sprintf("%d of %d", cur, m.historyTotal), m.focus == focusHistory)
			}
			return p
		}(),
	)

	// Right column: Diff on top, Command Log directly below it (matching lazygit
	// "extras" panel placement under the main content, not spanning full width).
	diffW := m.diff.vp.Width + 1
	diffPanel := renderTitledPanel(m.focus == focusDiff, diffW, effDiffH, "5", m.mainPanelTitle(), m.diff.viewWithScrollbar())

	logInnerH := max(0, commandLogPanelHeight-borderHeight)
	logContent := m.log.LastLines(logInnerH)
	logContent = strings.TrimLeft(logContent, "\n\r")
	// Blank-line trim inside titled panel too (for consistency with other panels).
	{
		lines := strings.Split(logContent, "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
			lines = lines[1:]
		}
		logContent = strings.Join(lines, "\n")
	}
	logPanel := renderTitledPanel(false, diffW, logInnerH, "", "Command Log", logContent)

	right := lipgloss.JoinVertical(lipgloss.Left, diffPanel, logPanel)
	main := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	keybindBarText := keybindBarFor(m.focus)
	keybindText := keybindBarText
	if m.selectMode {
		keybindText = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).
			Render("Select mode - highlight text in your terminal, then press any key to restore mouse")
	} else if m.appStatus != "" {
		spinners := []string{"/", "-", "\\", "|"}
		spin := spinners[m.spinner%4]
		statusPart := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(m.appStatus + " " + spin + " ")
		keybindText = statusPart + keybindBarText
	}
	keybind := keybindBarStyle.Render(keybindText)

	bottom := keybind
	if footer != "" {
		bottom = lipgloss.JoinVertical(lipgloss.Left, footer, keybind)
	}

	full := lipgloss.JoinVertical(lipgloss.Left, main, bottom)

	// Prompts, confirmations, and the "?" keybindings list render as a
	// centered popup on top of the full screen (see modal.go/overlay.go),
	// matching lazyp4's own modal treatment instead of a cramped footer line.
	if m.showHelp {
		return overlayCenter(m.renderHelpModal(), full, m.width, m.height)
	}
	if m.prompt != promptNone {
		return overlayCenter(m.renderPromptModal(), full, m.width, m.height)
	}

	return full
}
