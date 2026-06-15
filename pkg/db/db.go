// Package db provides functions to manage task storage on the filesystem.
package db

import (
	"fmt"
	"os"
	"path/filepath"
	"tigo/pkg/git"
	"tigo/pkg/logs"
	"tigo/pkg/task"
	"time"
)

func ResolveDefaultRoot() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".local", "share", "tigo"), nil
}

// ResolveRoot figures out the task root directory.
// Checks if `./.tigo` exists. If not, uses `$HOME/.local/share/tigo`.
func ResolveRoot() string {
	cwd, err := os.Getwd()
	if err == nil {
		localTigo := filepath.Join(cwd, ".tigo")
		if info, err := os.Stat(localTigo); err == nil && info.IsDir() {
			return localTigo
		}
	}

	defaultRoot, err := ResolveDefaultRoot()
	if err != nil {
		// Fallback if no home dir
		return filepath.Join(cwd, ".tigo")
	}

	return defaultRoot
}

// Init ensures the root directory exists.
func Init(root string) error {
	err := os.MkdirAll(root, 0755)
	if err != nil {
		return err
	}
	defaultRoot, err := ResolveDefaultRoot()
	if err == nil && root == defaultRoot {
		if hasGit, _ := git.HasGit(); hasGit && !git.IsGitRepo(root) {
			logs.Append(logs.LevelInfo, "Initializing git repository in %s", root)
			git.RunGitCommand(root, "init")
		}
	}
	return nil
}

// GenerateID creates a new taskId formatted as [0-9]{8}-[0-9]{6} based on UTC
func GenerateID() string {
	return time.Now().UTC().Format("20060102-150405")
}

// CreateNewTask creates a directory and boilerplate TASK.md for a new task.
// Returns the generated task ID.
func CreateNewTask(root, title string, priority int, tags []string, dueDate string, description string) (string, error) {
	maxRetries := 50
	for {
		id := GenerateID()
		taskDir := filepath.Join(root, id)

		// check collision
		if _, err := os.Stat(taskDir); err == nil {
			time.Sleep(100 * time.Millisecond)
			maxRetries--
			if maxRetries <= 0 {
				return "", fmt.Errorf("failed to generate unique task ID after multiple attempts")
			}
			continue
		}

		err := os.MkdirAll(taskDir, 0755)
		if err != nil {
			return "", err
		}

		taskMDPath := filepath.Join(taskDir, "TASK.md")

		if err := task.Serialize(&task.Task{
			ID:          id,
			Title:       title,
			Status:      "OPEN",
			Priority:    priority,
			Tags:        tags,
			DueDate:     dueDate,
			Description: description,
		}, taskMDPath); err != nil {
			return "", err
		}

		logs.Append(logs.LevelInfo, "\x1b[32mCreated\x1b[0m task \x1b[34m%s\x1b[0m: %s", id, title)
		return id, nil
	}
}

// DiscoverTasks scans the root directory for all folders formatted as an ID.
// It returns a list of task IDs.
func DiscoverTasks(root string) ([]string, error) {
	var taskIDs []string

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// A brief format check could be added, but checking for TASK.md is safer
			taskMD := filepath.Join(root, entry.Name(), "TASK.md")
			if _, err := os.Stat(taskMD); err == nil {
				taskIDs = append(taskIDs, entry.Name())
			}
		}
	}

	return taskIDs, nil
}

// DeleteTask removes the task directory and all its contents.
func DeleteTask(root, id string) error {
	taskDir := filepath.Join(root, id)
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		return fmt.Errorf("task with ID %s does not exist", id)
	}

	err := os.RemoveAll(taskDir)
	if err == nil {
		logs.Append(logs.LevelInfo, "\x1b[31mDeleted\x1b[0m task \x1b[34m%s\x1b[0m", id)
	}
	return err
}

// ToggleStatus switches between OPEN and CLOSED status.
// If current status is something other than OPEN or CLOSED, it will not change it.
// Returns the new status.
func ToggleStatus(root string, t *task.Task) (string, error) {
	taskMDPath := filepath.Join(root, t.ID, "TASK.md")

	switch t.Status {
	case "OPEN":
		t.Status = "CLOSED"
	case "CLOSED":
		t.Status = "OPEN"
	default:
		return t.Status, fmt.Errorf("unrecognized status: %s", t.Status)
	}

	// Update the raw lines to reflect the new status
	err := task.Serialize(t, taskMDPath)
	if err == nil {
		var statusText string
		switch t.Status {
		case "OPEN":
			statusText = "\x1b[33mOPEN\x1b[0m"
		case "CLOSED":
			statusText = "\x1b[32mCLOSED\x1b[0m"
		default:
			statusText = fmt.Sprintf("\x1b[35m%s\x1b[0m", t.Status)
		}
		logs.Append(logs.LevelInfo, "Changed task \x1b[34m%s\x1b[0m status to %s", t.ID, statusText)
	}
	return t.Status, err
}
