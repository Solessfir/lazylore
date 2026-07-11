package lore

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Revision is one entry from `lore --json history`.
type Revision struct {
	Number  int
	Message string
	Hash    string
	Author  string
}

// revisionHistoryEntryData mirrors LoreRevisionHistoryEntryEventData
// (lore-revision/src/revision/history.rs) - only the field Revision uses.
type revisionHistoryEntryData struct {
	RevisionNumber int    `json:"revisionNumber"`
	Revision       string `json:"revision"`
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
			current = &Revision{Number: data.RevisionNumber, Hash: data.Revision}

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
				raw := string(data.Value.Data)
				author := strings.Trim(raw, `"`)
				if author == "" || strings.HasPrefix(author, "{") {
					var s string
					if json.Unmarshal([]byte(raw), &s) == nil && s != "" {
						author = s
					}
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
