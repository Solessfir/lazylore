package lore

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Result is the outcome of running one lore CLI invocation.
type Result struct {
	Args     []string
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes lore CLI commands. A nonzero ExitCode is not an error -
// only a failure to start the process is. Callers decide what a nonzero
// exit code means for the command they ran.
type Runner interface {
	Run(args ...string) (Result, error)
}

// ExecRunner runs commands against a real lore binary on disk.
type ExecRunner struct {
	BinaryPath string
	RepoPath   string
}

func (r ExecRunner) Run(args ...string) (Result, error) {
	cmd := exec.Command(r.BinaryPath, args...)
	if r.RepoPath != "" {
		cmd.Dir = r.RepoPath
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	result := Result{Args: args, Stdout: stdout.String(), Stderr: stderr.String()}

	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		return result, nil
	case errors.As(runErr, &exitErr):
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	default:
		return result, fmt.Errorf("running lore %s: %w", strings.Join(args, " "), runErr)
	}
}
