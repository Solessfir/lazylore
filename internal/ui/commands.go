package ui

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

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
func loadDiffCmd(r lore.Runner, path string, lock lore.Lock, locked bool) tea.Cmd {
	return func() tea.Msg {
		text, err := lore.Diff(r, path)
		if err != nil {
			return diffMsg{err: err}
		}
		return diffMsg{text: enrichFileDiffText(text, path, lock, locked)}
	}
}

// enrichFileDiffText prepends a lock line when the file is locked, and
// renames lore's bare "Binary files differ" marker (lore-revision/src/file/
// diff.rs's emit_binary_diff has no filename baked in, unlike git's own
// binary-diff line) to name the file - for a locked binary file the diff
// text is otherwise the only thing shown, so both are worth surfacing here.
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
// when the Branches panel is focused (lazygit's "Log" main view).
func loadBranchLogCmd(r lore.Runner, branch string) tea.Cmd {
	return func() tea.Msg {
		revisions, err := lore.HistoryForBranch(r, branch, 50)
		if err != nil {
			return diffMsg{err: err}
		}
		return diffMsg{text: formatBranchLog(revisions), raw: true}
	}
}

// loadRevisionPatchCmd fills the main panel with the selected revision's
// full patch, for when the History panel is focused (lazygit's "Patch"
// main view). Callers pass parent == "" for the root revision (lore.
// IsZeroHash) - it has no parent to diff against, which lore diff --source
// can't target, so this skips the call rather than guess at a comparison
// lore doesn't expose.
func loadRevisionPatchCmd(r lore.Runner, parent, revision string) tea.Cmd {
	return func() tea.Msg {
		if parent == "" {
			return diffMsg{text: "Initial revision - no parent to diff against.", raw: true}
		}
		text, err := lore.DiffRevision(r, parent, revision)
		return diffMsg{text: text, err: err}
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

func resetCmd(r lore.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.DiscardChanges(rr, path)
		return actionDoneMsg{label: "Discard changes", err: err, commands: rr.commands}
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
// carry extra args (e.g. "code -w"), so only the first field is the binary.
func editorCommand(absPath string) *exec.Cmd {
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
	fields := strings.Fields(editor)
	args := append(append([]string{}, fields[1:]...), absPath)
	return exec.Command(fields[0], args...)
}

func editFileCmd(absPath string) tea.Cmd {
	return tea.ExecProcess(editorCommand(absPath), func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}

func commitCmd(r lore.Runner, message string) tea.Cmd {
	return func() tea.Msg {
		rr := &runRecorder{inner: r}
		_, err := lore.Commit(rr, message)
		return actionDoneMsg{label: "Commit", err: err, commands: rr.commands}
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
