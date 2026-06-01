package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAndSerialize(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "TASK.md")

	initialContent := `# Example Task

- STATUS: OPEN
- PRIORITY: 75
- TAGS: feature, ui
- DUE: 2026-05-11

This is an example description.
With multiple lines.`

	err := os.WriteFile(filePath, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write initial file: %v", err)
	}

	task, err := Parse("123", filePath)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if task.Title != "Example Task" {
		t.Errorf("Expected title 'Example Task', got '%s'", task.Title)
	}
	if task.Status != "OPEN" {
		t.Errorf("Expected status 'OPEN', got '%s'", task.Status)
	}
	if task.Priority != 75 {
		t.Errorf("Expected priority 75, got %d", task.Priority)
	}
	if len(task.Tags) != 2 || task.Tags[0] != "feature" || task.Tags[1] != "ui" {
		t.Errorf("Expected tags [feature, ui], got %v", task.Tags)
	}
	if task.DueDate != "2026-05-11" {
		t.Errorf("Expected due date '2026-05-11', got '%s'", task.DueDate)
	}
	if task.Description != "This is an example description.\nWith multiple lines." {
		t.Errorf("Unexpected description: %s", task.Description)
	}

	// Modify and serialize
	task.Title = "Updated Task"
	task.Status = "WIP"
	task.Priority = 90
	task.Tags = []string{"backend"}
	task.DueDate = "2026-06-01"
	task.Description = "New description."

	err = Serialize(task, filePath)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Read back
	task2, err := Parse("123", filePath)
	if err != nil {
		t.Fatalf("Failed to parse after serialize: %v", err)
	}

	if task2.Title != "Updated Task" {
		t.Errorf("Expected title 'Updated Task', got '%s'", task2.Title)
	}
	if task2.Status != "WIP" {
		t.Errorf("Expected status 'WIP', got '%s'", task2.Status)
	}
	if task2.DueDate != "2026-06-01" {
		t.Errorf("Expected due date '2026-06-01', got '%s'", task2.DueDate)
	}
}
