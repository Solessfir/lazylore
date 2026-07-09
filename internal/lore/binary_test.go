package lore_test

import (
	"errors"
	"testing"

	"lazylore/internal/lore"
)

func TestResolveBinaryPath_OverrideWins(t *testing.T) {
	lookPath := func(string) (string, error) {
		t.Fatal("lookPath should not be called when an override is set")
		return "", nil
	}
	path, err := lore.ResolveBinaryPath(`C:\custom\lore.exe`, lookPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != `C:\custom\lore.exe` {
		t.Fatalf("path = %q, want the override", path)
	}
}

func TestResolveBinaryPath_FallsBackToLookPath(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name != "lore" {
			t.Fatalf("lookPath called with %q, want %q", name, "lore")
		}
		return `C:\Git\LoreBin\lore.exe`, nil
	}
	path, err := lore.ResolveBinaryPath("", lookPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != `C:\Git\LoreBin\lore.exe` {
		t.Fatalf("path = %q, want the looked-up path", path)
	}
}

func TestResolveBinaryPath_ErrorsWhenNeitherResolves(t *testing.T) {
	lookPath := func(string) (string, error) {
		return "", errors.New("exec: \"lore\": executable file not found in $PATH")
	}
	_, err := lore.ResolveBinaryPath("", lookPath)
	if err == nil {
		t.Fatal("expected an error when there is no override and lookPath fails")
	}
}
