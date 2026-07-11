package lore

import (
	"encoding/json"
	"fmt"
)

// Branch is one entry from `lore --json branch list`.
type Branch struct {
	Name    string
	Current bool
	Remote  bool
	Latest  string
	Created int64
}

// branchListEntryData mirrors LoreBranchListEntryEventData
// (lore-revision/src/branch.rs) - only the fields Branch uses.
type branchListEntryData struct {
	Location  string `json:"location"` // "local" or "remote"
	Name      string `json:"name"`
	IsCurrent bool   `json:"isCurrent"`
	Latest    string `json:"latest"`
	Created   int64  `json:"created"`
}

// ParseBranchList reads `lore --json branch list` output: a
// branchListEntry event per branch, tagged with which location ("local" or
// "remote") it came from.
func ParseBranchList(output string) ([]Branch, error) {
	events, err := parseEvents(output)
	if err != nil {
		return nil, err
	}

	var branches []Branch
	for _, e := range events {
		if e.TagName != "branchListEntry" {
			continue
		}
		var data branchListEntryData
		if err := json.Unmarshal(e.Data, &data); err != nil {
			return nil, fmt.Errorf("parsing branchListEntry event: %w", err)
		}
		branches = append(branches, Branch{
			Name:    data.Name,
			Current: data.IsCurrent,
			Remote:  data.Location == "remote",
			Latest:  data.Latest,
			Created: data.Created,
		})
	}
	return branches, nil
}
