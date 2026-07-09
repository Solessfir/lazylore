package ui

import (
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
		h, err := lore.HistoryOneline(r, 50)
		return historyMsg{revisions: h, err: err}
	}
}

func loadDiffCmd(r lore.Runner, path string) tea.Cmd {
	return func() tea.Msg {
		text, err := lore.Diff(r, path)
		return diffMsg{text: text, err: err}
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
		_, err := lore.Reset(r, path)
		return actionDoneMsg{label: "reset " + path, err: err}
	}
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
