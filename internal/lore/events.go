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
// Push event payloads, confirmed against lore's real event schema
// (lore-revision/src/branch/push.rs's LoreBranchPush*EventData structs) and
// the CLI's own println formatting for them
// (lore-client/src/cli/commands/branch.rs's handle_branch_push) - a subset
// of the tagNames a --json push actually emits (skips the per-revision
// update/create bookkeeping events, which are noise for a Command Log line
// but real for the CLI's own verbose printing).
type pushEventData struct {
	BranchName           string `json:"branchName"`
	LocalHistory         uint64 `json:"localHistory"`
	Fragments            uint64 `json:"fragments"`
	BytesTransferred     uint64 `json:"bytesTransferred"`
	NewRemoteRevision    string `json:"newRemoteRevision"`
	OldRemoteRevision    string `json:"oldRemoteRevision"`
	NewRemoteRevisionNum uint64 `json:"newRemoteRevisionNumber"`
	FastForwardMerged    bool   `json:"fastForwardMerged"`
}

// FormatPushEventLine renders one push --json event as a Command Log line,
// matching the CLI's own printed text for the same event (see
// pushEventData). branchName carries the name across calls: only the
// "branchPush" event actually has it (branchPushRevisionPushEnd doesn't),
// so the caller must thread nextBranchName back in as branchName on its
// next call, exactly like the CLI's own captured-in-a-mutex branch_name.
// ok is false for tagNames this doesn't have a line for (event parses
// fine but there's nothing worth showing).
func FormatPushEventLine(tagName string, data json.RawMessage, branchName string) (line, nextBranchName string, ok bool) {
	var d pushEventData
	switch tagName {
	case "branchPush":
		if err := json.Unmarshal(data, &d); err != nil {
			return "", branchName, false
		}
		if d.LocalHistory > 0 {
			return fmt.Sprintf("Local branch is %d revision(s) ahead of remote, pushing all revisions", d.LocalHistory), d.BranchName, true
		}
		return "", d.BranchName, false
	case "branchPushFragmentBegin":
		if err := json.Unmarshal(data, &d); err != nil || d.Fragments == 0 {
			return "", branchName, false
		}
		return fmt.Sprintf("Pushing %d fragment(s)", d.Fragments), branchName, true
	case "branchPushFragmentEnd":
		if err := json.Unmarshal(data, &d); err != nil || d.Fragments == 0 {
			return "", branchName, false
		}
		if d.BytesTransferred > 0 {
			return fmt.Sprintf("Pushed %d fragment(s), %d bytes", d.Fragments, d.BytesTransferred), branchName, true
		}
		return fmt.Sprintf("Pushed %d fragment(s)", d.Fragments), branchName, true
	case "branchPushRevisionPushEnd":
		if err := json.Unmarshal(data, &d); err != nil {
			return "", branchName, false
		}
		switch {
		case d.FastForwardMerged:
			return fmt.Sprintf("Pushed revision %d -> %s to branch %s (fast-forward merged on server, run sync to update)", d.NewRemoteRevisionNum, d.NewRemoteRevision, branchName), branchName, true
		case d.OldRemoteRevision != d.NewRemoteRevision:
			return fmt.Sprintf("Pushed revision %d -> %s to branch %s", d.NewRemoteRevisionNum, d.NewRemoteRevision, branchName), branchName, true
		default:
			return fmt.Sprintf("Revision %d -> %s already at latest of branch %s", d.NewRemoteRevisionNum, d.NewRemoteRevision, branchName), branchName, true
		}
	default:
		return "", branchName, false
	}
}

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
