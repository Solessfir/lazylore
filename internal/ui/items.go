package ui

import (
	"fmt"
	"io"

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

type fileItem struct {
	change lore.FileChange
	staged bool
}

func (i fileItem) FilterValue() string { return i.change.Path }

// fileDelegate renders Files list items itself, rather than going through
// list.DefaultDelegate's Title()/Description() two-line convention - lore's
// per-file status is conveyed by color (see fileStatusColor/fileNameStyle),
// not a text label, and each file is a single line.
type fileDelegate struct {
	focused bool
}

func (d fileDelegate) Height() int                         { return 1 }
func (d fileDelegate) Spacing() int                        { return 0 }
func (d fileDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d fileDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	fi, ok := listItem.(fileItem)
	if !ok {
		return
	}
	line := fmt.Sprintf("%c %s", fi.change.Status, fi.change.Path)

	if d.focused && index == m.Index() {
		selectedStyle := lipgloss.NewStyle().Background(selectedBg).Foreground(selectedFg).Bold(true)
		fmt.Fprint(w, selectedStyle.Render(line))
		return
	}

	statusStyle := lipgloss.NewStyle().Foreground(fileStatusColor(fi.staged))
	fmt.Fprint(w, statusStyle.Render(string(fi.change.Status))+" "+fileNameStyle(fi.staged).Render(fi.change.Path))
}

func statusToItems(s lore.Status) []list.Item {
	items := make([]list.Item, 0, len(s.Staged)+len(s.Unstaged))
	for _, c := range s.Staged {
		items = append(items, fileItem{change: c, staged: true})
	}
	for _, c := range s.Unstaged {
		items = append(items, fileItem{change: c, staged: false})
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
