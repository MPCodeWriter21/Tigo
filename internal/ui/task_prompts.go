package ui

import (
	"fmt"
	"strconv"
	"strings"

	"tigo/pkg/db"

	"github.com/awesome-gocui/gocui"
)

func promptCreateTask(g *gocui.Gui, v *gocui.View) error {
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

		if _, err := g.SetCurrentView("createDialogTitle"); err != nil {
			return err
		}

		g.SetKeybinding("createDialogTitle", gocui.KeyEnter, gocui.ModNone, submitCreateTask)
		g.SetKeybinding("createDialogTitle", gocui.KeyEsc, gocui.ModNone, closeCreateTaskDialog)
		g.SetKeybinding("createDialogTitle", gocui.KeyTab, gocui.ModNone, SetCurrentViewCallback("createDialogDescription"))
	}
	if v, err := g.SetView("createDialogDescription", x0, y0+heightTitle, x0+widthTitle-1, y0+heightTitle+heightDesc-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Description"
		v.Editable = true
		v.Wrap = true

		g.SetKeybinding("createDialogDescription", gocui.KeyEsc, gocui.ModNone, closeCreateTaskDialog)
		g.SetKeybinding("createDialogDescription", gocui.KeyTab, gocui.ModNone, SetCurrentViewCallback("createDialogPriority"))
	}
	if v, err := g.SetView("createDialogPriority", x0+widthTitle, y0, x0+widthTitle+widthPriority-1, y0+heightPriority-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Priority"
		v.Editable = false
		v.Wrap = false

		priority := "50"
		printPriority := func() {
			v.Clear()
			spaceCount := (widthPriority - len(priority) - 2) / 2
			spaces := strings.Repeat(" ", spaceCount)
			fmt.Fprintf(v, "%s%s%s", spaces, priority, spaces)
			v.SetCursor(spaceCount+len(priority), 0)
		}
		printPriority()

		g.SetKeybinding("createDialogTitle", gocui.KeyEnter, gocui.ModNone, submitCreateTask)
		g.SetKeybinding("createDialogPriority", gocui.KeyEsc, gocui.ModNone, closeCreateTaskDialog)
		g.SetKeybinding("createDialogPriority", gocui.KeyTab, gocui.ModNone, SetCurrentViewCallback("createDialogTags"))

		// Set keybinds for 0-9 and backspace to modify the priority
		for i := '0'; i <= '9'; i++ {
			digit := rune(i)
			g.SetKeybinding("createDialogPriority", digit, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
				if len(priority) < 10 {
					priority += string(digit)
					priority = strings.TrimLeft(priority, "0")
				}
				printPriority()
				return nil
			})
		}
		g.SetKeybinding("createDialogPriority", gocui.KeyBackspace, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			if len(priority) > 0 {
				priority = priority[:len(priority)-1]
			}
			printPriority()
			return nil
		})
	}
	if v, err := g.SetView("createDialogTags", x0+widthTitle, y0+heightPriority, x0+widthTitle+widthPriority-1, y0+heightPriority+heightTags-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Tags (comma-separated)"
		v.Editable = true
		v.Wrap = true

		g.SetKeybinding("createDialogTags", gocui.KeyEsc, gocui.ModNone, closeCreateTaskDialog)
		g.SetKeybinding("createDialogTags", gocui.KeyTab, gocui.ModNone, SetCurrentViewCallback("createDialogTitle"))
	}

	return nil
}

func submitCreateTask(g *gocui.Gui, v *gocui.View) error {
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

	if title == "" {
		return nil
	}

	_, err = db.CreateNewTask(tigoRoot, title, priority, tags, description)
	if err != nil {
		return err
	}

	if err := closeCreateTaskDialog(g, v); err != nil {
		return err
	}

	// Reload
	if err := loadTasks(); err != nil {
		return err
	}

	selected = len(tasks) - 1
	return updateViews(g)
}

func closeCreateTaskDialog(g *gocui.Gui, _ *gocui.View) error {
	g.Cursor = false
	if err := g.DeleteView("createDialogTitle"); err != nil {
		return err
	}
	if err := g.DeleteView("createDialogDescription"); err != nil {
		return err
	}
	if err := g.DeleteView("createDialogPriority"); err != nil {
		return err
	}
	if err := g.DeleteView("createDialogTags"); err != nil {
		return err
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
