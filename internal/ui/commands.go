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

func loadDiffCmd(r lore.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		text, err := lore.Diff(r, path)
		return diffMsg{text: text, err: err}
	}
}

func loadLocksCmd(r lore.Runner, paths []string) tea.Cmd {
	return func() tea.Msg {
		locks, err := lore.LockStatus(r, paths...)
		return locksMsg{locks: locks, err: err}
	}
}

func lockToggleCmd(r lore.Runner, path string, locked bool) tea.Cmd {
	return func() tea.Msg {
		if locked {
			_, err := lore.LockRelease(r, path)
			return actionDoneMsg{label: "unlock " + path, err: err}
		}
		_, err := lore.LockAcquire(r, path)
		return actionDoneMsg{label: "lock " + path, err: err}
	}
}

func stageCmd(r lore.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		_, err := lore.Stage(r, path)
		return actionDoneMsg{label: "stage " + path, err: err}
	}
}

func unstageCmd(r lore.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		_, err := lore.Unstage(r, path)
		return actionDoneMsg{label: "unstage " + path, err: err}
	}
}

func resetCmd(r lore.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		_, err := lore.DiscardChanges(r, path)
		return actionDoneMsg{label: "reset " + path, err: err}
	}
}

func discardAllCmd(r lore.Runner, paths []string) tea.Cmd {
	return func() tea.Msg {
		_, err := lore.DiscardAllChanges(r, paths)
		return actionDoneMsg{label: "discard all changes", err: err}
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
		_, err := lore.Commit(r, message)
		return actionDoneMsg{label: "commit", err: err}
	}
}

func switchBranchCmd(r lore.Runner, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := lore.SwitchBranch(r, name)
		return actionDoneMsg{label: "switch " + name, err: err}
	}
}

func createBranchCmd(r lore.Runner, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := lore.CreateBranch(r, name)
		return actionDoneMsg{label: "create branch " + name, err: err}
	}
}

func resetBranchCmd(r lore.Runner, revision, label string) tea.Cmd {
	return func() tea.Msg {
		_, err := lore.ResetBranchTo(r, revision)
		return actionDoneMsg{label: label, err: err}
	}
}

func syncToCmd(r lore.Runner, revision, label string) tea.Cmd {
	return func() tea.Msg {
		_, err := lore.SyncTo(r, revision)
		return actionDoneMsg{label: label, err: err}
	}
}
