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

type actionDoneMsg struct {
	label string
	err   error
}

type editorDoneMsg struct {
	err error
}

type tickMsg time.Time

type setAppStatusMsg string
