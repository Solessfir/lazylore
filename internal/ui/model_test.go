package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

func TestModel_StatusMsgPopulatesFilesList(t *testing.T) {
	m := NewModel(&lore.FakeRunner{})
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
	m := NewModel(&lore.FakeRunner{})
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
		"stage a.txt":   {ExitCode: 0},
		"status --scan": {ExitCode: 0, Stdout: "Repository x\nOn branch main revision 0 -> 0\n"},
	}}
	m := NewModel(fake)
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for Space on an unstaged file")
	}
	msg := cmd()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err != nil {
		t.Fatalf("unexpected error: %v", am.err)
	}
	if len(fake.Calls) != 1 || fake.Calls[0][0] != "stage" {
		t.Fatalf("Calls = %+v, want a single stage call", fake.Calls)
	}
}

func TestModel_EnterOnFileDispatchesLoadDiffCmd(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"diff a.txt": {ExitCode: 0, Stdout: "+++ a.txt\n"},
	}}
	m := NewModel(fake)
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for Enter on a file")
	}
	msg := cmd()
	dm, ok := msg.(diffMsg)
	if !ok {
		t.Fatalf("msg = %#v, want diffMsg", msg)
	}
	if dm.text != "+++ a.txt\n" {
		t.Fatalf("text = %q", dm.text)
	}
}

func TestModel_CommitPromptSubmitsMessage(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"commit hi": {ExitCode: 0},
	}}
	m := NewModel(fake)

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
	if len(fake.Calls) != 1 || fake.Calls[0][0] != "commit" || fake.Calls[0][1] != "hi" {
		t.Fatalf("Calls = %+v, want a single commit call with message \"hi\"", fake.Calls)
	}
}

func TestModel_ActionDoneMsgAppendsToCommandLogAndRefreshes(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"status --scan":        {ExitCode: 0, Stdout: "Repository x\nOn branch main revision 0 -> 0\n"},
		"branch list":          {ExitCode: 0, Stdout: "Local branches:\n* main\nRemote branches:\n"},
		"history --oneline 50": {ExitCode: 0, Stdout: ""},
	}}
	m := NewModel(fake)
	updated, cmd := m.Update(actionDoneMsg{label: "stage a.txt"})
	m2 := updated.(Model)
	if len(m2.log.entries) != 1 || m2.log.entries[0] != "stage a.txt: OK" {
		t.Fatalf("log.entries = %+v", m2.log.entries)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil refresh Cmd after a successful action")
	}
}
