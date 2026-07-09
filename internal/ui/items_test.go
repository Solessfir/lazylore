package ui

import (
	"testing"

	"lazylore/internal/lore"
)

func TestFileItem_TitleShowsStatusAndPath(t *testing.T) {
	item := fileItem{change: lore.FileChange{Status: 'M', Path: "hello.txt"}, staged: false}
	if item.Title() != "M hello.txt" {
		t.Fatalf("Title() = %q, want %q", item.Title(), "M hello.txt")
	}
	if item.Description() != "unstaged" {
		t.Fatalf("Description() = %q, want %q", item.Description(), "unstaged")
	}
	if item.FilterValue() != "hello.txt" {
		t.Fatalf("FilterValue() = %q, want %q", item.FilterValue(), "hello.txt")
	}
}

func TestFileItem_StagedDescription(t *testing.T) {
	item := fileItem{change: lore.FileChange{Status: 'A', Path: "new.txt"}, staged: true}
	if item.Description() != "staged" {
		t.Fatalf("Description() = %q, want %q", item.Description(), "staged")
	}
}

func TestStatusToItems_StagedThenUnstaged(t *testing.T) {
	s := lore.Status{
		Staged:   []lore.FileChange{{Status: 'A', Path: "a.txt"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "b.txt"}},
	}
	items := statusToItems(s)
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2 entries", items)
	}
	first, ok := items[0].(fileItem)
	if !ok || !first.staged || first.change.Path != "a.txt" {
		t.Fatalf("items[0] = %+v, want the staged a.txt entry first", items[0])
	}
	second, ok := items[1].(fileItem)
	if !ok || second.staged || second.change.Path != "b.txt" {
		t.Fatalf("items[1] = %+v, want the unstaged b.txt entry second", items[1])
	}
}

func TestBranchItem_TitleMarksCurrent(t *testing.T) {
	current := branchItem{branch: lore.Branch{Name: "main", Current: true}}
	if current.Title() != "* main" {
		t.Fatalf("Title() = %q, want %q", current.Title(), "* main")
	}
	other := branchItem{branch: lore.Branch{Name: "dev", Current: false}}
	if other.Title() != "  dev" {
		t.Fatalf("Title() = %q, want %q", other.Title(), "  dev")
	}
}

func TestBranchesToItems_PreservesOrder(t *testing.T) {
	branches := []lore.Branch{{Name: "main", Current: true}, {Name: "dev"}}
	items := branchesToItems(branches)
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2 entries", items)
	}
}

func TestRevisionItem_TitleShowsNumberAndMessage(t *testing.T) {
	item := revisionItem{revision: lore.Revision{Number: 3, Message: "Fix bug"}}
	if item.Title() != "3  Fix bug" {
		t.Fatalf("Title() = %q, want %q", item.Title(), "3  Fix bug")
	}
}

func TestHistoryToItems_PreservesOrder(t *testing.T) {
	revisions := []lore.Revision{{Number: 2, Message: "second"}, {Number: 1, Message: "first"}}
	items := historyToItems(revisions)
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2 entries", items)
	}
}
