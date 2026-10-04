package ui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

func TestModel_ResizeLeavesAtLeastOneColumnOfMargin(t *testing.T) {
	// Panel widths must sum to strictly less than the terminal width - at an
	// exact fit, a single-column mismatch (terminal wrap quirks, a glyph
	// rendering one column wider than lipgloss counts it) wraps a row.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 141, Height: 40})
	m2 := updated.(Model)

	leftOuter := m2.panelWidth + borderWidth
	rightOuter := m2.diff.vp.Width + 1 + borderWidth // +1 for the scrollbar column
	if total := leftOuter + rightOuter; total >= 141 {
		t.Fatalf("left+right outer width = %d, want strictly less than terminal width 141 (some margin)", total)
	}
}

func TestModel_DiffScrollBoundsMatchDisplayedHeightAndResize(t *testing.T) {
	for _, wheel := range []bool{false, true} {
		m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		m = updated.(Model)
		m.focus = focusDiff
		var lines []string
		for i := 0; i < 50; i++ {
			lines = append(lines, fmt.Sprintf("line%02d", i))
		}
		m.diff.SetContentRaw(strings.Join(lines, "\n"))
		for i := 0; i < 20; i++ {
			var msg tea.Msg = tea.KeyMsg{Type: tea.KeyPgDown}
			if wheel {
				msg = tea.MouseMsg{X: m.computeMouseLayout().leftW + 3, Y: 5, Button: tea.MouseButtonWheelDown}
			}
			updated, _ = m.Update(msg)
			m = updated.(Model)
		}
		l := m.computeMouseLayout()
		if m.diff.vp.Height != l.effDiffH || m.diff.vp.YOffset != 50-l.effDiffH {
			t.Fatalf("wheel=%v scroll offset=%d height=%d, displayed height=%d", wheel, m.diff.vp.YOffset, m.diff.vp.Height, l.effDiffH)
		}
		updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
		m = updated.(Model)
		l = m.computeMouseLayout()
		if m.diff.vp.YOffset != 50-l.effDiffH {
			t.Fatalf("wheel=%v resize kept offset=%d, want %d", wheel, m.diff.vp.YOffset, 50-l.effDiffH)
		}
		viewLines := strings.Split(m.View(), "\n")
		if !strings.Contains(viewLines[l.diffAreaEnd-2], "line49") {
			t.Fatal("last displayed diff row is blank after reaching bottom and growing the terminal")
		}
	}
}

func TestModel_DiffBoundsFollowAsynchronousFooterChanges(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	m.focus = focusDiff
	m.diff.SetContentRaw(strings.Repeat("line\n", 49) + "last")
	updated, _ = m.Update(branchesMsg{err: errors.New("error")})
	m = updated.(Model)
	if m.diff.vp.Height != m.computeMouseLayout().effDiffH {
		t.Fatal("error footer changed display without synchronizing viewport bounds")
	}
	m.diff.vp.GotoBottom()
	updated, _ = m.Update(statusMsg{status: lore.Status{}})
	m = updated.(Model)
	l := m.computeMouseLayout()
	if m.diff.vp.Height != l.effDiffH || m.diff.vp.YOffset != 50-l.effDiffH {
		t.Fatalf("clearing error kept offset=%d height=%d, displayed=%d", m.diff.vp.YOffset, m.diff.vp.Height, l.effDiffH)
	}
}

func TestModel_ListPaginationMatchesDisplayedRowsWithoutHiddenFilterChrome(t *testing.T) {
	for _, source := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		m = updated.(Model)
		var changes []lore.FileChange
		var branches []lore.Branch
		var revisions []lore.Revision
		for i := 0; i < 50; i++ {
			changes = append(changes, lore.FileChange{Status: 'M', Path: fmt.Sprintf("file%02d", i)})
			branches = append(branches, lore.Branch{Name: fmt.Sprintf("branch%02d", i)})
			revisions = append(revisions, lore.Revision{Hash: fmt.Sprintf("revision%02d", i), Message: fmt.Sprintf("revision%02d", i)})
		}
		updated, _ = m.Update(statusMsg{status: lore.Status{Unstaged: changes}})
		m = updated.(Model)
		updated, _ = m.Update(branchesMsg{branches: branches})
		m = updated.(Model)
		updated, _ = m.Update(historyMsg{revisions: revisions})
		m = updated.(Model)
		m.focus = source
		m.panelList(source).FilterInput.Cursor.SetMode(cursor.CursorStatic)
		l := m.computeMouseLayout()
		rows := map[focusPanel]int{focusFiles: l.effFilesH, focusBranches: l.effBranchesH, focusHistory: l.effHistoryH}[source]
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = updated.(Model)
		if m.panelList(source).Index() != rows || m.panelList(source).Paginator.PerPage != rows {
			t.Fatalf("panel %v page selected=%d perpage=%d, displayed rows=%d", source, m.panelList(source).Index(), m.panelList(source).Paginator.PerPage, rows)
		}
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
		m = updated.(Model)
		if !m.panelList(source).ShowFilter() || m.panelList(source).Paginator.PerPage != rows-2 {
			t.Fatalf("panel %v filtering did not reserve its visible input and blank rows", source)
		}
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0")})
		m = updated.(Model)
		applyFilterTestCommand(&m, cmd)
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(Model)
		index := m.panelList(source).Index()
		updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
		m = updated.(Model)
		l = m.computeMouseLayout()
		rows = map[focusPanel]int{focusFiles: l.effFilesH, focusBranches: l.effBranchesH, focusHistory: l.effHistoryH}[source]
		if m.panelList(source).ShowFilter() || m.panelList(source).Paginator.PerPage != rows || m.panelList(source).Index() != index || m.panelList(source).FilterValue() != "0" {
			t.Fatalf("panel %v applied filter/resize lost cursor, query, or displayed page geometry", source)
		}
	}
}

func TestModel_HistoryRecolorsWhenStatusArrivesAfterHistory(t *testing.T) {
	// statusMsg and historyMsg load independently with no ordering guarantee.
	// If historyMsg lands first (no remote info yet), a later statusMsg must
	// still recolor the already-rendered list.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{{Number: 2, Hash: "abc", Message: "local only"}}})
	m2 := updated.(Model)
	item0, _ := m2.history.Items()[0].(revisionItem)
	if item0.unpushed {
		t.Fatalf("before status arrives, nothing should be marked unpushed yet: %+v", item0)
	}

	updated, _ = m2.Update(statusMsg{status: lore.Status{
		Branch: "main", HasRemoteInfo: true, RemoteRevisionNumber: 1,
	}})
	m3 := updated.(Model)
	item0, ok := m3.history.Items()[0].(revisionItem)
	if !ok || !item0.unpushed {
		t.Fatalf("after status arrives (remote at revision 1, this revision is 2), it must be marked unpushed: %+v", item0)
	}
}

