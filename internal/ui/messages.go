package ui

import (
	"time"

	"lazylore/internal/lore"
)

type statusMsg struct {
	status lore.Status
	err    error
}

type branchesMsg struct {
	branches []lore.Branch
	err      error
}

type historyMsg struct {
	revisions []lore.Revision
	err       error
}

type locksMsg struct {
	locks []lore.Lock
	err   error
}

// diffMsg carries text for the shared main content panel - a file diff
// (Files panel), a branch log (Branches panel), or a revision patch
// (History panel). raw skips diff coloring for content that isn't actually
// diff/patch text (a log listing), see diffModel.SetContentRaw.
type diffMsg struct {
	text string
	err  error
	raw  bool
}

// actionDoneMsg reports a background lore command's result. opKey and
// revert are only set by actions that flipped something in the UI
// optimistically (see model.go's setFileStagedByPath/setFileLockedByPath)
// before this message arrived: opKey (if non-empty) is cleared from
// pendingFileOps regardless of outcome, and revert (if non-nil) is run to
// undo the optimistic flip when err != nil - a real refresh already
// reconciles the success case, but a failure never triggers one, so the
// optimistic guess has to be walked back by hand.
type actionDoneMsg struct {
	label  string
	err    error
	opKey  string
	revert func(*Model)
}

type editorDoneMsg struct {
	err error
}

type tickMsg time.Time

type setAppStatusMsg string
