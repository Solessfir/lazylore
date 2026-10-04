package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// One universal centered-popup system (see overlay.go) backs every prompt,
// confirmation, and the keybindings overlay through a single style.
var (
	modalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderFocused).
			Padding(1, 2)
	modalTitleStyle = lipgloss.NewStyle().Foreground(borderFocused)
	modalHintStyle  = lipgloss.NewStyle().Foreground(borderFocused)
)

// renderModal builds a centered popup with a title, body, and optional hints.
func (m Model) renderModal(title, body, hint string) string {
	width := 60
	if m.width > 0 {
		width = max(1, m.width-modalBoxStyle.GetHorizontalFrameSize())
	}
	title = ansi.Wrap(title, width, "")
	body = ansi.Wrap(body, width, "")
	hint = ansi.Wrap(hint, width, "")
	if m.height > 0 {
		reserved := modalBoxStyle.GetVerticalFrameSize() + lipgloss.Height(title) + 2
		if hint != "" {
			reserved += 2 + lipgloss.Height(hint)
		}
		bodyLines := strings.Split(body, "\n")
		bodyHeight := max(1, m.height-reserved)
		if len(bodyLines) > bodyHeight {
			bodyLines = bodyLines[:bodyHeight]
			bodyLines[bodyHeight-1] = ansi.Truncate(bodyLines[bodyHeight-1], width-1, "") + "…"
			body = strings.Join(bodyLines, "\n")
		}
	}
	content := modalTitleStyle.Render(title) + "\n\n" + body
	if hint != "" {
		content += "\n\n" + modalHintStyle.Render(hint)
	}
	return modalBoxStyle.Render(content)
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
		return m.renderModal("No files staged", "You have not staged any files. Commit all files?", "y - confirm   n / esc - cancel")
	case promptNewBranch:
		if m.width > 0 {
			m.input.Width = max(1, min(m.input.Width, m.width-modalBoxStyle.GetHorizontalFrameSize()-lipgloss.Width(m.input.Prompt)-1))
			m.input.SetCursor(m.input.Position())
		}
		return m.renderModal("New Branch", m.input.View(), "enter - create   esc - cancel")
	case promptDiscardMenu:
		return m.renderDiscardMenuModal()
	case promptConfirmDiscardAll:
		return m.renderModal("Discard All Changes", "Discard ALL changes in the working tree?", "y - confirm   n / esc - cancel")
	case promptConfirmBranchReset:
		return m.renderModal("Reset Branch", m.pendingResetLabel+"?", "y - confirm   n / esc - cancel")
	case promptConfirmBranchMerge:
		return m.renderModal("Merge Branch", m.pendingMergeLabel+"?", "y - confirm   n / esc - cancel")
	case promptConfirmRevert:
		return m.renderModal("Drop Revision", m.pendingResetLabel+"?", "y - confirm   n / esc - cancel")
	case promptConfirmForceUnlock:
		owner := m.locks[m.pendingForceUnlockPath].Owner
		return m.renderModal("Force Unlock", "Force-unlock "+owner+"'s lock on "+m.pendingForceUnlockPath+"?", "y - confirm   n / esc - cancel")
	default:
		return ""
	}
}

// renderCommitModal renders the commit-message prompt as a titled border
// box (title + live char count baked into the top border, like every other
// panel - see titles.go) instead of a bold inline title line, matching
// lazygit's own "Commit summary" box.
func (m Model) renderCommitModal() string {
	width := m.input.Width
	if m.width > 0 {
		width = max(1, min(width, m.width-borderWidth))
	}
	// Textinput renders a cursor cell in addition to its configured width.
	m.input.Width = max(1, width-1)
	m.input.SetCursor(m.input.Position())
	box := focusedPanelStyle.Width(width).Height(1).Render(m.input.View())
	box = injectTitle(box, "", "Commit summary", 0, true)
	box = withTopRightCount(box, fmt.Sprintf("%d", len([]rune(m.input.Value()))), true)
	return box
}

// renderDiscardMenuModal renders the "d" discard menu (x = discard all,
// u = discard unstaged). "Discard unstaged" only means anything for a
// directory with both staged and unstaged files under it - lore stages a
// file as a whole, so a single file is never "mixed" and the option stays
// struck through for it.
func (m Model) renderDiscardMenuModal() string {
	dim := lipgloss.NewStyle().Strikethrough(true)

	body := "Discard all changes in " + m.pendingDiscardPath
	unstagedLine := "u - Discard unstaged changes"
	if !m.pendingDiscardIsDir || !m.pendingDiscardDirMixed {
		unstagedLine = dim.Render(unstagedLine)
	}

	hint := "x - Discard all changes\n" + unstagedLine + "\nesc - cancel"
	return m.renderModal("Discard Changes", body, hint)
}

