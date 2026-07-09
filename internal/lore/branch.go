package lore

import (
	"fmt"
	"strings"
)

// Branch is one entry from `lore branch list`.
type Branch struct {
	Name    string
	Current bool
	Remote  bool
}

func ParseBranchList(output string) ([]Branch, error) {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	var branches []Branch
	var remote bool
	sawHeader := false

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		if line == "" {
			continue
		}

		switch line {
		case "Local branches:":
			remote = false
			sawHeader = true
			continue
		case "Remote branches:":
			remote = true
			sawHeader = true
			continue
		}
		if !sawHeader {
			continue
		}

		switch {
		case strings.HasPrefix(line, "* "):
			branches = append(branches, Branch{Name: strings.TrimPrefix(line, "* "), Current: true, Remote: remote})
		case strings.HasPrefix(line, "  "):
			branches = append(branches, Branch{Name: strings.TrimPrefix(line, "  "), Remote: remote})
		default:
			return nil, fmt.Errorf("lore branch list: unrecognized line: %q", line)
		}
	}

	if !sawHeader {
		return nil, fmt.Errorf("lore branch list: no section headers found in output")
	}
	return branches, nil
}
