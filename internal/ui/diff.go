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
	vp viewport.Model
}

func newDiffModel(width, height int) diffModel {
	return diffModel{vp: viewport.New(width, height)}
}

func (m *diffModel) SetContent(text string) {
	m.vp.SetContent(colorizeDiff(text))
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
