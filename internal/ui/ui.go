package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"tigo/pkg/db"
	"tigo/pkg/task"

	"github.com/awesome-gocui/gocui"
)

var (
	tigoRoot   string
	tasks      []*task.Task
	selected   int  = 0
	showClosed bool = false
)

type keybinding struct {
	view    string
	key     any // gocui.Key or rune
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
		v.Highlight = true
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
		v.BgColor = gocui.ColorCyan
		v.FgColor = gocui.ColorBlack
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
	for _, t := range tasks {
		text := fmt.Sprintf(" [%s] %s", t.Status, t.Title)
		pad := strings.Repeat(" ", max(0, listWidth-len(text)))
		if t.Status == "CLOSED" {
			text = fmt.Sprintf("\x1b[32m%s\x1b[0m", text)
		}
		fmt.Fprintf(listView, "%s%s\n", text, pad)
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

	// details view
	detailsView.Clear()
	if len(tasks) > 0 && selected >= 0 && selected < len(tasks) {
		t := tasks[selected]
		fmt.Fprintf(detailsView, "ID: %s\n", t.ID)
		fmt.Fprintf(detailsView, "Title: %s\n", t.Title)
		fmt.Fprintf(detailsView, "Status: %s\n", t.Status)
		fmt.Fprintf(detailsView, "Priority: %d\n", t.Priority)
		fmt.Fprintf(detailsView, "Tags: %v\n", t.Tags)
		fmt.Fprintf(detailsView, "\nDescription:\n%s\n", t.Description)
	} else {
		fmt.Fprintln(detailsView, "No task selected.")
	}

	// help view
	var (
		spaceKeyText string
		hKeyText     string
	)
	if len(tasks) > 0 && selected >= 0 && selected < len(tasks) {
		switch tasks[selected].Status {
		case "CLOSED":
			spaceKeyText = "| Space: Open "
		case "OPEN":
			spaceKeyText = "| Space: Close "
		}
	}
	if showClosed {
		hKeyText = "Hide"
	} else {
		hKeyText = "Show"
	}
	helpText := fmt.Sprintf(" q: Quit | n: New | d: Delete %s| H: %s CLOSED | \u2191/\u2193 j/k: Navigate | g/G: Top/Bottom ", spaceKeyText, hKeyText)
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
		{"list", gocui.KeyArrowDown, gocui.ModNone, cursorDown},
		{"list", 'j', gocui.ModNone, cursorDown},
		{"list", gocui.KeyArrowUp, gocui.ModNone, cursorUp},
		{"list", 'k', gocui.ModNone, cursorUp},
		{"list", gocui.KeySpace, gocui.ModNone, toggleTaskState},
		{"list", 'n', gocui.ModNone, promptCreateTask},
		{"list", 'd', gocui.ModNone, promptDeleteTask},
		{"list", 'g', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { selected = 0; return updateViews(g) }},
		{"list", 'G', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { selected = len(tasks) - 1; return updateViews(g) }},
		{"list", 'H', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { showClosed = !showClosed; return loadTasks() }},
	}

	for _, b := range bindings {
		if err := g.SetKeybinding(b.view, b.key, b.mod, b.handler); err != nil {
			return fmt.Errorf("bind %v to view %q: %w", b.key, b.view, err)
		}
	}
	return nil
}

func promptCreateTask(g *gocui.Gui, v *gocui.View) error {
	maxX, maxY := g.Size()
	width := maxX / 2
	height := 2
	x0 := maxX/2 - width/2
	y0 := maxY/2 - height/2

	if v, err := g.SetView("createDialog", x0, y0, x0+width, y0+height, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " New Task Title "
		v.Editable = true
		v.Wrap = true
		g.Cursor = true

		if _, err := g.SetCurrentView("createDialog"); err != nil {
			return err
		}

		g.SetKeybinding("createDialog", gocui.KeyEnter, gocui.ModNone, submitCreateTask)
		g.SetKeybinding("createDialog", gocui.KeyEsc, gocui.ModNone, cancelDialog)
	}

	return nil
}

func submitCreateTask(g *gocui.Gui, v *gocui.View) error {
	title := strings.TrimSpace(v.Buffer())
	g.Cursor = false

	// Create task
	_, err := db.CreateTaskDirectory(tigoRoot, title)
	if err != nil {
		return err
	}

	if err := cancelDialog(g, v); err != nil {
		return err
	}

	// Reload
	if err := loadTasks(); err != nil {
		return err
	}

	selected = len(tasks) - 1
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
		g.SetKeybinding("deleteDialog", gocui.KeyEsc, gocui.ModNone, cancelDialog)
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

	if err := cancelDialog(g, v); err != nil {
		return err
	}

	if err := loadTasks(); err != nil {
		return err
	}
	return updateViews(g)
}

func cancelDialog(g *gocui.Gui, v *gocui.View) error {
	g.DeleteKeybindings(v.Name())
	g.Cursor = false

	if err := g.DeleteView(v.Name()); err != nil {
		return err
	}
	if _, err := g.SetCurrentView("list"); err != nil {
		return err
	}
	return updateViews(g)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func toggleTaskState(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) > 0 && selected < len(tasks) {
		db.ToggleStatus(tigoRoot, tasks[selected])
	}
	return nil
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	if selected < len(tasks)-1 {
		selected++
	}
	return updateViews(g)
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	if selected > 0 {
		selected--
	}
	return updateViews(g)
}
