package ui

import (
	"strings"
	"time"

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

	files    list.Model
	branches list.Model
	history  list.Model
	stashes  list.Model
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
	stashesTotal  int

	currentDiffPath string // last file path we issued a diff load for (avoids spamming loads on every cursor move)

	focus              focusPanel
	prompt             promptKind
	input              textinput.Model
	pendingDiscardPath string
	selectMode         bool // mouse capture dropped so the terminal can select text (mirrors lazyp4)

	status        lore.Status
	collapsedDirs map[string]bool // Files-panel tree: which directory paths are closed
	err           error

	width, height int

	// Panel content dimensions computed by resize(), reused by View(). Each
	// *Height is the interior budget (after borders) for that panel.
	// The command log lives in its own fixed-height panel under Diff only.
	panelWidth     int
	filesHeight    int
	branchesHeight int
	historyHeight  int
	stashHeight    int
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
	m.stashes.SetDelegate(compactTitleDelegate{focused: m.focus == focusStash, width: m.panelWidth})

	// Re-assert no chrome so "X of Y" count text never appears in bottom right of panels
	m.files.SetShowStatusBar(false)
	m.files.SetShowPagination(false)
	m.branches.SetShowStatusBar(false)
	m.branches.SetShowPagination(false)
	m.history.SetShowStatusBar(false)
	m.history.SetShowPagination(false)
	m.stashes.SetShowStatusBar(false)
	m.stashes.SetShowPagination(false)
}

