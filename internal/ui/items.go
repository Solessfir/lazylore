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

// Colors match lazygit's actual file-status convention
// (pkg/gui/presentation/files.go, formatFileStatus/getFileLine): a staged
// file's status letter AND filename render green; an unstaged/untracked
// file's status letter renders red, with the filename left uncolored -
// there's no separate "staged"/"unstaged" text label anywhere, the color
// alone conveys it.
var (
	fileStagedColor   = lipgloss.Color("2") // green
	fileUnstagedColor = lipgloss.Color("1") // red
)

// lockBadge marks a file with an active lore file lock (any owner - see
// internal/lore/lock.go; git/lazygit have no equivalent concept).
const lockBadge = "\U0001F512" // 🔒

var lockBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow

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
// FileChange it represents - one tree, built fresh from Status on every
// refresh by statusToItems, with collapse state carried separately on
// Model so it survives a refresh.
type fileItem struct {
	path      string
	isDir     bool
	depth     int
	change    lore.FileChange
	staged    bool
	collapsed bool // only meaningful when isDir
	locked    bool // only meaningful for files; lore lock held by anyone
}

func (i fileItem) FilterValue() string { return i.path }

func (i fileItem) baseName() string {
	if idx := strings.LastIndex(i.path, "/"); idx >= 0 {
		return i.path[idx+1:]
	}
	return i.path
}

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
// coloring shows through underneath it - matching lazyp4's styleCursor
// (internal/ui/panes/filelist.go), which wraps the already-colored line
// rather than replacing its style outright. Width is set explicitly to the
// list's own content width (m.Width(), captured at Render time) - without
// it the background fill isn't bounded to this panel at all and bleeds
// across the rest of the terminal row, past the panel's own border.
//
// We use the full width for the highlight (so it reaches near the right
// border like lazygit), while individual item text may have a small right
// gutter for breathing room.
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
		if fi.isDir {
			arrow := "▼"
			if fi.collapsed {
				arrow = "▶"
			}
			fmt.Fprint(w, indent+arrow+" "+fi.baseName())
			return
		}
		// File, not selected: apply per-part colors (status red/green, name color for staged)
		statusStyle := lipgloss.NewStyle().Foreground(fileStatusColor(fi.staged))
		nameStyle := fileNameStyle(fi.staged)
		line := indent + statusStyle.Render(string(fi.change.Status)) + " " + nameStyle.Render(fi.baseName())
		if fi.locked {
			line += " " + lockBadgeStyle.Render(lockBadge)
		}
		fmt.Fprint(w, line)
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
		display := indent + arrow + " " + fi.baseName()
		fmt.Fprint(w, selectedRowStyle(rowWidth).Render(display))
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
		nameStyle.Render(fi.baseName())
	if fi.locked {
		colored += selSpace.Render(" ") + lockBadgeStyle.Background(selectedBg).Render(lockBadge)
	}

	// selectedRowStyle ensures full-width background fill (including gutter area)
	fmt.Fprint(w, selectedRowStyle(rowWidth).Render(colored))
}

// statusToItems flattens a Status's changed files into a directory tree
// (see filetree.go), respecting which directories are currently collapsed.
// locks maps path -> held lock (see loadLocksCmd); nil is fine (no badges).
func statusToItems(s lore.Status, collapsedDirs map[string]bool, locks map[string]lore.Lock) []list.Item {
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, collapsedDirs)

	items := make([]list.Item, 0, len(rows))
	for _, row := range rows {
		_, locked := locks[row.node.path]
		items = append(items, fileItem{
			path:      row.node.path,
			isDir:     row.node.isDir,
			depth:     row.depth,
			change:    row.node.change,
			staged:    row.node.staged,
			collapsed: collapsedDirs[row.node.path],
			locked:    locked,
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

type revisionItem struct {
	revision lore.Revision
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

func historyToItems(revisions []lore.Revision) []list.Item {
	items := make([]list.Item, 0, len(revisions))
	for _, rv := range revisions {
		items = append(items, revisionItem{revision: rv})
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
			fmt.Fprint(w, lipgloss.NewStyle().Width(contentW).Render(display))
		}
		return
	}

	// Special for history/revision to match lazygit:
	// green short hash | purple author ○ | message
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

		green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		purple := lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // purple-ish
		msgStyle := lipgloss.NewStyle()
		spaceStyle := lipgloss.NewStyle()
		if selected {
			green = green.Background(selectedBg).Bold(true)
			purple = purple.Background(selectedBg)
			msgStyle = msgStyle.Background(selectedBg)
			spaceStyle = spaceStyle.Background(selectedBg)
		}

		// Build with explicit styles on all parts (including spaces and msg)
		// so the blue selection background fills the entire row, matching
		// the fix for Files and Branches.
		display := green.Render(hash) +
			spaceStyle.Render(" ") +
			purple.Render(author+" ○") +
			spaceStyle.Render(" ") +
			msgStyle.Render(msg)

		if selected {
			fmt.Fprint(w, selectedRowStyle(d.width).Render(display))
		} else {
			fmt.Fprint(w, lipgloss.NewStyle().Width(contentW).Render(display))
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
		fmt.Fprint(w, lipgloss.NewStyle().Width(contentW).Render(title))
	}
}
