package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

func TestModel_SetAppStatusMsgDoesNotShowSpinnerImmediately(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, cmd := m.Update(setAppStatusMsg("Staging..."))
	m2 := updated.(Model)
	if m2.appStatus != "" {
		t.Fatalf("appStatus = %q, want empty until the reveal delay elapses", m2.appStatus)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd (the delayed reveal)")
	}
}

func TestModel_RevealStatusMsgShowsSpinnerWhenStillCurrent(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(setAppStatusMsg("Staging..."))
	m2 := updated.(Model)

	updated, cmd := m2.Update(revealStatusMsg{gen: m2.statusGen, text: "Staging..."})
	m3 := updated.(Model)
	if m3.appStatus != "Staging..." {
		t.Fatalf("appStatus = %q, want %q", m3.appStatus, "Staging...")
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd to kick off the spinner tick loop")
	}
}

func TestModel_RevealStatusMsgSkippedWhenActionAlreadyFinished(t *testing.T) {
	// Regression: the whole point of the delay is that a fast action (e.g.
	// stage/unstage) completes and clears appStatus before the reveal timer
	// fires - the stale reveal must not un-clear it and flash the spinner
	// after the fact.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(setAppStatusMsg("Staging..."))
	m2 := updated.(Model)
	staleGen := m2.statusGen

	// Action finishes before the reveal fires.
	updated, _ = m2.Update(statusMsg{status: lore.Status{Branch: "main"}})
	m3 := updated.(Model)

	updated, cmd := m3.Update(revealStatusMsg{gen: staleGen, text: "Staging..."})
	m4 := updated.(Model)
	if m4.appStatus != "" {
		t.Fatalf("appStatus = %q, want empty - the reveal was for an action that already finished", m4.appStatus)
	}
	if cmd != nil {
		t.Fatal("expected a nil Cmd for a stale reveal (no spinner tick loop to start)")
	}
}

func TestModel_HistoryRecolorsWhenStatusArrivesAfterHistory(t *testing.T) {
	// Regression: statusMsg and historyMsg load independently (Init()
	// batches both) with no ordering guarantee. Unpushed coloring depends
	// on data from BOTH - if historyMsg happens to land first, the initial
	// render has no remote info yet (nothing marked unpushed); statusMsg
	// landing afterward must still recolor the already-rendered list, not
	// wait for a future historyMsg that may never come.
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
	// Regression: tabbing into the Diff panel itself must keep showing
	// whichever panel's content (Log/Patch/diff) is actually loaded, not
	// fall back to reading the Files selection just because m.focus is no
	// longer Files/Branches/History.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusBranches
	(&m).ensureMainContent()
	m.focus = focusDiff
	if got := m.mainPanelTitle(); got != "Log" {
		t.Fatalf("mainPanelTitle() after tabbing from Branches into Diff = %q, want %q", got, "Log")
	}
}

func TestModel_EnsureMainContent_SelectingDirectoryClearsDiff(t *testing.T) {
	// Matches lazygit: selecting a directory clears the main panel instead
	// of leaving the last-selected file's diff (including its "Locked by
	// ..." line) stuck on screen.
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
		"--json history 50 --branch dev": {ExitCode: 0, Stdout: `{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}` + "\n"},
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
	msg := cmd()
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
	msg := cmd()
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
	msg := cmd()
	dm, ok := msg.(diffMsg)
	if !ok || !dm.raw || !strings.Contains(dm.text, "Initial revision") {
		t.Fatalf("msg = %#v, want a raw diffMsg explaining there's no parent to diff against", msg)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("Calls = %+v, want no runner calls for the root revision", fake.Calls)
	}
}

func TestRowsPerPage_ReservesOneRowForBubblesFilterInput(t *testing.T) {
	// bubbles/list always reserves one row for a potential filter input
	// (tied to filteringEnabled, independent of ShowTitle), so real per-page
	// capacity is contentH-1, not contentH - see rowsPerPage's doc comment.
	if got := rowsPerPage(10); got != 9 {
		t.Fatalf("rowsPerPage(10) = %d, want 9", got)
	}
	if got := rowsPerPage(1); got != 1 {
		t.Fatalf("rowsPerPage(1) = %d, want 1 (floor of 1)", got)
	}
}

