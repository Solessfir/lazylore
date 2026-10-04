package ui

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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
	promptConfirmStageAllForCommit
	promptConfirmBranchMerge
)

func helpKeyMsg(binding string) tea.KeyMsg {
	types := map[string]tea.KeyType{
		" ": tea.KeySpace, "enter": tea.KeyEnter, "esc": tea.KeyEsc,
		"tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab,
		"up": tea.KeyUp, "down": tea.KeyDown,
	}
	if kind, ok := types[binding]; ok {
		return tea.KeyMsg{Type: kind}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(binding)}
}

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
			return m, tea.Batch(m.refreshBranchesList(), (&m).ensureMainContent())
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
		hasUnstaged, hasStaged := m.dirStageCounts("")
		if !hasUnstaged && !hasStaged {
			m.err = errors.New("No files staged")
			return m, nil
		}
		if !hasStaged {
			m.prompt = promptConfirmStageAllForCommit
			return m, nil
		}
		m.openCommitPrompt()
		return m, textinput.Blink

	case "p":
		return m, m.activityCmd("Pulling", pullCmd(m.runner))

	case "P":
		if m.pushInFlight {
			return m, nil
		}
		m.pushInFlight = true
		m.log.BeginLive("Push")
		return m, m.activityCmd("Pushing", pushStreamCmd(m.runner))

	case "n":
		m.prompt = promptNewBranch
		m.input = textinput.New()
		m.input.Prompt = ""
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
						m.activityCmd("Unstaging", unstageCmd(m.runner, path)),
					)
				}
				return m, tea.Batch(
					optimisticCmd,
					m.activityCmd("Staging", stageCmd(m.runner, path)),
				)
			}
		case focusBranches:
			// Checkout, matching lazygit's Branches-panel space key.
			if item, ok := m.branches.SelectedItem().(branchItem); ok {
				return m, m.activityCmd("Checking out", switchBranchCmd(m.runner, item.branch.Name))
			}
		case focusHistory:
			// Checkout, matching lazygit's Commits-panel space key: sync the
			// working state to the selected revision.
			if item, ok := m.history.SelectedItem().(revisionItem); ok && item.revision.Hash != "" {
				return m, m.activityCmd("Checking out", syncToCmd(m.runner, item.revision.Hash, "checkout "+shortHash(item.revision.Hash)))
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
				m.discardCursor = 0
				m.pendingDiscardPath = lorePath
				m.pendingDiscardIsDir = item.isDir
				m.pendingDiscardDirMixed = false
				m.pendingDiscardPaths = []string{lorePath}
				m.pendingDiscardUnstaged = nil
				if item.isDir {
					hasUnstaged, hasStaged := m.dirStageCounts(item.path)
					m.pendingDiscardDirMixed = hasUnstaged && hasStaged
					m.pendingDiscardPaths = nil
					for _, path := range changedFilePaths(m.status) {
						if dirPrefixMatches(item.path, path) {
							m.pendingDiscardPaths = append(m.pendingDiscardPaths, path)
						}
					}
					for _, change := range m.status.Unstaged {
						if !change.Directory && dirPrefixMatches(item.path, change.Path) {
							m.pendingDiscardUnstaged = append(m.pendingDiscardUnstaged, change.Path)
						}
					}
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
			m.pendingDiscardPaths = changedFilePaths(m.status)
		}
		return m, nil

	case "e":
		if m.focus == focusFiles {
			if item, ok := m.files.SelectedItem().(fileItem); ok && !item.isDir {
				return m, m.editActivityCmd(filepath.Join(m.repoRoot, item.change.Path))
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
				m.mainContentSource = focusFiles
				m.currentDiffPath = item.change.Path
				request := (&m).beginMainContentRequest(focusFiles, item.change.Path)
				return m, m.activityCmd("Loading diff", loadDiffCmd(m.runner, item.change.Path, lock, locked, request))
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

	case "M":
		// Merge selected branch into the current one, matching lazygit's
		// Branches-panel "M". lore auto-commits a clean merge; a conflicting
		// one surfaces as an error (see lore.MergeBranch) - lazylore has no
		// resolve/abort UI to fall into.
		if m.focus == focusBranches {
			if item, ok := m.branches.SelectedItem().(branchItem); ok && !item.branch.Current {
				m.prompt = promptConfirmBranchMerge
				m.pendingMergeBranch = item.branch.Name
				m.pendingMergeLabel = "Merge " + item.branch.Name + " into the current branch"
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
					m.activityCmd("Updating lock", lockToggleCmd(m.runner, path, wasLocked)),
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
	filterBefore := ""
	if lst := m.panelList(m.focus); lst != nil {
		filterBefore = lst.FilterValue()
		if msg.Type == tea.KeyEnter && lst.SettingFilter() && filterBefore != "" && len(lst.VisibleItems()) == 0 {
			lst.SetFilterState(list.FilterApplied)
			lst.FilterInput.Blur()
			return m, m.ensureMainContent()
		}
	}
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
	if lst := m.panelList(m.focus); lst != nil {
		if lst.FilterValue() != filterBefore {
			m.filterGenerations[m.focus]++
		}
		cmd = wrapFilterCommand(cmd, m.focus, m.filterGenerations[m.focus])
	}
	if mcmd := (&m).ensureMainContent(); mcmd != nil {
		cmd = tea.Batch(cmd, mcmd)
	}
	return m, cmd
}

func (m Model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.prompt == promptDiscardMenu {
		isDir := m.pendingDiscardIsDir
		mixed := m.pendingDiscardDirMixed
		key := msg.String()
		switch key {
		case "down", "j":
			m.discardCursor = min(2, m.discardCursor+1)
			return m, nil
		case "up", "k":
			m.discardCursor = max(0, m.discardCursor-1)
			return m, nil
		case "enter", " ":
			key = [3]string{"x", "u", "esc"}[m.discardCursor]
		}
		switch key {
		case "x":
			paths := m.pendingDiscardPaths
			m.prompt = promptNone
			m.pendingDiscardPath = ""
			m.pendingDiscardIsDir = false
			m.pendingDiscardDirMixed = false
			m.pendingDiscardPaths = nil
			m.pendingDiscardUnstaged = nil
			if len(paths) == 0 {
				return m, nil
			}
			return m, m.activityCmd("Discarding", discardAllCmd(m.runner, m.repoRoot, paths))
		case "u":
			if !isDir || !mixed {
				return m, nil // disabled - see renderPromptModal's tooltip
			}
			currentUnstaged := make(map[string]bool, len(m.status.Unstaged))
			for _, change := range m.status.Unstaged {
				if !change.Directory {
					currentUnstaged[change.Path] = true
				}
			}
			var unstagedPaths []string
			for _, path := range m.pendingDiscardUnstaged {
				if currentUnstaged[path] {
					unstagedPaths = append(unstagedPaths, path)
				}
			}
			m.prompt = promptNone
			m.pendingDiscardPath = ""
			m.pendingDiscardIsDir = false
			m.pendingDiscardDirMixed = false
			m.pendingDiscardPaths = nil
			m.pendingDiscardUnstaged = nil
			if len(unstagedPaths) == 0 {
				return m, nil
			}
			return m, m.activityCmd("Discarding unstaged changes", discardUnstagedInDirCmd(m.runner, m.repoRoot, unstagedPaths))
		case "esc", "n":
			m.prompt = promptNone
			m.pendingDiscardPath = ""
			m.pendingDiscardIsDir = false
			m.pendingDiscardDirMixed = false
			m.pendingDiscardPaths = nil
			m.pendingDiscardUnstaged = nil
		}
		return m, nil
	}

	confirmed := msg.String() == "enter" || msg.String() == "y"
	if m.prompt != promptCommit && m.prompt != promptNewBranch {
		switch msg.String() {
		case "enter", "y", "esc", "n":
		default:
			return m, nil
		}
	}

	if m.prompt == promptConfirmStageAllForCommit {
		m.prompt = promptNone
		if confirmed {
			m.pendingCommitAfterStageAll = true
			return m, m.toggleDirStage("")
		}
		return m, nil
	}

	if m.prompt == promptConfirmBranchMerge {
		m.prompt = promptNone
		branch := m.pendingMergeBranch
		label := m.pendingMergeLabel
		m.pendingMergeBranch = ""
		m.pendingMergeLabel = ""
		if confirmed {
			return m, m.activityCmd("Merging", mergeBranchCmd(m.runner, branch, label))
		}
		return m, nil
	}

	if m.prompt == promptConfirmDiscardAll {
		m.prompt = promptNone
		paths := m.pendingDiscardPaths
		m.pendingDiscardPaths = nil
		if confirmed && len(paths) > 0 {
			return m, m.activityCmd("Discarding all changes", discardAllCmd(m.runner, m.repoRoot, paths))
		}
		return m, nil
	}

	if m.prompt == promptConfirmBranchReset {
		m.prompt = promptNone
		revision := m.pendingResetRevision
		label := m.pendingResetLabel
		m.pendingResetRevision = ""
		m.pendingResetLabel = ""
		if confirmed {
			return m, m.activityCmd("Resetting", resetBranchCmd(m.runner, revision, label))
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
		if confirmed {
			return m, m.activityCmd("Reverting", revertCmd(m.runner, revision, message, label))
		}
		return m, nil
	}

	if m.prompt == promptConfirmForceUnlock {
		m.prompt = promptNone
		path := m.pendingForceUnlockPath
		m.pendingForceUnlockPath = ""
		if confirmed {
			opKey := "lock:" + path
			if m.pendingFileOps[opKey] {
				return m, nil
			}
			optimisticCmd := (&m).setFileLockedByPath(path, false)
			(&m).setPendingFileOp(opKey, true)
			return m, tea.Batch(
				optimisticCmd,
				m.activityCmd("Force-unlocking", lockForceReleaseCmd(m.runner, path)),
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
		if value == "" && (kind == promptCommit || kind == promptNewBranch) {
			// Keep the prompt open instead of silently closing it - an empty
			// commit message/branch name isn't a valid submission.
			return m, nil
		}
		m.prompt = promptNone
		m.input.Blur()
		switch kind {
		case promptCommit:
			return m, m.activityCmd("Committing", commitCmd(m.runner, value))
		case promptNewBranch:
			return m, m.activityCmd("Creating branch", createBranchCmd(m.runner, value))
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
