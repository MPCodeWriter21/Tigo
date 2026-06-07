package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tigo/pkg/db"
	"tigo/pkg/task"

	"github.com/awesome-gocui/gocui"
)

var (
	ErrPreventDialogClose = errors.New("prevent dialog from closing")
)

func promptCreateTask(g *gocui.Gui, v *gocui.View) error {
	return _promptTask(g,
		func(title string, priority int, tags []string, dueDate string, description string) error {
			if title == "" {
				err := promptMessageBox(g, "Empty Title", "\x1b[31mA task's title cannot be empty string!", "taskDialogTitle", true)
				if err != nil {
					return err
				}
				return ErrPreventDialogClose
			}

			newTaskID, err := db.CreateNewTask(tigoRoot, title, priority, tags, dueDate, description)
			if err != nil {
				return err
			}

			loadTasks()
			// Select the newly created task
			for i, t := range tasks {
				if t.ID == newTaskID {
					selectedTask = i
					break
				}
			}

			return nil
		},
		"", cfg.DefaultPriority, []string{}, "", "")
}

func promptEditTask(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) == 0 && selectedTask > len(tasks) {
		return nil
	}
	t := tasks[selectedTask]

	return _promptTask(g,
		func(title string, priority int, tags []string, dueDate string, description string) error {
			if title == "" {
				err := promptMessageBox(g, "Empty Title", "\x1b[31mA task's title cannot be empty string!", "taskDialogTitle", true)
				if err != nil {
					return err
				}
				return ErrPreventDialogClose
			}

			t.Title = title
			t.Priority = priority
			t.Tags = tags
			t.DueDate = dueDate
			t.Description = description

			task.Serialize(t, filepath.Join(tigoRoot, t.ID, "TASK.md"))

			return nil
		},
		t.Title, t.Priority, t.Tags, t.DueDate, t.Description)
}

