package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// popupWidth matches LazyGit's bounded popup width, excluding the frame.
func (m Model) popupWidth(maxWidth int) int {
	terminalWidth := m.width
	if terminalWidth <= 0 {
		terminalWidth = 80
	}
	outerWidth := min(4*terminalWidth/7, maxWidth)
	if outerWidth < 80 {
		outerWidth = min(terminalWidth-2, 80)
	}
	return max(1, outerWidth-borderWidth)
}

func (m Model) renderModal(title, body string) string {
	width := m.popupWidth(80)
	body = ansi.Wrap(body, width, "")
	if m.height > 0 {
		bodyLines := strings.Split(body, "\n")
		bodyHeight := max(1, 3*m.height/4-borderHeight)
		if len(bodyLines) > bodyHeight {
			bodyLines = bodyLines[:bodyHeight]
			bodyLines[bodyHeight-1] = ansi.Truncate(bodyLines[bodyHeight-1], width-1, "") + "…"
			body = strings.Join(bodyLines, "\n")
		}
	}
	return renderTitledPanel(true, width, lipgloss.Height(body), "", title, body)
}

// renderPromptModal renders whichever prompt is currently open (text input
// or y/N confirmation) as a popup, replacing the old bottom-of-screen
// footer line for these - matching lazygit/lazyp4's own modal treatment of
// commit messages and confirmations instead of a cramped single line.
func (m Model) renderPromptModal() string {
	switch m.prompt {
	case promptCommit:
		return m.renderCommitModal()
	case promptConfirmStageAllForCommit:
		return m.renderModal("No files staged", "You have not staged any files. Commit all files?")
	case promptNewBranch:
		title := "New branch name"
		if m.status.Branch != "" {
			title += " (branch is off of '" + singleLineDisplay(m.status.Branch) + "')"
		}
		return m.renderInputModal(title, false)
	case promptDiscardMenu:
		return m.renderDiscardMenuModal()
	case promptConfirmDiscardAll:
		return m.renderModal("Discard all changes", "Discard ALL changes in the working tree?")
	case promptConfirmBranchReset:
		return m.renderModal("Reset branch", m.pendingResetLabel+"?")
	case promptConfirmBranchMerge:
		return m.renderModal("Merge branch", m.pendingMergeLabel+"?")
	case promptConfirmRevert:
		return m.renderModal("Drop revision", m.pendingResetLabel+"?")
	case promptConfirmForceUnlock:
		owner := m.locks[m.pendingForceUnlockPath].Owner
		return m.renderModal("Force unlock", "Force-unlock "+owner+"'s lock on "+m.pendingForceUnlockPath+"?")
	default:
		return ""
	}
}

// renderCommitModal renders the commit-message prompt as a titled border
// box (title + live char count baked into the top border, like every other
// panel - see titles.go) instead of a bold inline title line, matching
// lazygit's own "Commit summary" box.
func (m Model) renderCommitModal() string {
	return m.renderInputModal("Commit summary", true)
}

func (m Model) renderInputModal(title string, count bool) string {
	width := m.popupWidth(80)
	// Textinput renders a cursor cell in addition to its configured width.
	m.input.Width = max(1, width-lipgloss.Width(m.input.Prompt)-1)
	m.input.SetCursor(m.input.Position())
	box := focusedPanelStyle.Width(width).Height(1).Render(m.input.View())
	box = injectTitle(box, "", title, 0, true)
	if count {
		box = withTopRightCount(box, fmt.Sprintf("%d", len([]rune(m.input.Value()))), true)
	}
	return box
}

