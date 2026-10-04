# lazylore

A terminal UI for [Lore](https://github.com/EpicGames/lore), Epic Games' open
source version control system - the same relationship
[lazygit](https://github.com/jesseduffield/lazygit) has to `git`.

![Screenshot](.github/Screenshot.png)

## Status

Pre-1.0. Covers the daily-driver core loop: viewing status, staging/unstaging
files, discarding changes, committing, viewing diffs, switching/creating/
resetting and merging branches, browsing revision history, pulling, pushing,
and file locking. `layers` and the remaining `lore` commands are not wired up
yet.

## Requirements

- A Lore installation discoverable automatically or configured through
  `lorePath` in `config.yml`, see below.
- Go 1.24.2 or newer when building from source.

## Install

Download the archive for your OS and architecture from the
[Releases page](https://github.com/Solessfir/lazylore/releases), extract
`lazylore` (`lazylore.exe` on Windows), and place it on your `PATH`.

Or build from source:

```bash
git clone https://github.com/Solessfir/lazylore.git
cd lazylore
go build -o bin/lazylore ./cmd/lazylore
```

Run the executable from a Lore working repository, using its full path if
it is not on your `PATH`. When launched outside one, it prompts for a remote
URL and clones into the current directory.

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
On Linux, `$XDG_CONFIG_HOME/lazylore` overrides the default config directory.

Lore handles repository and server configuration. For servers requiring
authentication, sign in with the Lore CLI before launching; `lazylore` uses
the existing Lore credentials and environment.

File editing uses `$VISUAL`, then `$EDITOR`. With neither set, Windows uses
Notepad; Linux and macOS use the first installed editor from `vi`, `vim`,
`nvim`, and `nano`. Editor commands may include quoted arguments.

## Keybindings

Global (during normal pane navigation):

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` or `l` / `h` | cycle focused panel |
| `1`-`6` | jump directly to Status, Files, Branches, History, Diff, or Command Log |
| `[` / `]` | cycle Branches sub-tab (Local / Remotes) |
| `p` / `P` | pull / push the current branch |
| `/` | filter the focused list |
| `v` | select mode (release mouse capture to copy text with your terminal; any key restores capture) |
| Mouse click | focus a panel / select a row |
| Mouse wheel | scroll the list or diff under the cursor, without changing focus or selection |
| `?` | full keybindings help |
| `q` / `Ctrl+C` | quit |

Press `Enter` to apply a filter, including zero matches. `Esc` clears it.

Popups use compact titled borders and contextual footer shortcuts. `Enter`
confirms and `Esc` cancels; confirmation dialogs also accept `y` and `n`.
In keybindings help, `Enter` executes the selected binding through its usual
confirmation and guards. Resizing preserves the selected help row and input text.

Scrollable panes draw their scrollbar on the border. Below 40 terminal rows,
the command log shrinks to one content row, leaving more room for the main pane.
Focus the command log with `6` to expand it.

The bottom-left footer shows the current action or background load with animated dots every 180 ms. Overlapping activities stay tracked until each finishes; the newest appears first, then any earlier activity resumes. Shortcuts beside it appear only when the complete entry fits.

The TUI inherits the terminal's default text and background, preserving transparency. Focused borders, hints, and active tabs use terminal blue; inactive borders use `#44464f`. Selected rows are bold, with a `#292a2e` background only in the focused pane. Semantic status and diff colors follow the terminal palette.

When the terminal cannot fit the panels or an open prompt, resize it to continue. Prompts retain their text, and only `q` / `Ctrl+C` remain active while controls are hidden.

Files panel:

| Key | Action |
|---|---|
| `↑↓` / `j` `k` | move selection |
| `Space` | stage / unstage the selected file, or a whole folder recursively |
| `a` | stage / unstage everything |
| `Enter` | expand/collapse a folder, or view the selected file's diff |
| `c` | commit staged changes; if none are staged, confirm staging all changes first |
| `e` | edit the file in `$VISUAL`/`$EDITOR` |
| `d` | discard menu (`x` discard all, `u` discard unstaged - folders with a mix of staged/unstaged files only) |
| `D` | confirm discarding changes in the files listed when the prompt opens |
| `L` | lock / unlock a file; another owner's lock requires force-unlock confirmation |

The discard menu supports Up/Down or `k`/`j`, Enter/Space to execute the selected
option, and Esc to cancel. Its description follows the selection and explains
disabled options. The `x` and `u` shortcuts execute their options directly.

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

Reset (`g`) moves the current branch's latest revision pointer; it does not
reset working files.

History panel:

| Key | Action |
|---|---|
| `Space` | checkout the selected revision |
| `d` | revert the selected revision by creating a new revision |
| `g` | reset the current branch to the selected revision |

Clean merges and reverts commit automatically. Use the Lore CLI to resolve
or abort conflicts; the TUI reports them as errors.

## Development

```bash
go build -o bin/lazylore ./cmd/lazylore       # build
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

## Releases

Pushing a version tag beginning with `v` runs the release workflow. It runs
tests and vet, then builds Linux, Windows, and macOS binaries for amd64 and
arm64 and uploads the archives and `checksums.txt` to a GitHub release.

After pushing the changes to GitHub:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Use a new semantic version for each release. GitHub's built-in `GITHUB_TOKEN`
handles publishing; no additional secret is required.

## License

Licensed under the [MIT License](LICENSE).
