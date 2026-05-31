package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"tigo/pkg/db"
	"tigo/pkg/task"

	"github.com/awesome-gocui/gocui"
)

func promptCreateTask(g *gocui.Gui, v *gocui.View) error {
	return _promptTask(g,
		func(title string, priority int, tags []string, description string) error {
			if title == "" {
				return nil
			}

			_, err := db.CreateNewTask(tigoRoot, title, priority, tags, description)
			if err != nil {
				return err
			}

			selected = len(tasks) - 1
			return nil
		},
		"", 50, []string{}, "")
}

func promptEditTask(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) == 0 && selected > len(tasks) {
		return nil
	}
	t := tasks[selected]

	return _promptTask(g,
		func(title string, priority int, tags []string, description string) error {
			if title == "" {
				return fmt.Errorf("title cannot be empty")
			}

			t.Title = title
			t.Priority = priority
			t.Tags = tags
			t.Description = description

			task.Serialize(t, filepath.Join(tigoRoot, t.ID, "TASK.md"))

			return nil
		},
		t.Title, t.Priority, t.Tags, t.Description)
}

func _promptTask(
	g *gocui.Gui,
	successCallback func(title string, priority int, tags []string, description string) error,
	title string, priority int, tags []string, description string) error {

	maxX, maxY := g.Size()
	widthTitle := maxX / 2
	widthPriority := maxX / 6
	heightTitle := 3
	heightDesc := 6
	heightPriority := heightTitle
	heightTags := heightDesc
	x0 := maxX/2 - widthTitle/2 - widthPriority/2
	y0 := maxY/2 - heightTitle/2 - heightDesc/2
	g.Cursor = true

	if v, err := g.SetView("createDialogTitle", x0, y0, x0+widthTitle-1, y0+heightTitle-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Title"
		v.Editable = true
		v.Wrap = false
		fmt.Fprint(v, title)
		v.SetCursor(len(title), 0)

		if _, err := g.SetCurrentView("createDialogTitle"); err != nil {
			return err
		}

		g.SetKeybinding("createDialogTitle", gocui.KeyEnter, gocui.ModNone, _submitPromptTaskCallback(successCallback))
		g.SetKeybinding("createDialogTitle", gocui.KeyEsc, gocui.ModNone, closePromptTaskDialog)
		g.SetKeybinding("createDialogTitle", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("createDialogDescription"))
		g.SetKeybinding("createDialogTitle", gocui.KeyCtrlJ, gocui.ModNone, setCurrentViewCallback("createDialogDescription"))
		g.SetKeybinding("createDialogTitle", gocui.KeyCtrlL, gocui.ModNone, setCurrentViewCallback("createDialogPriority"))
	}
	if v, err := g.SetView("createDialogDescription", x0, y0+heightTitle, x0+widthTitle-1, y0+heightTitle+heightDesc-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Description"
		v.Editable = true
		v.Wrap = true
		fmt.Fprint(v, description)
		v.SetCursor(len(description), strings.Count(description, "\n"))

		g.SetKeybinding("createDialogDescription", gocui.KeyEsc, gocui.ModNone, closePromptTaskDialog)
		g.SetKeybinding("createDialogDescription", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("createDialogPriority"))
		g.SetKeybinding("createDialogDescription", gocui.KeyCtrlK, gocui.ModNone, setCurrentViewCallback("createDialogTitle"))
		g.SetKeybinding("createDialogDescription", gocui.KeyCtrlL, gocui.ModNone, setCurrentViewCallback("createDialogTags"))
		g.SetKeybinding("createDialogDescription", gocui.KeyEnter, gocui.ModShift, _submitPromptTaskCallback(successCallback))
	}
	if v, err := g.SetView("createDialogPriority", x0+widthTitle, y0, x0+widthTitle+widthPriority-1, y0+heightPriority-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Priority"
		v.Editable = false
		v.Wrap = false

		priority := strconv.Itoa(priority)
		printPriority := func() error {
			v.Clear()
			padding, err := centeredFprintf(v, "%s", priority)
			if err != nil {
				return err
			}
			return v.SetCursor(padding+len(priority), 0)
		}

		if err := printPriority(); err != nil {
			return err
		}

		g.SetKeybinding("createDialogPriority", gocui.KeyEnter, gocui.ModNone, _submitPromptTaskCallback(successCallback))
		g.SetKeybinding("createDialogPriority", gocui.KeyEsc, gocui.ModNone, closePromptTaskDialog)
		g.SetKeybinding("createDialogPriority", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("createDialogTags"))
		g.SetKeybinding("createDialogPriority", gocui.KeyCtrlJ, gocui.ModNone, setCurrentViewCallback("createDialogTags"))
		g.SetKeybinding("createDialogPriority", gocui.KeyCtrlH, gocui.ModNone, setCurrentViewCallback("createDialogTitle"))

		// Set keybinds for 0-9 and backspace to modify the priority
		for i := '0'; i <= '9'; i++ {
			digit := rune(i)
			g.SetKeybinding("createDialogPriority", digit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
				if len(priority) < 10 {
					priority += string(digit)
					priority = strings.TrimLeft(priority, "0")
				}
				return printPriority()
			})
		}
		g.SetKeybinding("createDialogPriority", gocui.KeyBackspace, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			if len(priority) > 0 {
				priority = priority[:len(priority)-1]
			}
			return printPriority()
		})
	}
	if v, err := g.SetView("createDialogTags", x0+widthTitle, y0+heightPriority, x0+widthTitle+widthPriority-1, y0+heightPriority+heightTags-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Tags (comma-separated)"
		v.Editable = true
		v.Wrap = true
		fmt.Fprint(v, strings.Join(tags, ", "))
		v.SetCursor(len(v.Buffer()), 0)

		g.SetKeybinding("createDialogTags", gocui.KeyEsc, gocui.ModNone, closePromptTaskDialog)
		g.SetKeybinding("createDialogTags", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("createDialogTitle"))
		g.SetKeybinding("createDialogTags", gocui.KeyCtrlK, gocui.ModNone, setCurrentViewCallback("createDialogPriority"))
		g.SetKeybinding("createDialogTags", gocui.KeyCtrlH, gocui.ModNone, setCurrentViewCallback("createDialogDescription"))
		g.SetKeybinding("createDialogTags", gocui.KeyEnter, gocui.ModShift, _submitPromptTaskCallback(successCallback))
	}

	return nil
}

