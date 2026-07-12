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

	focus                focusPanel
	prompt               promptKind
	input                textinput.Model
	pendingDiscardPath   string
	pendingResetRevision string         // revision `g` (branch reset) will target once confirmed
	pendingResetLabel    string         // human phrase for the confirm popup + command log, e.g. "Reset current branch to main"
	pendingRevertMessage string         // auto-commit message `d` (Drop/revert) will pass to lore, e.g. `Revert "oops"`
	selectMode           bool           // mouse capture dropped so the terminal can select text (mirrors lazyp4)
	showHelp             bool           // "?" keybindings popup (see modal.go), mirrors lazyp4's own help overlay
	helpViewport         viewport.Model // scrolls the keybindings popup's body (j/k/arrows/mouse wheel via its own default keymap)

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

// rowsPerPage is bubbles/list's own real per-page item capacity for a panel
// whose content area is contentH rows tall (fileDelegate/compactTitleDelegate
// both report Height()=1, Spacing()=0). bubbles/list.Model always reserves
// one row at the top of its own View() output for a potential filter input
// (tied to filteringEnabled, independent of ShowTitle - see list.go's
// updatePagination), which lazylore's border rendering strips and lipgloss
// re-pads at the bottom instead - so real capacity is contentH-1, not contentH.
func rowsPerPage(contentH int) int {
	return max(1, contentH-1)
}

