# lazylore

A terminal UI for [Lore](https://github.com/EpicGames/lore), Epic Games' open
source version control system - the same relationship
[lazygit](https://github.com/jesseduffield/lazygit) has to `git`.

> [!WARNING]
> This project was built with agentic AI coding (Claude).

## Status

Pre-1.0. Covers the daily-driver core loop: viewing status, staging/unstaging
files, discarding changes, committing, viewing diffs, switching/creating/
resetting and merging branches, browsing revision history, pulling, pushing,
and file locking. `layers` and the remaining `lore` commands are not wired up
yet.

## Requirements

- A `lore` binary on your `PATH` (or configured via `lorePath` in
  `config.yml`, see below).
- When launched outside a Lore repository, `lazylore` prompts for a remote to
  clone.

## Install

Download a release binary from the
[Releases page](https://github.com/Solessfir/lazylore/releases), or build
from source:

```bash
go build -o bin/lazylore ./cmd/lazylore
```

## Configuration

Optional. `lazylore` auto-detects the `lore` binary: first on `PATH`, then
the platform's default install location (`C:\Program Files\lore\lore.exe`
on Windows; `/usr/local/bin/lore`, then `/opt/lore/bin/lore`, on Linux and
macOS). Most installs need no configuration at all.

If `lore` lives somewhere else, override it via `config.yml` in `lazylore`'s
OS config directory (`%AppData%\lazylore` on Windows, `~/.config/lazylore`
on Linux, `~/Library/Application Support/lazylore` on macOS):

```yaml
lorePath: C:\custom\path\to\lore.exe
```

Relative binary paths resolve from the directory where `lazylore` is launched.

File editing uses `$VISUAL`, then `$EDITOR`. With neither set, Windows uses
Notepad; Linux and macOS use the first installed editor from `vi`, `vim`,
`nvim`, and `nano`. Editor commands may include quoted arguments.

## Keybindings

Global (work regardless of which panel is focused):

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` or `l` / `h` | cycle focused panel |
| `1`-`6` | jump directly to Status, Files, Branches, History, Diff, or Command Log |
| `[` / `]` | cycle Branches sub-tab (Local / Remotes) |
| `p` / `P` | pull / push the current branch |
| `/` | filter the focused list |
| `v` | select mode (release mouse capture to copy text with your terminal) |
| Mouse click | focus a panel / select a row |
| Mouse wheel | scroll whichever panel is under the cursor, without changing focus or selection |
| `?` | full keybindings help |
| `q` / `Ctrl+C` | quit |

Press `Enter` to apply a filter, including zero matches. `Esc` clears it.

The bottom-left footer shows the current action or background load with animated dots every 180 ms. Overlapping activities stay tracked until each finishes; the newest appears first, then any earlier activity resumes. Shortcuts beside it appear only when the complete entry fits.

When the terminal cannot fit the panels or an open prompt, resize it to continue. Prompts retain their text, and only `q` / `Ctrl+C` remain active while controls are hidden.

Files panel:

| Key | Action |
|---|---|
| `↑↓` / `j` `k` | move selection |
| `Space` | stage / unstage the selected file, or a whole folder recursively |
| `a` | stage / unstage everything |
| `Enter` | expand/collapse a folder, or view the selected file's diff |
| `c` | commit staged changes (opens a message prompt) |
| `e` | edit the file in `$VISUAL`/`$EDITOR` |
| `d` | discard menu (`x` discard all, `u` discard unstaged - folders with a mix of staged/unstaged files only) |
| `D` | confirm discarding changes in the files listed when the prompt opens |
| `L` | toggle a file lock |

Folder discard captures its file list when the menu opens, preserving later
changes in other files. Discarding unstaged changes also preserves files that
become staged while the menu is open.
If a listed file becomes a directory, discard stops so its new contents stay intact.

Branches panel:

| Key | Action |
|---|---|
| `Space` | checkout the selected branch |
| `n` | create a new branch (opens a name prompt) |
| `g` | reset the current branch to the selected branch |
| `M` | merge the selected branch into the current branch |

History panel:

| Key | Action |
|---|---|
| `Space` | checkout the selected revision |
| `d` | drop (revert) the selected revision |
| `g` | reset the current branch to the selected revision |

## Development

```bash
go build -o bin/lazylore.exe ./cmd/lazylore   # build
go run ./cmd/lazylore                         # build and run
go test ./...                                 # run the test suite
go vet ./...                                  # static checks
gofmt -l .                                    # list any unformatted files (gofmt -w . to fix)
```

To exercise the real CLI and a disposable local server, set absolute paths
to matching Epic Lore binaries:

```bash
LORE_TEST_BINARY=/path/to/lore LORE_TEST_SERVER=/path/to/loreserver go test ./internal/lore -run TestNativeWorkflow -v
```

The native test creates its own repository, server storage, and configuration,
and removes them afterward. It is skipped when the binary paths are unset.