func _submitPromptTaskCallback(successCallback func(title string, priority int, tags []string, description string) error) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		titleView, err := g.View("createDialogTitle")
		if err != nil {
			return err
		}
		priorityView, err := g.View("createDialogPriority")
		if err != nil {
			return err
		}
		tagsView, err := g.View("createDialogTags")
		if err != nil {
			return err
		}
		descriptionView, err := g.View("createDialogDescription")
		if err != nil {
			return err
		}

		title := strings.TrimSpace(titleView.Buffer())
		priorityStr := strings.TrimSpace(priorityView.Buffer())
		var priority int
		if priority, err = strconv.Atoi(priorityStr); err != nil {
			priority = 50
		}
		tags := []string{}
		for tag := range strings.SplitSeq(tagsView.Buffer(), ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
		description := strings.TrimSpace(descriptionView.Buffer())

		if err := successCallback(title, priority, tags, description); err != nil {
			return err
		}

		if err := closePromptTaskDialog(g, v); err != nil {
			return err
		}

		// Reload
		if err := loadTasks(); err != nil {
			return err
		}

		return updateViews(g)
	}
}

func closePromptTaskDialog(g *gocui.Gui, _ *gocui.View) error {
	g.Cursor = false
	views := []string{"createDialogTitle", "createDialogDescription", "createDialogPriority", "createDialogTags"}

	for _, v := range views {
		if err := g.DeleteView(v); err != nil {
			return err
		}
		g.DeleteKeybindings(v)
	}

	if _, err := g.SetCurrentView("list"); err != nil {
		return err
	}
	return updateViews(g)
}

