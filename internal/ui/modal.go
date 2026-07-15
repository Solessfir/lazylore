package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// One universal centered-popup system (see overlay.go) backs every prompt,
// confirmation, and the keybindings overlay through a single style.
var (
	modalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("2")).
			Padding(1, 2)
	modalTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	modalHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// renderModal builds a centered popup box: a bold title line, a blank line,
// the body, and (if given) a blank line followed by a dim hint line.
func renderModal(title, body, hint string) string {
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
		return renderModal("No files staged", "You have not staged any files. Commit all files?", "y - confirm   n / esc - cancel")
	case promptNewBranch:
		return renderModal("New Branch", m.input.View(), "enter - create   esc - cancel")
	case promptDiscardMenu:
		return m.renderDiscardMenuModal()
	case promptConfirmDiscardAll:
		return renderModal("Discard All Changes", "Discard ALL changes in the working tree?", "y - confirm   n / esc - cancel")
	case promptConfirmBranchReset:
		return renderModal("Reset Branch", m.pendingResetLabel+"?", "y - confirm   n / esc - cancel")
	case promptConfirmRevert:
		return renderModal("Drop Revision", m.pendingResetLabel+"?", "y - confirm   n / esc - cancel")
	case promptConfirmForceUnlock:
		owner := m.locks[m.pendingForceUnlockPath].Owner
		return renderModal("Force Unlock", "Force-unlock "+owner+"'s lock on "+m.pendingForceUnlockPath+"?", "y - confirm   n / esc - cancel")
	default:
		return ""
	}
}

// renderCommitModal renders the commit-message prompt as a titled border
// box (title + live char count baked into the top border, like every other
// panel - see titles.go) instead of a bold inline title line, matching
// lazygit's own "Commit summary" box.
func (m Model) renderCommitModal() string {
	box := focusedPanelStyle.Width(m.input.Width).Height(1).Render(m.input.View())
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
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Strikethrough(true)

	body := "Discard all changes in " + m.pendingDiscardPath
	unstagedLine := "u - Discard unstaged changes"
	if !m.pendingDiscardIsDir || !m.pendingDiscardDirMixed {
		unstagedLine = dim.Render(unstagedLine)
	}

	hint := "x - Discard all changes\n" + unstagedLine + "\nesc - cancel"
	return renderModal("Discard Changes", body, hint)
}

// helpRow is one line of the keybindings overlay: either a section header
// (Local/Global) or a key/description pair.
type helpRow struct {
	key, desc string
	section   bool
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
			{key: "d", desc: "Discard changes to selected file/folder (x - all, u - unstaged only)"},
			{key: "D", desc: "Discard ALL changes"},
			{key: "L", desc: "Toggle file lock"},
		}
	case focusBranches:
		return []helpRow{
			{key: "space", desc: "Checkout selected branch"},
			{key: "n", desc: "Create new branch"},
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
		{key: "1-6", desc: "Jump to panel (1 Status, 2 Files, 3 Branches, 4 History, 5 Diff, 6 Command Log)"},
		{key: "[ / ]", desc: "Cycle panel sub-tabs (Branches: Local/Remotes)"},
		{key: "p", desc: "Pull (sync to latest remote)"},
		{key: "P", desc: "Push current branch"},
		{key: "/", desc: "Filter list"},
		{key: "v", desc: "Select mode (release mouse to copy text)"},
		{key: "q", desc: "Quit"},
		{key: "j/k / ↑/↓", desc: "Scroll this help (or mouse wheel)"},
		{key: "?, esc", desc: "Close this help"},
	}
}

// helpContent renders the full keybindings overlay body: the focused
// panel's own bindings under "Local", then everything else under "Global" -
// lazylore's answer to lazygit falling back on "?" for a full list when a
// binding isn't in the bottom bar.
func (m Model) helpContent() string {
	key := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true) // blue
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))            // white
	hdr := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true) // green

	var rows []helpRow
	if local := m.localHelpRows(); len(local) > 0 {
		rows = append(rows, helpRow{key: "Local", section: true})
		rows = append(rows, local...)
	}
	rows = append(rows, helpRow{key: "Global", section: true})
	rows = append(rows, globalHelpRows()...)

	var sb strings.Builder
	for i, r := range rows {
		if r.section {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(hdr.Render(fmt.Sprintf("── %s ──", r.key)))
		} else {
			sb.WriteString(fmt.Sprintf("  %s  %s", key.Render(fmt.Sprintf("%-14s", r.key)), dim.Render(r.desc)))
		}
		if i < len(rows)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// renderHelpModal renders the "?" keybindings overlay as a popup: a titled
// bordered box (title embedded in the top border, like every other panel -
// see titles.go's renderTitledPanel) wrapping a scrollable viewport, rather
// than the plain title-as-first-line box the other prompts use.
func (m Model) renderHelpModal() string {
	return renderTitledPanel(true, m.helpViewport.Width, m.helpViewport.Height, "", "Keybindings", m.helpViewport.View())
}
