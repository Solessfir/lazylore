package ui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/solessfir/lazylore/internal/lore"
)

func TestModel_PublicationsClampShrinkingSelectionAndPreserveValidIndex(t *testing.T) {
	for _, source := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		for _, filtered := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/filtered=%v", source, filtered), func(t *testing.T) {
				m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
				updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
				m = updated.(Model)
				m.focus = source
				publish := func(count int) tea.Cmd {
					var changes []lore.FileChange
					var branches []lore.Branch
					var revisions []lore.Revision
					for i := 0; i < count; i++ {
						name := fmt.Sprintf("match%02d", i)
						changes = append(changes, lore.FileChange{Status: 'M', Path: name})
						branches = append(branches, lore.Branch{Name: name})
						revisions = append(revisions, lore.Revision{Hash: name, Message: name})
					}
					msg := map[focusPanel]tea.Msg{
						focusFiles:    statusMsg{status: lore.Status{Unstaged: changes}},
						focusBranches: branchesMsg{branches: branches},
						focusHistory:  historyMsg{revisions: revisions},
					}[source]
					updated, cmd := m.Update(msg)
					m = updated.(Model)
					return cmd
				}
				publish(20)
				if filtered {
					m.panelList(source).FilterInput.Cursor.SetMode(cursor.CursorStatic)
					updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
					m = updated.(Model)
					updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("match")})
					m = updated.(Model)
					applyFilterTestCommand(&m, cmd)
					updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
					m = updated.(Model)
				}
				m.panelList(source).Select(len(m.panelList(source).VisibleItems()) - 1)
				cmd := publish(1)
				if filtered {
					if m.panelList(source).Index() == 0 {
						t.Fatal("publication clamped intermediate empty matches before filtering completed")
					}
					applyFilterTestCommand(&m, cmd)
				}
				if m.panelList(source).Index() != 0 || m.panelList(source).SelectedItem() == nil {
					t.Fatalf("shrink left invalid selection: index=%d item=%v", m.panelList(source).Index(), m.panelList(source).SelectedItem())
				}
				cmd = publish(20)
				if filtered {
					applyFilterTestCommand(&m, cmd)
				}
				m.panelList(source).Select(3)
				cmd = publish(20)
				if filtered {
					applyFilterTestCommand(&m, cmd)
				}
				if m.panelList(source).Index() != 3 {
					t.Fatalf("publication changed valid selected index to %d", m.panelList(source).Index())
				}
			})
		}
	}
}

func TestModel_AcceptingEmptyFilterKeepsRowActionsInactiveUntilEscape(t *testing.T) {
	for _, source := range []focusPanel{focusFiles, focusBranches, focusHistory} {
		m := newFilteredTestModel(source)
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		m = updated.(Model)
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ZZZZ_no_matches")})
		m = updated.(Model)
		applyFilterTestCommand(&m, cmd)
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(Model)
		if m.panelList(source).SettingFilter() || m.panelList(source).FilterValue() != "ZZZZ_no_matches" || len(m.panelList(source).VisibleItems()) != 0 {
			t.Fatalf("panel %v accepting zero matches cleared the filter", source)
		}
		for _, key := range []tea.KeyMsg{
			{Type: tea.KeySpace},
			{Type: tea.KeyRunes, Runes: []rune("d")},
		} {
			updated, cmd = m.Update(key)
			m = updated.(Model)
			if cmd != nil || m.prompt != promptNone || len(m.pendingFileOps) != 0 {
				t.Fatalf("panel %v empty filtered row action %q triggered a command or prompt", source, key)
			}
		}
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(Model)
		if m.panelList(source).FilterValue() != "" || len(m.panelList(source).VisibleItems()) == 0 {
			t.Fatalf("panel %v Escape did not explicitly restore rows", source)
		}
	}
}
