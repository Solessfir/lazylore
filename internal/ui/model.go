package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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

	appStatus string
	spinner   int
	// statusGen is bumped every time appStatus is set or cleared. A delayed
	// reveal (see statusRevealDelay) captures the generation it was
	// scheduled under; if that no longer matches by the time the delay
	// elapses, the action finished before it was ever worth showing a
	// spinner for, and the reveal is dropped.
	statusGen int

	filesTotal    int
	branchesTotal int
	historyTotal  int

	currentDiffPath  string // last file path we issued a diff load for (avoids spamming loads on every cursor move)
	currentLogBranch string // last branch we issued a Log load for (Branches panel focused)
	currentPatchRev  string // last revision we issued a Patch load for (History panel focused)

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
	pendingDiscardIsDir    bool           // true when `d` was pressed on a directory row
	pendingDiscardDirMixed bool           // only meaningful when pendingDiscardIsDir: true when the directory has both staged and unstaged files under it - the only case "discard unstaged" means anything (see discardUnstagedInDirCmd: lore staging is all-or-nothing per file, so a single file is never "mixed")
	pendingResetRevision   string         // revision `g` (branch reset) will target once confirmed
	pendingResetLabel      string         // human phrase for the confirm popup + command log, e.g. "Reset current branch to main"
	pendingRevertMessage   string         // auto-commit message `d` (Drop/revert) will pass to lore, e.g. `Revert "oops"`
	pendingForceUnlockPath string         // path `L` (unlock) will force-release once confirmed, when it's someone else's lock
	selectMode             bool           // mouse capture dropped so the terminal can select text (mirrors lazyp4)
	showHelp               bool           // "?" keybindings popup (see modal.go), mirrors lazyp4's own help overlay
	helpViewport           viewport.Model // scrolls the keybindings popup's body (j/k/arrows/mouse wheel via its own default keymap)

	// pendingFileOps guards optimistic-UI re-entrancy: "stage:"+path or
	// "lock:"+path while that path's background lore command is still in
	// flight. A second toggle on the same path while one is pending is a
	// no-op rather than firing a second overlapping lore call.
	pendingFileOps map[string]bool

	status        lore.Status
	revisions     []lore.Revision      // last-loaded History list; kept so statusMsg (which can arrive before or after historyMsg) can recompute unpushed coloring on its own
	collapsedDirs map[string]bool      // Files-panel tree: which directory paths are closed
	locks         map[string]lore.Lock // path -> lock, for files currently shown in the Files panel
	currentUserID string               // this session's identity (lore.CurrentUserID), for locked-by-me coloring; "" until loaded or if unauthenticated
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
	// set by mouse-wheel scrolling (see handleMouseWheel) so the view can
	// pan independently of the selected row - matching lazygit, where the
	// wheel moves what's visible without moving the cursor. -1 means no
	// override: the window just follows the cursor (scrollWindowStart).
	// Cleared back to -1 by any keyboard/click cursor movement, so the view
	// snaps back to following the selection the moment the user interacts
	// with it that way again.
	filesScrollOverride    int
	branchesScrollOverride int
	historyScrollOverride  int
}

// Colors match lazygit's actual default theme (pkg/config/user_config.go):
// ActiveBorderColor "green bold", SelectedLineBgColor "blue" (a background
// fill, not just a foreground change - that's what makes lazygit's
// selected row unmistakable regardless of terminal color profile).
// Basic 16-color ANSI codes only (0-15), since those are the one palette
// every terminal renders correctly - no 256-color/TrueColor detection to
// get wrong.
var (
	selectedBg = lipgloss.Color("4")  // blue, matches lazygit's SelectedLineBgColor
	selectedFg = lipgloss.Color("15") // bright white, for contrast against the blue fill
)

