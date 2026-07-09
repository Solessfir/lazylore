# lazylore

A terminal UI for [Lore](https://github.com/EpicGames/lore), Epic Games' open
source version control system - the same relationship
[lazygit](https://github.com/jesseduffield/lazygit) has to `git`.

## Status

Pre-1.0. Covers the daily-driver core loop: viewing status, staging/unstaging
files, committing, viewing diffs, and switching/creating branches. `push`,
`sync`, locks, layers, and everything else `lore` exposes are not wired up
yet.

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

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | cycle focused panel |
| `↑↓` / `j` `k` | navigate the focused list |
| `Space` | stage / unstage the selected file |
| `c` | commit staged changes (opens a message prompt) |
| `n` | create a new branch (opens a name prompt) |
| `Enter` | view diff (Files panel) or switch to branch (Branches panel) |
| `d` | reset/discard the selected file's changes |
| `q` / `Ctrl+C` | quit |

## Development

```bash
just build   # go build -> bin/lazylore.exe
just run     # build and run
just test    # go test ./...
```
