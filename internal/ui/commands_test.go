package ui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

// jsonCompleteSuccess is the minimal real shape of a --json command's
// terminal event on success - shared by every test in this package that
// exercises a Runner call without needing to assert on the response body.
const jsonCompleteSuccess = `{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`

func TestDiscardCmd_RejectsFileReplacedByDirectory(t *testing.T) {
	for name, discard := range map[string]func(lore.Runner, string, []string) tea.Cmd{
		"all":      discardAllCmd,
		"unstaged": discardUnstagedInDirCmd,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "changed.txt")
			if err := os.WriteFile(path, []byte("pending change"), 0o600); err != nil {
				t.Fatal(err)
			}
			fake := &lore.FakeRunner{}
			cmd := discard(fake, root, []string{"changed.txt"})
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			later := filepath.Join(path, "later.txt")
			if err := os.WriteFile(later, []byte("keep this"), 0o600); err != nil {
				t.Fatal(err)
			}
			msg := cmd().(actionDoneMsg)
			if msg.err == nil || !strings.Contains(msg.err.Error(), "directory") {
				t.Fatalf("error = %v, want a changed-directory error", msg.err)
			}
			if len(fake.Calls) != 0 {
				t.Fatalf("discard ran commands on a new directory: %v", fake.Calls)
			}
			content, err := os.ReadFile(later)
			if err != nil || string(content) != "keep this" {
				t.Fatalf("new file changed: %q, %v", content, err)
			}
		})
	}
}

func TestLoadStatusCmd_ReturnsStatusMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json status --scan": {ExitCode: 0, Stdout: `{"tagName":"repositoryStatusRevision","data":{"repository":"abc","branchName":"main"}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
	}}
	msg := loadStatusCmd(fake, 7)()
	sm, ok := msg.(statusMsg)
	if !ok {
		t.Fatalf("msg = %#v, want statusMsg", msg)
	}
	if sm.err != nil {
		t.Fatalf("unexpected error: %v", sm.err)
	}
	if sm.generation != 7 {
		t.Fatalf("generation = %d, want 7", sm.generation)
	}
	if sm.status.Repository != "abc" {
		t.Fatalf("Repository = %q, want abc", sm.status.Repository)
	}
}

func TestLoadBranchesCmd_ReturnsBranchesMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json branch list": {ExitCode: 0, Stdout: `{"tagName":"branchListEntry","data":{"location":"local","name":"main","isCurrent":true}}
{"tagName":"branchListEntry","data":{"location":"remote","name":"main","isCurrent":false}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
	}}
	msg := loadBranchesCmd(fake, 7)()
	bm, ok := msg.(branchesMsg)
	if !ok {
		t.Fatalf("msg = %#v, want branchesMsg", msg)
	}
	if len(bm.branches) != 2 {
		t.Fatalf("branches = %+v, want 2 entries", bm.branches)
	}
	if bm.generation != 7 {
		t.Fatalf("generation = %d, want 7", bm.generation)
	}
}

func TestLoadHistoryCmd_ReturnsHistoryMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json history 50": {ExitCode: 0, Stdout: `{"tagName":"revisionHistoryEntry","data":{"revision":"abc","revisionNumber":1,"parent":["0","0"]}}
{"tagName":"metadata","data":{"key":"message","value":{"tagName":"string","data":"Initial revision"}}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
	}}
	msg := loadHistoryCmd(fake, 7)()
	hm, ok := msg.(historyMsg)
	if !ok {
		t.Fatalf("msg = %#v, want historyMsg", msg)
	}
	if len(hm.revisions) != 1 {
		t.Fatalf("revisions = %+v, want 1 entry", hm.revisions)
	}
	if hm.generation != 7 {
		t.Fatalf("generation = %d, want 7", hm.generation)
	}
}

