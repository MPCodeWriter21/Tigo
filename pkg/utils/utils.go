// Package utils provides utility functions for the Tigo application, including
// text processing, date parsing, file operations, and task-related utilities.
package utils

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	DetailsRegEx  = regexp.MustCompile(`(?:\x1b\[(1;[0-9]+)m)|(?:\x1b\[(3[2-4];4)m)`)
	AllANSIRegex  = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	IDRegEx       = regexp.MustCompile(`[0-9]{8}-[0-9]{6}`)
	TaskRegEx     = regexp.MustCompile(`(?i)TASK\([0-9]{8}-[0-9]{6}\)`)
	FilePathRegEx = regexp.MustCompile(`(?:(?P<relative>\.\.?\/)(?P<path>[\w\-\/\. ]+))`)
	URLRegEx      = regexp.MustCompile(`(?:(?P<protocol>[a-z\-]+):\/\/(?P<hostname>[-a-zA-Z0-9]+(?:\.[-a-zA-Z0-9]+)+)(?P<port>:[0-9]+)?(?P<path>(?:\/[-a-zA-Z0-9()@:%_\+.~#?&=!]*)*))`)

	// DateFormats lists all ISO/RFC date layouts tried when parsing a due date.
	DateFormats = []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05+07:00",
		"2006-01-02 15:04:05+07:00",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02T15:04Z07:00",
		"2006-01-02T15:04+07:00",
		"2006-01-02 15:04+07:00",
		"2006-01-02 15:04 -0700",
		"2006-01-02 15:04Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
)

// TextLen returns the length of the text without counting ANSI escape codes.
func TextLen(text string) int {
	return utf8.RuneCountInString(AllANSIRegex.ReplaceAllString(text, ""))
}

// CalcVisualLines returns the number of visual lines the given text would
// occupy when word-wrapped to contentWidth columns.
func CalcVisualLines(content string, contentWidth int) int {
	if contentWidth < 1 {
		contentWidth = 1
	}
	lines := 0
	for line := range strings.SplitSeq(content, "\n") {
		lineLen := TextLen(line)
		lines += lineLen/contentWidth + 1
	}
	return max(lines, 1)
}

// ParseRelativeDateTime takes a string like:
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
func ParseRelativeDateTime(input string) (string, *time.Time, error) {
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
	re := regexp.MustCompile(`^next \s*(second|minute|hour|day|week|month|season|year|decade|century)s?$`)
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

// ParseDueDateTime parses a date string into a time.Time value.
// Returns nil if the string cannot be parsed (invalid/empty format).
func ParseDueDateTime(dueDate string) *time.Time {
	if dueDate == "" {
		return nil
	}
	for _, f := range DateFormats {
		t, err := time.Parse(f, dueDate)
		if err == nil {
			return &t
		}
	}
	return nil
}

// OpenFile opens the file at the given path with the default application.
func OpenFile(path string) error {
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

// OpenInEditor opens the given file in the user's default text editor.
// IMPORTANT: Make sure to suspend the UI before calling this function, and resume it after the editor is closed, to prevent UI glitches:
//
//	gocui.Suspend()
//	defer gocui.Resume()
func OpenInEditor(filePath string) error {
	// 1. Check environment variables
	if editor := os.Getenv("VISUAL"); editor != "" {
		return RunEditor(editor, filePath)
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return RunEditor(editor, filePath)
	}

	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		// 2. Try Debian/Ubuntu alternatives
		if editor, err := exec.LookPath("sensible-editor"); err == nil {
			return RunEditor(editor, filePath)
		}
		if editor, err := exec.LookPath("editor"); err == nil {
			return RunEditor(editor, filePath)
		}

		// 3. Search common editors
		for _, ed := range []string{
			"vim", "vi", "nano", "emacs", "micro", "helix", "code", "gedit", "kate",
		} {
			if path, err := exec.LookPath(ed); err == nil {
				return RunEditor(path, filePath)
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

// RunEditor starts an editor with arguments; if the editor supports
// multiple files you might need to split the `editor` variable smartly.
func RunEditor(editor, filePath string) error {
	// Handle editors defined with arguments, e.g. "code --wait".
	parts := strings.Fields(editor)
	args := append(parts[1:], filePath)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run() // Run blocks until the editor exits (useful for terminal editors)
}

// DueColor returns an ANSI color code for the task based on its due date proximity.
// Returns empty string if due coloring is disabled or no due date is set.
func DueColor(dt *time.Time) string {
	if dt == nil {
		return ""
	}
	now := time.Now()
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

// SortedKeysByValue returns a slice of the map's keys sorted by their values in descending order.
// If two keys have the same value, they are sorted in alphabetical order.
func SortedKeysByValue(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] == m[keys[j]] {
			return keys[i] < keys[j]
		}
		return m[keys[i]] > m[keys[j]]
	})
	return keys
}
