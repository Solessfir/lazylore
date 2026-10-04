package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// This file is a near-verbatim port of lazyp4's internal/ui/overlay.go
// (C:\Git\lazyp4\internal\ui\overlay.go) - the ANSI-safe string splicing
// needed to composite a modal box on top of the already-rendered screen
// without corrupting the background's own color codes.

// placeOverlay renders fg on top of bg at position (x, y).
// Preserve background styling on both sides of the popup.
func placeOverlay(x, y int, fg, bg string) string {
	fgLines := strings.Split(fg, "\n")
	bgLines := strings.Split(bg, "\n")
	result := make([]string, len(bgLines))
	copy(result, bgLines)

	for fi, fgLine := range fgLines {
		bi := y + fi
		if bi < 0 || bi >= len(result) {
			continue
		}
		result[bi] = spliceLine(x, fgLine, result[bi])
	}
	return strings.Join(result, "\n")
}

// overlayCenter centers fg over bg and returns the merged string.
func overlayCenter(fg, bg string, bgW, bgH int) string {
	fgLines := strings.Split(fg, "\n")
	fgW := 0
	for _, l := range fgLines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}
	fgH := len(fgLines)
	ox := (bgW - fgW) / 2
	oy := (bgH - fgH) / 2
	if ox < 0 {
		ox = 0
	}
	if oy < 0 {
		oy = 0
	}
	return placeOverlay(ox, oy, fg, bg)
}

// spliceLine overlays fgLine onto bgLine starting at column x.
func spliceLine(x int, fg, bg string) string {
	fgW := lipgloss.Width(fg)

	// Left: bg truncated to x columns with ANSI preserved.
	left := ansiTruncate(bg, x)
	leftW := lipgloss.Width(left)
	if leftW < x {
		left += strings.Repeat(" ", x-leftW)
	}

	// Right: retain background styles after resetting the popup's styles.
	right := ansiSkip(bg, x+fgW)

	return left + "\033[0m" + fg + "\033[0m" + right
}

// ansiTruncate truncates s to maxWidth visible columns, preserving ANSI sequences.
func ansiTruncate(s string, maxWidth int) string {
	return ansi.Truncate(s, max(0, maxWidth), "")
}

// ansiSkip skips visible columns while retaining the suffix's ANSI styles.
func ansiSkip(s string, skipCols int) string {
	plain := ansi.Strip(s)
	right := ansi.TruncateLeft(s, max(0, skipCols), "")
	if excess := lipgloss.Width(right) - max(0, lipgloss.Width(plain)-skipCols); excess > 0 {
		// A wide glyph crossing the cut cannot be rendered as a partial cell.
		_, width := ansi.FirstGraphemeCluster(ansi.Strip(right), ansi.GraphemeWidth)
		right = strings.Repeat(" ", width-excess) + ansi.TruncateLeft(right, width, "")
	}
	return right
}

func ansiStrip(s string) string {
	return ansi.Strip(s)
}