// rowClickTarget maps a list panel's clicked row (relY, 0-based within the
// panel's content area) to the absolute item index it corresponds to on the
// CURRENT page, or ok=false if relY falls on a blank/padding row.
//
// perPage must be freshly computed from what's actually on screen (see
// rowsPerPage), not read from the list's own Paginator.PerPage: View() calls
// list.Model.SetSize with a footer-shrink-adjusted height on every render,
// but View() has a value receiver, so that call never persists back to the
// real model - Paginator.PerPage silently drifts stale (commonly by exactly
// one row) relative to what's actually rendered. Bounding the click only
// against the TOTAL item count across all pages (`target < len(vis)`) then
// let a click on the last real row - or the padding row below it - resolve
// to a real item on a page that isn't even visible, silently flipping
// Select()'s page and desyncing every click after it from what's on screen.
func rowClickTarget(page, perPage int, vis []list.Item, relY int) (int, bool) {
	if perPage < 1 {
		perPage = 1
	}
	pageStart := page * perPage
	itemsOnPage := min(perPage, len(vis)-pageStart)
	if relY < 0 || relY >= itemsOnPage {
		return 0, false
	}
	return pageStart + relY, true
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

	x, y := msg.X, msg.Y

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

	// Use full distribute over grown main area for accurate hit rects. Matches View().
	mainAvail := bodyHeight + extra
	leftOuters := distributeSpace([]layoutBox{
		{Size: statusPanelHeight},
		{Weight: 1},
		{Weight: 1},
		{Weight: 1},
	}, mainAvail)
	effFilesH := max(0, leftOuters[1]-borderHeight)
	effBranchesH := max(0, leftOuters[2]-borderHeight)
	effHistoryH := max(0, leftOuters[3]-borderHeight)
	effDiffH := m.diffHeight + extra

	// Resync each list's own Paginator.PerPage to the height actually
	// rendered THIS frame before doing any click math. list.Model.Select
	// (called below) divides by its own Paginator.PerPage internally, which
	// - like the field rowsPerPage/rowClickTarget already route around on
	// the read side - drifts stale relative to what's on screen (View() has
	// a value receiver, so its own SetSize call never persists). Without
	// this resync, Select still re-derives Page from the stale PerPage and
	// can jump to a different page on the very next render, even though the
	// clicked row was computed correctly - a visible "shift" on click.
	m.files.SetSize(m.panelWidth, effFilesH)
	m.branches.SetSize(m.panelWidth, effBranchesH)
	m.history.SetSize(m.panelWidth, effHistoryH)

	leftW := m.panelWidth + borderWidth
	statusH := statusPanelHeight
	filesH := effFilesH + borderHeight
	branchesH := effBranchesH + borderHeight
	historyH := effHistoryH + borderHeight
	diffH := effDiffH + borderHeight
	// mainH is height of full left stack (and of right column = diff + commandLogPanelHeight)
	mainH := statusH + filesH + branchesH + historyH

	if y >= mainH {
		return m, nil // footer / keybind area
	}

	// Left column panels
	filesBoxTop := statusH
	branchesBoxTop := filesBoxTop + filesH
	historyBoxTop := branchesBoxTop + branchesH

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
					if target, ok := rowClickTarget(m.files.Paginator.Page, rowsPerPage(effFilesH), m.files.VisibleItems(), relY); ok {
						m.files.Select(target)
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
					if target, ok := rowClickTarget(m.branches.Paginator.Page, rowsPerPage(effBranchesH), m.branches.VisibleItems(), relY); ok {
						m.branches.Select(target)
					}
				}
			}
		default:
			newFocus = focusHistory
			if y >= historyBoxTop+1 && y < historyBoxTop+historyH-1 {
				relY := y - (historyBoxTop + 1)
				if relY >= 0 && relY < effHistoryH {
					if target, ok := rowClickTarget(m.history.Paginator.Page, rowsPerPage(effHistoryH), m.history.VisibleItems(), relY); ok {
						m.history.Select(target)
					}
				}
			}
		}
	} else {
		// Right side: Diff (top) + Command Log (directly below it)
		newFocus = focusDiff
		diffBoxLeft := leftW
		diffBoxTop := 0
		diffAreaEnd := diffBoxTop + diffH
		if y >= diffBoxTop+1 && y < diffAreaEnd-1 {
			relY := y - (diffBoxTop + 1)
			relX := x - (diffBoxLeft + 1)
			if relY >= 0 && relY < effDiffH {
				mm := tea.MouseMsg{X: relX, Y: relY, Button: msg.Button, Action: msg.Action}
				m.diff.vp, cmd = m.diff.vp.Update(mm)
			}
		}
		// Clicks inside the command log area (below diffH) are intentionally
		// not forwarded (log is not a scrollable viewport yet).
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
const commandLogPanelHeight = 5

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
	widths := distributeSpace([]layoutBox{{Weight: 1}, {Weight: 2}}, m.width)
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
	// Status has no main-panel content of its own (matches lazygit - it's
	// not one of the contexts that drives the main view), so focusing it
	// must not overwrite what mainPanelTitle/the diff panel still shows.
	if m.focus != focusDiff && m.focus != focusStatus {
		m.mainContentSource = m.focus
	}
	switch m.focus {
	case focusFiles:
		item, ok := m.files.SelectedItem().(fileItem)
		if !ok || item.isDir || item.change.Path == m.currentDiffPath {
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
		m.appStatus = string(msg)
		if m.appStatus != "" {
			return m, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
				return tickMsg{}
			})
		}
		return m, nil

	case tea.MouseMsg:
		if m.showHelp {
			// Forward for mouse-wheel scrolling (viewport.Model handles it
			// via its own MouseWheelEnabled default) - other clicks harmlessly
			// no-op inside the viewport rather than reaching panels underneath.
			var cmd tea.Cmd
			m.helpViewport, cmd = m.helpViewport.Update(msg)
			return m, cmd
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
		m.appStatus = ""
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
		m.appStatus = ""
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
		m.appStatus = ""
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
		m.appStatus = ""
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
		m.appStatus = ""
		if msg.opKey != "" {
			(&m).setPendingFileOp(msg.opKey, false)
		}
		if msg.err != nil {
			if msg.revert != nil {
				msg.revert(&m)
			}
			m.log.AppendAction(msg.label, msg.commands, msg.err)
			m.err = msg.err
			return m, nil
		}
		m.log.AppendAction(msg.label, msg.commands, nil)
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
