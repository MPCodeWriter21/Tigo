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
	"golang.design/x/clipboard"
)

var (
	tigoRoot          string
	tasks             []*task.Task
	sortBy            string = "id"
	showClosed        bool   = false
	selectedTask      int    = 0
	searchQuery       string
	currentDetail     string
	currentDetailLine string
	allANSIRegex      = regexp.MustCompile(`\x1b\[[0-9;]*m`)
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
	var (
		cx = new(int)
		cy = new(int)
	)
	*cx, *cy = detailsView.Cursor()
	detailsView.Clear()
	if len(tasks) > 0 && selectedTask >= 0 && selectedTask < len(tasks) {
		detailsView.FgColor = gocui.ColorWhite
		t := tasks[selectedTask]
		showSelection := g.CurrentView() == detailsView

		detailsFprintf(detailsView, cx, cy, showSelection, "ID: \x1b[1;36m%s\x1b[0m\n", t.ID)
		detailsFprintf(detailsView, cx, cy, showSelection, "Title: \x1b[1;32m%s\x1b[0m\n", t.Title)
		switch t.Status {
		case "OPEN":
			detailsFprintf(detailsView, cx, cy, showSelection, "Status: \x1b[1;37mOPEN\x1b[0m\n")
		case "CLOSED":
			detailsFprintf(detailsView, cx, cy, showSelection, "Status: \x1b[1;32mCLOSED\x1b[0m\n")
		default:
			detailsFprintf(detailsView, cx, cy, showSelection, "Status: \x1b[1;35m%s\x1b[0m\n", t.Status)
		}
		detailsFprintf(detailsView, cx, cy, showSelection, "Priority: \x1b[1;34m%d\x1b[0m\n", t.Priority)
		tagsStr := ""
		for _, tag := range t.Tags {
			tagsStr = fmt.Sprintf("%s\x1b[1;33m%s\x1b[0m ", tagsStr, tag)
		}
		detailsFprintf(detailsView, cx, cy, showSelection, "Tags: %s\n", tagsStr)
		detailsFprintf(detailsView, cx, cy, showSelection, "\nDescription:\n%s\n", t.Description)
	} else {
		detailsView.FgColor = gocui.ColorRed
		fmt.Fprintln(detailsView, "No task selected.")
	}
	if g.CurrentView() == detailsView {
		detailsView.SetCursor(*cx, *cy)
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
		{"details", 'y', gocui.ModNone, copyDetail},
		{"details", gocui.KeyTab, gocui.ModNone, setCurrentViewCallback("tasks")},
		{"details", gocui.KeyEsc, gocui.ModNone, setCurrentViewCallback("tasks")},
		{"details", gocui.KeyArrowDown, gocui.ModNone, cursorDown},
		{"details", 'j', gocui.ModNone, cursorDown},
		{"details", gocui.KeyArrowUp, gocui.ModNone, cursorUp},
		{"details", 'k', gocui.ModNone, cursorUp},
		{"details", 'h', gocui.ModNone, detailsLeft},
		{"details", gocui.KeyArrowLeft, gocui.ModNone, detailsLeft},
		{"details", 'l', gocui.ModNone, detailsRight},
		{"details", gocui.KeyArrowRight, gocui.ModNone, detailsRight},
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
		{"tasks", gocui.KeyEsc, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			searchQuery = ""
			g.DeleteView("search")
			loadTasks()
			return updateViews(g)
		}},
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

// detailsFprintfLine is a helper function that prints a line to the details view, while
// also handling search query highlighting and selection highlighting.
func detailsFprintfLine(v *gocui.View, cx, cy *int, showSelection bool, format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	y := v.LinesHeight() // Line numbers start at 1
	if y == 0 {
		y = 1
	}
	y -= 1 // Convert to 0-based index
	if searchQuery != "" {
		re := regexp.MustCompile("(?i)" + searchQuery)
		line = re.ReplaceAllStringFunc(line, func(match string) string {
			return fmt.Sprintf("\x1b[43m%s\x1b[40m", match)
		})
	}
	if y == *cy && showSelection {
		currentDetailLine = line
		// Find the Bold ANSI escape code in the line after cx and change its color to have a cyan background
		re := regexp.MustCompile(`\x1b\[1;(\d+)m`)
		locs := re.FindAllStringIndex(line, -1)
		for i, loc := range locs {
			if loc[0] > *cx || i == len(locs)-1 {
				linePrefix := line[:loc[0]]
				linePrefixClean := allANSIRegex.ReplaceAllString(linePrefix, "")
				if len(linePrefixClean) < *cx && i != len(locs)-1 {
					continue
				}
				*cx = len(linePrefixClean)
				line = linePrefix + "\x1b[46;30m" + line[loc[1]:]
				// From `loc[1]` to `first \x1b[0m after loc[1]` or the `end of the line`
				detailEnd := strings.Index(line[loc[1]:], "\x1b[0m")
				if detailEnd == -1 {
					detailEnd = len(line)
				} else {
					detailEnd += loc[1]
				}
				currentDetail = line[loc[1]+1 : detailEnd]
				break
			}
		}
		if locs == nil {
			currentDetail = line
			line += "\x1b[46m \x1b[0m"
		}
		currentDetail = strings.TrimSpace(allANSIRegex.ReplaceAllString(currentDetail, ""))
		fmt.Fprintf(v, "%s\n", line)
	} else {
		fmt.Fprintf(v, "%s\n", line)
	}
}

func detailsFprintf(v *gocui.View, cx, cy *int, showSelection bool, format string, a ...any) {
	text := fmt.Sprintf(format, a...)
	lines := strings.SplitSeq(text, "\n")
	for line := range lines {
		if line == "" {
			continue
		}
		detailsFprintfLine(v, cx, cy, showSelection, "%s", line)
	}
}

// Yank the currently selected detail line to the clipboard, without ANSI escape codes
func copyDetail(g *gocui.Gui, v *gocui.View) error {
	if currentDetail != "" {
		if err := clipboard.Init(); err != nil {
			return err
		}
		clipboard.Write(clipboard.FmtText, []byte(currentDetail))
	}
	// TODO: After the Console view is added, show a message in the console view that the detail has been copied to the clipboard.
	return nil
}

// Move the cursor in the details view to the left, jumping over ANSI escape codes
// If the cursor is at the beginning of the line, move it to the tasks view
func detailsLeft(g *gocui.Gui, v *gocui.View) error {
	cx, cy := v.Cursor()
	if cx == 0 {
		return nil
	}
	// Find the first Bold ANSI escape code before cx and move the cursor to it
	re := regexp.MustCompile(`\x1b\[1;3[0-9]+m`)
	locs := re.FindAllStringIndex(currentDetailLine, -1)
	for i := len(locs) - 1; i >= 0; i-- {
		cleanPrefix := allANSIRegex.ReplaceAllString(currentDetailLine[:locs[i][0]], "")
		if len(cleanPrefix) < cx {
			return v.SetCursor(len(cleanPrefix)-1, cy)
		}
	}
	_, err := g.SetCurrentView("tasks")
	return err
}

// Move the cursor in the details view to the right, jumping over ANSI escape codes
func detailsRight(g *gocui.Gui, v *gocui.View) error {
	cx, cy := v.Cursor()
	// Find the first Bold ANSI escape code after cx and move the cursor to it
	re := regexp.MustCompile(`\x1b\[1;3[0-9]+m`)
	locs := re.FindAllStringIndex(currentDetailLine, -1)
	for _, loc := range locs {
		cleanPrefix := allANSIRegex.ReplaceAllString(currentDetailLine[:loc[0]], "")
		if loc[0] > cx {
			if len(cleanPrefix) <= cx {
				continue
			}
			return v.SetCursor(len(cleanPrefix)-1, cy)
		}
	}
	return nil
}
