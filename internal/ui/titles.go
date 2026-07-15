package ui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// These helpers produce titled borders by post-processing a lipgloss
// bordered box render. The title (and optional jump number prefix) is
// drawn as part of the top border line itself, e.g.:
//
//	╭───[1]─Files────────────────────────────╮
//
// instead of rendering "Files" as the first interior content row. Border
// colors come from the focused/unfocused panel styles; the injected top
// line is recolored to keep the "frame" appearance consistent.
var (
	borderFocused   = lipgloss.Color("2") // green for focused
	borderUnfocused = lipgloss.Color("7") // white for inactive borders
)

// injectTitle replaces the top border of a rendered rounded box with a
// lazygit-style titled top border. num may be "" (no [N] prefix).
// paneWidth is the outer width including borders; we measure the actual
// rendered line for robustness.
func injectTitle(rendered, num, name string, paneWidth int, focused bool) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	measuredW := lipgloss.Width(lines[0])
	if measuredW > 0 {
		paneWidth = measuredW
	}

	// Frame (corners, dashes, [N]) uses the border color (green when panel focused)
	frameCol := borderUnfocused
	if focused {
		frameCol = borderFocused
	}
	frameStyle := lipgloss.NewStyle().Foreground(frameCol)
	if focused {
		frameStyle = frameStyle.Bold(true)
	}

	// Title name for single titles (Status, Files, History, Diff, etc.) is white.
	// Green is reserved exclusively to indicate the *selected tab* in dual/split titles
	// (e.g. the active one between "Local branches" and "Remotes").
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	if focused {
		nameStyle = nameStyle.Bold(true)
	}

	// Build the left frame part (includes left border char)
	leftFrame := "╭"
	namePrefix := "─"
	if num != "" {
		namePrefix = "─[" + num + "]─"
	}
	leftFrame += namePrefix

	// dash count so full top line matches target width (same logic as original)
	dashLabel := namePrefix + name
	innerW := paneWidth - 2
	dashCount := innerW - len([]rune(dashLabel))
	if dashCount < 0 {
		dashCount = 0
	}

	top := frameStyle.Render(leftFrame) +
		nameStyle.Render(name) +
		frameStyle.Render(strings.Repeat("─", dashCount)+"╮")

	lines[0] = top
	return strings.Join(lines, "\n")
}

// injectDualTitle renders a split tab-like title for the Branches panel
// matching lazygit: ╭─[3]─{green}Local branches{green} - {white}Remotes{white}──╮
// Active tab name is green, inactive white. Separator " - " per user spec.
func injectDualTitle(rendered, num, firstName, secondName string, firstActive bool, paneWidth int, focused bool) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	measuredW := lipgloss.Width(lines[0])
	if measuredW > 0 {
		paneWidth = measuredW
	}

	borderCol := borderUnfocused
	if focused {
		borderCol = borderFocused
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderCol)
	if focused {
		borderStyle = borderStyle.Bold(true)
	}

	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))   // green for active tab name
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // white

	prefix := "─[" + num + "]─ "
	separator := " - "
	firstStyle, secondStyle := activeStyle, inactiveStyle
	if !firstActive {
		firstStyle, secondStyle = inactiveStyle, activeStyle
	}

	plainLabel := prefix + firstName + separator + secondName
	innerW := paneWidth - 2
	dashCount := innerW - len([]rune(plainLabel))
	if dashCount < 0 {
		dashCount = 0
	}

	top := borderStyle.Render("╭"+prefix) +
		firstStyle.Render(firstName) +
		borderStyle.Render(separator) +
		secondStyle.Render(secondName) +
		borderStyle.Render(strings.Repeat("─", dashCount)+"╮")

	lines[0] = top
	return strings.Join(lines, "\n")
}

