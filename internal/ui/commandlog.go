package ui

import "strings"

type commandLogModel struct {
	entries []string
	max     int
}

func newCommandLogModel(max int) commandLogModel {
	return commandLogModel{max: max}
}

func (m *commandLogModel) Append(entry string) {
	m.entries = append(m.entries, entry)
	if len(m.entries) > m.max {
		m.entries = m.entries[len(m.entries)-m.max:]
	}
}

func (m commandLogModel) View() string {
	return strings.Join(m.entries, "\n")
}
