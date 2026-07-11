package lore

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// runChecked runs a lore command with --json prepended, and treats the
// output's "complete" event (see events.go) as the authoritative
// success/failure signal - not the raw process exit code or stderr text.
// args is the caller-facing command (without --json) so error messages
// read the way the caller would type the command themselves.
func runChecked(r Runner, args ...string) (Result, error) {
	fullArgs := append([]string{"--json"}, args...)
	res, err := r.Run(fullArgs...)
	if err != nil {
		return res, err
	}

	events, err := parseEvents(res.Stdout)
	if err != nil {
		return res, fmt.Errorf("lore %s: %w", strings.Join(args, " "), err)
	}
	complete, err := findComplete(events)
	if err != nil {
		return res, fmt.Errorf("lore %s: %w", strings.Join(args, " "), err)
	}
	if complete.Status != 0 {
		msg := complete.Error.Message
		if msg == "" {
			msg = strings.TrimSpace(res.Stderr)
		}
		return res, fmt.Errorf("lore %s: %s", strings.Join(args, " "), msg)
	}
	return res, nil
}

func GetStatus(r Runner) (Status, error) {
	res, err := runChecked(r, "status", "--scan")
	if err != nil {
		return Status{}, err
	}
	return ParseStatus(res.Stdout)
}

func BranchList(r Runner) ([]Branch, error) {
	res, err := runChecked(r, "branch", "list")
	if err != nil {
		return nil, err
	}
	return ParseBranchList(res.Stdout)
}

func History(r Runner, length int) ([]Revision, error) {
	args := []string{"history"}
	if length > 0 {
		args = append(args, strconv.Itoa(length))
	}
	res, err := runChecked(r, args...)
	if err != nil {
		return nil, err
	}
	return ParseHistory(res.Stdout)
}

// fileDiffData mirrors the fileDiff event's data: `patch` is the same
// unified-diff text `lore diff` prints without --json, just delivered
// through a JSON envelope instead of raw stdout mixed with pager/log noise.
type fileDiffData struct {
	Path  string `json:"path"`
	Patch string `json:"patch"`
}

func Diff(r Runner, path string) (string, error) {
	res, err := runChecked(r, "diff", path)
	if err != nil {
		return "", err
	}
	events, err := parseEvents(res.Stdout)
	if err != nil {
		return "", fmt.Errorf("lore diff %s: %w", path, err)
	}
	for _, e := range events {
		if e.TagName != "fileDiff" {
			continue
		}
		var data fileDiffData
		if err := json.Unmarshal(e.Data, &data); err != nil {
			return "", fmt.Errorf("parsing fileDiff event: %w", err)
		}
		return data.Patch, nil
	}
	return "", nil // no fileDiff event: nothing changed
}

func Stage(r Runner, paths ...string) (Result, error) {
	return runChecked(r, append([]string{"stage"}, paths...)...)
}

func Unstage(r Runner, paths ...string) (Result, error) {
	return runChecked(r, append([]string{"unstage"}, paths...)...)
}

func Reset(r Runner, paths ...string) (Result, error) {
	return runChecked(r, append([]string{"reset"}, paths...)...)
}

// DiscardChanges fully discards a single file's changes, whether staged,
// unstaged, or both. `lore reset <path>` alone fails with "invalid
// arguments: Failed to reset staged node" if the file has staged content, so
// this always unstages first - a safe no-op when nothing was staged - then
// resets, which is then guaranteed to succeed since nothing remains staged.
func DiscardChanges(r Runner, path string) (Result, error) {
	if _, err := runChecked(r, "unstage", path); err != nil {
		return Result{}, err
	}
	return runChecked(r, "reset", path)
}

func Commit(r Runner, message string) (Result, error) {
	return runChecked(r, "commit", message)
}

func SwitchBranch(r Runner, name string) (Result, error) {
	return runChecked(r, "branch", "switch", name)
}

func CreateBranch(r Runner, name string) (Result, error) {
	return runChecked(r, "branch", "create", name)
}
