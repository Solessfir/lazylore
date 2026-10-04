package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazylore/internal/lore"
)

// runBatch executes cmd and, if it's a tea.Batch, every sub-cmd too - real
// ordering/concurrency doesn't matter here, only that every runner call in
// the batch actually happens.
func runBatch(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if b, ok := commandResult(cmd).(tea.BatchMsg); ok {
		for _, sub := range b {
			runBatch(sub)
		}
	}
}

func TestModel_JumpToBranchesLoadsLogForSelectedBranch(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json history 50 --branch=main": {ExitCode: 0, Stdout: `{"tagName":"revisionHistoryEntry","data":{"revision":"abc123","revisionNumber":1,"parent":["0000000000000000000000000000000000000000000000000000000000000000","0000000000000000000000000000000000000000000000000000000000000000"]}}
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
	msg := commandResult(cmd)
	dm, ok := msg.(diffMsg)
	if !ok || !dm.raw {
		t.Fatalf("msg = %#v, want a raw diffMsg (Log content)", msg)
	}
	if len(fake.Calls) != 1 || len(fake.Calls[0]) != 4 || fake.Calls[0][3] != "--branch=main" {
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
	// "1" jumps to Status, "2" to Files - distinct panels, not both Files.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusStatus
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m2 := updated.(Model)
	if m2.focus != focusFiles {
		t.Fatalf("focus = %v, want focusFiles", m2.focus)
	}
}

func TestModel_TabCyclesThroughAllSixPanelsIncludingStatus(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.focus = focusStatus // NewModel defaults to focusFiles - start from a known point
	order := []focusPanel{focusStatus, focusFiles, focusBranches, focusHistory, focusDiff, focusCommandLog, focusStatus}
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
	if m2.focus != focusCommandLog {
		t.Fatalf("focus = %v, want focusCommandLog (wrapped backward from Status)", m2.focus)
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

func TestModel_SixKeyFocusesCommandLogAndExpandsItOverDiff(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.width, m.height = 100, 40
	(&m).resize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("6")})
	m2 := updated.(Model)
	if m2.focus != focusCommandLog {
		t.Fatalf("focus = %v, want focusCommandLog", m2.focus)
	}

	out := m2.View()
	if strings.Contains(out, "╭─[5]─") {
		t.Fatalf("Diff panel border ([5]) should not appear while Command Log is expanded, got:\n%s", out)
	}
	if !strings.Contains(out, "Command Log") {
		t.Fatal("expected the Command Log title to still be present")
	}
}

func TestModel_MouseWheelPansFilesViewWithoutMovingSelectionOrChangingFocus(t *testing.T) {
	// The wheel moves what's visible, not the cursor - unlike keyboard/click
	// navigation, which snaps the view back to following the selection.
	var files []lore.FileChange
	for i := 0; i < 30; i++ {
		files = append(files, lore.FileChange{Status: 'M', Path: fmt.Sprintf("file%02d.txt", i)})
	}
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.width, m.height = 100, 40
	(&m).resize()
	m.focus = focusBranches // hovering Files while Branches is focused
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: files}})
	m2 := updated.(Model)
	startIdx := m2.files.Index()
	if m2.filesScrollOverride != -1 {
		t.Fatalf("precondition failed: filesScrollOverride = %d, want -1 (no override yet)", m2.filesScrollOverride)
	}

	// Y=5 lands inside the Files panel content area (Status panel is 3 rows
	// tall plus its border, Files starts right after).
	updated, _ = m2.Update(tea.MouseMsg{X: 5, Y: 5, Button: tea.MouseButtonWheelDown})
	m3 := updated.(Model)

	if m3.focus != focusBranches {
		t.Fatalf("focus = %v, want focusBranches unchanged - hovering to scroll shouldn't steal focus", m3.focus)
	}
	if m3.files.Index() != startIdx {
		t.Fatalf("files.Index() = %d, want unchanged %d - the wheel must not move the selection", m3.files.Index(), startIdx)
	}
	if m3.filesScrollOverride != 1 {
		t.Fatalf("filesScrollOverride = %d, want 1 (panned down by one row)", m3.filesScrollOverride)
	}
}

func TestModel_MouseWheelNoOpWhenListHasNoRoomToScroll(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.width, m.height = 100, 40
	(&m).resize()
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	startIdx := m2.files.Index()

	updated, _ = m2.Update(tea.MouseMsg{X: 5, Y: 5, Button: tea.MouseButtonWheelDown})
	m3 := updated.(Model)

	if m3.files.Index() != startIdx {
		t.Fatalf("files.Index() = %d, want unchanged %d - only one item, nothing to scroll", m3.files.Index(), startIdx)
	}
}

