package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	focusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderFocused).
				Padding(0)
	unfocusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderUnfocused).
				Padding(0)
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	keybindBarStyle = lipgloss.NewStyle().Foreground(borderFocused).Height(1).Padding(0)
)

// bracketedKey wraps a named (non-printable) key in angle brackets:
// "<space>", "<enter>" vs bare "c", "d", "D" for literal characters.
func bracketedKey(k string) string {
	switch k {
	case "space", "enter", "esc", "tab", "shift+tab":
		return "<" + k + ">"
	default:
		return k
	}
}

// keybindBarFor returns the keybinding legend for the currently focused
// panel, pinned to the bottom of the screen - only the keys handleKey/
// handlePromptKey actually implement for that panel. "Keybindings: ?" is
// appended last on every panel; truncateKeybindBar (below) drops it first
// on a narrow terminal.
func keybindBarFor(focus focusPanel) string {
	switch focus {
	case focusStatus:
		return "Keybindings: ?"
	case focusFiles:
		return "Stage: " + bracketedKey("space") + " | Commit: c | Edit: e | Discard: d | Reset: D | Lock: L | Keybindings: ?"
	case focusBranches:
		return "Checkout: " + bracketedKey("space") + " | New branch: n | Reset: g | Keybindings: ?"
	case focusHistory:
		return "Checkout: " + bracketedKey("space") + " | Drop: d | Reset: g | Keybindings: ?"
	default:
		return ""
	}
}

// Keep shortcuts whole, including when even the first entry cannot fit.
func truncateKeybindBar(bar string, width int) string {
	if width <= 0 {
		return ""
	}
	const sep = " | "
	const ellipsis = "…"
	entries := strings.Split(bar, sep)
	var b strings.Builder
	length := 0
	for i, e := range entries {
		textLen := lipgloss.Width(e)
		prefix := ""
		if i > 0 {
			prefix = sep
		}
		if length+lipgloss.Width(prefix)+textLen > width {
			if length+lipgloss.Width(prefix+ellipsis) <= width {
				b.WriteString(prefix + ellipsis)
			}
			break
		}
		if i > 0 {
			b.WriteString(sep)
			length += lipgloss.Width(sep)
		}
		b.WriteString(e)
		length += textLen
	}
	return b.String()
}

var busyFrames = [...]string{"●∙∙", "∙●∙", "∙∙●", "∙●∙"}

