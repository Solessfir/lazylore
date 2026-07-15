package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// commandLogActionStyle colors the log's action title gold; the plain
// command lines under it use the terminal's default text color.
var commandLogActionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

// commandLogEntry is one user-initiated action: a human title plus the
// actual lore command line(s) it ran, shown without the leading "--json"
// flag since that's plumbing, not something a user typing the command
// themselves would include.
type commandLogEntry struct {
	action   string
	commands []string
	err      error
}

type commandLogModel struct {
	entries []commandLogEntry
	max     int
}

func newCommandLogModel(max int) commandLogModel {
	return commandLogModel{max: max}
}

// AppendAction records one user-initiated action and the real command
// line(s) it ran. err (if non-nil) is shown as an extra indented line so a
// failure is still visible in the log, not just in the footer.
func (m *commandLogModel) AppendAction(action string, commands []string, err error) {
	m.entries = append(m.entries, commandLogEntry{action: action, commands: commands, err: err})
	if len(m.entries) > m.max {
		m.entries = m.entries[len(m.entries)-m.max:]
	}
}

func (m commandLogModel) View() string {
	var lines []string
	for i, e := range m.entries {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, commandLogActionStyle.Render(e.action))
		for _, c := range e.commands {
			lines = append(lines, "  "+c)
		}
		if e.err != nil {
			lines = append(lines, errorStyle.Render("  "+e.err.Error()))
		}
	}
	return strings.Join(lines, "\n")
}

// LastLines returns the last n rendered log lines (not entries - one entry
// can span several lines), joined by newlines. It caps what's rendered
// without discarding older entries (retention is governed by max in
// AppendAction). n <= 0 yields "".
func (m commandLogModel) LastLines(n int) string {
	full := m.View()
	if n <= 0 || full == "" {
		return ""
	}
	lines := strings.Split(full, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
