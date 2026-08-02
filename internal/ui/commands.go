package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

func loadStatusCmd(r lore.Runner) tea.Cmd {
	return func() tea.Msg {
		s, err := lore.GetStatus(r)
		return statusMsg{status: s, err: err}
	}
}

func loadBranchesCmd(r lore.Runner) tea.Cmd {
	return func() tea.Msg {
		b, err := lore.BranchList(r)
		return branchesMsg{branches: b, err: err}
	}
}

func loadHistoryCmd(r lore.Runner) tea.Cmd {
	return func() tea.Msg {
		h, err := lore.History(r, 50)
		return historyMsg{revisions: h, err: err}
	}
}

// loadDiffCmd loads a file's diff for the Files panel's main content, then
// enriches it (see enrichFileDiffText) with lock ownership and a real
// filename on lore's otherwise-bare binary-diff marker. lock/locked come
// from the caller's already-loaded m.locks - not re-fetched here.
func loadDiffCmd(r lore.Runner, path string, lock lore.Lock, locked bool, request mainContentRequest) tea.Cmd {
	return func() tea.Msg {
		text, err := lore.Diff(r, path)
		if err != nil {
			return diffMsg{request: request, err: err}
		}
		return diffMsg{request: request, text: enrichFileDiffText(text, path, lock, locked)}
	}
}

// enrichFileDiffText prepends a lock line when the file is locked, and
// names the file in lore's bare "Binary files differ" marker (it has no
// filename baked in) - for a locked binary file the diff text is otherwise
// the only thing shown, so both are worth surfacing.
func enrichFileDiffText(text, path string, lock lore.Lock, locked bool) string {
	const binaryMarker = "Binary files differ"
	if strings.Contains(text, binaryMarker) {
		text = strings.ReplaceAll(text, binaryMarker, "Binary file "+path+" differs")
	}
	if locked {
		text = "Locked by " + lock.Owner + "\n\n" + text
	}
	return text
}

func loadCurrentUserCmd(r lore.Runner) tea.Cmd {
	return func() tea.Msg {
		id, err := lore.CurrentUserID(r)
		return currentUserMsg{id: id, err: err}
	}
}

// loadBranchLogCmd fills the main panel with branch's revision log, for
// when the Branches panel is focused.
func loadBranchLogCmd(r lore.Runner, branch string, request mainContentRequest) tea.Cmd {
	return func() tea.Msg {
		revisions, err := lore.HistoryForBranch(r, branch, 50)
		if err != nil {
			return diffMsg{request: request, err: err}
		}
		return diffMsg{request: request, text: formatBranchLog(revisions), raw: true}
	}
}

// loadRevisionPatchCmd fills the main panel with the selected revision's
// full patch. Callers pass parent == "" for the root revision - it has no
// parent to diff against, so this skips the call rather than guess.
func loadRevisionPatchCmd(r lore.Runner, parent, revision string, request mainContentRequest) tea.Cmd {
	return func() tea.Msg {
		if parent == "" {
			return diffMsg{request: request, text: "Initial revision - no parent to diff against.", raw: true}
		}
		text, err := lore.DiffRevision(r, parent, revision)
		return diffMsg{request: request, text: text, err: err}
	}
}

func loadLocksCmd(r lore.Runner, paths []string) tea.Cmd {
	return func() tea.Msg {
		locks, err := lore.LockStatus(r, paths...)
		return locksMsg{locks: locks, err: err}
	}
}

// runRecorder wraps a Runner and records the display form of every command
// it runs (args minus the leading "--json", which is plumbing a user typing
// the command themselves wouldn't include) - for the Command Log (see
// commandlog.go), matching lazygit's own LogCommand next to each LogAction.
type runRecorder struct {
	inner    lore.Runner
	commands []string
}

func (rr *runRecorder) Run(args ...string) (lore.Result, error) {
	res, err := rr.inner.Run(args...)
	display := args
	if len(display) > 0 && display[0] == "--json" {
		display = display[1:]
	}
	rr.commands = append(rr.commands, "lore "+strings.Join(display, " "))
	return res, err
}

// RunStream makes runRecorder itself satisfy lore.StreamRunner, delegating
// to inner's own RunStream when it has one (ExecRunner does) and falling
// back to a plain Run otherwise (FakeRunner, in tests) - runCheckedStream
// only streams live when the Runner it's actually given supports it.
func (rr *runRecorder) RunStream(onLine func(string), args ...string) (lore.Result, error) {
	display := args
	if len(display) > 0 && display[0] == "--json" {
		display = display[1:]
	}
	rr.commands = append(rr.commands, "lore "+strings.Join(display, " "))
	if sr, ok := rr.inner.(lore.StreamRunner); ok {
		return sr.RunStream(onLine, args...)
	}
	return rr.inner.Run(args...)
}

