package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/awesome-gocui/gocui"

	"github.com/MPCodeWriter21/Tigo/internal/config"
	"github.com/MPCodeWriter21/Tigo/pkg/db"
	"github.com/MPCodeWriter21/Tigo/pkg/logs"
	"github.com/MPCodeWriter21/Tigo/pkg/task"
	"github.com/MPCodeWriter21/Tigo/pkg/utils"
)

func doNothing(g *gocui.Gui, v *gocui.View) error { return nil }

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func deleteViewAndSetCurrent(viewName string, cursor bool, deleteKeybindings bool) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		if deleteKeybindings {
			g.DeleteKeybindings(v.Name())
		}
		g.Cursor = cursor

		if err := g.DeleteView(v.Name()); err != nil {
			return err
		}
		if _, err := g.SetCurrentView(viewName); err != nil {
			return err
		}
		return updateViews(g)
	}
}

var deleteViewDefault = deleteViewAndSetCurrent("tasks", false, false)

func toggleTaskStatus(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) == 0 || selectedTask >= len(tasks) || selectedTask < 0 {
		return nil
	}
	t := tasks[selectedTask]
	oldStatus := t.Status
	_, err := db.ToggleStatus(tigoRoot, t)
	if err == nil && oldStatus != t.Status {
		switch t.Status {
		case "OPEN":
			trackChange("Open", t.ID, t.Title, "")
		case "CLOSED":
			trackChange("Close", t.ID, t.Title, "")
		default:
			trackChange("Update", t.ID, t.Title, fmt.Sprintf("Status: %s -> %s", oldStatus, t.Status))
		}
	}
	return err
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	cx, cy := v.Cursor()
	y := v.LinesHeight()
	if cy < y-2 {
		return v.SetCursor(cx, cy+1)
	}
	return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	cx, cy := v.Cursor()
	if cy > 0 {
		return v.SetCursor(cx, cy-1)
	}
	return nil
}

func copyLine(g *gocui.Gui, v *gocui.View) error {
	_, cy := v.Cursor()
	line, err := v.Line(cy)
	if err != nil {
		return err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	clipboard.WriteAll(line)
	logs.Append(logs.LevelInfo, "Copied line to clipboard: \x1b[32m%q\x1b[39m", line)
	return nil
}

func setCurrentViewCallback(name string) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		_, err := g.SetCurrentView(name)
		return err
	}
}

func setCurrentViewCallbackCursor(name string, cursor bool) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		g.Cursor = cursor
		_, err := g.SetCurrentView(name)
		return err
	}
}

// Works the same as fmt.Fprintf, except if searchQuery is set, it highlights the matching text in the view.
func searchedFprintf(v *gocui.View, format string, a ...any) {
	// Some queries match the already existing ANSI escape codes, which USED TO CAUSE
	// the highlighting to break. For example, searching for "43m" WOULD match the yellow
	// background code and break the highlighting.
	// I AM SO F***KING PROUD OF MYSELF FOR SOLVING THIS!
	line := fmt.Sprintf(format, a...)
	// If the search query is not empty, highlight the searched query
	if searchQuery.value != "" {
		re := regexp.MustCompile("(?i)" + searchQuery.value)
		// Get all the positions where line matches the given search query, and insert the highlight color before the match and the reset color after the match
		matches := re.FindAllStringIndex(line, -1)
		// Find all the places ANSI escape codes are
		// We use these positions to invalidate the search query matches that are inside ANSI escape codes
		allANSIRegexPlus := regexp.MustCompile(`(?:\x1b\[[0-9;]*m)+`)
		allAnsiMatches := allANSIRegexPlus.FindAllStringIndex(line, -1)
		validMatches := make([][]int, 0, len(matches))
		for _, match := range matches {
			invalid := false
			for _, ansiMatch := range allAnsiMatches {
				// A N S I
				//  match
				if match[0] >= ansiMatch[0] && match[1] <= ansiMatch[1] {
					invalid = true
					break
				}
				//     A N S I
				//  match
				if match[0] <= ansiMatch[0] && match[1] > ansiMatch[0] {
					invalid = true
					break
				}
				// A N S I
				//      match
				if match[0] < ansiMatch[1] && match[1] > ansiMatch[1] {
					invalid = true
					break
				}
			}
			if !invalid {
				validMatches = append(validMatches, match)
			}
		}
		offset := 0
		for _, match := range validMatches {
			start := match[0] + offset
			end := match[1] + offset

			resetColor := "\x1b[49m"
			// Try to find the last ANSI escape code before the match and use its background color as the reset color,
			// so that only the searched query is highlighted with the highlight color, and the rest of the line remains the same.
			ansiMatches := utils.AllANSIRegex.FindAllStringIndex(line[:start], -1)
			if len(ansiMatches) > 0 {
				// Loop through results from the last to the first
				for i := len(ansiMatches) - 1; i >= 0; i-- {
					lastAnsi := line[ansiMatches[i][0]:ansiMatches[i][1]]
					bgRegex := regexp.MustCompile(`(4[0-9])(m|;)`)
					bgMatch := bgRegex.FindStringSubmatch(lastAnsi)
					if bgMatch != nil {
						// Join all the colors from i to the end, and use it as the reset color
						resetColor = ""
						for j := i; j < len(ansiMatches); j++ {
							resetColor += line[ansiMatches[j][0]:ansiMatches[j][1]]
						}
						break
					}
				}
			}

			line = line[:start] + "\x1b[43m" + line[start:end] + resetColor + line[end:]
			offset += len("\x1b[43m") + len(resetColor)
		}
	}
	fmt.Fprint(v, line)
}

