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
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
				return m, resetCmd(m.runner, item.change.Path)
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

func (m Model) updateFocusedList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case focusFiles:
		m.files, cmd = m.files.Update(msg)
	case focusBranches:
		m.branches, cmd = m.branches.Update(msg)
	case focusHistory:
		m.history, cmd = m.history.Update(msg)
	}
	return m, cmd
}

func (m Model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
