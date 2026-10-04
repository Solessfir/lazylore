package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/lore"
)

type statusMsg struct {
	statusGeneration int
	generation       uint64
	status           lore.Status
	err              error
}

type branchesMsg struct {
	statusGeneration int
	generation       uint64
	branches         []lore.Branch
	err              error
}

type historyMsg struct {
	statusGeneration int
	generation       uint64
	revisions        []lore.Revision
	err              error
}

type locksMsg struct {
	requestID uint64
	locks     []lore.Lock
	err       error
}

type currentUserMsg struct {
	id  string
	err error
}

type filterMatchesMsg struct {
	source     focusPanel
	generation uint64
	matches    list.FilterMatchesMsg
}

type mainContentRequest struct {
	id     uint64
	source focusPanel
	target string
}

// diffMsg carries text for the shared main content panel - a file diff
// (Files panel), a branch log (Branches panel), or a revision patch
// (History panel). raw skips diff coloring for content that isn't actually
// diff/patch text (a log listing), see diffModel.SetContentRaw.
type diffMsg struct {
	statusGeneration int
	request          mainContentRequest
	text             string
	err              error
	raw              bool
}

// actionDoneMsg reports a background lore command's result. opKey, revert,
// and confirm are only set by actions that flipped something in the UI
// optimistically before this message arrived: opKey (if non-empty) is
// cleared from pendingFileOps regardless of outcome. On failure, revert (if
// non-nil) undoes the optimistic flip. On success, confirm (if non-nil)
// updates any Model-level cache the optimistic flip didn't touch (e.g.
// m.locks), so a triggered refresh doesn't rebuild from a stale cache.
type actionDoneMsg struct {
	statusGeneration int
	label            string
	err              error
	opKey            string
	revert           func(*Model)
	confirm          func(*Model)
	commands         []string // actual lore command line(s) run, for the Command Log (see commandlog.go)
	// liveStreamed marks this as the terminal message of a pushStreamCmd -
	// its Command Log entry was already started via BeginLive and needs
	// FinishLive, not a second AppendAction (see logResult in model.go).
	liveStreamed   bool
	syncGeneration int
}

// pushLineMsg is one live progress line from pushStreamCmd (see
// lore.FormatPushEventLine), appended to the Command Log as it arrives.
type pushLineMsg string

// pushChanMsg wraps one message read off a streaming push's channel
// (pushLineMsg or the terminal actionDoneMsg), plus the channel itself so
// Update can re-issue the read for the next message.
type pushChanMsg struct {
	ch    chan tea.Msg
	inner tea.Msg
}

type editorDoneMsg struct {
	err error
}

type tickMsg time.Time

type setAppStatusMsg struct {
	gen  int
	text string
}

// revealStatusMsg is setAppStatusMsg's delayed reveal (see
// statusRevealDelay in model.go) - it only actually shows the spinner if
// gen still matches Model.statusGen when it fires, i.e. the action it was
// scheduled for hasn't finished yet.
type revealStatusMsg struct {
	gen  int
	text string
}

// revealSyncMsg is branchSyncLabel's delayed reveal, same
// statusRevealDelay/generation pattern as revealStatusMsg - a pull/push that
// finishes before the delay elapses never flashes the inline spinner.
type revealSyncMsg struct {
	gen   int
	label string
}
