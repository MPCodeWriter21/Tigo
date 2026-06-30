// Package cli implements the command-line interface for Tigo.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MPCodeWriter21/Tigo/internal/config"
	"github.com/MPCodeWriter21/Tigo/pkg/db"
	"github.com/MPCodeWriter21/Tigo/pkg/task"
	"github.com/MPCodeWriter21/Tigo/pkg/utils"
)

// Run executes a CLI command against the given tigo root and config.
func Run(root string, cfg *config.TigoConfig, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified; use 'help' for available commands")
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "help":
		return cmdHelp(cmdArgs)
	case "create":
		return cmdCreate(root, cfg, cmdArgs)
	case "list":
		return cmdList(root, cfg, cmdArgs)
	case "show":
		return cmdShow(root, cmdArgs)
	case "edit":
		return cmdEdit(root, cmdArgs)
	case "delete":
		return cmdDelete(root, cmdArgs)
	case "close":
		return cmdClose(root, cmdArgs)
	case "open":
		return cmdOpen(root, cmdArgs)
	case "tags":
		return cmdTags(root, cmdArgs)
	case "top":
		return cmdTop(root, cmdArgs)
	case "next":
		return cmdNext(root, cmdArgs)
	case "overdue":
		return cmdOverdue(root, cfg, cmdArgs)
	case "search":
		return cmdSearch(root, cfg, cmdArgs)
	case "stats":
		return cmdStats(root, cmdArgs)
	default:
		return fmt.Errorf("unknown command: %q; use 'help' for available commands", cmd)
	}
}

// PrintHelp prints the help text for all CLI commands.
func PrintHelp() {
	fmt.Println("  help [cmd]              Show help for a command")
	fmt.Println("  create <title>          Create a new task")
	fmt.Println("  list                    List tasks")
	fmt.Println("  show <id>               Show task details")
	fmt.Println("  edit <id>               Edit a task")
	fmt.Println("  delete <id>             Delete a task")
	fmt.Println("  open <id>               Set a task's status to OPEN")
	fmt.Println("  close <id>              Set a task's status to CLOSED")
	fmt.Println("  tags                    Show task counts per tag")
	fmt.Println("  top <n>                 Show top n tasks by priority")
	fmt.Println("  next                    Show the next task due")
	fmt.Println("  overdue                 Show overdue tasks")
	fmt.Println("  search <query>          Search tasks by title/description/tags")
	fmt.Println("  stats                   Show task statistics")
	fmt.Println()
	fmt.Println("Use 'help <cmd>' for detailed usage of a specific command.")
}

// loadAllTasks loads and parses all tasks from the root directory.
func loadAllTasks(root string) ([]*task.Task, error) {
	ids, err := db.DiscoverTasks(root)
	if err != nil {
		return nil, err
	}
	var tasks []*task.Task
	for _, id := range ids {
		t, err := task.Parse(id, filepath.Join(root, id, "TASK.md"))
		if err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// findTaskByID finds a single task by its ID.
func findTaskByID(root, id string) (*task.Task, error) {
	taskPath := filepath.Join(root, id, "TASK.md")
	if _, err := os.Stat(taskPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("task %q not found", id)
	}
	return task.Parse(id, taskPath)
}

// parseStatusFilter parses a --status flag value.
// Returns the list of statuses to accept.
// If the value is empty or "*", returns nil (accept all).
// Multiple statuses can be comma-separated: "open,closed,custom".
func parseStatusFilter(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return nil
	}
	var statuses []string
	for s := range strings.SplitSeq(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			statuses = append(statuses, strings.ToUpper(s))
		}
	}
	if len(statuses) == 0 {
		return nil
	}
	return statuses
}

// matchesStatus checks if a task's status is in the given list.
// If statuses is nil, all statuses match.
func matchesStatus(t *task.Task, statuses []string) bool {
	if statuses == nil {
		return true
	}
	return slices.Contains(statuses, t.Status)
}

// formatTaskShort formats a task as a compact one-liner.
func formatTaskShort(t *task.Task) string {
	due := ""
	if t.DueDate != "" {
		due = " DUE:" + t.DueDate
	}
	tags := ""
	if len(t.Tags) > 0 {
		tags = " [" + strings.Join(t.Tags, ", ") + "]"
	}
	return fmt.Sprintf("%s %s P:%d %s%s%s", t.ID, t.Status, t.Priority, t.Title, tags, due)
}