func TestRowClickTarget_RealItemOnCurrentPage(t *testing.T) {
	vis := make([]list.Item, 10)
	target, ok := rowClickTarget(0, 8, vis, 2)
	if !ok || target != 2 {
		t.Fatalf("target=%d ok=%v, want 2,true", target, ok)
	}
}

func TestRowClickTarget_RejectsPaddingRowEvenWhenGloballyInRange(t *testing.T) {
	// Regression: page 0 has exactly 8 real items (perPage=8), 10 items total
	// across 2 pages. Row 8 is blank padding on page 0, but 8 < len(vis)=10 -
	// the old naive `target < len(vis)` bound would wrongly resolve this to
	// item 8, which lives on page 1 and isn't even visible, silently flipping
	// Select()'s page and desyncing every click after it.
	vis := make([]list.Item, 10)
	_, ok := rowClickTarget(0, 8, vis, 8)
	if ok {
		t.Fatal("expected relY=8 (padding row) to be rejected, not resolved to a hidden item")
	}
}

func TestRowClickTarget_RejectsRowPastLastRealItemOnPartialPage(t *testing.T) {
	vis := make([]list.Item, 3)
	_, ok := rowClickTarget(0, 8, vis, 3)
	if ok {
		t.Fatal("expected relY=3 (past the 3 real items) to be rejected")
	}
}

func TestRowClickTarget_SecondPageOffsetsCorrectly(t *testing.T) {
	vis := make([]list.Item, 10)
	target, ok := rowClickTarget(1, 8, vis, 1)
	if !ok || target != 9 {
		t.Fatalf("target=%d ok=%v, want 9,true (page 1 starts at absolute index 8)", target, ok)
	}
}

func TestRowClickTarget_AcceptsLastRealItemEvenWhenPaginatorPerPageWouldBeStale(t *testing.T) {
	// Regression: View() calls list.Model.SetSize with a footer-shrink-
	// adjusted (taller) height on every render, but View() has a value
	// receiver, so that call never persists back to the real model -
	// Paginator.PerPage silently drifts stale (commonly by exactly one row)
	// relative to what's actually rendered. handleMouseClick must compute
	// perPage fresh (via rowsPerPage) from what's actually on screen instead
	// of trusting the model's own (possibly stale) Paginator.PerPage - this
	// is why rowClickTarget takes perPage as an explicit argument rather
	// than reading it off a paginator.Model.
	vis := make([]list.Item, 9)
	target, ok := rowClickTarget(0, rowsPerPage(10), vis, 8)
	if !ok || target != 8 {
		t.Fatalf("target=%d ok=%v, want 8,true (the 9th and last item, on a 10-row-tall panel)", target, ok)
	}
}

