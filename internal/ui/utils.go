package ui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tigo/pkg/db"
	"tigo/pkg/logs"

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
	// FIXME: Some queries match the already existing ANSI escape codes, which causes
	// the highlighting to break. For example, searching for "43m" will match the yellow
	// background code and break the highlighting.
	line := fmt.Sprintf(format, a...)
	if searchQuery.value != "" {
		re := regexp.MustCompile("(?i)" + searchQuery.value)
		line = re.ReplaceAllStringFunc(line, func(match string) string {
			return fmt.Sprintf("\x1b[43m%s\x1b[40m", match) // Highlight with yellow background
		})
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

// textLen returns the length of the text without counting ANSI escape codes.
// It uses utf8.RuneCountInString to count the number of runes, which correctly handles multi-byte characters.
func textLen(text string) (length int) {
	return utf8.RuneCountInString(allANSIRegex.ReplaceAllString(text, ""))
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
func parseRelativeDateTime(input string) (string, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	now := time.Now()

	if input == "today" {
		return now.Format("2006-01-02"), nil
	}
	if input == "tonight" {
		// Set the time to 23:59:59 to represent the end of the day
		return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Format("2006-01-02 15:04:05"), nil
	}
	if input == "tomorrow" {
		return now.AddDate(0, 0, 1).Format("2006-01-02"), nil
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
		if seconds == 0 && minutes == 0 && hours == 0 {
			return now.AddDate(years, months, days).Format("2006-01-02"), nil
		} else {
			return now.
				AddDate(years, months, days).
				Add(time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second).
				Format("2006-01-02 15:04:05"), nil
		}
	}

	re = regexp.MustCompile(`^([0-9]+)\s*(second|minute|hour|day|week|month|season|year)s?$`)
	matches := re.FindStringSubmatch(input)
	if len(matches) != 3 {
		return "", fmt.Errorf("invalid relative date format")
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

	if seconds == 0 && minutes == 0 && hours == 0 {
		return now.AddDate(years, months, days).Format("2006-01-02"), nil
	} else {
		return now.
			AddDate(years, months, days).
			Add(time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second).
			Format("2006-01-02 15:04:05"), nil
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
