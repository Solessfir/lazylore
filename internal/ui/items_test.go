package ui

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"lazylore/internal/lore"
)

func TestSelectedRowStyle_BoundedToGivenWidth(t *testing.T) {
	// This is the actual bug: without an explicit Width(), the background
	// fill isn't bounded to this panel's own content width at all, and
	// bleeds across the rest of the terminal row past the panel's border.
	got := selectedRowStyle(42)
	if got.GetWidth() != 42 {
		t.Fatalf("selectedRowStyle(42).GetWidth() = %d, want 42", got.GetWidth())
	}
	if got.GetBackground() != selectedBg {
		t.Fatalf("selectedRowStyle background = %v, want %v", got.GetBackground(), selectedBg)
	}
	// No Foreground override - the row's own status color (set before this
	// style wraps it) should still show through underneath the highlight.
	if got.GetForeground() != (lipgloss.NoColor{}) {
		t.Fatalf("selectedRowStyle foreground = %v, want unset (preserves the wrapped text's own color)", got.GetForeground())
	}
}

func TestFileItem_FilterValueIsThePath(t *testing.T) {
	item := fileItem{path: "hello.txt", change: lore.FileChange{Status: 'M', Path: "hello.txt"}, staged: false}
	if item.FilterValue() != "hello.txt" {
		t.Fatalf("FilterValue() = %q, want %q", item.FilterValue(), "hello.txt")
	}
}

func TestFileStatusColor_StagedIsGreenUnstagedIsRed(t *testing.T) {
	if got := fileStatusColor(true); got != fileStagedColor {
		t.Fatalf("fileStatusColor(true) = %v, want %v (green)", got, fileStagedColor)
	}
	if got := fileStatusColor(false); got != fileUnstagedColor {
		t.Fatalf("fileStatusColor(false) = %v, want %v (red)", got, fileUnstagedColor)
	}
}

func TestFileNameStyle_StagedIsGreenUnstagedIsUncolored(t *testing.T) {
	if got := fileNameStyle(true).GetForeground(); got != fileStagedColor {
		t.Fatalf("staged filename foreground = %v, want %v (green)", got, fileStagedColor)
	}
	// Unstaged/untracked filenames are left uncolored (matches lazygit: only
	// the status letter is colored, the filename stays in the default text
	// color) - GetForeground on a style with no Foreground() call returns
	// lipgloss.NoColor{}.
	if got := fileNameStyle(false).GetForeground(); got != (lipgloss.NoColor{}) {
		t.Fatalf("unstaged filename foreground = %v, want no color set", got)
	}
}

func TestFileDelegate_RenderColorsStatusLetterButNotUnstagedName(t *testing.T) {
	items := []list.Item{fileItem{path: "hello.txt", change: lore.FileChange{Status: 'M', Path: "hello.txt"}, staged: false}}
	l := list.New(items, fileDelegate{focused: false}, 40, 5)

	var buf bytes.Buffer
	fileDelegate{focused: false}.Render(&buf, l, 0, items[0])
	out := buf.String()

	if !bytes.Contains(buf.Bytes(), []byte("hello.txt")) {
		t.Fatalf("rendered output missing the filename: %q", out)
	}
}

func TestStatusToItems_StagedThenUnstaged(t *testing.T) {
	s := lore.Status{
		Staged:   []lore.FileChange{{Status: 'A', Path: "a.txt"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "b.txt"}},
	}
	items := statusToItems(s, nil, nil)
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

func TestStatusToItems_MarksLockedFiles(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}, {Status: 'M', Path: "b.txt"}}}
	locks := map[string]lore.Lock{"a.txt": {Path: "a.txt", Owner: "someone"}}
	items := statusToItems(s, nil, locks)
	a, ok := items[0].(fileItem)
	if !ok || !a.locked {
		t.Fatalf("items[0] = %+v, want a.txt marked locked", items[0])
	}
	b, ok := items[1].(fileItem)
	if !ok || b.locked {
		t.Fatalf("items[1] = %+v, want b.txt not locked", items[1])
	}
}

func TestFileDelegate_RenderShowsLockBadgeForLockedFile(t *testing.T) {
	item := fileItem{path: "a.txt", change: lore.FileChange{Status: 'M', Path: "a.txt"}, locked: true}
	l := list.New([]list.Item{item}, fileDelegate{focused: false}, 40, 5)

	var buf bytes.Buffer
	fileDelegate{focused: false}.Render(&buf, l, 0, item)
	if !bytes.Contains(buf.Bytes(), []byte(lockBadge)) {
		t.Fatalf("rendered output missing the lock badge: %q", buf.String())
	}
}

func TestBranchItem_TitleMarksCurrent(t *testing.T) {
	// Title() now returns structural text; actual colors/* /✓ are applied
	// at render time in the delegate to match lazygit.
	current := branchItem{branch: lore.Branch{Name: "main", Current: true}}
	if current.Title() != "main" {
		t.Fatalf("Title() = %q, want %q", current.Title(), "main")
	}
	other := branchItem{branch: lore.Branch{Name: "dev", Current: false}}
	if other.Title() != "dev" {
		t.Fatalf("Title() = %q, want %q", other.Title(), "dev")
	}
	remote := branchItem{branch: lore.Branch{Name: "origin/main", Current: false, Remote: true}}
	if remote.Title() != "origin/main" {
		t.Fatalf("Title() = %q, want %q", remote.Title(), "origin/main")
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
	items := historyToItems(revisions, 0, false)
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2 entries", items)
	}
}

func TestHistoryToItems_MarksRevisionsNewerThanRemoteAsUnpushed(t *testing.T) {
	revisions := []lore.Revision{
		{Number: 3, Message: "local only"},
		{Number: 2, Message: "pushed"},
		{Number: 1, Message: "pushed too"},
	}
	items := historyToItems(revisions, 2, true)
	if ri := items[0].(revisionItem); !ri.unpushed {
		t.Fatalf("revision 3 (> remote 2) should be unpushed: %+v", ri)
	}
	if ri := items[1].(revisionItem); ri.unpushed {
		t.Fatalf("revision 2 (== remote 2) should be pushed: %+v", ri)
	}
	if ri := items[2].(revisionItem); ri.unpushed {
		t.Fatalf("revision 1 (< remote 2) should be pushed: %+v", ri)
	}
}

func TestHistoryToItems_NoRemoteInfoMeansNothingMarkedUnpushed(t *testing.T) {
	revisions := []lore.Revision{{Number: 5, Message: "who knows"}}
	items := historyToItems(revisions, 0, false)
	if ri := items[0].(revisionItem); ri.unpushed {
		t.Fatalf("without remote info nothing should be marked unpushed: %+v", ri)
	}
}
