package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazylore/internal/lore"
)

// A staged file's status letter and filename render green; an unstaged/
// untracked file's status letter renders red, filename left uncolored -
// the color alone conveys staged/unstaged, no text label.
var (
	fileStagedColor   = lipgloss.Color("2") // green
	fileUnstagedColor = lipgloss.Color("1") // red
)

// lockBadge marks a file with an active lore file lock. Plain ASCII rather
// than an emoji glyph - emoji column-width handling is inconsistent across
// terminals, which misaligns the badge.
const lockBadge = "[L]"

// lockBadgeStyle colors the badge by ownership: green for a lock you hold
// yourself (safe, informational), yellow for someone else's (the case that
// actually blocks you).
func lockBadgeStyle(lockedByMe bool) lipgloss.Style {
	if lockedByMe {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
}

// scrollMargin is how many rows of context past the cursor scrollWindowStart
// tries to keep visible ("scrolloff") - a couple of upcoming rows show
// before the window scrolls, rather than the cursor sitting on the last
// visible row.
const scrollMargin = 2

// scrollWindowStart returns the first visible index for a continuous
// scroll window of size height around cursor, clamped to [0, total-height].
// Shared by renderListWindow and rowClickTarget so rendering and click
// hit-testing can never disagree about what's on screen.
func scrollWindowStart(cursor, total, height int) int {
	if height <= 0 || total <= height {
		return 0
	}
	start := 0
	if cursor+scrollMargin >= height {
		start = cursor + scrollMargin - height + 1
	}
	if start > total-height {
		start = total - height
	}
	if start < 0 {
		start = 0
	}
	return start
}

// effectiveScrollStart resolves the window's actual top row: override when
// set (>= 0, meaning mouse-wheel scrolling has panned the view away from
// the cursor) and still valid for the current item count, otherwise the
// cursor-follow window (scrollWindowStart).
func effectiveScrollStart(override, cursor, total, height int) int {
	if height <= 0 || total <= height {
		return 0
	}
	if override >= 0 && override <= total-height {
		return override
	}
	return scrollWindowStart(cursor, total, height)
}

// renderListWindow renders m's visible items through delegate in a
// continuous scroll window (see effectiveScrollStart) instead of calling
// list.Model's own paginated View(). Only valid while m isn't actively
// showing its filter-input row - callers fall back to m.View() while
// typing a filter (see view.go).
func renderListWindow(m list.Model, delegate list.ItemDelegate, height, scrollOverride int) string {
	items := m.VisibleItems()
	if len(items) == 0 || height <= 0 {
		return ""
	}
	start := effectiveScrollStart(scrollOverride, m.Index(), len(items), height)
	end := min(len(items), start+height)

	var b strings.Builder
	for i := start; i < end; i++ {
		delegate.Render(&b, m, i, items[i])
		if i != end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func fileStatusColor(staged bool) lipgloss.Color {
	if staged {
		return fileStagedColor
	}
	return fileUnstagedColor
}

func fileNameStyle(staged bool) lipgloss.Style {
	if staged {
		return lipgloss.NewStyle().Foreground(fileStagedColor)
	}
	return lipgloss.NewStyle()
}

// fileItem is one row in the Files panel: either a directory node (isDir,
// toggled open/closed with Enter/Space) or a file leaf carrying the
// FileChange it represents. Rebuilt fresh from Status on every refresh by
// statusToItems; collapse state is carried separately on Model so it
// survives a refresh. label is the row's display text (see rowLabel).
type fileItem struct {
	path       string
	label      string
	isDir      bool
	depth      int
	change     lore.FileChange
	staged     bool
	allStaged  bool // only meaningful when isDir - see fileTreeNode.allStaged
	collapsed  bool // only meaningful when isDir
	locked     bool // only meaningful for files; lore lock held by anyone
	lockedByMe bool // only meaningful when locked is true
}

func (i fileItem) FilterValue() string { return i.path }

// fileDelegate renders Files list items itself, rather than going through
// list.DefaultDelegate's Title()/Description() two-line convention - lore's
// per-file status is conveyed by color (see fileStatusColor/fileNameStyle),
// not a text label, and each row (file or directory) is a single line.
type fileDelegate struct {
	focused bool
}

func (d fileDelegate) Height() int                         { return 1 }
func (d fileDelegate) Spacing() int                        { return 0 }
func (d fileDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

// selectedRowStyle is the highlight overlay for the row under the cursor:
// a background fill only, no Foreground override, so the row's own status
// coloring shows through underneath it. Width must be set explicitly to
// the list's own content width - without it the fill isn't bounded to this
// panel and bleeds across the rest of the terminal row.
func selectedRowStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().Background(selectedBg).Bold(true).Width(width)
}

func (d fileDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	fi, ok := listItem.(fileItem)
	if !ok {
		return
	}
	indent := strings.Repeat("  ", fi.depth)
	selected := d.focused && index == m.Index()

	if !selected {
		// ANSI-aware clip to the panel's own content width - a deeply
		// indented or long-labeled row would otherwise overflow past the
		// Files panel's right border into the panel beside it.
		clip := lipgloss.NewStyle().MaxWidth(m.Width())
		if fi.isDir {
			arrow := "▼"
			if fi.collapsed {
				arrow = "▶"
			}
			display := indent + arrow + " " + fi.label
			if fi.allStaged {
				display = fileNameStyle(true).Render(display)
			}
			fmt.Fprint(w, clip.Render(display))
			return
		}
		// File, not selected: apply per-part colors (status red/green, name color for staged)
		statusStyle := lipgloss.NewStyle().Foreground(fileStatusColor(fi.staged))
		nameStyle := fileNameStyle(fi.staged)
		line := indent + statusStyle.Render(string(fi.change.Status)) + " " + nameStyle.Render(fi.label)
		if fi.locked {
			line += " " + lockBadgeStyle(fi.lockedByMe).Render(lockBadge)
		}
		fmt.Fprint(w, clip.Render(line))
		return
	}

	// Selected row: we must include Background on the colored segments so the
	// blue selection shows behind file names (and status letters). Plain
	// concatenation of pre-styled segments + outer bg fails because inner
	// .Render() calls emit resets that kill the background.
	rowWidth := m.Width()

	if fi.isDir {
		arrow := "▼"
		if fi.collapsed {
			arrow = "▶"
		}
		style := selectedRowStyle(rowWidth)
		if fi.allStaged {
			style = style.Foreground(fileStagedColor)
		}
		display := indent + arrow + " " + fi.label
		fmt.Fprint(w, style.Render(display))
		return
	}

	// Selected file
	statColor := fileStatusColor(fi.staged)
	statStyle := lipgloss.NewStyle().
		Foreground(statColor).
		Background(selectedBg).
		Bold(true)

	nameStyle := fileNameStyle(fi.staged).
		Background(selectedBg).
		Bold(true)

	// The separator space must also carry the background, otherwise you get
	// "A{no-bg space}Filename" under selection.
	selSpace := lipgloss.NewStyle().Background(selectedBg).Bold(true)

	colored := selSpace.Render(indent) +
		statStyle.Render(string(fi.change.Status)) +
		selSpace.Render(" ") +
		nameStyle.Render(fi.label)
	if fi.locked {
		colored += selSpace.Render(" ") + lockBadgeStyle(fi.lockedByMe).Background(selectedBg).Render(lockBadge)
	}

	// selectedRowStyle ensures full-width background fill (including gutter area)
	fmt.Fprint(w, selectedRowStyle(rowWidth).Render(colored))
}

// statusToItems flattens a Status's changed files into a directory tree
// (see filetree.go), respecting which directories are currently collapsed.
// locks maps path -> held lock (see loadLocksCmd); nil is fine (no badges).
// currentUserID (see lore.CurrentUserID) decides lockedByMe; "" (not yet
// loaded, or unauthenticated) just means every lock shows as someone
// else's rather than guessing.
func statusToItems(s lore.Status, collapsedDirs map[string]bool, locks map[string]lore.Lock, currentUserID string) []list.Item {
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, collapsedDirs)

	items := make([]list.Item, 0, len(rows))
	for _, row := range rows {
		lock, locked := locks[row.node.path]
		lockedByMe := locked && currentUserID != "" && lock.Owner == currentUserID
		items = append(items, fileItem{
			path:       row.node.path,
			label:      row.label,
			isDir:      row.node.isDir,
			depth:      row.depth,
			change:     row.node.change,
			staged:     row.node.staged,
			allStaged:  row.node.allStaged,
			collapsed:  collapsedDirs[row.node.path],
			locked:     locked,
			lockedByMe: lockedByMe,
		})
	}
	return items
}

type branchItem struct {
	branch lore.Branch
}

func (i branchItem) Title() string {
	// Structural title for filtering / list model. Visual rendering
	// (recency, colors, * / ✓ ) happens in compactTitleDelegate.
	return i.branch.Name
}

func (i branchItem) Description() string { return "" }

func (i branchItem) FilterValue() string { return i.branch.Name }

func branchesToItems(branches []lore.Branch) []list.Item {
	items := make([]list.Item, 0, len(branches))
	for _, b := range branches {
		items = append(items, branchItem{branch: b})
	}
	return items
}

// branchRecency returns a short recency string like "3d" or "  " for display,
// matching lazygit style. "3d" here represents age of the branch tip.
func branchRecency(b lore.Branch) string {
	if b.Current {
		return "*"
	}
	if b.Created == 0 {
		return "  "
	}
	// created is always epoch milliseconds (verified against lore's own
	// formatting: lore-client/src/cli/commands/branch.rs uses
	// DateTime::from_timestamp_millis on this same field).
	t := time.UnixMilli(b.Created)
	age := time.Since(t)
	if age < 0 {
		age = 0
	}
	if d := int(age.Hours() / 24); d > 0 {
		if d < 10 {
			return fmt.Sprintf(" %dd", d)
		}
		return fmt.Sprintf("%dd", d)
	}
	if h := int(age.Hours()); h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return " 1h"
}

// shortHash truncates a revision hash to the compact form shown in the
// History panel and used in confirm prompts/command-log labels.
func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

// revertCommitMessage builds the auto-commit message lore.RevertRevision
// passes via --message for a Drop (see keys.go's "d" on focusHistory),
// matching git revert's own default of naming what was undone rather than
// leaving the new revision's message blank. Empty when the reverted
// revision itself had no message to quote.
func revertCommitMessage(original string) string {
	if original == "" {
		return ""
	}
	return `Revert "` + original + `"`
}

// formatBranchLog renders a branch's revision list as plain colored text
// for the main panel's "Log" view (Branches focused) - one line per
// revision, matching the History panel's own hash/author/message styling.
func formatBranchLog(revisions []lore.Revision) string {
	if len(revisions) == 0 {
		return "No revisions."
	}
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	purple := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	lines := make([]string, 0, len(revisions))
	for _, rv := range revisions {
		author := rv.Author
		if author == "" {
			author = "unknown"
		}
		lines = append(lines, green.Render(shortHash(rv.Hash))+" "+purple.Render(author)+" "+rv.Message)
	}
	return strings.Join(lines, "\n")
}

type revisionItem struct {
	revision lore.Revision
	unpushed bool // true when this revision hasn't reached the remote branch yet
}

func (i revisionItem) Title() string {
	// structural
	if i.revision.Hash != "" {
		return fmt.Sprintf("%s %s", i.revision.Hash, i.revision.Message)
	}
	return fmt.Sprintf("%d  %s", i.revision.Number, i.revision.Message)
}

func (i revisionItem) Description() string { return "" }

func (i revisionItem) FilterValue() string { return i.revision.Message }

// historyToItems builds the History panel's rows, marking each revision
// unpushed when it's newer than the remote branch's latest known revision.
// When hasRemoteInfo is false (offline, unauthorized), nothing is marked
// unpushed rather than guessing.
func historyToItems(revisions []lore.Revision, remoteRevisionNumber uint64, hasRemoteInfo bool) []list.Item {
	items := make([]list.Item, 0, len(revisions))
	for _, rv := range revisions {
		unpushed := hasRemoteInfo && uint64(rv.Number) > remoteRevisionNumber
		items = append(items, revisionItem{revision: rv, unpushed: unpushed})
	}
	return items
}

// compactTitleDelegate is a minimal ItemDelegate for single-line branch/history
// lists. It renders only the Title() on one row (Description is ignored/empty).
// Selection highlight is a full-width background only on the focused panel,
// matching the fileDelegate and lazygit's focused vs inactive selection.
type compactTitleDelegate struct {
	focused bool
	width   int // content width for bounding the bg highlight
}

func (d compactTitleDelegate) Height() int                         { return 1 }
func (d compactTitleDelegate) Spacing() int                        { return 0 }
func (d compactTitleDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d compactTitleDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	selected := d.focused && index == m.Index()

	// Breathing room inside the panel (1 col right gutter for text).
	contentW := d.width
	if contentW > 1 {
		contentW--
	}

	// Branch rows: current branch shows "* name" in green; others show a
	// cyan recency prefix ("3d"). Title split (Local/Remotes) handled at
	// panel title level.
	if bi, ok := listItem.(branchItem); ok {
		name := bi.branch.Name

		green := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
		white := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
		cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
		if selected {
			green = green.Background(selectedBg).Bold(true)
			white = white.Background(selectedBg)
			cyan = cyan.Background(selectedBg)
		}

		var display string
		if bi.branch.Current {
			display = green.Render("* ") + white.Render(name)
		} else {
			rec := branchRecency(bi.branch)
			display = cyan.Render(rec+" ") + white.Render(name)
		}

		if selected {
			// Full selection background (blue) across the row for the entire
			// branch line, including the cyan recency part. Matches the
			// fileDelegate pattern to ensure "full ... filled selection".
			fmt.Fprint(w, selectedRowStyle(d.width).Render(display))
		} else {
			fmt.Fprint(w, lipgloss.NewStyle().Width(contentW).MaxWidth(contentW).Render(display))
		}
		return
	}

	// Special for history/revision to match lazygit's own unpushed/pushed
	// hash coloring (pkg/gui/presentation/commits.go's getHashColor: red
	// StatusUnpushed vs green StatusPushed/StatusMerged - lore only has the
	// two-state distinction, no separate "merged upstream" status) |
	// purple author ○ | message.
	if ri, ok := listItem.(revisionItem); ok {
		hash := ri.revision.Hash
		if hash == "" {
			hash = fmt.Sprintf("%d", ri.revision.Number)
		} else {
			hash = shortHash(hash)
		}
		author := ri.revision.Author
		if author == "" {
			author = "unknown"
		}
		if len(author) > 10 {
			author = author[:10]
		}
		msg := ri.revision.Message

		hashColor := fileStagedColor // green
		if ri.unpushed {
			hashColor = fileUnstagedColor // red
		}
		hashStyle := lipgloss.NewStyle().Foreground(hashColor)
		purple := lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // purple-ish
		msgStyle := lipgloss.NewStyle()
		spaceStyle := lipgloss.NewStyle()
		if selected {
			hashStyle = hashStyle.Background(selectedBg).Bold(true)
			purple = purple.Background(selectedBg)
			msgStyle = msgStyle.Background(selectedBg)
			spaceStyle = spaceStyle.Background(selectedBg)
		}

		// Build with explicit styles on all parts (including spaces and msg)
		// so the blue selection background fills the entire row, matching
		// the fix for Files and Branches.
		display := hashStyle.Render(hash) +
			spaceStyle.Render(" ") +
			purple.Render(author+" ○") +
			spaceStyle.Render(" ") +
			msgStyle.Render(msg)

		if selected {
			fmt.Fprint(w, selectedRowStyle(d.width).Render(display))
		} else {
			fmt.Fprint(w, lipgloss.NewStyle().Width(contentW).MaxWidth(contentW).Render(display))
		}
		return
	}

	// Generic (other): use Title()
	title := ""
	if it, ok := listItem.(interface{ Title() string }); ok {
		title = it.Title()
	}
	if selected {
		fmt.Fprint(w, selectedRowStyle(d.width).Render(title))
	} else {
		fmt.Fprint(w, lipgloss.NewStyle().Width(contentW).MaxWidth(contentW).Render(title))
	}
}
