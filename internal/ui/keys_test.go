package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

// runBatch executes cmd and, if it's a tea.Batch, every sub-cmd too - real
// ordering/concurrency doesn't matter here, only that every runner call in
// the batch actually happens.
func runBatch(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if b, ok := cmd().(tea.BatchMsg); ok {
		for _, sub := range b {
			runBatch(sub)
		}
	}
}

func TestModel_JumpToBranchesLoadsLogForSelectedBranch(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json history 50 --branch main": {ExitCode: 0, Stdout: `{"tagName":"revisionHistoryEntry","data":{"revision":"abc123","revisionNumber":1,"parent":["0000000000000000000000000000000000000000000000000000000000000000","0000000000000000000000000000000000000000000000000000000000000000"]}}
{"tagName":"metadata","data":{"key":"message","value":{"tagName":"string","data":"initial"}}}
` + jsonCompleteSuccess,
		},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(branchesMsg{branches: []lore.Branch{{Name: "main", Current: true}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after jumping to Branches")
	}
	msg := cmd()
	dm, ok := msg.(diffMsg)
	if !ok || !dm.raw {
		t.Fatalf("msg = %#v, want a raw diffMsg (Log content)", msg)
	}
	if len(fake.Calls) != 1 || fake.Calls[0][4] != "main" {
		t.Fatalf("Calls = %+v, want a single history --branch main call", fake.Calls)
	}
}

func TestModel_JumpToHistoryLoadsPatchForSelectedRevision(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json diff --source parenthash --target abc123": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{{Number: 1, Hash: "abc123", Parent: "parenthash", Message: "first"}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after jumping to History")
	}
	msg := cmd()
	if dm, ok := msg.(diffMsg); !ok || dm.raw {
		t.Fatalf("msg = %#v, want a non-raw diffMsg (Patch content, diff-colored)", msg)
	}
}

func TestModel_DKeyOnHistoryOpensRevertConfirmWithRevisionHash(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusHistory
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{{Number: 1, Hash: "abcdef1234567890", Message: "oops"}}})
	m2 := updated.(Model)

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)
	if m3.prompt != promptConfirmRevert {
		t.Fatalf("prompt = %v, want promptConfirmRevert", m3.prompt)
	}
	if m3.pendingResetRevision != "abcdef1234567890" {
		t.Fatalf("pendingResetRevision = %q, want the full hash", m3.pendingResetRevision)
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd yet (confirmation pending), got %v", cmd)
	}
}

func TestModel_YKeyConfirmsRevert(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json revision revert abcdef12": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.focus = focusHistory
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{{Number: 1, Hash: "abcdef12", Message: "oops"}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after y = %v, want promptNone", m4.prompt)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after confirming revert")
	}
	runBatch(cmd)
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "revision" || fake.Calls[0][2] != "revert" || fake.Calls[0][3] != "abcdef12" {
		t.Fatalf("Calls = %+v, want a single revision revert call", fake.Calls)
	}
}

func TestModel_EscCancelsRevertPromptWithoutRunnerCalls(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json revision revert abcdef12": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.focus = focusHistory
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{{Number: 1, Hash: "abcdef12", Message: "oops"}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after esc = %v, want promptNone", m4.prompt)
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd after cancelling, got %v", cmd)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("expected no runner calls after cancelling, got %+v", fake.Calls)
	}
}

func TestModel_SpaceOnBranchesChecksOutSelectedBranch(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json branch switch dev": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.focus = focusBranches
	updated, _ := m.Update(branchesMsg{branches: []lore.Branch{{Name: "main", Current: true}, {Name: "dev"}}})
	m2 := updated.(Model)
	m2.branches.Select(1) // "dev"

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for space on a branch")
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				if am, ok := item().(actionDoneMsg); ok {
					msg = am
					break
				}
			}
		}
	}
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err != nil {
		t.Fatalf("unexpected error: %v", am.err)
	}
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "branch" || fake.Calls[0][2] != "switch" || fake.Calls[0][3] != "dev" {
		t.Fatalf("Calls = %+v, want a single branch switch call", fake.Calls)
	}
}

func TestModel_GKeyOnBranchesOpensResetConfirmWithBranchLatest(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusBranches
	updated, _ := m.Update(branchesMsg{branches: []lore.Branch{
		{Name: "main", Current: true},
		{Name: "dev", Latest: "abc123"},
	}})
	m2 := updated.(Model)
	m2.branches.Select(1) // "dev"

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m3 := updated.(Model)
	if m3.prompt != promptConfirmBranchReset {
		t.Fatalf("prompt = %v, want promptConfirmBranchReset", m3.prompt)
	}
	if m3.pendingResetRevision != "abc123" {
		t.Fatalf("pendingResetRevision = %q, want %q", m3.pendingResetRevision, "abc123")
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd yet (confirmation pending), got %v", cmd)
	}
}

func TestModel_YKeyConfirmsBranchReset(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json branch reset abc123": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.focus = focusBranches
	updated, _ := m.Update(branchesMsg{branches: []lore.Branch{
		{Name: "main", Current: true},
		{Name: "dev", Latest: "abc123"},
	}})
	m2 := updated.(Model)
	m2.branches.Select(1)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after y = %v, want promptNone", m4.prompt)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after confirming reset")
	}
	runBatch(cmd)
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "branch" || fake.Calls[0][2] != "reset" || fake.Calls[0][3] != "abc123" {
		t.Fatalf("Calls = %+v, want a single branch reset call", fake.Calls)
	}
}

func TestModel_SpaceOnHistoryChecksOutSelectedRevision(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json sync abc123": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.focus = focusHistory
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{{Number: 1, Hash: "abc123", Message: "first"}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for space on a revision")
	}
	runBatch(cmd)
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "sync" || fake.Calls[0][2] != "abc123" {
		t.Fatalf("Calls = %+v, want a single sync call", fake.Calls)
	}
}

func TestModel_GKeyOnHistoryOpensResetConfirmWithRevisionHash(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusHistory
	updated, _ := m.Update(historyMsg{revisions: []lore.Revision{{Number: 1, Hash: "abcdef1234567890", Message: "first"}}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m3 := updated.(Model)
	if m3.prompt != promptConfirmBranchReset {
		t.Fatalf("prompt = %v, want promptConfirmBranchReset", m3.prompt)
	}
	if m3.pendingResetRevision != "abcdef1234567890" {
		t.Fatalf("pendingResetRevision = %q, want the full hash", m3.pendingResetRevision)
	}
}

func TestModel_LKeyOnUnlockedFileAcquiresLock(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock acquire a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for L on a file")
	}
	runBatch(cmd)
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "lock" || fake.Calls[0][2] != "acquire" {
		t.Fatalf("Calls = %+v, want a single lock acquire call", fake.Calls)
	}
}

func TestModel_LKeyOnLockedFileReleasesLock(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock release a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(locksMsg{locks: []lore.Lock{{Path: "a.txt", Owner: "someone"}}})
	m3 := updated.(Model)

	_, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for L on a locked file")
	}
	runBatch(cmd)
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "lock" || fake.Calls[0][2] != "release" {
		t.Fatalf("Calls = %+v, want a single lock release call", fake.Calls)
	}
}

func TestModel_ShiftDOnFilesOpensDiscardAllConfirmPrompt(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m2 := updated.(Model)
	if m2.prompt != promptConfirmDiscardAll {
		t.Fatalf("prompt = %v, want promptConfirmDiscardAll", m2.prompt)
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd yet (confirmation pending), got %v", cmd)
	}
}

func TestModel_YKeyConfirmsDiscardAllForEveryChangedPath(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage a.txt b.txt":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge a.txt b.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{
		Staged:   []lore.FileChange{{Status: 'A', Path: "a.txt"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "b.txt"}},
	}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after y = %v, want promptNone", m4.prompt)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after confirming discard-all")
	}
	c := cmd()
	if b, ok := c.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				if am, ok := item().(actionDoneMsg); ok {
					c = am
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
	if len(fake.Calls) != 2 || fake.Calls[0][1] != "unstage" || fake.Calls[1][1] != "reset" {
		t.Fatalf("Calls = %+v, want unstage then reset --purge across both paths", fake.Calls)
	}
}

func TestModel_EKeyOnFileDispatchesEditorCmd(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd (tea.ExecProcess) for 'e' on a file")
	}
}

func TestModel_EKeyOnDirectoryDoesNothing(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "src/a.go"}}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd != nil {
		t.Fatal("expected no Cmd for 'e' on a directory")
	}
}

func TestModel_VKeyEntersSelectModeAndDisablesMouse(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	m2 := updated.(Model)
	if !m2.selectMode {
		t.Fatal("expected selectMode = true after 'v'")
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd (tea.DisableMouse) after 'v'")
	}
}

func TestModel_AnyKeyExitsSelectModeAndRestoresMouse(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	m2 := updated.(Model)

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m3 := updated.(Model)
	if m3.selectMode {
		t.Fatal("expected selectMode = false after any key")
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd (tea.EnableMouseCellMotion) after exiting select mode")
	}
}

func TestModel_VKeyWhileFilteringGoesToFilterInputNotSelectMode(t *testing.T) {
	// Regression: 'v' used to be checked before the filter-typing bypass,
	// so it hijacked the keystroke instead of reaching the filter box -
	// making any filter query containing "v" untypable.
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "view.go"}}}})
	m2 := updated.(Model)

	var filterCmd tea.Cmd
	m2.files, filterCmd = m2.files.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_ = filterCmd
	if !m2.files.SettingFilter() {
		t.Fatal("expected files list to be in filter-typing state after \"/\"")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	m3 := updated.(Model)

	if m3.selectMode {
		t.Fatal("'v' while filtering must not enter select mode")
	}
	if got := m3.files.FilterInput.Value(); got != "v" {
		t.Fatalf("filter input value = %q, want %q", got, "v")
	}
}

func TestUpdateFocusedList_RoutesKeysToDiffViewport(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusDiff
	m.diff.vp.Width = 10
	m.diff.vp.Height = 2
	m.diff.SetContent("line1\nline2\nline3\nline4\nline5")

	if m.diff.vp.YOffset != 0 {
		t.Fatalf("precondition failed: YOffset = %d, want 0", m.diff.vp.YOffset)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := updated.(Model)
	if m2.diff.vp.YOffset == 0 {
		t.Fatalf("expected the down key to scroll the diff viewport, YOffset = %d", m2.diff.vp.YOffset)
	}
}

func TestHandleKey_FilterModeBypassesGlobalShortcuts(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset a.txt":   {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	// Enter filter-typing mode on the focused (Files) list, as "/" would.
	var filterCmd tea.Cmd
	m2.files, filterCmd = m2.files.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_ = filterCmd
	if !m2.files.SettingFilter() {
		t.Fatal("expected files list to be in filter-typing state after \"/\"")
	}

	// "d" is normally the global reset shortcut. While filtering, it must be
	// typed into the filter box instead of firing resetCmd.
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	if len(fake.Calls) != 0 {
		t.Fatalf("global shortcut fired while filtering: Calls = %+v, want none", fake.Calls)
	}
	if got := m3.files.FilterInput.Value(); got != "d" {
		t.Fatalf("filter input value = %q, want %q", got, "d")
	}
	if !m3.files.SettingFilter() {
		t.Fatal("expected files list to still be in filter-typing state")
	}
}

func TestModel_DKeyOnFileOpensDiscardConfirmPrompt(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset a.txt":   {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)
	if m3.prompt != promptConfirmDiscard {
		t.Fatalf("prompt = %v, want promptConfirmDiscard", m3.prompt)
	}
	if m3.pendingDiscardPath != "a.txt" {
		t.Fatalf("pendingDiscardPath = %q, want %q", m3.pendingDiscardPath, "a.txt")
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd yet (confirmation pending), got %v", cmd)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("expected no runner calls before confirmation, got %+v", fake.Calls)
	}
}

func TestModel_YKeyConfirmsDiscardAndUnstagesThenResets(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset a.txt":   {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after y = %v, want promptNone", m4.prompt)
	}
	if m4.pendingDiscardPath != "" {
		t.Fatalf("pendingDiscardPath after y = %q, want empty", m4.pendingDiscardPath)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after confirming discard")
	}
	c := cmd()
	if b, ok := c.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				res := item()
				if am, ok := res.(actionDoneMsg); ok {
					c = am
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
	if len(fake.Calls) != 2 || fake.Calls[0][1] != "unstage" || fake.Calls[1][1] != "reset" {
		t.Fatalf("Calls = %+v, want unstage then reset (via lore.DiscardChanges)", fake.Calls)
	}
}

func TestModel_EscCancelsDiscardPromptWithoutRunnerCalls(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset a.txt":   {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after esc = %v, want promptNone", m4.prompt)
	}
	if m4.pendingDiscardPath != "" {
		t.Fatalf("pendingDiscardPath after esc = %q, want empty", m4.pendingDiscardPath)
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd after cancelling, got %v", cmd)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("expected no runner calls after cancelling, got %+v", fake.Calls)
	}
}

func TestModel_EnterOnDirectoryTogglesCollapseInsteadOfLoadingDiff(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "src/a.go"}}}})
	m2 := updated.(Model)

	item, ok := m2.files.SelectedItem().(fileItem)
	if !ok || !item.isDir || item.path != "src" {
		t.Fatalf("precondition failed: selected item = %+v, want the 'src' directory", item)
	}
	if item.collapsed {
		t.Fatal("precondition failed: 'src' should start expanded")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(Model)

	if len(fake.Calls) != 0 {
		t.Fatalf("Enter on a directory should not dispatch any runner call, got %+v", fake.Calls)
	}
	item2, ok := m3.files.SelectedItem().(fileItem)
	if !ok || !item2.isDir || !item2.collapsed {
		t.Fatalf("expected 'src' to be collapsed after Enter, got %+v", item2)
	}
}

func TestModel_SpaceOnDirectoryTogglesCollapseInsteadOfStaging(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "src/a.go"}}}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	m3 := updated.(Model)

	if len(fake.Calls) != 0 {
		t.Fatalf("Space on a directory should not dispatch any runner call, got %+v", fake.Calls)
	}
	item, ok := m3.files.SelectedItem().(fileItem)
	if !ok || !item.isDir || !item.collapsed {
		t.Fatalf("expected 'src' to be collapsed after Space, got %+v", item)
	}
}

func TestModel_DKeyOnDirectoryDoesNothing(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "src/a.go"}}}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	if m3.prompt != promptNone {
		t.Fatalf("'d' on a directory should not open the discard prompt, got prompt = %v", m3.prompt)
	}
}

func TestModel_CollapsedDirectoryHidesItsFiles(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "src/a.go"},
		{Status: 'M', Path: "src/b.go"},
	}}})
	m2 := updated.(Model)

	if len(m2.files.Items()) != 3 { // src/, a.go, b.go
		t.Fatalf("expanded items = %+v, want 3 rows", m2.files.Items())
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(Model)

	if len(m3.files.Items()) != 1 {
		t.Fatalf("collapsed items = %+v, want just the 'src' row", m3.files.Items())
	}
}