func TestModel_MainPanelTitle_MatchesLazygitPerContext(t *testing.T) {
	// Ground truth: files_controller.go's renderWorkingTreeDiff (Unstaged/
	// Staged changes), branches_controller.go's LogTitle ("Log"),
	// local_commits_controller.go's hardcoded "Patch".
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	if got := m2.mainPanelTitle(); got != "Unstaged changes" {
		t.Fatalf("mainPanelTitle() (unstaged file selected) = %q, want %q", got, "Unstaged changes")
	}

	updated, _ = m2.Update(statusMsg{status: lore.Status{Staged: []lore.FileChange{{Status: 'A', Path: "b.txt"}}}})
	m3 := updated.(Model)
	if got := m3.mainPanelTitle(); got != "Staged changes" {
		t.Fatalf("mainPanelTitle() (staged file selected) = %q, want %q", got, "Staged changes")
	}

	m3.focus = focusBranches
	(&m3).ensureMainContent() // real usage always pairs a focus change with this, see keys.go
	if got := m3.mainPanelTitle(); got != "Log" {
		t.Fatalf("mainPanelTitle() (Branches focused) = %q, want %q", got, "Log")
	}

	m3.focus = focusHistory
	(&m3).ensureMainContent()
	if got := m3.mainPanelTitle(); got != "Patch" {
		t.Fatalf("mainPanelTitle() (History focused) = %q, want %q", got, "Patch")
	}
}

func TestModel_MainPanelTitle_PersistsSourceWhenDiffPanelItselfFocused(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusBranches
	(&m).ensureMainContent()
	m.focus = focusDiff
	if got := m.mainPanelTitle(); got != "Log" {
		t.Fatalf("mainPanelTitle() after tabbing from Branches into Diff = %q, want %q", got, "Log")
	}
}

func TestModel_EnsureMainContent_SelectingDirectoryClearsDiff(t *testing.T) {
	// Selecting a directory clears the main panel instead of leaving the
	// last-selected file's diff stuck on screen.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "src/a.go"}}}})
	m2 := updated.(Model)
	m2.focus = focusFiles

	m2.diff.SetContentRaw("Locked by Solessfir\n\nsome stale diff text")
	m2.currentDiffPath = "src/a.go"

	// The tree's only row for this status is the "src" directory itself
	// (see filetree.go's compression).
	item, ok := m2.files.SelectedItem().(fileItem)
	if !ok || !item.isDir {
		t.Fatalf("precondition failed: selected item = %+v, want the 'src' directory", item)
	}

	cmd := (&m2).ensureMainContent()
	if cmd != nil {
		t.Fatalf("expected no load Cmd for a directory selection, got %v", cmd)
	}
	if m2.currentDiffPath != "" {
		t.Fatalf("currentDiffPath = %q, want cleared", m2.currentDiffPath)
	}
	if strings.Contains(m2.diff.vp.View(), "stale diff text") {
		t.Fatal("expected the diff panel content to be cleared, but stale text is still showing")
	}
}

func TestModel_EnsureMainContent_LoadsBranchLogOnce(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json history 50 --branch=dev": {ExitCode: 0, Stdout: `{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}` + "\n"},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(branchesMsg{branches: []lore.Branch{{Name: "main", Current: true}, {Name: "dev"}}})
	m2 := updated.(Model)
	m2.focus = focusBranches
	m2.branches.Select(1) // "dev"

	cmd := (&m2).ensureMainContent()
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for the first Log load")
	}
	msg := commandResult(cmd)
	dm, ok := msg.(diffMsg)
	if !ok || !dm.raw {
		t.Fatalf("msg = %#v, want a raw diffMsg (Log content, not diff-colored)", msg)
	}

	// Calling again for the same selection must not re-issue the load.
	if cmd := (&m2).ensureMainContent(); cmd != nil {
		t.Fatalf("expected nil Cmd on the second call for the same branch, got %v", cmd)
	}
}

func TestModel_EnsureMainContent_LoadsRevisionPatchWithParent(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json diff --source parenthash --target abc123": {ExitCode: 0, Stdout: `{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}` + "\n"},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{{Number: 2, Hash: "abc123", Parent: "parenthash", Message: "second"}}})
	m2 := updated.(Model)
	m2.focus = focusHistory

	cmd := (&m2).ensureMainContent()
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for the Patch load")
	}
	msg := commandResult(cmd)
	if dm, ok := msg.(diffMsg); !ok || dm.raw {
		t.Fatalf("msg = %#v, want a non-raw diffMsg (real diff/patch text, should be diff-colored)", msg)
	}
	if len(fake.Calls) != 1 || fake.Calls[0][2] != "--source" || fake.Calls[0][3] != "parenthash" {
		t.Fatalf("Calls = %+v, want a single diff --source parenthash --target abc123 call", fake.Calls)
	}
}

func TestModel_EnsureMainContent_RootRevisionSkipsDiffCall(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{
		{Number: 1, Hash: "root123", Parent: "0000000000000000000000000000000000000000000000000000000000000000", Message: "initial"},
	}})
	m2 := updated.(Model)
	m2.focus = focusHistory

	cmd := (&m2).ensureMainContent()
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd (the 'no parent' message), even though no runner call is made")
	}
	msg := commandResult(cmd)
	dm, ok := msg.(diffMsg)
	if !ok || !dm.raw || !strings.Contains(dm.text, "Initial revision") {
		t.Fatalf("msg = %#v, want a raw diffMsg explaining there's no parent to diff against", msg)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("Calls = %+v, want no runner calls for the root revision", fake.Calls)
	}
}

func TestRowClickTarget_RealItemInWindow(t *testing.T) {
	target, ok := rowClickTarget(0, 10, 8, -1, 2)
	if !ok || target != 2 {
		t.Fatalf("target=%d ok=%v, want 2,true", target, ok)
	}
}

func TestRowClickTarget_RejectsRowBelowLastItemInWindow(t *testing.T) {
	// cursor=0, 10 items, an 8-row-tall window shows items 0-7; row 8 is
	// past the window (blank/nonexistent), not item 8 - unlike bubbles' own
	// page-based Paginator, there's no "next page" for a click to wrongly
	// resolve into here.
	_, ok := rowClickTarget(0, 10, 8, -1, 8)
	if ok {
		t.Fatal("expected relY=8 (past the window) to be rejected")
	}
}

func TestRowClickTarget_RejectsRowPastLastRealItemOnShortList(t *testing.T) {
	_, ok := rowClickTarget(0, 3, 8, -1, 3)
	if ok {
		t.Fatal("expected relY=3 (past the 3 real items) to be rejected")
	}
}