// newListDelegate returns a list.ItemDelegate for a list's selected row.
// Each list keeps its own cursor position regardless of which panel our
// own app-level focus is on, so without this distinction every list would
// show a highlighted row at once - only the panel that's actually focused
// should get the strong background fill; the others get a much subtler
// style, matching lazygit's own InactiveViewSelectedLineBgColor: "bold"
// (no background fill at all for unfocused panels).
//
// width bounds the background fill to the panel's own content width.
// list.DefaultDelegate.Render (bubbles' own code, not ours) never calls
// .Width() on the selected style itself, so without setting it here the
// fill isn't bounded to this panel at all - it bleeds across the rest of
// the terminal row, past the panel's own border. Because the delegate
// bakes width in at construction time rather than reading it live, callers
// must rebuild it (via syncFocusDelegates) whenever that width changes,
// not just when focus changes - resize() does both.
func newListDelegate(focused bool, width int) list.ItemDelegate {
	d := list.NewDefaultDelegate()
	if focused {
		d.Styles.SelectedTitle = d.Styles.SelectedTitle.
			Background(selectedBg).
			Foreground(selectedFg).
			Bold(true).
			Width(width)
		d.Styles.SelectedDesc = d.Styles.SelectedDesc.
			Background(selectedBg).
			Foreground(selectedFg).
			Width(width)
	} else {
		d.Styles.SelectedTitle = d.Styles.SelectedTitle.Bold(true)
	}
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
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	return l
}

