package lore_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lazylore/internal/lore"
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