func TestModel_MouseWheelIgnoredWhilePromptOpen(t *testing.T) {
	var files []lore.FileChange
	for i := 0; i < 30; i++ {
		files = append(files, lore.FileChange{Status: 'M', Path: fmt.Sprintf("file%02d.txt", i)})
	}
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.width, m.height = 100, 40
	(&m).resize()
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: files}})
	m2 := updated.(Model)
	m2.prompt = promptConfirmDiscardAll
	startIdx := m2.files.Index()

	updated, _ = m2.Update(tea.MouseMsg{X: 5, Y: 5, Button: tea.MouseButtonWheelDown})
	m3 := updated.(Model)

	if m3.files.Index() != startIdx {
		t.Fatalf("files.Index() = %d, want unchanged %d - a popup is covering the screen", m3.files.Index(), startIdx)
	}
}

func TestModel_MouseDoesNotSelectOrPanListsWhileEditingFilter(t *testing.T) {
	for _, source := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		m = updated.(Model)
		var changes []lore.FileChange
		var branches []lore.Branch
		var revisions []lore.Revision
		for i := 0; i < 30; i++ {
			changes = append(changes, lore.FileChange{Status: 'M', Path: fmt.Sprintf("file%02d.txt", i)})
			branches = append(branches, lore.Branch{Name: fmt.Sprintf("branch%02d", i)})
			revisions = append(revisions, lore.Revision{Hash: fmt.Sprintf("revision%02d", i)})
		}
		updated, _ = m.Update(statusMsg{status: lore.Status{Unstaged: changes}})
		m = updated.(Model)
		updated, _ = m.Update(branchesMsg{branches: branches})
		m = updated.(Model)
		updated, _ = m.Update(historyMsg{revisions: revisions})
		m = updated.(Model)
		m.focus = source
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
		m = updated.(Model)
		if !m.panelList(source).SettingFilter() {
			t.Fatalf("panel %v did not enter filter editing", source)
		}
		index := m.panelList(source).Index()
		l := m.computeMouseLayout()
		top := map[focusPanel]int{focusFiles: l.filesBoxTop, focusBranches: l.branchesBoxTop, focusHistory: l.historyBoxTop}[source]
		for _, msg := range []tea.MouseMsg{
			{X: 10, Y: top + 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
			{X: 10, Y: top + 4, Button: tea.MouseButtonWheelDown},
		} {
			t.Run(fmt.Sprintf("%v/%v", source, msg.Button), func(t *testing.T) {
				updated, _ := m.Update(msg)
				result := updated.(Model)
				override := map[focusPanel]int{focusFiles: result.filesScrollOverride, focusBranches: result.branchesScrollOverride, focusHistory: result.historyScrollOverride}[source]
				if result.panelList(source).Index() != index || override != -1 {
					t.Fatalf("mouse %v changed selection or scroll during filter editing: index=%d override=%d", msg, result.panelList(source).Index(), override)
				}
			})
		}
	}
}

func TestModel_MouseCannotInteractWithHiddenSmallTerminalPanels(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 12})
	m = updated.(Model)
	var changes []lore.FileChange
	for i := 0; i < 30; i++ {
		changes = append(changes, lore.FileChange{Status: 'M', Path: fmt.Sprintf("file%02d.txt", i)})
	}
	updated, _ = m.Update(statusMsg{status: lore.Status{Unstaged: changes}})
	m = updated.(Model)
	if !strings.Contains(m.View(), "Terminal too small") {
		t.Fatal("expected the fit warning to replace the panels")
	}
	for _, msg := range []tea.MouseMsg{
		{X: 5, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		{X: 5, Y: 5, Button: tea.MouseButtonWheelDown},
	} {
		updated, cmd := m.Update(msg)
		m = updated.(Model)
		if cmd != nil || m.focus != focusFiles || m.files.Index() != 0 || m.filesScrollOverride != -1 {
			t.Fatalf("mouse affected hidden panels: focus=%v index=%d scroll=%d cmd=%v", m.focus, m.files.Index(), m.filesScrollOverride, cmd)
		}
	}
}

