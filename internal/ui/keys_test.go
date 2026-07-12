package ui

import (
	"errors"
	"strings"
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

func TestModel_OneKeyJumpsToStatus(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	m2 := updated.(Model)
	if m2.focus != focusStatus {
		t.Fatalf("focus = %v, want focusStatus", m2.focus)
	}
}

func TestModel_TwoKeyJumpsToFiles(t *testing.T) {
	// Regression: "1" and "2" used to both jump to Files (Status had no
	// focusable state of its own) - lazygit's real numbering (Gui.SidePanels)
	// is 1=Status, 2=Files, distinct panels.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusStatus
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m2 := updated.(Model)
	if m2.focus != focusFiles {
		t.Fatalf("focus = %v, want focusFiles", m2.focus)
	}
}

func TestModel_TabCyclesThroughAllFivePanelsIncludingStatus(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusStatus // NewModel defaults to focusFiles - start from a known point
	order := []focusPanel{focusStatus, focusFiles, focusBranches, focusHistory, focusDiff, focusStatus}
	for i := 1; i < len(order); i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
		m = updated.(Model)
		if m.focus != order[i] {
			t.Fatalf("after %d tab(s): focus = %v, want %v", i, m.focus, order[i])
		}
	}
}

func TestModel_ShiftTabCyclesBackwardThroughStatus(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusStatus
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m2 := updated.(Model)
	if m2.focus != focusDiff {
		t.Fatalf("focus = %v, want focusDiff (wrapped backward from Status)", m2.focus)
	}
}

func TestModel_BracketKeysToggleRemoteTabOnlyWhenBranchesFocused(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusBranches
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	m2 := updated.(Model)
	if !m2.showRemoteBranches {
		t.Fatal("expected showRemoteBranches = true after ']' on Branches")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	m3 := updated.(Model)
	if m3.showRemoteBranches {
		t.Fatal("expected showRemoteBranches = false after '[' on Branches")
	}
}

func TestModel_BracketKeysNoOpOutsideBranches(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusFiles
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	m2 := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected a nil Cmd for ']' outside Branches, got %v", cmd)
	}
	if m2.showRemoteBranches {
		t.Fatal("expected showRemoteBranches to stay false when Files is focused")
	}
}

