package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazylore/internal/lore"
)

func TestModel_ViewShowsContextualMainPanelTitle(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 155, Height: 40})
	m2 := updated.(Model)
	updated, _ = m2.Update(statusMsg{status: lore.Status{Unstaged: []lore.FileChange{{Status: 'M', Path: "a.txt"}}}})
	m3 := updated.(Model)

	if v := m3.View(); !strings.Contains(v, "Unstaged changes") {
		t.Fatalf("View() with Files focused on an unstaged file must show 'Unstaged changes'; got:\n%s", v)
	}

	updated, _ = m3.Update(branchesMsg{branches: []lore.Branch{{Name: "main", Current: true}}})
	m4 := updated.(Model)
	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m5 := updated.(Model)
	if v := m5.View(); !strings.Contains(v, "Log") {
		t.Fatalf("View() with Branches focused must show 'Log'; got:\n%s", v)
	}

	updated, _ = m5.Update(historyMsg{revisions: []lore.Revision{{Number: 1, Hash: "abc"}}})
	m6 := updated.(Model)
	updated, _ = m6.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m7 := updated.(Model)
	if v := m7.View(); !strings.Contains(v, "Patch") {
		t.Fatalf("View() with History focused must show 'Patch'; got:\n%s", v)
	}
}

func TestFormatBranchLog_RendersOneLinePerRevision(t *testing.T) {
	revisions := []lore.Revision{
		{Hash: "abcdef1234567890", Author: "dev@example.com", Message: "second"},
		{Hash: "0123456789abcdef", Message: "first"}, // no author
	}
	got := formatBranchLog(revisions)
	if !strings.Contains(got, "second") || !strings.Contains(got, "first") {
		t.Fatalf("formatBranchLog missing a message: %q", got)
	}
	if !strings.Contains(got, "dev@example.com") || !strings.Contains(got, "unknown") {
		t.Fatalf("formatBranchLog missing an author (real or 'unknown' fallback): %q", got)
	}
}

func TestFormatBranchLog_EmptyList(t *testing.T) {
	if got := formatBranchLog(nil); got != "No revisions." {
		t.Fatalf("formatBranchLog(nil) = %q, want %q", got, "No revisions.")
	}
}

func TestAheadBehindArrows_MatchesLazygitsFormat(t *testing.T) {
	// Ground truth: pkg/gui/presentation/branches.go's BranchStatus -
	// "↓N↑N" both, "↓N" behind only, "↑N" ahead only, "" in sync.
	cases := []struct {
		ahead, behind int
		want          string
	}{
		{0, 0, ""},
		{3, 0, "↑3"},
		{0, 5, "↓5"},
		{2, 4, "↓4↑2"},
	}
	for _, c := range cases {
		got := strings.TrimSpace(aheadBehindArrows(c.ahead, c.behind))
		if !strings.Contains(got, c.want) || (c.want == "" && got != "") {
			t.Fatalf("aheadBehindArrows(%d, %d) = %q, want to contain %q", c.ahead, c.behind, got, c.want)
		}
	}
}

func TestModel_StatusTextShowsAheadBehindArrows(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	updated, _ = m2.Update(statusMsg{status: lore.Status{Branch: "main", AheadCount: 2, BehindCount: 1}})
	m3 := updated.(Model)

	v := m3.View()
	if !strings.Contains(v, "↓1↑2") {
		t.Fatalf("View() must show the ahead/behind arrows; got:\n%s", v)
	}
}

func TestModel_StatusTextHasNoArrowsWhenInSync(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	updated, _ = m2.Update(statusMsg{status: lore.Status{Branch: "main"}})
	m3 := updated.(Model)

	v := m3.View()
	if strings.Contains(v, "↓") || strings.Contains(v, "↑") {
		t.Fatalf("View() must show no arrows when AheadCount/BehindCount are both 0; got:\n%s", v)
	}
}