func TestModel_SmallTerminalPreservesPromptAndBlocksConfirmations(t *testing.T) {
	for _, prompt := range []promptKind{promptCommit, promptDiscardMenu, promptConfirmForceUnlock} {
		m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		m = updated.(Model)
		m.openCommitPrompt()
		m.prompt = prompt
		m.input.SetValue("preserve this summary")
		m.pendingDiscardPath = "file.txt"
		m.pendingDiscardPaths = []string{"file.txt"}
		m.pendingForceUnlockPath = "file.txt"
		updated, _ = m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
		m = updated.(Model)
		if view := m.View(); !strings.Contains(view, "q / ctrl+c - quit") {
			t.Fatalf("fallback lost quit guidance: %q", view)
		}
		for _, key := range []tea.KeyMsg{
			{Type: tea.KeyEnter},
			{Type: tea.KeyRunes, Runes: []rune("y")},
			{Type: tea.KeyRunes, Runes: []rune("x")},
		} {
			updated, cmd := m.Update(key)
			m = updated.(Model)
			if cmd != nil || m.prompt != prompt || m.input.Value() != "preserve this summary" {
				t.Fatalf("hidden prompt %v handled %q: prompt=%v text=%q cmd=%v", prompt, key, m.prompt, m.input.Value(), cmd)
			}
		}
		updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		m = updated.(Model)
		if m.prompt != prompt || m.input.Value() != "preserve this summary" || strings.Contains(m.View(), "Terminal too small") {
			t.Fatalf("restoring terminal lost prompt %v or input", prompt)
		}
	}
}

func TestModel_SmallTerminalAllowsQuit(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyCtrlC},
	} {
		m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
		m = updated.(Model)
		_, cmd := m.Update(key)
		if cmd == nil {
			t.Fatalf("fallback did not handle quit key %q", key)
		}
		if _, ok := commandResult(cmd).(tea.QuitMsg); !ok {
			t.Fatalf("fallback key %q did not quit", key)
		}
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
	msg := commandResult(cmd)
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
		`--json revision revert abcdef12 --message=Revert "oops"`: {ExitCode: 0, Stdout: jsonCompleteSuccess},
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
	if len(fake.Calls[0]) != 5 || fake.Calls[0][4] != `--message=Revert "oops"` {
		t.Fatalf("Calls[0] = %+v, want a trailing %q", fake.Calls[0], `--message=Revert "oops"`)
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
		"--json branch switch -- dev": {ExitCode: 0, Stdout: jsonCompleteSuccess},
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
	msg := commandResult(cmd)
	if b, ok := msg.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				if am, ok := commandResult(item).(actionDoneMsg); ok {
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
	if len(fake.Calls) != 1 || len(fake.Calls[0]) != 5 || fake.Calls[0][1] != "branch" || fake.Calls[0][2] != "switch" || fake.Calls[0][3] != "--" || fake.Calls[0][4] != "dev" {
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
		"--json lock acquire -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
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
		"--json lock release -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.currentUserID = "me"
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(locksMsg{requestID: m2.lockRequestID, locks: []lore.Lock{{Path: "a.txt", Owner: "me"}}})
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
	// Unlocking someone else's lock shouldn't fire on a bare keypress - it
	// either no-ops against stock lore, or actually releases another
	// person's lock against a lore fork with the AdminUnlock capability.
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	m.currentUserID = "me"
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(locksMsg{requestID: m2.lockRequestID, locks: []lore.Lock{{Path: "a.txt", Owner: "someone"}}})
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
		"--json lock release --force -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	m.currentUserID = "me"
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(locksMsg{requestID: m2.lockRequestID, locks: []lore.Lock{{Path: "a.txt", Owner: "someone"}}})
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
	updated, _ = m2.Update(locksMsg{requestID: m2.lockRequestID, locks: []lore.Lock{{Path: "a.txt", Owner: "someone"}}})
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
	// actionDoneMsg's confirm must update m.locks immediately - refreshCmd's
	// statusMsg rebuilds Files from m.locks before loadLocksCmd resolves.
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
	updated, _ = m4.Update(statusMsg{generation: m4.refreshGeneration, status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m5 := updated.(Model)

	item, ok := m5.files.SelectedItem().(fileItem)
	if !ok || !item.locked || !item.lockedByMe {
		t.Fatalf("expected a.txt to stay locked (and lockedByMe) through the refresh rebuild, got %+v", item)
	}
}

func TestModel_SpaceOnFileOptimisticallyFlipsStagedBeforeCommandResolves(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
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
		"--json stage -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
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

func TestModel_YKeyConfirmsDiscardAllWithPathsKnownWhenPromptOpened(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- a.txt b.txt":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge -- a.txt b.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{
		Staged:   []lore.FileChange{{Status: 'A', Path: "a.txt"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "b.txt"}},
	}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m3 := updated.(Model)
	updated, _ = m3.Update(statusMsg{status: lore.Status{
		Staged: []lore.FileChange{{Status: 'A', Path: "a.txt"}},
		Unstaged: []lore.FileChange{
			{Status: 'M', Path: "b.txt"},
			{Status: 'A', Path: "after-confirmation.txt"},
			{Status: 'M', Path: "nested/child.txt"},
		},
	}})
	m3 = updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after y = %v, want promptNone", m4.prompt)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after confirming discard-all")
	}
	c := commandResult(cmd)
	if b, ok := c.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				if am, ok := commandResult(item).(actionDoneMsg); ok {
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
	if len(fake.Calls) != 2 || strings.Join(fake.Calls[0], " ") != "--json unstage -- a.txt b.txt" || strings.Join(fake.Calls[1], " ") != "--json reset --purge -- a.txt b.txt" {
		t.Fatalf("Calls = %+v, want unstage then reset --purge only for the frozen paths", fake.Calls)
	}
	if len(m4.pendingDiscardPaths) != 0 {
		t.Fatalf("confirmed prompt retained paths: %v", m4.pendingDiscardPaths)
	}
}

func TestModel_DiscardAllEmptySnapshotDoesNothing(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	updated, _ = m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'A', Path: "late.txt"}}}})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if cmd != nil || m.prompt != promptNone || len(fake.Calls) != 0 {
		t.Fatalf("empty snapshot produced an action: cmd=%v prompt=%v calls=%v", cmd, m.prompt, fake.Calls)
	}
}

