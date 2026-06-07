/* Package ui contains TUI Code for Tigo */
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"tigo/internal/config"
	"tigo/pkg/db"
	"tigo/pkg/logs"
	"tigo/pkg/task"

	"github.com/atotto/clipboard"
	"github.com/awesome-gocui/gocui"
)

var (
	tigoRoot      string
	tasks         []*task.Task
	selectedTask  int = 0 // Index of the currently selected task in the tasks slice
	cfg           *config.TigoConfig
	searchQuery   searchQueryType
	currentDetail detail
	detailsRegEx  = regexp.MustCompile(`(?:\x1b\[(1;[0-9]+)m)|(?:\x1b\[(3[2-4];4)m)`)
	allANSIRegex  = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	taskRegEx     = regexp.MustCompile(`(?i)TASK\([0-9]{8}-[0-9]{6}\)`)
	filePathRegEx = regexp.MustCompile(`(?:(?P<relative>\.\.?\/)(?P<path>[\w\-\/\.]+))`)
	URLRegEx      = regexp.MustCompile(`(?:(?P<protocol>[a-z\-]+):\/\/(?P<hostname>[-a-zA-Z0-9]+(?:\.[-a-zA-Z0-9]+)+)(?P<port>:[0-9]+)?(?P<path>(?:\/[-a-zA-Z0-9()@:%_\+.~#?&=!]*)*))`)
)

type detailType int

const (
	normalDetail detailType = iota
	taskIDDetail
	tagDetail
	filePathDetail
	urlDetail
)

type detail struct {
	type_      detailType // The type of the detail
	value      string     // The value of the detail, without ANSI escape codes and trimmed of whitespace
	detailLine string     // The whole line of the detail, with ANSI escape codes, used for determining the position of the detail in the details view
}

type queryType int

const (
	normalQuery = iota
	tagQuery
)

type searchQueryType struct {
	type_ queryType // The type of the search query, normal search query or tag search query
	value string    // The value of the search query
}

