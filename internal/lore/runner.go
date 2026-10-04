package lore

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	context    context.Context
	cancel     context.CancelFunc
	state      *execRunnerState
}

type execRunnerState struct {
	mu      sync.Mutex
	changed *sync.Cond
	active  int
	closing bool
}

// NewExecRunner creates a runner whose subprocesses are owned by the
// application lifecycle. Shutdown cancels every active command and waits for
// its process-reaping path to finish before lazylore exits.
func NewExecRunner(binaryPath, repoPath string) *ExecRunner {
	ctx, cancel := context.WithCancel(context.Background())
	state := &execRunnerState{}
	state.changed = sync.NewCond(&state.mu)
	return &ExecRunner{
		BinaryPath: binaryPath,
		RepoPath:   repoPath,
		context:    ctx,
		cancel:     cancel,
		state:      state,
	}
}

func (r ExecRunner) command(args ...string) *exec.Cmd {
	ctx := r.context
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, r.BinaryPath, args...)
	// Resolve relative executables before the child switches to the repository.
	if cmd.Err == nil && !filepath.IsAbs(cmd.Path) {
		cmd.Path, cmd.Err = filepath.Abs(cmd.Path)
	}
	return cmd
}

func (r ExecRunner) begin() error {
	if r.state == nil {
		return nil
	}

	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	if r.state.closing {
		return context.Canceled
	}

	r.state.active++
	return nil
}

func (r ExecRunner) done() {
	if r.state == nil {
		return
	}

	r.state.mu.Lock()
	r.state.active--
	r.state.changed.Broadcast()
	r.state.mu.Unlock()
}

// Shutdown prevents new commands from starting, cancels active subprocesses,
// and waits until every Run/RunStream call has reaped its child process.
func (r ExecRunner) Shutdown() {
	if r.state == nil {
		return
	}

	r.state.mu.Lock()
	r.state.closing = true
	r.state.mu.Unlock()

	r.cancel()

	r.state.mu.Lock()
	for r.state.active > 0 {
		r.state.changed.Wait()
	}
	r.state.mu.Unlock()
}

func (r ExecRunner) Run(args ...string) (Result, error) {
	if err := r.begin(); err != nil {
		return Result{Args: args}, fmt.Errorf("running lore %s: %w", strings.Join(args, " "), err)
	}
	defer r.done()

	cmd := r.command(args...)
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
// process emits it. The full stdout is still returned in Result, same as Run,
// since callers (see runCheckedStream) need it to find the terminal
// "complete" event same as any other --json command.
func (r ExecRunner) RunStream(onLine func(string), args ...string) (Result, error) {
	if err := r.begin(); err != nil {
		return Result{Args: args}, fmt.Errorf("running lore %s: %w", strings.Join(args, " "), err)
	}
	defer r.done()

	cmd := r.command(args...)
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

	reader := bufio.NewReader(stdoutPipe)
	var readErr error
	for {
		line, err := reader.ReadString('\n')
		stdout.WriteString(line)
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
		if onLine != nil {
			if line != "" {
				onLine(line)
			}
		}

		if err != nil {
			if !errors.Is(err, io.EOF) {
				readErr = err
			}
			break
		}
	}

	runErr := cmd.Wait()
	result := Result{Args: args, Stdout: stdout.String(), Stderr: stderr.String()}
	if readErr != nil {
		return result, fmt.Errorf("reading lore %s output: %w", strings.Join(args, " "), readErr)
	}

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
