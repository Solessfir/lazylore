package lore

import (
	"encoding/json"
	"fmt"
)

// Revision is one entry from `lore --json history`.
type Revision struct {
	Number  int
	Message string
	Hash    string
	Author  string
	Parent  string // direct parent's hash; zero-filled hash string when this is the root revision
}

// revisionHistoryEntryData mirrors LoreRevisionHistoryEntryEventData
// (lore-revision/src/revision/history.rs). Parent is [direct_parent,
// merge_other_parent] - only the direct parent (index 0) is used here.
type revisionHistoryEntryData struct {
	RevisionNumber int       `json:"revisionNumber"`
	Revision       string    `json:"revision"`
	Parent         [2]string `json:"parent"`
}

// IsZeroHash reports whether h is lore's all-zero sentinel hash (used e.g.
// for "no parent"/"no revision"), without hardcoding its exact length.
func IsZeroHash(h string) bool {
	if h == "" {
		return true
	}
	for _, r := range h {
		if r != '0' {
			return false
		}
	}
	return true
}

// metadataEventData mirrors LoreMetadataEventData (lore-revision/src/event.rs):
// a key plus a tagged value ({"tagName":"string","data":...},
// {"tagName":"numeric","data":...}, etc.) - only "message" and "created-by"/"committed-by" for author.
type metadataEventData struct {
	Key   string `json:"key"`
	Value struct {
		TagName string          `json:"tagName"`
		Data    json.RawMessage `json:"data"`
	} `json:"value"`
}

// ParseHistory reads `lore --json history` output. Lore reports each
// revision as a revisionHistoryEntry event immediately followed by zero or
// more metadata events (message/created-by/timestamp/...) describing it -
// the commit message lives in a separate "message" metadata event, not on
// the history entry itself - so entries are only flushed once the next
// revisionHistoryEntry (or the end of the stream) is reached.
func ParseHistory(output string) ([]Revision, error) {
	events, err := parseEvents(output)
	if err != nil {
		return nil, err
	}

	var revisions []Revision
	var current *Revision

	flush := func() {
		if current != nil {
			revisions = append(revisions, *current)
			current = nil
		}
	}

	for _, e := range events {
		switch e.TagName {
		case "revisionHistoryEntry":
			flush()
			var data revisionHistoryEntryData
			if err := json.Unmarshal(e.Data, &data); err != nil {
				return nil, fmt.Errorf("parsing revisionHistoryEntry event: %w", err)
			}
			current = &Revision{Number: data.RevisionNumber, Hash: data.Revision, Parent: data.Parent[0]}

		case "metadata":
			if current == nil {
				continue
			}
			var data metadataEventData
			if err := json.Unmarshal(e.Data, &data); err != nil {
				return nil, fmt.Errorf("parsing metadata event: %w", err)
			}
			switch data.Key {
			case "message":
				var message string
				if err := json.Unmarshal(data.Value.Data, &message); err != nil {
					return nil, fmt.Errorf("parsing metadata message value: %w", err)
				}
				current.Message = message
			case "created-by", "committed-by":
				var author string
				if err := json.Unmarshal(data.Value.Data, &author); err != nil {
					return nil, fmt.Errorf("parsing metadata author value: %w", err)
				}
				if author != "" {
					current.Author = author
				}
			}
		}
	}
	flush()

	return revisions, nil
}