func _promptTask(
	g *gocui.Gui,
	successCallback func(title string, priority int, tags []string, dueDate string, description string) error,
	title string, priority int, tags []string, dueDate string, description string) error {

	maxX, maxY := g.Size()
	widthTitle := maxX / 2
	widthPriority := maxX / 6
	heightTitle := 3
	heightPriority := 3
	heightDueDate := 3
	heightTags := 3
	heightDesc := heightPriority + heightDueDate + heightTags - heightTitle // 9 - 3 = 6
	x0 := maxX/2 - widthTitle/2 - widthPriority/2
	y0 := maxY/2 - heightTitle/2 - heightDesc/2
	g.Cursor = true

	if v, err := g.SetView("taskDialogTitle", x0, y0, x0+widthTitle-1, y0+heightTitle-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Title"
		v.Editable = true
		v.Editor = gocui.EditorFunc(oneLineEditor)
		v.Wrap = false
		fmt.Fprint(v, title)
		v.SetCursor(len(title), 0)

		if _, err := g.SetCurrentView("taskDialogTitle"); err != nil {
			return err
		}

		g.SetKeybinding("taskDialogTitle", gocui.KeyEnter, gocui.ModNone, _submitPromptTaskCallback(successCallback))
		g.SetKeybinding("taskDialogTitle", gocui.KeyEsc, gocui.ModNone, closePromptTaskDialog)
		g.SetKeybinding("taskDialogTitle", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("taskDialogDescription"))
		g.SetKeybinding("taskDialogTitle", gocui.KeyCtrlJ, gocui.ModNone, setCurrentViewCallback("taskDialogDescription"))
		g.SetKeybinding("taskDialogTitle", gocui.KeyCtrlL, gocui.ModNone, setCurrentViewCallback("taskDialogPriority"))
	}
	if v, err := g.SetView("taskDialogDescription", x0, y0+heightTitle, x0+widthTitle-1, y0+heightTitle+heightDesc-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Description"
		v.Editable = true
		v.Wrap = true
		fmt.Fprint(v, description)
		v.SetCursor(len(description), strings.Count(description, "\n"))

		g.SetKeybinding("taskDialogDescription", gocui.KeyEsc, gocui.ModNone, closePromptTaskDialog)
		g.SetKeybinding("taskDialogDescription", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("taskDialogPriority"))
		g.SetKeybinding("taskDialogDescription", gocui.KeyCtrlK, gocui.ModNone, setCurrentViewCallback("taskDialogTitle"))
		g.SetKeybinding("taskDialogDescription", gocui.KeyCtrlL, gocui.ModNone, setCurrentViewCallback("taskDialogDueDate"))
		g.SetKeybinding("taskDialogDescription", gocui.KeyEnter, gocui.ModShift, _submitPromptTaskCallback(successCallback))
	}
	if v, err := g.SetView("taskDialogPriority", x0+widthTitle, y0, x0+widthTitle+widthPriority-1, y0+heightPriority-1, 0); err != nil {
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

		g.SetKeybinding("taskDialogPriority", gocui.KeyEnter, gocui.ModNone, _submitPromptTaskCallback(successCallback))
		g.SetKeybinding("taskDialogPriority", gocui.KeyEsc, gocui.ModNone, closePromptTaskDialog)
		g.SetKeybinding("taskDialogPriority", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("taskDialogDueDate"))
		g.SetKeybinding("taskDialogPriority", gocui.KeyCtrlJ, gocui.ModNone, setCurrentViewCallback("taskDialogDueDate"))
		// TODO: Add support for Ctrl+H (Stupidly enough, it overlaps with backspace and I couldn't find a way to distinguish between them)

		// Set keybinds for 0-9 and backspace to modify the priority
		for i := '0'; i <= '9'; i++ {
			digit := rune(i)
			g.SetKeybinding("taskDialogPriority", digit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
				if len(priority) < 10 {
					priority += string(digit)
					for len(priority) > 1 && priority[0] == '0' {
						priority = priority[1:]
					}
				}
				return printPriority()
			})
		}
		g.SetKeybinding("taskDialogPriority", gocui.KeyBackspace, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			if len(priority) > 0 {
				priority = priority[:len(priority)-1]
			}
			return printPriority()
		})
		// Make sure global keybindings don't interfere when priority view is focused
		g.SetKeybinding("taskDialogPriority", '/', gocui.ModNone, doNothing)
		g.SetKeybinding("taskDialogPriority", 'o', gocui.ModNone, doNothing)
		g.SetKeybinding("taskDialogPriority", '`', gocui.ModNone, doNothing)
	}
	if v, err := g.SetView("taskDialogDueDate", x0+widthTitle, y0+heightPriority, x0+widthTitle+widthPriority-1, y0+heightPriority+heightDueDate-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Due Date"
		v.Editable = true
		v.Wrap = false
		v.Editor = gocui.EditorFunc(oneLineEditor)
		fmt.Fprint(v, dueDate)
		v.SetCursor(len(dueDate), 0)

		g.SetKeybinding("taskDialogDueDate", gocui.KeyEsc, gocui.ModNone, closePromptTaskDialog)
		g.SetKeybinding("taskDialogDueDate", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("taskDialogTags"))
		g.SetKeybinding("taskDialogDueDate", gocui.KeyCtrlK, gocui.ModNone, setCurrentViewCallback("taskDialogPriority"))
		g.SetKeybinding("taskDialogDueDate", gocui.KeyCtrlJ, gocui.ModNone, setCurrentViewCallback("taskDialogTags"))
		g.SetKeybinding("taskDialogDueDate", gocui.KeyEnter, gocui.ModNone, _submitPromptTaskCallback(successCallback))
	}
	if v, err := g.SetView("taskDialogTags", x0+widthTitle, y0+heightPriority+heightDueDate, x0+widthTitle+widthPriority-1, y0+heightPriority+heightDueDate+heightTags-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Tags (comma-separated)"
		v.Editable = true
		v.Wrap = false
		v.Editor = gocui.EditorFunc(oneLineEditor)
		fmt.Fprint(v, strings.Join(tags, ", "))
		v.SetCursor(len(v.Buffer()), 0)

		g.SetKeybinding("taskDialogTags", gocui.KeyEsc, gocui.ModNone, closePromptTaskDialog)
		g.SetKeybinding("taskDialogTags", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("taskDialogTitle"))
		g.SetKeybinding("taskDialogTags", gocui.KeyCtrlK, gocui.ModNone, setCurrentViewCallback("taskDialogDueDate"))
		// TODO: Add support for Ctrl+H (Stupidly enough, it overlaps with backspace and I couldn't find a way to distinguish between them)
		g.SetKeybinding("taskDialogTags", gocui.KeyEnter, gocui.ModNone, _submitPromptTaskCallback(successCallback))
	}

	return nil
}

