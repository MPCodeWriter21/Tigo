package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awesome-gocui/gocui"

	"github.com/MPCodeWriter21/Tigo/pkg/utils"
)

type acType int

const (
	acNone acType = iota
	acTask
	acFilePath
)

type autoComplete struct {
	g         *gocui.Gui
	active    bool
	typ       acType
	trigger   string // the trigger text e.g. "Task(", "./", "../"
	prefix    string // what the user typed after the trigger
	items     []string
	selected  int
	searchDir string // for file paths, the directory being searched (relative to task root) - if non-empty, ends with /
	startCol  int    // column where the trigger starts
	viewName  string
}

var ac autoComplete

func acReset() {
	ac.active = false
	ac.typ = acNone
	ac.trigger = ""
	ac.prefix = ""
	ac.items = nil
	ac.selected = 0
	ac.searchDir = ""
	ac.startCol = 0
	ac.viewName = ""
}

func checkTrigger(v *gocui.View) {
	cx, cy := v.Cursor()
	line, err := v.Line(cy)
	if err != nil || cx <= 0 {
		acHide()
		return
	}

	textBefore := line[:cx]

	// Check for Task( trigger
	if idx := strings.LastIndex(strings.ToUpper(textBefore), "TASK("); idx >= 0 {
		prefix := textBefore[idx+len("TASK("):]
		showTaskAutoComplete(v.Name(), idx, prefix)
		return
	}

	// Check for ../ trigger (must be preceded by space or at line start)
	dotIdx := strings.LastIndex(textBefore, "../")
	if dotIdx == 0 || (dotIdx > 0 && !strings.ContainsAny(string(textBefore[dotIdx-1]), "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_")) {
		prefix := textBefore[dotIdx+len("../"):]
		showFilePathComplete(v.Name(), dotIdx, "../", prefix)
		return
	}
	// Check for ./ trigger (must be preceded by space or at line start, and not part of ../)
	dotIdx = strings.LastIndex(textBefore, "./")
	if dotIdx == 0 || (dotIdx > 0 && !strings.ContainsAny(string(textBefore[dotIdx-1]), "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_")) {
		prefix := textBefore[dotIdx+len("./"):]
		showFilePathComplete(v.Name(), dotIdx, "./", prefix)
		return
	}

	acHide()
}

func showTaskAutoComplete(viewName string, startCol int, prefix string) {
	itemMap := make(map[string]int)
	prefixLower := strings.ToLower(prefix)
	for _, t := range tasks {
		display := fmt.Sprintf("%s  %s", t.ID, t.Title)
		if prefixLower == "" || strings.Contains(strings.ToLower(t.ID), prefixLower) || strings.HasPrefix(strings.ToLower(t.Title), prefixLower) {
			itemMap[display] = 10
		} else if strings.Contains(strings.ToLower(t.Title), prefixLower) {
			itemMap[display] = strings.Count(strings.ToLower(t.Title), prefixLower)
		}
	}
	if len(itemMap) == 0 {
		acHide()
		return
	}

	ac.active = true
	ac.typ = acTask
	ac.trigger = "Task("
	ac.prefix = prefix
	ac.items = utils.SortedKeysByValue(itemMap)
	ac.selected = 0
	ac.searchDir = ""
	ac.startCol = startCol
	ac.viewName = viewName

	acShow()
}