func TestRowClickTarget_ScrolledWindowOffsetsCorrectly(t *testing.T) {
	// cursor=9 (last of 10 items) with an 8-row window scrolls so the
	// window covers items 2-9 (scrollWindowStart(9, 10, 8) == 2); relY=7 is
	// the last visible row, item 9.
	target, ok := rowClickTarget(9, 10, 8, -1, 7)
	if !ok || target != 9 {
		t.Fatalf("target=%d ok=%v, want 9,true", target, ok)
	}
}

func TestRowClickTarget_AcceptsLastRealItemOnAnUnscrolledShortList(t *testing.T) {
	target, ok := rowClickTarget(0, 9, 10, -1, 8)
	if !ok || target != 8 {
		t.Fatalf("target=%d ok=%v, want 8,true (the 9th and last item, on a 10-row-tall panel)", target, ok)
	}
}

func TestModel_ClickSelectsCorrectRowInScrolledFilesWindow(t *testing.T) {
	// More files than fit in the panel at once, so the click has to land
	// correctly on a row inside a scrolled continuous window (see
	// scrollWindowStart), not just on an unscrolled first page.
	var files []lore.FileChange
	for i := 0; i < 20; i++ {
		files = append(files, lore.FileChange{Status: 'M', Path: fmt.Sprintf("file%02d.txt", i)})
	}
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 155, Height: 40})
	m2 := updated.(Model)
	updated, _ = m2.Update(statusMsg{status: lore.Status{Unstaged: files}})
	m3 := updated.(Model)

	// Scroll the cursor deep into the list so the window is no longer
	// anchored at the top (j moves selection down one row at a time) -
	// more presses than there are rows, clamped to the last item.
	for i := 0; i < 30; i++ {
		updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m3 = updated.(Model)
	}

	// Rather than duplicating handleMouseClick's exact footer/extra-height
	// math to predict a precise row, scan every plausible row in the Files
	// panel and confirm exactly one of them selects file19.txt - and that
	// it isn't the very top row, proving the window actually scrolled
	// rather than staying anchored at file00.txt.
	foundAt := -1
	for relY := 0; relY < 30; relY++ { // generous upper bound - the panel is nowhere near 30 rows tall in this test
		clickY := 3 + 1 + relY // statusPanelHeight(3) + files top border(1) + row offset
		updated, _ := m3.Update(tea.MouseMsg{X: 10, Y: clickY, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		m4 := updated.(Model)
		if sel, ok := m4.files.SelectedItem().(fileItem); ok && sel.path == "file19.txt" {
			foundAt = relY
			break
		}
	}
	if foundAt == -1 {
		t.Fatal("no row in the Files panel selected file19.txt after scrolling")
	}
	if foundAt == 0 {
		t.Fatal("file19.txt was found at the very top row - the window doesn't look scrolled")
	}
}

func TestChangedFilePaths_CombinesFilesAndSkipsDirectories(t *testing.T) {
	s := lore.Status{
		Staged: []lore.FileChange{
			{Status: 'A', Path: "empty-dir", Directory: true},
			{Status: 'A', Path: "a.txt"},
		},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "b.txt"}},
	}
	paths := changedFilePaths(s)
	want := []string{"a.txt", "b.txt"}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("changedFilePaths = %+v, want %+v", paths, want)
	}
}

func TestModel_DropsStaleMainContentResponse(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.mainContentRequestID = 2
	m.diff.vp.Width = 80
	m.diff.vp.Height = 10
	m.diff.SetContentRaw("new selection")

	updated, cmd := m.Update(diffMsg{
		request: mainContentRequest{id: 1, source: focusFiles, target: "old.txt"},
		text:    "stale response",
	})
	m2 := updated.(Model)
	if cmd != nil {
		t.Fatalf("stale response returned unexpected command: %v", cmd)
	}
	if strings.Contains(m2.diff.vp.View(), "stale response") || !strings.Contains(m2.diff.vp.View(), "new selection") {
		t.Fatalf("diff content changed after stale response: %q", m2.diff.vp.View())
	}
}

func TestModel_MainContentReloadsWhenReturningToSource(t *testing.T) {
	panels := []struct {
		name string
		key  string
	}{
		{name: "Files", key: "2"},
		{name: "Branches", key: "3"},
		{name: "History", key: "4"},
	}
	for _, first := range panels {
		for _, second := range panels {
			if first == second {
				continue
			}
			for _, pending := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/pending=%t", first.name, second.name, pending), func(t *testing.T) {
					m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
					m.diff.vp.Width, m.diff.vp.Height = 80, 10
					m.status = lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}
					m.rebuildFileItems()
					m.localBranches = []lore.Branch{{Name: "main"}}
					m.refreshBranchesList()
					m.revisions = []lore.Revision{{Hash: "revision", Parent: "parent"}}
					m.rebuildHistoryItems()

					jump := func(key string) tea.Cmd {
						updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
						m = updated.(Model)
						return cmd
					}
					apply := func(msg diffMsg) {
						updated, _ := m.Update(msg)
						m = updated.(Model)
					}
					if jump(first.key) == nil {
						t.Fatal("initial source did not load content")
					}
					firstResponse := diffMsg{request: mainContentRequest{id: m.mainContentRequestID}, text: "first response", raw: true}
					if !pending {
						apply(firstResponse)
					}
					if jump(second.key) == nil {
						t.Fatal("second source did not load content")
					}
					secondResponse := diffMsg{request: mainContentRequest{id: m.mainContentRequestID}, text: "second response", raw: true}
					if !pending {
						apply(secondResponse)
					}
					if jump(first.key) == nil {
						t.Fatal("returning to the first source did not reload its content")
					}
					if m.mainContentRequestID <= secondResponse.request.id {
						t.Fatal("returning to the first source did not invalidate the previous requests")
					}
					if pending {
						apply(firstResponse)
						apply(secondResponse)
						if strings.Contains(m.diff.vp.View(), "response") {
							t.Fatalf("stale content appeared after returning to the first source: %q", m.diff.vp.View())
						}
					}
					apply(diffMsg{request: mainContentRequest{id: m.mainContentRequestID}, text: "returned content", raw: true})
					if !strings.Contains(m.diff.vp.View(), "returned content") {
						t.Fatalf("current response was not displayed: %q", m.diff.vp.View())
					}
				})
			}
		}
	}
}

func TestModel_MainContentKeepsRequestAcrossPassivePanels(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.localBranches = []lore.Branch{{Name: "main"}}
	m.refreshBranchesList()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("initial branch selection did not load content")
	}
	requestID := m.mainContentRequestID
	for _, key := range []string{"1", "5", "6", "3"} {
		updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = updated.(Model)
		if cmd != nil || m.mainContentSource != focusBranches || m.mainContentRequestID != requestID {
			t.Fatalf("jump to %s changed the branch content request", key)
		}
	}
}

