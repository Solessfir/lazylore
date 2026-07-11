package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"lazylore/internal/config"
	"lazylore/internal/lore"
	"lazylore/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazylore: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("resolving config dir: %w", err)
	}
	cfg, err := config.Load(filepath.Join(configDir, "lazylore"))
	if err != nil {
		return err
	}

	binPath, err := lore.ResolveBinaryPath(cfg.LorePath, exec.LookPath, fileExists, runtime.GOOS)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving working directory: %w", err)
	}
	repoRoot, err := lore.FindRepoRoot(cwd)
	if err != nil {
		return err
	}

	runner := lore.ExecRunner{BinaryPath: binPath, RepoPath: repoRoot}
	model := ui.NewModel(runner, filepath.Base(repoRoot))

	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = program.Run()
	return err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