func lockToggleCmd(r lore.Runner, path string, locked bool) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		opKey := "lock:" + path
		if locked {
			_, err := lore.LockRelease(rr, path)
			return actionDoneMsg{label: "Unlock file", err: err, opKey: opKey, commands: rr.commands,
				revert:  func(m *Model) { m.setFileLockedByPath(path, true) },
				confirm: func(m *Model) { delete(m.locks, path) },
			}
		}
		_, err := lore.LockAcquire(rr, path)
		return actionDoneMsg{label: "Lock file", err: err, opKey: opKey, commands: rr.commands,
			revert: func(m *Model) { m.setFileLockedByPath(path, false) },
			confirm: func(m *Model) {
				if m.locks == nil {
					m.locks = map[string]lore.Lock{}
				}
				// m.currentUserID is "" whenever lore auth info can't resolve an
				// identity (see project_lazylore_lock_owner_todo memory) - the
				// real lock status the next refresh fetches will report the
				// server's own "<unknown>" placeholder for that case, so use
				// the same placeholder here rather than a blank owner ("Locked
				// by " with nothing after it).
				owner := m.currentUserID
				if owner == "" {
					owner = "<unknown>"
				}
				m.locks[path] = lore.Lock{Path: path, Owner: owner}
			},
		}
	}
}

// lockForceReleaseCmd is lockToggleCmd's unlock branch for someone else's
// lock via LockReleaseForce - only reachable after the
// promptConfirmForceUnlock confirm, and only succeeds against a lore build
// with the AdminUnlock capability.
func lockForceReleaseCmd(r lore.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		opKey := "lock:" + path
		_, err := lore.LockReleaseForce(rr, path)
		return actionDoneMsg{label: "Force-unlock file", err: err, opKey: opKey, commands: rr.commands,
			revert:  func(m *Model) { m.setFileLockedByPath(path, true) },
			confirm: func(m *Model) { delete(m.locks, path) },
		}
	}
}

func stageCmd(r lore.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.Stage(rr, path)
		return actionDoneMsg{label: "Stage file", err: err, opKey: "stage:" + path, commands: rr.commands,
			revert: func(m *Model) { m.setFileStagedByPath(path, true, false) }}
	}
}

func unstageCmd(r lore.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.Unstage(rr, path)
		return actionDoneMsg{label: "Unstage file", err: err, opKey: "stage:" + path, commands: rr.commands,
			revert: func(m *Model) { m.setFileStagedByPath(path, false, true) }}
	}
}

// dirStageCmd is stageCmd's recursive analog for a directory/root row: a
// failed call reverts every optimistically-updated row under it, not just
// one path. dirPath is the fileItem-space prefix ("" for the root row);
// lorePath is the actual lore CLI argument ("." for the root row).
func dirStageCmd(r lore.Runner, dirPath, lorePath string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.Stage(rr, lorePath)
		return actionDoneMsg{label: "Stage folder", err: err, opKey: "stage:" + lorePath, commands: rr.commands,
			revert: func(m *Model) { m.setDirStagedByPrefix(dirPath, true, false) }}
	}
}

func dirUnstageCmd(r lore.Runner, dirPath, lorePath string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.Unstage(rr, lorePath)
		return actionDoneMsg{label: "Unstage folder", err: err, opKey: "stage:" + lorePath, commands: rr.commands,
			revert: func(m *Model) { m.setDirStagedByPrefix(dirPath, false, true) }}
	}
}

// discardUnstagedInDirCmd discards only the given paths - the unstaged
// files under a directory row's discard menu - leaving any staged files
// elsewhere under that directory untouched. Staging in lore is
// all-or-nothing per file, so this only makes sense at directory
// granularity, never for a single file.
func discardUnstagedInDirCmd(r lore.Runner, paths []string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.DiscardAllChanges(rr, paths)
		return actionDoneMsg{label: "Discard unstaged changes", err: err, commands: rr.commands}
	}
}

func discardAllCmd(r lore.Runner, paths []string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.DiscardAllChanges(rr, paths)
		return actionDoneMsg{label: "Discard all changes", err: err, commands: rr.commands}
	}
}

