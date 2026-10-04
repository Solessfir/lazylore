package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazylore/internal/lore"
)

// Model is lazylore's root Bubble Tea model.
type Model struct {
	runner   lore.Runner
	repoName string
	repoRoot string // absolute repo root, for resolving file paths lore reports relative to it

	files    list.Model
	branches list.Model
	history  list.Model
	diff     diffModel
	log      commandLogModel

	localBranches      []lore.Branch
	remoteBranches     []lore.Branch
	showRemoteBranches bool // for the Branches panel tab (Local vs Remotes)

	activities             map[uint64]string
	nextActivityID         uint64
	activityFrame          int
	activityTickPending    bool
	activityTickGeneration uint64
	pushInFlight           bool

	// pendingCommitAfterStageAll: user confirmed "stage all and commit" from
	// the promptConfirmStageAllForCommit prompt - opens the commit input
	// once that stage-all lands (see actionDoneMsg handling), matching
	// lazygit's own promptToStageAllAndRetry.
	pendingCommitAfterStageAll bool
	currentDiffPath            string // last file path we issued a diff load for (avoids spamming loads on every cursor move)
	currentLogBranch           string // last branch we issued a Log load for (Branches panel focused)
	currentPatchRev            string // last revision we issued a Patch load for (History panel focused)
	mainContentRequestID       uint64
	lockRequestID              uint64
	refreshGeneration          uint64
	filterGenerations          [focusPanelCount]uint64

	// mainContentSource is which of Files/Branches/History last populated
	// the shared main panel, so mainPanelTitle() knows what's actually
	// showing there even once focus moves onto the main panel itself
	// (focusDiff), where m.focus alone no longer says which kind of content
	// (diff/log/patch) is currently loaded.
	mainContentSource focusPanel

	focus                  focusPanel
	prompt                 promptKind
	input                  textinput.Model
	pendingDiscardPath     string
	pendingDiscardPaths    []string
	pendingDiscardUnstaged []string
	pendingDiscardIsDir    bool   // true when `d` was pressed on a directory row
	pendingDiscardDirMixed bool   // only meaningful when pendingDiscardIsDir: true when the directory has both staged and unstaged files under it - the only case "discard unstaged" means anything (see discardUnstagedInDirCmd: lore staging is all-or-nothing per file, so a single file is never "mixed")
	pendingResetRevision   string // revision `g` (branch reset) will target once confirmed
	pendingResetLabel      string // human phrase for the confirm popup + command log, e.g. "Reset current branch to main"
	pendingRevertMessage   string // auto-commit message `d` (Drop/revert) will pass to lore, e.g. `Revert "oops"`
	pendingForceUnlockPath string // path `L` (unlock) will force-release once confirmed, when it's someone else's lock
	pendingMergeBranch     string // branch `M` will merge into the current one once confirmed
	pendingMergeLabel      string // human phrase for the confirm popup + command log, e.g. "Merge feature-x into main"
	selectMode             bool   // mouse capture dropped so the terminal can select text (mirrors lazyp4)
	showHelp               bool   // "?" keybindings popup (see modal.go), mirrors lazyp4's own help overlay
	helpRows               []helpRow
	helpCursor             int // index into helpRows of the selected row (never a section header)
	helpWidth, helpHeight  int // popup content size, computed once in openHelp

	// pendingFileOps guards optimistic-UI re-entrancy: "stage:"+path or
	// "lock:"+path while that path's background lore command is still in
	// flight. A second toggle on the same path while one is pending is a
	// no-op rather than firing a second overlapping lore call.
	pendingFileOps map[string]bool

	status        lore.Status
	revisions     []lore.Revision      // last-loaded History list; kept so statusMsg (which can arrive before or after historyMsg) can recompute unpushed coloring on its own
	collapsedDirs map[string]bool      // Files-panel tree: which directory paths are closed
	locks         map[string]lore.Lock // path -> lock, for files currently shown in the Files panel
	currentUserID string               // resolved lock owner; "" until loaded or if lookup fails
	err           error

	width, height int

	// Panel content dimensions computed by resize(), reused by View(). Each
	// *Height is the interior budget (after borders) for that panel.
	// The command log lives in its own fixed-height panel under Diff only.
	panelWidth     int
	filesHeight    int
	branchesHeight int
	historyHeight  int
	diffHeight     int

	// *ScrollOverride is the Files/Branches/History scroll window's top row,
	// set by mouse-wheel scrolling so the view can pan independently of the
	// selected row. -1 means no override: the window follows the cursor
	// (scrollWindowStart). Cleared back to -1 by any keyboard/click cursor
	// movement.
	filesScrollOverride    int
	branchesScrollOverride int
	historyScrollOverride  int
}

var selectedBg = lipgloss.Color("#292a2e")

// newListDelegate returns a list.ItemDelegate for a list's selected row.
// Only the panel that's actually focused gets the strong background fill;
// others get a subtler bold-only style, since each list keeps its own
// cursor regardless of app-level focus.
//
// width bounds the background fill to the panel's own content width -
// list.DefaultDelegate.Render never calls .Width() on the selected style
// itself, so without it the fill bleeds across the rest of the terminal
// row. The delegate bakes width in at construction time, so callers must
// rebuild it (via syncFocusDelegates) whenever that width changes.
func newListDelegate(focused bool, width int) list.ItemDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.NormalTitle = d.Styles.NormalTitle.UnsetForeground()
	d.Styles.NormalDesc = d.Styles.NormalDesc.UnsetForeground()
	d.Styles.DimmedTitle = d.Styles.DimmedTitle.UnsetForeground()
	d.Styles.DimmedDesc = d.Styles.DimmedDesc.UnsetForeground()
	d.Styles.SelectedTitle = d.Styles.NormalTitle.Inherit(cursorRowStyle(focused, width))
	d.Styles.SelectedDesc = d.Styles.NormalDesc.Inherit(cursorRowStyle(focused, width))
	return d
}

// newPanelList builds a list.Model with the given delegate and its own
// built-in title/status-bar/help/pagination chrome turned off - the panel
// border (drawn in View) already carries the title, keybindings live in
// the single global bar at the bottom of the screen instead of being
// repeated per panel, and with only ever one page of items visible at
// these panel sizes the pagination dots just take up a row for nothing.
func newPanelList(delegate list.ItemDelegate) list.Model {
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.Styles.FilterPrompt = l.Styles.FilterPrompt.UnsetForeground()
	l.Styles.FilterCursor = l.Styles.FilterCursor.UnsetForeground()
	l.Styles.NoItems = l.Styles.NoItems.UnsetForeground()
	l.FilterInput.PromptStyle = l.Styles.FilterPrompt
	l.FilterInput.Cursor.Style = l.Styles.FilterCursor
	return l
}

