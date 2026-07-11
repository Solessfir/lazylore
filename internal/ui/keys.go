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
	focusStash
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

	case "v":
		// Drop mouse capture so the terminal's own click-drag selection works.
		m.selectMode = true
		return m, tea.DisableMouse

	case "tab", "l":
		m.focus = (m.focus + 1) % 5
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, nil

	case "shift+tab", "h":
		m.focus = (m.focus + 4) % 5
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, nil

	// Panel jump keys like lazygit (1/2=Files (under Status), 3=Branches, 4=History, 5=Stash, 6=Diff)
	case "1":
		m.focus = focusFiles
		m.syncFocusDelegates()
		return m, nil
	case "2":
		m.focus = focusFiles
		m.syncFocusDelegates()
		return m, nil
	case "3":
		m.focus = focusBranches
		m.syncFocusDelegates()
		return m, nil
	case "4":
		m.focus = focusHistory
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, nil
	case "5":
		m.focus = focusStash
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, nil
	case "6":
		m.focus = focusDiff
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
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
				if item.isDir {
					return m, m.toggleDirCollapse(item.path)
				}
				if item.staged {
					return m, tea.Batch(
						func() tea.Msg { return setAppStatusMsg("Unstaging...") },
						unstageCmd(m.runner, item.change.Path),
					)
				}
				return m, tea.Batch(
					func() tea.Msg { return setAppStatusMsg("Staging...") },
					stageCmd(m.runner, item.change.Path),
				)
			}
		}
		return m, nil

	case "d":
		if m.focus == focusFiles {
			if item, ok := m.files.SelectedItem().(fileItem); ok && !item.isDir {
				m.prompt = promptConfirmDiscard
				m.pendingDiscardPath = item.change.Path
			}
		}
		return m, nil

	case "enter":
		switch m.focus {
		case focusFiles:
			if item, ok := m.files.SelectedItem().(fileItem); ok {
				if item.isDir {
					return m, m.toggleDirCollapse(item.path)
				}
				return m, tea.Batch(
					func() tea.Msg { return setAppStatusMsg("Loading diff...") },
					loadDiffCmd(m.runner, item.change.Path),
				)
			}
		case focusBranches:
			if item, ok := m.branches.SelectedItem().(branchItem); ok {
				return m, tea.Batch(
					func() tea.Msg { return setAppStatusMsg("Switching branch...") },
					switchBranchCmd(m.runner, item.branch.Name),
				)
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
	case focusStash:
		return m.stashes.SettingFilter()
	}
	return false
}

func (m Model) updateFocusedList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case focusFiles:
		m.files, cmd = m.files.Update(msg)
		if dcmd := (&m).ensureDiffForSelectedFile(); dcmd != nil {
			cmd = tea.Batch(cmd, dcmd)
		}
	case focusBranches:
		m.branches, cmd = m.branches.Update(msg)
	case focusHistory:
		m.history, cmd = m.history.Update(msg)
	case focusStash:
		m.stashes, cmd = m.stashes.Update(msg)
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
			return m, tea.Batch(
				func() tea.Msg { return setAppStatusMsg("Discarding...") },
				resetCmd(m.runner, path),
			)
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
