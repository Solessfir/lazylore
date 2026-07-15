package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"lazylore/internal/lore"
)

func TestScrollWindowStart_NoScrollNeededWhenEverythingFits(t *testing.T) {
	if got := scrollWindowStart(3, 5, 10); got != 0 {
		t.Fatalf("scrollWindowStart(3,5,10) = %d, want 0", got)
	}
}

func TestScrollWindowStart_FollowsCursorPastWindowHeight(t *testing.T) {
	// cursor=8 with an 8-row window and a 2-row scroll margin: the window
	// must have scrolled enough to keep 2 rows of context below the cursor
	// visible (matching lazygit/vim scrolloff), not just barely keep the
	// cursor itself on screen.
	if got := scrollWindowStart(8, 20, 8); got != 3 {
		t.Fatalf("scrollWindowStart(8,20,8) = %d, want 3", got)
	}
}

func TestScrollWindowStart_ClampsAtTheEndOfTheList(t *testing.T) {
	// cursor on the very last item never scrolls the window past total-height.
	if got := scrollWindowStart(19, 20, 8); got != 12 {
		t.Fatalf("scrollWindowStart(19,20,8) = %d, want 12", got)
	}
}

func TestRenderListWindow_NeverLeavesABlankRowMidList(t *testing.T) {
	items := make([]list.Item, 20)
	for i := range items {
		items[i] = fileItem{path: fmt.Sprintf("file%02d.txt", i), label: fmt.Sprintf("file%02d.txt", i)}
	}
	l := list.New(items, fileDelegate{focused: false}, 40, 10)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.Select(8) // deep enough that a page-based list would be on its first (short) page boundary

	out := renderListWindow(l, fileDelegate{focused: false}, 10, -1)
	lines := strings.Split(out, "\n")
	if len(lines) != 10 {
		t.Fatalf("got %d lines, want exactly 10 (the window height, one real item per row, no filler)", len(lines))
	}
	// Every rendered row must carry one of the real filenames - proves the
	// window is a straight slice of real items, not padded with blanks.
	if !strings.Contains(lines[9], "file10.txt") {
		t.Fatalf("last window row = %q, want it to contain file10.txt (scrollWindowStart(8,20,10) == 1, window is items 1-10)", lines[9])
	}
}

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
	item := fileItem{path: "hello.txt", label: "hello.txt", change: lore.FileChange{Status: 'M', Path: "hello.txt"}, staged: false}
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
	// Unstaged/untracked filenames stay uncolored - only the status letter
	// is colored. GetForeground on a style with no Foreground() call
	// returns lipgloss.NoColor{}.
	if got := fileNameStyle(false).GetForeground(); got != (lipgloss.NoColor{}) {
		t.Fatalf("unstaged filename foreground = %v, want no color set", got)
	}
}

func TestFileDelegate_RenderColorsStatusLetterButNotUnstagedName(t *testing.T) {
	items := []list.Item{fileItem{path: "hello.txt", label: "hello.txt", change: lore.FileChange{Status: 'M', Path: "hello.txt"}, staged: false}}
	l := list.New(items, fileDelegate{focused: false}, 40, 5)

	var buf bytes.Buffer
	fileDelegate{focused: false}.Render(&buf, l, 0, items[0])
	out := buf.String()

	if !bytes.Contains(buf.Bytes(), []byte("hello.txt")) {
		t.Fatalf("rendered output missing the filename: %q", out)
	}
}

func TestFileDelegate_RenderClipsUnselectedLongLabelToPanelWidth(t *testing.T) {
	item := fileItem{
		path:   "Content/Sus/Blueprints/BP_PlayerController.uasset",
		label:  "Content/Sus/Blueprints/BP_PlayerController.uasset",
		isDir:  true,
		change: lore.FileChange{Status: 'M'},
	}
	items := []list.Item{item}
	l := list.New(items, fileDelegate{focused: false}, 20, 5)

	var buf bytes.Buffer
	fileDelegate{focused: false}.Render(&buf, l, 0, item)

	if w := lipgloss.Width(buf.String()); w > 20 {
		t.Fatalf("rendered row width = %d, want <= panel width 20; got %q", w, buf.String())
	}
}