func (m Model) paneFocused(panel focusPanel) bool {
	return m.focus == panel && m.prompt == promptNone && !m.showHelp
}

// Lists retain their cursor while dialogs temporarily take focus.
func (m *Model) syncFocusDelegates() {
	m.files.SetDelegate(fileDelegate{focused: m.paneFocused(focusFiles)})
	m.branches.SetDelegate(compactTitleDelegate{focused: m.paneFocused(focusBranches), width: m.panelWidth})
	m.history.SetDelegate(compactTitleDelegate{focused: m.paneFocused(focusHistory), width: m.panelWidth})

	// Re-assert no chrome so "X of Y" count text never appears in bottom right of panels
	m.files.SetShowStatusBar(false)
	m.files.SetShowPagination(false)
	m.branches.SetShowStatusBar(false)
	m.branches.SetShowPagination(false)
	m.history.SetShowStatusBar(false)
	m.history.SetShowPagination(false)
}

// rowClickTarget maps a list panel's clicked row (relY, 0-based within the
// panel's content area) to the absolute item index it corresponds to, or
// ok=false if relY falls below the last real row on screen. cursor/total/
// perPage/scrollOverride must match what renderListWindow (items.go) was
// just called with - both derive from effectiveScrollStart, so render and
// click can never disagree about what's on screen.
func rowClickTarget(cursor, total, perPage, scrollOverride, relY int) (int, bool) {
	if perPage < 1 {
		perPage = 1
	}
	start := effectiveScrollStart(scrollOverride, cursor, total, perPage)
	itemsOnScreen := min(perPage, total-start)
	if relY < 0 || relY >= itemsOnScreen {
		return 0, false
	}
	return start + relY, true
}

// mouseLayout is the pixel geometry of every hit-testable panel for the
// current frame, shared by handleMouseClick and handleMouseWheel so they
// can never disagree about where a panel actually is on screen.
type mouseLayout struct {
	effFilesH, effBranchesH, effHistoryH, effDiffH int
	leftW                                          int
	statusH, filesH, branchesH, historyH, diffH    int
	mainH                                          int
	filesBoxTop, branchesBoxTop, historyBoxTop     int
	diffBoxLeft, diffBoxTop, diffAreaEnd           int
}

// computeMouseLayout mirrors View()'s own panel sizing (distributeSpace over
// the footer-shrink-adjusted "extra" area) so hit-testing matches what's
// actually on screen this frame - see resize()/View() for the canonical
// layout this is kept in sync with.
func (m Model) computeMouseLayout() mouseLayout {
	footerStr := m.currentFooter()
	footerLineCount := 0
	if footerStr != "" {
		footerLineCount = strings.Count(footerStr, "\n") + 1
	}
	actualBottom := footerLineCount + 1
	reservedBottom := footerHeight + keybindBarHeight
	extra := reservedBottom - actualBottom
	if extra < 0 {
		extra = 0
	}

	bodyHeight := max(0, m.height-footerHeight-keybindBarHeight)

	mainAvail := bodyHeight + extra
	leftOuters := distributeSpace([]layoutBox{
		{Size: statusPanelHeight},
		{Weight: 1},
		{Weight: 1},
		{Weight: 1},
	}, mainAvail)

	var l mouseLayout
	l.effFilesH = max(0, leftOuters[1]-borderHeight)
	l.effBranchesH = max(0, leftOuters[2]-borderHeight)
	l.effHistoryH = max(0, leftOuters[3]-borderHeight)
	l.effDiffH = m.diffHeight + extra

	l.leftW = m.panelWidth + borderWidth
	l.statusH = statusPanelHeight
	l.filesH = l.effFilesH + borderHeight
	l.branchesH = l.effBranchesH + borderHeight
	l.historyH = l.effHistoryH + borderHeight
	l.diffH = l.effDiffH + borderHeight
	l.mainH = l.statusH + l.filesH + l.branchesH + l.historyH

	l.filesBoxTop = l.statusH
	l.branchesBoxTop = l.filesBoxTop + l.filesH
	l.historyBoxTop = l.branchesBoxTop + l.branchesH

	l.diffBoxLeft = l.leftW
	l.diffBoxTop = 0
	l.diffAreaEnd = l.diffBoxTop + l.diffH
	return l
}

// handleMouseWheel pans whichever panel the mouse is hovering over -
// Files/Branches/History move their scroll window (the *ScrollOverride
// fields) without touching the selected row, Diff forwards to its own
// viewport - without requiring a click first or changing focus, matching
// lazygit's hover-to-scroll behavior (the wheel moves what's visible, not
// the cursor). A list only scrolls when it actually has more items than
// fit on screen ("room to scroll"); otherwise the wheel event is simply a
// no-op there.
func (m Model) handleMouseWheel(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseButtonWheelUp && msg.Button != tea.MouseButtonWheelDown {
		return m, nil
	}
	if m.prompt != promptNone || !m.layoutFits() {
		return m, nil
	}

	l := m.computeMouseLayout()
	x, y := msg.X, msg.Y
	if y >= l.mainH && x < l.leftW {
		return m, nil // footer / keybind area
	}

	delta := 1
	if msg.Button == tea.MouseButtonWheelUp {
		delta = -1
	}

	panList := func(lst list.Model, height int, override *int) {
		if lst.SettingFilter() {
			return
		}
		total := len(lst.VisibleItems())
		if total <= height {
			return // nothing to scroll
		}
		start := effectiveScrollStart(*override, lst.Index(), total, height)
		start += delta
		if start < 0 {
			start = 0
		}
		if max := total - height; start > max {
			start = max
		}
		*override = start
	}

	if x < l.leftW {
		switch {
		case y < l.filesBoxTop:
			// Status area has nothing to scroll.
		case y < l.branchesBoxTop:
			panList(m.files, l.effFilesH, &m.filesScrollOverride)
		case y < l.historyBoxTop:
			panList(m.branches, l.effBranchesH, &m.branchesScrollOverride)
		default:
			panList(m.history, l.effHistoryH, &m.historyScrollOverride)
		}
		return m, nil
	}

	// Right side: only the Diff viewport actually scrolls (Command Log has
	// no independent scroll position yet - see handleMouseClick's own note).
	// Nothing to do here at all while Command Log is focused/expanded -
	// Diff isn't even on screen then (see View()).
	if m.focus != focusCommandLog && y >= l.diffBoxTop+1 && y < l.diffAreaEnd-1 {
		relY := y - (l.diffBoxTop + 1)
		relX := x - (l.diffBoxLeft + 1)
		if relY >= 0 && relY < l.effDiffH {
			mm := tea.MouseMsg{X: relX, Y: relY, Button: msg.Button, Action: msg.Action}
			var cmd tea.Cmd
			m.diff.vp, cmd = m.diff.vp.Update(mm)
			return m, cmd
		}
	}
	return m, nil
}