func showFilePathComplete(viewName string, startCol int, trigger, prefix string) {
	if len(tasks) == 0 || selectedTask < 0 || selectedTask >= len(tasks) {
		acHide()
		return
	}
	currentTask := tasks[selectedTask]

	var baseDir string
	switch trigger {
	case "../":
		baseDir = filepath.Dir(filepath.Join(tigoRoot, currentTask.ID))
	case "./":
		baseDir = filepath.Join(tigoRoot, currentTask.ID)
	default:
		panic("Unexpected trigger in showFilePathComplete: " + trigger)
	}

	searchDir := baseDir
	filterPrefix := prefix

	if prefix != "" {
		if strings.HasSuffix(prefix, "/") || strings.HasSuffix(prefix, string(filepath.Separator)) {
			searchDir = filepath.Join(baseDir, prefix)
			filterPrefix = ""
		} else {
			searchDir = filepath.Dir(filepath.Join(baseDir, prefix))
			filterPrefix = filepath.Base(prefix)
		}
	}

	ac.searchDir = strings.Trim(strings.ReplaceAll(strings.TrimPrefix(searchDir, baseDir), "\\", "/"), "/")
	if ac.searchDir != "" {
		ac.searchDir += "/"
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		acHide()
		return
	}

	itemMap := make(map[string]int)
	filterLower := strings.ToLower(filterPrefix)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		if prefix == ac.searchDir+name {
			continue
		}
		if filterLower == "" || strings.HasPrefix(strings.ToLower(name), filterLower) {
			itemMap[name] = 10
		} else if strings.Contains(strings.ToLower(name), filterLower) {
			itemMap[name] = strings.Count(strings.ToLower(name), filterLower)
		}
	}
	if len(itemMap) == 0 {
		acHide()
		return
	}

	ac.active = true
	ac.typ = acFilePath
	ac.trigger = trigger
	ac.prefix = prefix
	ac.items = utils.SortedKeysByValue(itemMap)
	ac.selected = 0
	ac.startCol = startCol
	ac.viewName = viewName

	acShow()
}

func acShow() {
	if ac.active && len(ac.items) == 0 {
		return
	}

	maxX, maxY := ac.g.Size()
	width := max(maxX/3, 30)
	height := min(max(maxY/4, 6), len(ac.items)+2)

	parentView, err := ac.g.View(ac.viewName)
	if err != nil {
		return
	}
	x0, y0, x1, _ := parentView.Dimensions()
	w := x1 - x0
	cx, cy := parentView.Cursor()
	if cx >= w {
		cy += cx / (w - 1)
		cx = 1
	} else {
		cx -= len(ac.prefix)
	}
	ox, oy := parentView.Origin()
	x0 += cx - ox + 1
	y0 += cy - oy + 2
	if x0+width > maxX {
		x0 = maxX - width - 1
	}
	if y0+height > maxY {
		y0 = maxY - height
		x0 += len(ac.prefix)
	}

	v, err := ac.g.SetView("autoComplete", x0, y0, x0+width-1, y0+height-1, 0)
	if err != nil && err != gocui.ErrUnknownView {
		return
	}
	v.Frame = true
	v.FgColor = gocui.Get256Color(255)

	_, oy = v.Origin()
	_, h := v.Size()
	v.Clear()

	for i, item := range ac.items {
		if i == ac.selected {
			fmt.Fprintf(v, "\x1b[7m %s \x1b[0m\n", item)
		} else {
			fmt.Fprintf(v, " %s \n", item)
		}
	}

	if ac.selected < oy+2 {
		oy = ac.selected - 2
	}
	if ac.selected > oy+h-3 {
		oy = ac.selected - h + 3
	}
	if oy < 0 {
		oy = 0
	}
	if oy > len(ac.items)-h {
		oy = len(ac.items) - h
	}
	v.SetOrigin(0, oy)
	v.SetCursor(0, ac.selected-oy)
}

func acHide() {
	ac.g.DeleteView("autoComplete")
	acReset()
}

func acPrev() {
	if ac.selected > 0 {
		ac.selected--
		acShow()
	}
}

func acNext() {
	if ac.selected < len(ac.items)-1 {
		ac.selected++
		acShow()
	}
}

func acAccept(v *gocui.View) {
	if !ac.active || v.Name() != ac.viewName || ac.selected >= len(ac.items) {
		return
	}

	item := ac.items[ac.selected]
	cx, cy := v.Cursor()

	var insertText string
	switch ac.typ {
	case acTask:
		id, _, _ := strings.Cut(item, "  ")
		insertText = "Task(" + id + ")"
	case acFilePath:
		insertText = ac.trigger + ac.searchDir + item
	}

	// Delete from startCol to cursor
	v.SetCursor(ac.startCol, cy)
	for range cx - ac.startCol {
		v.EditDelete(false)
	}

	// Insert the completion
	for _, ch := range insertText {
		v.EditWrite(ch)
	}

	acHide()
	checkTrigger(v)
}