func newFilteredTestModel(panel focusPanel) Model {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.status = lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "match.txt"}, {Status: 'M', Path: "other.txt"}}}
	m.rebuildFileItems()
	m.localBranches = []lore.Branch{{Name: "match"}, {Name: "other"}}
	m.refreshBranchesList()
	m.revisions = []lore.Revision{{Hash: "match", Message: "match"}, {Hash: "other", Message: "other"}}
	m.rebuildHistoryItems()
	m.files.FilterInput.Cursor.SetMode(cursor.CursorStatic)
	m.branches.FilterInput.Cursor.SetMode(cursor.CursorStatic)
	m.history.FilterInput.Cursor.SetMode(cursor.CursorStatic)
	m.focus = panel
	m.ensureMainContent()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	return updated.(Model)
}

func applyFilterTestCommand(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := commandResult(cmd)
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			applyFilterTestCommand(m, sub)
		}
		return
	}
	updated, _ := m.Update(msg)
	*m = updated.(Model)
}

func filterTestList(m Model, panel focusPanel) list.Model {
	switch panel {
	case focusFiles:
		return m.files
	case focusBranches:
		return m.branches
	default:
		return m.history
	}
}

func TestModel_FilterResultsReachEveryPanelAndEmptyMatchesBlockActions(t *testing.T) {
	for _, panel := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		t.Run(fmt.Sprint(panel), func(t *testing.T) {
			m := newFilteredTestModel(panel)
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("match")})
			m = updated.(Model)
			applyFilterTestCommand(&m, cmd)
			if got := filterTestList(m, panel).VisibleItems(); len(got) != 1 || !strings.Contains(got[0].FilterValue(), "match") {
				t.Fatalf("filter matches = %v, want only match", got)
			}
			updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("_no_match")})
			m = updated.(Model)
			applyFilterTestCommand(&m, cmd)
			if got := filterTestList(m, panel).VisibleItems(); len(got) != 0 {
				t.Fatalf("nonexistent query retained rows: %v", got)
			}
			for _, key := range []string{" ", "c", "x"} {
				updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
				m = updated.(Model)
				applyFilterTestCommand(&m, cmd)
			}
			if m.prompt != promptNone || len(m.pendingFileOps) != 0 || len(m.runner.(*lore.FakeRunner).Calls) != 0 {
				t.Fatalf("typing filter characters triggered an action: prompt=%v pending=%v calls=%v", m.prompt, m.pendingFileOps, m.runner.(*lore.FakeRunner).Calls)
			}
		})
	}
}

func TestModel_FilterResultsKeepOriginAfterFocusChanges(t *testing.T) {
	for _, panel := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		t.Run(fmt.Sprint(panel), func(t *testing.T) {
			m := newFilteredTestModel(panel)
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("match")})
			m = updated.(Model)
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			other := focusFiles
			if panel == focusFiles {
				other = focusBranches
			}
			m.focus = other
			otherCount := len(filterTestList(m, other).VisibleItems())
			applyFilterTestCommand(&m, cmd)
			if got := filterTestList(m, panel).VisibleItems(); len(got) != 1 || !strings.Contains(got[0].FilterValue(), "match") {
				t.Fatalf("origin list did not receive its matches: %v", got)
			}
			if got := filterTestList(m, other).VisibleItems(); len(got) != otherCount {
				t.Fatalf("filter results changed another panel: %v", got)
			}
		})
	}
}

func TestModel_FilterDropsOldQueriesAndRefreshResults(t *testing.T) {
	for _, panel := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		t.Run(fmt.Sprint(panel), func(t *testing.T) {
			m := newFilteredTestModel(panel)
			updated, oldQuery := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("match")})
			m = updated.(Model)
			for range "match" {
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				m = updated.(Model)
			}
			updated, currentQuery := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("other")})
			m = updated.(Model)
			applyFilterTestCommand(&m, currentQuery)
			applyFilterTestCommand(&m, oldQuery)
			if got := filterTestList(m, panel).VisibleItems(); len(got) != 1 || !strings.Contains(got[0].FilterValue(), "other") {
				t.Fatalf("old query overwrote current matches: %v", got)
			}
			refresh := func(name string) tea.Cmd {
				var msg tea.Msg
				switch panel {
				case focusFiles:
					msg = statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: name}}}}
				case focusBranches:
					msg = branchesMsg{branches: []lore.Branch{{Name: name}}}
				case focusHistory:
					msg = historyMsg{revisions: []lore.Revision{{Hash: name, Message: name}}}
				}
				updated, cmd := m.Update(msg)
				m = updated.(Model)
				return cmd
			}
			oldRefresh := refresh("other-old")
			currentRefresh := refresh("other-new")
			applyFilterTestCommand(&m, currentRefresh)
			applyFilterTestCommand(&m, oldRefresh)
			if got := filterTestList(m, panel).VisibleItems(); len(got) != 1 || !strings.Contains(got[0].FilterValue(), "other-new") {
				t.Fatalf("old refresh overwrote current matches: %v", got)
			}
		})
	}
}

func TestModel_FilteredRefreshKeepsLaterRowsReachable(t *testing.T) {
	for _, panel := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		t.Run(fmt.Sprint(panel), func(t *testing.T) {
			m := newFilteredTestModel(panel)
			m.panelList(panel).SetSize(30, 3)
			var status lore.Status
			var branches []lore.Branch
			var revisions []lore.Revision
			for i := range 12 {
				name := fmt.Sprintf("match-%02d", i)
				status.Unstaged = append(status.Unstaged, lore.FileChange{Status: 'M', Path: name})
				branches = append(branches, lore.Branch{Name: name})
				revisions = append(revisions, lore.Revision{Hash: name, Message: name})
			}
			var refresh tea.Msg
			switch panel {
			case focusFiles:
				refresh = statusMsg{status: status}
			case focusBranches:
				refresh = branchesMsg{branches: branches}
			case focusHistory:
				refresh = historyMsg{revisions: revisions}
			}
			updated, cmd := m.Update(refresh)
			m = updated.(Model)
			applyFilterTestCommand(&m, cmd)
			updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("match")})
			m = updated.(Model)
			applyFilterTestCommand(&m, cmd)
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)

			updated, cmd = m.Update(refresh)
			m = updated.(Model)
			applyFilterTestCommand(&m, cmd)
			m.panelList(panel).Select(0)
			for range 10 {
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = updated.(Model)
			}
			if got := m.panelList(panel).Index(); got != 10 {
				t.Fatalf("ten Down keys selected index %d, want 10", got)
			}
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
			m = updated.(Model)
			if got := m.panelList(panel).SelectedItem(); got == nil || !strings.Contains(got.FilterValue(), "match-11") {
				t.Fatalf("End selected %v, want match-11", got)
			}
		})
	}
}

