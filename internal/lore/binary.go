package lore

import "fmt"

// ResolveBinaryPath picks the lore binary to run: an explicit override if
// given, otherwise a PATH lookup for "lore".
func ResolveBinaryPath(override string, lookPath func(string) (string, error)) (string, error) {
	if override != "" {
		return override, nil
	}
	path, err := lookPath("lore")
	if err != nil {
		return "", fmt.Errorf("lore binary not found on PATH and no override is configured: %w", err)
	}
	return path, nil
}