// syncFocusDelegates re-applies the focused/unfocused delegate to each of
// the Files/Branches/History lists to match m.focus, called whenever focus
// changes. list.Model has no "am I focused" concept of its own - that's
// this app's, so the delegate has to be pushed in from outside.
//
// For Branches and History we now use compact single-line delegates (no
// description rows) to more closely match lazygit row density.
func (m *Model) syncFocusDelegates() {
	m.files.SetDelegate(fileDelegate{focused: m.focus == focusFiles})
	m.branches.SetDelegate(compactTitleDelegate{focused: m.focus == focusBranches, width: m.panelWidth})
	m.history.SetDelegate(compactTitleDelegate{focused: m.focus == focusHistory, width: m.panelWidth})

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
// ok=false if relY falls below the last real row on screen.
//
// cursor/total/perPage/scrollOverride must match what renderListWindow (see
// items.go) was just called with, so the two can never disagree about
// what's on screen - effectiveScrollStart is the single source of truth
// both derive from, rather than each independently computing (or, as
// before this switched away from bubbles/list.Model's own pagination,
// reading) a page boundary that could drift out of sync between the
// render and the click.
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
	if m.prompt != promptNone {
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
	if m.prompt != promptNone {
		// A popup is covering the screen - clicks shouldn't reach the panels underneath.
		// (showHelp's own MouseMsg never reaches here - see Update.)
		return m, nil
	}

	l := m.computeMouseLayout()
	x, y := msg.X, msg.Y
	effFilesH, effBranchesH, effHistoryH, effDiffH := l.effFilesH, l.effBranchesH, l.effHistoryH, l.effDiffH

	// list.Model.Select (called below) stores the target index as
	// Page*Paginator.PerPage+cursor internally, so it needs an accurate
	// PerPage for that round-trip to come back out right on the next
	// Index() read - resync each list's PerPage to the height actually
	// rendered THIS frame (a footer-shrink-adjusted height View() computes
	// fresh every render, via a value receiver, so it never persists back
	// to the real model on its own) before doing any click math.
	m.files.SetSize(m.panelWidth, effFilesH)
	m.branches.SetSize(m.panelWidth, effBranchesH)
	m.history.SetSize(m.panelWidth, effHistoryH)

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
			if y >= filesBoxTop+1 && y < filesBoxTop+filesH-1 {
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
				// Click on the title bar (tab area) -> switch Local <-> Remotes
				// Rough split: left side of title area -> Local, right -> Remotes
				// This makes the "Remotes" part of the title clickable.
				if x > leftW/2 {
					m.showRemoteBranches = true
				} else {
					m.showRemoteBranches = false
				}
				cmd = m.refreshBranchesList()
				break
			}
			if y >= branchesBoxTop+1 && y < branchesBoxTop+branchesH-1 {
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
			if y >= historyBoxTop+1 && y < historyBoxTop+historyH-1 {
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

// openHelp builds the "?" keybindings popup's content for the currently
// focused panel and sizes its viewport to fit the content (capped to the
// terminal, so it never overflows on a small window).
func (m *Model) openHelp() {
	m.showHelp = true
	content := m.helpContent()
	lines := strings.Split(content, "\n")
	longest := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > longest {
			longest = w
		}
	}
	m.helpViewport = viewport.New(
		min(longest, max(20, m.width-8)),
		min(len(lines), max(3, m.height-6)),
	)
	m.helpViewport.SetContent(content)
}

// clearAppStatus hides the spinner/status line and bumps statusGen so any
// delayed reveal still in flight for the action that just finished (see
// statusRevealDelay) gets dropped instead of flashing on screen after the
// fact.
func (m *Model) clearAppStatus() {
	m.appStatus = ""
	m.statusGen++
}

// setPendingFileOp marks (or clears) a "stage:"/"lock:" + path key as
// having a background command in flight, guarding optimistic-UI
// re-entrancy (see pendingFileOps).
func (m *Model) setPendingFileOp(key string, pending bool) {
	if m.pendingFileOps == nil {
		m.pendingFileOps = map[string]bool{}
	}
	if pending {
		m.pendingFileOps[key] = true
	} else {
		delete(m.pendingFileOps, key)
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
	items := m.files.Items()
	for i, it := range items {
		fi, ok := it.(fileItem)
		if !ok || fi.isDir || fi.change.Path != path || fi.staged != from {
			continue
		}
		fi.staged = to
		return m.files.SetItem(i, fi)
	}
	return nil
}

// setFileLockedByPath flips the lock badge on every Files-panel row for
// path (a path can appear as both a staged and an unstaged row - see
// buildFileTree - and a lore lock is a per-path property, not per-row, so
// both need updating together).
func (m *Model) setFileLockedByPath(path string, locked bool) tea.Cmd {
	var cmd tea.Cmd
	items := m.files.Items()
	for i, it := range items {
		fi, ok := it.(fileItem)
		if !ok || fi.isDir || fi.change.Path != path {
			continue
		}
		fi.locked = locked
		cmd = m.files.SetItem(i, fi)
	}
	return cmd
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

// dirStageCounts reports whether any file under dirPath is currently
// unstaged and/or staged, for toggleDirStage's lazygit-style decision
// (stage if anything's unstaged, else unstage).
func (m Model) dirStageCounts(dirPath string) (hasUnstaged, hasStaged bool) {
	for _, it := range m.files.Items() {
		fi, ok := it.(fileItem)
		if !ok || fi.isDir || !dirPrefixMatches(dirPath, fi.change.Path) {
			continue
		}
		if fi.staged {
			hasStaged = true
		} else {
			hasUnstaged = true
		}
	}
	return hasUnstaged, hasStaged
}

// setDirStagedByPrefix is setFileStagedByPath's recursive analog: flips
// every Files-panel row under dirPath (see dirPrefixMatches) currently
// staged as `from` over to `to`, for space on a directory/root row's
// optimistic UI (see toggleDirStage).
func (m *Model) setDirStagedByPrefix(dirPath string, from, to bool) tea.Cmd {
	var cmds []tea.Cmd
	items := m.files.Items()
	for i, it := range items {
		fi, ok := it.(fileItem)
		if !ok || fi.isDir || fi.staged != from || !dirPrefixMatches(dirPath, fi.change.Path) {
			continue
		}
		fi.staged = to
		if cmd := m.files.SetItem(i, fi); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// toggleDirStage handles space on a directory (or the root "/" row, path
// ""): matches lazygit's own directory behavior (files_controller.go's
// press/toggleStaged), not lazylore's old behavior of treating space on a
// directory the same as Enter (collapse toggle - still Enter's job, see
// keys.go). If anything under the directory is unstaged, stage all of it;
// otherwise unstage everything staged under it; no-op if the directory has
// no changes at all. lore stage/unstage accept a directory path directly
// and recurse over already-dirty files under it without needing --scan
// (lore-client's FileStageArgs doc: "without --scan, directory staging
// stages only files already marked dirty under that directory" - exactly
// the already-known-dirty files this tree is built from), so one call
// covers the whole subtree; "." stands in for the repo root since lore has
// no path for the synthetic "/" row itself.
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

	if hasUnstaged {
		optimisticCmd := m.setDirStagedByPrefix(dirPath, false, true)
		return tea.Batch(
			optimisticCmd,
			func() tea.Msg { return setAppStatusMsg("Staging...") },
			dirStageCmd(m.runner, dirPath, lorePath),
		)
	}

	optimisticCmd := m.setDirStagedByPrefix(dirPath, true, false)
	return tea.Batch(
		optimisticCmd,
		func() tea.Msg { return setAppStatusMsg("Unstaging...") },
		dirUnstageCmd(m.runner, dirPath, lorePath),
	)
}

func NewModel(r lore.Runner, repoName, repoRoot string) Model {
	m := Model{
		runner:         r,
		repoName:       repoName,
		repoRoot:       repoRoot,
		collapsedDirs:  map[string]bool{},
		pendingFileOps: map[string]bool{},
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
		func() tea.Msg { return setAppStatusMsg("Loading...") },
		loadStatusCmd(m.runner),
		loadBranchesCmd(m.runner),
		loadHistoryCmd(m.runner),
		loadCurrentUserCmd(m.runner),
		tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} }),
	)
}

func refreshCmd(r lore.Runner) tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return setAppStatusMsg("Refreshing...") },
		loadStatusCmd(r),
		loadBranchesCmd(r),
		loadHistoryCmd(r),
	)
}

