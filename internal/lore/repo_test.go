package lore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/solessfir/lazylore/internal/lore"
)

func TestFindRepoRoot_FindsFromDeepSubdir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".lore"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := lore.FindRepoRoot(deep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantAbs, _ := filepath.Abs(root)
	gotAbs, _ := filepath.Abs(got)
	if gotAbs != wantAbs {
		t.Fatalf("root = %q, want %q", gotAbs, wantAbs)
	}
}

func TestFindRepoRoot_ErrorsWhenNoLoreDirExists(t *testing.T) {
	dir := t.TempDir()
	_, err := lore.FindRepoRoot(dir)
	if err == nil {
		t.Fatal("expected an error when no .lore directory exists up the tree")
	}
}
