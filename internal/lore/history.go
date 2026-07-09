package lore

import (
	"fmt"
	"strconv"
	"strings"
)

// Revision is one entry from `lore history --oneline`.
type Revision struct {
	Number  int
	Message string
}

func ParseHistoryOneline(output string) ([]Revision, error) {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	var revisions []Revision

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("lore history --oneline: unrecognized line: %q", line)
		}
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("lore history --oneline: invalid revision number in line %q: %w", line, err)
		}
		revisions = append(revisions, Revision{Number: n, Message: parts[1]})
	}
	return revisions, nil
}
