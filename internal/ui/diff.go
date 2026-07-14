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
	diffHunkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	diffHeaderStyle = lipgloss.NewStyle().Bold(true)
)

type diffModel struct {
	vp         viewport.Model
	totalLines int
}

func newDiffModel(width, height int) diffModel {
	vp := viewport.New(width, height)
	// The diff pane never wraps or scrolls horizontally (lazygit clips
	// instead) - a stray left/right arrow press while it's focused would
	// otherwise scroll it via viewport's own default keymap, silently
	// offsetting every line rendered afterward (see viewWithScrollbar's
	// ansi.Cut(..., xOffset, ...) call inside bubbles itself) until a wide
	// enough line exposed it as visibly cut-off/garbled text.
	vp.KeyMap.Left = key.Binding{}
	vp.KeyMap.Right = key.Binding{}
	return diffModel{vp: vp}
}

func (m *diffModel) SetContent(text string) {
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

// viewWithScrollbar returns the diff content with a simple right-side scrollbar
// (visible when total content > visible height). The scrollbar appears just
// inside the right border to approximate lazygit behavior.
func (m diffModel) viewWithScrollbar() string {
	content := m.vp.View()
	lines := strings.Split(content, "\n")
	h := m.vp.Height
	if h <= 0 {
		return content
	}

	// pad/truncate
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}

	if m.totalLines <= h {
		// no scrollbar needed
		return strings.Join(lines, "\n")
	}

	// compute thumb
	percent := 0.0
	if m.vp.YOffset > 0 && m.totalLines > 0 {
		percent = float64(m.vp.YOffset) / float64(max(1, m.totalLines-h))
	}
	thumbStart := int(percent * float64(h-1))
	thumbSize := max(1, h*h/m.totalLines)

	innerW := m.vp.Width
	// Pad-or-truncate every line to exactly innerW so the scrollbar lands
	// flush against the right border. Must go through lipgloss (ANSI-aware)
	// rather than slicing runes directly - colorizeDiff-styled lines carry
	// ANSI escape codes, and a raw []rune slice cuts through the middle of
	// those on any line longer than innerW, corrupting the terminal's color
	// state for everything rendered afterward.
	cellStyle := lipgloss.NewStyle().Width(innerW).MaxWidth(innerW)
	var result []string
	for i, line := range lines {
		padded := cellStyle.Render(line)

		bar := "│" // track
		if i >= thumbStart && i < thumbStart+thumbSize {
			bar = "█" // thumb
		}
		result = append(result, padded+bar)
	}
	return strings.Join(result, "\n")
}

func colorizeDiff(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			lines[i] = diffHeaderStyle.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = diffAddStyle.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = diffDelStyle.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = diffHunkStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