// renderDiscardMenuModal renders the "d" discard menu (x = discard all,
// u = discard unstaged). "Discard unstaged" only means anything for a
// directory with both staged and unstaged files under it - lore stages a
// file as a whole, so a single file is never "mixed" and the option stays
// struck through for it.
func (m Model) renderDiscardMenuModal() string {
	width := m.popupWidth(90)
	disabled := !m.pendingDiscardIsDir || !m.pendingDiscardDirMixed
	labels := [3]string{"Discard all changes", "Discard unstaged changes", "Cancel"}
	keys := [3]string{"x", "u", " "}
	rows := make([]string, len(labels))
	for i, label := range labels {
		style := lipgloss.NewStyle().Width(width)
		if i == m.discardCursor {
			style = cursorRowStyle(true, width)
		}
		textStyle := style.UnsetWidth()
		separator := textStyle.Render(" ")
		keyStyle := textStyle.Foreground(lipgloss.Color("6"))
		if i == 1 && disabled {
			textStyle = textStyle.Strikethrough(true)
		}
		row := keyStyle.Render(keys[i]) + separator + textStyle.Render(label)
		rows[i] = renderStyledRow(style, ansi.Truncate(row, width, "…"))
	}
	menu := renderTitledPanel(true, width, 3, "", "Discard changes", strings.Join(rows, "\n"))
	menu = withBottomCount(menu, fmt.Sprintf("%d of 3", m.discardCursor+1), true)

	description := ""
	path := singleLineDisplay(m.pendingDiscardPath)
	switch m.discardCursor {
	case 0:
		description = "Discard both staged and unstaged changes in '" + path + "'."
	case 1:
		description = "Discard unstaged changes in '" + path + "'."
	}
	// Leave the menu and footer visible even for paths spanning many lines.
	bodyHeight := 20
	if m.height > 0 {
		bodyHeight = max(1, m.height-8)
	}
	var reason []string
	if m.discardCursor == 1 && disabled {
		text := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Disabled: ") +
			"The selected items don't have both staged and unstaged changes."
		reason = strings.Split(ansi.Wrap(text, width, ""), "\n")
		if len(reason) > bodyHeight-1 {
			reason = reason[:max(0, bodyHeight-1)]
		}
	}
	lines := strings.Split(ansi.Wrap(description, width, ""), "\n")
	limit := max(1, bodyHeight-len(reason))
	if len(reason) > 0 && limit > 1 {
		limit--
	}
	if len(lines) > limit {
		lines = lines[:limit]
		lines[limit-1] = ansi.Truncate(lines[limit-1], width-1, "") + "…"
	}
	if len(reason) > 0 {
		if len(lines)+len(reason) < bodyHeight {
			lines = append(lines, "")
		}
		lines = append(lines, reason...)
	}
	tooltip := unfocusedPanelStyle.Width(width).Render(strings.Join(lines, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, menu, tooltip)
}

// helpRow is one line of the keybindings overlay: either a section header
// (Local/Global), a key/description pair, or a blank spacer. Blank is its
// own row (not synthesized at render time) so every row costs exactly one
// rendered line - the scroll math (scrollWindowStart, shared with the list
// panels) assumes that 1:1 mapping between row index and screen line.
type helpRow struct {
	key, desc string
	binding   string
	section   bool
	blank     bool
}

// localHelpRows returns the current panel's own bindings, describing the
// same keys as keybindBarFor (see view.go) but in full sentences and
// including keys intentionally left off that compact bar (Enter).
func (m Model) localHelpRows() []helpRow {
	switch m.focus {
	case focusFiles:
		return []helpRow{
			{key: "space", binding: " ", desc: "Stage / unstage selected file, or a whole folder recursively"},
			{key: "a", binding: "a", desc: "Stage / unstage everything"},
			{key: "enter", binding: "enter", desc: "Expand/collapse folder, or show the selected file's diff"},
			{key: "c", binding: "c", desc: "Commit staged changes"},
			{key: "e", binding: "e", desc: "Edit file in $VISUAL/$EDITOR"},
			{key: "d", binding: "d", desc: "Discard changes to selected file/folder"},
			{key: "D", binding: "D", desc: "Discard ALL changes"},
			{key: "L", binding: "L", desc: "Toggle file lock"},
		}
	case focusBranches:
		return []helpRow{
			{key: "space", binding: " ", desc: "Checkout selected branch"},
			{key: "n", binding: "n", desc: "Create new branch"},
			{key: "M", binding: "M", desc: "Merge selected branch into the current one"},
			{key: "g", binding: "g", desc: "Reset current branch to selected branch"},
		}
	case focusHistory:
		return []helpRow{
			{key: "space", binding: " ", desc: "Checkout selected revision"},
			{key: "d", binding: "d", desc: "Drop (revert) selected revision"},
			{key: "g", binding: "g", desc: "Reset current branch to selected revision"},
		}
	default:
		return nil
	}
}

// globalHelpRows returns bindings that work regardless of which panel is
// focused (see handleKey in keys.go).
func globalHelpRows() []helpRow {
	return []helpRow{
		{key: "p", binding: "p", desc: "Pull (sync to latest remote)"},
		{key: "P", binding: "P", desc: "Push current branch"},
		{key: "/", binding: "/", desc: "Filter list"},
		{key: "v", binding: "v", desc: "Select mode (release mouse to copy text)"},
		{key: "q", binding: "q", desc: "Quit"},
		{key: "?, esc", binding: "esc", desc: "Close this help"},
	}
}

func navigationHelpRows() []helpRow {
	rows := []helpRow{
		{key: "tab / l", binding: "tab", desc: "Next panel"},
		{key: "shift+tab / h", binding: "shift+tab", desc: "Previous panel"},
		{key: "[ / ]", binding: "]", desc: "Cycle panel sub-tabs (Branches: Local/Remotes)"},
		{key: "↑ / k", binding: "up", desc: "Previous item"},
		{key: "↓ / j", binding: "down", desc: "Next item"},
	}
	for i, name := range []string{"Status", "Files", "Branches", "History", "Diff", "Command log"} {
		key := fmt.Sprintf("%d", i+1)
		rows = append(rows, helpRow{key: key, binding: key, desc: "Go to " + name + " panel"})
	}
	return rows
}

// buildHelpRows combines the focused panel's own bindings ("Local") with
// everything else ("Global") into one flat row list for the keybindings
// overlay - lazylore's answer to lazygit falling back on "?" for a full
// list when a binding isn't in the bottom bar.
func (m Model) buildHelpRows() []helpRow {
	var rows []helpRow
	if local := m.localHelpRows(); len(local) > 0 {
		rows = append(rows, helpRow{key: "Local", section: true})
		rows = append(rows, local...)
	}
	if len(rows) > 0 {
		rows = append(rows, helpRow{blank: true})
	}
	rows = append(rows, helpRow{key: "Global", section: true})
	rows = append(rows, globalHelpRows()...)
	rows = append(rows, helpRow{blank: true}, helpRow{key: "Navigation", section: true})
	rows = append(rows, navigationHelpRows()...)
	return rows
}

// firstSelectable, nextSelectable, and prevSelectable move a cursor between
// helpRows entries while skipping section headers and blank spacers, which
// are never selectable themselves.
func firstSelectable(rows []helpRow) int {
	for i, r := range rows {
		if !r.section && !r.blank {
			return i
		}
	}
	return 0
}

func nextSelectable(rows []helpRow, from int) int {
	for i := from + 1; i < len(rows); i++ {
		if !rows[i].section && !rows[i].blank {
			return i
		}
	}
	return from
}

func prevSelectable(rows []helpRow, from int) int {
	for i := from - 1; i >= 0; i-- {
		if !rows[i].section && !rows[i].blank {
			return i
		}
	}
	return from
}

// selectableRank returns cursor's 1-based position among the selectable
// rows and the total selectable count, for the "N of M" bottom-border label.
func selectableRank(rows []helpRow, cursor int) (rank, total int) {
	for i, r := range rows {
		if r.section || r.blank {
			continue
		}
		total++
		if i == cursor {
			rank = total
		}
	}
	return rank, total
}

// renderHelpModal renders the "?" keybindings overlay as a selectable,
// scrollable list (see openHelp/helpCursor) inside a titled border box,
// matching lazygit's own keybindings popup.
func (m Model) renderHelpModal() string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	hdrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)

	// Right-align keys against a shared column so ragged key lengths (e.g.
	// "y" vs "shift+tab / h") all end at the same point, with the section
	// header's "─── Local" starting where descriptions do - lazygit's own
	// layout for this popup.
	keyColWidth := 0
	for _, r := range m.helpRows {
		if !r.section {
			keyColWidth = max(keyColWidth, lipgloss.Width(r.key))
		}
	}

	start := scrollWindowStart(m.helpCursor, len(m.helpRows), m.helpHeight)
	end := min(len(m.helpRows), start+m.helpHeight)

	var b strings.Builder
	if len(m.helpRows) == 0 {
		b.WriteString("No matching keybindings")
	}
	for i := start; i < end; i++ {
		r := m.helpRows[i]
		switch {
		case r.blank:
			// Nothing to write - just the newline below.
		case r.section:
			line := strings.Repeat(" ", keyColWidth+2) + fmt.Sprintf("─── %s", r.key)
			b.WriteString(hdrStyle.Render(line))
		default:
			rowStyle := lipgloss.NewStyle().Width(m.helpWidth)
			if i == m.helpCursor {
				rowStyle = selectedRowStyle(m.helpWidth)
			}
			segment := rowStyle.UnsetWidth()
			key := segment.Foreground(keyStyle.GetForeground()).Render(fmt.Sprintf("%*s", keyColWidth, r.key))
			line := key + segment.Render("  "+r.desc)
			b.WriteString(renderStyledRow(rowStyle, ansi.Truncate(line, m.helpWidth, "…")))
		}
		if i != end-1 {
			b.WriteByte('\n')
		}
	}

	box := renderTitledPanel(true, m.helpWidth, m.helpHeight, "", "Keybindings", b.String())
	if m.helpInput.Focused() {
		box = withTopRightCount(box, m.helpInput.View(), true)
	} else {
		box = withTopRightCount(box, "(Type to filter; @ for keys)", true)
	}
	box = withScrollbar(box, start, len(m.helpRows), m.helpHeight, true)
	rank, total := selectableRank(m.helpRows, m.helpCursor)
	box = withBottomCount(box, fmt.Sprintf("%d of %d", rank, total), true)
	return box
}