// cmdHelp prints help for a specific command or lists all commands.
func cmdHelp(args []string) error {
	if len(args) == 0 {
		PrintHelp()
		return nil
	}

	cmd := args[0]
	switch cmd {
	case "create":
		fmt.Println("Usage: create <title> [--priority <n>] [--tags <t1,t2>] [--due <date>] [--description <text>]")
		fmt.Println("  Create a new task with the given title and optional attributes.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c create \"Fix login bug\"")
		fmt.Println("  tigo -c create \"Write docs\" --priority 80 --tags documentation,urgent")
		fmt.Println("  tigo -c create \"Buy groceries\" --due tomorrow --priority 30")
		fmt.Println("  tigo -c create \"Release v2.0\" --priority 100 --due 2026-12-31 --tags release")
	case "list":
		fmt.Println("Usage: list [--status <status>] [--tag <tag>] [--priority-min <n>] [--priority-max <n>] [--sort <field>] [--limit <n>]")
		fmt.Println("  List tasks, optionally filtered and sorted.")
		fmt.Println("  --status: open, closed, *, or comma-separated (default: open)")
		fmt.Println("  --sort: id, priority, due-date, title (default: from config)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c list")
		fmt.Println("  tigo -c list --status open")
		fmt.Println("  tigo -c list --status *")
		fmt.Println("  tigo -c list --status open,closed")
		fmt.Println("  tigo -c list --tag urgent --priority-min 50")
		fmt.Println("  tigo -c list --sort priority --limit 5")
	case "show":
		fmt.Println("Usage: show <id>")
		fmt.Println("  Show the full details of a task.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c show 20260630-123456")
	case "edit":
		fmt.Println("Usage: edit <id> [--title <t>] [--priority <n>] [--tags <t1,t2>] [--due <date>] [--description <text>] [--status <open|closed>]")
		fmt.Println("  Edit an existing task. Only provided fields are changed.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c edit 20260630-123456 --priority 90 --status closed")
		fmt.Println("  tigo -c edit 20260630-123456 --title \"New title\" --tags bug,frontend")
		fmt.Println("  tigo -c edit 20260630-123456 --due \"next week\" --description \"Updated description\"")
	case "delete":
		fmt.Println("Usage: delete <id>")
		fmt.Println("  Permanently delete a task.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c delete 20260630-123456")
	case "open":
		fmt.Println("Usage: open <id>")
		fmt.Println("  Set the task's status to OPEN.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c open 20260630-123456")
	case "close":
		fmt.Println("Usage: close <id>")
		fmt.Println("  Set the task's status to CLOSED.")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c close 20260630-123456")
	case "tags":
		fmt.Println("Usage: tags [--status <status>] [--sort <name|count>]")
		fmt.Println("  Show the number of tasks per tag.")
		fmt.Println("  --status: open, closed, *, or comma-separated (default: open)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c tags")
		fmt.Println("  tigo -c tags --status open")
		fmt.Println("  tigo -c tags --status *")
		fmt.Println("  tigo -c tags --sort count")
	case "top":
		fmt.Println("Usage: top <n> [--status <status>]")
		fmt.Println("  Show the top n tasks by priority (highest first).")
		fmt.Println("  --status: open, closed, *, or comma-separated (default: open)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c top 5")
		fmt.Println("  tigo -c top 10 --status open")
		fmt.Println("  tigo -c top 5 --status *")
	case "next":
		fmt.Println("Usage: next [--status <status>]")
		fmt.Println("  Show the next task due.")
		fmt.Println("  --status: open, closed, *, or comma-separated (default: open)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c next")
		fmt.Println("  tigo -c next --status open")
		fmt.Println("  tigo -c next --status *")
	case "overdue":
		fmt.Println("Usage: overdue [--status <status>]")
		fmt.Println("  Show all overdue tasks.")
		fmt.Println("  --status: open, closed, *, or comma-separated (default: open)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c overdue")
		fmt.Println("  tigo -c overdue --status open")
		fmt.Println("  tigo -c overdue --status *")
	case "search":
		fmt.Println("Usage: search <query> [--status <status>] [--limit <n>]")
		fmt.Println("  Search tasks by title, description, or tags.")
		fmt.Println("  --status: open, closed, *, or comma-separated (default: open)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c search database")
		fmt.Println("  tigo -c search urgent --status open --limit 5")
		fmt.Println("  tigo -c search bug --status open,closed")
	case "stats":
		fmt.Println("Usage: stats")
		fmt.Println("  Show task statistics (total, open, closed, overdue).")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  tigo -c stats")
	default:
		return fmt.Errorf("unknown command: %q", cmd)
	}
	return nil
}

