package ui

import (
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
	diff     diffModel
	log      commandLogModel

	focus              focusPanel
	prompt             promptKind
	input              textinput.Model
	pendingDiscardPath string

	status lore.Status
	err    error

	width, height int

	// Panel content dimensions computed by resize(), reused by View(). Each
	// *Height is the FULL interior budget for that panel (title row + list
	// content), not just the list's own row count - list.Model.View() and
	// viewport.Model.View() don't pad their own output to fill a configured
	// size when they have little content, they only render actual content
	// lines, so View() must set an explicit Width/Height on every panel's
	// border itself (using these fields) or panels with different content
	// shrink to different sizes instead of lining up.
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

// newListDelegate returns a list.ItemDelegate whose selected row gets a
// solid background fill (like lazygit) on top of the default delegate's
// own left-border marker, not just a foreground color tweak - shared by
// the Files/Branches/History lists.
func newListDelegate() list.ItemDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.
		Background(selectedBg).
		Foreground(selectedFg).
		Bold(true)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.
		Background(selectedBg).
		Foreground(selectedFg)
	return d
}

// newPanelList builds a list.Model with this app's shared delegate and its
// own built-in title/status-bar/help chrome turned off - the panel border
// (drawn in View) already carries the title, and keybindings live in the
// single global bar at the bottom of the screen instead of being repeated
// per panel.
func newPanelList() list.Model {
	l := list.New(nil, newListDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	return l
}

func NewModel(r lore.Runner, repoName string) Model {
	return Model{
		runner:   r,
		repoName: repoName,
		files:    newPanelList(),
		branches: newPanelList(),
		history:  newPanelList(),
		diff:     newDiffModel(0, 0),
		log:      newCommandLogModel(20),
		focus:    focusFiles,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadStatusCmd(m.runner), loadBranchesCmd(m.runner), loadHistoryCmd(m.runner))
}

func refreshCmd(r lore.Runner) tea.Cmd {
	return tea.Batch(loadStatusCmd(r), loadBranchesCmd(r), loadHistoryCmd(r))
}

// footerHeight is the number of terminal rows reserved for the command
// log / prompt / error line, directly above the global keybinding bar.
// It's used both by resize(), to leave room for that area above the
// panels, and by View(), to cap how many command-log entries are actually
// rendered so the footer can't grow past its reserved space and push the
// panel layout around.
const footerHeight = 3

// keybindBarHeight is the single always-visible row at the very bottom of
// the screen showing the global keybinding legend.
const keybindBarHeight = 1

// statusPanelHeight is the small bordered panel above Files showing the
// repo name and current branch: a "Status" title line, one content line,
// plus its border.
const statusPanelHeight = 4

// borderWidth/borderHeight are the space every bordered panel's lipgloss
// rounded border consumes. resize() uses them to size each panel's inner
// content; View() reuses borderWidth to size the Status panel, which has
// no bubbles widget of its own to size it automatically.
const (
	borderWidth  = 2
	borderHeight = 2
)

// resize propagates the terminal size to every sub-widget: a Status panel
// plus three stacked lists on the left (Files/Branches/History), the diff
// viewport on the right, the command log / prompt / error footer, and the
// global keybinding bar.
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

	bodyHeight := max(0, m.height-footerHeight-keybindBarHeight)

	heights := distributeSpace([]layoutBox{
		{Size: statusPanelHeight}, // Status
		{Weight: 1},               // Files
		{Weight: 1},               // Branches
		{Weight: 1},               // History
	}, bodyHeight)

	// titleRowHeight is the "Files"/"Branches"/"History"/"Diff" title line
	// each panel renders above its content - part of the panel's interior
	// budget, not extra space on top of it, so the wrapped widget itself
	// gets one row less than the panel's full interior height.
	const titleRowHeight = 1

	m.panelWidth = max(0, leftWidth-borderWidth)
	m.filesHeight = max(0, heights[1]-borderHeight)
	m.branchesHeight = max(0, heights[2]-borderHeight)
	m.historyHeight = max(0, heights[3]-borderHeight)
	m.diffHeight = max(0, bodyHeight-borderHeight)

	m.files.SetSize(m.panelWidth, max(0, m.filesHeight-titleRowHeight))
	m.branches.SetSize(m.panelWidth, max(0, m.branchesHeight-titleRowHeight))
	m.history.SetSize(m.panelWidth, max(0, m.historyHeight-titleRowHeight))

	m.diff.vp.Width = max(0, rightWidth-borderWidth)
	m.diff.vp.Height = max(0, m.diffHeight-titleRowHeight)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tea.KeyMsg:
		if m.prompt != promptNone {
			return m.handlePromptKey(msg)
		}
		return m.handleKey(msg)

	case statusMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.status = msg.status
		if cmd := m.files.SetItems(statusToItems(msg.status)); cmd != nil {
			return m, cmd
		}
		return m, nil

	case branchesMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if cmd := m.branches.SetItems(branchesToItems(msg.branches)); cmd != nil {
			return m, cmd
		}
		return m, nil

	case historyMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if cmd := m.history.SetItems(historyToItems(msg.revisions)); cmd != nil {
			return m, cmd
		}
		return m, nil

	case diffMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.diff.SetContent(msg.text)
		return m, nil

	case actionDoneMsg:
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