func TestModel_MouseClickOnStatusAreaFocusesStatus(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.width, m.height = 100, 40
	(&m).resize()
	m.focus = focusFiles

	updated, _ := m.Update(tea.MouseMsg{X: 5, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m2 := updated.(Model)
	if m2.focus != focusStatus {
		t.Fatalf("focus = %v, want focusStatus after clicking the Status area", m2.focus)
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
	if want := `Revert "oops"`; m3.pendingRevertMessage != want {
		t.Fatalf("pendingRevertMessage = %q, want %q", m3.pendingRevertMessage, want)
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd yet (confirmation pending), got %v", cmd)
	}
}

func TestModel_YKeyConfirmsRevert(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		`--json revision revert abcdef12 --message Revert "oops"`: {ExitCode: 0, Stdout: jsonCompleteSuccess},
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
	if fake.Calls[0][4] != "--message" || fake.Calls[0][5] != `Revert "oops"` {
		t.Fatalf("Calls[0] = %+v, want a trailing --message %q", fake.Calls[0], `Revert "oops"`)
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

func TestModel_LKeyOnOwnLockedFileReleasesLockImmediately(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock release a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.currentUserID = "me"
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(locksMsg{locks: []lore.Lock{{Path: "a.txt", Owner: "me"}}})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	m4 := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for L on your own locked file")
	}
	if m4.prompt != promptNone {
		t.Fatalf("prompt = %v, want promptNone - unlocking your own lock needs no confirm", m4.prompt)
	}
	runBatch(cmd)
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "lock" || fake.Calls[0][2] != "release" {
		t.Fatalf("Calls = %+v, want a single lock release call", fake.Calls)
	}
}

func TestModel_LKeyOnOtherOwnersLockedFileOpensForceUnlockConfirm(t *testing.T) {
	// Matches lazygit's own confirm-before-destructive-action pattern
	// (discard/reset/revert) - unlocking someone else's lock shouldn't fire
	// on a bare keypress, since it either no-ops against stock lore or
	// actually releases another person's lock against a lore fork with the
	// AdminUnlock capability.
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	m.currentUserID = "me"
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(locksMsg{locks: []lore.Lock{{Path: "a.txt", Owner: "someone"}}})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	m4 := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no Cmd yet (confirmation pending), got %v", cmd)
	}
	if m4.prompt != promptConfirmForceUnlock || m4.pendingForceUnlockPath != "a.txt" {
		t.Fatalf("prompt = %v, pendingForceUnlockPath = %q, want promptConfirmForceUnlock for a.txt", m4.prompt, m4.pendingForceUnlockPath)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("expected no runner calls before confirming, got %+v", fake.Calls)
	}
}

func TestModel_ConfirmingForceUnlockCallsLockReleaseWithForce(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock release --force a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.currentUserID = "me"
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(locksMsg{locks: []lore.Lock{{Path: "a.txt", Owner: "someone"}}})
	m3 := updated.(Model)
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	m4 := updated.(Model)

	updated, cmd := m4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m5 := updated.(Model)
	if m5.prompt != promptNone {
		t.Fatalf("prompt after confirming = %v, want promptNone", m5.prompt)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after confirming force-unlock")
	}
	runBatch(cmd)
	if len(fake.Calls) != 1 || fake.Calls[0][1] != "lock" || fake.Calls[0][2] != "release" || fake.Calls[0][3] != "--force" {
		t.Fatalf("Calls = %+v, want a single lock release --force call", fake.Calls)
	}
}

func TestModel_CancellingForceUnlockMakesNoRunnerCalls(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	m.currentUserID = "me"
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(locksMsg{locks: []lore.Lock{{Path: "a.txt", Owner: "someone"}}})
	m3 := updated.(Model)
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	m4 := updated.(Model)

	updated, cmd := m4.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m5 := updated.(Model)
	if m5.prompt != promptNone || m5.pendingForceUnlockPath != "" {
		t.Fatalf("after esc: prompt = %v, pendingForceUnlockPath = %q, want cleared", m5.prompt, m5.pendingForceUnlockPath)
	}
	if cmd != nil {
		t.Fatalf("expected no Cmd after cancelling, got %v", cmd)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("expected no runner calls after cancelling, got %+v", fake.Calls)
	}
}

func TestModel_LockConfirmUpdatesLocksMapBeforeRefreshRebuildsItems(t *testing.T) {
	// Regression: after a successful lock acquire, refreshCmd's statusMsg
	// rebuilds Files from m.locks - but its own loadLocksCmd (dispatched in
	// that same handler) hasn't resolved yet, so without actionDoneMsg's
	// confirm updating m.locks immediately, that rebuild used to run
	// against the still-stale (pre-lock) m.locks and show the badge
	// disappearing for a frame before the real loadLocksCmd caught up -
	// the reported "[L] blinks" bug.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(currentUserMsg{id: "user-123"})
	m2 := updated.(Model)
	updated, _ = m2.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m3 := updated.(Model)

	// Simulate lockToggleCmd's own success message (see commands.go) rather
	// than running it through the real Runner - only the confirm wiring is
	// under test here.
	updated, _ = m3.Update(actionDoneMsg{
		label: "Lock file",
		opKey: "lock:a.txt",
		confirm: func(mm *Model) {
			if mm.locks == nil {
				mm.locks = map[string]lore.Lock{}
			}
			mm.locks["a.txt"] = lore.Lock{Path: "a.txt", Owner: mm.currentUserID}
		},
	})
	m4 := updated.(Model)

	if _, locked := m4.locks["a.txt"]; !locked {
		t.Fatal("expected confirm to have populated m.locks immediately, before any refresh")
	}

	// The statusMsg refreshCmd triggers next - it must rebuild Files using
	// the now-current m.locks, not a stale copy.
	updated, _ = m4.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m5 := updated.(Model)

	item, ok := m5.files.SelectedItem().(fileItem)
	if !ok || !item.locked || !item.lockedByMe {
		t.Fatalf("expected a.txt to stay locked (and lockedByMe) through the refresh rebuild, got %+v", item)
	}
}

func TestModel_SpaceOnFileOptimisticallyFlipsStagedBeforeCommandResolves(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	m3 := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for space on a file")
	}
	item, ok := m3.files.SelectedItem().(fileItem)
	if !ok || !item.staged {
		t.Fatalf("expected the file to show as staged immediately, before stageCmd resolves, got %+v", item)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("Calls = %+v, want none yet - the real command only runs once its Cmd is invoked", fake.Calls)
	}
}

func TestModel_SecondSpacePressOnSamePathWhilePendingIsNoOp(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeySpace})
	m4 := updated.(Model)
	if cmd != nil {
		t.Fatal("expected a nil Cmd for a second space press while the first stage is still pending")
	}
	item, ok := m4.files.SelectedItem().(fileItem)
	if !ok || !item.staged {
		t.Fatalf("expected the file to stay staged (no flip-back) while pending, got %+v", item)
	}
}