func TestLoadDiffCmd_ReturnsDiffMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json diff -- hello.txt": {ExitCode: 0, Stdout: `{"tagName":"fileDiff","data":{"path":"hello.txt","patch":"+++ hello.txt\n","action":"keep"}}
{"tagName":"complete","data":{"status":0,"error":{"errorCode":0,"message":"","traceLocations":[]}}}
`},
	}}
	msg := loadDiffCmd(fake, "hello.txt", lore.Lock{}, false, mainContentRequest{})()
	dm, ok := msg.(diffMsg)
	if !ok {
		t.Fatalf("msg = %#v, want diffMsg", msg)
	}
	if dm.text != "+++ hello.txt\n" {
		t.Fatalf("text = %q", dm.text)
	}
}

func TestLoadDiffCmd_PrependsLockLineWhenLocked(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json diff -- hello.txt": {ExitCode: 0, Stdout: `{"tagName":"fileDiff","data":{"path":"hello.txt","patch":"+++ hello.txt\n","action":"keep"}}
` + jsonCompleteSuccess},
	}}
	msg := loadDiffCmd(fake, "hello.txt", lore.Lock{Path: "hello.txt", Owner: "user-123"}, true, mainContentRequest{})()
	dm := msg.(diffMsg)
	want := "Locked by user-123\n\n+++ hello.txt\n"
	if dm.text != want {
		t.Fatalf("text = %q, want %q", dm.text, want)
	}
}

func TestLoadDiffCmd_RenamesBareBinaryMarkerWithThePath(t *testing.T) {
	// lore's own binary-diff marker has no filename baked in (unlike git's),
	// see lore-revision/src/file/diff.rs's emit_binary_diff.
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json diff -- a.uasset": {ExitCode: 0, Stdout: `{"tagName":"fileDiff","data":{"path":"a.uasset","patch":"Binary files differ\n","action":"keep"}}
` + jsonCompleteSuccess},
	}}
	msg := loadDiffCmd(fake, "a.uasset", lore.Lock{}, false, mainContentRequest{})()
	dm := msg.(diffMsg)
	want := "Binary file a.uasset differs\n"
	if dm.text != want {
		t.Fatalf("text = %q, want %q", dm.text, want)
	}
}

func TestStageCmd_CallsRunnerAndReturnsActionDoneMsg(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- hello.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	msg := stageCmd(fake, "hello.txt")()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err != nil {
		t.Fatalf("unexpected error: %v", am.err)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("Calls = %+v, want exactly one call", fake.Calls)
	}
	if am.label != "Stage file" {
		t.Fatalf("label = %q, want %q", am.label, "Stage file")
	}
	if want := []string{"lore stage -- hello.txt"}; !reflect.DeepEqual(am.commands, want) {
		t.Fatalf("commands = %+v, want %+v (no --json - that's plumbing, not something a user would type)", am.commands, want)
	}
}

func TestDiscardAllCmd_CommandLogShowsBothRealCommands(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- a.txt b.txt":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge -- a.txt b.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	msg := discardAllCmd(fake, t.TempDir(), []string{"a.txt", "b.txt"})()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	want := []string{"lore unstage -- a.txt b.txt", "lore reset --purge -- a.txt b.txt"}
	if !reflect.DeepEqual(am.commands, want) {
		t.Fatalf("commands = %+v, want %+v", am.commands, want)
	}
}

func TestStageCmd_RevertFlipsFileBackToUnstagedOnFailure(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json stage -- hello.txt": {ExitCode: 1, Stdout: `{"tagName":"complete","data":{"status":-1,"error":{"errorCode":-1,"message":"conflict","traceLocations":[]}}}` + "\n"},
	}}
	msg := stageCmd(fake, "hello.txt")()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err == nil {
		t.Fatal("expected an error")
	}
	if am.opKey != "stage:hello.txt" {
		t.Fatalf("opKey = %q, want %q", am.opKey, "stage:hello.txt")
	}
	if am.revert == nil {
		t.Fatal("expected a non-nil revert func on failure")
	}

	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Staged: []lore.FileChange{{Status: 'M', Path: "hello.txt"}}}})
	m2 := updated.(Model)
	am.revert(&m2)
	item, ok := m2.files.SelectedItem().(fileItem)
	if !ok || item.staged {
		t.Fatalf("expected hello.txt flipped back to unstaged after revert, got %+v", item)
	}
}

func TestUnstageCmd_RevertFlipsFileBackToStagedOnFailure(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- hello.txt": {ExitCode: 1, Stdout: `{"tagName":"complete","data":{"status":-1,"error":{"errorCode":-1,"message":"boom","traceLocations":[]}}}` + "\n"},
	}}
	msg := unstageCmd(fake, "hello.txt")()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.opKey != "stage:hello.txt" {
		t.Fatalf("opKey = %q, want %q", am.opKey, "stage:hello.txt")
	}
	if am.revert == nil {
		t.Fatal("expected a non-nil revert func on failure")
	}

	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "hello.txt"}}}})
	m2 := updated.(Model)
	am.revert(&m2)
	item, ok := m2.files.SelectedItem().(fileItem)
	if !ok || !item.staged {
		t.Fatalf("expected hello.txt flipped back to staged after revert, got %+v", item)
	}
}

func TestLockToggleCmd_AcquireRevertsLockedFlagOnFailure(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock acquire -- hello.txt": {ExitCode: 1, Stdout: `{"tagName":"complete","data":{"status":-1,"error":{"errorCode":-1,"message":"boom","traceLocations":[]}}}` + "\n"},
	}}
	msg := lockToggleCmd(fake, "hello.txt", false)()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.opKey != "lock:hello.txt" {
		t.Fatalf("opKey = %q, want %q", am.opKey, "lock:hello.txt")
	}
	if am.revert == nil {
		t.Fatal("expected a non-nil revert func on failure")
	}

	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "hello.txt"}}}})
	m2 := updated.(Model)
	am.revert(&m2)
	item, ok := m2.files.SelectedItem().(fileItem)
	if !ok || item.locked {
		t.Fatalf("expected hello.txt flipped back to unlocked after a failed acquire, got %+v", item)
	}
}

func TestLockToggleCmd_AcquireConfirmUsesCurrentUserIDAsOwner(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock acquire -- hello.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	msg := lockToggleCmd(fake, "hello.txt", false)()
	am := msg.(actionDoneMsg)
	if am.confirm == nil {
		t.Fatal("expected a non-nil confirm func on success")
	}

	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	m.currentUserID = "user-123"
	am.confirm(&m)
	if got := m.locks["hello.txt"].Owner; got != "user-123" {
		t.Fatalf("locks[hello.txt].Owner = %q, want %q", got, "user-123")
	}
}

func TestLockToggleCmd_AcquireConfirmFallsBackToUnknownOwnerWhenUnauthenticated(t *testing.T) {
	// With no resolvable identity, currentUserID is "" - the optimistic lock
	// record must fall back to a placeholder owner, not store that empty
	// string directly (which would show "Locked by " with nothing after it).
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json lock acquire -- hello.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	msg := lockToggleCmd(fake, "hello.txt", false)()
	am := msg.(actionDoneMsg)

	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	// m.currentUserID left at its zero value ("").
	am.confirm(&m)
	if got := m.locks["hello.txt"].Owner; got != "<unknown>" {
		t.Fatalf("locks[hello.txt].Owner = %q, want %q", got, "<unknown>")
	}
}

func TestDiscardAllCmd_CallsRunnerForEveryPath(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json unstage -- a.txt b.txt":       {ExitCode: 0, Stdout: jsonCompleteSuccess},
		"--json reset --purge -- a.txt b.txt": {ExitCode: 0, Stdout: jsonCompleteSuccess},
	}}
	msg := discardAllCmd(fake, t.TempDir(), []string{"a.txt", "b.txt"})()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err != nil {
		t.Fatalf("unexpected error: %v", am.err)
	}
	if len(fake.Calls) != 2 {
		t.Fatalf("Calls = %+v, want 2 calls", fake.Calls)
	}
}

func TestEditorCommand_UsesVisualOverEditor(t *testing.T) {
	t.Setenv("VISUAL", "myvisual")
	t.Setenv("EDITOR", "myeditor")
	c, err := editorCommand("/repo/a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.Args[0]; got != "myvisual" {
		t.Fatalf("editor binary = %q, want %q", got, "myvisual")
	}
	if got := c.Args[len(c.Args)-1]; got != "/repo/a.txt" {
		t.Fatalf("last arg = %q, want the target path", got)
	}
}

func TestEditorCommand_FallsBackToEditor(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "code -w")
	c, err := editorCommand("/repo/a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.Args[0]; got != "code" {
		t.Fatalf("editor binary = %q, want %q", got, "code")
	}
	if got := c.Args[1]; got != "-w" {
		t.Fatalf("extra editor arg = %q, want %q", got, "-w")
	}
}

func TestEditorCommand_UsesInstalledDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses notepad as its platform default")
	}
	editors := []string{"vi", "vim", "nvim", "nano"}
	for index, editor := range editors {
		t.Run(editor, func(t *testing.T) {
			dir := t.TempDir()
			for _, available := range editors[index:] {
				if err := os.WriteFile(filepath.Join(dir, available), nil, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", dir)
			t.Setenv("VISUAL", "")
			t.Setenv("EDITOR", "")
			command, err := editorCommand("/repo/a.txt")
			if err != nil {
				t.Fatal(err)
			}
			if command.Path != filepath.Join(dir, editor) || !reflect.DeepEqual(command.Args, []string{editor, "/repo/a.txt"}) {
				t.Fatalf("command = %q %#v, want installed %q", command.Path, command.Args, editor)
			}
		})
	}
}

func TestEditorCommand_ErrorsWithoutInstalledDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses notepad as its platform default")
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if _, err := editorCommand("/repo/a.txt"); err == nil || err.Error() != "no editor found on PATH; set VISUAL or EDITOR" {
		t.Fatalf("error = %v, want missing editor configuration guidance", err)
	}
}

func TestEditorCommand_PreservesConfiguredMissingEditorError(t *testing.T) {
	for _, variable := range []string{"VISUAL", "EDITOR"} {
		t.Run(variable, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			t.Setenv("VISUAL", "")
			t.Setenv("EDITOR", "")
			t.Setenv(variable, "missing-editor --wait")
			command, err := editorCommand("/repo/a.txt")
			if err != nil {
				t.Fatal(err)
			}
			if command.Args[0] != "missing-editor" || !errors.Is(command.Err, exec.ErrNotFound) {
				t.Fatalf("command = %#v, error = %v, want configured missing editor", command.Args, command.Err)
			}
		})
	}
}

func TestEditorCommand_PreservesQuotedExecutablePathAndArguments(t *testing.T) {
	t.Setenv("VISUAL", `"C:\Program Files\Editor\editor.exe" --wait "two words"`)
	c, err := editorCommand(`C:\repo\a.txt`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{`C:\Program Files\Editor\editor.exe`, "--wait", "two words", `C:\repo\a.txt`}
	if !reflect.DeepEqual(c.Args, want) {
		t.Fatalf("Args = %#v, want %#v", c.Args, want)
	}
}

func TestEditorCommand_RejectsUnterminatedQuote(t *testing.T) {
	t.Setenv("VISUAL", `"C:\Program Files\Editor\editor.exe`)
	if _, err := editorCommand(`C:\repo\a.txt`); err == nil {
		t.Fatal("expected an error for an unterminated quote")
	}
}

func TestCommitCmd_ReturnsErrorOnFailure(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{
		"--json commit -- oops": {ExitCode: 1, Stdout: "{\"tagName\":\"complete\",\"data\":{\"status\":-1,\"error\":{\"errorCode\":-1,\"message\":\"nothing staged\",\"traceLocations\":[]}}}\n"},
	}}
	msg := commitCmd(fake, "oops")()
	am, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("msg = %#v, want actionDoneMsg", msg)
	}
	if am.err == nil {
		t.Fatal("expected an error to be carried on actionDoneMsg")
	}
}
