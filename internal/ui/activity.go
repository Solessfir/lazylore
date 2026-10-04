package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const activityTickInterval = 180 * time.Millisecond

type activityStartMsg struct {
	id   uint64
	name string
	cmd  tea.Cmd
}

type activityResultMsg struct {
	id    uint64
	inner tea.Msg
}

type activityTickMsg struct {
	generation uint64
}

func startActivityCmd(id uint64, name string, cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg { return activityStartMsg{id: id, name: name, cmd: cmd} }
}

func activityResultCmd(id uint64, cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg { return activityResultMsg{id: id, inner: cmd()} }
}

func (m *Model) activityCmd(name string, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	m.nextActivityID++
	return startActivityCmd(m.nextActivityID, name, activityResultCmd(m.nextActivityID, cmd))
}

func (m *Model) editActivityCmd(path string) tea.Cmd {
	m.nextActivityID++
	// ExecProcess must reach Bubble Tea directly to hand over the terminal.
	return startActivityCmd(m.nextActivityID, "Editing file", editFileCmd(path, m.nextActivityID))
}

func (m Model) activityName() string {
	var newest uint64
	var name string
	for id, activeName := range m.activities {
		if id > newest {
			newest, name = id, activeName
		}
	}
	return name
}

func (m *Model) activityTickCmd() tea.Cmd {
	if len(m.activities) == 0 {
		if m.activityTickPending {
			m.activityTickGeneration++
			m.activityTickPending = false
		}
		m.activityFrame = 0
		return nil
	}
	if m.activityTickPending {
		return nil
	}
	m.activityTickPending = true
	m.activityTickGeneration++
	generation := m.activityTickGeneration
	return tea.Tick(activityTickInterval, func(time.Time) tea.Msg {
		return activityTickMsg{generation: generation}
	})
}