// handleMouseClick handles left-clicks to focus panels and select items inside
// lists (Files, Branches, History), similar to lazygit mouse behavior.
// Diff viewport also receives mouse events for scrolling.
func (m Model) handleMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if m.prompt != promptNone || !m.layoutFits() {
		// A popup is covering the screen - clicks shouldn't reach the panels underneath.
		// (showHelp's own MouseMsg never reaches here - see Update.)
		return m, nil
	}

	l := m.computeMouseLayout()
	x, y := msg.X, msg.Y
	effFilesH, effBranchesH, effHistoryH, effDiffH := l.effFilesH, l.effBranchesH, l.effHistoryH, l.effDiffH

	leftW := l.leftW
	filesH, branchesH, historyH, diffH := l.filesH, l.branchesH, l.historyH, l.diffH
	mainH := l.mainH

	if y >= mainH {
		return m, nil // footer / keybind area
	}

	// Left column panels
	filesBoxTop, branchesBoxTop, historyBoxTop := l.filesBoxTop, l.branchesBoxTop, l.historyBoxTop

	newFocus := m.focus
	var cmd tea.Cmd

	if x < leftW {
		// Left side
		switch {
		case y < filesBoxTop:
			// Status area -> focus Status itself (jump key 1)
			newFocus = focusStatus
		case y < branchesBoxTop:
			newFocus = focusFiles
			// Click inside Files content: select the exact row under mouse.
			// (bubbles/list.Update ignores MouseMsg for cursor; it only reacts to keys.
			// We compute the target index in the visible list and Select it.)
			if !m.files.SettingFilter() && y >= filesBoxTop+1 && y < filesBoxTop+filesH-1 {
				relY := y - (filesBoxTop + 1)
				if relY >= 0 && relY < effFilesH {
					if target, ok := rowClickTarget(m.files.Index(), len(m.files.VisibleItems()), effFilesH, m.filesScrollOverride, relY); ok {
						m.files.Select(target)
						m.filesScrollOverride = -1
					}
				}
			}
		case y < historyBoxTop:
			newFocus = focusBranches
			if y == branchesBoxTop {
				if local, ok := dualTitleTabAtColumn("3", "Local branches", "Remotes", leftW, x); ok {
					m.showRemoteBranches = !local
					cmd = m.refreshBranchesList()
				}
				break
			}
			if !m.branches.SettingFilter() && y >= branchesBoxTop+1 && y < branchesBoxTop+branchesH-1 {
				relY := y - (branchesBoxTop + 1)
				if relY >= 0 && relY < effBranchesH {
					if target, ok := rowClickTarget(m.branches.Index(), len(m.branches.VisibleItems()), effBranchesH, m.branchesScrollOverride, relY); ok {
						m.branches.Select(target)
						m.branchesScrollOverride = -1
					}
				}
			}
		default:
			newFocus = focusHistory
			if !m.history.SettingFilter() && y >= historyBoxTop+1 && y < historyBoxTop+historyH-1 {
				relY := y - (historyBoxTop + 1)
				if relY >= 0 && relY < effHistoryH {
					if target, ok := rowClickTarget(m.history.Index(), len(m.history.VisibleItems()), effHistoryH, m.historyScrollOverride, relY); ok {
						m.history.Select(target)
						m.historyScrollOverride = -1
					}
				}
			}
		}
	} else if m.focus == focusCommandLog {
		// Command Log is currently focused, which expands it to fill the
		// entire right column (see View()) - there's no Diff area on
		// screen at all to hit-test, the whole side is Command Log.
		newFocus = focusCommandLog
	} else {
		// Right side: Diff (top) + Command Log (directly below it)
		diffBoxLeft := leftW
		diffBoxTop := 0
		diffAreaEnd := diffBoxTop + diffH
		if y >= diffAreaEnd {
			newFocus = focusCommandLog
		} else {
			newFocus = focusDiff
			if y >= diffBoxTop+1 && y < diffAreaEnd-1 {
				relY := y - (diffBoxTop + 1)
				relX := x - (diffBoxLeft + 1)
				if relY >= 0 && relY < effDiffH {
					mm := tea.MouseMsg{X: relX, Y: relY, Button: msg.Button, Action: msg.Action}
					m.diff.vp, cmd = m.diff.vp.Update(mm)
				}
			}
		}
	}

	if newFocus != m.focus {
		m.focus = newFocus
		m.syncFocusDelegates()
		(&m).recomputePanelHeights()
	}

	if mcmd := (&m).ensureMainContent(); mcmd != nil {
		cmd = tea.Batch(cmd, mcmd)
	}

	return m, cmd
}

// toggleDirCollapse flips the open/closed state of a Files-panel directory
// and rebuilds the list from the already-known Status - no need to re-fetch
// from lore, collapsing is purely a display concern.
func (m *Model) toggleDirCollapse(path string) tea.Cmd {
	if m.collapsedDirs == nil {
		m.collapsedDirs = map[string]bool{}
	}
	m.collapsedDirs[path] = !m.collapsedDirs[path]
	return m.rebuildFileItems()
}

// openHelp builds the "?" keybindings popup's row list for the currently
// focused panel. The popup's height is a fixed fraction of the terminal
// (like every other panel), not sized to the row count, scrolling instead
// of growing when there are more rows than fit.
func (m *Model) openHelp() {
	m.showHelp = true
	m.helpRows = m.buildHelpRows()
	keyColWidth := 0
	for _, r := range m.helpRows {
		if !r.section {
			keyColWidth = max(keyColWidth, lipgloss.Width(r.key))
		}
	}
	longest := 0
	for _, r := range m.helpRows {
		line := strings.Repeat(" ", keyColWidth+2) + fmt.Sprintf("── %s ──", r.key)
		if !r.section {
			line = fmt.Sprintf("%*s  %s", keyColWidth, r.key, r.desc)
		}
		if w := lipgloss.Width(line); w > longest {
			longest = w
		}
	}
	m.helpWidth = min(longest, max(20, m.width-8))
	m.helpHeight = max(3, (m.height-6)/2)
	m.helpCursor = firstSelectable(m.helpRows)
}

