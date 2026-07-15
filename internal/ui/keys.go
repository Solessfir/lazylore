package ui

import (
	"path/filepath"
	"strings"
	"time"

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
	focusCommandLog
)

// focusPanelCount is the number of cyclable panels (Tab/Shift+Tab wrap
// through all of them, including Command Log).
const focusPanelCount = 6

type promptKind int

const (
	promptNone promptKind = iota
	promptCommit
	promptNewBranch
	promptConfirmDiscardAll
	promptDiscardMenu
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
		m.focus = (m.focus + 1) % focusPanelCount
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, (&m).ensureMainContent()

	case "shift+tab", "h":
		m.focus = (m.focus + focusPanelCount - 1) % focusPanelCount
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, (&m).ensureMainContent()

	// Panel jump keys: 1=Status, 2=Files, 3=Branches, 4=History, 5=Diff, 6=Command Log.
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
	case "6":
		m.focus = focusCommandLog
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
		return m, nil

	// Cycle the focused panel's sub-tabs - only Branches has more than one
	// (Local/Remotes), so this no-ops elsewhere.
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

	case "p":
		m.syncGen++
		gen := m.syncGen
		return m, tea.Batch(
			tea.Tick(statusRevealDelay, func(time.Time) tea.Msg { return revealSyncMsg{gen: gen, label: "Pulling"} }),
			pullCmd(m.runner),
		)

	case "P":
		m.syncGen++
		gen := m.syncGen
		return m, tea.Batch(
			tea.Tick(statusRevealDelay, func(time.Time) tea.Msg { return revealSyncMsg{gen: gen, label: "Pushing"} }),
			pushCmd(m.runner),
		)

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
			// Same menu for a file or a directory row, no separate key for folders.
			if item, ok := m.files.SelectedItem().(fileItem); ok {
				lorePath := item.path
				if lorePath == "" {
					lorePath = "." // synthetic "/" root row - see toggleDirStage
				}
				m.prompt = promptDiscardMenu
				m.pendingDiscardPath = lorePath
				m.pendingDiscardIsDir = item.isDir
				m.pendingDiscardDirMixed = false
				if item.isDir {
					hasUnstaged, hasStaged := m.dirStageCounts(item.path)
					m.pendingDiscardDirMixed = hasUnstaged && hasStaged
				}
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
				// Unlocking someone else's lock needs a confirm - it can
				// actually release another person's lock, unlike toggling
				// your own lock/unlock.
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
		m.filesScrollOverride = -1 // keyboard nav always snaps the view back to following the cursor
	case focusBranches:
		m.branches, cmd = m.branches.Update(msg)
		m.branchesScrollOverride = -1
	case focusHistory:
		m.history, cmd = m.history.Update(msg)
		m.historyScrollOverride = -1
	case focusDiff:
		m.diff.vp, cmd = m.diff.vp.Update(msg)
	}
	if mcmd := (&m).ensureMainContent(); mcmd != nil {
		cmd = tea.Batch(cmd, mcmd)
	}
	return m, cmd
}

func (m Model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.prompt == promptDiscardMenu {
		path := m.pendingDiscardPath
		isDir := m.pendingDiscardIsDir
		mixed := m.pendingDiscardDirMixed
		switch msg.String() {
		case "x":
			m.prompt = promptNone
			m.pendingDiscardPath = ""
			m.pendingDiscardIsDir = false
			m.pendingDiscardDirMixed = false
			return m, tea.Batch(
				func() tea.Msg { return setAppStatusMsg("Discarding...") },
				discardAllCmd(m.runner, []string{path}),
			)
		case "u":
			if !isDir || !mixed {
				return m, nil // disabled - see renderPromptModal's tooltip
			}
			dirPath := path
			if dirPath == "." {
				dirPath = ""
			}
			var unstagedPaths []string
			for _, fc := range m.status.Unstaged {
				if dirPrefixMatches(dirPath, fc.Path) {
					unstagedPaths = append(unstagedPaths, fc.Path)
				}
			}
			m.prompt = promptNone
			m.pendingDiscardPath = ""
			m.pendingDiscardIsDir = false
			m.pendingDiscardDirMixed = false
			return m, tea.Batch(
				func() tea.Msg { return setAppStatusMsg("Discarding unstaged changes...") },
				discardUnstagedInDirCmd(m.runner, unstagedPaths),
			)
		case "esc", "n":
			m.prompt = promptNone
			m.pendingDiscardPath = ""
			m.pendingDiscardIsDir = false
			m.pendingDiscardDirMixed = false
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