func TestModel_FilterCommandsKeepImmutableItemsDuringMutation(t *testing.T) {
	for _, origin := range []string{"input", "lock"} {
		for _, mutation := range []string{"stage", "lock", "folder"} {
			t.Run(origin+"/"+mutation, func(t *testing.T) {
				m := newFilteredTestModel(focusFiles)
				index := len(m.files.Items()) - 1
				file := m.files.Items()[index].(fileItem)
				ready := make(chan struct{})
				start := make(chan struct{})
				m.files.Filter = func(string, []string) []list.Rank {
					close(ready)
					<-start
					return []list.Rank{{Index: index}}
				}
				var cmd tea.Cmd
				if origin == "input" {
					updated, inputCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("match")})
					m = updated.(Model)
					cmd = inputCmd
				} else {
					m.files.FilterInput.SetValue("match")
					cmd = m.setFileLockedByPath(file.change.Path, false)
				}
				snapshot := m.files
				result := make(chan filterMatchesMsg, 1)
				var execute func(tea.Cmd)
				execute = func(cmd tea.Cmd) {
					if cmd == nil {
						return
					}
					switch msg := commandResult(cmd).(type) {
					case tea.BatchMsg:
						for _, sub := range msg {
							execute(sub)
						}
					case filterMatchesMsg:
						result <- msg
					}
				}
				var done sync.WaitGroup
				done.Add(2)
				go func() {
					defer done.Done()
					execute(cmd)
				}()
				<-ready
				go func() {
					defer done.Done()
					<-start
					switch mutation {
					case "stage":
						m.setFileStagedByPath(file.change.Path, false, true)
					case "lock":
						m.setFileLockedByPath(file.change.Path, true)
					case "folder":
						m.setDirStagedByPrefix("", false, true)
					}
				}()
				close(start)
				done.Wait()
				matches := <-result
				snapshot, _ = snapshot.Update(matches.matches)
				item, ok := snapshot.SelectedItem().(fileItem)
				if !ok || item.change.Path != file.change.Path || item.locked || item.staged {
					t.Fatalf("filter command observed later item mutation: %+v", item)
				}
			})
		}
	}
}

func TestModel_FilterInputKeepsQuerySnapshotAcrossKeys(t *testing.T) {
	for _, panel := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		t.Run(fmt.Sprint(panel), func(t *testing.T) {
			m := newFilteredTestModel(panel)
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("match")})
			m = updated.(Model)
			snapshot := m.panelList(panel).FilterInput
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
			m = updated.(Model)
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
			m = updated.(Model)
			if snapshot.Value() != "match" || m.panelList(panel).FilterValue() != "matcxh" {
				t.Fatalf("editing changed the prior query: snapshot=%q current=%q", snapshot.Value(), m.panelList(panel).FilterValue())
			}
		})
	}
}

func TestWrapFilterCommandKeepsNonFilterMessagesAndNestedBatches(t *testing.T) {
	key := tea.KeyMsg{Type: tea.KeyEsc}
	if msg := wrapFilterCommand(func() tea.Msg { return key }, focusFiles, 7)(); fmt.Sprint(msg) != fmt.Sprint(key) {
		t.Fatalf("non-filter message changed: %v", msg)
	}
	cmd := func() tea.Msg {
		return tea.BatchMsg{func() tea.Msg {
			return tea.BatchMsg{func() tea.Msg { return list.FilterMatchesMsg(nil) }}
		}}
	}
	outer := wrapFilterCommand(cmd, focusHistory, 7)().(tea.BatchMsg)
	inner := outer[0]().(tea.BatchMsg)
	msg, ok := inner[0]().(filterMatchesMsg)
	if !ok || msg.source != focusHistory || msg.generation != 7 {
		t.Fatalf("nested filter message did not retain its request: %#v", msg)
	}
}

func TestModel_FailedStageRefreshesFilteredOptimisticRow(t *testing.T) {
	m := newFilteredTestModel(focusFiles)
	m.files.SetSize(30, 3)
	m.status.Unstaged = nil
	for i := range 12 {
		m.status.Unstaged = append(m.status.Unstaged, lore.FileChange{Status: 'M', Path: fmt.Sprintf("match-%02d.txt", i)})
	}
	applyFilterTestCommand(&m, m.rebuildFileItems())
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("match")})
	m = updated.(Model)
	applyFilterTestCommand(&m, cmd)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	var failure actionDoneMsg
	var execute func(tea.Cmd)
	execute = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := commandResult(cmd).(type) {
		case tea.BatchMsg:
			for _, sub := range msg {
				execute(sub)
			}
		case actionDoneMsg:
			failure = msg
		default:
			updated, _ := m.Update(msg)
			m = updated.(Model)
		}
	}
	execute(cmd)
	if failure.err == nil || !m.files.SelectedItem().(fileItem).staged {
		t.Fatal("expected a failed command after displaying the optimistic staged row")
	}
	updated, cmd = m.Update(failure)
	m = updated.(Model)
	applyFilterTestCommand(&m, cmd)
	if item := m.files.SelectedItem().(fileItem); item.staged {
		t.Fatalf("failed stage retained the optimistic filtered row: %+v", item)
	}
	m.files.Select(0)
	for range 10 {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	if got := m.files.Index(); got != 10 {
		t.Fatalf("ten Down keys after rollback selected index %d, want 10", got)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(Model)
	if item, ok := m.files.SelectedItem().(fileItem); !ok || item.change.Path != "match-11.txt" {
		t.Fatalf("End after rollback selected %v, want match-11.txt", m.files.SelectedItem())
	}
}

func TestModel_RefreshDropsOlderResponses(t *testing.T) {
	for _, panel := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		t.Run(fmt.Sprint(panel), func(t *testing.T) {
			m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
			response := func(generation uint64, name string, err error) tea.Msg {
				switch panel {
				case focusFiles:
					return statusMsg{generation: generation, status: lore.Status{Branch: name}, err: err}
				case focusBranches:
					return branchesMsg{generation: generation, branches: []lore.Branch{{Name: name}}, err: err}
				default:
					return historyMsg{generation: generation, revisions: []lore.Revision{{Hash: name}}, err: err}
				}
			}
			oldGeneration := m.refreshGeneration
			m.refreshCmd()
			updated, _ := m.Update(response(m.refreshGeneration, "new", nil))
			m = updated.(Model)
			for _, err := range []error{nil, errors.New("old failure")} {
				updated, cmd := m.Update(response(oldGeneration, "old", err))
				m = updated.(Model)
				if cmd != nil || m.err != nil {
					t.Fatalf("old response was applied: cmd=%v err=%v", cmd, m.err)
				}
			}
			switch panel {
			case focusFiles:
				if m.status.Branch != "new" {
					t.Fatalf("old status replaced branch: %q", m.status.Branch)
				}
			case focusBranches:
				if len(m.localBranches) != 1 || m.localBranches[0].Name != "new" {
					t.Fatalf("old branches replaced current list: %v", m.localBranches)
				}
			case focusHistory:
				if len(m.revisions) != 1 || m.revisions[0].Hash != "new" {
					t.Fatalf("old history replaced current list: %v", m.revisions)
				}
			}
		})
	}
}