// handleMouseClick handles left-clicks to focus panels and select items inside
// lists (Files, Branches, History), similar to lazygit mouse behavior.
// Diff viewport also receives mouse events for scrolling.
func (m Model) handleMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
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

	// Use full distribute over grown main area for accurate hit rects (so unfocused
	// stash stays small, etc). Matches View().
	mainAvail := bodyHeight + extra
	stashBox := layoutBox{Weight: 1}
	if m.focus != focusStash {
		stashBox = layoutBox{Size: 3}
	}
	leftOuters := distributeSpace([]layoutBox{
		{Size: statusPanelHeight},
		{Weight: 1},
		{Weight: 1},
		{Weight: 1},
		stashBox,
	}, mainAvail)
	effFilesH := max(0, leftOuters[1]-borderHeight)
	effBranchesH := max(0, leftOuters[2]-borderHeight)
	effHistoryH := max(0, leftOuters[3]-borderHeight)
	effStashH := max(0, leftOuters[4]-borderHeight)
	effDiffH := m.diffHeight + extra

	leftW := m.panelWidth + borderWidth
	statusH := statusPanelHeight
	filesH := effFilesH + borderHeight
	branchesH := effBranchesH + borderHeight
	historyH := effHistoryH + borderHeight
	stashH := effStashH + borderHeight
	diffH := effDiffH + borderHeight
	// mainH is height of full left stack (and of right column = diff + commandLogPanelHeight)
	mainH := statusH + filesH + branchesH + historyH + stashH

	if y >= mainH {
		return m, nil // footer / keybind area
	}

	// Left column panels
	filesBoxTop := statusH
	branchesBoxTop := filesBoxTop + filesH
	historyBoxTop := branchesBoxTop + branchesH
	stashBoxTop := historyBoxTop + historyH

	newFocus := m.focus
	var cmd tea.Cmd

	if x < leftW {
		// Left side
		switch {
		case y < filesBoxTop:
			// Status area -> focus Files (like jump key 1/2)
			newFocus = focusFiles
		case y < branchesBoxTop:
			newFocus = focusFiles
			// Click inside Files content: select the exact row under mouse.
			// (bubbles/list.Update ignores MouseMsg for cursor; it only reacts to keys.
			// We compute the target index in the visible list and Select it.)
			if y >= filesBoxTop+1 && y < filesBoxTop+filesH-1 {
				relY := y - (filesBoxTop + 1)
				if relY >= 0 && relY < effFilesH {
					perPage := m.files.Paginator.PerPage
					if perPage < 1 {
						perPage = 1
					}
					target := m.files.Paginator.Page*perPage + relY
					vis := m.files.VisibleItems()
					if target < len(vis) {
						m.files.Select(target)
						if dcmd := (&m).ensureDiffForSelectedFile(); dcmd != nil {
							cmd = dcmd
						}
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
				return m, m.refreshBranchesList()
			}
			if y >= branchesBoxTop+1 && y < branchesBoxTop+branchesH-1 {
				relY := y - (branchesBoxTop + 1)
				if relY >= 0 && relY < effBranchesH {
					perPage := m.branches.Paginator.PerPage
					if perPage < 1 {
						perPage = 1
					}
					target := m.branches.Paginator.Page*perPage + relY
					vis := m.branches.VisibleItems()
					if target < len(vis) {
						m.branches.Select(target)
					}
				}
			}
		case y < stashBoxTop:
			newFocus = focusHistory
			if y >= historyBoxTop+1 && y < historyBoxTop+historyH-1 {
				relY := y - (historyBoxTop + 1)
				if relY >= 0 && relY < effHistoryH {
					perPage := m.history.Paginator.PerPage
					if perPage < 1 {
						perPage = 1
					}
					target := m.history.Paginator.Page*perPage + relY
					vis := m.history.VisibleItems()
					if target < len(vis) {
						m.history.Select(target)
					}
				}
			}
		default:
			newFocus = focusStash
			if y >= stashBoxTop+1 && y < stashBoxTop+stashH-1 {
				relY := y - (stashBoxTop + 1)
				if relY >= 0 && relY < effStashH {
					perPage := m.stashes.Paginator.PerPage
					if perPage < 1 {
						perPage = 1
					}
					target := m.stashes.Paginator.Page*perPage + relY
					vis := m.stashes.VisibleItems()
					if target < len(vis) {
						m.stashes.Select(target)
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
	items := statusToItems(m.status, m.collapsedDirs)
	cmd := m.files.SetItems(items)
	m.filesTotal = len(items)
	m.files.SetShowStatusBar(false)
	m.files.SetShowPagination(false)
	return cmd
}

func NewModel(r lore.Runner, repoName string) Model {
	m := Model{
		runner:        r,
		repoName:      repoName,
		collapsedDirs: map[string]bool{},
		// focusFiles is the initial focus, below. width is 0 until the first
		// resize() - fine, syncFocusDelegates rebuilds these once real
		// dimensions are known.
		files:    newPanelList(fileDelegate{focused: true}),
		branches: newPanelList(compactTitleDelegate{focused: false, width: 0}),
		history:  newPanelList(compactTitleDelegate{focused: false, width: 0}),
		stashes:  newPanelList(compactTitleDelegate{focused: false, width: 0}),
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
	m.stashes.SetShowStatusBar(false)
	m.stashes.SetShowPagination(false)

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return setAppStatusMsg("Loading...") },
		loadStatusCmd(m.runner),
		loadBranchesCmd(m.runner),
		loadHistoryCmd(m.runner),
		loadStashesCmd(m.runner),
		tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} }),
	)
}

func refreshCmd(r lore.Runner) tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return setAppStatusMsg("Refreshing...") },
		loadStatusCmd(r),
		loadBranchesCmd(r),
		loadHistoryCmd(r),
		loadStashesCmd(r),
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
// plus stacked lists on the left (Files/Branches/History/Stash), the diff
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
	m.stashes.SetSize(m.panelWidth, max(0, m.stashHeight))

	// Leave 1 column inside the panel for the scrollbar (drawn after content, before right border)
	diffInnerW := max(0, rightWidth-borderWidth)
	m.diff.vp.Width = max(0, diffInnerW-1)
	m.diff.vp.Height = max(0, m.diffHeight)

	// Branches/History's selected-row width is baked into their delegate
	// (see newListDelegate) rather than read live, so it has to be rebuilt
	// whenever panelWidth changes here, not just on focus changes.
	m.syncFocusDelegates()
}