// editorCommand resolves the user's editor the same way git tooling
// conventionally does (VISUAL then EDITOR), falling back to a platform
// default, and builds the exec.Cmd to open absPath with it. Env values may
// carry quoted executable paths and extra args (for example
// `"C:\Program Files\Code.exe" --wait`).
func editorCommand(absPath string) (*exec.Cmd, error) {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}
	fields, err := splitEditorCommand(editor)
	if err != nil {
		return nil, err
	}
	args := append(append([]string{}, fields[1:]...), absPath)
	return exec.Command(fields[0], args...), nil
}

func editFileCmd(absPath string) tea.Cmd {
	cmd, err := editorCommand(absPath)
	if err != nil {
		return func() tea.Msg {
			return editorDoneMsg{err: err}
		}
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}

func splitEditorCommand(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	started := false
	runes := []rune(command)

	for index := 0; index < len(runes); index++ {
		char := runes[index]
		if char == '\\' && index+1 < len(runes) {
			next := runes[index+1]
			if next == quote || quote == 0 && (next == '\'' || next == '"' || unicode.IsSpace(next)) {
				current.WriteRune(next)
				started = true
				index++
				continue
			}
		}

		if quote != 0 {
			if char == quote {
				quote = 0
				continue
			}

			current.WriteRune(char)
			started = true
			continue
		}

		switch {
		case char == '\'' || char == '"':
			quote = char
			started = true
		case unicode.IsSpace(char):
			if started {
				args = append(args, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(char)
			started = true
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("invalid editor command: unterminated %c quote", quote)
	}
	if started {
		args = append(args, current.String())
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("invalid editor command: empty command")
	}

	return args, nil
}

func commitCmd(r lore.Runner, message string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.Commit(rr, message)
		return actionDoneMsg{label: "Commit", err: err, commands: rr.commands}
	}
}

func pullCmd(r lore.Runner) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.Pull(rr)
		return actionDoneMsg{label: "Pull", err: err, commands: rr.commands}
	}
}

// pushStreamCmd runs push in a goroutine and feeds each formatted progress
// line (see lore.FormatPushEventLine) to the Command Log as it arrives,
// instead of only showing a result once the whole push finishes - matching
// lazygit's own live command output. The channel is read one message at a
// time via readPushChan/pushChanMsg (see Update's handling of it), which
// re-issues the read after every line until the final actionDoneMsg closes
// the channel.
func pushStreamCmd(r lore.Runner) tea.Cmd {
	ch := make(chan tea.Msg, 64)
	go func() {
		rr := &runRecorder{inner: r}
		branchName := ""
		_, err := lore.PushStream(rr, func(tagName string, data json.RawMessage) {
			line, next, ok := lore.FormatPushEventLine(tagName, data, branchName)
			branchName = next
			if ok {
				select {
				case ch <- pushLineMsg(line):
				default:
					// Progress is best-effort. Never stall lore's stdout reader
					// just because the UI is temporarily busy or shutting down.
				}
			}
		})
		terminal := actionDoneMsg{label: "Push", err: err, commands: rr.commands, liveStreamed: true}
		for {
			select {
			case ch <- terminal:
				close(ch)
				return
			default:
				// Make room by discarding the oldest progress message. The
				// terminal result must always reach a running UI.
				select {
				case <-ch:
				default:
				}
			}
		}
	}()
	return readPushChan(ch)
}

func readPushChan(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return pushChanMsg{ch: ch, inner: msg}
	}
}

func switchBranchCmd(r lore.Runner, name string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.SwitchBranch(rr, name)
		return actionDoneMsg{label: "Checkout branch", err: err, commands: rr.commands}
	}
}

func createBranchCmd(r lore.Runner, name string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.CreateBranch(rr, name)
		return actionDoneMsg{label: "Create branch", err: err, commands: rr.commands}
	}
}

func resetBranchCmd(r lore.Runner, revision, label string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.ResetBranchTo(rr, revision)
		return actionDoneMsg{label: label, err: err, commands: rr.commands}
	}
}

func mergeBranchCmd(r lore.Runner, name, label string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.MergeBranch(rr, name)
		return actionDoneMsg{label: label, err: err, commands: rr.commands}
	}
}

func syncToCmd(r lore.Runner, revision, label string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.SyncTo(rr, revision)
		return actionDoneMsg{label: label, err: err, commands: rr.commands}
	}
}

func revertCmd(r lore.Runner, revision, message, label string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.RevertRevision(rr, revision, message)
		return actionDoneMsg{label: label, err: err, commands: rr.commands}
	}
}
