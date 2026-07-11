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

type diffMsg struct {
	text string
	err  error
}

type actionDoneMsg struct {
	label string
	err   error
}

type tickMsg time.Time

type setAppStatusMsg string
