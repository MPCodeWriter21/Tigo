package task

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Task struct {
	ID          string
	Title       string
	Status      string
	Priority    int
	Tags        []string
	Description string
	RawLines    []string // Used to preserve formatting when serializing
}

var (
	titleRegex    = regexp.MustCompile(`^#\s+(.*)$`)
	statusRegex   = regexp.MustCompile(`^- STATUS:\s*(.*)$`)
	priorityRegex = regexp.MustCompile(`^- PRIORITY:\s*(\d+)$`)
	tagsRegex     = regexp.MustCompile(`^- TAGS:\s*(.*)$`)
)

// Parse reads a TASK.md file and extracts data into a Task object.
func Parse(id, filePath string) (*Task, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	t := &Task{
		ID:       id,
		Status:   "OPEN",
		Priority: 50,
	}

	scanner := bufio.NewScanner(file)
	var descriptionBuilder strings.Builder
	metadataDone := false

	// Basic state machine: title -> metadata -> description
	for scanner.Scan() {
		line := scanner.Text()
		t.RawLines = append(t.RawLines, line)

		if titleRegex.MatchString(line) && t.Title == "" {
			t.Title = strings.TrimSpace(titleRegex.FindStringSubmatch(line)[1])
			continue
		}

		if statusRegex.MatchString(line) {
			t.Status = strings.TrimSpace(statusRegex.FindStringSubmatch(line)[1])
			continue
		}

		if priorityRegex.MatchString(line) {
			p, _ := strconv.Atoi(priorityRegex.FindStringSubmatch(line)[1])
			t.Priority = p
			continue
		}

		if tagsRegex.MatchString(line) {
			tagsStr := tagsRegex.FindStringSubmatch(line)[1]
			parts := strings.Split(tagsStr, ",")
			for _, p := range parts {
				tag := strings.TrimSpace(p)
				if tag != "" {
					t.Tags = append(t.Tags, tag)
				}
			}
			continue
		}

		// Skip empty lines between metadata
		if !metadataDone && strings.TrimSpace(line) == "" {
			continue
		}

		// Assume everything else is description
		metadataDone = true
		descriptionBuilder.WriteString(line + "\n")
	}

	t.Description = strings.TrimSpace(descriptionBuilder.String())

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return t, nil
}

// Serialize writes the Task back into the file, maintaining original structure where possible
func Serialize(filePath string, t *Task) error {
	var newLines []string

	// Create from scratch if no raw lines (or replace completely for simplicity)
	newLines = append(newLines, fmt.Sprintf("# %s", t.Title))
	newLines = append(newLines, "")
	newLines = append(newLines, fmt.Sprintf("- STATUS: %s", t.Status))
	newLines = append(newLines, fmt.Sprintf("- PRIORITY: %d", t.Priority))

	if len(t.Tags) > 0 {
		newLines = append(newLines, fmt.Sprintf("- TAGS: %s", strings.Join(t.Tags, ", ")))
	}

	newLines = append(newLines, "")
	newLines = append(newLines, t.Description)
	newLines = append(newLines, "")

	return os.WriteFile(filePath, []byte(strings.Join(newLines, "\n")), 0644)
}
