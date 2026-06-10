package db

import (
	"slices"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tigo/pkg/logs"
	"tigo/pkg/task"
)

func TestInit(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")

	err := Init(root)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("Stat after Init failed: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("Init did not create a directory")
	}
}

func TestInit_Existing(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")

	os.MkdirAll(root, 0755)
	err := Init(root)
	if err != nil {
		t.Fatalf("Init on existing dir failed: %v", err)
	}
}

func TestGenerateID_Format(t *testing.T) {
	id := GenerateID()
	matched, err := regexp.MatchString(`^[0-9]{8}-[0-9]{6}$`, id)
	if err != nil {
		t.Fatalf("regexp error: %v", err)
	}
	if !matched {
		t.Errorf("GenerateID() = %q, want format YYYYMMDD-HHmmss", id)
	}
}

func TestCreateNewTask(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, err := CreateNewTask(root, "Test Task", 75, []string{"test", "unit"}, "2026-12-31", "A test description.")
	if err != nil {
		t.Fatalf("CreateNewTask failed: %v", err)
	}

	if id == "" {
		t.Fatal("CreateNewTask returned empty ID")
	}

	matched, _ := regexp.MatchString(`^[0-9]{8}-[0-9]{6}$`, id)
	if !matched {
		t.Errorf("CreateNewTask returned invalid ID format: %q", id)
	}

	taskDir := filepath.Join(root, id)
	info, err := os.Stat(taskDir)
	if err != nil {
		t.Fatalf("task directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("task path is not a directory")
	}

	taskFile := filepath.Join(taskDir, "TASK.md")
	if _, err := os.Stat(taskFile); os.IsNotExist(err) {
		t.Fatal("TASK.md not created in task directory")
	}
}

func TestCreateNewTask_FileContent(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, err := CreateNewTask(root, "Content Test", 50, []string{"check"}, "", "Some content.")
	if err != nil {
		t.Fatalf("CreateNewTask failed: %v", err)
	}

	taskFile := filepath.Join(root, id, "TASK.md")
	data, err := os.ReadFile(taskFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# Content Test") {
		t.Errorf("TASK.md missing title, got:\n%s", content)
	}
	if !strings.Contains(content, "- STATUS: OPEN") {
		t.Errorf("TASK.md missing status, got:\n%s", content)
	}
	if !strings.Contains(content, "- PRIORITY: 50") {
		t.Errorf("TASK.md missing priority, got:\n%s", content)
	}
	if !strings.Contains(content, "- TAGS: check") {
		t.Errorf("TASK.md missing tags, got:\n%s", content)
	}
	if !strings.Contains(content, "Some content.") {
		t.Errorf("TASK.md missing description, got:\n%s", content)
	}
}

func TestCreateNewTask_NoOptionalFields(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, err := CreateNewTask(root, "Minimal", 50, nil, "", "")
	if err != nil {
		t.Fatalf("CreateNewTask failed: %v", err)
	}

	taskFile := filepath.Join(root, id, "TASK.md")
	data, err := os.ReadFile(taskFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "- TAGS:") {
		t.Errorf("TASK.md should not contain TAGS for nil tags, got:\n%s", content)
	}
	if strings.Contains(content, "- DUE:") {
		t.Errorf("TASK.md should not contain DUE for empty dueDate, got:\n%s", content)
	}
}

func TestDiscoverTasks(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	ids := make([]string, 3)
	for i := range 3 {
		id, err := CreateNewTask(root, "Task", 50, nil, "", "")
		if err != nil {
			t.Fatalf("CreateNewTask failed: %v", err)
		}
		ids[i] = id
	}

	found, err := DiscoverTasks(root)
	if err != nil {
		t.Fatalf("DiscoverTasks failed: %v", err)
	}

	if len(found) != 3 {
		t.Fatalf("expected 3 tasks, got %d: %v", len(found), found)
	}

	for _, id := range ids {
		present := slices.Contains(found, id)
		if !present {
			t.Errorf("task %q not found in DiscoverTasks result", id)
		}
	}
}

func TestDiscoverTasks_SkipsDirsWithoutTASKMD(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	CreateNewTask(root, "Real Task", 50, nil, "", "")
	os.MkdirAll(filepath.Join(root, "20261234-567890"), 0755)

	found, err := DiscoverTasks(root)
	if err != nil {
		t.Fatalf("DiscoverTasks failed: %v", err)
	}

	if len(found) != 1 {
		t.Errorf("expected 1 task (skip dir without TASK.md), got %d: %v", len(found), found)
	}
}

func TestDiscoverTasks_NonExistentDir(t *testing.T) {
	_, err := DiscoverTasks(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestDeleteTask(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, _ := CreateNewTask(root, "To Delete", 50, nil, "", "")
	taskDir := filepath.Join(root, id)

	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		t.Fatal("task directory should exist before deletion")
	}

	err := DeleteTask(root, id)
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Fatal("task directory should not exist after deletion")
	}
}

func TestDeleteTask_NonExistent(t *testing.T) {
	err := DeleteTask(t.TempDir(), "20260101-000000")
	if err == nil {
		t.Fatal("expected error when deleting non-existent task")
	}
}

func TestToggleStatus_OpenToClosed(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, _ := CreateNewTask(root, "Toggle Test", 50, nil, "", "")
	taskFile := filepath.Join(root, id, "TASK.md")

	parsed, err := task.Parse(id, taskFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.Status != "OPEN" {
		t.Fatalf("expected initial status OPEN, got %s", parsed.Status)
	}

	newStatus, err := ToggleStatus(root, parsed)
	if err != nil {
		t.Fatalf("ToggleStatus failed: %v", err)
	}

	if newStatus != "CLOSED" {
		t.Errorf("expected CLOSED, got %s", newStatus)
	}
	if parsed.Status != "CLOSED" {
		t.Errorf("parsed.Status should be CLOSED, got %s", parsed.Status)
	}

	reparsed, err := task.Parse(id, taskFile)
	if err != nil {
		t.Fatalf("Re-parse failed: %v", err)
	}
	if reparsed.Status != "CLOSED" {
		t.Errorf("on-disk status should be CLOSED, got %s", reparsed.Status)
	}
}

func TestToggleStatus_ClosedToOpen(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, _ := CreateNewTask(root, "Toggle Back", 50, nil, "", "")
	taskFile := filepath.Join(root, id, "TASK.md")

	parsed, _ := task.Parse(id, taskFile)
	parsed.Status = "CLOSED"
	task.Serialize(parsed, taskFile)

	parsed2, _ := task.Parse(id, taskFile)
	if parsed2.Status != "CLOSED" {
		t.Fatalf("expected status CLOSED before toggle, got %s", parsed2.Status)
	}

	newStatus, err := ToggleStatus(root, parsed2)
	if err != nil {
		t.Fatalf("ToggleStatus failed: %v", err)
	}

	if newStatus != "OPEN" {
		t.Errorf("expected OPEN, got %s", newStatus)
	}
}

func TestToggleStatus_UnrecognizedStatus(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, _ := CreateNewTask(root, "Bad Status", 50, nil, "", "")
	taskFile := filepath.Join(root, id, "TASK.md")

	parsed, _ := task.Parse(id, taskFile)
	parsed.Status = "WIP"
	task.Serialize(parsed, taskFile)

	parsed2, _ := task.Parse(id, taskFile)
	_, err := ToggleStatus(root, parsed2)
	if err == nil {
		t.Fatal("expected error for unrecognized status WIP")
	}
}

func TestCreateAndDiscoverRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	titles := []string{"First", "Second", "Third"}
	created := make(map[string]string)

	for _, title := range titles {
		id, err := CreateNewTask(root, title, 50, nil, "", "")
		if err != nil {
			t.Fatalf("CreateNewTask(%q) failed: %v", title, err)
		}
		created[id] = title
	}

	found, err := DiscoverTasks(root)
	if err != nil {
		t.Fatalf("DiscoverTasks failed: %v", err)
	}

	if len(found) != len(titles) {
		t.Fatalf("expected %d tasks, got %d", len(titles), len(found))
	}

	for _, id := range found {
		title, ok := created[id]
		if !ok {
			t.Errorf("unexpected task ID %q in discovery", id)
			continue
		}
		parsed, err := task.Parse(id, filepath.Join(root, id, "TASK.md"))
		if err != nil {
			t.Errorf("Parse(%q) failed: %v", id, err)
			continue
		}
		if parsed.Title != title {
			t.Errorf("task %q: expected title %q, got %q", id, title, parsed.Title)
		}
	}
}

func TestCreateNewTask_FiresLog(t *testing.T) {
	logs.Clear()
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, err := CreateNewTask(root, "Log Test", 50, nil, "", "")
	if err != nil {
		t.Fatalf("CreateNewTask failed: %v", err)
	}

	entries := logs.Entries()
	found := false
	for _, e := range entries {
		if strings.Contains(e.Message, id) && strings.Contains(e.Message, "Log Test") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log entry containing task ID and title after CreateNewTask")
	}
}

func TestDeleteTask_FiresLog(t *testing.T) {
	logs.Clear()
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, _ := CreateNewTask(root, "Log Delete", 50, nil, "", "")
	logs.Clear()

	DeleteTask(root, id)

	entries := logs.Entries()
	found := false
	for _, e := range entries {
		if strings.Contains(e.Message, id) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log entry containing task ID after DeleteTask")
	}
}

func TestToggleStatus_FiresLog(t *testing.T) {
	logs.Clear()
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, ".tigo")
	Init(root)

	id, _ := CreateNewTask(root, "Log Toggle", 50, nil, "", "")
	parsed, _ := task.Parse(id, filepath.Join(root, id, "TASK.md"))
	logs.Clear()

	ToggleStatus(root, parsed)

	entries := logs.Entries()
	found := false
	for _, e := range entries {
		if strings.Contains(e.Message, id) && strings.Contains(e.Message, "CLOSED") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log entry containing task ID and new status after ToggleStatus")
	}
}