// Prints the formatted string centered in the view. Returns the number of padding spaces added on each side.
func centeredFprintf(v *gocui.View, format string, a ...any) (int, error) {
	line := fmt.Sprintf(format, a...)
	trimmedLine := strings.TrimRight(line, "\n")
	trailingNewLines := line[len(trimmedLine):]
	width, _ := v.Size()
	paddingLength := (width - utils.TextLen(trimmedLine)) / 2
	padding := strings.Repeat(" ", paddingLength)
	_, err := fmt.Fprintf(v, "%s%s%s%s", padding, trimmedLine, padding, trailingNewLines)
	return paddingLength, err
}

// An extension of gocui's default editor that supports
// + Home and End keys
func simpleEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if ch != 0 && mod == 0 {
		v.EditWrite(ch)
		return
	}
	_, cy := v.Cursor()

	switch key {
	case gocui.KeySpace:
		v.EditWrite(' ')
	case gocui.KeyBackspace, gocui.KeyBackspace2:
		v.EditDelete(true)
	case gocui.KeyDelete:
		v.EditDelete(false)
	case gocui.KeyInsert:
		v.Overwrite = !v.Overwrite
	case gocui.KeyEnter:
		v.EditNewLine()
		v.MoveCursor(0, 0)
	case gocui.KeyArrowDown:
		v.MoveCursor(0, 1)
	case gocui.KeyArrowUp:
		v.MoveCursor(0, -1)
	case gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0)
	case gocui.KeyArrowRight:
		v.MoveCursor(1, 0)
	case gocui.KeyHome:
		v.SetCursor(0, cy)
	case gocui.KeyEnd:
		line, _ := v.Line(cy)
		v.SetCursor(len(line), cy)
	case gocui.KeyTab:
		v.EditWrite('\t')
	case gocui.KeyEsc:
		// If not here the esc key will act like the KeySpace
	default:
		v.EditWrite(ch)
	}
}

// A simple editor that does not allow more than one line of text, and supports
// Home and End keys
func oneLineEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if ch != 0 && mod == 0 {
		v.EditWrite(ch)
		return
	}
	_, cy := v.Cursor()

	switch key {
	case gocui.KeySpace:
		v.EditWrite(' ')
	case gocui.KeyBackspace, gocui.KeyBackspace2:
		v.EditDelete(true)
	case gocui.KeyDelete:
		v.EditDelete(false)
	case gocui.KeyInsert:
		v.Overwrite = !v.Overwrite
	case gocui.KeyEnter:
		// Ignore enter
	case gocui.KeyArrowDown:
		v.MoveCursor(0, 1)
	case gocui.KeyArrowUp:
		v.MoveCursor(0, -1)
	case gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0)
	case gocui.KeyArrowRight:
		v.MoveCursor(1, 0)
	case gocui.KeyHome:
		v.SetCursor(0, cy)
	case gocui.KeyEnd:
		line, _ := v.Line(cy)
		v.SetCursor(len(line), cy)
	case gocui.KeyTab:
		v.EditWrite('\t')
	case gocui.KeyEsc:
		// If not here the esc key will act like the KeySpace
	default:
		v.EditWrite(ch)
	}
}