func TestModel_CancellingDiscardAllClearsSnapshot(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.status = lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if cmd != nil || m.prompt != promptNone || len(m.pendingDiscardPaths) != 0 {
		t.Fatalf("cancel retained an action: cmd=%v prompt=%v paths=%v", cmd, m.prompt, m.pendingDiscardPaths)
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

func TestModel_TypingWhileHelpOpenStartsSearch(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m3 := updated.(Model)
	if !m3.showHelp {
		t.Fatal("expected showHelp to stay true after typing")
	}
	if !m3.helpInput.Focused() || m3.helpInput.Value() != "x" {
		t.Fatal("typing did not start help search")
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

func TestModel_JKeyWhileHelpOpenMovesSelection(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.width, m.height = 100, 20
	(&m).resize()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m2 := updated.(Model)
	startCursor := m2.helpCursor

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m3 := updated.(Model)
	if !m3.showHelp {
		t.Fatal("expected showHelp to stay true after 'j'")
	}
	if m3.helpCursor <= startCursor {
		t.Fatalf("helpCursor = %d, want it to have advanced from %d", m3.helpCursor, startCursor)
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
		"--json unstage -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset -- a.txt":   {ExitCode: 0, Stdout: jsonCompleteSuccess},
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

func TestModel_DKeyOnFileOpensDiscardMenuPrompt(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- a.txt":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)
	if m3.prompt != promptDiscardMenu {
		t.Fatalf("prompt = %v, want promptDiscardMenu", m3.prompt)
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

func TestModel_XKeyConfirmsDiscardAllAndUnstagesThenResetsPurge(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- a.txt":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after x = %v, want promptNone", m4.prompt)
	}
	if m4.pendingDiscardPath != "" {
		t.Fatalf("pendingDiscardPath after x = %q, want empty", m4.pendingDiscardPath)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after confirming discard")
	}
	c := commandResult(cmd)
	if b, ok := c.(tea.BatchMsg); ok && len(b) > 0 {
		for _, item := range b {
			if item != nil {
				res := commandResult(item)
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
		t.Fatalf("Calls = %+v, want unstage then reset --purge (via lore.DiscardAllChanges)", fake.Calls)
	}
}

func TestModel_EscCancelsDiscardPromptWithoutRunnerCalls(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- a.txt":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge -- a.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
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
	if len(m4.pendingDiscardPaths) != 0 || len(m4.pendingDiscardUnstaged) != 0 {
		t.Fatal("cancelled menu retained its snapshot")
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
	// "." is lore's repo-root path, used since there's no real path for
	// the synthetic root row.
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- .": {ExitCode: 0, Stdout: jsonCompleteSuccess},
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

	if len(fake.Calls) != 1 || strings.Join(fake.Calls[0], " ") != "--json stage -- ." {
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
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- src": {ExitCode: 0, Stdout: jsonCompleteSuccess},
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

	if len(fake.Calls) != 1 || strings.Join(fake.Calls[0], " ") != "--json stage -- src" {
		t.Fatalf("Space on 'src' should dispatch a single 'stage src' call, got %+v", fake.Calls)
	}
	item2, ok := m3.files.SelectedItem().(fileItem)
	if !ok || !item2.isDir || item2.collapsed {
		t.Fatalf("Space on a directory should not collapse it (that's Enter's job), got %+v", item2)
	}
}

func TestModel_DKeyOnDirectoryOpensDiscardMenuWithUnstagedDisabledWhenNotMixed(t *testing.T) {
	// "u" stays disabled here since every file under "src" is unstaged -
	// nothing staged to preserve, so "discard unstaged" and "discard all"
	// would do the exact same thing.
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "src/a.go"}}}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)

	if m3.prompt != promptDiscardMenu {
		t.Fatalf("'d' on a directory should open the discard menu, got prompt = %v", m3.prompt)
	}
	if !m3.pendingDiscardIsDir {
		t.Fatal("expected pendingDiscardIsDir = true for a directory row")
	}
	if m3.pendingDiscardPath != "src" {
		t.Fatalf("pendingDiscardPath = %q, want %q", m3.pendingDiscardPath, "src")
	}

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m4 := updated.(Model)
	if m4.prompt != promptDiscardMenu {
		t.Fatal("'u' on a directory row should be a no-op (disabled), not dismiss the menu")
	}
	if cmd != nil {
		t.Fatal("'u' on a directory row should not dispatch a Cmd (disabled)")
	}
}

func TestModel_UKeyOnMixedDirectoryDiscardsOnlyUnstagedFiles(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- src/b.go":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge -- src/b.go": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{
		Staged:   []lore.FileChange{{Status: 'M', Path: "src/a.go"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "src/b.go"}},
	}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3 := updated.(Model)
	if !m3.pendingDiscardDirMixed {
		t.Fatal("expected pendingDiscardDirMixed = true for a directory with both staged and unstaged files")
	}

	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m4 := updated.(Model)
	if m4.prompt != promptNone {
		t.Fatalf("prompt after u = %v, want promptNone", m4.prompt)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd after 'u' on a mixed directory")
	}
	runBatch(cmd)
	if len(fake.Calls) != 1 || strings.Join(fake.Calls[0], " ") != "--json reset --purge -- src/b.go" {
		t.Fatalf("Calls = %+v, want reset --purge for src/b.go only (src/a.go stays staged)", fake.Calls)
	}
}

func TestModel_BranchTabSwitchUpdatesSelectedContent(t *testing.T) {
	for _, key := range []string{"[", "]"} {
		t.Run(key, func(t *testing.T) {
			m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
			m.focus = focusBranches
			m.localBranches = []lore.Branch{{Name: "local"}}
			m.remoteBranches = []lore.Branch{{Name: "remote", Remote: true}}
			m.refreshBranchesList()
			m.ensureMainContent()
			for _, want := range []string{"remote", "local"} {
				previous := m.mainContentRequestID
				updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
				m = updated.(Model)
				selected := m.branches.SelectedItem().(branchItem).branch.Name
				if selected != want || m.currentLogBranch != want || m.mainContentRequestID == previous || cmd == nil {
					t.Fatalf("selected = %q, log = %q, request = %d, cmd nil = %v; want %q with a new request", selected, m.currentLogBranch, m.mainContentRequestID, cmd == nil, want)
				}
			}
			m.remoteBranches = nil
			previous := m.mainContentRequestID
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			m = updated.(Model)
			if m.currentLogBranch != "" || m.mainContentRequestID == previous {
				t.Fatal("empty remote tab retained the local branch log")
			}
		})
	}
}

func TestModel_MouseBranchTabsUseVisibleLabelBounds(t *testing.T) {
	for _, width := range []int{80, 120, 155} {
		m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
		m = updated.(Model)
		m.localBranches = []lore.Branch{{Name: "local"}}
		m.remoteBranches = []lore.Branch{{Name: "remote", Remote: true}}
		m.refreshBranchesList()
		l := m.computeMouseLayout()
		line := ansiStrip(strings.Split(m.View(), "\n")[l.branchesBoxTop])
		for _, tc := range []struct {
			label  string
			remote bool
		}{
			{label: "Re", remote: true},
			{label: "branches", remote: false},
		} {
			start := lipgloss.Width(line[:strings.Index(line, tc.label)])
			for x := start; x < start+lipgloss.Width(tc.label); x++ {
				updated, _ = m.Update(tea.MouseMsg{X: x, Y: l.branchesBoxTop, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
				m = updated.(Model)
				if m.showRemoteBranches != tc.remote || m.branches.SelectedItem().(branchItem).branch.Name != map[bool]string{false: "local", true: "remote"}[tc.remote] {
					t.Fatalf("width %d click at %d on %q chose wrong tab: remote=%v", width, x, tc.label, m.showRemoteBranches)
				}
			}
		}
		for _, x := range []int{0, l.leftW - 1} {
			updated, _ = m.Update(tea.MouseMsg{X: x, Y: l.branchesBoxTop, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			m = updated.(Model)
			if m.showRemoteBranches {
				t.Fatalf("width %d clicking border at %d changed tabs", width, x)
			}
		}
	}
}

func TestModel_MouseRowsMatchLongFileAndMultilineRevisionDisplay(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	longPath := strings.Repeat("a", 90) + ".txt"
	updated, _ = m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: longPath}, {Status: 'M', Path: "b.txt"}}}})
	m = updated.(Model)
	m.files.Select(1)
	l := m.computeMouseLayout()
	lines := strings.Split(m.View(), "\n")
	if !strings.Contains(lines[l.filesBoxTop+3], "b.txt") || m.files.SelectedItem().(fileItem).change.Path != longPath {
		t.Fatal("selected long filename changed row geometry or its raw action path")
	}
	updated, _ = m.Update(tea.MouseMsg{X: 5, Y: l.filesBoxTop + 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = updated.(Model)
	if m.files.SelectedItem().(fileItem).change.Path != "b.txt" {
		t.Fatal("click on visible b.txt selected another file")
	}
	updated, _ = m.Update(historyMsg{revisions: []lore.Revision{{Hash: "first", Message: "summary\nbody"}, {Hash: "second", Message: "next"}}})
	m = updated.(Model)
	lines = strings.Split(m.View(), "\n")
	if !strings.Contains(lines[l.historyBoxTop+2], "second") || m.history.Items()[0].(revisionItem).revision.Message != "summary\nbody" {
		t.Fatal("multiline revision changed row geometry or its raw metadata")
	}
	updated, _ = m.Update(tea.MouseMsg{X: 5, Y: l.historyBoxTop + 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = updated.(Model)
	if m.history.SelectedItem().(revisionItem).revision.Hash != "second" {
		t.Fatal("click on visible second revision selected another revision")
	}
}

func TestModel_FolderDiscardKeepsPromptSnapshotAfterRefresh(t *testing.T) {
	for _, tc := range []struct {
		name        string
		path        string
		key         string
		want        []string
		stagedAfter bool
	}{
		{"folder all", "src", "x", []string{"src/staged.go", "src/known.go"}, false},
		{"folder unstaged", "src", "u", []string{"src/known.go"}, false},
		{"root all", "", "x", []string{"src/staged.go", "src/known.go", "other/known.go"}, false},
		{"root all via Enter", "", "enter", []string{"src/staged.go", "src/known.go", "other/known.go"}, false},
		{"folder unstaged via Space", "src", " ", []string{"src/known.go"}, false},
		{"root unstaged", "", "u", []string{"src/known.go", "other/known.go"}, false},
		{"unstaged file staged after prompt", "src", "u", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &lore.FakeRunner{Results: map[string]lore.Result{
				"--json unstage -- " + strings.Join(tc.want, " "):       {Stdout: jsonCompleteSuccess},
				"--json reset --purge -- " + strings.Join(tc.want, " "): {Stdout: jsonCompleteSuccess},
			}}
			m := NewModel(fake, "repo", "/repo")
			status := lore.Status{
				Staged: []lore.FileChange{{Status: 'M', Path: "src/staged.go"}},
				Unstaged: []lore.FileChange{
					{Status: 'M', Path: "src", Directory: true},
					{Status: 'M', Path: "src/known.go"},
					{Status: 'M', Path: "other/known.go"},
				},
			}
			updated, _ := m.Update(statusMsg{status: status})
			m = updated.(Model)
			found := false
			for i, item := range m.files.Items() {
				file := item.(fileItem)
				if file.isDir && file.path == tc.path {
					m.files.Select(i)
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing directory row %q", tc.path)
			}
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
			m = updated.(Model)
			status.Unstaged = append(status.Unstaged,
				lore.FileChange{Status: 'A', Path: "src/late.go"},
				lore.FileChange{Status: 'M', Path: "other/late.go"},
			)
			if tc.stagedAfter {
				status.Staged = append(status.Staged, status.Unstaged[1])
				status.Unstaged = append(status.Unstaged[:1], status.Unstaged[2:]...)
			}
			updated, _ = m.Update(statusMsg{status: status})
			m = updated.(Model)
			key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)}
			if tc.key == "enter" {
				key = tea.KeyMsg{Type: tea.KeyEnter}
			} else if tc.key == " " {
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = updated.(Model)
				key = tea.KeyMsg{Type: tea.KeySpace}
			}
			updated, cmd := m.Update(key)
			m = updated.(Model)
			runBatch(cmd)
			if len(tc.want) == 0 && len(fake.Calls) != 0 {
				t.Fatalf("discard touched a newly staged file: %v", fake.Calls)
			}
			if len(tc.want) > 0 {
				expected := []string{"--json reset --purge -- " + strings.Join(tc.want, " ")}
				if tc.key == "x" || tc.key == "enter" {
					expected = append([]string{"--json unstage -- " + strings.Join(tc.want, " ")}, expected...)
				}
				var calls []string
				for _, call := range fake.Calls {
					calls = append(calls, strings.Join(call, " "))
				}
				if strings.Join(calls, "\n") != strings.Join(expected, "\n") {
					t.Fatalf("discard broadened its targets: %v", fake.Calls)
				}
			}
			if m.prompt != promptNone || len(m.pendingDiscardPaths) != 0 || len(m.pendingDiscardUnstaged) != 0 {
				t.Fatal("confirmed menu retained its snapshot")
			}
		})
	}
}

func TestDiscardMenuNavigationDisabledOptionAndCancel(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, _ = m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "file.txt"}}}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	for _, key := range []tea.KeyMsg{{Type: tea.KeyUp}, {Type: tea.KeyDown}, {Type: tea.KeyEnter}, {Type: tea.KeySpace}} {
		updated, cmd := m.Update(key)
		m = updated.(Model)
		if cmd != nil || m.prompt != promptDiscardMenu || len(fake.Calls) != 0 {
			t.Fatalf("key %q executed work or closed the menu: prompt=%v cursor=%d cmd=%v calls=%v", key.String(), m.prompt, m.discardCursor, cmd != nil, fake.Calls)
		}
	}
	if m.discardCursor != 1 {
		t.Fatal("disabled option was not selectable")
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 20, Height: 5}, {Width: 120, Height: 40}} {
		updated, _ = m.Update(size)
		m = updated.(Model)
		if m.discardCursor != 1 || m.pendingDiscardPath != "file.txt" {
			t.Fatal("resize lost the menu selection or discard target")
		}
	}
	for _, key := range []string{"j", "j", "k", "j"} {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = updated.(Model)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.prompt != promptNone || len(fake.Calls) != 0 || len(m.pendingDiscardPaths) != 0 {
		t.Fatal("Cancel executed work or retained the discard snapshot")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if updated.(Model).discardCursor != 0 {
		t.Fatal("reopening did not select the first option")
	}
}

func TestConfirmationKeysOnlyConfirmOrCancelExplicitly(t *testing.T) {
	for _, prompt := range []promptKind{
		promptConfirmStageAllForCommit, promptConfirmDiscardAll, promptConfirmBranchReset,
		promptConfirmBranchMerge, promptConfirmRevert, promptConfirmForceUnlock,
	} {
		for _, key := range []tea.KeyMsg{{Type: tea.KeyEnter}, {Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune("n")}} {
			m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
			updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "known.txt"}}}})
			m = updated.(Model)
			m.prompt = prompt
			m.pendingDiscardPaths = []string{"known.txt"}
			m.pendingResetRevision = "revision"
			m.pendingMergeBranch = "branch"
			m.pendingRevertMessage = "Revert revision"
			m.pendingForceUnlockPath = "known.txt"
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
			m = updated.(Model)
			if cmd != nil || m.prompt != prompt || len(m.pendingDiscardPaths) != 1 || m.pendingResetRevision != "revision" {
				t.Fatalf("unrelated key changed confirmation %v or its pending targets", prompt)
			}
			updated, cmd = m.Update(key)
			m = updated.(Model)
			if m.prompt != promptNone || (cmd != nil) != (key.Type == tea.KeyEnter) {
				t.Fatalf("confirmation %v handling %q: prompt=%v cmd=%v", prompt, key.String(), m.prompt, cmd != nil)
			}
		}
	}
}

