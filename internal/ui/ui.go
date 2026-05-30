/* Package ui contains TUI Code for Tigo */
package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"tigo/pkg/db"
	"tigo/pkg/task"

	"github.com/awesome-gocui/gocui"
)

var (
	tigoRoot    string
	tasks       []*task.Task
	selected    int  = 0
	showClosed  bool = false
	searchQuery string
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
	selected = min(selected, len(tasks)-1)
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

	if v, err := g.SetView("list", 0, 0, maxX/3-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Tasks"
		v.FgColor = gocui.ColorWhite
		if _, err := g.SetCurrentView("list"); err != nil {
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

	return updateViews(g)
}

func updateViews(g *gocui.Gui) error {
	listView, err := g.View("list")
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

	// list view
	ox, oy := listView.Origin()
	listWidth, listHeight := listView.Size()
	listView.Clear()
	if len(tasks) > 0 {
		listView.Highlight = true
		for _, t := range tasks {
			text := fmt.Sprintf(" [%s] %s", t.Status, t.Title)
			pad := strings.Repeat(" ", max(0, listWidth-len(text)))
			if t.Status == "CLOSED" {
				text = fmt.Sprintf("\x1b[32m%s\x1b[0m", text)
			} else if t.Status != "OPEN" {
				text = fmt.Sprintf("\x1b[35m%s\x1b[0m", text)
			}
			searchedFprintf(listView, "%s%s\n", text, pad)
		}
		if selected < oy+3 {
			oy = selected - 3
		}
		if selected > oy+listHeight-3 {
			oy = selected - listHeight + 3
		}
		if oy < 0 {
			oy = 0
		}
		listView.SetOrigin(ox, oy)
		listView.SetCursor(0, selected-oy)
	} else {
		listView.Highlight = false
		fmt.Fprintln(listView, "\x1b[31mNo tasks found.\x1b[0m")
		if searchQuery != "" {
			fmt.Fprintf(listView, "Search query: \x1b[32m\"%s\"\x1b[0m\n", searchQuery)
		}
		fmt.Fprintf(listView, "Directory: \x1b[32m\"%s\"\x1b[0m\n", tigoRoot)
		fmt.Fprintln(listView, "\n\x1b[34mPress 'n' to create a new task.\x1b[0m")
	}

	// details view
	cx, cy := detailsView.Cursor()
	detailsView.Clear()
	if len(tasks) > 0 && selected >= 0 && selected < len(tasks) {
		detailsView.FgColor = gocui.ColorWhite
		t := tasks[selected]
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
	if len(tasks) > 0 && selected >= 0 && selected < len(tasks) {
		switch tasks[selected].Status {
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
		{"details", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("list")},
		{"details", 'h', gocui.ModNone, setCurrentViewCallback("list")},
		{"details", gocui.KeyArrowDown, gocui.ModNone, cursorDown},
		{"details", 'j', gocui.ModNone, cursorDown},
		{"details", gocui.KeyArrowUp, gocui.ModNone, cursorUp},
		{"details", 'k', gocui.ModNone, cursorUp},
		{"details", gocui.KeyEsc, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { searchQuery = ""; return loadTasks() }},
		{"list", 'y', gocui.ModNone, copyLine},
		{"list", gocui.KeyTab, gocui.ModNone, showDetails},
		{"list", 'l', gocui.ModNone, showDetails},
		{"list", gocui.KeyArrowDown, gocui.ModNone, taskListDown},
		{"list", 'j', gocui.ModNone, taskListDown},
		{"list", gocui.KeyArrowUp, gocui.ModNone, taskListUp},
		{"list", 'k', gocui.ModNone, taskListUp},
		{"list", gocui.KeySpace, gocui.ModNone, toggleTaskStatus},
		{"list", 'n', gocui.ModNone, promptCreateTask},
		{"list", 'e', gocui.ModNone, promptEditTask},
		{"list", 'd', gocui.ModNone, promptDeleteTask},
		{"list", 'g', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { selected = 0; return updateViews(g) }},
		{"list", 'G', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { selected = len(tasks) - 1; return updateViews(g) }},
		{"list", 'H', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { showClosed = !showClosed; return loadTasks() }},
		{"list", gocui.KeyEsc, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { searchQuery = ""; return loadTasks() }},
	}

	for _, binding := range bindings {
		if err := g.SetKeybinding(binding.view, binding.key, binding.mod, binding.handler); err != nil {
			return fmt.Errorf("bind %v to view %q: %w", binding.key, binding.view, err)
		}
	}
	return nil
}

func taskListDown(g *gocui.Gui, v *gocui.View) error {
	if selected < len(tasks)-1 {
		selected++
	}
	return updateViews(g)
}

func taskListUp(g *gocui.Gui, v *gocui.View) error {
	if selected > 0 {
		selected--
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