// cmdCreate handles the 'create' command.
func cmdCreate(root string, cfg *config.TigoConfig, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: create <title> [--priority <n>] [--tags <t1,t2>] [--due <date>] [--description <text>]")
	}

	title := args[0]
	priority := cfg.DefaultPriority
	var tags []string
	var dueDate, description string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--priority":
			if i+1 >= len(args) {
				return fmt.Errorf("--priority requires a value")
			}
			p, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid priority: %q", args[i+1])
			}
			if p < 0 {
				return fmt.Errorf("priority must be non-negative")
			}
			priority = p
			i++
		case "--tags":
			if i+1 >= len(args) {
				return fmt.Errorf("--tags requires a value")
			}
			for tag := range strings.SplitSeq(args[i+1], ",") {
				t := strings.TrimSpace(tag)
				if t != "" {
					tags = append(tags, t)
				}
			}
			i++
		case "--due":
			if i+1 >= len(args) {
				return fmt.Errorf("--due requires a value")
			}
			dueDate = args[i+1]
			// Validate the due date is parseable
			if utils.ParseDueDateTime(dueDate) == nil {
				// Try relative date parsing
				parsed, _, err := utils.ParseRelativeDateTime(dueDate)
				if err != nil {
					return fmt.Errorf("invalid due date: %q (use YYYY-MM-DD or relative like 'tomorrow', '3 days')", dueDate)
				}
				dueDate = parsed
			}
			i++
		case "--description":
			if i+1 >= len(args) {
				return fmt.Errorf("--description requires a value")
			}
			description = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag: %q", args[i])
		}
	}

	id, err := db.CreateNewTask(root, title, priority, tags, dueDate, description)
	if err != nil {
		return err
	}
	fmt.Printf("Created task %s: %s\n", id, title)
	return nil
}

// cmdList handles the 'list' command.
func cmdList(root string, cfg *config.TigoConfig, args []string) error {
	statusFilter := "OPEN"
	tagFilter := ""
	priorityMin := -1
	priorityMax := -1
	sortBy := cfg.SortBy
	limit := -1

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			statusFilter = args[i+1]
			i++
		case "--tag":
			if i+1 >= len(args) {
				return fmt.Errorf("--tag requires a value")
			}
			tagFilter = args[i+1]
			i++
		case "--priority-min":
			if i+1 >= len(args) {
				return fmt.Errorf("--priority-min requires a value")
			}
			p, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid priority: %q", args[i+1])
			}
			priorityMin = p
			i++
		case "--priority-max":
			if i+1 >= len(args) {
				return fmt.Errorf("--priority-max requires a value")
			}
			p, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid priority: %q", args[i+1])
			}
			priorityMax = p
			i++
		case "--sort":
			if i+1 >= len(args) {
				return fmt.Errorf("--sort requires a value")
			}
			sortBy = args[i+1]
			if sortBy != "id" && sortBy != "priority" && sortBy != "due-date" && sortBy != "title" {
				return fmt.Errorf("sort must be one of: id, priority, due-date, title")
			}
			i++
		case "--limit":
			if i+1 >= len(args) {
				return fmt.Errorf("--limit requires a value")
			}
			l, err := strconv.Atoi(args[i+1])
			if err != nil || l <= 0 {
				return fmt.Errorf("invalid limit: %q", args[i+1])
			}
			limit = l
			i++
		default:
			return fmt.Errorf("unknown flag: %q", args[i])
		}
	}

	tasks, err := loadAllTasks(root)
	if err != nil {
		return err
	}

	// Apply filters
	statuses := parseStatusFilter(statusFilter)
	var filtered []*task.Task
	for _, t := range tasks {
		if !matchesStatus(t, statuses) {
			continue
		}
		if tagFilter != "" {
			match := false
			for _, tag := range t.Tags {
				if strings.EqualFold(tag, tagFilter) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		if priorityMin >= 0 && t.Priority < priorityMin {
			continue
		}
		if priorityMax >= 0 && t.Priority > priorityMax {
			continue
		}
		filtered = append(filtered, t)
	}

	// Sort
	switch sortBy {
	case "id":
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].ID < filtered[j].ID
		})
	case "priority":
		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].Priority != filtered[j].Priority {
				return filtered[i].Priority > filtered[j].Priority
			}
			return filtered[i].ID < filtered[j].ID
		})
	case "due-date":
		sort.Slice(filtered, func(i, j int) bool {
			ai, aj := filtered[i], filtered[j]
			if ai.DueDateTime == nil && aj.DueDateTime == nil {
				return ai.ID < aj.ID
			}
			if ai.DueDateTime == nil {
				return false
			}
			if aj.DueDateTime == nil {
				return true
			}
			return ai.DueDateTime.Before(*aj.DueDateTime)
		})
	case "title":
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Title < filtered[j].Title
		})
	}

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	if len(filtered) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}
	for _, t := range filtered {
		fmt.Println(formatTaskShort(t))
	}
	return nil
}

