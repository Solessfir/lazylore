package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/solessfir/lazylore/internal/lore"
)

// commandResult executes tracked work without running the model's timer chain.
func commandResult(cmd tea.Cmd) tea.Msg {
	msg := cmd()
	switch msg := msg.(type) {
	case activityStartMsg:
		return commandResult(msg.cmd)
	case activityResultMsg:
		return msg.inner
	default:
		return msg
	}
}

func startTestActivity(t *testing.T, m *Model, cmd tea.Cmd) activityStartMsg {
	t.Helper()
	start, ok := cmd().(activityStartMsg)
	if !ok {
		t.Fatal("expected an activity start")
	}
	updated, _ := m.Update(start)
	*m = updated.(Model)
	return start
}

func TestActivity_StartPrecedesNativeWorkAndCompletion(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	calls := 0
	cmd := m.activityCmd("Staging", func() tea.Msg {
		calls++
		return actionDoneMsg{opKey: "stage:a.txt"}
	})
	if m.activityName() != "" || calls != 0 {
		t.Fatal("scheduling ran work or registered an activity before execution")
	}
	start := startTestActivity(t, &m, cmd)
	if m.activityName() != "Staging" || calls != 0 || !m.activityTickPending {
		t.Fatal("start did not register before scheduling native work and the timer")
	}
	m.setPendingFileOp("stage:b.txt", true) // Keep the completion from scheduling a refresh.
	updated, _ := m.Update(start.cmd())
	m = updated.(Model)
	if calls != 1 || m.activityName() != "" || m.activityTickPending {
		t.Fatalf("completed activity survived: calls=%d active=%v pending=%v", calls, m.activities, m.activityTickPending)
	}
}