// logResult records an actionDoneMsg's outcome in the Command Log: a live-
// streamed action (see pushStreamCmd) already has its entry open from
// BeginLive and just needs FinishLive, everything else gets a normal
// AppendAction.
func (m *Model) logResult(msg actionDoneMsg) {
	if msg.liveStreamed {
		m.log.FinishLive(msg.commands, msg.err)
		return
	}
	m.log.AppendAction(msg.label, msg.commands, msg.err)
}

// setPendingFileOp marks (or clears) a "stage:"/"lock:" + path key as
// having a background command in flight, guarding optimistic-UI
// re-entrancy (see pendingFileOps).
func (m *Model) setPendingFileOp(key string, pending bool) {
	if m.pendingFileOps == nil {
		m.pendingFileOps = map[string]bool{}
	}
	if pending {
		m.refreshGeneration++
		m.pendingFileOps[key] = true
	} else {
		delete(m.pendingFileOps, key)
	}
}

func (m *Model) moveFileStatus(path string, fromStaged, toStaged bool) {
	if fromStaged == toStaged {
		return
	}

	source := &m.status.Unstaged
	destination := &m.status.Staged
	if fromStaged {
		source = &m.status.Staged
		destination = &m.status.Unstaged
	}

	for i, change := range *source {
		if change.Path != path || change.Directory {
			continue
		}

		*source = append((*source)[:i], (*source)[i+1:]...)
		*destination = append(*destination, change)
		return
	}
}

// setFileStagedByPath flips the Files-panel row for path currently at
// `from` staged-state to `to`, for optimistic UI: space toggles the color
// instantly, before the actual lore stage/unstage call (see
// stageCmd/unstageCmd) even starts. The row is identified by (path, staged)
// rather than path alone, since a path can appear as both a staged and an
// unstaged row simultaneously (see buildFileTree) - matching staged too
// picks the right one.
func (m *Model) setFileStagedByPath(path string, from, to bool) tea.Cmd {
	m.moveFileStatus(path, from, to)

	items := slices.Clone(m.files.Items())
	for i, it := range items {
		fi, ok := it.(fileItem)
		if !ok || fi.isDir || fi.change.Path != path || fi.staged != from {
			continue
		}
		fi.staged = to
		items[i] = fi
		break
	}
	m.refreshVisibleDirectoryStageState(items)
	return m.listItemsChanged(focusFiles, m.files.SetItems(items))
}

// setFileLockedByPath flips the lock badge on every Files-panel row for
// path (a path can appear as both a staged and an unstaged row - see
// buildFileTree - and a lore lock is a per-path property, not per-row, so
// both need updating together).
func (m *Model) setFileLockedByPath(path string, locked bool) tea.Cmd {
	// Ignore snapshots started before this optimistic lock change.
	m.lockRequestID++
	items := slices.Clone(m.files.Items())
	changed := false
	for i, it := range items {
		fi, ok := it.(fileItem)
		if !ok || fi.isDir || fi.change.Path != path {
			continue
		}
		fi.locked = locked
		items[i] = fi
		changed = true
	}
	if !changed {
		return nil
	}
	return m.listItemsChanged(focusFiles, m.files.SetItems(items))
}

// dirPrefixMatches reports whether filePath sits under dirPath (strictly
// nested inside it, never merely equal to it). dirPath == "" matches every
// path - the root "/" row covers the whole tree. Deliberately not an
// equality match too: a UE-style project can have a plain file and a
// directory sharing the exact same repo path segment (e.g. a file named
// "SonarV2" alongside a "SonarV2/" folder full of related assets) - treating
// filePath == dirPath as "inside" would wrongly sweep that unrelated
// namesake file into a folder-scoped stage/unstage/discard.
func dirPrefixMatches(dirPath, filePath string) bool {
	if dirPath == "" {
		return true
	}
	return strings.HasPrefix(filePath, dirPath+"/")
}

// changeIsInDirectory keeps a real directory change at dirPath in scope,
// while a namesake file at that exact path remains a sibling rather than a
// descendant. Synthetic directory rows have no corresponding FileChange.
func changeIsInDirectory(dirPath string, change lore.FileChange) bool {
	if dirPath == "" {
		return true
	}

	if change.Directory && change.Path == dirPath {
		return true
	}

	return dirPrefixMatches(dirPath, change.Path)
}

func (m *Model) moveDirectoryStatus(dirPath string, fromStaged, toStaged bool, onlyChanges map[lore.FileChange]bool) {
	if fromStaged == toStaged {
		return
	}

	source := &m.status.Unstaged
	destination := &m.status.Staged
	if fromStaged {
		source = &m.status.Staged
		destination = &m.status.Unstaged
	}

	kept := (*source)[:0]
	for _, change := range *source {
		if changeIsInDirectory(dirPath, change) && (onlyChanges == nil || onlyChanges[change]) {
			*destination = append(*destination, change)
			continue
		}

		kept = append(kept, change)
	}
	*source = kept
}

// dirStageCounts reports whether any file under dirPath is currently
// unstaged and/or staged, for toggleDirStage's lazygit-style decision
// (stage if anything's unstaged, else unstage).
func (m Model) dirStageCounts(dirPath string) (hasUnstaged, hasStaged bool) {
	for _, change := range m.status.Unstaged {
		if changeIsInDirectory(dirPath, change) {
			hasUnstaged = true
		}
	}

	for _, change := range m.status.Staged {
		if changeIsInDirectory(dirPath, change) {
			hasStaged = true
		}
	}

	return hasUnstaged, hasStaged
}

func (m *Model) refreshVisibleDirectoryStageState(items []list.Item) {
	for index, item := range items {
		file, ok := item.(fileItem)
		if !ok || !file.isDir {
			continue
		}

		hasUnstaged, hasStaged := m.dirStageCounts(file.path)
		file.allStaged = hasStaged && !hasUnstaged
		items[index] = file
	}
}

