package lore

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRepoRoot walks up from startDir looking for a .lore directory, the
// same way git walks up looking for .git.
func FindRepoRoot(startDir string) (string, error) {
	dir := startDir
	for {
		info, err := os.Stat(filepath.Join(dir, ".lore"))
		if err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a lore repository (no .lore directory found in %q or any parent)", startDir)
		}
		dir = parent
	}
}
