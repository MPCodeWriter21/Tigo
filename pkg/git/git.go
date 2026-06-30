// Package git provides utility functions for interacting with git repositories, such as checking for changes, committing updates, and blaming lines in task files.
package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/MPCodeWriter21/Tigo/pkg/logs"
)

// HasGit returns true if git is installed on the machine
func HasGit() (bool, error) {
	_, err := exec.LookPath("git")
	if err != nil {
		return false, err
	}
	return true, nil
}

// RunGitCommand runs a git command in the provided directory and returns the output.
func RunGitCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	logs.Append(logs.LevelGit, "git %s", strings.Join(args, " "))
	output, err := cmd.CombinedOutput()
	return strings.TrimRight(string(output), " \t\r\n\v\f"), err
}

// RunGitCommandQuiet is like RunGitCommand but does not log the command.
// Used for probe commands (status checks, repo detection) to avoid log noise.
func RunGitCommandQuiet(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return strings.TrimRight(string(output), " \t\r\n\v\f"), err
}

// IsGitRepo checks if the given directory is within a git repository.
func IsGitRepo(dir string) bool {
	_, err := RunGitCommandQuiet(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// CommitAll stages all changes in rootDir and commits
func CommitAll(tigoDir, message string) (string, error) {
	return CommitFiles(tigoDir, message, nil)
}

// CommitFiles stages and commits the specified files in the tigo directory.
// If files is nil, all changes in the directory are staged.
func CommitFiles(tigoDir, message string, files []string) (string, error) {
	// First check what files are staged to be committed
	// if there are staged files outside the tasks directory, note them and unstage them
	// After committing the tasks changes, restage the previously staged files
	out, err := RunGitCommandQuiet(tigoDir, "diff", "--cached", "--name-only", "--", ":!.")
	if err != nil {
		return "", err
	}
	var restageFiles []string
	if len(out) > 0 {
		restageFiles = strings.Split(out, "\n")
		_, err = RunGitCommand(tigoDir, "reset", "HEAD", "--", ":!.")
		if err != nil {
			return "", err
		}
	}

	if files == nil {
		_, err = RunGitCommand(tigoDir, "add", ".")
	} else {
		args := append([]string{"add", "--"}, files...)
		// This command must be run in the root directory of the git repo to work, otherwise it will not find the files to stage
		var rootDir string
		rootDir, err = RunGitCommandQuiet(tigoDir, "rev-parse", "--show-toplevel")
		if err != nil {
			return "", err
		}
		_, err = RunGitCommand(rootDir, args...)
	}
	if err != nil {
		return "", err
	}

	out, err = RunGitCommandQuiet(tigoDir, "commit", "--no-verify", "-m", message)
	if err == nil {
		logs.Append(logs.LevelGit, "Committed all changes: \x1b[32m%q\x1b[39m", message)
	}

	// Restage previously staged files outside tasks directory
	if len(restageFiles) > 0 {
		// This add command must be run in the root directory of the git repo to work, otherwise it will not find the files to restage
		rootDir, err := RunGitCommandQuiet(tigoDir, "rev-parse", "--show-toplevel")
		if err != nil {
			return "", err
		}
		args := append([]string{"add", "--"}, restageFiles...)
		_, err = RunGitCommand(rootDir, args...)
		if err != nil {
			return "", err
		}
	}

	return out, err
}

// Log returns basic git log formats
func Log(dir string, maxCount int) (string, error) {
	return RunGitCommand(dir, "log", "--oneline", "-n", "10")
}

// TaskIsChanged checks whether the changes to a specific task were already committed or not
// Returns true if task has uncommitted changes (Checks the whole task directory)
func TaskIsChanged(rootDir, taskID string) (bool, error) {
	out, err := RunGitCommandQuiet(rootDir, "status", "--porcelain", taskID)
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

// IsDirty checks whether the current tasks directory has any uncommitted changes.
// Returns true if there are any modified, added, deleted, or untracked files.
func IsDirty(rootDir string) (bool, error) {
	out, err := RunGitCommandQuiet(rootDir, "status", "--porcelain", ".")
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

// HasNonTaskChanges checks whether the git repo containing the tasks has staged changes other than changes to the tasks
// This is used for warning user before committing changes that may not be meant to be committed with Tigo
// Returns true if there are staged changes outside tasks root directory
func HasNonTaskChanges(rootDir string) (bool, error) {
	out, err := RunGitCommandQuiet(rootDir, "diff", "--cached", "--name-only", "--", ":!.")
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

// BlameTask returns the time of last change and who's to blame for the change for each line of a `TASK.md`
func BlameTask(rootDir, taskID string) ([]time.Time, []string, error) {
	taskFile := taskID + "/TASK.md"

	// Check if the file is tracked by git; blame fails with exit 128 for untracked files
	out, err := RunGitCommandQuiet(rootDir, "ls-files", taskFile)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil, nil, fmt.Errorf("task %s is not tracked by git yet", taskID)
	}

	out, err = RunGitCommand(rootDir, "blame", "--line-porcelain", taskFile)
	if err != nil {
		return nil, nil, err
	}

	var times []time.Time
	var names []string

	lines := strings.Split(out, "\n")
	var currentAuthor string
	var currentTime int64

	for _, line := range lines {
		if currentAuthorAfter, ok := strings.CutPrefix(line, "author "); ok {
			currentAuthor = currentAuthorAfter
		} else if tStr, ok := strings.CutPrefix(line, "author-time "); ok {
			t, err := strconv.ParseInt(tStr, 10, 64)
			if err == nil {
				currentTime = t
			}
		} else if strings.HasPrefix(line, "\t") {
			times = append(times, time.Unix(currentTime, 0))
			names = append(names, currentAuthor)
		}
	}

	return times, names, nil
}

// AheadBehind returns the number of commits ahead of and behind the upstream
// tracking branch. Returns 0, 0 if there is no upstream or on error.
func AheadBehind(rootDir string) (ahead, behind int) {
	out, err := RunGitCommandQuiet(rootDir, "rev-parse", "--abbrev-ref", "@{upstream}")
	if err != nil || strings.TrimSpace(out) == "" {
		return 0, 0
	}

	out, err = RunGitCommandQuiet(rootDir, "rev-list", "--count", "--left-right", "@{upstream}...HEAD")
	if err != nil {
		return 0, 0
	}

	parts := strings.Split(strings.TrimSpace(out), "\t")
	if len(parts) == 2 {
		behind, _ = strconv.Atoi(parts[0])
		ahead, _ = strconv.Atoi(parts[1])
	}
	return ahead, behind
}
