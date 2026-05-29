package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tigo/pkg/db"
	"tigo/pkg/task"

	"github.com/awesome-gocui/gocui"
)

var (
	tigoRoot string
	tasks    []*task.Task
	selected int
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
			tasks = append(tasks, t)
		}
	}
	selected = 0
	return nil
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("list", 0, 0, maxX/3-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Tasks"
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
		fmt.Fprintf(v, " q/Ctrl+C: Quit | n: New | d: Delete | \u2191/\u2193 j/k: Navigate ")
	}

	return updateViews(g)
}

func updateViews(g *gocui.Gui) error {
	lv, err := g.View("list")
	if err != nil {
		return err
	}

	dv, err := g.View("details")
	if err != nil {
		return err
	}

	lv.Clear()
	for i, t := range tasks {
		var color string
		if i == selected {
			color = "\x1b[44m"
		}
		fmt.Fprintf(lv, " %s[%s] %s\x1b[0m\n", color, t.Status, t.Title)
	}

	dv.Clear()
	if len(tasks) > 0 && selected >= 0 && selected < len(tasks) {
		t := tasks[selected]
		fmt.Fprintf(dv, "ID: %s\n", t.ID)
		fmt.Fprintf(dv, "Title: %s\n", t.Title)
		fmt.Fprintf(dv, "Status: %s\n", t.Status)
		fmt.Fprintf(dv, "Priority: %d\n", t.Priority)
		fmt.Fprintf(dv, "Tags: %v\n", t.Tags)
		fmt.Fprintf(dv, "\nDescription:\n%s\n", t.Description)
	} else {
		fmt.Fprintln(dv, "No task selected.")
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
		{"list", 'n', gocui.ModNone, promptCreateTask},
		{"list", 'd', gocui.ModNone, promptDeleteTask},
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

	// Create task
	_, err := db.CreateTaskDirectory(tigoRoot, title)
	if err != nil {
		return err
	}

	// Close dialog
	if err := g.DeleteView("createDialog"); err != nil {
		return err
	}
	if _, err := g.SetCurrentView("list"); err != nil {
		return err
	}

	// Reload
	if err := loadTasks(); err != nil {
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
		g.SetKeybinding("deleteDialog", gocui.KeyEsc, gocui.ModNone, cancelDialog)
	}

	return nil
}

func submitDeleteTask(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) > 0 && selected < len(tasks) {
		t := tasks[selected]

		err := os.RemoveAll(filepath.Join(tigoRoot, t.ID))
		if err != nil {
			return err
		}

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
	g.DeleteView(v.Name())
	g.DeleteKeybindings(v.Name())

	if _, err := g.SetCurrentView("list"); err != nil {
		return err
	}
	return updateViews(g)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
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