func TestModel_OptimisticStageInvalidatesEarlierStatus(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	oldStatus := statusMsg{generation: m.refreshGeneration, status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}}
	updated, _ := m.Update(oldStatus)
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	updated, cmd := m.Update(oldStatus)
	m = updated.(Model)
	if cmd != nil || len(m.status.Staged) != 1 || m.status.Staged[0].Path != "a.txt" || len(m.status.Unstaged) != 0 {
		t.Fatalf("old status undid optimistic staging: staged=%v unstaged=%v cmd=%v", m.status.Staged, m.status.Unstaged, cmd)
	}
}

func TestModel_LastFailedOperationRefreshesSuccessfulOverlappingStage(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- .": {Stdout: jsonCompleteSuccess},
		"--json status --scan": {Stdout: `{"tagName":"repositoryStatusRevision","data":{"branchName":"main"}}
{"tagName":"repositoryStatusFile","data":{"path":"a.txt","action":"modify","type":"file","flagStaged":true,"flagDirty":true}}
{"tagName":"repositoryStatusFile","data":{"path":"b.txt","action":"modify","type":"file","flagStaged":true,"flagDirty":true}}
` + jsonCompleteSuccess},
		"--json branch list": {Stdout: jsonCompleteSuccess},
		"--json history 50":  {Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.status.Unstaged = []lore.FileChange{{Status: 'M', Path: "a.txt"}, {Status: 'M', Path: "b.txt"}}
	m.rebuildFileItems()
	m.files.Select(1)
	updated, fileCmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	updated, folderCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(Model)
	var completion actionDoneMsg
	var execute func(tea.Cmd)
	execute = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := commandResult(cmd).(type) {
		case tea.BatchMsg:
			for _, sub := range msg {
				execute(sub)
			}
		case actionDoneMsg:
			completion = msg
		}
	}
	execute(folderCmd)
	if completion.err != nil {
		t.Fatalf("folder stage failed: %v", completion.err)
	}
	updated, cmd := m.Update(completion)
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("folder completion refreshed while the file operation remained pending")
	}
	execute(fileCmd)
	if completion.err == nil {
		t.Fatal("expected the file operation to fail")
	}
	updated, cmd = m.Update(completion)
	m = updated.(Model)
	applyFilterTestCommand(&m, cmd)
	if len(m.pendingFileOps) != 0 || len(m.status.Staged) != 2 || len(m.status.Unstaged) != 0 {
		t.Fatalf("last failure did not refresh the successful stage-all: pending=%v staged=%v unstaged=%v", m.pendingFileOps, m.status.Staged, m.status.Unstaged)
	}
}

func TestModel_FailedFolderStageRestoresOnlyOptimisticChanges(t *testing.T) {
	for _, dirPath := range []string{"", "dir"} {
		for _, unstage := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/unstage=%v", dirPath, unstage), func(t *testing.T) {
				m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
				m.status.Staged = []lore.FileChange{{Status: 'M', Path: "dir/staged.txt"}, {Status: 'A', Path: "dir/staged-dir", Directory: true}}
				if !unstage {
					m.status.Unstaged = []lore.FileChange{{Status: 'M', Path: "dir/unstaged.txt"}, {Status: 'A', Path: "dir/unstaged-dir", Directory: true}}
				}
				wantStaged := slices.Clone(m.status.Staged)
				wantUnstaged := slices.Clone(m.status.Unstaged)
				m.rebuildFileItems()
				cmd := m.toggleDirStage(dirPath)
				if unstage {
					lateChange := lore.FileChange{Status: 'M', Path: "dir/late.txt"}
					m.status.Unstaged = append(m.status.Unstaged, lateChange)
					wantUnstaged = append(wantUnstaged, lateChange)
				}
				var failure actionDoneMsg
				var execute func(tea.Cmd)
				execute = func(cmd tea.Cmd) {
					if cmd == nil {
						return
					}
					switch msg := commandResult(cmd).(type) {
					case tea.BatchMsg:
						for _, sub := range msg {
							execute(sub)
						}
					case actionDoneMsg:
						failure = msg
					}
				}
				execute(cmd)
				if failure.err == nil {
					t.Fatal("expected the folder operation to fail")
				}
				updated, _ := m.Update(failure)
				m = updated.(Model)
				if len(m.status.Staged) != len(wantStaged) || len(m.status.Unstaged) != len(wantUnstaged) {
					t.Fatalf("rollback changed unrelated staging: staged=%v unstaged=%v, want staged=%v unstaged=%v", m.status.Staged, m.status.Unstaged, wantStaged, wantUnstaged)
				}
				for _, change := range wantStaged {
					if !slices.Contains(m.status.Staged, change) {
						t.Fatalf("rollback lost staged change %v", change)
					}
				}
				for _, change := range wantUnstaged {
					if !slices.Contains(m.status.Unstaged, change) {
						t.Fatalf("rollback lost unstaged change %v", change)
					}
				}
			})
		}
	}
}

func TestModel_StageAllCompletionKeepsOpenPrompt(t *testing.T) {
	for _, prompt := range []promptKind{promptNewBranch, promptCommit} {
		t.Run(fmt.Sprint(prompt), func(t *testing.T) {
			m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
			m.pendingCommitAfterStageAll = true
			m.openCommitPrompt()
			m.prompt = prompt
			m.input.SetValue("existing text")
			updated, _ := m.Update(actionDoneMsg{opKey: "stage:."})
			m = updated.(Model)
			if m.prompt != prompt || m.input.Value() != "existing text" {
				t.Fatalf("stage completion replaced prompt %v or its text: prompt=%v text=%q", prompt, m.prompt, m.input.Value())
			}
			if m.pendingCommitAfterStageAll {
				t.Fatal("completed stage-all retained the pending commit")
			}
		})
	}
}