// Run initializes and runs the GUI.
func Run(root string, conf *config.TigoConfig) error {
	// Enter Alternate Screen Buffer (hide main terminal content)
	fmt.Print("\x1b[?1049h")

	tigoRoot = root
	cfg = conf

	g, err := gocui.NewGui(gocui.OutputNormal, true)
	if err != nil {
		return err
	}
	defer func() { g.Cursor = true }()
	defer g.Close()
	defer fmt.Print("\x1b[?25h")   // Make cursor visible
	defer fmt.Print("\x1b[?1049l") // Disable alternate screen

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
			// If the search query is a task id, show the task no matter what
			if regexp.MustCompile("[0-9]{8}-[0-9]{6}").MatchString(searchQuery.value) {
				idMatched, err := regexp.MatchString("(?i)"+searchQuery.value, t.ID)
				if err == nil && idMatched {
					tasks = append(tasks, t)
					continue
				}
			}

			if !cfg.ShowClosed && t.Status == "CLOSED" {
				continue
			}
			if searchQuery.value != "" {
				if searchQuery.type_ == tagQuery {
					tagMatched := false
					for _, tag := range t.Tags {
						matched, err := regexp.MatchString("(?i)"+searchQuery.value, tag)
						if err == nil && matched {
							tagMatched = true
							break
						}
					}
					if !tagMatched {
						continue
					}
				} else if searchQuery.type_ == normalQuery {
					matched, err := regexp.MatchString("(?i)"+searchQuery.value, t.Title+t.Description+strings.Join(t.Tags, " "))
					if err != nil || !matched {
						continue
					}
				} else {
					panic(fmt.Sprintf("invalid search query type: %v", searchQuery.type_))
				}
			}
			tasks = append(tasks, t)
		}
	}
	switch cfg.SortBy {
	case "id":
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].ID < tasks[j].ID
		})
	case "priority":
		sort.Slice(tasks, func(i, j int) bool {
			// Higher priority tasks should come first, so we use > instead of <.
			return tasks[i].Priority > tasks[j].Priority
		})
	case "due-date":
		sort.Slice(tasks, func(i, j int) bool {
			if tasks[i].DueDate == "" {
				return false
			}
			if tasks[j].DueDate == "" {
				return true
			}
			return tasks[i].DueDate < tasks[j].DueDate
		})
	case "title":
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].Title < tasks[j].Title
		})
	default:
		return fmt.Errorf("invalid sort option: %s", cfg.SortBy)
	}
	selectedTask = min(selectedTask, len(tasks)-1)
	return nil
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	var frameRunes []rune
	switch cfg.FrameStyle {
	case "single":
		frameRunes = []rune{'─', '│', '┌', '┐', '└', '┘', '├', '┤', '┬', '┴', '┼'}
	case "double":
		frameRunes = []rune{'═', '║', '╔', '╗', '╚', '╝', '╠', '╣', '╦', '╩', '╬'}
	case "round":
		frameRunes = []rune{'─', '│', '╭', '╮', '╰', '╯'}
	default:
		return fmt.Errorf("invalid frame style: %s", cfg.FrameStyle)
	}
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

	detailHeight := (maxY - 2) * 2 / 3
	if detailHeight < 5 {
		detailHeight = maxY - 2
	}
	logHeight := (maxY - 2) - detailHeight
	hasLogs := logHeight >= 3

	if v, err := g.SetView("details", maxX/3, 0, maxX-1, detailHeight-1, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Details"
		v.Wrap = true
	}

	if hasLogs {
		if v, err := g.SetView("logs", maxX/3, detailHeight, maxX-1, maxY-2, 0); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = "Logs"
			v.Wrap = false
		}
	} else {
		g.DeleteView("logs")
	}

	if v, err := g.SetView("help", -1, maxY-2, maxX, maxY, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.BgColor = gocui.ColorBlack
		v.FgColor = gocui.ColorBlue
	}

	if searchQuery.value != "" {
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
		if searchQuery.value != "" {
			fmt.Fprintf(tasksView, "Search query: \x1b[32m\"%s\"\x1b[0m\n", searchQuery.value)
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

		detailsFprintf(detailsView, cx, cy, showSelection, "ID: \x1b[1;36m%s\x1b[0m", t.ID)
		detailsFprintf(detailsView, cx, cy, showSelection, "Title: \x1b[1;32m%s\x1b[0m", t.Title)
		switch t.Status {
		case "OPEN":
			detailsFprintf(detailsView, cx, cy, showSelection, "Status: \x1b[1;37mOPEN\x1b[0m")
		case "CLOSED":
			detailsFprintf(detailsView, cx, cy, showSelection, "Status: \x1b[1;32mCLOSED\x1b[0m")
		default:
			detailsFprintf(detailsView, cx, cy, showSelection, "Status: \x1b[1;35m%s\x1b[0m", t.Status)
		}
		detailsFprintf(detailsView, cx, cy, showSelection, "Priority: \x1b[1;34m%d\x1b[0m", t.Priority)
		if t.DueDate != "" {
			detailsFprintf(detailsView, cx, cy, showSelection, "Due Date: \x1b[1;33m%s\x1b[0m", t.DueDate)
		}
		tagsStr := ""
		for _, tag := range t.Tags {
			tagsStr = fmt.Sprintf("%s\x1b[33;4m%s\x1b[0m ", tagsStr, tag)
		}
		detailsFprintf(detailsView, cx, cy, showSelection, "Tags: %s", strings.TrimSpace(tagsStr))
		detailsFprintf(detailsView, cx, cy, showSelection, "\nDescription:\n%s", t.Description)
	} else {
		detailsView.FgColor = gocui.ColorRed
		fmt.Fprintln(detailsView, "No task selected.")
	}
	if g.CurrentView() == detailsView {
		detailsView.SetCursor(*cx, min(*cy, detailsView.LinesHeight()-2))
	}

	// logs view
	logsView, err := g.View("logs")
	if err == nil {
		cx, cy := logsView.Cursor()
		if g.CurrentView() != logsView {
			cy = logsView.LinesHeight() - 2
		}
		ox, oy = tasksView.Origin()
		logsView.Clear()
		entries := logs.Entries()
		for _, e := range entries {
			timeStr := e.Time.Format("15:04:05")
			var color string
			switch e.Level {
			case logs.LevelInfo:
				color = "\x1b[32m"
			case logs.LevelWarn:
				color = "\x1b[33m"
			case logs.LevelError:
				color = "\x1b[31m"
			case logs.LevelGit:
				color = "\x1b[36m"
			}
			fmt.Fprintf(logsView, "[\x1b[36m%s\x1b[0m] [%s%s\x1b[0m] \x1b[1;37m%s\x1b[0m\n", timeStr, color, e.Level, e.Message)
		}
		_, logsHeight := logsView.Size()
		if cy < oy+3 {
			oy = cy - 3
		}
		if cy > oy+logsHeight-3 {
			oy = cy - logsHeight + 3
		}
		if oy < 0 {
			oy = 0
		}
		logsView.SetOrigin(ox, oy)
		logsView.SetCursor(cx, min(cy, logsView.LinesHeight()-2))
	}

	// help view
	var (
		spaceKeyText string
		hKeyText     string
	)
	if g.CurrentView() == tasksView {
		if len(tasks) > 0 && selectedTask >= 0 && selectedTask < len(tasks) {
			switch tasks[selectedTask].Status {
			case "CLOSED":
				spaceKeyText = "| <space>: Open "
			case "OPEN":
				spaceKeyText = "| <space>: Close "
			}
		}
	} else if g.CurrentView() == detailsView {
		spaceKeyText = "| <space>: Follow Link "
	}
	if cfg.ShowClosed {
		hKeyText = "Hide"
	} else {
		hKeyText = "Show"
	}
	helpText := fmt.Sprintf(" ?: Help | e: Edit | d: Delete %s| H: %s CLOSED | /: Search | \u2193/\u2191 j/k: Navigate | g/G: Top/Bottom", spaceKeyText, hKeyText)
	if helpText != helpView.Buffer() {
		helpView.Clear()
		fmt.Fprint(helpView, helpText)
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

func toggleShowClosed(g *gocui.Gui, v *gocui.View) error {
	cfg.ShowClosed = !cfg.ShowClosed
	if len(tasks) != 0 && selectedTask < len(tasks) {
		task := tasks[selectedTask]
		if task.Status != "CLOSED" {
			// Make sure the previously selected task is still selected after reloading tasks, if it still exists.
			selectedID := task.ID
			err := loadTasks()
			if err != nil {
				return err
			}
			for i, t := range tasks {
				if t.ID == selectedID {
					selectedTask = i
					break
				}
			}
			return nil
		}
	}
	return loadTasks()
}

func clearSearchQuery(g *gocui.Gui, v *gocui.View) error {
	searchQuery.value = ""
	g.DeleteView("search")
	var selectedID string
	if len(tasks) > 0 && selectedTask >= 0 && selectedTask < len(tasks) {
		selectedID = tasks[selectedTask].ID
	}
	loadTasks()
	// Make sure the previously selected task is still selected after reloading tasks
	for i, t := range tasks {
		if t.ID == selectedID {
			selectedTask = i
			break
		}
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

// Find the details that should be highlighted for copying to clipboard and hyperlinks.
// The details are the text that comes after the special ANSI escape codes in the line, or the whole line if there are no special ANSI escape codes.
//
//	Normal details are the text that comes after the bold ANSI escape code in the line
//	They are highlighted with a cyan background and black foreground.
//
//	The hyperlinks are the text that comes after the underlined ANSI escape code in the line.
//	The color of the hyperlinks determines what the hyperlink is.
//	  Blue hyperlinks are TASK(ID)s, yellow hyperlinks are tags.
//	  They are highlighted with a blue background.
func highlight(cx *int, line string) string {
	locs := detailsRegEx.FindAllStringIndex(line, -1)
	for i, loc := range locs {
		if loc[0] >= *cx || i == len(locs)-1 {
			linePrefix := line[:loc[0]]
			linePrefixClean := allANSIRegex.ReplaceAllString(linePrefix, "")
			if len(linePrefixClean) < *cx && i != len(locs)-1 {
				continue
			}
			*cx = len(linePrefixClean)
			originalColor := line[loc[0]:loc[1]]

			// From `loc[1]` to `first \x1b[0m after loc[1]` or the `end of the line`
			detailEnd := strings.Index(line[loc[1]:], "\x1b[0m")
			if detailEnd == -1 {
				detailEnd = len(line)
			} else {
				detailEnd += loc[1]
			}

			var (
				highlightColor     string
				currentDetailType  detailType
				currentDetailValue = allANSIRegex.ReplaceAllString(line[loc[1]:detailEnd], "")
			)
			switch originalColor {
			case "\x1b[34;4m": // TASK(ID) or URL
				highlightColor = "\x1b[44;37;4m"
				if taskRegEx.MatchString(currentDetailValue) {
					currentDetailType = taskIDDetail
				} else if URLRegEx.MatchString(currentDetailValue) {
					currentDetailType = urlDetail
				} else {
					panic(fmt.Sprintf("invalid hyperlink format: %s", currentDetailValue))
				}
			case "\x1b[33;4m": // tag
				highlightColor = "\x1b[44;33;4m"
				currentDetailType = tagDetail
			case "\x1b[32;4m": // file path
				highlightColor = "\x1b[44;32;4m"
				currentDetailType = filePathDetail
			default: // normal detail
				highlightColor = "\x1b[46;30m"
				currentDetailType = normalDetail
			}

			currentDetail = detail{
				type_:      currentDetailType,
				value:      currentDetailValue,
				detailLine: line,
			}

			line = linePrefix + highlightColor + line[loc[1]:]
			break
		}
	}
	return line
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
	if searchQuery.value != "" {
		re := regexp.MustCompile("(?i)" + searchQuery.value)
		line = re.ReplaceAllStringFunc(line, func(match string) string {
			return fmt.Sprintf("\x1b[43;37m%s\x1b[40m", match)
		})
	}

	// Highlight TASK(ID) pattern with blue foreground and underline
	line = taskRegEx.ReplaceAllStringFunc(line, func(match string) string {
		return fmt.Sprintf("\x1b[34;4m%s\x1b[0m", match)
	})
	// URLs are highlighted the same way
	line = URLRegEx.ReplaceAllStringFunc(line, func(match string) string {
		return fmt.Sprintf("\x1b[34;4m%s\x1b[0m", match)
	})

	// Highlight file path patterns with green foreground and underline
	// E.g. `./path/to/file` or `../path/to/file`
	// These paths are relative to the current tasks's directory
	line = filePathRegEx.ReplaceAllStringFunc(line, func(match string) string {
		return fmt.Sprintf("\x1b[32;4m%s\x1b[0m", match)
	})

	if y == *cy && showSelection {
		originalLine := line
		line = highlight(cx, line)

		if line == originalLine {
			currentDetail = detail{
				type_:      normalDetail,
				value:      line,
				detailLine: line,
			}
			line += "\x1b[46m \x1b[0m"
		}
		currentDetail.value = strings.TrimSpace(allANSIRegex.ReplaceAllString(currentDetail.value, ""))
		fmt.Fprintf(v, "%s\n", line)
	} else {
		fmt.Fprintf(v, "%s\n", line)
	}
}

func detailsFprintf(v *gocui.View, cx, cy *int, showSelection bool, format string, a ...any) {
	text := fmt.Sprintf(format, a...)
	lines := strings.SplitSeq(text, "\n")
	for line := range lines {
		detailsFprintfLine(v, cx, cy, showSelection, "%s", line)
	}
}

// Yank the currently selected detail to the clipboard
func copyDetail(g *gocui.Gui, v *gocui.View) error {
	if currentDetail.value != "" {
		clipboard.WriteAll(currentDetail.value)
		logs.Append(logs.LevelInfo, "Copied to clipboard: %q", currentDetail.value)
	}
	return nil
}

// Move the cursor in the details view to the left, jumping over ANSI escape codes
// If the cursor is at the beginning of the line, move it to the tasks view
func detailsLeft(g *gocui.Gui, v *gocui.View) error {
	cx, cy := v.Cursor()
	// Find the first Bold ANSI escape code before cx and move the cursor to it
	locs := detailsRegEx.FindAllStringIndex(currentDetail.detailLine, -1)
	for i := len(locs) - 1; i >= 0; i-- {
		cleanPrefix := allANSIRegex.ReplaceAllString(currentDetail.detailLine[:locs[i][0]], "")
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
	locs := detailsRegEx.FindAllStringIndex(currentDetail.detailLine, -1)
	for _, loc := range locs {
		cleanPrefix := allANSIRegex.ReplaceAllString(currentDetail.detailLine[:loc[0]], "")
		if loc[0] > cx {
			if len(cleanPrefix) <= cx {
				continue
			}
			return v.SetCursor(len(cleanPrefix)-1, cy)
		}
	}
	return nil
}

// Follow the hyperlink if the cursor is on a hyperlink, otherwise do nothing.
func followDetail(g *gocui.Gui, v *gocui.View) error {
	switch currentDetail.type_ {
	case normalDetail:
		return nil
	case taskIDDetail:
		re := regexp.MustCompile(`[0-9]{8}-[0-9]{6}`)
		taskID := re.FindString(currentDetail.value)
		for i, t := range tasks {
			if t.ID == taskID {
				selectedTask = i
				return updateViews(g)
			}
		}
		t, err := task.Parse(taskID, filepath.Join(tigoRoot, taskID, "TASK.md"))
		if err != nil {
			return promptMessageBox(g, "Task Not Found", fmt.Sprintf("\x1b[31mTask \x1b[34m`%s`\x1b[31m was not found!", taskID), "details", false)
		}
		tasks = append(tasks, t)
		selectedTask = len(tasks) - 1
		return nil
	case tagDetail:
		tag := currentDetail.value
		searchQuery = searchQueryType{
			type_: tagQuery,
			value: tag,
		}
		maxX, maxY := g.Size()
		searchView, err := g.SetView("search", 0, maxY-4, maxX/3-1, maxY-2, 0)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		searchView.Title = "/"
		searchView.Wrap = true
		searchView.Clear()
		fmt.Fprint(searchView, tag)
		g.SetCurrentView("tasks")
		return loadTasks()
	case filePathDetail:
		// The file path is relative to the current task's directory, so we need to get the current task's directory and join it with the file path.
		currentTask := tasks[selectedTask]
		taskDir := filepath.Join(tigoRoot, currentTask.ID)
		filePath := filepath.Join(taskDir, currentDetail.value)
		err := openFile(filePath)
		if err == os.ErrNotExist {
			return promptMessageBox(g, "File Not Found", fmt.Sprintf("\x1b[31mFile \x1b[34m`%s`\x1b[31m was not found!", filePath), "details", false)
		}
		return err
	case urlDetail:
		return openFile(currentDetail.value)
	}
	return fmt.Errorf("invalid detail type: %v", currentDetail.type_)
}

func openSelectedTaskDirectory(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) == 0 {
		return nil
	}
	currentTask := tasks[selectedTask]
	taskDir := filepath.Join(tigoRoot, currentTask.ID)
	return openFile(taskDir)
}

func openSelectedTaskFile(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) == 0 {
		return nil
	}
	gocui.Suspend()
	defer gocui.Resume()

	currentTask := tasks[selectedTask]
	taskFile := filepath.Join(tigoRoot, currentTask.ID, "TASK.md")
	err := openInEditor(taskFile)
	if err != nil {
		return err
	}
	return loadTasks()
}
