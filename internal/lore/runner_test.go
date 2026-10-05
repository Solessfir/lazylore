package lore_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/solessfir/lazylore/internal/lore"
)

func TestExecRunner_Run_CapturesStdoutAndExitCode(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH, skipping subprocess plumbing test")
	}
	r := lore.ExecRunner{BinaryPath: goBin}

	result, err := r.Run("version")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "go version") {
		t.Fatalf("Stdout = %q, want it to contain %q", result.Stdout, "go version")
	}
}

func TestExecRunner_Run_NonZeroExitIsNotAnError(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH, skipping subprocess plumbing test")
	}
	r := lore.ExecRunner{BinaryPath: goBin}

	result, err := r.Run("env", "-thisflagdoesnotexist")
	if err != nil {
		t.Fatalf("Run returned error for a nonzero exit: %v", err)
	}
	if result.ExitCode == 0 {
		t.Fatalf("ExitCode = 0, want nonzero for an invalid flag")
	}
	if result.Stderr == "" {
		t.Fatalf("Stderr is empty, want the invalid-flag message")
	}
}

func TestExecRunner_Run_BinaryNotFoundIsAnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.exe")
	r := lore.ExecRunner{BinaryPath: missing}

	_, err := r.Run("status")
	if err == nil {
		t.Fatal("Run returned nil error for a nonexistent binary, want an error")
	}
}

func TestExecRunner_RelativeBinaryUsesLaunchDirectory(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH")
	}
	launchDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(launchDir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(goBin, filepath.Join(launchDir, "bin", "go")); err != nil {
		t.Skipf("cannot create executable symlink: %v", err)
	}
	repoDir := t.TempDir()
	t.Chdir(launchDir)
	runner := lore.NewExecRunner(filepath.Join("bin", "go"), repoDir)
	defer runner.Shutdown()
	for _, stream := range []bool{false, true} {
		var result lore.Result
		if stream {
			result, err = runner.RunStream(nil, "version")
		} else {
			result, err = runner.Run("version")
		}
		if err != nil || result.ExitCode != 0 || !strings.Contains(result.Stdout, "go version") {
			t.Fatalf("stream=%v: result=%#v, error=%v", stream, result, err)
		}
	}
}

func TestExecRunner_RunStreamAcceptsLargeLines(t *testing.T) {
	t.Setenv("LAZYLORE_RUNNER_HELPER_MODE", "large-line")
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolving test executable: %v", err)
	}
	runner := lore.NewExecRunner(executable, "")
	defer runner.Shutdown()

	longest := 0
	result, err := runner.RunStream(func(line string) {
		if len(line) > longest {
			longest = len(line)
		}
	}, "-test.run=^TestExecRunnerHelper$")
	if err != nil {
		t.Fatalf("RunStream returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if longest != 2*1024*1024 {
		t.Fatalf("longest streamed line = %d bytes, want %d", longest, 2*1024*1024)
	}
}

func TestExecRunner_ShutdownCancelsActiveCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("LAZYLORE_RUNNER_HELPER_MODE", "wait")
	t.Setenv("LAZYLORE_RUNNER_HELPER_MARKER", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolving test executable: %v", err)
	}
	runner := lore.NewExecRunner(executable, "")

	done := make(chan error, 1)
	go func() {
		_, runErr := runner.Run("-test.run=^TestExecRunnerHelper$")
		done <- runErr
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper process did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	started := time.Now()
	runner.Shutdown()
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("Shutdown took %s, want active command canceled promptly", elapsed)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Shutdown")
	}
	if _, err := runner.Run("version"); err == nil {
		t.Fatal("Run started a new command after Shutdown")
	}
}

func TestExecRunnerHelper(t *testing.T) {
	switch os.Getenv("LAZYLORE_RUNNER_HELPER_MODE") {
	case "large-line":
		fmt.Println(strings.Repeat("x", 2*1024*1024))
	case "wait":
		if err := os.WriteFile(os.Getenv("LAZYLORE_RUNNER_HELPER_MARKER"), []byte("started"), 0o600); err != nil {
			t.Fatalf("writing marker: %v", err)
		}
		time.Sleep(30 * time.Second)
	}
}