func TestModel_FailedStageRevertsOptimisticFlipAndClearsPending(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	m3 := updated.(Model)

	// Simulate the background stageCmd failing - feed Update the same shape
	// of actionDoneMsg it would have produced, without going through the
	// returned Cmd (see TestStageCmd_RevertFlipsFileBackToUnstaged in
	// commands_test.go for the Cmd's own opKey/revert wiring).
	updated, _ = m3.Update(actionDoneMsg{
		label: "stage a.txt",
		err:   errors.New("boom"),
		opKey: "stage:a.txt",
		revert: func(mm *Model) {
			mm.setFileStagedByPath("a.txt", true, false)
		},
	})
	m4 := updated.(Model)

	item, ok := m4.files.SelectedItem().(fileItem)
	if !ok || item.staged {
		t.Fatalf("expected the optimistic flip to be reverted after failure, got %+v", item)
	}
	if m4.pendingFileOps["stage:a.txt"] {
		t.Fatal("expected pendingFileOps to be cleared once the action resolved")
	}
	if _, cmd := m4.Update(tea.KeyMsg{Type: tea.KeySpace}); cmd == nil {
		t.Fatal("expected space to work again now that the pending flag is cleared")
	}
}

func TestModel_ActionFailureOnlyLogsToCommandLogNotFooter(t *testing.T) {
	// Regression: actionDoneMsg used to both append to the Command Log AND
	// set m.err (rendered in currentFooter, near the keybind bar) - the
	// same failure shown twice. m.err must stay untouched here; the
	// Command Log is the only place this error should surface.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	updated, _ = m2.Update(actionDoneMsg{
		label: "stage a.txt",
		err:   errors.New("boom"),
		opKey: "stage:a.txt",
	})
	m3 := updated.(Model)

	if m3.err != nil {
		t.Fatalf("m.err = %v, want nil - the error should only appear in the Command Log", m3.err)
	}
	if len(m3.log.entries) == 0 || m3.log.entries[len(m3.log.entries)-1].err == nil {
		t.Fatal("expected the failure to have been appended to the Command Log")
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

func TestModel_QuestionMarkKeyOpensHelp(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := updated.(Model)
	if !m2.showHelp {
		t.Fatal("expected showHelp = true after '?'")
	}
}

func TestModel_UnrecognizedKeyWhileHelpOpenScrollsInsteadOfClosing(t *testing.T) {
	// Only esc/'?' close the popup now - everything else (including keys
	// the viewport doesn't recognize) must leave it open so j/k/arrows can
	// scroll a keybindings list too long to fit on screen.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m3 := updated.(Model)
	if !m3.showHelp {
		t.Fatal("expected showHelp to stay true after an unrecognized key")
	}
}

func TestModel_EscClosesHelp(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated.(Model)
	if m3.showHelp {
		t.Fatal("expected showHelp = false after esc")
	}
}

func TestModel_QuestionMarkAgainClosesHelp(t *testing.T) {
	// '?' both opens (via handleKey, only reached when showHelp is already
	// false) and closes (caught explicitly in Update's showHelp branch) -
	// a plain toggle.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m3 := updated.(Model)
	if m3.showHelp {
		t.Fatal("expected showHelp = false after a second '?'")
	}
}

func TestModel_JKeyWhileHelpOpenScrollsViewport(t *testing.T) {
	// A small terminal height caps the popup below the content's real line
	// count (see openHelp), guaranteeing there's something to scroll.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.width, m.height = 100, 10
	(&m).resize()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := updated.(Model)
	startOffset := m2.helpViewport.YOffset

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m3 := updated.(Model)
	if !m3.showHelp {
		t.Fatal("expected showHelp to stay true after 'j'")
	}
	if m3.helpViewport.YOffset <= startOffset {
		t.Fatalf("YOffset = %d, want it to have scrolled down from %d", m3.helpViewport.YOffset, startOffset)
	}
}

func TestModel_MouseClickIgnoredWhileHelpOpen(t *testing.T) {
	// A popup covers the whole screen - clicks must not reach panels
	// underneath it (e.g. silently changing focus or list selection). Goes
	// through Update (not handleMouseClick directly) since that's what
	// actually routes MouseMsg to the help viewport instead - see Update's
	// tea.MouseMsg case.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.width, m.height = 100, 40
	(&m).resize()
	m.showHelp = true

	updated, _ := m.Update(tea.MouseMsg{X: 5, Y: 5, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m2 := updated.(Model)
	if m2.focus != m.focus {
		t.Fatalf("focus changed to %v from a click while help was open, want unchanged %v", m2.focus, m.focus)
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

func TestModel_AKeyStagesEverythingRegardlessOfSelection(t *testing.T) {
	// Matches lazygit's "a" (toggleStagedAll): same stage-if-anything's-
	// unstaged-else-unstage rule as space on a directory, just always
	// applied to the whole tree via toggleDirStage("") - "." is lore's
	// repo-root path since there's no real path for the synthetic root row.
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage .": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "a.txt"},
		{Status: 'M', Path: "b.txt"},
	}}})
	m2 := updated.(Model)

	// Select something other than root, to prove "a" doesn't depend on
	// the current selection the way space does.
	m2.files.Select(1)

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m3 := updated.(Model)
	runBatch(cmd)

	if len(fake.Calls) != 1 || strings.Join(fake.Calls[0], " ") != "--json stage ." {
		t.Fatalf("'a' should dispatch a single 'stage .' call, got %+v", fake.Calls)
	}
	for _, it := range m3.files.Items() {
		fi, ok := it.(fileItem)
		if ok && !fi.isDir && !fi.staged {
			t.Fatalf("expected every file staged optimistically after 'a', got %+v", fi)
		}
	}
}

func TestModel_SpaceOnDirectoryStagesEverythingUnderItRecursively(t *testing.T) {
	// Matches lazygit's own Files-panel space key (files_controller.go's
	// press/toggleStaged): space on a directory stages every unstaged file
	// under it in one lore call, rather than colliding with Enter's collapse
	// toggle (see TestModel_EnterOnDirectoryTogglesCollapseInsteadOfLoadingDiff).
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage src": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "src/a.go"}}}})
	m2 := updated.(Model)

	item, ok := m2.files.SelectedItem().(fileItem)
	if !ok || !item.isDir || item.path != "src" {
		t.Fatalf("precondition failed: selected item = %+v, want the 'src' directory", item)
	}

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	m3 := updated.(Model)
	runBatch(cmd)

	if len(fake.Calls) != 1 || strings.Join(fake.Calls[0], " ") != "--json stage src" {
		t.Fatalf("Space on 'src' should dispatch a single 'stage src' call, got %+v", fake.Calls)
	}
	item2, ok := m3.files.SelectedItem().(fileItem)
	if !ok || !item2.isDir || item2.collapsed {
		t.Fatalf("Space on a directory should not collapse it (that's Enter's job), got %+v", item2)
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
