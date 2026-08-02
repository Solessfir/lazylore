package lore

import (
	"encoding/json"
	"fmt"
)

// Lock is one file lock reported by `lore --json lock status`.
type Lock struct {
	Path     string
	Owner    string
	LockedAt int64 // epoch milliseconds
}

// lockFileStatusData mirrors LoreLockFileStatusEventData
// (lore-revision/src/lock/file/status.rs).
type lockFileStatusData struct {
	Path     string `json:"path"`
	Owner    string `json:"owner"`
	LockedAt int64  `json:"lockedAt"`
}

// LockStatus reports which of paths are currently locked. Paths with no
// lock simply don't appear in the result. Requires an online remote (lore
// locks are server-tracked); returns (nil, nil) for an empty path list
// rather than making an invalid zero-path call.
func LockStatus(r Runner, paths ...string) ([]Lock, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	var locks []Lock
	for _, batch := range pathArgumentBatches([]string{"lock", "status"}, paths) {
		res, err := runChecked(r, append([]string{"lock", "status"}, batch...)...)
		if err != nil {
			return nil, err
		}
		events, err := parseEvents(res.Stdout)
		if err != nil {
			return nil, fmt.Errorf("lore lock status: %w", err)
		}
		for _, e := range events {
			if e.TagName != "lockFileStatus" {
				continue
			}
			var data lockFileStatusData
			if err := json.Unmarshal(e.Data, &data); err != nil {
				return nil, fmt.Errorf("parsing lockFileStatus event: %w", err)
			}
			locks = append(locks, Lock{Path: data.Path, Owner: data.Owner, LockedAt: data.LockedAt})
		}
	}

	return locks, nil
}

// LockAcquire locks path for the current user.
func LockAcquire(r Runner, path string) (Result, error) {
	return runChecked(r, "lock", "acquire", path)
}

// LockRelease releases the current user's lock on path.
func LockRelease(r Runner, path string) (Result, error) {
	return runChecked(r, "lock", "release", path)
}

// LockReleaseForce releases path's lock regardless of who holds it,
// bypassing the usual ownership check. Requires a lore build with the
// AdminUnlock capability (see lazylore's memory/project notes on the
// lock-owner TODO); against a stock lore build this just fails the same
// way a plain LockRelease on someone else's lock always has, surfaced
// through the normal error path rather than anything special-cased here.
func LockReleaseForce(r Runner, path string) (Result, error) {
	return runChecked(r, "lock", "release", "--force", path)
}