// cmdShow handles the 'show' command.
func cmdShow(root string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: show <id>")
	}

	t, err := findTaskByID(root, args[0])
	if err != nil {
		return err
	}

	fmt.Printf("ID:          %s\n", t.ID)
	fmt.Printf("Title:       %s\n", t.Title)
	fmt.Printf("Status:      %s\n", t.Status)
	fmt.Printf("Priority:    %d\n", t.Priority)
	if len(t.Tags) > 0 {
		fmt.Printf("Tags:        %s\n", strings.Join(t.Tags, ", "))
	}
	if t.DueDate != "" {
		fmt.Printf("Due:         %s\n", t.DueDate)
	}
	if t.Description != "" {
		fmt.Println("Description:")
		for _, line := range strings.Split(t.Description, "\n") {
			fmt.Printf("  %s\n", line)
		}
	}
	return nil
}

// cmdEdit handles the 'edit' command.
func cmdEdit(root string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: edit <id> [--title <t>] [--priority <n>] [--tags <t1,t2>] [--due <date>] [--description <text>] [--status <open|closed>]")
	}

	t, err := findTaskByID(root, args[0])
	if err != nil {
		return err
	}

	hasChanges := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--title":
			if i+1 >= len(args) {
				return fmt.Errorf("--title requires a value")
			}
			t.Title = args[i+1]
			hasChanges = true
			i++
		case "--priority":
			if i+1 >= len(args) {
				return fmt.Errorf("--priority requires a value")
			}
			p, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid priority: %q", args[i+1])
			}
			if p < 0 {
				return fmt.Errorf("priority must be non-negative")
			}
			t.Priority = p
			hasChanges = true
			i++
		case "--tags":
			if i+1 >= len(args) {
				return fmt.Errorf("--tags requires a value")
			}
			var tags []string
			for _, tag := range strings.Split(args[i+1], ",") {
				tr := strings.TrimSpace(tag)
				if tr != "" {
					tags = append(tags, tr)
				}
			}
			t.Tags = tags
			hasChanges = true
			i++
		case "--due":
			if i+1 >= len(args) {
				return fmt.Errorf("--due requires a value")
			}
			dueDate := args[i+1]
			if utils.ParseDueDateTime(dueDate) == nil {
				parsed, _, err := utils.ParseRelativeDateTime(dueDate)
				if err != nil {
					return fmt.Errorf("invalid due date: %q", dueDate)
				}
				dueDate = parsed
			}
			t.DueDate = dueDate
			t.DueDateTime = utils.ParseDueDateTime(dueDate)
			hasChanges = true
			i++
		case "--description":
			if i+1 >= len(args) {
				return fmt.Errorf("--description requires a value")
			}
			t.Description = args[i+1]
			hasChanges = true
			i++
		case "--status":
			if i+1 >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			t.Status = strings.ToUpper(args[i+1])
			hasChanges = true
			i++
		default:
			return fmt.Errorf("unknown flag: %q", args[i])
		}
	}

	if !hasChanges {
		return fmt.Errorf("no changes provided; use --title, --priority, --tags, --due, --description, or --status")
	}

	if err := task.Serialize(t, filepath.Join(root, t.ID, "TASK.md")); err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	fmt.Printf("Updated task %s\n", t.ID)
	return nil
}

