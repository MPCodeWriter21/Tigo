Tigo
====

TODO/Task management program written in Go, featuring a Terminal User Interface (TUI).

Features
--------

- **Store Tasks Locally**: Keeps tasks as `TASK.md` files within structured
  directories (YYYYMMDD-HHmmss).
- **TUI interface**: Uses `wesome-gocui/gocui` for a responsive and intuitive layout.
- **Git Integration**: Your tasks can be easily version controlled, and Tigo includes
  wrappers to interact with git.

Installation
------------

Ensure you have [Go](https://go.dev/) installed.

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
- `n`: Create a new task
- `e`: Edit the selected task
- `d`: Delete the selected task
- `H`: Hide/show CLOSED tasks
- `j` / `<arrow-down>`: Cursor down in the task list
- `k` / `<arrow-up>`: Cursor up in the task list
- `h` / `<arrow-left>`: Move to the previous selectable item / focus tasks list
- `l` / `<arrow-right>`: Move to the next selectable item
- `g` / `G`: Jump to the top/bottom
- `/`: Search tasks by title, description or tags (supports RegEx)
- `s`: Sort tasks by priority, due date, ID or title
- `y`: Yank (copy) the selected task's content to the clipboard
- `Y`: Yank the whole current line
- `o`: Open the tasks containing directory in the file explorer
- `O`: Open the selected task's TASK.md in the default editor
- `r`: Refresh the task list (useful if tasks are modified outside Tigo)
- `` ` ``: Show the current Tigo directory
- `<space>`: Toggle task status (OPEN/CLOSED) / Follow hyperlink
- `<tab>`: Switch between different views (e.g., task list, task details)
- `<enter>`: Submit dialogs / Go to the selected task's details
- `<esc>`: Cancel dialogs / Clear search / Exit task details view

Task Format
-----------

Each task directory contains a TASK.md containing task metadata and a description.

```md
# <title-of-the-task>

- STATUS: OPEN
- PRIORITY: 70
[- TAGS: bug, UI]
[- DUE: 2026-05-11]

[description-of-the-task]
```

Inspired by
-----------

- [lazygit](https://github.com/jesseduffield/lazygit)
- [ticko](https://github.com/CESA-UT/os-lab-1404-Ticko-TUI)
- [Tsoding's task database](https://www.youtube.com/watch?v=QH6KOEVnSZA)
