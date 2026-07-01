package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/awesome-gocui/gocui"

	"github.com/MPCodeWriter21/Tigo/pkg/logs"
	"github.com/MPCodeWriter21/Tigo/pkg/utils"
)

type keybinding struct {
	views       []string                            // empty string means global
	keys        []any                               // gocui.Key or rune
	mod         gocui.Modifier                      // Modifier keys like Shift, Alt, etc. Use gocui.ModNone for no modifier.
	handler     func(*gocui.Gui, *gocui.View) error // A function that gets called when the keybinding is triggered.
	description string                              // A short description of the keybinding, used in the help view
	bind        bool                                // Whether the keybinding should be registered by initKeybindings. This allows for keybindings that are added to temporary views to be included in the help view
}

// NOTE: The order of the keybindings in this slice determines the order they are displayed in the help view
var bindings []keybinding = []keybinding{
	{[]string{"details"}, []any{gocui.KeySpace}, gocui.ModNone, followDetail, "Follow the selected detail", true},
	{[]string{"details"}, []any{gocui.KeyArrowDown, 'j'}, gocui.ModNone, cursorDown, "Move down", true},
	{[]string{"details"}, []any{gocui.KeyArrowUp, 'k'}, gocui.ModNone, cursorUp, "Move up", true},
	{[]string{"details"}, []any{gocui.KeyArrowLeft, 'h'}, gocui.ModNone, detailsLeft, "Move left", true},
	{[]string{"details"}, []any{gocui.KeyArrowRight, 'l'}, gocui.ModNone, detailsRight, "Move right", true},
	{[]string{"tasks"}, []any{gocui.KeySpace}, gocui.ModNone, toggleTaskStatus, "Toggle the status of the selected task", true},
	{[]string{"tasks"}, []any{gocui.KeyArrowDown, 'j'}, gocui.ModNone, tasksDown, "Move down", true},
	{[]string{"tasks"}, []any{gocui.KeyArrowUp, 'k'}, gocui.ModNone, tasksUp, "Move up", true},
	{[]string{"tasks"}, []any{'a', 'n'}, gocui.ModNone, promptCreateTask, "Create a new task", true},
	{[]string{"tasks"}, []any{'d'}, gocui.ModNone, promptDeleteTask, "Delete the selected task", true},
	{[]string{"tasks", "details"}, []any{'e'}, gocui.ModNone, promptEditTask, "Edit the selected task", true},
	{[]string{"tasks"}, []any{'b'}, gocui.ModNone, showTaskBlame, "Show blame summary", true},
	{[]string{"details"}, []any{'b'}, gocui.ModNone, showLineBlame, "Show blame for the current line", true},
	{[]string{"tasks"}, []any{'s'}, gocui.ModNone, promptSort, "Sort tasks", true},
	{[]string{"tasks"}, []any{'g'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { setSelectedTask(0); return updateViews(g) }, "Move to the top", true},
	{[]string{"tasks"}, []any{'G'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { setSelectedTask(len(tasks) - 1); return updateViews(g) }, "Move to the bottom", true},
	{[]string{"tasks"}, []any{'y'}, gocui.ModNone, copyLine, "Copy the selected task to the clipboard", true},
	{[]string{"details"}, []any{'g'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { return v.SetCursor(0, 0) }, "Move to the top", true},
	{[]string{"details"}, []any{'G'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { h := v.LinesHeight(); return v.SetCursor(1, h-2) }, "Move to the bottom", true},
	{[]string{"details"}, []any{'y'}, gocui.ModNone, copyDetail, "Copy the selected detail to the clipboard", true},
	{[]string{"details"}, []any{'Y'}, gocui.ModNone, copyLine, "Copy the selected line to the clipboard", true},
	{[]string{"tasks"}, []any{'H'}, gocui.ModNone, toggleShowClosed, "Toggle showing closed tasks", true},
	{[]string{"tasks"}, []any{'r'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		logs.Append(logs.LevelInfo, "Reloading tasks from disk...")
		return reloadTasks(g, v)
	}, "Reload tasks from disk", true},
	{[]string{"tasks"}, []any{gocui.KeyEsc}, gocui.ModNone, clearSearchQuery, "Clear the search query", true},
	{[]string{"tasks", "details", "logs"}, []any{'c'}, gocui.ModNone, promptCommit, "Commit changes", true},
	{[]string{"tasks", "details", "logs"}, []any{'f'}, gocui.ModNone, gitFetch, "Fetch from remote", true},
	{[]string{"tasks", "details", "logs"}, []any{'p'}, gocui.ModNone, gitPull, "Pull commits from remote", true},
	{[]string{"tasks", "details", "logs"}, []any{'P'}, gocui.ModNone, gitPush, "Push commits to remote", true},
	{[]string{"tasks", "details"}, []any{'['}, gocui.ModNone, goBackInHistory, "Go back in task history", true},
	{[]string{"tasks", "details"}, []any{']'}, gocui.ModNone, goForwardInHistory, "Go forward in task history", true},
	{[]string{"tasks", "details", "logs"}, []any{'B'}, gocui.ModNone, promptSwitchBranch, "Switch git branch", true},
	{[]string{"tasks", "details"}, []any{'/'}, gocui.ModNone, promptSearch, "Search tasks", true},
	{[]string{"tasks", "details"}, []any{'o'}, gocui.ModNone, openSelectedTaskDirectory, "Open the directory of the selected task", true},
	{[]string{"tasks", "details"}, []any{'O'}, gocui.ModNone, openSelectedTaskFile, "Open the selected task in the default editor", true},
	{[]string{"tasks", "details"}, []any{'`'}, gocui.ModNone, showCurrentTigoDirectory, "Show the current Tigo directory", true},
	{[]string{"tasks", "details"}, []any{'L'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { _, err := g.SetCurrentView("logs"); return err }, "Focus the logs view", true},
	{[]string{"tasks", "details", "logs"}, []any{'c'}, gocui.ModAlt, openConfigFile, "Open local config", true},
	{[]string{"tasks", "details", "logs"}, []any{'r'}, gocui.ModAlt, reloadConfig, "Reload tigo config", true},
	{[]string{"tasks"}, []any{gocui.KeyTab, gocui.KeyEnter, 'l'}, gocui.ModNone, showDetails, "Focus the details view", true},
	{[]string{"details"}, []any{gocui.KeyTab, gocui.KeyEsc}, gocui.ModNone, setCurrentViewCallback("tasks"), "Focus the tasks view", true},

	{[]string{"logs"}, []any{gocui.KeyTab, gocui.KeyEsc}, gocui.ModNone, setCurrentViewCallback("tasks"), "Focus the tasks view", true},
	{[]string{"logs"}, []any{'h', gocui.KeyArrowLeft}, gocui.ModNone, setCurrentViewCallback("details"), "Focus the details view", true},
	{[]string{"logs", "help"}, []any{gocui.KeyArrowDown, 'j'}, gocui.ModNone, cursorDown, "Scroll down", true},
	{[]string{"logs", "help"}, []any{gocui.KeyArrowUp, 'k'}, gocui.ModNone, cursorUp, "Scroll up", true},
	{[]string{"logs"}, []any{'g'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { return v.SetCursor(0, 0) }, "Scroll to the top", true},
	{[]string{"logs"}, []any{'G'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { h := v.LinesHeight(); return v.SetCursor(1, h-2) }, "Scroll to the bottom", true},
	{[]string{"logs"}, []any{gocui.KeyCtrlL}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { logs.Clear(); return nil }, "Clear all log entries", true},

	{[]string{"deleteDialog"}, []any{gocui.KeyEnter}, gocui.ModNone, submitDeleteTask, "Confirm deleting the selected task", true},
	{[]string{"deleteDialog"}, []any{gocui.KeyEsc}, gocui.ModNone, deleteViewDefault, "Cancel deleting the selected task", true},

	{[]string{"sort"}, []any{'1'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { v.SetCursor(0, 0); return submitSort(g, v) }, "Sort by task ID", true},
	{[]string{"sort"}, []any{'2'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { v.SetCursor(0, 1); return submitSort(g, v) }, "Sort by priority", true},
	{[]string{"sort"}, []any{'3'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { v.SetCursor(0, 2); return submitSort(g, v) }, "Sort by due-date", true},
	{[]string{"sort"}, []any{'4'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { v.SetCursor(0, 3); return submitSort(g, v) }, "Sort by title", true},
	{[]string{"sort"}, []any{'j', gocui.KeyArrowDown}, gocui.ModNone, cursorDown, "Select next sort option", true},
	{[]string{"sort"}, []any{'k', gocui.KeyArrowUp}, gocui.ModNone, cursorUp, "Select previous sort option", true},
	{[]string{"sort"}, []any{gocui.KeyEnter}, gocui.ModNone, submitSort, "Apply the selected sort option", true},
	{[]string{"sort"}, []any{gocui.KeyEsc}, gocui.ModNone, deleteViewDefault, "Close the sort menu", true},

	{[]string{"search"}, []any{gocui.KeyEsc}, gocui.ModNone, searchClose, "Close the search prompt", true},
	{[]string{"search"}, []any{gocui.KeyEnter}, gocui.ModNone, submitSearch, "Submit the search query", true},
	{[]string{"search"}, []any{gocui.KeyArrowUp}, gocui.ModNone, searchCursorUp, "Fill the search view with the previous search query", true},

	{[]string{"commitSubject"}, []any{gocui.KeyEnter}, gocui.ModNone, submitCommit, "Submit the commit message", true},
	{[]string{"commitBody"}, []any{gocui.KeyEnter}, gocui.ModShift, submitCommit, "Submit the commit message", true},
	{[]string{"commitFiles"}, []any{gocui.KeyEnter}, gocui.ModNone, submitCommit, "Submit the commit message", true},
	{[]string{"commitFiles"}, []any{gocui.KeySpace}, gocui.ModNone, toggleCommitFile, "Toggle file selection", true},
	{[]string{"commitSubject"}, []any{gocui.KeyTab}, gocui.ModNone, setCurrentViewCallback("commitBody"), "Switch to the commit description field", true},
	{[]string{"commitBody"}, []any{gocui.KeyTab}, gocui.ModNone, setCurrentViewCallbackCursor("commitFiles", false), "Switch to the file list", true},
	{[]string{"commitFiles"}, []any{gocui.KeyTab}, gocui.ModNone, setCurrentViewCallbackCursor("commitSubject", true), "Switch to the commit message field", true},
	{[]string{"commitFiles"}, []any{'j', gocui.KeyArrowDown}, gocui.ModNone, cursorDown, "Scroll down", true},
	{[]string{"commitFiles"}, []any{'k', gocui.KeyArrowUp}, gocui.ModNone, cursorUp, "Scroll up", true},
	{[]string{"commitFiles"}, []any{'g'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { return v.SetCursor(0, 0) }, "Scroll to the top", true},
	{[]string{"commitFiles"}, []any{'G'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { h := v.LinesHeight(); return v.SetCursor(1, h-2) }, "Scroll to the bottom", true},
	{[]string{"commitSubject", "commitBody", "commitFiles"}, []any{gocui.KeyEsc}, gocui.ModNone, closeCommitDialog, "Close the commit dialog", true},

	{[]string{"branches"}, []any{gocui.KeyEnter}, gocui.ModNone, submitSwitchBranch, "Switch to the selected branch", true},
	{[]string{"branches"}, []any{gocui.KeyEsc}, gocui.ModNone, deleteViewDefault, "Close the branch switcher", true},
	{[]string{"branches"}, []any{'j', gocui.KeyArrowDown}, gocui.ModNone, branchesDown, "Select next branch", true},
	{[]string{"branches"}, []any{'k', gocui.KeyArrowUp}, gocui.ModNone, branchesUp, "Select previous branch", true},
	{[]string{"branches"}, []any{'g'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		selectedBranch = 0
		renderBranches(v)
		v.SetOrigin(0, 0)
		return v.SetCursor(0, 0)
	}, "Go to first branch", true},
	{[]string{"branches"}, []any{'G'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		selectedBranch = len(branches) - 1
		renderBranches(v)
		_, h := v.Size()
		v.SetOrigin(0, max(0, selectedBranch-h+1))
		return v.SetCursor(0, min(selectedBranch, h-1))
	}, "Go to last branch", true},

	{[]string{""}, []any{gocui.KeyCtrlC, 'q'}, gocui.ModNone, quit, "Quit the application", true},
}

// initKeybindings sets up all the keybindings defined in the bindings slice. It also adds a global keybinding for '?' to show the help view with all keybindings.
func initKeybindings(g *gocui.Gui) error {
	for _, binding := range bindings {
		if !binding.bind {
			continue
		}
		for _, key := range binding.keys {
			for _, view := range binding.views {
				if err := g.SetKeybinding(view, key, binding.mod, binding.handler); err != nil {
					return fmt.Errorf("bind %v to view %q: %w", key, view, err)
				}
			}
		}
	}
	// Add the help keybinding separately to avoid initialization cycle for bindings
	if err := g.SetKeybinding("", '?', gocui.ModNone, promptHelpKeybindings); err != nil {
		return fmt.Errorf("bind help key: %w", err)
	}
	if err := g.SetKeybinding("", '?', gocui.ModAlt, promptHelpKeybindings); err != nil {
		return fmt.Errorf("bind alt-help key: %w", err)
	}
	return nil
}

// promptHelpKeybindings shows a message box with all the keybindings and their descriptions.
// It filters the keybindings based on the current view, showing only the relevant ones.
func promptHelpKeybindings(g *gocui.Gui, v *gocui.View) error {
	var message strings.Builder
	type keysDescription struct {
		textLen     int // length of the keysText without ANSI escape codes, used for alignment
		keysText    string
		description string
	}
	var keyDescriptions []keysDescription

	currentView := v.Name()
	// Add the eligible keybindings for the current view to the keyDescriptions slice
	for _, binding := range bindings {
		var keyStrs []string
		if len(binding.views) == 0 || slices.Contains(binding.views, currentView) || slices.Contains(binding.views, "") {
			for _, key := range binding.keys {
				keyStr := fmt.Sprintf("%s%s%s", "\x1b[1;33m", keyToString(key, binding.mod), "\x1b[0m")
				keyStrs = append(keyStrs, keyStr)
			}
			text := strings.Join(keyStrs, ", ")
			keyDescriptions = append(keyDescriptions, keysDescription{
				textLen: utils.TextLen(text), keysText: text, description: binding.description,
			})
		}
	}

	// Calculate the maximum width of the keysText column for alignment
	maxKeysWidth := len("Keybindings") - 2
	for _, kd := range keyDescriptions {
		maxKeysWidth = max(kd.textLen, maxKeysWidth)
	}

	fmt.Fprintf(&message, "\x1b[1;36m%-*s    %s\x1b[0m\n\n", maxKeysWidth, "Keybindings", "Description")
	// Build the message with aligned columns
	for _, kd := range keyDescriptions {
		spaces := strings.Repeat(" ", maxKeysWidth-kd.textLen)
		fmt.Fprintf(&message, "  %s%s  %s\n", kd.keysText, spaces, kd.description)
	}

	maxX, maxY := g.Size()
	width := 0
	height := 1
	for line := range strings.SplitSeq(message.String(), "\n") {
		width = max(width, utils.TextLen(line)+4)
		height++
	}

	// Check if the help view can fit in the current terminal size
	if width > maxX {
		width = maxX - 1
		message.Reset()
		for _, kd := range keyDescriptions {
			// kd.keysText
			paddingLength := (width - utils.TextLen(kd.keysText) - 2) / 2
			padding := strings.Repeat(" ", paddingLength)
			fmt.Fprintf(&message, "%s%s%s\n", padding, kd.keysText, padding)
			// kd.description
			paddingLength = (width - utils.TextLen(kd.description) - 2) / 2
			padding = strings.Repeat(" ", paddingLength)
			// I am not sure if I should underline the description to somewhat separate them from the next keybind
			// White underlined: \x1b[37m;4m
			desc := fmt.Sprintf("\x1b[37m%s%s", padding, kd.description)
			fmt.Fprintf(&message, "%s%-*s\x1b[39m\n", desc, width-utils.TextLen(desc)-1, " ")
		}
	}
	if height > maxY {
		height = maxY - 1
	}

	x0 := maxX/2 - width/2
	y0 := maxY/2 - height/2
	if v, err := g.SetView("help", x0, y0, x0+width, y0+height, 0); err != nil {
		originalCursor := g.Cursor
		g.Cursor = false
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Help / Keybindings"
		v.Editable = false
		fmt.Fprint(v, message.String())
		closeHelp := func(g *gocui.Gui, v *gocui.View) error {
			g.DeleteKeybinding("help", gocui.KeyEnter, gocui.ModNone)
			g.DeleteKeybinding("help", gocui.KeyEsc, gocui.ModNone)
			return deleteViewAndSetCurrent(currentView, originalCursor, false)(g, v)
		}
		g.SetKeybinding("help", gocui.KeyEnter, gocui.ModNone, closeHelp)
		g.SetKeybinding("help", gocui.KeyEsc, gocui.ModNone, closeHelp)
	}
	_, err := g.SetCurrentView("help")
	return err
}

// keyToString converts a gocui.Key to a human-readable string, using Unicode arrows for the arrow keys and angle brackets for special keys.
func keyToString(key any, mod gocui.Modifier) string {
	text := ""
	switch mod {
	case gocui.ModShift:
		text += "Shift+"
	case gocui.ModAlt:
		text += "Alt+"
	case gocui.ModMouseCtrl:
		text += "MouseCtrl+"
	}
	switch k := key.(type) {
	case rune:
		text += fmt.Sprintf("%c", k)
	case gocui.Key:
		switch key {
		case gocui.KeyArrowLeft:
			text += "\u2190"
		case gocui.KeyArrowUp:
			text += "\u2191"
		case gocui.KeyArrowRight:
			text += "\u2192"
		case gocui.KeyArrowDown:
			text += "\u2193"
		case gocui.KeyTab:
			text += "<tab>"
		case gocui.KeyEsc:
			text += "<esc>"
		case gocui.KeyEnter:
			text += "<enter>"
		case gocui.KeySpace:
			text += "<space>"
		case gocui.KeyCtrlC:
			text += "<ctrl+c>"
		case gocui.KeyCtrlL:
			text += "<ctrl+l>"
		default:
			text += fmt.Sprintf("%v", key)
		}
	default:
		panic(fmt.Sprintf("invalid key type: %T", key))
	}
	return text
}
