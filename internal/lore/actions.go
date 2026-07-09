package lore

import (
	"fmt"
	"strconv"
	"strings"
)

func runChecked(r Runner, args ...string) (Result, error) {
	res, err := r.Run(args...)
	if err != nil {
		return res, err
	}
	if res.ExitCode != 0 {
		return res, fmt.Errorf("lore %s: %s", strings.Join(args, " "), strings.TrimSpace(res.Stderr))
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

func HistoryOneline(r Runner, length int) ([]Revision, error) {
	args := []string{"history", "--oneline"}
	if length > 0 {
		args = append(args, strconv.Itoa(length))
	}
	res, err := runChecked(r, args...)
	if err != nil {
		return nil, err
	}
	return ParseHistoryOneline(res.Stdout)
}

func Diff(r Runner, path string) (string, error) {
	res, err := runChecked(r, "diff", path)
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
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
