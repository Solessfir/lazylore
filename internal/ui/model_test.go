package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

func TestRowClickTarget_RealItemOnCurrentPage(t *testing.T) {
	p := paginator.Model{Page: 0, PerPage: 8}
	vis := make([]list.Item, 10)
	target, ok := rowClickTarget(p, vis, 2)
	if !ok || target != 2 {
		t.Fatalf("target=%d ok=%v, want 2,true", target, ok)
	}
}

func TestRowClickTarget_RejectsPaddingRowEvenWhenGloballyInRange(t *testing.T) {
	// Regression: page 0 has exactly 8 real items (PerPage=8), 10 items total
	// across 2 pages. Row 8 is blank padding on page 0 (bubbles/list always
	// reserves a filter-input row that lazylore's border rendering strips
	// and re-pads at the bottom - see rowClickTarget's doc comment), but
	// 8 < len(vis)=10 - the old naive `target < len(vis)` bound would wrongly
	// resolve this to item 8, which lives on page 1 and isn't even visible,
	// silently flipping Select()'s page and desyncing every click after it.
	p := paginator.Model{Page: 0, PerPage: 8}
	vis := make([]list.Item, 10)
	_, ok := rowClickTarget(p, vis, 8)
	if ok {
		t.Fatal("expected relY=8 (padding row) to be rejected, not resolved to a hidden item")
	}
}

func TestRowClickTarget_RejectsRowPastLastRealItemOnPartialPage(t *testing.T) {
	p := paginator.Model{Page: 0, PerPage: 8}
	vis := make([]list.Item, 3)
	_, ok := rowClickTarget(p, vis, 3)
	if ok {
		t.Fatal("expected relY=3 (past the 3 real items) to be rejected")
	}
}

func TestRowClickTarget_SecondPageOffsetsCorrectly(t *testing.T) {
	p := paginator.Model{Page: 1, PerPage: 8}
	vis := make([]list.Item, 10)
	target, ok := rowClickTarget(p, vis, 1)
	if !ok || target != 9 {
		t.Fatalf("target=%d ok=%v, want 9,true (page 1 starts at absolute index 8)", target, ok)
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
	updated, cmd := m.Update(actionDoneMsg{label: "stage a.txt"})
	m2 := updated.(Model)
	if len(m2.log.entries) != 1 || m2.log.entries[0] != "stage a.txt: OK" {
		t.Fatalf("log.entries = %+v", m2.log.entries)
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

	for _, want := range []string{"Status", "[1]─Files", "[3]─", "[4]─History", "[5]─Diff", "Command Log"} {
		if !strings.Contains(v, want) {
			t.Fatalf("View must contain %q; got:\n%s", want, v)
		}
	}

	// Left stack order: Files above History.
	filesIdx := strings.Index(v, "[1]─Files")
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
