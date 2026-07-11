package ui

import (
	"fmt"
	"io"
	"strings"

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

	var line string
	if fi.isDir {
		arrow := "▼"
		if fi.collapsed {
			arrow = "▶"
		}
		line = indent + arrow + " " + fi.baseName()
	} else {
		statusStyle := lipgloss.NewStyle().Foreground(fileStatusColor(fi.staged))
		line = indent + statusStyle.Render(string(fi.change.Status)) + " " + fileNameStyle(fi.staged).Render(fi.baseName())
	}

	if selected {
		line = selectedRowStyle(m.Width()).Render(line)
	}
	fmt.Fprint(w, line)
}

// statusToItems flattens a Status's changed files into a directory tree
// (see filetree.go), respecting which directories are currently collapsed.
func statusToItems(s lore.Status, collapsedDirs map[string]bool) []list.Item {
	tree := buildFileTree(s)
	rows := flattenFileTree(tree, collapsedDirs)

	items := make([]list.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, fileItem{
			path:      row.node.path,
			isDir:     row.node.isDir,
			depth:     row.depth,
			change:    row.node.change,
			staged:    row.node.staged,
			collapsed: collapsedDirs[row.node.path],
		})
	}
	return items
}

type branchItem struct {
	branch lore.Branch
}

func (i branchItem) Title() string {
	if i.branch.Current {
		return "* " + i.branch.Name
	}
	return "  " + i.branch.Name
}

func (i branchItem) Description() string {
	if i.branch.Remote {
		return "remote"
	}
	return "local"
}

func (i branchItem) FilterValue() string { return i.branch.Name }

func branchesToItems(branches []lore.Branch) []list.Item {
	items := make([]list.Item, 0, len(branches))
	for _, b := range branches {
		items = append(items, branchItem{branch: b})
	}
	return items
}

type revisionItem struct {
	revision lore.Revision
}

func (i revisionItem) Title() string {
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