// statusRevealDelay is how long an action must still be running before its
// "Staging..."/"Refreshing..."/etc spinner actually appears (see
// setAppStatusMsg/revealStatusMsg in Update). Long enough that fast, local
// lore calls - especially now that stage/unstage/lock already show instant
// optimistic feedback - never show it at all; short enough that a genuinely
// slow action (a big sync, a slow remote) still gets a "please wait" cue
// promptly.
const statusRevealDelay = 200 * time.Millisecond

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
// This matches lazygit where the "extras" / command log lives under the main
// (diff) window rather than spanning under the entire left+right area.
const commandLogPanelHeight = 10

// resize propagates the terminal size to every sub-widget: a Status panel
// plus stacked lists on the left (Files/Branches/History), the diff
// viewport + command log panel below it on the right, plus the prompt/error
// footer and global keybinding bar.
//
// Sizes are computed with distributeSpace (see boxlayout.go) rather than
// plain division, so the panels always sum to exactly the space available
// - matching lazygit's own approach (side panels at weight 1 apiece below
// a fixed-height status box) instead of silently losing a row or two to
// integer-division truncation, which left this layout visibly misaligned.
func (m *Model) resize() {
	// Reserve 1 column of slack instead of filling the terminal to its
	// exact last column. At an exact fit (panel widths summing to exactly
	// m.width), a single-column mismatch anywhere - a terminal's own
	// edge-of-screen auto-wrap behavior, or a glyph this UI uses (▼/▶
	// tree arrows, ✓, the █ scrollbar thumb) rendering one column wider
	// in some terminal/font than lipgloss counts it - wraps that row and
	// visually cascades a shift through every panel below it. Confirmed
	// via debug logging: reported-corrupted rows measured well under
	// their panel's own content width, but the two panels' widths summed
	// to exactly the terminal's reported width with zero margin.
	usableWidth := max(0, m.width-1)
	widths := distributeSpace([]layoutBox{{Weight: 1}, {Weight: 2}}, usableWidth)
	leftWidth := widths[0]
	rightWidth := widths[1]

	m.panelWidth = max(0, leftWidth-borderWidth)
	m.recomputePanelHeights()

	m.files.SetSize(m.panelWidth, max(0, m.filesHeight))
	m.branches.SetSize(m.panelWidth, max(0, m.branchesHeight))
	m.history.SetSize(m.panelWidth, max(0, m.historyHeight))

	// Leave 1 column inside the panel for the scrollbar (drawn after content, before right border)
	diffInnerW := max(0, rightWidth-borderWidth)
	m.diff.vp.Width = max(0, diffInnerW-1)
	m.diff.vp.Height = max(0, m.diffHeight)

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

// rebuildHistoryItems recomputes the History list's items (including
// unpushed/pushed hash coloring) from the model's last-known revisions and
// status. Called from both statusMsg and historyMsg handlers since either
// can determine a row's color and they load independently - whichever
// arrives second must still leave the list correct.
// rebuildFileItems recomputes the Files list's items (including lock/
// lockedByMe badges) from the model's last-known status/locks/currentUserID.
// Called from every handler that can independently learn one of those three
// (statusMsg, locksMsg, currentUserMsg, toggleDirCollapse) since they load
// independently and can arrive in any order - whichever lands last must
// still leave the list correct, same reasoning as rebuildHistoryItems below.
func (m *Model) rebuildFileItems() tea.Cmd {
	items := statusToItems(m.status, m.collapsedDirs, m.locks, m.currentUserID)
	cmd := m.files.SetItems(items)
	m.filesTotal = len(items)
	m.files.SetShowStatusBar(false)
	m.files.SetShowPagination(false)
	return cmd
}

func (m *Model) rebuildHistoryItems() tea.Cmd {
	items := historyToItems(m.revisions, m.status.RemoteRevisionNumber, m.status.HasRemoteInfo)
	cmd := m.history.SetItems(items)
	m.historyTotal = len(items)
	m.history.SetShowStatusBar(false)
	m.history.SetShowPagination(false)
	return cmd
}

// mainPanelTitle returns the shared main panel's title for whatever's
// currently focused, matching lazygit's contextual main view (checked
// against pkg/gui/controllers/files_controller.go's renderWorkingTreeDiff,
// branches_controller.go's LogTitle, and local_commits_controller.go's
// hardcoded "Patch"): Files shows "Unstaged changes"/"Staged changes" for
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
	cmd := m.branches.SetItems(branchesToItems(bs))
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

// ensureMainContent returns a command to (re)load the shared main panel's
// content for whatever is currently focused/selected, matching lazygit's
// own contextual main view: Files shows the selected file's diff, Branches
// shows the selected branch's Log, History shows the selected revision's
// Patch. Skips when the relevant selection hasn't actually changed (avoids
// a load/flash on every cursor move that lands on the same target), and
// when the focused panel has no meaningful content selected (a directory
// in Files, an empty list).
func (m *Model) ensureMainContent() tea.Cmd {
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
			return nil
		}
		if item.isDir {
			// Matches lazygit: selecting a directory clears the main panel
			// instead of leaving the last-selected file's diff (and its
			// "Locked by ..." line) stuck on screen.
			if m.currentDiffPath != "" {
				m.currentDiffPath = ""
				m.diff.SetContentRaw("")
			}
			return nil
		}
		if item.change.Path == m.currentDiffPath {
			return nil
		}
		m.currentDiffPath = item.change.Path
		lock, locked := m.locks[item.change.Path]
		return loadDiffCmd(m.runner, item.change.Path, lock, locked)

	case focusBranches:
		item, ok := m.branches.SelectedItem().(branchItem)
		if !ok || item.branch.Name == m.currentLogBranch {
			return nil
		}
		m.currentLogBranch = item.branch.Name
		return loadBranchLogCmd(m.runner, item.branch.Name)

	case focusHistory:
		item, ok := m.history.SelectedItem().(revisionItem)
		if !ok || item.revision.Hash == m.currentPatchRev {
			return nil
		}
		m.currentPatchRev = item.revision.Hash
		parent := item.revision.Parent
		if lore.IsZeroHash(parent) {
			parent = ""
		}
		return loadRevisionPatchCmd(m.runner, parent, item.revision.Hash)
	}
	return nil
}

