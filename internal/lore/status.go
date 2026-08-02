package lore

import (
	"encoding/json"
	"fmt"
)

// FileChange is one changed file or directory from a lore status listing.
type FileChange struct {
	Status    byte // 'A' add, 'M' modify, 'D' delete, 'C' copy - see fileActionDisplayByte
	Path      string
	Directory bool
}

// Status is the parsed result of `lore --json status --scan`.
type Status struct {
	Repository string
	Branch     string
	Staged     []FileChange
	Unstaged   []FileChange

	// AheadCount/BehindCount are how many revisions the local branch leads/
	// trails its remote counterpart by. Both zero means either in sync or
	// there's no meaningful remote comparison available (offline, no
	// remote configured, branch not pushed yet, etc).
	AheadCount  int
	BehindCount int

	// RemoteRevisionNumber is the remote branch's latest revision number (0
	// if the branch has never been pushed - every local revision is then
	// correctly "ahead of" it). Only meaningful when HasRemoteInfo is true;
	// a stale/zero value while offline or unauthorized would otherwise look
	// identical to "never pushed" and wrongly mark everything unpushed.
	RemoteRevisionNumber uint64
	HasRemoteInfo        bool
}

// repositoryStatusRevisionData mirrors LoreRepositoryStatusRevisionEventData
// (lore-revision/src/repository/status.rs) - only the fields Status uses.
// Unlike repositoryStatusFileData's flags below, these serialize as raw 0/1
// numbers on the wire rather than JSON booleans (confirmed against a real
// captured `lore --json status` fixture), so they're plain ints here.
type repositoryStatusRevisionData struct {
	Repository           string `json:"repository"`
	BranchName           string `json:"branchName"`
	RevisionLocalNumber  uint64 `json:"revisionLocalNumber"`
	RevisionRemoteNumber uint64 `json:"revisionRemoteNumber"`
	IsLocalAhead         int    `json:"isLocalAhead"`
	IsRemoteAhead        int    `json:"isRemoteAhead"`
	RemoteAvailable      int    `json:"remoteAvailable"`
	RemoteAuthorized     int    `json:"remoteAuthorized"`
	RemoteBranchExist    int    `json:"remoteBranchExist"`
}

// repositoryStatusFileData mirrors LoreRepositoryStatusFileEventData
// (same file) - only the fields Status uses.
type repositoryStatusFileData struct {
	Path       string `json:"path"`
	Action     string `json:"action"`
	Type       string `json:"type"` // "directory", "file", or "link" (LoreNodeType, lore-revision/src/interface.rs)
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
			if data.RemoteAvailable != 0 && data.RemoteAuthorized != 0 {
				s.HasRemoteInfo = true
				s.RemoteRevisionNumber = data.RevisionRemoteNumber
				if data.RemoteBranchExist != 0 {
					if data.IsLocalAhead != 0 && data.RevisionLocalNumber > data.RevisionRemoteNumber {
						s.AheadCount = int(data.RevisionLocalNumber - data.RevisionRemoteNumber)
					}
					if data.IsRemoteAhead != 0 && data.RevisionRemoteNumber > data.RevisionLocalNumber {
						s.BehindCount = int(data.RevisionRemoteNumber - data.RevisionLocalNumber)
					}
				}
			}
			sawRevision = true

		case "repositoryStatusFile":
			var data repositoryStatusFileData
			if err := json.Unmarshal(e.Data, &data); err != nil {
				return Status{}, fmt.Errorf("parsing repositoryStatusFile event: %w", err)
			}
			change := FileChange{
				Status:    fileActionDisplayByte(data.Action, data.FlagDirty),
				Path:      data.Path,
				Directory: data.Type == "directory",
			}
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
