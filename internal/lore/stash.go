package lore

// Stash represents one stashed set of changes. This is a UI placeholder for
// lazygit parity ("Stash" panel below commits). lore itself has no native
// stash/shelve (per docs: use a branch to set work aside).
type Stash struct {
	Index   int
	Message string
}

// StashList returns stashes for the repo. Currently always empty.
func StashList(r Runner) ([]Stash, error) {
	return []Stash{}, nil
}
