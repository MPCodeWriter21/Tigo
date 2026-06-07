package ui

import (
	"fmt"
	"slices"
	"strings"
	"tigo/pkg/logs"
	"unicode/utf8"

	"github.com/awesome-gocui/gocui"
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
	{[]string{"details"}, []any{'g'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { return v.SetCursor(0, 0) }, "Move to the top", true},
	{[]string{"details"}, []any{'G'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { h := v.LinesHeight(); return v.SetCursor(1, h-2) }, "Move to the bottom", true},
	{[]string{"details"}, []any{'y'}, gocui.ModNone, copyDetail, "Copy the selected detail to the clipboard", true},
	{[]string{"details"}, []any{'Y'}, gocui.ModNone, copyLine, "Copy the selected line to the clipboard", true},
	{[]string{"tasks"}, []any{gocui.KeySpace}, gocui.ModNone, toggleTaskStatus, "Toggle the status of the selected task", true},
	{[]string{"tasks"}, []any{gocui.KeyArrowDown, 'j'}, gocui.ModNone, tasksDown, "Move down", true},
	{[]string{"tasks"}, []any{gocui.KeyArrowUp, 'k'}, gocui.ModNone, tasksUp, "Move up", true},
	{[]string{"tasks"}, []any{'a', 'n'}, gocui.ModNone, promptCreateTask, "Create a new task", true},
	{[]string{"tasks"}, []any{'d'}, gocui.ModNone, promptDeleteTask, "Delete the selected task", true},
	{[]string{"tasks"}, []any{'y'}, gocui.ModNone, copyLine, "Copy the selected task to the clipboard", true},
	{[]string{"tasks"}, []any{'s'}, gocui.ModNone, promptSort, "Sort tasks", true},
	{[]string{"tasks"}, []any{'g'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { selectedTask = 0; return updateViews(g) }, "Move to the top", true},
	{[]string{"tasks"}, []any{'G'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { selectedTask = len(tasks) - 1; return updateViews(g) }, "Move to the bottom", true},
	{[]string{"tasks"}, []any{'H'}, gocui.ModNone, toggleShowClosed, "Toggle showing closed tasks", true},
	{[]string{"tasks"}, []any{'r'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { return loadTasks() }, "Reload tasks from disk", true},
	{[]string{"tasks"}, []any{gocui.KeyEsc}, gocui.ModNone, clearSearchQuery, "Clear the search query", true},
	{[]string{"tasks", "details"}, []any{'e'}, gocui.ModNone, promptEditTask, "Edit the selected task", true},
	{[]string{"tasks", "details"}, []any{'/'}, gocui.ModNone, promptSearch, "Search tasks", true},
	{[]string{"tasks", "details"}, []any{'o'}, gocui.ModNone, openSelectedTaskDirectory, "Open the directory of the selected task", true},
	{[]string{"tasks", "details"}, []any{'O'}, gocui.ModNone, openSelectedTaskFile, "Open the selected task in the default editor", true},
	{[]string{"tasks", "details"}, []any{'`'}, gocui.ModNone, showCurrentTigoDirectory, "Show the current Tigo directory", true},
	{[]string{"tasks", "details"}, []any{'L'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { _, err := g.SetCurrentView("logs"); return err }, "Focus the logs view", true},
	{[]string{"tasks"}, []any{gocui.KeyTab, gocui.KeyEnter, 'l'}, gocui.ModNone, showDetails, "Focus the details view", true},
	{[]string{"details"}, []any{gocui.KeyTab, gocui.KeyEsc}, gocui.ModNone, setCurrentViewCallback("tasks"), "Focus the tasks view", true},

	{[]string{"logs"}, []any{gocui.KeyTab, gocui.KeyEsc}, gocui.ModNone, setCurrentViewCallback("tasks"), "Focus the tasks view", true},
	{[]string{"logs"}, []any{'h', gocui.KeyArrowLeft}, gocui.ModNone, setCurrentViewCallback("details"), "Focus the details view", true},
	{[]string{"logs"}, []any{gocui.KeyArrowDown, 'j'}, gocui.ModNone, cursorDown, "Scroll down", true},
	{[]string{"logs"}, []any{gocui.KeyArrowUp, 'k'}, gocui.ModNone, cursorUp, "Scroll up", true},
	{[]string{"logs"}, []any{'g'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { return v.SetCursor(0, 0) }, "Scroll to the top", true},
	{[]string{"logs"}, []any{'G'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { h := v.LinesHeight(); return v.SetCursor(1, h-2) }, "Scroll to the bottom", true},
	{[]string{"logs"}, []any{'C'}, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error { logs.Clear(); return nil }, "Clear all log entries", true},

	{[]string{"deleteDialog"}, []any{gocui.KeyEnter}, gocui.ModNone, submitDeleteTask, "Confirm deleting the selected task", true},
	{[]string{"deleteDialog"}, []any{gocui.KeyEsc}, gocui.ModNone, deleteViewDefault, "Cancel deleting the selected task", true},

	{[]string{"sort"}, []any{'1'}, gocui.ModNone, nil, "Sort by task ID", true},
	{[]string{"sort"}, []any{'2'}, gocui.ModNone, nil, "Sort by priority", true},
	{[]string{"sort"}, []any{'3'}, gocui.ModNone, nil, "Sort by due-date", true},
	{[]string{"sort"}, []any{'4'}, gocui.ModNone, nil, "Sort by title", true},
	{[]string{"sort"}, []any{'j', gocui.KeyArrowDown}, gocui.ModNone, nil, "Select next sort option", true},
	{[]string{"sort"}, []any{'k', gocui.KeyArrowUp}, gocui.ModNone, nil, "Select previous sort option", true},
	{[]string{"sort"}, []any{gocui.KeyEnter}, gocui.ModNone, nil, "Apply the selected sort option", true},
	{[]string{"sort"}, []any{gocui.KeyEsc}, gocui.ModNone, nil, "Close the sort menu", true},

	{[]string{"search"}, []any{gocui.KeyEsc}, gocui.ModNone, clearSearchQuery, "Close the search prompt", true},
	{[]string{"search"}, []any{gocui.KeyEnter}, gocui.ModNone, submitSearch, "Submit the search query", true},
	{[]string{"search"}, []any{gocui.KeyArrowUp}, gocui.ModNone, searchCursorUp, "Fill the search view with the previous search query", true},

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
				var keyStr string
				switch k := key.(type) {
				case gocui.Key:
					keyStr = fmt.Sprintf("%s%s%s", "\x1b[1;33m", keyToString(k), "\x1b[0m")
				case rune:
					keyStr = fmt.Sprintf("%s%c%s", "\x1b[1;33m", k, "\x1b[0m")
				default:
					panic(fmt.Sprintf("invalid key type: %T", key))
				}
				keyStrs = append(keyStrs, keyStr)
			}
			text := strings.Join(keyStrs, ", ")
			cleanText := allANSIRegex.ReplaceAllString(text, "")
			keyDescriptions = append(keyDescriptions, keysDescription{
				textLen: utf8.RuneCountInString(cleanText), keysText: text, description: binding.description,
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

	return promptMessageBox(g, "Help / Keybindings", message.String(), "", false)
}

// keyToString converts a gocui.Key to a human-readable string, using Unicode arrows for the arrow keys and angle brackets for special keys.
func keyToString(key gocui.Key) string {
	switch key {
	case gocui.KeyArrowLeft:
		return "\u2190"
	case gocui.KeyArrowUp:
		return "\u2191"
	case gocui.KeyArrowRight:
		return "\u2192"
	case gocui.KeyArrowDown:
		return "\u2193"
	case gocui.KeyTab:
		return "<tab>"
	case gocui.KeyEsc:
		return "<esc>"
	case gocui.KeyEnter:
		return "<enter>"
	case gocui.KeySpace:
		return "<space>"
	case gocui.KeyCtrlC:
		return "<ctrl+c>"
	default:
		return fmt.Sprintf("%v", key)
	}
}
