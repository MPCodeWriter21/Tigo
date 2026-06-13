package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tigo/internal/config"
	"tigo/pkg/db"
	"tigo/pkg/logs"
	"tigo/pkg/task"

	"github.com/atotto/clipboard"
	"github.com/awesome-gocui/gocui"
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
	logs.Append(logs.LevelInfo, "Copied line to clipboard: \x1b[32m%q\x1b[0m", line)
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
			ansiMatches := allANSIRegex.FindAllStringIndex(line[:start], -1)
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
	paddingLength := (width - textLen(trimmedLine)) / 2
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
			fmt.Sprintf("\x1b[31mFailed to reload config:\x1b[0m\n%s\nCurrent config is unchanged.", err.Error()),
			"tasks", false)
	}
	cfg = newCfg
	// Keep the same task selected modifying the config (if it still exists)
	var selectedID string
	if len(tasks) > 0 && selectedTask < len(tasks) {
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

// openConfigFile opens the local config file in the user's default text editor.
func openConfigFile(g *gocui.Gui, _ *gocui.View) error {
	gocui.Suspend()
	defer gocui.Resume()

	configFilePath := filepath.Join(tigoRoot, "config.yaml")
	err := openInEditor(configFilePath)
	if err != nil {
		return err
	}
	return reloadConfig(g, nil)
}

// textLen returns the length of the text without counting ANSI escape codes.
// It uses utf8.RuneCountInString to count the number of runes, which correctly handles multi-byte characters.
func textLen(text string) (length int) {
	return utf8.RuneCountInString(allANSIRegex.ReplaceAllString(text, ""))
}

// calcVisualLines returns the number of visual lines the given text would occupy
// when word-wrapped to contentWidth columns.
func calcVisualLines(content string, contentWidth int) int {
	if contentWidth < 1 {
		contentWidth = 1
	}
	lines := 0
	for line := range strings.SplitSeq(content, "\n") {
		lineLen := textLen(line)
		lines += lineLen/contentWidth + 1
	}
	return max(lines, 1)
}

// parseRelativeDateTime takes a string like:
//   - "tomorrow" -> current time + 1 day
//   - "next week" -> current time + 1 week
//   - "next month" -> current time + 1 month
//   - "30 seconds" -> current time + 30 seconds
//   - "5 minutes" -> current time + 5 minutes
//   - "2 hours" -> current time + 2 hours
//   - "3 days" -> current time + 3 days
//   - "1 week" -> current time + 1 week
//   - "2 months" -> current time + 2 months
//   - "3 seasons" -> current time + 3 * 3 months
//   - "1 year" -> current time + 1 year
//   - "next decade" -> current time + 10 years
//   - "next century" -> current time + 100 years
//
// Returns the formatted string, the parsed time.Time, and any error.
// Date-only values use YYYY-MM-DD format; values with time use RFC3339 (timezone-aware).
func parseRelativeDateTime(input string) (string, *time.Time, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	now := time.Now().Local()

	if input == "today" {
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return t.Format("2006-01-02"), &t, nil
	}
	if input == "tonight" {
		t := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
		return t.Format(time.RFC3339), &t, nil
	}
	if input == "tomorrow" {
		t := now.AddDate(0, 0, 1)
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		return t.Format("2006-01-02"), &t, nil
	}

	var seconds, minutes, hours, days, months, years int
	re := regexp.MustCompile(`^next \s*(second|minute|hour|day|week|month|season|year)s?$`)
	if re.MatchString(input) {
		input = re.ReplaceAllString(input, "next $1")
		switch input {
		case "next second":
			seconds = 1
		case "next minute":
			minutes = 1
		case "next hour":
			hours = 1
		case "next day":
			days = 1
		case "next week":
			days = 7
		case "next month":
			months = 1
		case "next season":
			months = 3
		case "next year":
			years = 1
		case "next decade":
			years = 10
		case "next century":
			years = 100
		}
		t := now.AddDate(years, months, days).Add(
			time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second)
		if seconds == 0 && minutes == 0 && hours == 0 {
			return t.Format("2006-01-02"), &t, nil
		} else {
			return t.Format(time.RFC3339), &t, nil
		}
	}

	re = regexp.MustCompile(`^([0-9]+)\s*(second|minute|hour|day|week|month|season|year)s?$`)
	matches := re.FindStringSubmatch(input)
	if len(matches) != 3 {
		return "", nil, fmt.Errorf("invalid relative date format")
	}
	value := matches[1]
	unit := matches[2]

	switch unit {
	case "second":
		seconds, _ = strconv.Atoi(value)
	case "minute":
		minutes, _ = strconv.Atoi(value)
	case "hour":
		hours, _ = strconv.Atoi(value)
	case "day":
		days, _ = strconv.Atoi(value)
	case "week":
		days, _ = strconv.Atoi(value)
		days *= 7
	case "month":
		months, _ = strconv.Atoi(value)
	case "season":
		months, _ = strconv.Atoi(value)
		months *= 3
	case "year":
		years, _ = strconv.Atoi(value)
	}

	t := now.AddDate(years, months, days).Add(
		time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second)
	if seconds == 0 && minutes == 0 && hours == 0 {
		return t.Format("2006-01-02"), &t, nil
	} else {
		return t.Format(time.RFC3339), &t, nil
	}
}

// Opens the file at the given path with the default application. Returns an error if it fails.
func openFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.ErrNotExist
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default: // linux, freebsd, etc.
		// Try common launchers in order
		for _, launcher := range []string{"xdg-open", "gio", "gnome-open", "kde-open", "exo-open", "mimeopen", "termux-open"} {
			if _, err := exec.LookPath(launcher); err == nil {
				return exec.Command(launcher, path).Start()
			}
		}
		return fmt.Errorf("no suitable open command found; install xdg-utils or a desktop environment")
	}
	return cmd.Start()
}