func TestModel_FailedStageAllClearsOnlyItsPendingCommit(t *testing.T) {
	for _, opKey := range []string{"stage:.", "stage:a.txt", "lock:a.txt", ""} {
		t.Run(opKey, func(t *testing.T) {
			m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
			m.pendingCommitAfterStageAll = true
			updated, _ := m.Update(actionDoneMsg{opKey: opKey, err: errors.New("failed")})
			m = updated.(Model)
			if want := opKey != "stage:."; m.pendingCommitAfterStageAll != want {
				t.Fatalf("pending commit = %v after %q failed, want %v", m.pendingCommitAfterStageAll, opKey, want)
			}
			updated, _ = m.Update(actionDoneMsg{opKey: "stage:."})
			m = updated.(Model)
			want := promptCommit
			if opKey == "stage:." {
				want = promptNone
			}
			if m.prompt != want || m.pendingCommitAfterStageAll {
				t.Fatalf("later successful stage-all left prompt=%v pending=%v, want prompt=%v and no pending commit", m.prompt, m.pendingCommitAfterStageAll, want)
			}
		})
	}
}

func TestModel_StageAllCompletionOpensPendingCommitPrompt(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.pendingCommitAfterStageAll = true
	updated, cmd := m.Update(actionDoneMsg{opKey: "stage:."})
	m = updated.(Model)
	if m.prompt != promptCommit || m.pendingCommitAfterStageAll || cmd == nil {
		t.Fatalf("successful stage-all left prompt=%v pending=%v cmd=%v", m.prompt, m.pendingCommitAfterStageAll, cmd)
	}
}

func TestModel_StatusMsgBatchesLockStatusForChangedPaths(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock status -- a.txt": {ExitCode: 0, Stdout: `{"tagName":"lockFileStatusBegin","data":{"count":1}}
{"tagName":"lockFileStatus","data":{"path":"a.txt","owner":"someone","lockedAt":1}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, cmd := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a non-nil batched Cmd after statusMsg")
	}

	b, ok := commandResult(cmd).(tea.BatchMsg)
	if !ok {
		t.Fatalf("commandResult(cmd) = %#v, want tea.BatchMsg", commandResult(cmd))
	}
	var lm locksMsg
	found := false
	for _, sub := range b {
		if sub == nil {
			continue
		}
		if got, ok := commandResult(sub).(locksMsg); ok {
			lm, found = got, true
		}
	}
	if !found {
		t.Fatal("expected one of the batched cmds to produce a locksMsg")
	}
	if lm.err != nil || len(lm.locks) != 1 || lm.locks[0].Path != "a.txt" {
		t.Fatalf("locksMsg = %+v, unexpected (should only cover the one changed path)", lm)
	}

	// Update() must be fed the locksMsg for m.locks/badges to actually
	// update - simulate what the runtime does after the cmd resolves.
	updated, _ = m2.Update(lm)
	m3 := updated.(Model)
	item, ok := m3.files.SelectedItem().(fileItem)
	if !ok || !item.locked {
		t.Fatalf("selected item = %+v, want a.txt marked locked after locksMsg", item)
	}
}

func TestModel_LockRefreshKeepsNewestSnapshot(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	status := statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}}
	updated, _ := m.Update(status)
	m = updated.(Model)
	old := locksMsg{requestID: m.lockRequestID}
	updated, _ = m.Update(status)
	m = updated.(Model)
	updated, _ = m.Update(locksMsg{requestID: m.lockRequestID, locks: []lore.Lock{{Path: "a.txt", Owner: "me"}}})
	m = updated.(Model)
	updated, _ = m.Update(old)
	m = updated.(Model)
	if m.locks["a.txt"].Owner != "me" || !m.files.SelectedItem().(fileItem).locked {
		t.Fatal("older response erased the newer lock")
	}
}

func TestModel_LockToggleRejectsEarlierResponses(t *testing.T) {
	for _, initiallyLocked := range []bool{false, true} {
		t.Run(fmt.Sprint(initiallyLocked), func(t *testing.T) {
			command := "acquire"
			if initiallyLocked {
				command = "release"
			}
			fake := &lore.FakeRunner{Results: map[string]lore.Result{
				"--json lock " + command + " -- a.txt": {Stdout: jsonCompleteSuccess},
			}}
			m := NewModel(fake, "repo", "/repo")
			m.currentUserID = "me"
			updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
			m = updated.(Model)
			old := locksMsg{requestID: m.lockRequestID}
			if initiallyLocked {
				old.locks = []lore.Lock{{Path: "a.txt", Owner: "me"}}
			}
			updated, _ = m.Update(old)
			m = updated.(Model)
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
			m = updated.(Model)
			updated, _ = m.Update(old)
			m = updated.(Model)
			if m.files.SelectedItem().(fileItem).locked == initiallyLocked {
				t.Fatal("earlier response undid the optimistic toggle")
			}
			var done actionDoneMsg
			var execute func(tea.Cmd)
			execute = func(cmd tea.Cmd) {
				switch msg := commandResult(cmd).(type) {
				case tea.BatchMsg:
					for _, sub := range msg {
						execute(sub)
					}
				case actionDoneMsg:
					done = msg
				}
			}
			execute(cmd)
			if done.err != nil || done.opKey == "" {
				t.Fatalf("lock command did not succeed: %+v", done)
			}
			old.requestID = m.lockRequestID
			updated, _ = m.Update(done)
			m = updated.(Model)
			updated, _ = m.Update(old)
			m = updated.(Model)
			_, locked := m.locks["a.txt"]
			if locked == initiallyLocked || m.files.SelectedItem().(fileItem).locked == initiallyLocked {
				t.Fatal("a response from before completion undid the confirmed toggle")
			}
		})
	}
}

func TestModel_LocksMsgErrorIsSwallowed(t *testing.T) {
	// Locking requires an online remote; a failure here (e.g. offline) must
	// not raise the main error banner on every refresh.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(locksMsg{err: errors.New("offline")})
	m2 := updated.(Model)
	if m2.err != nil {
		t.Fatalf("err = %v, want nil (locks are best-effort)", m2.err)
	}
}

func TestModel_StatusMsgPopulatesFilesList(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	s := lore.Status{
		Repository: "abc",
		Staged:     []lore.FileChange{{Status: 'A', Path: "a.txt"}},
	}
	updated, _ := m.Update(statusMsg{status: s})
	m2 := updated.(Model)
	if len(m2.files.Items()) != 1 {
		t.Fatalf("files list has %d items, want 1", len(m2.files.Items()))
	}
}

func TestModel_TabCyclesFocusForward(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	if m.focus != focusFiles {
		t.Fatalf("initial focus = %v, want focusFiles", m.focus)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.focus != focusBranches {
		t.Fatalf("focus after Tab = %v, want focusBranches", m2.focus)
	}
}

func TestModel_SpaceOnUnstagedFileDispatchesStageCmd(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for Space on an unstaged file")
	}
	c := commandResult(cmd)
	// handle if batched with status set
	if b, ok := c.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				res := commandResult(item)
				if am, ok := res.(actionDoneMsg); ok {
					c = am
					break
				}
				if dm, ok := res.(diffMsg); ok {
					c = dm
					break
				}
			}
		}
	}
	am, ok := c.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", c)
	}
	if am.err != nil {
		t.Fatalf("unexpected error: %v", am.err)
	}
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "stage" {
		t.Fatalf("Calls = %+v, want a single stage call", fake.Calls)
	}
}