// helpRow is one line of the keybindings overlay: either a section header
// (Local/Global), a key/description pair, or a blank spacer. Blank is its
// own row (not synthesized at render time) so every row costs exactly one
// rendered line - the scroll math (scrollWindowStart, shared with the list
// panels) assumes that 1:1 mapping between row index and screen line.
type helpRow struct {
	key, desc string
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
			{key: "space", desc: "Stage / unstage selected file, or a whole folder recursively"},
			{key: "a", desc: "Stage / unstage everything"},
			{key: "enter", desc: "Expand/collapse folder, or show the selected file's diff"},
			{key: "c", desc: "Commit staged changes"},
			{key: "e", desc: "Edit file in $VISUAL/$EDITOR"},
			{key: "d", desc: "Discard changes to selected file/folder"},
			{key: "D", desc: "Discard ALL changes"},
			{key: "L", desc: "Toggle file lock"},
		}
	case focusBranches:
		return []helpRow{
			{key: "space", desc: "Checkout selected branch"},
			{key: "n", desc: "Create new branch"},
			{key: "M", desc: "Merge selected branch into the current one"},
			{key: "g", desc: "Reset current branch to selected branch"},
		}
	case focusHistory:
		return []helpRow{
			{key: "space", desc: "Checkout selected revision"},
			{key: "d", desc: "Drop (revert) selected revision"},
			{key: "g", desc: "Reset current branch to selected revision"},
		}
	default:
		return nil
	}
}

// globalHelpRows returns bindings that work regardless of which panel is
// focused (see handleKey in keys.go).
func globalHelpRows() []helpRow {
	return []helpRow{
		{key: "tab / l", desc: "Next panel"},
		{key: "shift+tab / h", desc: "Previous panel"},
		{key: "1-6", desc: "Jump to panel"},
		{key: "[ / ]", desc: "Cycle panel sub-tabs (Branches: Local/Remotes)"},
		{key: "p", desc: "Pull (sync to latest remote)"},
		{key: "P", desc: "Push current branch"},
		{key: "/", desc: "Filter list"},
		{key: "v", desc: "Select mode (release mouse to copy text)"},
		{key: "q", desc: "Quit"},
		{key: "j/k / ↑/↓", desc: "Move selection in this help (or mouse wheel)"},
		{key: "?, esc", desc: "Close this help"},
	}
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
	dimStyle := lipgloss.NewStyle()
	hdrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)

	// Right-align keys against a shared column so ragged key lengths (e.g.
	// "y" vs "shift+tab / h") all end at the same point, with the section
	// header's "── Local ──" starting where descriptions do - lazygit's own
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
	for i := start; i < end; i++ {
		r := m.helpRows[i]
		switch {
		case r.blank:
			// Nothing to write - just the newline below.
		case r.section:
			line := strings.Repeat(" ", keyColWidth+2) + fmt.Sprintf("── %s ──", r.key)
			b.WriteString(hdrStyle.Render(line))
		case i == m.helpCursor:
			line := fmt.Sprintf("%*s  %s", keyColWidth, r.key, r.desc)
			b.WriteString(selectedRowStyle(m.helpWidth).Render(lipgloss.NewStyle().MaxWidth(m.helpWidth).Render(line)))
		default:
			line := fmt.Sprintf("%s  %s", keyStyle.Render(fmt.Sprintf("%*s", keyColWidth, r.key)), dimStyle.Render(r.desc))
			b.WriteString(lipgloss.NewStyle().MaxWidth(m.helpWidth).Render(line))
		}
		if i != end-1 {
			b.WriteByte('\n')
		}
	}

	box := renderTitledPanel(true, m.helpWidth, m.helpHeight, "", "Keybindings", b.String())
	box = withScrollbar(box, start, len(m.helpRows), m.helpHeight, true)
	if rank, total := selectableRank(m.helpRows, m.helpCursor); total > 0 {
		box = withBottomCount(box, fmt.Sprintf("%d of %d", rank, total), true)
	}
	return box
}
