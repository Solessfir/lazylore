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
	leftWidth     int // left column's outer width, incl. border; View reuses it for the Status panel
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
// repo name and current branch: one content line plus its border.
const statusPanelHeight = 3

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
func (m *Model) resize() {
	m.leftWidth = m.width / 3
	rightWidth := m.width - m.leftWidth
	bodyHeight := m.height - footerHeight - keybindBarHeight
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	listAreaHeight := bodyHeight - statusPanelHeight
	if listAreaHeight < 0 {
		listAreaHeight = 0
	}
	panelHeight := listAreaHeight / 3

	listWidth := max(0, m.leftWidth-borderWidth)
	listHeight := max(0, panelHeight-borderHeight)
	m.files.SetSize(listWidth, listHeight)
	m.branches.SetSize(listWidth, listHeight)
	m.history.SetSize(listWidth, listHeight)

	m.diff.vp.Width = max(0, rightWidth-borderWidth)
	m.diff.vp.Height = max(0, bodyHeight-borderHeight)
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