func TestModel_EnterOnFileDispatchesLoadDiffCmd(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json diff -- a.txt": {ExitCode: 0, Stdout: `{"tagName":"fileDiff","data":{"path":"a.txt","patch":"+++ a.txt\n","action":"keep"}}
` + jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for Enter on a file")
	}
	c := commandResult(cmd)
	if b, ok := c.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				res := commandResult(item)
				if dm, ok := res.(diffMsg); ok {
					c = dm
					break
				}
			}
		}
	}
	dm, ok := c.(diffMsg)
	if !ok {
		t.Fatalf("msg = %#v, want diffMsg", c)
	}
	if dm.text != "+++ a.txt\n" {
		t.Fatalf("text = %q", dm.text)
	}
}

func TestModel_CommitPromptSubmitsMessage(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json commit -- hi": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updatedStatus, _ := m.Update(statusMsg{status: lore.Status{Staged: []lore.FileChange{{Status: 'A', Path: "a.txt"}}}})
	m = updatedStatus.(Model)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m2 := updated.(Model)
	if m2.prompt != promptCommit {
		t.Fatalf("prompt = %v, want promptCommit", m2.prompt)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	m3 := updated.(Model)

	_, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd when submitting the commit prompt")
	}
	msg := commandResult(cmd)
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err != nil {
		t.Fatalf("unexpected error: %v", am.err)
	}
	if len(fake.Calls) != 1 || len(fake.Calls[0]) != 4 || fake.Calls[0][1] != "commit" || fake.Calls[0][2] != "--" || fake.Calls[0][3] != "hi" {
		t.Fatalf("Calls = %+v, want a single commit call with message \"hi\"", fake.Calls)
	}
}

func TestModel_ActionDoneMsgAppendsToCommandLogAndRefreshes(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json status --scan": {ExitCode: 0, Stdout: `{"tagName":"repositoryStatusRevision","data":{"repository":"x","branchName":"main"}}
` + jsonCompleteSuccess},
		"--json branch list": {ExitCode: 0, Stdout: `{"tagName":"branchListEntry","data":{"location":"local","name":"main","isCurrent":true}}
` + jsonCompleteSuccess},
		"--json history 50": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, cmd := m.Update(actionDoneMsg{label: "Stage file", commands: []string{"lore stage a.txt"}})
	m2 := updated.(Model)
	if want := "Stage file\n  lore stage a.txt"; m2.log.View() != want {
		t.Fatalf("log.View() = %q, want %q", m2.log.View(), want)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil refresh Cmd after a successful action")
	}
}

func TestModel_ActionDoneMsgSkipsRefreshWhileAnotherFileOpIsPending(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.pendingFileOps = map[string]bool{"stage:src": true} // the folder-stage is still in flight

	updated, cmd := m.Update(actionDoneMsg{label: "Stage file", opKey: "stage:a.txt", commands: []string{"lore stage a.txt"}})
	m2 := updated.(Model)
	if cmd != nil {
		t.Fatal("expected no refresh Cmd while another file op is still pending")
	}
	if !m2.pendingFileOps["stage:src"] {
		t.Fatal("the other still-pending op must not have been cleared")
	}

	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json status --scan": {ExitCode: 0, Stdout: `{"tagName":"repositoryStatusRevision","data":{"repository":"x","branchName":"main"}}
` + jsonCompleteSuccess},
		"--json branch list": {ExitCode: 0, Stdout: `{"tagName":"branchListEntry","data":{"location":"local","name":"main","isCurrent":true}}
` + jsonCompleteSuccess},
		"--json history 50": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m2.runner = fake
	updated, cmd = m2.Update(actionDoneMsg{label: "Stage folder", opKey: "stage:src", commands: []string{"lore stage src"}})
	m3 := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a refresh Cmd once the last pending file op completes")
	}
	if len(m3.pendingFileOps) != 0 {
		t.Fatalf("pendingFileOps = %+v, want empty", m3.pendingFileOps)
	}
}

func TestModel_HistoryMsgUpdatesItemsEvenWhileFiltering(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusHistory
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{{Number: 1, Message: "first"}}})
	m2 := updated.(Model)

	var filterCmd tea.Cmd
	m2.history, filterCmd = m2.history.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_ = filterCmd
	if !m2.history.SettingFilter() {
		t.Fatal("expected history list to be in filter-typing state after \"/\"")
	}

	updated, _ = m2.Update(historyMsg{revisions: []lore.Revision{
		{Number: 1, Message: "first"},
		{Number: 2, Message: "second"},
		{Number: 3, Message: "third"},
	}})
	m3 := updated.(Model)
	if len(m3.history.Items()) != 3 {
		t.Fatalf("history items = %d, want 3 (must update even while filtering)", len(m3.history.Items()))
	}
}

func TestModel_RefreshBranchesListPropagatesFilterCmd(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(branchesMsg{branches: []lore.Branch{{Name: "main", Current: true}, {Name: "dev"}}})
	m2 := updated.(Model)
	m2.focus = focusBranches

	var filterCmd tea.Cmd
	m2.branches, filterCmd = m2.branches.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_ = filterCmd
	if !m2.branches.SettingFilter() {
		t.Fatal("expected branches list to be in filter-typing state after \"/\"")
	}

	_, cmd := m2.Update(branchesMsg{branches: []lore.Branch{{Name: "main", Current: true}, {Name: "dev"}}})
	if cmd == nil {
		t.Fatal("expected refreshBranchesList's SetItems cmd to be propagated while filtering, got nil")
	}
}

func TestModel_LeftStackAndRightColumnPanelOrder(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	v := m2.View()

	for _, want := range []string{"[1]─Status", "[2]─Files", "[3]─", "[4]─History", "[5]─Diff", "Command Log"} {
		if !strings.Contains(v, want) {
			t.Fatalf("View must contain %q; got:\n%s", want, v)
		}
	}

	// Left stack order: Files above History.
	filesIdx := strings.Index(v, "[2]─Files")
	histIdx := strings.Index(v, "[4]─History")
	if filesIdx == -1 || histIdx == -1 || histIdx < filesIdx {
		t.Fatalf("History should appear below Files in left stack; filesIdx=%d histIdx=%d", filesIdx, histIdx)
	}

	// Right column: Command Log directly below Diff.
	diffIdx := strings.Index(v, "[5]─Diff")
	logIdx := strings.Index(v, "Command Log")
	if diffIdx == -1 || logIdx == -1 || logIdx < diffIdx {
		t.Fatalf("Command Log should appear after/below Diff in right column; diffIdx=%d logIdx=%d", diffIdx, logIdx)
	}
}