// setDirStagedByPrefix is setFileStagedByPath's recursive analog: flips
// every Files-panel row under dirPath (see dirPrefixMatches) currently
// staged as `from` over to `to`, for space on a directory/root row's
// optimistic UI (see toggleDirStage).
func (m *Model) setDirStagedByPrefix(dirPath string, from, to bool) tea.Cmd {
	m.moveDirectoryStatus(dirPath, from, to, nil)

	items := slices.Clone(m.files.Items())
	for i, it := range items {
		fi, ok := it.(fileItem)
		if !ok || !fi.trackedChange || fi.staged != from || !changeIsInDirectory(dirPath, fi.change) {
			continue
		}
		fi.staged = to
		items[i] = fi
	}

	// Recompute folders from authoritative status rather than visible rows;
	// collapsed directories deliberately omit their changed children.
	m.refreshVisibleDirectoryStageState(items)
	return m.listItemsChanged(focusFiles, m.files.SetItems(items))
}

// toggleDirStage handles space on a directory (or the root "/" row, path
// ""): if anything under it is unstaged, stage all of it; otherwise
// unstage everything staged; no-op if nothing has changed. `lore stage`/
// `unstage` on a directory path recurse over already-dirty files under it
// without needing --scan, so one call covers the whole subtree; "." stands
// in for the repo root since lore has no path for the synthetic "/" row.
// openCommitPrompt opens the commit-message input (see renderCommitModal).
func (m *Model) openCommitPrompt() {
	m.prompt = promptCommit
	m.input = textinput.New()
	m.input.Prompt = ""
	m.input.Width = 60
	m.input.Focus()
}

func (m *Model) toggleDirStage(dirPath string) tea.Cmd {
	hasUnstaged, hasStaged := m.dirStageCounts(dirPath)
	if !hasUnstaged && !hasStaged {
		return nil
	}

	lorePath := dirPath
	if lorePath == "" {
		lorePath = "."
	}
	opKey := "stage:" + lorePath
	if m.pendingFileOps[opKey] {
		return nil
	}
	m.setPendingFileOp(opKey, true)

	source := m.status.Staged
	if hasUnstaged {
		source = m.status.Unstaged
	}
	changes := make(map[lore.FileChange]bool)
	for _, change := range source {
		if changeIsInDirectory(dirPath, change) {
			changes[change] = true
		}
	}

	if hasUnstaged {
		optimisticCmd := m.setDirStagedByPrefix(dirPath, false, true)
		return tea.Batch(
			optimisticCmd,
			m.activityCmd("Staging", dirStageCmd(m.runner, dirPath, lorePath, changes)),
		)
	}

	optimisticCmd := m.setDirStagedByPrefix(dirPath, true, false)
	return tea.Batch(
		optimisticCmd,
		m.activityCmd("Unstaging", dirUnstageCmd(m.runner, dirPath, lorePath, changes)),
	)
}

func NewModel(r lore.Runner, repoName, repoRoot string) Model {
	m := Model{
		runner:         r,
		repoName:       repoName,
		repoRoot:       repoRoot,
		collapsedDirs:  map[string]bool{},
		pendingFileOps: map[string]bool{},
		activities:     map[uint64]string{},
		nextActivityID: 4,
		// focusFiles is the initial focus, below. width is 0 until the first
		// resize() - fine, syncFocusDelegates rebuilds these once real
		// dimensions are known.
		files:    newPanelList(fileDelegate{focused: true}),
		branches: newPanelList(compactTitleDelegate{focused: false, width: 0}),
		history:  newPanelList(compactTitleDelegate{focused: false, width: 0}),
		diff:     newDiffModel(0, 0),
		log:      newCommandLogModel(20),
		focus:    focusFiles,

		filesScrollOverride:    -1,
		branchesScrollOverride: -1,
		historyScrollOverride:  -1,
	}

	// Ensure no "X of Y" count text ever appears in the bottom-right of any panel
	m.files.SetShowStatusBar(false)
	m.files.SetShowPagination(false)
	m.branches.SetShowStatusBar(false)
	m.branches.SetShowPagination(false)
	m.history.SetShowStatusBar(false)
	m.history.SetShowPagination(false)

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		startActivityCmd(1, "Loading status", activityResultCmd(1, loadStatusCmd(m.runner, m.refreshGeneration))),
		startActivityCmd(2, "Loading branches", activityResultCmd(2, loadBranchesCmd(m.runner, m.refreshGeneration))),
		startActivityCmd(3, "Loading history", activityResultCmd(3, loadHistoryCmd(m.runner, m.refreshGeneration))),
		startActivityCmd(4, "Loading current user", activityResultCmd(4, loadCurrentUserCmd(m.runner))),
	)
}

func (m *Model) refreshCmd() tea.Cmd {
	m.refreshGeneration++
	return tea.Batch(
		m.activityCmd("Loading status", loadStatusCmd(m.runner, m.refreshGeneration)),
		m.activityCmd("Loading branches", loadBranchesCmd(m.runner, m.refreshGeneration)),
		m.activityCmd("Loading history", loadHistoryCmd(m.runner, m.refreshGeneration)),
	)
}

// footerHeight is the number of terminal rows reserved for the prompt /
// error line (directly above the global keybinding bar). The command log
// now lives in its own panel under the Diff (see commandLogPanelHeight).
// Used by resize() + View() for growing panels when this area is short.
const footerHeight = 3

// keybindBarHeight is the single always-visible row at the very bottom of
// the screen showing the global keybinding legend.
const keybindBarHeight = 1

// statusPanelHeight is the outer height (incl. top+bottom borders) of the
// compact status panel. With titled borders the title lives in the top
// border line itself (like lazygit), so only 1 content row is needed inside.
const statusPanelHeight = 3

// borderWidth/borderHeight are the space every bordered panel's lipgloss
// rounded border consumes. resize() uses them to size each panel's inner
// content; View() reuses borderWidth to size the Status panel, which has
// no bubbles widget of its own to size it automatically.
const (
	borderWidth  = 2
	borderHeight = 2
)

// commandLogPanelHeight is the full outer height (incl. borders) of the
// command log panel placed directly below the Diff on the right side only.
const commandLogPanelHeight = 10

