package lore

import (
	"fmt"
	"strings"
)

// FileChange is one entry from a lore status listing, e.g. "A hello.txt".
type FileChange struct {
	Status byte
	Path   string
}

// Status is the parsed result of `lore status` / `lore status --scan`.
type Status struct {
	Repository string
	Branch     string
	Summary    []string
	Staged     []FileChange
	Unstaged   []FileChange
}

func ParseStatus(output string) (Status, error) {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	var s Status
	var section string

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "Repository "):
			s.Repository = strings.TrimPrefix(line, "Repository ")
			s.Summary = append(s.Summary, line)
			continue
		case strings.HasPrefix(line, "On branch "):
			rest := strings.TrimPrefix(line, "On branch ")
			if idx := strings.Index(rest, " "); idx >= 0 {
				s.Branch = rest[:idx]
			} else {
				s.Branch = rest
			}
			s.Summary = append(s.Summary, line)
			continue
		case line == "Changes staged for commit:":
			section = "staged"
			continue
		case line == "Changes not staged for commit:", line == "Untracked files:":
			section = "unstaged"
			continue
		case strings.HasPrefix(line, "Tracked changes:"):
			continue
		}

		if section == "" {
			s.Summary = append(s.Summary, line)
			continue
		}

		change, ok := parseFileChangeLine(line)
		if !ok {
			return Status{}, fmt.Errorf("lore status: unrecognized line in %q section: %q", section, line)
		}
		switch section {
		case "staged":
			s.Staged = append(s.Staged, change)
		case "unstaged":
			s.Unstaged = append(s.Unstaged, change)
		}
	}

	if s.Repository == "" {
		return Status{}, fmt.Errorf("lore status: could not find a Repository line in output")
	}
	return s, nil
}

func parseFileChangeLine(line string) (FileChange, bool) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 || len(parts[0]) != 1 {
		return FileChange{}, false
	}
	return FileChange{Status: parts[0][0], Path: parts[1]}, true
}
