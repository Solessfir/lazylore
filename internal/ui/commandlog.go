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
	action    string
	commands  []string
	err       error
	liveLines []string // progress lines from a still-streaming action (see BeginLive)
}

type commandLogModel struct {
	entries []commandLogEntry
	max     int
	openIdx int // index into entries currently receiving live lines, -1 when none
}

func newCommandLogModel(max int) commandLogModel {
	return commandLogModel{max: max, openIdx: -1}
}

// AppendAction records one user-initiated action and the real command
// line(s) it ran. err (if non-nil) is shown as an extra indented line so a
// failure is still visible in the log, not just in the footer.
func (m *commandLogModel) AppendAction(action string, commands []string, err error) {
	m.entries = append(m.entries, commandLogEntry{action: action, commands: commands, err: err})
	m.trim()
}

// BeginLive opens a new entry that AppendLiveLine appends progress lines to
// as a streamed action (e.g. push) runs, and FinishLive later closes out
// with the real command line(s)/error - matching lazygit's live command
// output instead of only showing a result once the whole action finishes.
func (m *commandLogModel) BeginLive(action string) {
	m.entries = append(m.entries, commandLogEntry{action: action})
	m.openIdx = len(m.entries) - 1
	m.trim()
}

func (m *commandLogModel) AppendLiveLine(line string) {
	if m.openIdx < 0 || m.openIdx >= len(m.entries) {
		return
	}
	m.entries[m.openIdx].liveLines = append(m.entries[m.openIdx].liveLines, line)
}

// FinishLive closes out the entry opened by BeginLive with the actual
// command line(s) run and any error, same info AppendAction records for a
// non-streamed action.
func (m *commandLogModel) FinishLive(commands []string, err error) {
	if m.openIdx < 0 || m.openIdx >= len(m.entries) {
		return
	}
	m.entries[m.openIdx].commands = commands
	m.entries[m.openIdx].err = err
	m.openIdx = -1
}

func (m *commandLogModel) trim() {
	if len(m.entries) <= m.max {
		return
	}
	drop := len(m.entries) - m.max
	m.entries = m.entries[drop:]
	if m.openIdx >= 0 {
		m.openIdx -= drop
		if m.openIdx < 0 {
			// The entry still receiving live lines got trimmed off - stop
			// appending to it rather than corrupting an unrelated entry.
			m.openIdx = -1
		}
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
		for _, l := range e.liveLines {
			lines = append(lines, "  "+l)
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
