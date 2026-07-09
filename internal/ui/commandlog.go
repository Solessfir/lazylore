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

// LastLines returns the last n rendered log entries, joined by newlines.
// It caps what's rendered without discarding older entries from m.entries
// (retention is still governed by max in Append). n <= 0 yields "".
func (m commandLogModel) LastLines(n int) string {
	if n <= 0 || len(m.entries) == 0 {
		return ""
	}
	entries := m.entries
	if len(entries) > n {
		entries = entries[len(entries)-n:]
	}
	return strings.Join(entries, "\n")
}
