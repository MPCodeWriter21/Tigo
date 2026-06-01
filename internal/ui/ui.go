/* Package ui contains TUI Code for Tigo */
package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"tigo/pkg/db"
	"tigo/pkg/task"

	"github.com/awesome-gocui/gocui"
)

var (
	tigoRoot     string
	tasks        []*task.Task
	sortBy       string = "id"
	showClosed   bool   = false
	selectedTask int    = 0
	searchQuery  string
)

type keybinding struct {
	view    string // empty string means global
	key     any    // gocui.Key or rune
	mod     gocui.Modifier
	handler func(*gocui.Gui, *gocui.View) error
}

// Run initializes and runs the GUI.
func Run(root string) error {
	// Enter Alternate Screen Buffer (hide main terminal content)
	fmt.Print("\x1b[?1049h")
	defer fmt.Print("\x1b[?1049l")

	tigoRoot = root

	g, err := gocui.NewGui(gocui.OutputNormal, true)
	if err != nil {
		return err
	}
	defer g.Close()

	gocui.DefaultEditor = gocui.EditorFunc(simpleEditor)
	g.SetManagerFunc(layout)

	if err := initKeybindings(g); err != nil {
		return err
	}

	if err := loadTasks(); err != nil {
		return err
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}

	return nil
}

func loadTasks() error {
	taskIDs, err := db.DiscoverTasks(tigoRoot)
	if err != nil {
		return err
	}

	tasks = make([]*task.Task, 0, len(taskIDs))
	for _, id := range taskIDs {
		t, err := task.Parse(id, filepath.Join(tigoRoot, id, "TASK.md"))
		if err == nil {
			if !showClosed && t.Status == "CLOSED" {
				continue
			}
			if searchQuery != "" {
				matched, err := regexp.MatchString("(?i)"+searchQuery, t.Title+t.Description+strings.Join(t.Tags, " "))
				if err != nil || !matched {
					continue
				}
			}
			tasks = append(tasks, t)
		}
	}
	switch sortBy {
	case "id":
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].ID < tasks[j].ID
		})
	case "priority":
		sort.Slice(tasks, func(i, j int) bool {
			// Higher priority tasks should come first, so we use > instead of <.
			return tasks[i].Priority > tasks[j].Priority
		})
	case "title":
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].Title < tasks[j].Title
		})
	default:
		return fmt.Errorf("invalid sort option: %s", sortBy)
	}
	selectedTask = min(selectedTask, len(tasks)-1)
	return nil
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	frameRunes := []rune{'─', '│', '╭', '╮', '╰', '╯'}
	for _, view := range g.Views() {
		view.FrameRunes = frameRunes
		view.SelBgColor = gocui.ColorCyan
		view.FrameColor = gocui.ColorWhite
	}
	if currentView := g.CurrentView(); currentView != nil {
		currentView.FrameColor = gocui.ColorGreen
	}

	if v, err := g.SetView("tasks", 0, 0, maxX/3-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Tasks"
		v.FgColor = gocui.ColorWhite
		if _, err := g.SetCurrentView("tasks"); err != nil {
			return err
		}
	}

	if v, err := g.SetView("details", maxX/3, 0, maxX-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Details"
		v.Wrap = true
	}

	if v, err := g.SetView("help", -1, maxY-2, maxX, maxY, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.BgColor = gocui.ColorBlack
		v.FgColor = gocui.ColorBlue
	}

	if searchQuery != "" {
		if v, err := g.SetView("search", 0, maxY-4, maxX/3-1, maxY-2, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = "/"
			v.Wrap = true
			v.Editable = true
		}
	}

	return updateViews(g)
}