// resize propagates the terminal size to every sub-widget: a Status panel
// plus stacked lists on the left (Files/Branches/History), the diff
// viewport + command log panel below it on the right, plus the prompt/error
// footer and global keybinding bar. Sizes go through distributeSpace (see
// boxlayout.go), not plain division, so the panels always sum to exactly
// the space available.
func (m *Model) resize() {
	// Reserve 1 column of slack instead of filling the terminal to its
	// exact last column - at an exact fit, a single-column mismatch
	// anywhere (terminal wrap quirks, a glyph rendering one column wider
	// than lipgloss counts it) cascades a visual shift through every panel below it.
	usableWidth := max(0, m.width-1)
	widths := distributeSpace([]layoutBox{{Weight: 1}, {Weight: 2}}, usableWidth)
	leftWidth := widths[0]
	rightWidth := widths[1]

	m.panelWidth = max(0, leftWidth-borderWidth)
	m.recomputePanelHeights()

	// Leave 1 column inside the panel for the scrollbar (drawn after content, before right border)
	diffInnerW := max(0, rightWidth-borderWidth)
	m.diff.vp.Width = max(0, diffInnerW-1)
	m.syncPanelSizes()

	// Branches/History's selected-row width is baked into their delegate
	// (see newListDelegate) rather than read live, so it has to be rebuilt
	// whenever panelWidth changes here, not just on focus changes.
	m.syncFocusDelegates()

	if m.showHelp {
		// Re-fit the keybindings popup to the new terminal size. Content is
		// deterministic from m.focus (unaffected by resize), so a full
		// rebuild is simplest - it does lose scroll position, an acceptable
		// cost for the rare case of resizing mid-popup.
		m.openHelp()
	}
}

// recomputePanelHeights calculates the base inner heights for all panels
// using distributeSpace, weighted evenly. This must be called on focus
// changes (in addition to resize) so that m.*Height bases are up to date for
// View() eff growth and mouse hit testing.
func (m *Model) recomputePanelHeights() {
	bodyHeight := max(0, m.height-footerHeight-keybindBarHeight)

	heights := distributeSpace([]layoutBox{
		{Size: statusPanelHeight}, // Status
		{Weight: 1},               // Files
		{Weight: 1},               // Branches
		{Weight: 1},               // History
	}, bodyHeight)

	m.filesHeight = max(0, heights[1]-borderHeight)
	m.branchesHeight = max(0, heights[2]-borderHeight)
	m.historyHeight = max(0, heights[3]-borderHeight)

	// Diff leaves room under itself for the command log (fixed, right only).
	cmdLogOuter := commandLogPanelHeight
	diffOuter := max(0, bodyHeight-cmdLogOuter)
	m.diffHeight = max(0, diffOuter-borderHeight)
}

func (m *Model) syncPanelSizes() {
	if m.width == 0 {
		return
	}
	l := m.computeMouseLayout()
	for _, panel := range []struct {
		items  *list.Model
		height int
	}{
		{items: &m.files, height: l.effFilesH},
		{items: &m.branches, height: l.effBranchesH},
		{items: &m.history, height: l.effHistoryH},
	} {
		panel.items.SetShowFilter(panel.items.SettingFilter())
		panel.items.SetSize(m.panelWidth, max(0, panel.height))
	}
	m.diff.vp.Height = max(0, l.effDiffH)
	m.diff.vp.SetYOffset(m.diff.vp.YOffset)
}

// rebuildFileItems recomputes the Files list's items (including lock/
// lockedByMe badges) from the model's last-known status/locks/currentUserID.
// Called from every handler that can independently learn one of those
// three, since they load independently and can arrive in any order.
func (m *Model) rebuildFileItems() tea.Cmd {
	items := statusToItems(m.status, m.collapsedDirs, m.locks, m.currentUserID)
	cmd := m.listItemsChanged(focusFiles, m.files.SetItems(items))
	m.files.SetShowStatusBar(false)
	m.files.SetShowPagination(false)
	return cmd
}

// rebuildHistoryItems recomputes the History list's items (including
// unpushed/pushed hash coloring) from the model's last-known revisions and
// status - called from both statusMsg and historyMsg handlers, since either
// can arrive first.
func (m *Model) rebuildHistoryItems() tea.Cmd {
	items := historyToItems(m.revisions, m.status.RemoteRevisionNumber, m.status.HasRemoteInfo)
	cmd := m.listItemsChanged(focusHistory, m.history.SetItems(items))
	m.history.SetShowStatusBar(false)
	m.history.SetShowPagination(false)
	return cmd
}

// mainPanelTitle returns the shared main panel's title for whatever's
// currently focused: Files shows "Unstaged changes"/"Staged changes" for
// the selected file, Branches shows "Log", History shows "Patch".
func (m Model) mainPanelTitle() string {
	switch m.mainContentSource {
	case focusBranches:
		return "Log"
	case focusHistory:
		return "Patch"
	default:
		if item, ok := m.files.SelectedItem().(fileItem); ok && !item.isDir {
			if item.staged {
				return "Staged changes"
			}
			return "Unstaged changes"
		}
		return "Diff"
	}
}

// refreshBranchesList rebuilds the Branches list from local/remote state and
// returns SetItems' cmd (non-nil when the panel has an active filter, so it
// must be propagated back through Update rather than dropped - otherwise a
// filtered Branches view goes stale after any refresh).
func (m *Model) refreshBranchesList() tea.Cmd {
	bs := m.localBranches
	if m.showRemoteBranches {
		bs = m.remoteBranches
	}
	cmd := m.listItemsChanged(focusBranches, m.branches.SetItems(branchesToItems(bs)))
	m.branches.SetShowStatusBar(false)
	m.branches.SetShowPagination(false)
	return cmd
}

// currentFooter is the spanning footer line above the keybind bar. Prompts
// and confirmations now render as centered popups (see modal.go) instead of
// living here, matching lazyp4 - this is left for the error line only. The
// command log itself lives in its own panel under Diff (like lazygit extras).
func (m Model) currentFooter() string {
	if m.err != nil {
		return errorStyle.Render(m.err.Error())
	}
	return ""
}

func (m *Model) panelList(source focusPanel) *list.Model {
	switch source {
	case focusFiles:
		return &m.files
	case focusBranches:
		return &m.branches
	case focusHistory:
		return &m.history
	default:
		return nil
	}
}

func (m *Model) listItemsChanged(source focusPanel, cmd tea.Cmd) tea.Cmd {
	if items := m.panelList(source); items != nil && items.FilterState() == list.Unfiltered {
		clampListSelection(items)
	}
	m.filterGenerations[source]++
	return wrapFilterCommand(cmd, source, m.filterGenerations[source])
}

func clampListSelection(items *list.Model) {
	last := max(0, len(items.VisibleItems())-1)
	if index := items.Index(); index < 0 || index > last {
		items.Select(min(last, max(0, index)))
	}
}

