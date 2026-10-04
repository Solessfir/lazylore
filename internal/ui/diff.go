package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	diffAddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	diffDelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	diffHunkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	diffHeaderStyle = lipgloss.NewStyle().Bold(true)
)

type diffModel struct {
	vp         viewport.Model
	totalLines int
}

func newDiffModel(width, height int) diffModel {
	vp := viewport.New(width, height)
	// No horizontal scroll: a stray left/right arrow while focused would
	// otherwise offset every line via viewport's own default keymap.
	vp.KeyMap.Left = key.Binding{}
	vp.KeyMap.Right = key.Binding{}
	return diffModel{vp: vp}
}

func (m *diffModel) SetContent(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "")
	colored := colorizeDiff(text)
	m.totalLines = len(strings.Split(colored, "\n"))
	m.vp.SetContent(colored)
	m.vp.SetXOffset(0)
}

// SetContentRaw sets already-styled content (e.g. a branch log) without
// running it through colorizeDiff, which would misinterpret any line that
// happens to start with "+"/"-" (a commit message, an author name) as a
// diff addition/deletion.
func (m *diffModel) SetContentRaw(text string) {
	m.totalLines = len(strings.Split(text, "\n"))
	m.vp.SetContent(text)
	m.vp.SetXOffset(0)
}

func colorizeDiff(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
			lines[i] = diffHeaderStyle.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = diffAddStyle.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = diffDelStyle.Render(line)
		case strings.HasPrefix(line, "@@"):
			if end := strings.Index(line[2:], "@@"); end >= 0 {
				end += 4
				lines[i] = diffHunkStyle.Render(line[:end]) + line[end:]
			} else {
				lines[i] = diffHunkStyle.Render(line)
			}
		}
	}
	return strings.Join(lines, "\n")
}
