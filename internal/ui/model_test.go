package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

func TestModel_StatusMsgPopulatesFilesList(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo")
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
	m := NewModel(&lore.FakeRunner{}, "test-repo")
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
	m := NewModel(fake, "test-repo")
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
	m := NewModel(fake, "test-repo")
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
	m := NewModel(fake, "test-repo")

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
	m := NewModel(fake, "test-repo")
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
	m := NewModel(&lore.FakeRunner{}, "test-repo")
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
	m := NewModel(&lore.FakeRunner{}, "test-repo")
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

func TestModel_StashPanelRendersBelowHistory(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo")
	// Provide size so resize and View produce panels.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	// When not focused, stash should use small default size (Size:3 outer -> height 1 inner) like lazygit.
	if m2.focus != focusStash && m2.stashHeight > 2 {
		t.Fatalf("unfocused Stash should have small base height (Size=3), got stashHeight=%d", m2.stashHeight)
	}
	// Simulate selecting Stash: base should expand.
	m2.focus = focusStash
	m2.recomputePanelHeights()
	if m2.stashHeight <= 2 {
		t.Fatalf("focused Stash should have expanded base height, got stashHeight=%d", m2.stashHeight)
	}
	v := m2.View()
	if !strings.Contains(v, "[5]─Stash") {
		t.Fatalf("View must contain Stash panel title '[5]─Stash' (lazygit layout); got:\n%s", v)
	}
	if !strings.Contains(v, "[4]─History") {
		t.Fatalf("View must still contain History panel; got:\n%s", v)
	}
	// Rough order check: History title appears before Stash title in the rendered string.
	histIdx := strings.Index(v, "[4]─History")
	stashIdx := strings.Index(v, "[5]─Stash")
	if histIdx == -1 || stashIdx == -1 || stashIdx < histIdx {
		t.Fatalf("Stash panel should appear below History in left stack; histIdx=%d stashIdx=%d", histIdx, stashIdx)
	}

	// Command log must appear below the Diff (not as a full-width footer under left+right).
	if !strings.Contains(v, "Command Log") {
		t.Fatalf("View must contain Command Log panel under Diff; got:\n%s", v)
	}
	if !strings.Contains(v, "[6]─Diff") {
		t.Fatalf("View must contain Diff panel titled 6; got:\n%s", v)
	}
	// In the text, the Command Log title should come after the Diff title (because right column is vertical Join of diff then log).
	diffIdx := strings.Index(v, "[6]─Diff")
	logIdx := strings.Index(v, "Command Log")
	if diffIdx == -1 || logIdx == -1 || logIdx < diffIdx {
		t.Fatalf("Command Log should appear after/below Diff in right column; diffIdx=%d logIdx=%d", diffIdx, logIdx)
	}

	// Stash must not contain "No items." text (lazygit behavior for the empty stash panel).
	// We look for the stash section roughly by its title.
	stashTitleIdx := strings.Index(v, "[5]─Stash")
	if stashTitleIdx >= 0 {
		// Take a window after the stash title (enough for the small panel height).
		stashSection := v[stashTitleIdx:]
		if strings.Contains(strings.ToLower(stashSection), "no items") {
			t.Fatalf("Stash panel must not contain 'No items.' text (matches lazygit); got section:\n%s", stashSection[:min(len(stashSection), 300)])
		}
		// Regression: an empty Stash panel used to render as a totally blank,
		// unlabeled box, indistinguishable from broken. It must say plainly
		// that lore has no native stash.
		if !strings.Contains(stashSection, "lore has no stash") {
			t.Fatalf("empty Stash panel must explain lore has no native stash; got section:\n%s", stashSection[:min(len(stashSection), 300)])
		}
	}
}
