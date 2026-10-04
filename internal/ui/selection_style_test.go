package ui

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"

	"lazylore/internal/lore"
)

func TestCursorRowsPreserveAttributesAcrossColoredSegments(t *testing.T) {
	previous := lipgloss.DefaultRenderer()
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	lipgloss.SetDefaultRenderer(renderer)
	t.Cleanup(func() { lipgloss.SetDefaultRenderer(previous) })
	background := cellbuf.NewBuffer(1, 1)
	cellbuf.SetContent(background, "\x1b["+termenv.RGBColor("#292a2e").Sequence(true)+"m \x1b[0m")

	tests := []struct {
		name string
		item list.Item
		file bool
		fg   map[string]ansi.Color
	}{
		{"unstaged", fileItem{label: "file.txt", depth: 1, change: lore.FileChange{Status: 'M'}, locked: true}, true, map[string]ansi.Color{"M": ansi.BasicColor(1), "file.txt": nil, "[L]": ansi.BasicColor(3)}},
		{"staged", fileItem{label: "file.txt", staged: true, change: lore.FileChange{Status: 'A'}, locked: true, lockedByMe: true}, true, map[string]ansi.Color{"A": ansi.BasicColor(2), "file.txt": ansi.BasicColor(2), "[L]": ansi.BasicColor(2)}},
		{"directory", fileItem{label: "folder", isDir: true}, true, map[string]ansi.Color{"folder": nil}},
		{"staged directory", fileItem{label: "folder", isDir: true, allStaged: true}, true, map[string]ansi.Color{"folder": ansi.BasicColor(2)}},
		{"branch", branchItem{branch: lore.Branch{Name: "branch", Created: time.Now().UnixMilli()}}, false, map[string]ansi.Color{"1h": ansi.BasicColor(6), "branch": nil}},
		{"current branch", branchItem{branch: lore.Branch{Name: "branch", Current: true}}, false, map[string]ansi.Color{"*": ansi.BasicColor(2), "branch": nil}},
		{"pushed revision", revisionItem{revision: lore.Revision{Hash: "12345678", Author: "author", Message: "message"}}, false, map[string]ansi.Color{"12345678": ansi.BasicColor(2), "author": ansi.BasicColor(5), "message": nil}},
		{"unpushed revision", revisionItem{revision: lore.Revision{Hash: "12345678", Author: "author", Message: "message"}, unpushed: true}, false, map[string]ansi.Color{"12345678": ansi.BasicColor(1), "author": ansi.BasicColor(5), "message": nil}},
		{"truncated file", fileItem{label: strings.Repeat("f", 80), change: lore.FileChange{Status: 'M'}}, true, map[string]ansi.Color{"M": ansi.BasicColor(1), "fff": nil}},
		{"truncated branch", branchItem{branch: lore.Branch{Name: strings.Repeat("b", 80)}}, false, map[string]ansi.Color{"bbb": nil}},
	}
	for _, tc := range tests {
		for _, focused := range []bool{false, true} {
			for _, cursor := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/focused=%v/cursor=%v", tc.name, focused, cursor), func(t *testing.T) {
					var delegate list.ItemDelegate = compactTitleDelegate{focused: focused, width: 42}
					if tc.file {
						delegate = fileDelegate{focused: focused}
					}
					model := list.New([]list.Item{tc.item, tc.item}, delegate, 42, 5)
					index := 1
					if cursor {
						index = 0
					}
					var output bytes.Buffer
					delegate.Render(&output, model, index, tc.item)
					plain := ansi.Strip(output.String())
					cells := cellbuf.NewBuffer(lipgloss.Width(output.String()), 1)
					cellbuf.SetContent(cells, output.String())
					for x := 0; x < cells.Width(); x++ {
						cell := cells.Cell(x, 0)
						if cell == nil || cell.Width == 0 {
							continue
						}
						wantBold := cursor || tc.name == "current branch" && x < 2
						if gotBold := cell.Style.Attrs&cellbuf.BoldAttr != 0; gotBold != wantBold {
							t.Errorf("column %d (%q) bold = %v, want %v: %q", x, cell.String(), gotBold, wantBold, output.String())
						}
						var wantBG color.Color
						if cursor && focused {
							wantBG = background.Cell(0, 0).Style.Bg
						}
						if !sameSelectionColor(cell.Style.Bg, wantBG) {
							t.Errorf("column %d (%q) background = %v, want %v", x, cell.String(), cell.Style.Bg, wantBG)
						}
					}
					for text, fg := range tc.fg {
						byteIndex := strings.Index(plain, text)
						if byteIndex < 0 {
							t.Fatalf("row is missing %q: %q", text, output.String())
						}
						x := ansi.StringWidth(plain[:byteIndex])
						for offset := 0; offset < ansi.StringWidth(text); offset++ {
							if got := cells.Cell(x+offset, 0).Style.Fg; !sameSelectionColor(got, fg) {
								t.Errorf("%q column %d foreground = %v, want %v", text, offset, got, fg)
							}
						}
					}
				})
			}
		}
	}
}

func sameSelectionColor(got, want color.Color) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	gr, gg, gb, ga := got.RGBA()
	wr, wg, wb, wa := want.RGBA()
	return gr == wr && gg == wg && gb == wb && ga == wa
}
