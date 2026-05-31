package ui

import (
	"fmt"
	"regexp"
	"strings"

	"tigo/pkg/db"

	"github.com/awesome-gocui/gocui"
	"golang.design/x/clipboard"
)

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func closeDialog(g *gocui.Gui, v *gocui.View) error {
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

func toggleTaskStatus(g *gocui.Gui, v *gocui.View) error {
	if len(tasks) > 0 && selected < len(tasks) {
		db.ToggleStatus(tigoRoot, tasks[selected])
	}
	return nil
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
	if err := clipboard.Init(); err != nil {
		return err
	}
	clipboard.Write(clipboard.FmtText, []byte(line))
	return nil
}

func setCurrentViewCallback(name string) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		_, err := g.SetCurrentView(name)
		return err
	}
}

// Works the same as fmt.Fprintf, except if searchQuery is set, it highlights the matching text in the view.
func searchedFprintf(v *gocui.View, format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	if searchQuery != "" {
		re := regexp.MustCompile("(?i)" + searchQuery)
		highlighted := re.ReplaceAllStringFunc(line, func(match string) string {
			return fmt.Sprintf("\x1b[43m%s\x1b[40m", match) // Highlight with yellow background
		})
		fmt.Fprint(v, highlighted)
	} else {
		fmt.Fprint(v, line)
	}
}

// Prints the formatted string centered in the view. Returns the number of padding spaces added on each side.
func centeredFprintf(v *gocui.View, format string, a ...any) (int, error) {
	line := fmt.Sprintf(format, a...)
	trimmedLine := strings.TrimRight(line, "\n")
	trailingNewLines := line[len(trimmedLine):]
	width, _ := v.Size()
	paddingLength := (width - len(line)) / 2
	padding := strings.Repeat(" ", paddingLength)
	_, err := fmt.Fprintf(v, "%s%s%s%s", padding, trimmedLine, padding, trailingNewLines)
	return paddingLength, err
}