func TestActivity_KeyActionsAllocateDistinctIDsAndRespectPushGuard(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	var previous uint64 = 4
	for _, key := range []string{"p", "p", "P"} {
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = updated.(Model)
		start := startTestActivity(t, &m, cmd)
		if start.id <= previous || m.nextActivityID != start.id {
			t.Fatal("model update lost the allocated activity ID")
		}
		previous = start.id
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m = updated.(Model)
	if cmd != nil || m.nextActivityID != previous || len(m.activities) != 3 {
		t.Fatal("blocked second push allocated work or another timer")
	}
}

func TestActivity_BackgroundResultStillProcessesDuringModalAndResize(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	start := startTestActivity(t, &m, m.activityCmd("Loading history", func() tea.Msg {
		return historyMsg{revisions: []lore.Revision{{Hash: "loaded"}}}
	}))
	m.prompt = promptNewBranch
	m.showHelp = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, _ = m.Update(start.cmd())
	m = updated.(Model)
	if m.activityName() != "" || len(m.revisions) != 1 || m.prompt != promptNewBranch || !m.showHelp || m.width != 120 {
		t.Fatal("modal or resize blocked background completion")
	}
}

func TestActivity_OverlappingLoadsFinishIndependently(t *testing.T) {
	for _, failOlder := range []bool{false, true} {
		m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
		m.width, m.height = 120, 40
		m.resize()
		m.mainContentRequestID = 2
		var olderErr error
		if failOlder {
			olderErr = errors.New("old load failed")
		}
		older := startTestActivity(t, &m, m.activityCmd("Loading diff", func() tea.Msg {
			return diffMsg{request: mainContentRequest{id: 1}, err: olderErr, text: "stale"}
		}))
		newer := startTestActivity(t, &m, m.activityCmd("Loading branch log", func() tea.Msg {
			return diffMsg{request: mainContentRequest{id: 2}, text: "new", raw: true}
		}))
		updated, _ := m.Update(older.cmd())
		m = updated.(Model)
		if len(m.activities) != 1 || m.activityName() != "Loading branch log" || m.err != nil {
			t.Fatal("stale older completion changed the newer activity or content error")
		}
		updated, cmd := m.Update(newer.cmd())
		m = updated.(Model)
		if len(m.activities) != 0 || m.activityTickPending || cmd != nil || !strings.Contains(m.diff.vp.View(), "new") {
			t.Fatal("final completion retained progress or failed to apply content")
		}
	}
}

func TestActivity_NewestCompletionRevealsOlderActiveLoad(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	older := startTestActivity(t, &m, m.activityCmd("Loading status", func() tea.Msg {
		return statusMsg{generation: 9}
	}))
	newer := startTestActivity(t, &m, m.activityCmd("Loading locks", func() tea.Msg {
		return locksMsg{err: errors.New("offline")}
	}))
	updated, _ := m.Update(newer.cmd())
	m = updated.(Model)
	if m.activityName() != "Loading status" || len(m.activities) != 1 {
		t.Fatal("newest completion hid an older active load")
	}
	updated, cmd := m.Update(older.cmd())
	m = updated.(Model)
	if m.activityName() != "" || cmd != nil {
		t.Fatal("stale status payload stranded its activity")
	}
}

func TestActivity_OneTimerChainStopsAndIgnoresStaleTicks(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	start := startTestActivity(t, &m, m.activityCmd("Loading diff", func() tea.Msg { return diffMsg{} }))
	generation := m.activityTickGeneration
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	if cmd != nil || m.activityTickGeneration != generation {
		t.Fatal("resize scheduled a second timer chain")
	}
	for i := 1; i <= 4; i++ {
		updated, cmd = m.Update(activityTickMsg{generation: m.activityTickGeneration})
		m = updated.(Model)
		if cmd == nil || !m.activityTickPending || m.activityFrame != i%4 {
			t.Fatalf("tick %d: frame=%d pending=%v cmd=%v", i, m.activityFrame, m.activityTickPending, cmd)
		}
	}
	stale := activityTickMsg{generation: m.activityTickGeneration}
	updated, _ = m.Update(start.cmd())
	m = updated.(Model)
	updated, cmd = m.Update(stale)
	m = updated.(Model)
	if cmd != nil || m.activityTickPending || m.activityFrame != 0 {
		t.Fatal("idle tick revived animation")
	}
	startTestActivity(t, &m, m.activityCmd("Loading history", func() tea.Msg { return historyMsg{} }))
	generation = m.activityTickGeneration
	updated, cmd = m.Update(stale)
	m = updated.(Model)
	if cmd != nil || m.activityTickGeneration != generation || m.activityFrame != 0 {
		t.Fatal("old timer changed or duplicated a new activity's timer")
	}
	if activityTickInterval != 180*time.Millisecond {
		t.Fatalf("tick interval = %v", activityTickInterval)
	}
}

func TestActivity_InitIsLazyAndTracksEveryInitialLoad(t *testing.T) {
	fake := &lore.FakeRunner{}
	m := NewModel(fake, "repo", "/repo")
	initial := m.Init()().(tea.BatchMsg)
	if len(initial) != 4 || len(fake.Calls) != 0 || len(m.activities) != 0 || m.activityTickPending {
		t.Fatal("Init executed work, mutated the model, or started an idle timer")
	}
	var starts []activityStartMsg
	for _, cmd := range initial {
		starts = append(starts, startTestActivity(t, &m, cmd))
	}
	if len(m.activities) != 4 || m.activityName() != "Loading current user" || m.activityTickGeneration != 1 {
		t.Fatal("initial loads did not share one timer chain")
	}
	m.refreshGeneration++ // Supersede all initial content before responses arrive.
	for _, start := range starts {
		updated, _ := m.Update(start.cmd())
		m = updated.(Model)
	}
	if len(m.activities) != 0 || m.activityTickPending {
		t.Fatal("initial completions left stale progress")
	}
	start := m.activityCmd("Next action", func() tea.Msg { return nil })().(activityStartMsg)
	if start.id <= 4 {
		t.Fatal("new activity reused an initial load ID")
	}
}

func TestActivity_PushProgressKeepsIDUntilTerminalResult(t *testing.T) {
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	ch := make(chan tea.Msg, 2)
	ch <- pushLineMsg("Uploading 2/4")
	ch <- actionDoneMsg{label: "Push", liveStreamed: true}
	close(ch)
	m.pushInFlight = true
	m.log.BeginLive("Push")
	start := startTestActivity(t, &m, m.activityCmd("Pushing", readPushChan(ch)))
	updated, read := m.Update(start.cmd())
	m = updated.(Model)
	if m.activityName() != "Pushing" || !m.pushInFlight || read == nil {
		t.Fatal("progress line completed push activity")
	}
	terminal := read().(activityResultMsg)
	if terminal.id != start.id {
		t.Fatal("push continuation changed its activity ID")
	}
	updated, _ = m.Update(terminal)
	m = updated.(Model)
	if m.activityName() != "" || m.pushInFlight {
		t.Fatal("terminal push result retained activity or the push guard")
	}
}

func TestActivity_PushConstructionDoesNotStartRunner(t *testing.T) {
	fake := &lore.FakeRunner{Results: map[string]lore.Result{"--json push": {Stdout: jsonCompleteSuccess}}}
	cmd := pushStreamCmd(fake)
	if len(fake.Calls) != 0 {
		t.Fatal("constructing push command ran native work")
	}
	msg := cmd().(pushChanMsg)
	if _, ok := msg.inner.(actionDoneMsg); !ok || len(fake.Calls) != 1 {
		t.Fatal("executing push did not return its terminal result")
	}
}

func TestActivity_EditorSetupFailureCompletesTracking(t *testing.T) {
	t.Setenv("VISUAL", "'unterminated")
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	start := startTestActivity(t, &m, m.editActivityCmd("/repo/file.txt"))
	result := start.cmd().(activityResultMsg)
	if result.id != start.id || result.inner.(editorDoneMsg).err == nil {
		t.Fatal("editor setup failure did not return a tracked completion")
	}
	updated, cmd := m.Update(result)
	m = updated.(Model)
	if m.activityName() != "" || cmd != nil || m.err == nil {
		t.Fatal("editor setup failure left activity running or lost its error")
	}
}

func TestActivity_EditorHandoffKeepsTrackingUntilCallback(t *testing.T) {
	t.Setenv("VISUAL", "test-editor")
	m := NewModel(&lore.FakeRunner{}, "repo", "/repo")
	start := startTestActivity(t, &m, m.editActivityCmd("/repo/file.txt"))
	handoff := start.cmd()
	if _, wrapped := handoff.(activityResultMsg); wrapped {
		t.Fatal("editor handoff was wrapped before reaching Bubble Tea")
	}
	updated, _ := m.Update(handoff)
	m = updated.(Model)
	if m.activityName() != "Editing file" {
		t.Fatal("editor handoff completed tracking before its callback")
	}
	updated, _ = m.Update(activityResultMsg{id: start.id, inner: editorDoneMsg{}})
	m = updated.(Model)
	if m.activityName() != "" {
		t.Fatal("editor callback left activity running")
	}
}
