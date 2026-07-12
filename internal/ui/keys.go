package ui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type focusPanel int

const (
	focusStatus focusPanel = iota
	focusFiles
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
	promptConfirmDiscardAll
	promptConfirmBranchReset
	promptConfirmRevert
	promptConfirmForceUnlock
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

	case "?":
		(&m).openHelp()
		return m, nil

	case "tab", "l":
		m.focus = (m.focus + 1) % 5
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, (&m).ensureMainContent()

	case "shift+tab", "h":
		m.focus = (m.focus + 4) % 5
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, (&m).ensureMainContent()

	// Panel jump keys, matching lazygit's real numbering exactly
	// (pkg/config/user_config.go's default Gui.SidePanels: 1=Status,
	// 2=Files, 3=Branches, 4=Commits, 5=Stash) except 5, repurposed for
	// Diff since lore has no stash - lazygit doesn't number its main panel
	// at all (it's reached via NextBlock/PrevBlock, not a jump key), but
	// lazylore's simpler layout treats Diff as a fifth jumpable panel.
	case "1":
		m.focus = focusStatus
		m.syncFocusDelegates()
		return m, nil
	case "2":
		m.focus = focusFiles
		m.syncFocusDelegates()
		return m, (&m).ensureMainContent()
	case "3":
		m.focus = focusBranches
		m.syncFocusDelegates()
		return m, (&m).ensureMainContent()
	case "4":
		m.focus = focusHistory
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, (&m).ensureMainContent()
	case "5":
		m.focus = focusDiff
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, nil

	// Cycle the focused panel's own sub-tabs, matching lazygit's global
	// NextTab/PrevTab ("]"/"["): only Branches has more than one in lore
	// (Local/Remotes - no Tags, no Files/History/Status equivalent to
	// git's worktrees/submodules/reflog), so this no-ops elsewhere, same
	// as lazygit on a single-tab window. Hidden from the bottom bar in
	// lazygit too (no DisplayOnScreen on that binding) - see "?" instead.
	case "[", "]":
		if m.focus == focusBranches {
			m.showRemoteBranches = !m.showRemoteBranches
			return m, m.refreshBranchesList()
		}
		return m, nil

	case "a":
		// Stage/unstage everything, matching lazygit's own "a"
		// (toggleStagedAll): same stage-if-anything's-unstaged-else-unstage
		// rule as space on a directory, just always applied to the whole
		// tree regardless of the current selection.
		if m.focus == focusFiles {
			return m, m.toggleDirStage("")
		}
		return m, nil

	case "c":
		m.prompt = promptCommit
		m.input = textinput.New()
		m.input.Placeholder = "commit message"
		m.input.Width = 50
		m.input.Focus()
		return m, textinput.Blink

	case "n":
		m.prompt = promptNewBranch
		m.input = textinput.New()
		m.input.Placeholder = "branch name"
		m.input.Width = 40
		m.input.Focus()
		return m, textinput.Blink

	case " ":
		switch m.focus {
		case focusFiles:
			if item, ok := m.files.SelectedItem().(fileItem); ok {
				if item.isDir {
					return m, m.toggleDirStage(item.path)
				}
				path := item.change.Path
				opKey := "stage:" + path
				if m.pendingFileOps[opKey] {
					// Previous stage/unstage on this path hasn't resolved yet.
					return m, nil
				}
				wasStaged := item.staged
				optimisticCmd := (&m).setFileStagedByPath(path, wasStaged, !wasStaged)
				(&m).setPendingFileOp(opKey, true)
				if wasStaged {
					return m, tea.Batch(
						optimisticCmd,
						func() tea.Msg { return setAppStatusMsg("Unstaging...") },
						unstageCmd(m.runner, path),
					)
				}
				return m, tea.Batch(
					optimisticCmd,
					func() tea.Msg { return setAppStatusMsg("Staging...") },
					stageCmd(m.runner, path),
				)
			}
		case focusBranches:
			// Checkout, matching lazygit's Branches-panel space key.
			if item, ok := m.branches.SelectedItem().(branchItem); ok {
				return m, tea.Batch(
					func() tea.Msg { return setAppStatusMsg("Checking out...") },
					switchBranchCmd(m.runner, item.branch.Name),
				)
			}
		case focusHistory:
			// Checkout, matching lazygit's Commits-panel space key: sync the
			// working state to the selected revision.
			if item, ok := m.history.SelectedItem().(revisionItem); ok && item.revision.Hash != "" {
				return m, tea.Batch(
					func() tea.Msg { return setAppStatusMsg("Checking out...") },
					syncToCmd(m.runner, item.revision.Hash, "checkout "+shortHash(item.revision.Hash)),
				)
			}
		}
		return m, nil

	case "d":
		switch m.focus {
		case focusFiles:
			if item, ok := m.files.SelectedItem().(fileItem); ok && !item.isDir {
				m.prompt = promptConfirmDiscard
				m.pendingDiscardPath = item.change.Path
			}
		case focusHistory:
			// Drop: lore has no rebase/history-rewrite, so this reverts
			// (a new revision undoing the change) rather than truly erasing
			// the commit - see lore.RevertRevision.
			if item, ok := m.history.SelectedItem().(revisionItem); ok && item.revision.Hash != "" {
				m.prompt = promptConfirmRevert
				m.pendingResetRevision = item.revision.Hash
				m.pendingResetLabel = "Revert revision " + shortHash(item.revision.Hash)
				m.pendingRevertMessage = revertCommitMessage(item.revision.Message)
			}
		}
		return m, nil

	case "D":
		if m.focus == focusFiles {
			m.prompt = promptConfirmDiscardAll
		}
		return m, nil

	case "e":
		if m.focus == focusFiles {
			if item, ok := m.files.SelectedItem().(fileItem); ok && !item.isDir {
				return m, editFileCmd(filepath.Join(m.repoRoot, item.change.Path))
			}
		}
		return m, nil

	case "enter":
		if m.focus == focusFiles {
			if item, ok := m.files.SelectedItem().(fileItem); ok {
				if item.isDir {
					return m, m.toggleDirCollapse(item.path)
				}
				lock, locked := m.locks[item.change.Path]
				return m, tea.Batch(
					func() tea.Msg { return setAppStatusMsg("Loading diff...") },
					loadDiffCmd(m.runner, item.change.Path, lock, locked),
				)
			}
		}
		return m, nil

	case "g":
		// Reset: move the current branch's latest pointer, matching lazygit's
		// Branches/Commits-panel "g" (ViewResetOptions) - lore's branch reset
		// only moves the pointer, so there's no hard/soft/mixed menu to show.
		switch m.focus {
		case focusBranches:
			if item, ok := m.branches.SelectedItem().(branchItem); ok && item.branch.Latest != "" {
				m.prompt = promptConfirmBranchReset
				m.pendingResetRevision = item.branch.Latest
				m.pendingResetLabel = "Reset current branch to " + item.branch.Name
			}
		case focusHistory:
			if item, ok := m.history.SelectedItem().(revisionItem); ok && item.revision.Hash != "" {
				m.prompt = promptConfirmBranchReset
				m.pendingResetRevision = item.revision.Hash
				m.pendingResetLabel = "Reset current branch to revision " + shortHash(item.revision.Hash)
			}
		}
		return m, nil

	case "L":
		if m.focus == focusFiles {
			if item, ok := m.files.SelectedItem().(fileItem); ok && !item.isDir {
				path := item.change.Path
				wasLocked := item.locked
				// Unlocking someone else's lock needs a confirm - it either
				// no-ops against a stock lore build (see LockReleaseForce)
				// or actually releases another person's lock against a
				// fork with the AdminUnlock capability, so it shouldn't
				// fire on a bare keypress the way toggling your own
				// lock/unlock does.
				if wasLocked && !item.lockedByMe {
					m.prompt = promptConfirmForceUnlock
					m.pendingForceUnlockPath = path
					return m, nil
				}
				opKey := "lock:" + path
				if m.pendingFileOps[opKey] {
					// Previous lock toggle on this path hasn't resolved yet.
					return m, nil
				}
				optimisticCmd := (&m).setFileLockedByPath(path, !wasLocked)
				(&m).setPendingFileOp(opKey, true)
				return m, tea.Batch(
					optimisticCmd,
					func() tea.Msg { return setAppStatusMsg("Updating lock...") },
					lockToggleCmd(m.runner, path, wasLocked),
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
	if mcmd := (&m).ensureMainContent(); mcmd != nil {
		cmd = tea.Batch(cmd, mcmd)
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

	if m.prompt == promptConfirmDiscardAll {
		m.prompt = promptNone
		if msg.String() == "y" {
			return m, tea.Batch(
				func() tea.Msg { return setAppStatusMsg("Discarding all changes...") },
				discardAllCmd(m.runner, changedPaths(m.status)),
			)
		}
		return m, nil
	}

	if m.prompt == promptConfirmBranchReset {
		m.prompt = promptNone
		revision := m.pendingResetRevision
		label := m.pendingResetLabel
		m.pendingResetRevision = ""
		m.pendingResetLabel = ""
		if msg.String() == "y" {
			return m, tea.Batch(
				func() tea.Msg { return setAppStatusMsg("Resetting...") },
				resetBranchCmd(m.runner, revision, label),
			)
		}
		return m, nil
	}

	if m.prompt == promptConfirmRevert {
		m.prompt = promptNone
		revision := m.pendingResetRevision
		label := m.pendingResetLabel
		message := m.pendingRevertMessage
		m.pendingResetRevision = ""
		m.pendingResetLabel = ""
		m.pendingRevertMessage = ""
		if msg.String() == "y" {
			return m, tea.Batch(
				func() tea.Msg { return setAppStatusMsg("Reverting...") },
				revertCmd(m.runner, revision, message, label),
			)
		}
		return m, nil
	}

	if m.prompt == promptConfirmForceUnlock {
		m.prompt = promptNone
		path := m.pendingForceUnlockPath
		m.pendingForceUnlockPath = ""
		if msg.String() == "y" {
			opKey := "lock:" + path
			if m.pendingFileOps[opKey] {
				return m, nil
			}
			optimisticCmd := (&m).setFileLockedByPath(path, false)
			(&m).setPendingFileOp(opKey, true)
			return m, tea.Batch(
				optimisticCmd,
				func() tea.Msg { return setAppStatusMsg("Force-unlocking...") },
				lockForceReleaseCmd(m.runner, path),
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