func _submitPromptTaskCallback(successCallback func(title string, priority int, tags []string, dueDate string, description string) error) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		titleView, err := g.View("taskDialogTitle")
		if err != nil {
			return err
		}
		priorityView, err := g.View("taskDialogPriority")
		if err != nil {
			return err
		}
		dueDateView, err := g.View("taskDialogDueDate")
		if err != nil {
			return err
		}
		tagsView, err := g.View("taskDialogTags")
		if err != nil {
			return err
		}
		descriptionView, err := g.View("taskDialogDescription")
		if err != nil {
			return err
		}

		title := strings.TrimSpace(titleView.Buffer())
		priorityStr := strings.TrimSpace(priorityView.Buffer())
		dueDate := strings.TrimSpace(dueDateView.Buffer())

		// Validate due date before proceeding
		if dueDate != "" {
			parsedRelativeDate, errRelative := parseRelativeDateTime(dueDate)
			_, errDateOnly := time.Parse("2006-01-02", dueDate)
			_, errDateTime := time.Parse("2006-01-02 15:04", dueDate)
			_, errDateTimeFull := time.Parse("2006-01-02 15:04:05", dueDate)

			if errDateOnly != nil && errDateTime != nil && errDateTimeFull != nil && errRelative != nil {
				err := promptMessageBox(
					g, "Invalid Due Date",
					"\x1b[31mUnsupported date format!\x1b[0m Valid examples:\n"+
						"\t- \x1b[34mAbsolute\x1b[0m: 2024-12-31, 2024-12-31 23:59, 2024-12-31 23:59:59\n"+
						"\t- \x1b[34mRelative\x1b[0m: today, tonight, tomorrow, next week, 1 month, 3 days, 2 weeks, etc.\n"+
						"\t- \x1b[34mEmpty\x1b[0m due means no due date",
					"taskDialogDueDate", true)
				if err != nil {
					return err
				}
				// Return nil to abort the submission and prevent the dialog from closing
				return nil
			}
			if errRelative == nil {
				dueDate = parsedRelativeDate
			}
		}

		var priority int
		if priority, err = strconv.Atoi(priorityStr); err != nil {
			priority = cfg.DefaultPriority
		}
		tags := []string{}
		for tag := range strings.SplitSeq(tagsView.Buffer(), ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
		description := strings.TrimSpace(descriptionView.Buffer())

		if err := successCallback(title, priority, tags, dueDate, description); err != nil {
			// the callback can return an special error that prevents the dialog from
			// closing, allowing us to show an error message without losing the user's input
			if err == ErrPreventDialogClose {
				return nil
			}
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
	views := []string{"taskDialogTitle", "taskDialogDescription", "taskDialogPriority", "taskDialogDueDate", "taskDialogTags"}

	for _, v := range views {
		if err := g.DeleteView(v); err != nil {
			return err
		}
		// The keybindings need to be cleared each time to make sure each Task Dialog has the correct success callback
		g.DeleteKeybindings(v)
	}

	if _, err := g.SetCurrentView("tasks"); err != nil {
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
	}

	return nil
}

func submitDeleteTask(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) > 0 && selectedTask < len(tasks) {
		t := tasks[selectedTask]

		db.DeleteTask(tigoRoot, t.ID)

		if selectedTask > 0 {
			selectedTask--
		}
	}

	if err := deleteViewDefault(g, v); err != nil {
		return err
	}

	if err := loadTasks(); err != nil {
		return err
	}
	return updateViews(g)
}

func promptSearch(g *gocui.Gui, _ *gocui.View) error {
	maxX, maxY := g.Size()
	v, err := g.SetView("search", 0, maxY-4, maxX/3-1, maxY-2, 0)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
	}
	g.Cursor = true
	v.Title = "/"
	v.Wrap = true
	v.Editable = true
	v.Editor = gocui.EditorFunc(oneLineEditor)

	_, err = g.SetCurrentView("search")
	return err
}