func showCurrentTigoDirectory(g *gocui.Gui, v *gocui.View) error {
	return promptMessageBox(g, "Current Tigo Directory", tigoRoot, "", false)
}

// StartupConfigError sets the startup error to be displayed when the UI starts.
func StartupConfigError(err error) {
	startupErr = err
}

// reloadConfig reloads the configuration from disk and updates the views. It keeps the same task selected if it still exists after reloading.
func reloadConfig(g *gocui.Gui, _ *gocui.View) error {
	var newCfg *config.TigoConfig
	newCfg, err := config.LoadConfig(tigoRoot)
	if err != nil {
		return promptMessageBox(g, "Invalid Config",
			fmt.Sprintf("\x1b[31mFailed to reload config:\x1b[39m\n%s\nCurrent config is unchanged.", err.Error()),
			"tasks", false)
	}
	cfg = newCfg
	return reloadTasks(g, nil)
}

// openConfigFile opens the local config file in the user's default text editor.
func openConfigFile(g *gocui.Gui, _ *gocui.View) error {
	gocui.Suspend()
	defer gocui.Resume()

	configFilePath := filepath.Join(tigoRoot, "config.yaml")
	err := utils.OpenInEditor(configFilePath, tigoRoot)
	if err != nil {
		return err
	}
	return reloadConfig(g, nil)
}

// loadTasks loads the tasks from the disk and populates tasks slice
func loadTasks() error {
	taskIDs, err := db.DiscoverTasks(tigoRoot)
	if err != nil {
		return err
	}

	tasks = make([]*task.Task, 0, len(taskIDs))
	// Parse each task and filter based on search query and config settings
	for _, id := range taskIDs {
		t, err := task.Parse(id, filepath.Join(tigoRoot, id, "TASK.md"))
		if err == nil {
			// If the search query is a task id, show the task no matter what
			if utils.IDRegEx.MatchString(searchQuery.value) {
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

	// Ranks tasks by due date, using parsed time when available.
	// Falls back to ID comparison when both tasks lack a due date.
	dueDateSorter := func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		if a.DueDateTime == nil && b.DueDateTime == nil {
			if a.DueDate == "" || b.DueDate == "" {
				return a.ID < b.ID
			}
			return a.DueDate < b.DueDate
		}
		if a.DueDateTime == nil {
			return false
		}
		if b.DueDateTime == nil {
			return true
		}
		if !a.DueDateTime.Equal(*b.DueDateTime) {
			return a.DueDateTime.Before(*b.DueDateTime)
		}
		return a.DueDate < b.DueDate
	}

	// Sort tasks based on the configured sort option
	switch cfg.SortBy {
	case "id":
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].ID < tasks[j].ID
		})
	case "priority":
		sort.Slice(tasks, func(i, j int) bool {
			if tasks[i].Priority != tasks[j].Priority {
				return tasks[i].Priority > tasks[j].Priority
			}
			// Fall back to due date sorting if priorities are equal
			return dueDateSorter(i, j)
		})
	case "due-date":
		sort.Slice(tasks, dueDateSorter)
	case "title":
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].Title < tasks[j].Title
		})
	default:
		return fmt.Errorf("invalid sort option: %s", cfg.SortBy)
	}

	// Ensure selectedTask is within bounds after reloading tasks
	selectedTask = max(min(selectedTask, len(tasks)-1), 0)
	return nil
}

// reloadTasks reloads the tasks from disk and updates the views. It keeps the same task selected if it still exists after reloading.
func reloadTasks(g *gocui.Gui, _ *gocui.View) error {
	var selectedID string
	if len(tasks) > 0 && selectedTask < len(tasks) && selectedTask >= 0 {
		selectedID = tasks[selectedTask].ID
	}
	if err := loadTasks(); err != nil {
		return err
	}
	for i, t := range tasks {
		if t.ID == selectedID {
			selectedTask = i
			break
		}
	}
	return updateViews(g)
}
