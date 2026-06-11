package task

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Task struct {
	ID          string
	Title       string
	Status      string
	Priority    int
	Tags        []string
	DueDate     string     // Raw text from the file
	DueDateTime *time.Time // Parsed time value (nil if invalid/unset)
	Description string
	RawLines    []string // Used to preserve formatting when serializing
}

var (
	TitleRegex    = regexp.MustCompile(`^#\s+(.*)$`)
	StatusRegex   = regexp.MustCompile(`^- STATUS:\s*(.*)$`)
	PriorityRegex = regexp.MustCompile(`^- PRIORITY:\s*([0-9]+)$`)
	TagsRegex     = regexp.MustCompile(`^- TAGS:\s*(.*)$`)
	DueDateRegex  = regexp.MustCompile(`^- DUE:\s*(.*)$`)

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

	// Errors
	ErrInvalidTitle = errors.New("invalid title value")
	ErrEmptyTitle   = errors.New("title cannot be empty")
)

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

		if TitleRegex.MatchString(line) && t.Title == "" {
			t.Title = strings.TrimSpace(TitleRegex.FindStringSubmatch(line)[1])
			continue
		}

		if StatusRegex.MatchString(line) {
			t.Status = strings.TrimSpace(StatusRegex.FindStringSubmatch(line)[1])
			continue
		}

		if PriorityRegex.MatchString(line) {
			p, _ := strconv.Atoi(PriorityRegex.FindStringSubmatch(line)[1])
			t.Priority = p
			continue
		}

		if TagsRegex.MatchString(line) {
			tagsStr := TagsRegex.FindStringSubmatch(line)[1]
			parts := strings.SplitSeq(tagsStr, ",")
			for p := range parts {
				tag := strings.TrimSpace(p)
				if tag != "" {
					t.Tags = append(t.Tags, tag)
				}
			}
			continue
		}

		if DueDateRegex.MatchString(line) {
			t.DueDate = strings.TrimSpace(DueDateRegex.FindStringSubmatch(line)[1])
			t.DueDateTime = ParseDueDateTime(t.DueDate)
			continue
		}

		// Skip empty lines between metadata
		if !metadataDone && strings.TrimSpace(line) == "" {
			continue
		}

		// Assume everything else is description
		metadataDone = true
		descriptionBuilder.WriteString(line)
		descriptionBuilder.WriteString("\n")
	}

	t.Description = strings.TrimSpace(descriptionBuilder.String())

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return t, nil
}

// Serialize writes the Task back into the file, maintaining original structure where possible
func Serialize(t *Task, filePath string) error {
	var newLines []string

	if err := Validate(t); err != nil {
		return err
	}

	// TODO: Replace only the lines that need updating (e.g. status, priority) instead
	// of rewriting the whole file.

	// Create from scratch
	newLines = append(newLines, fmt.Sprintf("# %s", t.Title))
	newLines = append(newLines, "")
	newLines = append(newLines, fmt.Sprintf("- STATUS: %s", t.Status))
	newLines = append(newLines, fmt.Sprintf("- PRIORITY: %d", t.Priority))

	if len(t.Tags) > 0 {
		newLines = append(newLines, fmt.Sprintf("- TAGS: %s", strings.Join(t.Tags, ", ")))
	}

	if t.DueDate != "" {
		newLines = append(newLines, fmt.Sprintf("- DUE: %s", t.DueDate))
	}

	if len(t.Description) > 0 {
		newLines = append(newLines, "")
		newLines = append(newLines, t.Description)
		newLines = append(newLines, "")
	}

	return os.WriteFile(filePath, []byte(strings.Join(newLines, "\n")), 0644)
}

// Validate checks if the Task has valid fields (e.g. non-empty title, valid status).
func Validate(t *Task) error {
	if strings.TrimSpace(t.Title) == "" {
		return ErrEmptyTitle
	}
	if strings.ContainsAny(t.Title, "\n\r") {
		return ErrInvalidTitle
	}
	return nil
}