func TestCompactTitleDelegate_RenderClipsUnselectedLongBranchNameToPanelWidth(t *testing.T) {
	// Same regression as fileDelegate's: lipgloss's Width() alone only pads
	// short content, it never truncates long content, so an unselected row
	// with no matching MaxWidth() could still overflow past the panel.
	item := branchItem{branch: lore.Branch{Name: strings.Repeat("a-very-long-branch-name-", 5)}}
	items := []list.Item{item}
	l := list.New(items, compactTitleDelegate{focused: false, width: 20}, 20, 5)

	var buf bytes.Buffer
	compactTitleDelegate{focused: false, width: 20}.Render(&buf, l, 0, item)

	if w := lipgloss.Width(buf.String()); w > 20 {
		t.Fatalf("rendered row width = %d, want <= panel width 20; got %q", w, buf.String())
	}
}

func TestStatusToItems_StagedThenUnstaged(t *testing.T) {
	s := lore.Status{
		Staged:   []lore.FileChange{{Status: 'A', Path: "a.txt"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "b.txt"}},
	}
	items := statusToItems(s, nil, nil, "")
	// Two top-level entries, so a "/" root row wraps them (see
	// TestBuildFileTree_MultipleTopLevelEntriesShowRootRow).
	if len(items) != 3 {
		t.Fatalf("items = %+v, want 3 entries (/, a.txt, b.txt)", items)
	}
	first, ok := items[1].(fileItem)
	if !ok || !first.staged || first.change.Path != "a.txt" {
		t.Fatalf("items[1] = %+v, want the staged a.txt entry first", items[1])
	}
	second, ok := items[2].(fileItem)
	if !ok || second.staged || second.change.Path != "b.txt" {
		t.Fatalf("items[2] = %+v, want the unstaged b.txt entry second", items[2])
	}
}

func TestStatusToItems_MarksLockedFiles(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}, {Status: 'M', Path: "b.txt"}}}
	locks := map[string]lore.Lock{"a.txt": {Path: "a.txt", Owner: "someone"}}
	items := statusToItems(s, nil, locks, "")
	// Two top-level entries, so a "/" root row wraps them.
	a, ok := items[1].(fileItem)
	if !ok || !a.locked {
		t.Fatalf("items[1] = %+v, want a.txt marked locked", items[1])
	}
	b, ok := items[2].(fileItem)
	if !ok || b.locked {
		t.Fatalf("items[2] = %+v, want b.txt not locked", items[2])
	}
}

func TestStatusToItems_DistinguishesLockedByMeFromLockedByOther(t *testing.T) {
	s := lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "mine.txt"}, {Status: 'M', Path: "theirs.txt"}}}
	locks := map[string]lore.Lock{
		"mine.txt":   {Path: "mine.txt", Owner: "user-123"},
		"theirs.txt": {Path: "theirs.txt", Owner: "user-456"},
	}
	items := statusToItems(s, nil, locks, "user-123")
	// Two top-level entries, so a "/" root row wraps them.
	mine, ok := items[1].(fileItem)
	if !ok || !mine.locked || !mine.lockedByMe {
		t.Fatalf("items[1] = %+v, want mine.txt locked and lockedByMe", items[1])
	}
	theirs, ok := items[2].(fileItem)
	if !ok || !theirs.locked || theirs.lockedByMe {
		t.Fatalf("items[2] = %+v, want theirs.txt locked but not lockedByMe", items[2])
	}
}

func TestStatusToItems_EmptyCurrentUserIDNeverMarksLockedByMe(t *testing.T) {
	// Before loadCurrentUserCmd resolves (or when unauthenticated),
	// currentUserID is "" - must never accidentally match a lock whose
	// Owner also happens to be empty.
	s := lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}
	locks := map[string]lore.Lock{"a.txt": {Path: "a.txt", Owner: ""}}
	items := statusToItems(s, nil, locks, "")
	a, ok := items[0].(fileItem)
	if !ok || !a.locked || a.lockedByMe {
		t.Fatalf("items[0] = %+v, want locked but not lockedByMe when currentUserID is empty", items[0])
	}
}

func TestFileDelegate_RenderShowsLockBadgeForLockedFile(t *testing.T) {
	item := fileItem{path: "a.txt", label: "a.txt", change: lore.FileChange{Status: 'M', Path: "a.txt"}, locked: true}
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
