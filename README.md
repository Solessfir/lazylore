# lazylore

> [!WARNING]
> This project was built with agentic AI coding (Claude).

A terminal UI for [Lore](https://github.com/EpicGames/lore), Epic Games' open
source version control system - the same relationship
[lazygit](https://github.com/jesseduffield/lazygit) has to `git`.

## Status

Pre-1.0. Covers the daily-driver core loop: viewing status, staging/unstaging
files, discarding changes, committing, viewing diffs, switching/creating/
resetting branches, browsing revision history, and file locking. `push`,
`layers`, and everything else `lore` exposes are not wired up yet.

## Requirements

- A `lore` binary on your `PATH` (or configured via `lorePath` in
  `config.yml`, see below).
- A `lore` repository (a directory containing a `.lore/` folder) as your
  working directory when you launch `lazylore`.

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

## Keybindings

Global (work regardless of which panel is focused):

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` or `l` / `h` | cycle focused panel |
| `1`-`5` | jump to a panel directly (Status, Files, Branches, History, Diff) |
| `[` / `]` | cycle Branches sub-tab (Local / Remotes) |
| `/` | filter the focused list |
| `v` | select mode (release mouse capture to copy text with your terminal) |
| Mouse click | focus a panel / select a row |
| Mouse wheel | scroll whichever panel is under the cursor, without changing focus or selection |
| `?` | full keybindings help |
| `q` / `Ctrl+C` | quit |

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
| `D` | discard ALL changes in the working tree |
| `L` | toggle a file lock |

Branches panel:

| Key | Action |
|---|---|
| `Space` | checkout the selected branch |
| `n` | create a new branch (opens a name prompt) |
| `g` | reset the current branch to the selected branch |

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
