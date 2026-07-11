package lore

import (
	"encoding/json"
	"fmt"
	"strings"
)

// event is one line of `lore --json <command>` output: a tagged union
// keyed by tagName, with the tag-specific payload deferred as raw JSON
// until the caller knows which struct to decode it into. Confirmed against
// lore's real event schema (C:\Git\lore\lore-revision\src\event.rs) and by
// running the real binary with --json against a scratch repo - --json is
// undocumented (absent from `lore --help`), so nothing here is guessed.
type event struct {
	TagName string          `json:"tagName"`
	Data    json.RawMessage `json:"data"`
}

// parseEvents splits --json output into one event per NDJSON line,
// skipping blank lines. A line that fails to parse is a real error, not
// something to silently skip - it should never happen against a real lore
// binary, so surfacing it immediately is more useful than losing data.
func parseEvents(output string) ([]event, error) {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	events := make([]event, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("parsing --json event line %q: %w", line, err)
		}
		events = append(events, e)
	}
	return events, nil
}

// completeErrorDetail is the "error" field of a completeEventData: empty
// (zero errorCode, empty message) on success, populated on failure.
type completeErrorDetail struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
}

// completeEventData is the terminal event every --json command emits
// (tagName "complete"). Status is 0 on success, nonzero on failure - this
// is the authoritative success/failure signal for a --json command,
// preferred over the raw process exit code (which the two do agree on in
// sign, confirmed empirically, but Status.Error.Message is a real
// human-readable string instead of scraped stderr text).
type completeEventData struct {
	Status int32               `json:"status"`
	Error  completeErrorDetail `json:"error"`
}

// findComplete locates the FIRST "complete" event among events and decodes
// its data. Its absence means the output didn't come from a --json
// invocation, or lore's output format changed in a way this parser doesn't
// understand - either way, a caller needs to know rather than silently
// treating the command as successful.
//
// Taking the first match matters: `lore --json history` was observed
// emitting a real success "complete" (status 0) for the history listing
// itself, followed by a second, unrelated "complete" (status -1, "No auth
// endpoint available") from a background remote-availability check that
// fails when offline. The primary operation's terminal event comes first;
// anything after it is auxiliary.
func findComplete(events []event) (completeEventData, error) {
	for _, e := range events {
		if e.TagName != "complete" {
			continue
		}
		var data completeEventData
		if err := json.Unmarshal(e.Data, &data); err != nil {
			return completeEventData{}, fmt.Errorf("parsing complete event: %w", err)
		}
		return data, nil
	}
	return completeEventData{}, fmt.Errorf("no complete event found in --json output")
}