func TestBracketedKey_WrapsNamedKeysNotLiterals(t *testing.T) {
	if got := bracketedKey("space"); got != "<space>" {
		t.Fatalf("bracketedKey(space) = %q, want <space>", got)
	}
	if got := bracketedKey("enter"); got != "<enter>" {
		t.Fatalf("bracketedKey(enter) = %q, want <enter>", got)
	}
	if got := bracketedKey("d"); got != "d" {
		t.Fatalf("bracketedKey(d) = %q, want bare d (no brackets on literal keys)", got)
	}
}

func TestKeybindBarFor_FilesMatchesLazygitsDisplayOnScreenSet(t *testing.T) {
	// Ground truth: pkg/gui/controllers/files_controller.go's DisplayOnScreen:
	// true bindings, in registration order (Select/space, CommitChanges/c,
	// Edit/e, Remove/d, ViewResetOptions/D) - Stash dropped since lore has
	// none, Diff/enter is lazylore's own file-diff-load action, Lock/L is
	// lore-native (git/lazygit have no locking concept).
	got := keybindBarFor(focusFiles)
	want := "Stage: <space> | Commit: c | Edit: e | Diff: <enter> | Discard: d | Reset: D | Lock: L"
	if got != want {
		t.Fatalf("keybindBarFor(focusFiles) = %q, want %q", got, want)
	}
}

func TestKeybindBarFor_BranchesMatchesLazygitsSupportedSubset(t *testing.T) {
	// Ground truth: branches_controller.go's DisplayOnScreen set is
	// Checkout(space)/New(n)/Delete(d)/Rebase(r)/Merge(M)/Reset(g)/Upstream(u).
	// lore has no delete/rebase/merge/upstream equivalent (checked
	// lore-cli-commands.md), so only Checkout/New/Reset carry over.
	got := keybindBarFor(focusBranches)
	want := "Checkout: <space> | New branch: n | Reset: g"
	if got != want {
		t.Fatalf("keybindBarFor(focusBranches) = %q, want %q", got, want)
	}
}

func TestKeybindBarFor_HistoryMatchesLazygitsSupportedSubset(t *testing.T) {
	// Ground truth: local_commits_controller.go's Commits DisplayOnScreen set
	// includes Squash/Fixup/Reword/Drop/Edit/Amend, none of which lore
	// supports (no rebase-style history rewrite) - only Checkout(space, via
	// `lore sync`) and Reset(g, via `lore branch reset`) carry over.
	got := keybindBarFor(focusHistory)
	want := "Checkout: <space> | Reset: g"
	if got != want {
		t.Fatalf("keybindBarFor(focusHistory) = %q, want %q", got, want)
	}
}

func TestKeybindBarFor_DiffHasNoBoundActionsYet(t *testing.T) {
	if got := keybindBarFor(focusDiff); got != "" {
		t.Fatalf("keybindBarFor(focusDiff) = %q, want empty (Diff panel is scroll-only)", got)
	}
}

func TestModel_KeybindBarIsContextualNotGlobalNavOrQuit(t *testing.T) {
	// Regression: the bar used to be one static string always advertising
	// Focus/Quit/Select-copy alongside file actions, unlike lazygit's own
	// per-context bottom bar (which never advertises navigation or quit -
	// see pkg/gui/controllers/global_controller.go's DisplayOnScreen set:
	// only Cancel and the "?" keybindings menu, neither of which lazylore
	// implements). Those keys still work; they're just not advertised.
	m := NewModel(&lore.FakeRunner{}, "test-repo", "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(Model)
	v := m2.View()

	if !strings.Contains(v, "Stage: <space>") {
		t.Fatalf("Files-focused View() must show the Files keybind bar; got:\n%s", v)
	}
	for _, unwanted := range []string{"Focus: tab", "Quit: q", "Select/copy"} {
		if strings.Contains(v, unwanted) {
			t.Fatalf("View() must not advertise global nav/quit (%q), matching lazygit's own bar; got:\n%s", unwanted, v)
		}
	}
}

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
