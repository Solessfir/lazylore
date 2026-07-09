package ui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

// Model is lazylore's root Bubble Tea model.
type Model struct {
	runner lore.Runner

	files    list.Model
	branches list.Model
	history  list.Model
	diff     diffModel
	log      commandLogModel

	focus  focusPanel
	prompt promptKind
	input  textinput.Model

	status lore.Status
	err    error

	width, height int
}

func NewModel(r lore.Runner) Model {
	return Model{
		runner:   r,
		files:    list.New(nil, list.NewDefaultDelegate(), 0, 0),
		branches: list.New(nil, list.NewDefaultDelegate(), 0, 0),
		history:  list.New(nil, list.NewDefaultDelegate(), 0, 0),
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

// resize propagates the terminal size to every sub-widget: three stacked
// lists on the left (Files/Branches/History), the diff viewport on the
// right, and a 3-line footer for the command log / prompt / error line.
// Each panel is drawn with a lipgloss rounded border, so 2 is subtracted
// from both dimensions to leave room for it.
func (m *Model) resize() {
	const footerHeight = 3
	const borderWidth = 2
	const borderHeight = 2

	leftWidth := m.width / 3
	rightWidth := m.width - leftWidth
	bodyHeight := m.height - footerHeight
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	panelHeight := bodyHeight / 3

	listWidth := max(0, leftWidth-borderWidth)
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
