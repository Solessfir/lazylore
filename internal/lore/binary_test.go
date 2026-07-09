package lore_test

import (
	"errors"
	"testing"

	"lazylore/internal/lore"
)

func notFoundLookPath(string) (string, error) {
	return "", errors.New("exec: \"lore\": executable file not found in $PATH")
}

func noExists(string) bool { return false }

func TestResolveBinaryPath_OverrideWins(t *testing.T) {
	lookPath := func(string) (string, error) {
		t.Fatal("lookPath should not be called when an override is set")
		return "", nil
	}
	exists := func(string) bool {
		t.Fatal("exists should not be called when an override is set")
		return false
	}
	path, err := lore.ResolveBinaryPath(`C:\custom\lore.exe`, lookPath, exists, "windows")
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
	exists := func(string) bool {
		t.Fatal("exists should not be called when lookPath succeeds")
		return false
	}
	path, err := lore.ResolveBinaryPath("", lookPath, exists, "windows")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != `C:\Git\LoreBin\lore.exe` {
		t.Fatalf("path = %q, want the looked-up path", path)
	}
}

func TestResolveBinaryPath_FallsBackToWindowsDefaultLocation(t *testing.T) {
	exists := func(path string) bool {
		return path == `C:\Program Files\lore\lore.exe`
	}
	path, err := lore.ResolveBinaryPath("", notFoundLookPath, exists, "windows")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != `C:\Program Files\lore\lore.exe` {
		t.Fatalf("path = %q, want the Windows default install location", path)
	}
}

func TestResolveBinaryPath_FallsBackToLinuxDefaultLocations(t *testing.T) {
	exists := func(path string) bool {
		return path == "/opt/lore/bin/lore"
	}
	path, err := lore.ResolveBinaryPath("", notFoundLookPath, exists, "linux")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/opt/lore/bin/lore" {
		t.Fatalf("path = %q, want the second Linux default install location", path)
	}
}

func TestResolveBinaryPath_PrefersUsrLocalBinOverOptOnDarwin(t *testing.T) {
	exists := func(path string) bool {
		return path == "/usr/local/bin/lore" || path == "/opt/lore/bin/lore"
	}
	path, err := lore.ResolveBinaryPath("", notFoundLookPath, exists, "darwin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/usr/local/bin/lore" {
		t.Fatalf("path = %q, want /usr/local/bin/lore checked first", path)
	}
}

func TestResolveBinaryPath_ErrorsWhenNothingResolves(t *testing.T) {
	_, err := lore.ResolveBinaryPath("", notFoundLookPath, noExists, "windows")
	if err == nil {
		t.Fatal("expected an error when override, PATH, and default locations all fail")
	}
}
