package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type focusPanel int

const (
	focusFiles focusPanel = iota
	focusBranches
	focusHistory
	focusDiff
)

type promptKind int

const (
	promptNone promptKind = iota
	promptCommit
	promptNewBranch
	promptConfirmDiscard
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.focusedListIsFiltering() {
		return m.updateFocusedList(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab", "l":
		m.focus = (m.focus + 1) % 4
		return m, nil

	case "shift+tab", "h":
		m.focus = (m.focus + 3) % 4
		return m, nil

	case "c":
		m.prompt = promptCommit
		m.input = textinput.New()
		m.input.Placeholder = "commit message"
		m.input.Focus()
		return m, textinput.Blink

	case "n":
		m.prompt = promptNewBranch
		m.input = textinput.New()
		m.input.Placeholder = "branch name"
		m.input.Focus()
		return m, textinput.Blink

	case " ":
		if m.focus == focusFiles {
			if item, ok := m.files.SelectedItem().(fileItem); ok {
				if item.staged {
					return m, unstageCmd(m.runner, item.change.Path)
				}
				return m, stageCmd(m.runner, item.change.Path)
			}
		}
		return m, nil

	case "d":
		if m.focus == focusFiles {
			if item, ok := m.files.SelectedItem().(fileItem); ok {
				m.prompt = promptConfirmDiscard
				m.pendingDiscardPath = item.change.Path
			}
		}
		return m, nil

	case "enter":
		switch m.focus {
		case focusFiles:
			if item, ok := m.files.SelectedItem().(fileItem); ok {
				return m, loadDiffCmd(m.runner, item.change.Path)
			}
		case focusBranches:
			if item, ok := m.branches.SelectedItem().(branchItem); ok {
				return m, switchBranchCmd(m.runner, item.branch.Name)
			}
		}
		return m, nil
	}

	return m.updateFocusedList(msg)
}

// focusedListIsFiltering reports whether the list currently in focus is
// actively accepting filter input (i.e. the user pressed "/" and is typing
// a filter query). While true, the single-letter global shortcuts in
// handleKey must not fire - every keystroke belongs to the filter box.
// The diff panel has no filtering, so it's not part of this check.
func (m Model) focusedListIsFiltering() bool {
	switch m.focus {
	case focusFiles:
		return m.files.SettingFilter()
	case focusBranches:
		return m.branches.SettingFilter()
	case focusHistory:
		return m.history.SettingFilter()
	}
	return false
}

func (m Model) updateFocusedList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case focusFiles:
		m.files, cmd = m.files.Update(msg)
	case focusBranches:
		m.branches, cmd = m.branches.Update(msg)
	case focusHistory:
		m.history, cmd = m.history.Update(msg)
	case focusDiff:
		m.diff.vp, cmd = m.diff.vp.Update(msg)
	}
	return m, cmd
}

func (m Model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.prompt == promptConfirmDiscard {
		m.prompt = promptNone
		path := m.pendingDiscardPath
		m.pendingDiscardPath = ""
		if msg.String() == "y" {
			return m, resetCmd(m.runner, path)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.prompt = promptNone
		m.input.Blur()
		return m, nil

	case "enter":
		value := strings.TrimSpace(m.input.Value())
		kind := m.prompt
		m.prompt = promptNone
		m.input.Blur()
		if value == "" {
			return m, nil
		}
		switch kind {
		case promptCommit:
			return m, commitCmd(m.runner, value)
		case promptNewBranch:
			return m, createBranchCmd(m.runner, value)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