func searchClose(g *gocui.Gui, v *gocui.View) error {
	if searchQuery.value == "" {
		return deleteViewDefault(g, v)
	}
	v.Clear()
	fmt.Fprint(v, searchQuery.value)
	v.SetCursor(len(searchQuery.value), 0)
	g.Cursor = false
	if _, err := g.SetCurrentView("tasks"); err != nil {
		return err
	}
	return loadTasks()
}

func submitSearch(g *gocui.Gui, v *gocui.View) error {
	query := strings.TrimSpace(v.Buffer())
	v.Clear()
	fmt.Fprint(v, query)
	v.SetCursor(len(query), 0)
	searchQuery = searchQueryType{
		type_: normalQuery,
		value: query,
	}
	g.Cursor = false

	if _, err := g.SetCurrentView("tasks"); err != nil {
		return err
	}
	var selectedID string
	if len(tasks) > 0 && selectedTask < len(tasks) {
		selectedID = tasks[selectedTask].ID
	}
	if err := loadTasks(); err != nil {
		return err
	}
	// Try to keep the same task selected after searching (if it still exists)
	for i, t := range tasks {
		if t.ID == selectedID {
			selectedTask = i
			break
		}
	}
	return updateViews(g)
}

func searchCursorUp(g *gocui.Gui, v *gocui.View) error {
	if searchQuery.value != "" {
		v.Clear()
		fmt.Fprint(v, searchQuery.value)
		v.SetCursor(len(searchQuery.value), 0)
	}
	return nil
}

func promptSort(g *gocui.Gui, v *gocui.View) error {
	maxX, maxY := g.Size()
	width := maxX / 2
	height := 5
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
		centeredFprintf(v, "3. Due Date\n")
		centeredFprintf(v, "4. Title   \n")
	}
	_, err := g.SetCurrentView("sort")
	return err
}

func submitSort(g *gocui.Gui, v *gocui.View) error {
	_, cy := v.Cursor()
	switch cy {
	case 0:
		cfg.SortBy = "id"
	case 1:
		cfg.SortBy = "priority"
	case 2:
		cfg.SortBy = "due-date"
	case 3:
		cfg.SortBy = "title"
	default:
		return fmt.Errorf("selection out of range: %d", cy)
	}
	if err := deleteViewDefault(g, v); err != nil {
		return err
	}
	var selectedID string
	if len(tasks) > 0 && selectedTask < len(tasks) {
		selectedID = tasks[selectedTask].ID
	}
	if err := loadTasks(); err != nil {
		return err
	}
	// Keep the same task selected after sorting (if it still exists)
	for i, t := range tasks {
		if t.ID == selectedID {
			selectedTask = i
			break
		}
	}
	return updateViews(g)
}

func promptMessageBox(g *gocui.Gui, title, message, focusView string, focusCursor bool) error {
	if focusView == "" {
		focusView = g.CurrentView().Name()
	}
	maxX, maxY := g.Size()
	width := 0
	height := 1
	for line := range strings.SplitSeq(message, "\n") {
		cleanLine := allANSIRegex.ReplaceAllString(line, "")
		width = max(width, utf8.RuneCountInString(cleanLine)+4)
		height++
	}
	x0 := maxX/2 - width/2
	y0 := maxY/2 - height/2
	if v, err := g.SetView("messageBox", x0, y0, x0+width, y0+height, 0); err != nil {
		g.Cursor = false
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = title
		v.Editable = false
		fmt.Fprint(v, message)
		// We set the keybindings every time and clear them upon closing the message box to ensure that they always take the set the correct focusView and focusCursor values
		g.SetKeybinding(
			"messageBox", gocui.KeyEnter, gocui.ModNone,
			deleteViewAndSetCurrent(focusView, focusCursor, true))
		g.SetKeybinding(
			"messageBox", gocui.KeyEsc, gocui.ModNone,
			deleteViewAndSetCurrent(focusView, focusCursor, true))
	}
	_, err := g.SetCurrentView("messageBox")
	return err
}
