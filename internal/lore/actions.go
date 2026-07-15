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

// HistoryForBranch is History scoped to branch (`lore history --branch`),
// for the Branches panel's "Log" view.
func HistoryForBranch(r Runner, branch string, length int) ([]Revision, error) {
	args := []string{"history"}
	if length > 0 {
		args = append(args, strconv.Itoa(length))
	}
	args = append(args, "--branch", branch)
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

// parseFileDiffPatches collects every fileDiff event's patch text, in order.
func parseFileDiffPatches(output string) ([]string, error) {
	events, err := parseEvents(output)
	if err != nil {
		return nil, err
	}
	var patches []string
	for _, e := range events {
		if e.TagName != "fileDiff" {
			continue
		}
		var data fileDiffData
		if err := json.Unmarshal(e.Data, &data); err != nil {
			return nil, fmt.Errorf("parsing fileDiff event: %w", err)
		}
		patches = append(patches, data.Patch)
	}
	return patches, nil
}

func Diff(r Runner, path string) (string, error) {
	res, err := runChecked(r, "diff", path)
	if err != nil {
		return "", err
	}
	patches, err := parseFileDiffPatches(res.Stdout)
	if err != nil {
		return "", fmt.Errorf("lore diff %s: %w", path, err)
	}
	if len(patches) == 0 {
		return "", nil // no fileDiff event: nothing changed
	}
	return patches[0], nil
}

// DiffRevision returns the combined patch for every file that changed
// between source and target (e.g. a commit's parent and the commit itself)
// - lore's equivalent of `git show <commit>`. Confirmed `lore diff` accepts
// --source/--target with no path restriction to diff every changed file
// (lore-cli-commands.md: "--source ... by default the current revision",
// "--target ... by default the current file system state").
func DiffRevision(r Runner, source, target string) (string, error) {
	res, err := runChecked(r, "diff", "--source", source, "--target", target)
	if err != nil {
		return "", err
	}
	patches, err := parseFileDiffPatches(res.Stdout)
	if err != nil {
		return "", fmt.Errorf("lore diff --source %s --target %s: %w", source, target, err)
	}
	return strings.Join(patches, ""), nil
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

// DiscardAllChanges discards every currently staged and unstaged change in
// the working tree, purging added/untracked paths too. A no-op when paths
// is empty.
func DiscardAllChanges(r Runner, paths []string) (Result, error) {
	if len(paths) == 0 {
		return Result{}, nil
	}
	if _, err := runChecked(r, append([]string{"unstage"}, paths...)...); err != nil {
		return Result{}, err
	}
	return runChecked(r, append([]string{"reset", "--purge"}, paths...)...)
}

// ResetBranchTo moves the current branch's latest pointer to revision -
// lore's own reset primitive only moves the pointer (no git-style
// hard/soft/mixed working-tree modes to choose between).
func ResetBranchTo(r Runner, revision string) (Result, error) {
	return runChecked(r, "branch", "reset", revision)
}

// SyncTo synchronizes the working state to revision. If revision belongs to
// a different branch than the current one, lore updates the current branch
// accordingly (lore's equivalent of checking out an arbitrary commit).
func SyncTo(r Runner, revision string) (Result, error) {
	return runChecked(r, "sync", revision)
}

// RevertRevision undoes revision's changes by committing a new revision on
// top - lore has no rebase/history-rewrite, so this (not a true "drop") is
// the closest equivalent to git's drop for anything but the tip commit.
// Auto-commits when the revert is clean (lore's own default); a conflicting
// revert surfaces as a runChecked error here rather than dropping into
// lore's resolve/abort sub-flow, which lazylore has no UI for.
// message (when non-empty) becomes the auto-commit's message via lore's own
// `--message` flag (lore-client/src/cli/commands/revision.rs's
// RevisionRevertArgs.message) - without it lore commits with no message at
// all, unlike git revert's own "Revert \"<subject>\"" default.
func RevertRevision(r Runner, revision, message string) (Result, error) {
	args := []string{"revision", "revert", revision}
	if message != "" {
		args = append(args, "--message", message)
	}
	return runChecked(r, args...)
}

func Commit(r Runner, message string) (Result, error) {
	return runChecked(r, "commit", message)
}

// Pull syncs the current branch to its latest remote state - `lore sync`
// with no revision and --remote, lore's closest equivalent to git pull.
func Pull(r Runner) (Result, error) {
	return runChecked(r, "sync", "--remote")
}

// Push pushes the current branch's commits to remote.
func Push(r Runner) (Result, error) {
	return runChecked(r, "push")
}

func SwitchBranch(r Runner, name string) (Result, error) {
	return runChecked(r, "branch", "switch", name)
}

func CreateBranch(r Runner, name string) (Result, error) {
	return runChecked(r, "branch", "create", name)
}

// MergeBranch merges name into the current branch, auto-committing when
// clean. lore has no resolve/abort UI in lazylore for a conflicting merge -
// same limitation as RevertRevision - so a conflict just surfaces as a
// runChecked error here.
func MergeBranch(r Runner, name string) (Result, error) {
	return runChecked(r, "branch", "merge", name)
}