func updateViews(g *gocui.Gui) error {
	tasksView, err := g.View("tasks")
	if err != nil {
		return err
	}
	detailsView, err := g.View("details")
	if err != nil {
		return err
	}
	helpView, err := g.View("help")
	if err != nil {
		return err
	}

	// tasks view
	ox, oy := tasksView.Origin()
	tasksWidth, tasksHeight := tasksView.Size()
	tasksView.Clear()
	if len(tasks) > 0 {
		tasksView.Highlight = true
		for _, t := range tasks {
			text := fmt.Sprintf(" [%s] %s", t.Status, t.Title)
			pad := strings.Repeat(" ", max(0, tasksWidth-len(text)))
			if t.Status == "CLOSED" {
				text = fmt.Sprintf("\x1b[32m%s\x1b[0m", text)
			} else if t.Status != "OPEN" {
				text = fmt.Sprintf("\x1b[35m%s\x1b[0m", text)
			}
			searchedFprintf(tasksView, "%s%s\n", text, pad)
		}
		if selectedTask < oy+3 {
			oy = selectedTask - 3
		}
		if selectedTask > oy+tasksHeight-3 {
			oy = selectedTask - tasksHeight + 3
		}
		if oy < 0 {
			oy = 0
		}
		tasksView.SetOrigin(ox, oy)
		tasksView.SetCursor(0, selectedTask-oy)
	} else {
		tasksView.Highlight = false
		fmt.Fprintln(tasksView, "\x1b[31mNo tasks found.\x1b[0m")
		if searchQuery != "" {
			fmt.Fprintf(tasksView, "Search query: \x1b[32m\"%s\"\x1b[0m\n", searchQuery)
		}
		fmt.Fprintf(tasksView, "Directory: \x1b[32m\"%s\"\x1b[0m\n", tigoRoot)
		fmt.Fprintln(tasksView, "\n\x1b[34mPress 'n' to create a new task.\x1b[0m")
	}

	// details view
	cx, cy := detailsView.Cursor()
	detailsView.Clear()
	if len(tasks) > 0 && selectedTask >= 0 && selectedTask < len(tasks) {
		detailsView.FgColor = gocui.ColorWhite
		t := tasks[selectedTask]
		searchedFprintf(detailsView, "ID: %s\n", t.ID)
		searchedFprintf(detailsView, "Title: %s\n", t.Title)
		searchedFprintf(detailsView, "Status: %s\n", t.Status)
		searchedFprintf(detailsView, "Priority: %d\n", t.Priority)
		searchedFprintf(detailsView, "Tags: %v\n", t.Tags)
		searchedFprintf(detailsView, "\nDescription:\n%s\n", t.Description)
	} else {
		detailsView.FgColor = gocui.ColorRed
		fmt.Fprintln(detailsView, "No task selected.")
	}
	if g.CurrentView() != detailsView {
		detailsView.Highlight = false
	} else {
		detailsView.Highlight = true
		detailsView.SetCursor(cx, cy)
	}

	// help view
	var (
		spaceKeyText string
		hKeyText     string
	)
	if len(tasks) > 0 && selectedTask >= 0 && selectedTask < len(tasks) {
		switch tasks[selectedTask].Status {
		case "CLOSED":
			spaceKeyText = "| <space>: Open "
		case "OPEN":
			spaceKeyText = "| <space>: Close "
		}
	}
	if showClosed {
		hKeyText = "Hide"
	} else {
		hKeyText = "Show"
	}
	helpText := fmt.Sprintf(" e: Edit | d: Delete %s| H: %s CLOSED | /: Search | \u2191/\u2193 j/k: Navigate | g/G: Top/Bottom", spaceKeyText, hKeyText)
	if helpText != helpView.Buffer() {
		helpView.Clear()
		fmt.Fprint(helpView, helpText)
	}

	return nil
}

func initKeybindings(g *gocui.Gui) error {
	bindings := []keybinding{
		{"", gocui.KeyCtrlC, gocui.ModNone, quit},
		{"", 'q', gocui.ModNone, quit},
		{"", '/', gocui.ModNone, promptSearch},
		{"details", 'y', gocui.ModNone, copyLine},
		{"details", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("tasks")},
		{"details", 'h', gocui.ModNone, setCurrentViewCallback("tasks")},
		{"details", gocui.KeyArrowDown, gocui.ModNone, cursorDown},
		{"details", 'j', gocui.ModNone, cursorDown},
		{"details", gocui.KeyArrowUp, gocui.ModNone, cursorUp},
		{"details", 'k', gocui.ModNone, cursorUp},
		{"details", gocui.KeyEsc, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { searchQuery = ""; return loadTasks() }},
		{"tasks", 'y', gocui.ModNone, copyLine},
		{"tasks", gocui.KeyTab, gocui.ModNone, showDetails},
		{"tasks", 'l', gocui.ModNone, showDetails},
		{"tasks", gocui.KeyArrowDown, gocui.ModNone, tasksDown},
		{"tasks", 'j', gocui.ModNone, tasksDown},
		{"tasks", gocui.KeyArrowUp, gocui.ModNone, tasksUp},
		{"tasks", 'k', gocui.ModNone, tasksUp},
		{"tasks", gocui.KeySpace, gocui.ModNone, toggleTaskStatus},
		{"tasks", 'n', gocui.ModNone, promptCreateTask},
		{"tasks", 'e', gocui.ModNone, promptEditTask},
		{"tasks", 'd', gocui.ModNone, promptDeleteTask},
		{"tasks", 's', gocui.ModNone, promptSort},
		{"tasks", 'g', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { selectedTask = 0; return updateViews(g) }},
		{"tasks", 'G', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { selectedTask = len(tasks) - 1; return updateViews(g) }},
		{"tasks", 'H', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { showClosed = !showClosed; return loadTasks() }},
		{"tasks", gocui.KeyEsc, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { searchQuery = ""; g.DeleteView("search"); return loadTasks() }},
	}

	for _, binding := range bindings {
		if err := g.SetKeybinding(binding.view, binding.key, binding.mod, binding.handler); err != nil {
			return fmt.Errorf("bind %v to view %q: %w", binding.key, binding.view, err)
		}
	}
	return nil
}

func tasksDown(g *gocui.Gui, v *gocui.View) error {
	if selectedTask < len(tasks)-1 {
		selectedTask++
	}
	return updateViews(g)
}

func tasksUp(g *gocui.Gui, v *gocui.View) error {
	if selectedTask > 0 {
		selectedTask--
	}
	return updateViews(g)
}

func showDetails(g *gocui.Gui, v *gocui.View) error {
	detailsView, err := g.SetCurrentView("details")
	if err != nil {
		return err
	}
	return detailsView.SetCursor(0, 0)
}
