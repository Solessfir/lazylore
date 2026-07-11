package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazylore/internal/lore"
)

func TestRenderPanel_MatchesRequestedSizeRegardlessOfContentLength(t *testing.T) {
	const width, height = 30, 5

	cases := map[string]string{
		"short content": "x",
		"long content":  "a very very very long line of content that is much wider than thirty columns",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			out := renderPanel(false, width, height, "Title", content)
			lines := strings.Split(out, "\n")

			wantLines := height + borderHeight
			if len(lines) != wantLines {
				t.Fatalf("line count = %d, want %d; output:\n%s", len(lines), wantLines, out)
			}
			for i, line := range lines {
				wantWidth := width + borderWidth
				if got := lipgloss.Width(line); got != wantWidth {
					t.Fatalf("line %d width = %d, want %d; line: %q", i, got, wantWidth, line)
				}
			}
		})
	}
}

func TestRenderPanel_DifferentContentProducesSameSize(t *testing.T) {
	// This is the exact bug: two panels with different content (e.g. Files
	// with two long entries vs Branches with one short one) rendering at
	// different widths/heights instead of the same requested size.
	a := renderPanel(false, 20, 4, "Files", "M a.go\nM b.txt")
	b := renderPanel(false, 20, 4, "Branches", "* main")

	linesA := strings.Split(a, "\n")
	linesB := strings.Split(b, "\n")
	if len(linesA) != len(linesB) {
		t.Fatalf("line counts differ: %d vs %d", len(linesA), len(linesB))
	}
	for i := range linesA {
		wA, wB := lipgloss.Width(linesA[i]), lipgloss.Width(linesB[i])
		if wA != wB {
			t.Fatalf("line %d width differs: %d vs %d", i, wA, wB)
		}
	}
}

func TestRenderTitledPanel_TitleLivesInBorderNotContent(t *testing.T) {
	// Titles must appear in the top border line (lazygit style), not as a
	// separate interior row. This is the main visual fix vs original.
	const w, h = 25, 6 // w,h here are the lipgloss target (inner) sizes
	out := renderTitledPanel(false, w, h, "", "Files", "M foo.txt\nM bar.go")
	lines := strings.Split(out, "\n")

	// Outer dimensions must still be stable.
	if len(lines) != h+borderHeight {
		t.Fatalf("outer lines = %d, want %d", len(lines), h+borderHeight)
	}
	for _, ln := range lines {
		if lipgloss.Width(ln) != w+borderWidth {
			t.Fatalf("line width mismatch; got %d want %d", lipgloss.Width(ln), w+borderWidth)
		}
	}

	top := lines[0]
	if !strings.Contains(top, "Files") {
		t.Fatalf("top border must contain title 'Files', got: %q", top)
	}
	// The title should be inside the border runes, not a content line.
	if strings.HasPrefix(strings.TrimSpace(top), "Files") {
		t.Fatalf("title should not be bare content; it must be embedded in border: %q", top)
	}

	// Second line should be actual content (first data row), not another title.
	second := strings.TrimSpace(lines[1])
	if strings.Contains(second, "Files") {
		t.Fatalf("second line should be data, not contain title: %q", lines[1])
	}
}

func TestRenderTitledPanel_StatusCompact(t *testing.T) {
	out := renderTitledPanelForStatus(20, 1, "myrepo → main")
	lines := strings.Split(out, "\n")
	// status outer h = 1 (content) + 2 borders = 3
	if len(lines) != 3 {
		t.Fatalf("status lines = %d, want 3", len(lines))
	}
	if !strings.Contains(lines[0], "Status") {
		t.Fatalf("status top border must contain 'Status': %q", lines[0])
	}
}

func TestModel_StatusTextShowsBranchNotDuplicatedRepoName(t *testing.T) {
	// Regression: statusText used to render the repo name twice
	// ("repo(repo) → branch") instead of showing the branch name.
	m := NewModel(&lore.FakeRunner{}, "myrepo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	updated, _ = m2.Update(statusMsg{status: lore.Status{Branch: "feature-x"}})
	m3 := updated.(Model)

	v := m3.View()
	if !strings.Contains(v, "myrepo (feature-x)") {
		t.Fatalf("View must show 'myrepo (feature-x)'; got:\n%s", v)
	}
	if strings.Contains(v, "myrepo(myrepo)") {
		t.Fatalf("View must not duplicate the repo name; got:\n%s", v)
	}
}