// Filter commands may be nested in batches alongside cursor commands.
func wrapFilterCommand(cmd tea.Cmd, source focusPanel, generation uint64) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		switch msg := cmd().(type) {
		case list.FilterMatchesMsg:
			return filterMatchesMsg{source: source, generation: generation, matches: msg}
		case tea.BatchMsg:
			wrapped := make(tea.BatchMsg, len(msg))
			for i, sub := range msg {
				wrapped[i] = wrapFilterCommand(sub, source, generation)
			}
			return wrapped
		default:
			return msg
		}
	}
}

func (m *Model) beginMainContentRequest(source focusPanel, target string) mainContentRequest {
	m.mainContentRequestID++
	return mainContentRequest{id: m.mainContentRequestID, source: source, target: target}
}

func (m *Model) clearMainContentSelection(source focusPanel) {
	m.mainContentRequestID++

	switch source {
	case focusFiles:
		m.currentDiffPath = ""
	case focusBranches:
		m.currentLogBranch = ""
	case focusHistory:
		m.currentPatchRev = ""
	}

	m.diff.SetContentRaw("")
}

// ensureMainContent returns a command to (re)load the shared main panel's
// content for whatever is currently focused/selected, matching lazygit's
// own contextual main view: Files shows the selected file's diff, Branches
// shows the selected branch's Log, History shows the selected revision's
// Patch. Skips when the relevant selection hasn't actually changed (avoids
// a load/flash on every cursor move that lands on the same target), and
// when the focused panel has no meaningful content selected (a directory
// in Files, an empty list).
func (m *Model) ensureMainContent() tea.Cmd {
	// Each source remembers its selection, but they all share one content buffer.
	sourceChanged := m.mainContentSource != m.focus
	// Status/Command Log have no main-panel content of their own (matches
	// lazygit - they're not contexts that drive the main view), so
	// focusing either must not overwrite what mainPanelTitle/the diff
	// panel still shows.
	if m.focus != focusDiff && m.focus != focusStatus && m.focus != focusCommandLog {
		m.mainContentSource = m.focus
	}
	switch m.focus {
	case focusFiles:
		item, ok := m.files.SelectedItem().(fileItem)
		if !ok {
			m.clearMainContentSelection(focusFiles)
			return nil
		}
		if item.isDir {
			// Selecting a directory clears the main panel instead of leaving
			// the last-selected file's diff stuck on screen.
			m.clearMainContentSelection(focusFiles)
			return nil
		}
		if !sourceChanged && item.change.Path == m.currentDiffPath {
			return nil
		}
		m.currentDiffPath = item.change.Path
		lock, locked := m.locks[item.change.Path]
		request := m.beginMainContentRequest(focusFiles, item.change.Path)
		return m.activityCmd("Loading diff", loadDiffCmd(m.runner, item.change.Path, lock, locked, request))

	case focusBranches:
		item, ok := m.branches.SelectedItem().(branchItem)
		if !ok {
			m.clearMainContentSelection(focusBranches)
			return nil
		}
		if !sourceChanged && item.branch.Name == m.currentLogBranch {
			return nil
		}
		m.currentLogBranch = item.branch.Name
		request := m.beginMainContentRequest(focusBranches, item.branch.Name)
		return m.activityCmd("Loading branch log", loadBranchLogCmd(m.runner, item.branch.Name, request))

	case focusHistory:
		item, ok := m.history.SelectedItem().(revisionItem)
		if !ok {
			m.clearMainContentSelection(focusHistory)
			return nil
		}
		if !sourceChanged && item.revision.Hash == m.currentPatchRev {
			return nil
		}
		m.currentPatchRev = item.revision.Hash
		parent := item.revision.Parent
		if lore.IsZeroHash(parent) {
			parent = ""
		}
		request := m.beginMainContentRequest(focusHistory, item.revision.Hash)
		cmd := loadRevisionPatchCmd(m.runner, parent, item.revision.Hash, request)
		if parent == "" {
			return cmd
		}
		return m.activityCmd("Loading revision patch", cmd)
	}
	return nil
}

// changedFilePaths flattens a Status's staged and unstaged file entries for
// lock-status requests. Lore reports directory changes too, but locks apply
// only to files.
func changedFilePaths(s lore.Status) []string {
	paths := make([]string, 0, len(s.Staged)+len(s.Unstaged))
	for _, c := range s.Staged {
		if !c.Directory {
			paths = append(paths, c.Path)
		}
	}
	for _, c := range s.Unstaged {
		if !c.Directory {
			paths = append(paths, c.Path)
		}
	}
	return paths
}