// renderTitledPanel renders a bordered panel whose title lives in the top
// border (via injectTitle), not as an interior content line, freeing the
// entire interior height for actual list/diff content. width/height are
// the values passed to lipgloss's Width/Height; the rendered box has
// visual size width+borderWidth by height+borderHeight.
func renderTitledPanel(focused bool, width, height int, num, title, content string) string {
	s := unfocusedPanelStyle
	if focused {
		s = focusedPanelStyle
	}
	// Render the content (lists, diff, or status text) with no title inside.
	// Lipgloss will add the rounded border around the given w/h.
	// Trim any leading newlines/blank lines so there is no extra top padding/gap inside the window.
	content = strings.TrimLeft(content, "\n\r")
	lines := strings.Split(content, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	content = strings.Join(lines, "\n")
	box := s.Width(width).Height(height).Render(content)
	// Overwrite the first line with a titled border, e.g. ╭─[1]─Files────╮
	return injectTitle(box, num, title, 0, focused)
}

// renderDualTitledPanel is like renderTitledPanel but uses a split tab title
// (e.g. for Branches: Local branches - Remotes). firstActive controls which
// name gets the green color.
func renderDualTitledPanel(focused bool, width, height int, num, firstName, secondName string, firstActive bool, content string) string {
	s := unfocusedPanelStyle
	if focused {
		s = focusedPanelStyle
	}
	// Trim any leading newlines/blank lines so there is no extra top padding/gap inside the window.
	content = strings.TrimLeft(content, "\n\r")
	lines := strings.Split(content, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	content = strings.Join(lines, "\n")
	box := s.Width(width).Height(height).Render(content)
	return injectDualTitle(box, num, firstName, secondName, firstActive, 0, focused)
}

// withBottomCount replaces the bottom border line of a rendered panel with one
// that includes the count (e.g. "6 of 12") right-aligned near the right corner.
// This lets us show the "1 of N" without costing an extra content row (full
// items fit, count lives in the border). focused sets the rebuilt line's
// border color/weight, matching injectTitle.
func withBottomCount(rendered, count string, focused bool) string {
	if count == "" {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}
	last := lines[len(lines)-1]
	w := lipgloss.Width(last)
	if w < 4 {
		return rendered
	}

	frameCol := borderUnfocused
	if focused {
		frameCol = borderFocused
	}
	frameStyle := lipgloss.NewStyle().Foreground(frameCol)
	if focused {
		frameStyle = frameStyle.Bold(true)
	}

	countW := lipgloss.Width(count)
	// Right-align count in the bottom border, with one `─` of padding on
	// both sides so it doesn't touch the preceding dashes or the corner:
	// ╰────────────-6 of 12─╯
	inner := w - 2
	leftDashesLen := inner - countW - 2
	if leftDashesLen < 0 {
		leftDashesLen = 0
	}
	build := func(dashes int) string {
		return frameStyle.Render("╰"+strings.Repeat("─", dashes)+"─") + frameStyle.Render(count) + frameStyle.Render("─╯")
	}
	bottom := build(leftDashesLen)
	// If too short (shouldn't happen), pad on right before ╯
	for lipgloss.Width(bottom) < w {
		leftDashesLen++
		bottom = build(leftDashesLen)
	}
	// If over, trim dashes
	for lipgloss.Width(bottom) > w && leftDashesLen > 0 {
		leftDashesLen--
		bottom = build(leftDashesLen)
	}
	lines[len(lines)-1] = bottom
	return strings.Join(lines, "\n")
}

// withScrollbar replaces a rendered panel's right border character with a
// thumb glyph ("▐") on whichever content rows the scroll thumb currently
// covers - matching lazygit's own scrollbar look: the thumb merges directly
// into the border column instead of floating in an extra column beside it.
// Rows without a thumb keep their normal border character untouched. A
// no-op when there's nothing to scroll (total <= height).
func withScrollbar(rendered string, start, total, height int, focused bool) string {
	if height <= 0 || total <= height {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) < height+2 { // top border + height content rows + bottom border
		return rendered
	}

	percent := 0.0
	if start > 0 {
		percent = float64(start) / float64(max(1, total-height))
	}
	thumbStart := int(percent * float64(height-1))
	// Ceiling (not floor) and a 2-row floor - a 1-row thumb on a tall panel
	// (e.g. height=10, total=92) is barely visible against the track.
	thumbSize := max(2, int(math.Ceil(float64(height*height)/float64(total))))
	if thumbSize > height {
		thumbSize = height
	}

	frameCol := borderUnfocused
	if focused {
		frameCol = borderFocused
	}
	frameStyle := lipgloss.NewStyle().Foreground(frameCol)
	if focused {
		frameStyle = frameStyle.Bold(true)
	}

	// lines[0] is the top border, lines[len-1] the bottom border - content
	// rows are everything between, one row per index into [0, height).
	// MaxWidth (ANSI-aware) drops exactly the trailing border cell without
	// disturbing any color codes earlier in the line, so the thumb can take
	// its place as a same-width, same-position replacement.
	for row := thumbStart; row < thumbStart+thumbSize && row < height; row++ {
		i := row + 1
		w := lipgloss.Width(lines[i])
		if w == 0 {
			continue
		}
		trimmed := lipgloss.NewStyle().MaxWidth(w - 1).Render(lines[i])
		lines[i] = trimmed + frameStyle.Render("▐")
	}
	return strings.Join(lines, "\n")
}
