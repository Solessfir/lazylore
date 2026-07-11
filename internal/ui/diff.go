package ui

import (
	"strings"

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
	return diffModel{vp: viewport.New(width, height)}
}

func (m *diffModel) SetContent(text string) {
	colored := colorizeDiff(text)
	m.totalLines = len(strings.Split(colored, "\n"))
	m.vp.SetContent(colored)
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
	var result []string
	for i, line := range lines {
		// pad line to full inner content width so scrollbar is right-aligned inside the panel
		padded := line
		if w := lipgloss.Width(line); w < innerW {
			padded += strings.Repeat(" ", innerW-w)
		} else if w > innerW {
			padded = string([]rune(line)[:innerW])
		}

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
