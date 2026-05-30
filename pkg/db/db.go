package db

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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

	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback if no home dir
		return filepath.Join(cwd, ".tigo")
	}

	return filepath.Join(homeDir, ".local", "share", "tigo")
}

// Init ensures the root directory exists.
func Init(root string) error {
	return os.MkdirAll(root, 0755)
}

// GenerateID creates a new taskId formatted as [0-9]{8}-[0-9]{6} based on UTC
func GenerateID() string {
	return time.Now().UTC().Format("20060102-150405")
}

// CreateTaskDirectory creates a directory and boilerplate TASK.md for a new task.
func CreateTaskDirectory(root, title string) (string, error) {
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

		content := fmt.Sprintf("# %s\n\n- STATUS: OPEN\n- PRIORITY: 50\n- TAGS: \n\n", title)
		err = os.WriteFile(taskMDPath, []byte(content), 0644)
		if err != nil {
			return "", err
		}

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
