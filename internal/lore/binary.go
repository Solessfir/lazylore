package lore

import "fmt"

// ResolveBinaryPath picks the lore binary to run, checked in order:
// an explicit override, a PATH lookup for "lore", then the platform's
// default install location(s). exists reports whether a candidate path
// is present on disk (e.g. via os.Stat); goos is the target OS (e.g.
// runtime.GOOS in production, injected here for testability).
func ResolveBinaryPath(override string, lookPath func(string) (string, error), exists func(string) bool, goos string) (string, error) {
	if override != "" {
		return override, nil
	}
	if path, err := lookPath("lore"); err == nil {
		return path, nil
	}
	for _, candidate := range defaultInstallPaths(goos) {
		if exists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("lore binary not found on PATH or in the default install location, and no override is configured (set lorePath in config.yml)")
}

// defaultInstallPaths returns the platform's conventional lore install
// location(s), checked in order when PATH lookup fails.
func defaultInstallPaths(goos string) []string {
	switch goos {
	case "windows":
		return []string{`C:\Program Files\lore\lore.exe`}
	case "darwin", "linux":
		return []string{"/usr/local/bin/lore", "/opt/lore/bin/lore"}
	default:
		return nil
	}
}