// openInEditor opens the given file in the user's default text editor.
// IMPORTANT: Make sure to suspend the UI before calling this function, and resume it after the editor is closed, to prevent UI glitches:
//
//	gocui.Suspend()
//	defer gocui.Resume()
func openInEditor(filePath string) error {
	// 1. Check environment variables
	if editor := os.Getenv("VISUAL"); editor != "" {
		return runCommand(editor, filePath)
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return runCommand(editor, filePath)
	}

	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		// 2. Try Debian/Ubuntu alternatives
		if editor, err := exec.LookPath("sensible-editor"); err == nil {
			return runCommand(editor, filePath)
		}
		if editor, err := exec.LookPath("editor"); err == nil {
			return runCommand(editor, filePath)
		}

		// 3. Search common editors
		for _, ed := range []string{
			"vim", "vi", "nano", "emacs", "micro", "helix", "code", "gedit", "kate",
		} {
			if path, err := exec.LookPath(ed); err == nil {
				return runCommand(path, filePath)
			}
		}
		return fmt.Errorf("no text editor found; set $VISUAL or $EDITOR")

	case "darwin":
		// macOS has `open -t` for default text editor
		return exec.Command("open", "-t", filePath).Start()

	case "windows":
		// Use `start` to open with associated program
		return exec.Command("cmd", "/c", "start", "", filePath).Start()

	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// runCommand starts a command with arguments; if the editor supports
// multiple files you might need to split the EDITOR variable smartly.
func runCommand(editor, filePath string) error {
	// Handle editors defined with arguments, e.g. "code --wait".
	parts := strings.Fields(editor)
	args := append(parts[1:], filePath)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run() // Run blocks until the editor exits (useful for terminal editors)
}

// dueColor returns an ANSI color code for the task based on its due date proximity.
// Returns empty string if due coloring is disabled or no due date is set.
func dueColor(t *task.Task) string {
	if !cfg.DueColorEnabled || t.DueDateTime == nil {
		return ""
	}
	now := time.Now()
	dt := *t.DueDateTime
	if dt.Before(now) {
		return "\x1b[38;5;196m" // Red for overdue
	}
	if dt.Sub(now) <= 24*time.Hour {
		return "\x1b[38;5;208m" // Orange for due today
	}
	if dt.Sub(now) <= 48*time.Hour {
		return "\x1b[38;5;220m" // Yellow for due tomorrow
	}
	if dt.Sub(now) <= 7*24*time.Hour {
		return "\x1b[38;5;38m" // This week
	}
	return "\x1b[38;5;12m"
}

// dueDateSorter ranks tasks by due date, using parsed time when available.
// Falls back to ID comparison when both tasks lack a due date.
func dueDateSorter(i, j int) bool {
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

// sortedKeysByValue returns a slice of the map's keys sorted by their values in descending order
// If two keys have the same value, they are sorted in alphabetical order.
func sortedKeysByValue(m map[string]int) []string {
	// Extract keys into a slice
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	// Sort the keys based on map values
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] == m[keys[j]] {
			return keys[i] < keys[j]
		}
		return m[keys[i]] > m[keys[j]]
	})

	return keys
}