func (m Model) Update(msg tea.Msg) (updated tea.Model, cmd tea.Cmd) {
	defer func() {
		if next, ok := updated.(Model); ok {
			next.syncPanelSizes()
			next.syncFocusDelegates()
			if tick := next.activityTickCmd(); tick != nil {
				cmd = tea.Batch(cmd, tick)
			}
			updated = next
		}
	}()
	m.syncPanelSizes()
	var activityID uint64
	if result, ok := msg.(activityResultMsg); ok {
		activityID = result.id
		msg = result.inner
		push, streamed := msg.(pushChanMsg)
		_, progress := push.inner.(pushLineMsg)
		if !streamed || !progress {
			delete(m.activities, result.id)
		}
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case filterMatchesMsg:
		lst := m.panelList(msg.source)
		if lst == nil || msg.generation != m.filterGenerations[msg.source] {
			return m, nil
		}
		updated, cmd := lst.Update(msg.matches)
		*lst = updated
		// Bubbles applies matches without updating pagination.
		lst.SetSize(lst.Width(), lst.Height())
		clampListSelection(lst)
		return m, tea.Batch(wrapFilterCommand(cmd, msg.source, msg.generation), m.ensureMainContent())

	case activityStartMsg:
		if m.activities == nil {
			m.activities = map[uint64]string{}
		}
		m.activities[msg.id] = msg.name
		return m, msg.cmd

	case activityTickMsg:
		if msg.generation != m.activityTickGeneration || !m.activityTickPending {
			return m, nil
		}
		m.activityTickPending = false
		if len(m.activities) > 0 {
			m.activityFrame = (m.activityFrame + 1) % 4
		}
		return m, nil

	case tea.MouseMsg:
		if !m.layoutFits() {
			return m, nil
		}
		if m.showHelp {
			// Mouse wheel moves the selection like j/k; other clicks
			// harmlessly no-op instead of reaching panels underneath.
			switch msg.Button {
			case tea.MouseButtonWheelDown:
				m.helpCursor = nextSelectable(m.helpRows, m.helpCursor)
			case tea.MouseButtonWheelUp:
				m.helpCursor = prevSelectable(m.helpRows, m.helpCursor)
			}
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			return m.handleMouseWheel(msg)
		}
		return m.handleMouseClick(msg)

	case tea.KeyMsg:
		if m.width > 0 && !m.layoutFits() {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.selectMode {
			// Any key exits select mode and restores mouse capture.
			m.selectMode = false
			return m, tea.EnableMouseCellMotion
		}
		if m.showHelp {
			switch msg.String() {
			case "esc", "?":
				m.showHelp = false
			case "j", "down":
				m.helpCursor = nextSelectable(m.helpRows, m.helpCursor)
			case "k", "up":
				m.helpCursor = prevSelectable(m.helpRows, m.helpCursor)
			}
			// Any other key is a harmless no-op instead of closing the popup.
			return m, nil
		}
		if m.prompt != promptNone {
			return m.handlePromptKey(msg)
		}
		if lst := m.panelList(m.focus); lst != nil {
			// Filter commands retain the input's rune slice across updates.
			lst.FilterInput.SetValue(lst.FilterValue())
		}
		return m.handleKey(msg)

	case statusMsg:
		if msg.generation != m.refreshGeneration {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.status = msg.status
		// Force a diff refresh for the current selection because the
		// working tree (or staged state) may have changed.
		m.currentDiffPath = ""
		setCmd := (&m).rebuildFileItems()
		diffCmd := (&m).ensureMainContent()
		m.lockRequestID++
		paths := changedFilePaths(msg.status)
		lockCmd := loadLocksCmd(m.runner, paths, m.lockRequestID)
		if len(paths) > 0 {
			lockCmd = m.activityCmd("Loading locks", lockCmd)
		}
		// statusMsg and historyMsg load independently and can arrive in
		// either order; rebuild History's unpushed coloring here too so it's
		// correct even when status lands after history already rendered.
		historyCmd := (&m).rebuildHistoryItems()
		return m, tea.Batch(setCmd, diffCmd, lockCmd, historyCmd)

	case branchesMsg:
		if msg.generation != m.refreshGeneration {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.localBranches = nil
		m.remoteBranches = nil
		for _, b := range msg.branches {
			if b.Remote {
				m.remoteBranches = append(m.remoteBranches, b)
			} else {
				m.localBranches = append(m.localBranches, b)
			}
		}
		cmd := m.refreshBranchesList()
		return m, tea.Batch(cmd, (&m).ensureMainContent())

	case historyMsg:
		if msg.generation != m.refreshGeneration {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.revisions = msg.revisions
		cmd := (&m).rebuildHistoryItems()
		return m, tea.Batch(cmd, (&m).ensureMainContent())

	case locksMsg:
		if msg.requestID != m.lockRequestID {
			return m, nil
		}
		// Best-effort: locks require an online remote (see internal/lore/lock.go),
		// so a failure here (e.g. offline) shouldn't raise the main error banner
		// on every refresh - the Files panel just shows no lock badges.
		if msg.err != nil {
			return m, nil
		}
		locks := make(map[string]lore.Lock, len(msg.locks))
		for _, l := range msg.locks {
			locks[l.Path] = l
		}
		m.locks = locks
		return m, (&m).rebuildFileItems()

	case currentUserMsg:
		// Best-effort, same as locks: not authenticated just means every
		// lock shows as someone else's rather than "locked by me".
		if msg.err != nil {
			return m, nil
		}
		m.currentUserID = msg.id
		return m, (&m).rebuildFileItems()

	case diffMsg:
		if msg.request.id != m.mainContentRequestID {
			return m, nil
		}
		if msg.err != nil {
			switch msg.request.source {
			case focusFiles:
				if m.currentDiffPath == msg.request.target {
					m.currentDiffPath = ""
				}
			case focusBranches:
				if m.currentLogBranch == msg.request.target {
					m.currentLogBranch = ""
				}
			case focusHistory:
				if m.currentPatchRev == msg.request.target {
					m.currentPatchRev = ""
				}
			}

			m.err = msg.err
			return m, nil
		}
		m.err = nil
		if msg.raw {
			m.diff.SetContentRaw(msg.text)
		} else {
			m.diff.SetContent(msg.text)
		}
		return m, nil

	case pushChanMsg:
		switch inner := msg.inner.(type) {
		case pushLineMsg:
			m.log.AppendLiveLine(string(inner))
			read := readPushChan(msg.ch)
			if activityID != 0 {
				read = activityResultCmd(activityID, read)
			}
			return m, read
		case actionDoneMsg:
			return m.Update(inner)
		}
		return m, nil

	case actionDoneMsg:
		if strings.HasPrefix(msg.opKey, "lock:") {
			m.lockRequestID++
		}
		if msg.liveStreamed {
			m.pushInFlight = false
		}
		if msg.opKey != "" {
			(&m).setPendingFileOp(msg.opKey, false)
		}
		commitAfterStageAll := m.pendingCommitAfterStageAll && msg.opKey == "stage:."
		if commitAfterStageAll {
			m.pendingCommitAfterStageAll = false
		}
		if msg.err != nil {
			var revertCmd tea.Cmd
			if msg.revert != nil {
				msg.revert(&m)
				revertCmd = m.rebuildFileItems()
			}
			// Command Log already shows this error (logResult below) - don't
			// also set m.err, or it'd duplicate into the footer via
			// currentFooter().
			(&m).logResult(msg)
			if len(m.pendingFileOps) == 0 {
				return m, tea.Batch(revertCmd, m.refreshCmd())
			}
			return m, revertCmd
		}
		if msg.confirm != nil {
			msg.confirm(&m)
		}
		if commitAfterStageAll && m.prompt == promptNone {
			(&m).openCommitPrompt()
			(&m).logResult(msg)
			return m, tea.Batch(m.refreshCmd(), textinput.Blink)
		}
		(&m).logResult(msg)
		if len(m.pendingFileOps) > 0 {
			// Another file op (e.g. staging one file, then the whole folder
			// before the first call returned) is still in flight. Refreshing
			// now would fetch server state from before that other op has
			// landed, overwriting its still-correct optimistic UI with stale
			// data - a visible flicker back to the pre-optimistic state until
			// the next refresh corrects it again. Skip it here; whichever op
			// finishes last (when pendingFileOps is finally empty) triggers
			// the one refresh that reflects everything.
			return m, nil
		}
		return m, m.refreshCmd()

	case editorDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		return m, m.refreshCmd()
	}
	return m, nil
}