// recomputePanelHeights calculates the base inner heights for all panels
// using distributeSpace. Stash gets a small fixed Size (matching lazygit)
// unless it is currently focused, in which case it gets Weight:1 and expands.
// Other variable panels always weight evenly. This must be called on focus
// changes (in addition to resize) so that m.*Height bases are up to date for
// View() eff growth and mouse hit testing.
func (m *Model) recomputePanelHeights() {
	bodyHeight := max(0, m.height-footerHeight-keybindBarHeight)

	stashBox := layoutBox{Weight: 1}
	if m.focus != focusStash {
		stashBox = layoutBox{Size: 3} // lazygit default for unfocused stash
	}

	heights := distributeSpace([]layoutBox{
		{Size: statusPanelHeight}, // Status
		{Weight: 1},               // Files
		{Weight: 1},               // Branches
		{Weight: 1},               // History
		stashBox,                  // Stash (small unless focused)
	}, bodyHeight)

	m.filesHeight = max(0, heights[1]-borderHeight)
	m.branchesHeight = max(0, heights[2]-borderHeight)
	m.historyHeight = max(0, heights[3]-borderHeight)
	m.stashHeight = max(0, heights[4]-borderHeight)

	// Diff leaves room under itself for the command log (fixed, right only).
	cmdLogOuter := commandLogPanelHeight
	diffOuter := max(0, bodyHeight-cmdLogOuter)
	m.diffHeight = max(0, diffOuter-borderHeight)
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

func (m Model) currentFooter() string {
	switch {
	case m.prompt == promptConfirmDiscard:
		return "Discard changes to " + m.pendingDiscardPath + "? (y/N)"
	case m.prompt != promptNone:
		return m.input.View()
	case m.err != nil:
		// Error goes in the spanning footer area. The command log itself now
		// lives in its own panel under Diff (like lazygit extras).
		return errorStyle.Render(m.err.Error())
	default:
		return ""
	}
}

// ensureDiffForSelectedFile returns commands to load (and show status for)
// the diff of the currently selected *file* (not directory) in the Files
// panel. It skips if we're not focused on Files, the selection is a dir,
// or we've already loaded the diff for that exact path (prevents a
// "Loading diff..." flash on every arrow key).
func (m *Model) ensureDiffForSelectedFile() tea.Cmd {
	if m.focus != focusFiles {
		return nil
	}
	item, ok := m.files.SelectedItem().(fileItem)
	if !ok || item.isDir {
		return nil
	}
	if item.change.Path == m.currentDiffPath {
		return nil
	}
	m.currentDiffPath = item.change.Path
	return loadDiffCmd(m.runner, item.change.Path)
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
		return m.handleMouseClick(msg)

	case tea.KeyMsg:
		if m.selectMode {
			// Any key exits select mode and restores mouse capture (lazyp4's
			// handleKey does the same as the very first check).
			m.selectMode = false
			return m, tea.EnableMouseCellMotion
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
		items := statusToItems(msg.status, m.collapsedDirs)
		setCmd := m.files.SetItems(items)
		m.filesTotal = len(items)
		m.files.SetShowStatusBar(false)
		m.files.SetShowPagination(false)
		diffCmd := m.ensureDiffForSelectedFile()
		return m, tea.Batch(setCmd, diffCmd)

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
		return m, cmd

	case historyMsg:
		m.appStatus = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		items := historyToItems(msg.revisions)
		cmd := m.history.SetItems(items)
		m.historyTotal = len(items)
		m.history.SetShowStatusBar(false)
		m.history.SetShowPagination(false)
		return m, cmd

	case stashesMsg:
		m.appStatus = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		items := stashesToItems(msg.stashes)
		cmd := m.stashes.SetItems(items)
		m.stashesTotal = len(items)
		m.stashes.SetShowStatusBar(false)
		m.stashes.SetShowPagination(false)
		return m, cmd

	case diffMsg:
		m.appStatus = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.diff.SetContent(msg.text)
		return m, nil

	case actionDoneMsg:
		m.appStatus = ""
		if msg.err != nil {
			m.log.Append(msg.label + ": FAILED: " + msg.err.Error())
			m.err = msg.err
			return m, nil
		}
		m.log.Append(msg.label + ": OK")
		return m, refreshCmd(m.runner)
	}
	return m, nil
}