func promptDeleteTask(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) == 0 {
		return nil
	}

	maxX, maxY := g.Size()
	width := maxX / 2
	height := 2
	x0 := maxX/2 - width/2
	y0 := maxY/2 - height/2

	if v, err := g.SetView("deleteDialog", x0, y0, x0+width, y0+height, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Delete Task? "
		fmt.Fprintf(v, "Are you sure you want to delete this task? (Enter/Esc)")

		if _, err := g.SetCurrentView("deleteDialog"); err != nil {
			return err
		}

		g.SetKeybinding("deleteDialog", gocui.KeyEnter, gocui.ModNone, submitDeleteTask)
		g.SetKeybinding("deleteDialog", gocui.KeyEsc, gocui.ModNone, closeDialog)
	}

	return nil
}

func submitDeleteTask(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) > 0 && selected < len(tasks) {
		t := tasks[selected]

		db.DeleteTask(tigoRoot, t.ID)

		if selected > 0 {
			selected--
		}
	}

	if err := closeDialog(g, v); err != nil {
		return err
	}

	if err := loadTasks(); err != nil {
		return err
	}
	return updateViews(g)
}

func promptSearch(g *gocui.Gui, v *gocui.View) error {
	maxX, maxY := g.Size()
	g.Cursor = true
	if v, err := g.SetView("search", 0, maxY-4, maxX/3-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "/"
		v.Wrap = true
		v.Editable = true
		g.SetKeybinding("search", gocui.KeyEsc, gocui.ModNone, closeDialog)
		g.SetKeybinding("search", gocui.KeyEnter, gocui.ModNone, submitSearch)
		g.SetKeybinding("search", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			if searchQuery != "" {
				v.Clear()
				fmt.Fprint(v, searchQuery)
				v.SetCursor(len(searchQuery), 0)
			}
			return nil
		})
	}

	_, err := g.SetCurrentView("search")
	return err
}

func submitSearch(g *gocui.Gui, v *gocui.View) error {
	query := strings.TrimSpace(v.Buffer())
	v.Clear()
	fmt.Fprint(v, query)
	v.SetCursor(len(query), 0)
	searchQuery = query
	g.DeleteKeybindings(v.Name())
	g.Cursor = false

	if _, err := g.SetCurrentView("list"); err != nil {
		return err
	}
	if err := loadTasks(); err != nil {
		return err
	}
	return updateViews(g)
}

func promptSort(g *gocui.Gui, v *gocui.View) error {
	maxX, maxY := g.Size()
	width := maxX / 2
	height := 4
	x0 := maxX/2 - width/2
	y0 := maxY/2 - height/2
	if v, err := g.SetView("sort", x0, y0, x0+width, y0+height, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Sort By"
		v.Highlight = true
		v.SetCursor(0, 0)
		centeredFprintf(v, "1. Task ID \n")
		centeredFprintf(v, "2. Priority\n")
		centeredFprintf(v, "3. Title   \n")
		g.SetKeybinding("sort", gocui.KeyEsc, gocui.ModNone, closeDialog)
		g.SetKeybinding("sort", gocui.KeyEnter, gocui.ModNone, submitSort)
		g.SetKeybinding("sort", gocui.KeyArrowDown, gocui.ModNone, cursorDown)
		g.SetKeybinding("sort", 'j', gocui.ModNone, cursorDown)
		g.SetKeybinding("sort", gocui.KeyArrowUp, gocui.ModNone, cursorUp)
		g.SetKeybinding("sort", 'k', gocui.ModNone, cursorUp)
		g.SetKeybinding("sort", '1', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { v.SetCursor(0, 0); return submitSort(g, v) })
		g.SetKeybinding("sort", '2', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { v.SetCursor(0, 1); return submitSort(g, v) })
		g.SetKeybinding("sort", '3', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { v.SetCursor(0, 2); return submitSort(g, v) })
	}
	_, err := g.SetCurrentView("sort")
	return err
}

func submitSort(g *gocui.Gui, v *gocui.View) error {
	_, cy := v.Cursor()
	switch cy {
	case 0:
		sortBy = "id"
	case 1:
		sortBy = "priority"
	case 2:
		sortBy = "title"
	default:
		return fmt.Errorf("selection out of range: %d", cy)
	}
	if err := closeDialog(g, v); err != nil {
		return err
	}
	if err := loadTasks(); err != nil {
		return err
	}
	return updateViews(g)
}