// cmdDelete handles the 'delete' command.
func cmdDelete(root string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: delete <id>")
	}

	if err := db.DeleteTask(root, args[0]); err != nil {
		return err
	}
	fmt.Printf("Deleted task %s\n", args[0])
	return nil
}

// cmdClose handles the 'close' command.
func cmdClose(root string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: close <id>")
	}

	t, err := findTaskByID(root, args[0])
	if err != nil {
		return err
	}
	if t.Status == "CLOSED" {
		fmt.Printf("Task %s is already closed.\n", t.ID)
		return nil
	}

	_, err = db.ToggleStatus(root, t)
	if err != nil {
		return err
	}
	fmt.Printf("Closed task %s\n", t.ID)
	return nil
}

// cmdOpen handles the 'open' command.
func cmdOpen(root string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: open <id>")
	}

	t, err := findTaskByID(root, args[0])
	if err != nil {
		return err
	}
	if t.Status == "OPEN" {
		fmt.Printf("Task %s is already open.\n", t.ID)
		return nil
	}

	_, err = db.ToggleStatus(root, t)
	if err != nil {
		return err
	}
	fmt.Printf("Opened task %s\n", t.ID)
	return nil
}

// cmdTags handles the 'tags' command.
func cmdTags(root string, args []string) error {
	statusFilter := "OPEN"
	sortBy := "count"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			statusFilter = args[i+1]
			i++
		case "--sort":
			if i+1 >= len(args) {
				return fmt.Errorf("--sort requires a value")
			}
			sortBy = args[i+1]
			if sortBy != "name" && sortBy != "count" {
				return fmt.Errorf("sort must be 'name' or 'count', got %q", args[i+1])
			}
			i++
		default:
			return fmt.Errorf("unknown flag: %q", args[i])
		}
	}

	tasks, err := loadAllTasks(root)
	if err != nil {
		return err
	}

	statuses := parseStatusFilter(statusFilter)
	tagCounts := make(map[string]int)
	for _, t := range tasks {
		if !matchesStatus(t, statuses) {
			continue
		}
		for _, tag := range t.Tags {
			tagCounts[tag]++
		}
	}

	if len(tagCounts) == 0 {
		fmt.Println("No tags found.")
		return nil
	}

	var sortedTags []string
	if sortBy == "count" {
		sortedTags = utils.SortedKeysByValue(tagCounts)
	} else {
		sortedTags = make([]string, 0, len(tagCounts))
		for k := range tagCounts {
			sortedTags = append(sortedTags, k)
		}
		sort.Strings(sortedTags)
	}

	maxTagLen := 0
	for _, tag := range sortedTags {
		if len(tag) > maxTagLen {
			maxTagLen = len(tag)
		}
	}

	for _, tag := range sortedTags {
		fmt.Printf("  %-*s  %d\n", maxTagLen, tag, tagCounts[tag])
	}
	return nil
}

// cmdTop handles the 'top' command.
func cmdTop(root string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: top <n> [--status <open|closed>]")
	}

	n, err := strconv.Atoi(args[0])
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid count: %q (must be a positive integer)", args[0])
	}

	statusFilter := "OPEN"
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			statusFilter = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag: %q", args[i])
		}
	}

	tasks, err := loadAllTasks(root)
	if err != nil {
		return err
	}

	// Filter and sort by priority descending
	statuses := parseStatusFilter(statusFilter)
	var filtered []*task.Task
	for _, t := range tasks {
		if !matchesStatus(t, statuses) {
			continue
		}
		filtered = append(filtered, t)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Priority != filtered[j].Priority {
			return filtered[i].Priority > filtered[j].Priority
		}
		return filtered[i].ID < filtered[j].ID
	})

	if n > len(filtered) {
		n = len(filtered)
	}

	if n == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	fmt.Printf("Top %d tasks by priority:\n", n)
	for i := 0; i < n; i++ {
		fmt.Printf("  %s\n", formatTaskShort(filtered[i]))
	}
	return nil
}