func TestHelpExecutionRetainsConfirmationAndSelectionAfterResize(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, _ = m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "known.txt"}}}})
	m = updated.(Model)
	m.openHelp()
	for i, row := range m.helpRows {
		if row.binding == "D" {
			m.helpCursor = i
			break
		}
	}
	cursor := m.helpCursor
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 16}, {Width: 120, Height: 40}} {
		updated, _ = m.Update(size)
		m = updated.(Model)
		if m.helpCursor != cursor {
			t.Fatal("resize reset the selected help binding")
		}
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.showHelp || m.prompt != promptConfirmDiscardAll || len(fake.Calls) != 0 || len(m.pendingDiscardPaths) != 1 {
		t.Fatal("executing a destructive help binding bypassed its confirmation")
	}
}

func TestModel_SpaceOnDirectorySparesNamesakeSiblingFile(t *testing.T) {
	// A UE-style project can have a plain file and a directory sharing the
	// exact same path segment (e.g. "SonarV2" the file, "SonarV2/" the
	// folder, both siblings under the same parent). Space on the directory
	// row must only stage files strictly nested inside it, not the
	// unrelated namesake file that merely shares its path string
	// (dirPrefixMatches used to treat filePath == dirPath as "inside").
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- SonarV2": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{
		{Status: 'A', Path: "SonarV2"},                      // the namesake file
		{Status: 'A', Path: "SonarV2/BP_DummySonar.uasset"}, // inside the directory
	}}})
	m2 := updated.(Model)

	// Move the cursor onto the directory row (not the namesake leaf).
	var dirIdx int
	for i, it := range m2.files.Items() {
		fi := it.(fileItem)
		if fi.isDir && fi.path == "SonarV2" {
			dirIdx = i
		}
	}
	m2.files.Select(dirIdx)

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	m3 := updated.(Model)
	runBatch(cmd)

	if len(fake.Calls) != 1 || strings.Join(fake.Calls[0], " ") != "--json stage -- SonarV2" {
		t.Fatalf("Calls = %+v, want a single 'stage SonarV2' (directory) call", fake.Calls)
	}
	for _, it := range m3.files.Items() {
		fi := it.(fileItem)
		if !fi.isDir && fi.path == "SonarV2" && fi.staged {
			t.Fatal("the namesake leaf file 'SonarV2' must not be optimistically staged by staging the directory")
		}
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

func TestModel_SpaceOnCollapsedDirectoryStagesHiddenFiles(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- src": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{
		{Status: 'M', Path: "src/a.go"},
		{Status: 'M', Path: "src/b.go"},
	}}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(Model)
	updated, cmd := m3.Update(tea.KeyMsg{Type: tea.KeySpace})
	m4 := updated.(Model)
	runBatch(cmd)

	if len(fake.Calls) != 1 || strings.Join(fake.Calls[0], " ") != "--json stage -- src" {
		t.Fatalf("Calls = %+v, want stage src despite its children being hidden", fake.Calls)
	}
	if hasUnstaged, hasStaged := m4.dirStageCounts("src"); hasUnstaged || !hasStaged {
		t.Fatalf("dirStageCounts(src) = %v,%v, want false,true after optimistic stage", hasUnstaged, hasStaged)
	}
	item := m4.files.SelectedItem().(fileItem)
	if !item.allStaged {
		t.Fatal("collapsed src row should be visibly staged after its hidden files are staged")
	}
}

