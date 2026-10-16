package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/solessfir/lazylore/internal/config"
	"github.com/solessfir/lazylore/internal/lore"
	"github.com/solessfir/lazylore/internal/ui"
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
		cloned, cloneErr := promptCloneHere(binPath, cwd)
		if cloneErr != nil {
			return cloneErr
		}
		if !cloned {
			return nil
		}
		repoRoot, err = lore.FindRepoRoot(cwd)
		if err != nil {
			return err
		}
	}

	runner := lore.NewExecRunner(binPath, repoRoot)
	defer runner.Shutdown()
	model := ui.NewModel(runner, filepath.Base(repoRoot), repoRoot)

	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = program.Run()
	return err
}

// promptCloneHere asks for a repository URL and clones it into dir when the
// working directory isn't a lore repository - lazygit's own "not a git
// repository, create one?" prompt, except lore repos are server-registered
// (see lore repository create), so cloning an existing one is the closer
// equivalent to a bare local init. Leaving the prompt empty (or EOF on stdin)
// reports false so lazylore exits cleanly; a clone that fails (bad URL, no
// auth, ...) returns an error. Neither case starts the TUI.
func promptCloneHere(binPath, dir string) (bool, error) {
	fmt.Fprint(os.Stderr, "Not a lore repository. Please provide repository URL or Enter to exit: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	url := strings.TrimSpace(line)
	if url == "" {
		return false, nil
	}

	cmd := exec.Command(binPath, "clone", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("cloning %q: %w", url, err)
	}
	return true, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
