package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"

	"lazylore/internal/lore"
)

type fileItem struct {
	change lore.FileChange
	staged bool
}

func (i fileItem) Title() string {
	return fmt.Sprintf("%c %s", i.change.Status, i.change.Path)
}

func (i fileItem) Description() string {
	if i.staged {
		return "staged"
	}
	return "unstaged"
}

func (i fileItem) FilterValue() string { return i.change.Path }

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
