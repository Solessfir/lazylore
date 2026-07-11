package lore

import (
	"encoding/json"
	"fmt"
)

// FileChange is one changed file from a lore status listing.
type FileChange struct {
	Status byte // 'A' add, 'M' modify, 'D' delete, 'C' copy - see fileActionDisplayByte
	Path   string
}

// Status is the parsed result of `lore --json status --scan`.
type Status struct {
	Repository string
	Branch     string
	Staged     []FileChange
	Unstaged   []FileChange
}

// repositoryStatusRevisionData mirrors LoreRepositoryStatusRevisionEventData
// (lore-revision/src/repository/status.rs) - only the fields Status uses.
type repositoryStatusRevisionData struct {
	Repository string `json:"repository"`
	BranchName string `json:"branchName"`
}

// repositoryStatusFileData mirrors LoreRepositoryStatusFileEventData
// (same file) - only the fields Status uses.
type repositoryStatusFileData struct {
	Path       string `json:"path"`
	Action     string `json:"action"`
	FlagStaged bool   `json:"flagStaged"`
	FlagDirty  bool   `json:"flagDirty"`
}

// ParseStatus reads `lore --json status --scan` output: a
// repositoryStatusRevision event for Repository/Branch, then zero or more
// repositoryStatusFile events (one per changed path), routed into
// Staged/Unstaged by FlagStaged.
func ParseStatus(output string) (Status, error) {
	events, err := parseEvents(output)
	if err != nil {
		return Status{}, err
	}

	var s Status
	sawRevision := false

	for _, e := range events {
		switch e.TagName {
		case "repositoryStatusRevision":
			var data repositoryStatusRevisionData
			if err := json.Unmarshal(e.Data, &data); err != nil {
				return Status{}, fmt.Errorf("parsing repositoryStatusRevision event: %w", err)
			}
			s.Repository = data.Repository
			s.Branch = data.BranchName
			sawRevision = true

		case "repositoryStatusFile":
			var data repositoryStatusFileData
			if err := json.Unmarshal(e.Data, &data); err != nil {
				return Status{}, fmt.Errorf("parsing repositoryStatusFile event: %w", err)
			}
			change := FileChange{Status: fileActionDisplayByte(data.Action, data.FlagDirty), Path: data.Path}
			if data.FlagStaged {
				s.Staged = append(s.Staged, change)
			} else {
				s.Unstaged = append(s.Unstaged, change)
			}
		}
	}

	if !sawRevision {
		return Status{}, fmt.Errorf("lore status: no repositoryStatusRevision event found in output")
	}
	return s, nil
}

// fileActionDisplayByte maps lore's LoreFileAction ("add"/"delete"/"move"/
// "copy"/"keep" - confirmed at lore-revision/src/interface.rs) to a single
// git-style display letter. "keep" with FlagDirty set means the file's
// identity didn't change but its content did, i.e. a modification; "keep"
// without FlagDirty (an unchanged file) shouldn't be reported by status at
// all, but falls back to a blank marker rather than erroring if it ever is.
func fileActionDisplayByte(action string, dirty bool) byte {
	switch action {
	case "add":
		return 'A'
	case "delete":
		return 'D'
	case "move":
		return 'M'
	case "copy":
		return 'C'
	default: // "keep"
		if dirty {
			return 'M'
		}
		return ' '
	}
}