func (m Model) footerText(width int, shortcuts string) string {
	if width <= 0 {
		return ""
	}
	left := ""
	if name := m.activityName(); name != "" {
		dots := busyFrames[m.activityFrame%len(busyFrames)]
		if width >= lipgloss.Width(dots)+2 {
			left = ansi.Truncate(name, width-lipgloss.Width(dots)-1, "…") + " " + dots
		} else {
			left = ansi.Truncate(dots, width, "")
		}
		left = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(left)
	}
	remaining := width - lipgloss.Width(left)
	separator := ""
	if left != "" {
		separator = " | "
		remaining -= lipgloss.Width(separator)
	}
	right := truncateKeybindBar(shortcuts, remaining)
	if m.selectMode {
		right = lipgloss.NewStyle().Foreground(borderFocused).Render(
			ansi.Truncate("Select mode - highlight text in your terminal, then press any key to restore mouse", max(0, remaining), "…"))
	}
	if right == "" {
		return left
	}
	return left + separator + right
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

// renderTitledPanelForStatus is a convenience for the compact status panel,
// jumpable/focusable via "1" (see keys.go), matching lazygit's real
// numbering (Gui.SidePanels' default order: Status, Files, Branches,
// Commits, Stash).
func renderTitledPanelForStatus(focused bool, width, height int, content string) string {
	return renderTitledPanel(focused, width, height, "1", "Status", content)
}

// aheadBehindArrows renders lazygit's exact ahead/behind indicator
// (pkg/gui/presentation/branches.go's BranchStatus): "↓N↑N" when the branch
// diverges both ways, "↓N" behind only, "↑N" ahead only, "" when in sync or
// no remote comparison is available (offline, no remote, not pushed yet).
func aheadBehindArrows(ahead, behind int) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
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

func (m Model) layoutFits() bool {
	l := m.computeMouseLayout()
	bottom := keybindBarHeight
	if footer := m.currentFooter(); footer != "" {
		bottom += min(footerHeight, lipgloss.Height(footer))
	}
	available := m.height - bottom
	if m.panelWidth < 1 || m.diff.vp.Width < 1 ||
		l.effFilesH < 1 || l.effBranchesH < 1 || l.effHistoryH < 1 || l.effDiffH < 1 ||
		l.mainH > available || l.diffH+commandLogPanelHeight > available {
		return false
	}
	modal := ""
	if m.showHelp {
		modal = m.renderHelpModal()
	} else if m.prompt != promptNone {
		modal = m.renderPromptModal()
	}
	return modal == "" || lipgloss.Width(modal) <= m.width && lipgloss.Height(modal) <= m.height
}

func (m Model) smallTerminalView() string {
	if m.height <= 0 {
		return ""
	}
	width := max(1, m.width-1)
	if m.activityName() != "" {
		footer := m.footerText(width, "q: quit | ctrl+c: quit")
		if m.height == 1 {
			return footer
		}
		return ansi.Truncate("Terminal too small. Resize to show panels.", width, "…") + "\n" + footer
	}
	if m.height == 1 {
		return ansi.Truncate("q:quit - resize terminal", width, "")
	}
	return ansi.Truncate("Terminal too small. Resize to show panels.", width, "…") +
		"\n" + ansi.Truncate("q / ctrl+c - quit", width, "…")
}

func listPositionCount(items list.Model) string {
	total := len(items.VisibleItems())
	current := 0
	if total > 0 {
		current = items.Index() + 1
	}
	return fmt.Sprintf("%d of %d", current, total)
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}
	if !m.layoutFits() {
		return m.smallTerminalView()
	}

	statusText := aheadBehindArrows(m.status.AheadCount, m.status.BehindCount) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓ ") + m.repoName +
		" (" + lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(m.status.Branch) + ")"

	footer := m.currentFooter()
	if footer != "" {
		footerLines := strings.Split(footer, "\n")
		if len(footerLines) > footerHeight {
			footerLines = footerLines[:footerHeight]
			footerLines[footerHeight-1] = ansi.Truncate(footerLines[footerHeight-1], max(0, m.width-2), "") + "…"
		}
		for i, line := range footerLines {
			footerLines[i] = ansi.Truncate(line, m.width-1, "…")
		}
		footer = strings.Join(footerLines, "\n")
	}

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
	m.syncPanelSizes()

	// Use titled-border rendering so "Status"/"Files" etc. appear in the top
	// border line itself (╭─[N]─Title────╮), matching lazygit.
	statusInnerH := max(0, statusPanelHeight-borderHeight)
	statusPanel := renderTitledPanelForStatus(m.paneFocused(focusStatus), m.panelWidth, statusInnerH, statusText)

	left := lipgloss.JoinVertical(lipgloss.Left,
		statusPanel,
		func() string {
			v := ""
			switch {
			case len(m.files.Items()) == 0:
				// leave blank - see the SetShowStatusBar(false)-era "No items." fix
			case m.files.SettingFilter():
				// The filter-input row lives inside list.Model's own View();
				// renderListWindow doesn't account for it (see its doc comment).
				v = strings.TrimLeft(m.files.View(), "\n\r")
			default:
				v = renderListWindow(m.files, fileDelegate{focused: m.paneFocused(focusFiles)}, effFilesH, m.filesScrollOverride)
			}
			p := renderTitledPanel(m.paneFocused(focusFiles), m.panelWidth, effFilesH, "2", "Files", v)
			if !m.files.SettingFilter() {
				filesTotalItems := len(m.files.VisibleItems())
				start := effectiveScrollStart(m.filesScrollOverride, m.files.Index(), filesTotalItems, effFilesH)
				p = withScrollbar(p, start, filesTotalItems, effFilesH, m.paneFocused(focusFiles))
			}
			if len(m.files.Items()) > 0 {
				p = withBottomCount(p, listPositionCount(m.files), m.paneFocused(focusFiles))
			}
			return p
		}(),
		func() string {
			v := ""
			switch {
			case len(m.branches.Items()) == 0:
			case m.branches.SettingFilter():
				v = strings.TrimLeft(m.branches.View(), "\n\r")
			default:
				v = renderListWindow(m.branches, compactTitleDelegate{focused: m.paneFocused(focusBranches), width: m.panelWidth}, effBranchesH, m.branchesScrollOverride)
			}
			p := renderDualTitledPanel(m.paneFocused(focusBranches), m.panelWidth, effBranchesH, "3", "Local branches", "Remotes", !m.showRemoteBranches, v)
			if len(m.branches.Items()) > 0 {
				p = withBottomCount(p, listPositionCount(m.branches), m.paneFocused(focusBranches))
			}
			return p
		}(),
		func() string {
			v := ""
			switch {
			case len(m.history.Items()) == 0:
			case m.history.SettingFilter():
				v = strings.TrimLeft(m.history.View(), "\n\r")
			default:
				v = renderListWindow(m.history, compactTitleDelegate{focused: m.paneFocused(focusHistory), width: m.panelWidth}, effHistoryH, m.historyScrollOverride)
			}
			p := renderTitledPanel(m.paneFocused(focusHistory), m.panelWidth, effHistoryH, "4", "History", v)
			if len(m.history.Items()) > 0 {
				p = withBottomCount(p, listPositionCount(m.history), m.paneFocused(focusHistory))
			}
			return p
		}(),
	)

	// Right column: Diff on top, Command Log directly below it (matching lazygit
	// "extras" panel placement under the main content, not spanning full width).
	// Focusing Command Log expands it to take over Diff's space entirely,
	// matching lazygit's own extras-panel behavior - hiding rather than
	// merely resizing Diff, since there's nothing useful to show it shrunk.
	diffW := m.diff.vp.Width + 1
	logCommandLogFocused := m.focus == focusCommandLog

	logOuterH := commandLogPanelHeight
	if logCommandLogFocused {
		logOuterH = effDiffH + borderHeight + commandLogPanelHeight
	}
	logInnerH := max(0, logOuterH-borderHeight)
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
	logPanel := renderTitledPanel(m.paneFocused(focusCommandLog), diffW, logInnerH, "6", "Command Log", logContent)

	var right string
	if logCommandLogFocused {
		right = logPanel
	} else {
		diffPanel := renderTitledPanel(m.paneFocused(focusDiff), diffW, effDiffH, "5", m.mainPanelTitle(), m.diff.viewWithScrollbar())
		right = lipgloss.JoinVertical(lipgloss.Left, diffPanel, logPanel)
	}
	main := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	keybind := keybindBarStyle.Render(m.footerText(m.width-1, keybindBarFor(m.focus)))

	bottom := keybind
	if footer != "" {
		bottom = lipgloss.JoinVertical(lipgloss.Left, footer, keybind)
	}

	full := lipgloss.JoinVertical(lipgloss.Left, main, bottom)

	// Prompts, confirmations, and the "?" keybindings list render as a
	// centered popup on top of the full screen (see modal.go/overlay.go),
	// matching lazyp4's own modal treatment instead of a cramped footer line.
	modal := ""
	if m.showHelp {
		modal = m.renderHelpModal()
	} else if m.prompt != promptNone {
		modal = m.renderPromptModal()
	}
	if modal != "" {
		return overlayCenter(modal, full, m.width, m.height)
	}

	return full
}