// cmdNext handles the 'next' command.
func cmdNext(root string, args []string) error {
	statusFilter := "OPEN"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			statusFilter = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag: %q", args[i])
		}
	}

	tasks, err := loadAllTasks(root)
	if err != nil {
		return err
	}

	// Only include tasks with a due date and matching status
	statuses := parseStatusFilter(statusFilter)
	var withDue []*task.Task
	for _, t := range tasks {
		if !matchesStatus(t, statuses) {
			continue
		}
		if t.DueDateTime == nil {
			continue
		}
		withDue = append(withDue, t)
	}

	if len(withDue) == 0 {
		fmt.Println("No upcoming tasks with due dates.")
		return nil
	}

	sort.Slice(withDue, func(i, j int) bool {
		return withDue[i].DueDateTime.Before(*withDue[j].DueDateTime)
	})

	fmt.Println("Next task due:")
	fmt.Printf("  %s\n", formatTaskShort(withDue[0]))
	return nil
}

// cmdOverdue handles the 'overdue' command.
func cmdOverdue(root string, cfg *config.TigoConfig, args []string) error {
	statusFilter := "OPEN"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			statusFilter = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag: %q", args[i])
		}
	}

	tasks, err := loadAllTasks(root)
	if err != nil {
		return err
	}

	statuses := parseStatusFilter(statusFilter)
	now := time.Now()
	var overdue []*task.Task
	for _, t := range tasks {
		if !matchesStatus(t, statuses) {
			continue
		}
		if t.DueDateTime == nil {
			continue
		}
		if t.DueDateTime.Before(now) {
			overdue = append(overdue, t)
		}
	}

	sort.Slice(overdue, func(i, j int) bool {
		return overdue[i].DueDateTime.Before(*overdue[j].DueDateTime)
	})

	if len(overdue) == 0 {
		fmt.Println("No overdue tasks.")
		return nil
	}

	fmt.Printf("%d overdue task(s):\n", len(overdue))
	for _, t := range overdue {
		fmt.Printf("  %s\n", formatTaskShort(t))
	}
	return nil
}

// cmdSearch handles the 'search' command.
func cmdSearch(root string, cfg *config.TigoConfig, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: search <query> [--status <open|closed>] [--limit <n>]")
	}

	query := args[0]
	statusFilter := "OPEN"
	limit := -1

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			statusFilter = args[i+1]
			i++
		case "--limit":
			if i+1 >= len(args) {
				return fmt.Errorf("--limit requires a value")
			}
			l, err := strconv.Atoi(args[i+1])
			if err != nil || l <= 0 {
				return fmt.Errorf("invalid limit: %q", args[i+1])
			}
			limit = l
			i++
		default:
			return fmt.Errorf("unknown flag: %q", args[i])
		}
	}

	tasks, err := loadAllTasks(root)
	if err != nil {
		return err
	}

	statuses := parseStatusFilter(statusFilter)
	queryLower := strings.ToLower(query)
	var results []*task.Task
	for _, t := range tasks {
		if !matchesStatus(t, statuses) {
			continue
		}
		if strings.Contains(strings.ToLower(t.Title), queryLower) ||
			strings.Contains(strings.ToLower(t.Description), queryLower) ||
			strings.Contains(strings.ToLower(strings.Join(t.Tags, " ")), queryLower) {
			results = append(results, t)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	if len(results) == 0 {
		fmt.Println("No matching tasks found.")
		return nil
	}
	for _, t := range results {
		fmt.Println(formatTaskShort(t))
	}
	return nil
}

// cmdStats handles the 'stats' command.
func cmdStats(root string, args []string) error {
	tasks, err := loadAllTasks(root)
	if err != nil {
		return err
	}

	total := len(tasks)
	openCount := 0
	closedCount := 0
	overdueCount := 0
	now := time.Now()

	for _, t := range tasks {
		switch t.Status {
		case "OPEN":
			openCount++
		case "CLOSED":
			closedCount++
		}
		if t.DueDateTime != nil && t.DueDateTime.Before(now) {
			overdueCount++
		}
	}

	fmt.Printf("Total tasks:  %d\n", total)
	fmt.Printf("Open:         %d\n", openCount)
	fmt.Printf("Closed:       %d\n", closedCount)
	fmt.Printf("Overdue:      %d\n", overdueCount)
	return nil
}
