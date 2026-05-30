Tigo
====

TODO/Task management program written in Go, featuring a Terminal User Interface (TUI).

Features
--------

- **Store Tasks Locally**: Keeps tasks as `TASK.md` markdown files within structured
  directories (YYYYMMDD-HHMMSS).
- **TUI interface**: Uses `wesome-gocui/gocui` for a responsive and intuitive layout.
- **Git Integration**: Your tasks can be easily version controlled, and Tigo includes
  wrappers to interact with git.

Installation
------------

Ensure you have [Go](https://go.dev/) installed.

```bash
git clone https://github.com/MPCodeWriter21/Tigo.git
cd Tigo
go build -o tigo ./cmd/tigo
```

Usage
-----

```text
tigo [root]

By default tigo looks for a .tigo directory in the current working directory
and use that as the root directory of tasks. If .tigo does not exist, it will
use $HOME/.local/share/tigo.

    -h --help       Show the help
    -v --version    Show the version
```

### Keybindings (TUI)

- q / Ctrl+C: Quit Tigo
- n: Create a new task
- d: Delete the selected task
- Space: Toggle task status (OPEN/CLOSED)
- H: Hide/show CLOSED tasks
- j / ArrowDown: Cursor down in the task list
- k / ArrowUp: Cursor up in the task list
- g / G: Jump to the top/bottom of the task list
- Enter: Submit dialogs
- Esc: Cancel dialogs

Task Format
-----------

Each task directory contains a TASK.md containing task metadata and a description.

```md
# <title-of-the-task>

- STATUS: OPEN
- PRIORITY: 70
[- TAGS: bug, UI]

[description-of-the-task]
```

Inspired by
-----------

- [lazygit](https://github.com/jesseduffield/lazygit)
- [ticko](https://github.com/CESA-UT/os-lab-1404-Ticko-TUI)
- [Tsoding's task database](https://www.youtube.com/watch?v=QH6KOEVnSZA)
