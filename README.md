Tigo
====

TODO/Task management program written in Go, featuring a Terminal User Interface (TUI).

Features
--------

- **Store Tasks Locally**: Keeps tasks as `TASK.md` files within structured
  directories (`YYYYMMDD-HHmmss`). Portable, grep-able, and git-friendly.
- **TUI interface**: Uses `awesome-gocui/gocui` for a responsive and intuitive
  three-panel layout (tasks list, details, logs).
- **Git Integration**: Tracks session changes and provides a commit dialog with
  auto-filled messages.
- **Git Blame**: Per-line metadata and description blame via `git blame`,
  accessible with the `b` keybinding from any view.
- **Hyperlinks**: Task references (`TASK(20260601-123456)`), URLs, file paths,
  and tags are recognized and clickable in the details view.
- **Relative Dates**: Supports "tomorrow", "next week", "3 days", "2 months",
  etc. when setting due dates.
- **Search**: RegEx-powered search across title, description, and tags. Filter
  by tag by clicking on tag hyperlinks.
- **Clipboard**: Yank individual detail fields or entire lines to the clipboard.
- **Configurable**: YAML config with per-directory overrides for sort order,
  default priority, frame style, and showing closed tasks.

Installation
------------

### Download from GitHub Releases

Pre-built binaries and packages are available for download from the
[releases page](https://github.com/MPCodeWriter21/Tigo/releases).
Supported platforms:

| Platform | amd64 | arm64 | arm |
|----------|-------|-------|-----|
| Linux    |   ✓   |   ✓   |  ✓  |
| Windows  |   ✓   |   ✓   |     |
| macOS    |   ✓   |   ✓   |     |
| FreeBSD  |   ✓   |   ✓   |  ✓  |
| NetBSD   |   ✓   |   ✓   |  ✓  |
| Android  |       |   ✓   |     |

Download the binary for your platform, rename it to `tigo`, place it somewhere
in your `PATH`, and use it wherever you want.

### Install via `go install`

If you have [Go](https://go.dev/) installed:

```bash
go install github.com/MPCodeWriter21/Tigo/cmd/tigo@latest
```

### Build from Source

```bash
git clone https://github.com/MPCodeWriter21/Tigo.git
cd Tigo
go install ./cmd/tigo

# Directly run without installing
go run ./cmd/tigo
```

Usage
-----

```text
tigo [root]

By default tigo looks for a `.tigo` directory in the current working directory
and use that as the root directory of tasks. If `.tigo` does not exist, it will
use `$HOME/.local/share/tigo`.

    -h --help             Show this help and exit
    -v --version          Show the version and exit
    --default-config      Print the default configuration and exit
    --user-config-path    Print the path to the user configuration and exit
```

### Keybindings (TUI)

- `q` / `Ctrl+C`: Quit Tigo
- `?`: Show the help dialog
- `n`: Create a new task
- `e`: Edit the selected task
- `d`: Delete the selected task
- `H`: Hide/show CLOSED tasks
- `j` / `<arrow-down>`: Cursor down in the task list
- `k` / `<arrow-up>`: Cursor up in the task list
- `h` / `<arrow-left>`: Move to the previous selectable item / focus tasks list
- `l` / `<arrow-right>`: Move to the next selectable item / details view
- `g` / `G`: Jump to the top/bottom
- `L`: Focus the logs view
- `b`: Show blame summary for the selected task or line
- `p`: Push local commits to the remote
- `P`: Pull commits from the remote
- `/`: Search tasks by title, description or tags (supports RegEx)
- `s`: Sort tasks by priority, due date, ID or title
- `y`: Yank (copy) the selected task's content to the clipboard
- `Y`: Yank the whole current line
- `o`: Open the tasks containing directory in the file explorer
- `O`: Open the selected task's TASK.md in the default editor
- `r`: Refresh the task list (useful if tasks are modified outside Tigo)
- `<alt+c>`: Open local Tigo config in the default editor
- `<alt+r>`: Reload the Tigo configuration from disk
- `` ` ``: Show the current Tigo directory
- `<space>`: Toggle task status (OPEN/CLOSED) / Follow hyperlink
- `<tab>`: Switch between different views (e.g., task list, task details)
- `<enter>`: Submit dialogs / Go to the selected task's details
- `<esc>`: Cancel dialogs / Clear search / Exit task details view

Task Format
-----------

Each task directory contains a `TASK.md` containing task metadata and a description.

```md
# <title-of-the-task>

- STATUS: OPEN
- PRIORITY: 70
[- TAGS: bug, UI]
[- DUE: 2026-05-11]

[description-of-the-task]
```

`STATUS` can be `OPEN`, `CLOSED`, or any custom workflow state. `PRIORITY` is an
integer (higher = more important).
`DUE` accepts absolute dates (`2026-05-11`, `2026-05-11 23:59`) but you can enter
relative expressions (`tomorrow`, `next week`, `3 days`, `2 months`) in the TUI.

Configuration
-------------

Tigo looks for config in this order:

1. `$XDG_CONFIG_HOME/tigo/config.yaml` (or `~/.config/tigo/config.yaml`)
2. `.tigo/config.yaml` in the current working directory (overrides user config for that directory)
3. `$HOME/.local/share/tigo/config.yaml`

```yaml
sort_by: id              # Sort tasks by id, priority, due-date, or title
default_priority: 50     # The default priority for new tasks
frame_style: round       # The style of the frames (round, double, single)
show_closed: false       # Whether to show closed tasks in the list by default
due_color_enabled: true  # Whether to color the due date based on how close it is
```

Git Integration
---------------

Tigo tracks session changes (create, edit, delete, toggle) and autofills the
commit dialog. The dialog has three panels: a file list, commit subject,
and description body. Tab cycles through them.

At startup, Tigo fetches from the remote for all git repos. The status bar
shows `↓N↑M` when ahead/behind, or `push`/`pull` during background operations.
`p` pulls, `P` pushes.

Press `b` for git blame: task-level summary in the tasks view, per-line blame
in the details view.

Git commands are logged in the logs view (`L` to focus, `C` to clear).

Inspired by
-----------

- [lazygit](https://github.com/jesseduffield/lazygit)
- [ticko](https://github.com/CESA-UT/os-lab-1404-Ticko-TUI)
- [Tsoding's task database](https://www.youtube.com/watch?v=QH6KOEVnSZA)