// changedPaths flattens a Status's staged and unstaged entries into a single
// path list, for actions (like discard-all) that operate on everything at
// once rather than one selected file.
func changedPaths(s lore.Status) []string {
	paths := make([]string, 0, len(s.Staged)+len(s.Unstaged))
	for _, c := range s.Staged {
		paths = append(paths, c.Path)
	}
	for _, c := range s.Unstaged {
		paths = append(paths, c.Path)
	}
	return paths
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tickMsg:
		if m.appStatus != "" {
			m.spinner = (m.spinner + 1) % 4
			return m, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
				return tickMsg{}
			})
		}
		return m, nil

	case setAppStatusMsg:
		text := string(msg)
		if text == "" {
			(&m).clearAppStatus()
			return m, nil
		}
		// Don't show the spinner immediately - most actions (stage/unstage,
		// lock toggle, the refresh that follows any of them, ...) finish
		// well under statusRevealDelay, especially now that stage/unstage/
		// lock already give instant optimistic feedback of their own (see
		// setFileStagedByPath/setFileLockedByPath). Showing "Staging..."
		// for one frame and yanking it away read as a flicker, not
		// information. Only an action that's genuinely still running once
		// the delay elapses gets a spinner at all.
		m.statusGen++
		gen := m.statusGen
		return m, tea.Tick(statusRevealDelay, func(time.Time) tea.Msg {
			return revealStatusMsg{gen: gen, text: text}
		})

	case revealStatusMsg:
		if msg.gen != m.statusGen {
			// The action this was scheduled for already finished (or was
			// superseded by a newer one) before the delay elapsed.
			return m, nil
		}
		m.appStatus = msg.text
		return m, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
			return tickMsg{}
		})

	case tea.MouseMsg:
		if m.showHelp {
			// Forward for mouse-wheel scrolling (viewport.Model handles it
			// via its own MouseWheelEnabled default) - other clicks harmlessly
			// no-op inside the viewport rather than reaching panels underneath.
			var cmd tea.Cmd
			m.helpViewport, cmd = m.helpViewport.Update(msg)
			return m, cmd
		}
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			return m.handleMouseWheel(msg)
		}
		return m.handleMouseClick(msg)

	case tea.KeyMsg:
		if m.selectMode {
			// Any key exits select mode and restores mouse capture.
			m.selectMode = false
			return m, tea.EnableMouseCellMotion
		}
		if m.showHelp {
			switch msg.String() {
			case "esc", "?":
				m.showHelp = false
				return m, nil
			}
			// Everything else (j/k, arrows, pgup/pgdown, ...) scrolls the
			// popup - viewport.Model's own default keymap handles it;
			// unrecognized keys are a harmless no-op inside it.
			var cmd tea.Cmd
			m.helpViewport, cmd = m.helpViewport.Update(msg)
			return m, cmd
		}
		if m.prompt != promptNone {
			return m.handlePromptKey(msg)
		}
		return m.handleKey(msg)

	case statusMsg:
		(&m).clearAppStatus()
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
		lockCmd := loadLocksCmd(m.runner, changedPaths(msg.status))
		// statusMsg and historyMsg load independently and can arrive in
		// either order; rebuild History's unpushed coloring here too so it's
		// correct even when status lands after history already rendered.
		historyCmd := (&m).rebuildHistoryItems()
		return m, tea.Batch(setCmd, diffCmd, lockCmd, historyCmd)

	case branchesMsg:
		(&m).clearAppStatus()
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
		bs := m.localBranches
		if m.showRemoteBranches {
			bs = m.remoteBranches
		}
		m.branchesTotal = len(bs)
		return m, tea.Batch(cmd, (&m).ensureMainContent())

	case historyMsg:
		(&m).clearAppStatus()
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.revisions = msg.revisions
		cmd := (&m).rebuildHistoryItems()
		return m, tea.Batch(cmd, (&m).ensureMainContent())

	case locksMsg:
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
		(&m).clearAppStatus()
		if msg.err != nil {
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

	case actionDoneMsg:
		(&m).clearAppStatus()
		if msg.opKey != "" {
			(&m).setPendingFileOp(msg.opKey, false)
		}
		if msg.err != nil {
			if msg.revert != nil {
				msg.revert(&m)
			}
			// Command Log already shows this error (AppendAction below) -
			// don't also set m.err, or it'd duplicate into the footer via
			// currentFooter().
			m.log.AppendAction(msg.label, msg.commands, msg.err)
			return m, nil
		}
		if msg.confirm != nil {
			msg.confirm(&m)
		}
		m.log.AppendAction(msg.label, msg.commands, nil)
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
		return m, refreshCmd(m.runner)

	case editorDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		return m, refreshCmd(m.runner)
	}
	return m, nil
}