func TestModel_ClickDoesNotPageJumpWhenPaginatorWasStale(t *testing.T) {
	// Regression: rowClickTarget already computed the right target using a
	// freshly-derived perPage, but list.Model.Select (bubbles' own method)
	// re-derives Page by dividing by its OWN Paginator.PerPage internally -
	// which was still the stale (pre-resize) value at click time. Select
	// could then land on a DIFFERENT page than the one actually on screen,
	// causing a visible jump on the next render even though the click
	// itself resolved to the correct item. handleMouseClick must resync
	// each list's SetSize to the current frame's height before calling
	// Select, so Select's own division uses the same perPage rowClickTarget
	// did.
	files := []lore.FileChange{
		{Status: 'A', Path: "Plugins/LoreSourceControl/Config"},
		{Status: 'A', Path: "Plugins/LoreSourceControl/Config/FilterPlugin.ini"},
		{Status: 'M', Path: "Plugins/LoreSourceControl/LoreSourceControl.uplugin"},
		{Status: 'A', Path: "Plugins/LoreSourceControl/README.md"},
		{Status: 'A', Path: "Plugins/LoreSourceControl/Resources/Icon128.png"},
	}
	m := NewModel(&lore.FakeRunner{}, "Sus", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 155, Height: 40})
	m2 := updated.(Model)
	updated, _ = m2.Update(statusMsg{status: lore.Status{Unstaged: files}})
	m3 := updated.(Model)

	items := m3.files.Items()
	iconIdx := -1
	for i, it := range items {
		if fi, ok := it.(fileItem); ok && fi.path == "Plugins/LoreSourceControl/Resources/Icon128.png" {
			iconIdx = i
		}
	}
	if iconIdx == -1 {
		t.Fatal("Icon128.png not found in flattened items")
	}
	if got := m3.files.Paginator.PerPage; got != 8 {
		t.Fatalf("precondition failed: Paginator.PerPage = %d, want the stale value 8 (real per-page capacity is 9)", got)
	}

	// statusPanelHeight(3) + files top border(1) + row offset = Icon128.png's row.
	clickY := 3 + 1 + iconIdx
	updated, _ = m3.Update(tea.MouseMsg{X: 10, Y: clickY, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m4 := updated.(Model)

	sel, ok := m4.files.SelectedItem().(fileItem)
	if !ok || sel.path != "Plugins/LoreSourceControl/Resources/Icon128.png" {
		t.Fatalf("click did not select Icon128.png; got %+v", sel)
	}
	if m4.files.Paginator.Page != 0 {
		t.Fatalf("Paginator.Page = %d, want 0 - the click should not have paged away from what was on screen", m4.files.Paginator.Page)
	}
}

func TestChangedPaths_CombinesStagedThenUnstaged(t *testing.T) {
	s := lore.Status{
		Staged:   []lore.FileChange{{Status: 'A', Path: "a.txt"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "b.txt"}},
	}
	paths := changedPaths(s)
	want := []string{"a.txt", "b.txt"}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("changedPaths = %+v, want %+v", paths, want)
	}
}

func TestModel_StatusMsgBatchesLockStatusForChangedPaths(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock status a.txt": {ExitCode: 0, Stdout: `{"tagName":"lockFileStatusBegin","data":{"count":1}}
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

	b, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("cmd() = %#v, want tea.BatchMsg", cmd())
	}
	var lm locksMsg
	found := false
	for _, sub := range b {
		if sub == nil {
			continue
		}
		if got, ok := sub().(locksMsg); ok {
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
		"--json stage a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for Space on an unstaged file")
	}
	c := cmd()
	// handle if batched with status set
	if b, ok := c.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				res := item()
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
		"--json diff a.txt": {ExitCode: 0, Stdout: `{"tagName":"fileDiff","data":{"path":"a.txt","patch":"+++ a.txt\n","action":"keep"}}
` + jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for Enter on a file")
	}
	c := cmd()
	if b, ok := c.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				res := item()
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
		"--json commit hi": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")

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
	msg := cmd()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err != nil {
		t.Fatalf("unexpected error: %v", am.err)
	}
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "commit" || fake.Calls[0][2] != "hi" {
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

func TestModel_HistoryMsgUpdatesTotalEvenWhileFiltering(t *testing.T) {
	// Regression: historyMsg used to return early (skipping historyTotal and
	// chrome bookkeeping) whenever list.SetItems returned a non-nil cmd,
	// which only happens while the panel has an active filter.
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
	if m3.historyTotal != 3 {
		t.Fatalf("historyTotal = %d, want 3 (must update even while filtering)", m3.historyTotal)
	}
}

func TestModel_RefreshBranchesListPropagatesFilterCmd(t *testing.T) {
	// Regression: refreshBranchesList silently dropped the cmd SetItems
	// returns while the Branches panel has an active filter, so a filtered
	// view never got reconciled after a refresh.
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