func TestModel_CommitSeesStagedFilesInsideCollapsedDirectory(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Staged: []lore.FileChange{
		{Status: 'M', Path: "src/a.go"},
		{Status: 'M', Path: "src/b.go"},
	}}})
	m2 := updated.(Model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(Model)
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m4 := updated.(Model)

	if m4.prompt != promptCommit {
		t.Fatalf("prompt = %v, want commit prompt for hidden staged files", m4.prompt)
	}
	if m4.err != nil {
		t.Fatalf("unexpected error for hidden staged files: %v", m4.err)
	}
}

func TestModel_SpaceStagesChangedEmptyDirectory(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- Content/Empty": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{
		{Status: 'A', Path: "Content/Empty", Directory: true},
	}}})
	m2 := updated.(Model)

	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeySpace})
	runBatch(cmd)

	if len(fake.Calls) != 1 || strings.Join(fake.Calls[0], " ") != "--json stage -- Content/Empty" {
		t.Fatalf("Calls = %+v, want stage Content/Empty", fake.Calls)
	}
}

func TestModel_AKeyUsesHiddenChangesWhenDirectoryCollapsed(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- .": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	m := NewModel(fake, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{
		Staged:   []lore.FileChange{{Status: 'M', Path: "visible.txt"}},
		Unstaged: []lore.FileChange{{Status: 'M', Path: "src/hidden.go"}},
	}})
	m2 := updated.(Model)

	for i, item := range m2.files.Items() {
		file := item.(fileItem)
		if file.isDir && file.path == "src" {
			m2.files.Select(i)
			break
		}
	}
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(Model)
	_, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	runBatch(cmd)

	if len(fake.Calls) != 1 || strings.Join(fake.Calls[0], " ") != "--json stage -- ." {
		t.Fatalf("Calls = %+v, want stage . because a hidden file is unstaged", fake.Calls)
	}
}

func TestModel_SecondPushIsIgnoredWhileFirstIsRunning(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, firstCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m2 := updated.(Model)
	if firstCmd == nil || !m2.pushInFlight {
		t.Fatal("first push should start and mark a push in flight")
	}
	logBefore := m2.log.View()

	updated, secondCmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m3 := updated.(Model)
	if secondCmd != nil {
		t.Fatal("second push should be ignored while the first is running")
	}
	if m3.log.View() != logBefore {
		t.Fatal("second push should not replace the first live Command Log entry")
	}
}
