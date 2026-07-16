package lore

import (
	"bufio"
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

// StreamRunner is implemented by runners that can report a --json command's
// NDJSON output line by line as it arrives, instead of only the final
// buffered Result - for live-updating the Command Log during a long-running
// command like push. ExecRunner implements it; FakeRunner (tests) doesn't
// need to.
type StreamRunner interface {
	RunStream(onLine func(string), args ...string) (Result, error)
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

// RunStream is Run, plus onLine is called with each line of stdout as the
// process emits it (buffered by bufio.Scanner's default line-at-a-time
// behavior). The full stdout is still returned in Result, same as Run,
// since callers (see runCheckedStream) need it to find the terminal
// "complete" event same as any other --json command.
func (r ExecRunner) RunStream(onLine func(string), args ...string) (Result, error) {
	cmd := exec.Command(r.BinaryPath, args...)
	if r.RepoPath != "" {
		cmd.Dir = r.RepoPath
	}
	var stdout, stderr bytes.Buffer
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{Args: args}, fmt.Errorf("running lore %s: %w", strings.Join(args, " "), err)
	}
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return Result{Args: args}, fmt.Errorf("running lore %s: %w", strings.Join(args, " "), err)
	}

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		stdout.WriteString(line)
		stdout.WriteByte('\n')
		if onLine != nil {
			onLine(line)
		}
	}

	runErr := cmd.Wait()
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
