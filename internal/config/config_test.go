package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/solessfir/lazylore/internal/config"
)

func TestLoad_MissingFileReturnsZeroValue(t *testing.T) {
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LorePath != "" {
		t.Fatalf("LorePath = %q, want empty", cfg.LorePath)
	}
}

func TestLoad_ReadsLorePathOverride(t *testing.T) {
	dir := t.TempDir()
	content := "lorePath: C:\\custom\\lore.exe\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LorePath != `C:\custom\lore.exe` {
		t.Fatalf("LorePath = %q, want the configured override", cfg.LorePath)
	}
}

func TestLoad_ErrorsOnMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("lorePath: [unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected an error for malformed YAML")
	}
}

func TestLoad_LegacyFilenameAndYAMLPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("lorePath: legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		content, want string
		wantError     bool
	}{
		{"", "legacy", false},
		{"lorePath: preferred\n", "preferred", false},
		{" \n", "", false},
		{"lorePath: [unterminated\n", "", true},
	} {
		if tc.content != "" {
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cfg, err := config.Load(dir)
		if (err != nil) != tc.wantError || cfg.LorePath != tc.want {
			t.Fatalf("Load with config.yaml=%q: cfg=%+v err=%v", tc.content, cfg, err)
		}
	}
}
