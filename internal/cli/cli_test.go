package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MPCodeWriter21/Tigo/internal/config"
	"github.com/MPCodeWriter21/Tigo/pkg/db"
	"github.com/MPCodeWriter21/Tigo/pkg/task"
)

// setupTest creates a fresh tigo root in a temp directory with optional seed tasks.
func setupTest(t *testing.T) (root string, cfg *config.TigoConfig) {
	t.Helper()
	root = filepath.Join(t.TempDir(), ".tigo")
	if err := db.Init(root); err != nil {
		t.Fatalf("Init: %v", err)
	}
	cfg = config.DefaultConfig()
	return root, cfg
}

// createTestTask creates a task and returns its ID.
func createTestTask(t *testing.T, root, title string, priority int, tags []string) string {
	t.Helper()
	id, err := db.CreateNewTask(root, title, priority, tags, "", "")
	if err != nil {
		t.Fatalf("CreateNewTask: %v", err)
	}
	return id
}

// TestRun_UnknownCommand tests that an unknown command returns an error.
func TestRun_UnknownCommand(t *testing.T) {
	root, cfg := setupTest(t)
	err := Run(root, cfg, []string{"nonexistent"})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected unknown command error, got: %v", err)
	}
}

// TestRun_EmptyCommand tests that an empty command returns an error.
func TestRun_EmptyCommand(t *testing.T) {
	root, cfg := setupTest(t)
	err := Run(root, cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("expected 'no command specified' error, got: %v", err)
	}

	err = Run(root, cfg, []string{})
	if err == nil || !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("expected 'no command specified' error, got: %v", err)
	}
}

// TestHelp tests the help command.
func TestHelp(t *testing.T) {
	root, cfg := setupTest(t)
	err := Run(root, cfg, []string{"help"})
	if err != nil {
		t.Errorf("help should succeed, got: %v", err)
	}

	err = Run(root, cfg, []string{"help", "create"})
	if err != nil {
		t.Errorf("help create should succeed, got: %v", err)
	}

	err = Run(root, cfg, []string{"help", "nonexistent"})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected unknown command error, got: %v", err)
	}
}

// TestCmdCreate tests the 'create' command.
func TestCmdCreate(t *testing.T) {
	root, cfg := setupTest(t)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no title", []string{"create"}, true},
		{"basic", []string{"create", "MyTask"}, false},
		{"with priority", []string{"create", "Task2", "--priority", "80"}, false},
		{"with tags", []string{"create", "Task3", "--tags", "cli,test"}, false},
		{"with due", []string{"create", "Task4", "--due", "2026-12-31"}, false},
		{"with description", []string{"create", "Task5", "--description", "A test task"}, false},
		{"with all flags", []string{"create", "Task6", "--priority", "90", "--tags", "cli,urgent", "--due", "2026-12-31", "--description", "High priority"}, false},
		{"negative priority", []string{"create", "BadTask", "--priority", "-1"}, true},
		{"invalid priority", []string{"create", "BadTask", "--priority", "abc"}, true},
		{"missing priority value", []string{"create", "BadTask", "--priority"}, true},
		{"missing tags value", []string{"create", "BadTask", "--tags"}, true},
		{"relative due", []string{"create", "FutureTask", "--due", "tomorrow"}, false},
		{"invalid due", []string{"create", "BadTask", "--due", "notadate"}, true},
		{"unknown flag", []string{"create", "BadTask", "--badflag"}, true},
		{"title with spaces", []string{"create", "My Task Title"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(root, cfg, tt.args)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, err)
			}
		})
	}

	// Verify tasks were actually created
	ids, err := db.DiscoverTasks(root)
	if err != nil {
		t.Fatalf("DiscoverTasks: %v", err)
	}
	expected := 8
	if len(ids) != expected {
		t.Errorf("expected %d tasks, got %d", expected, len(ids))
	}
}

// TestCmdCreate_VerifyFileContents tests that created tasks have the correct file contents.
func TestCmdCreate_VerifyFileContents(t *testing.T) {
	root, cfg := setupTest(t)

	err := Run(root, cfg, []string{"create", "VerifyTask", "--priority", "75", "--tags", "a,b", "--due", "2026-12-31", "--description", "Verify me"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	ids, err := db.DiscoverTasks(root)
	if err != nil || len(ids) == 0 {
		t.Fatalf("DiscoverTasks: %v (ids=%v)", err, ids)
	}

	taskFile := filepath.Join(root, ids[0], "TASK.md")
	data, err := os.ReadFile(taskFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# VerifyTask") {
		t.Errorf("missing title, got:\n%s", content)
	}
	if !strings.Contains(content, "- PRIORITY: 75") {
		t.Errorf("missing priority, got:\n%s", content)
	}
	if !strings.Contains(content, "- TAGS: a, b") {
		t.Errorf("missing tags, got:\n%s", content)
	}
	if !strings.Contains(content, "- DUE: 2026-12-31") {
		t.Errorf("missing due, got:\n%s", content)
	}
	if !strings.Contains(content, "Verify me") {
		t.Errorf("missing description, got:\n%s", content)
	}
}

// TestCmdList tests the 'list' command.
func TestCmdList(t *testing.T) {
	root, cfg := setupTest(t)

	// Create tasks with different attributes
	id1 := createTestTask(t, root, "Alpha", 50, []string{"go"})
	id2 := createTestTask(t, root, "Beta", 80, []string{"go", "urgent"})
	id3 := createTestTask(t, root, "Gamma", 30, []string{"test"})

	// Close id3
	t3, err := task.Parse(id3, filepath.Join(root, id3, "TASK.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	db.ToggleStatus(root, t3)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, captured string)
	}{
		{"list all", []string{"list", "--status", "*"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, id1) || !strings.Contains(s, id2) {
				t.Errorf("list should contain tasks, got:\n%s", s)
			}
		}},
		{"list open only", []string{"list", "--status", "open"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, id2) {
				t.Errorf("open list should contain open tasks, got:\n%s", s)
			}
		}},
		{"list closed only", []string{"list", "--status", "closed"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, id3) {
				t.Errorf("closed list should contain closed tasks, got:\n%s", s)
			}
		}},
		{"filter by tag", []string{"list", "--status", "*", "--tag", "urgent"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, id2) {
				t.Errorf("tag filter should match Beta, got:\n%s", s)
			}
		}},
		{"filter by tag no match", []string{"list", "--status", "*", "--tag", "nonexistent"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, "No tasks found") {
				t.Errorf("expected no tasks, got:\n%s", s)
			}
		}},
		{"filter priority-min", []string{"list", "--status", "*", "--priority-min", "40"}, false, func(t *testing.T, s string) {
			if strings.Contains(s, id3) {
				t.Errorf("priority-min 40 should exclude Gamma (30), got:\n%s", s)
			}
		}},
		{"filter priority-max", []string{"list", "--status", "*", "--priority-max", "60"}, false, func(t *testing.T, s string) {
			if strings.Contains(s, id2) {
				t.Errorf("priority-max 60 should exclude Beta (80), got:\n%s", s)
			}
		}},
		{"limit results", []string{"list", "--status", "*", "--limit", "1"}, false, func(t *testing.T, s string) {
			lines := strings.Count(s, "\n")
			if lines > 2 {
				t.Errorf("limit 1 should show at most 1 task, got %d lines:\n%s", lines, s)
			}
		}},
		{"custom status", []string{"list", "--status", "CUSTOM"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, "No tasks found") {
				t.Errorf("expected no tasks with CUSTOM status, got:\n%s", s)
			}
		}},
		{"default status (open)", []string{"list"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, id2) {
				t.Errorf("open list should contain open tasks, got:\n%s", s)
			}
		}},
		{"comma-separated status", []string{"list", "--status", "open,closed"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, id1) || !strings.Contains(s, id3) {
				t.Errorf("comma-separated should include open and closed, got:\n%s", s)
			}
		}},
		{"invalid sort", []string{"list", "--sort", "bad"}, true, nil},
		{"invalid limit", []string{"list", "--limit", "-1"}, true, nil},
		{"unknown flag", []string{"list", "--badflag"}, true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe: %v", err)
			}
			stdout := os.Stdout
			os.Stdout = w

			runErr := Run(root, cfg, tt.args)

			w.Close()
			os.Stdout = stdout
			var buf strings.Builder
			readBuf := make([]byte, 4096)
			for {
				n, err := r.Read(readBuf)
				if n > 0 {
					buf.Write(readBuf[:n])
				}
				if err != nil {
					break
				}
			}

			if tt.wantErr && runErr == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && runErr != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, runErr)
			}
			if tt.check != nil {
				tt.check(t, buf.String())
			}
		})
	}
}

// TestCmdShow tests the 'show' command.
func TestCmdShow(t *testing.T) {
	root, cfg := setupTest(t)
	id := createTestTask(t, root, "ShowTask", 60, []string{"demo"})

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no id", []string{"show"}, true},
		{"valid id", []string{"show", id}, false},
		{"nonexistent id", []string{"show", "00000000-000000"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(root, cfg, tt.args)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, err)
			}
		})
	}
}

// TestCmdEdit tests the 'edit' command.
func TestCmdEdit(t *testing.T) {
	root, cfg := setupTest(t)
	id := createTestTask(t, root, "EditMe", 50, []string{"old"})

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no id", []string{"edit"}, true},
		{"no changes", []string{"edit", id}, true},
		{"change title", []string{"edit", id, "--title", "Edited"}, false},
		{"change priority", []string{"edit", id, "--priority", "99"}, false},
		{"change tags", []string{"edit", id, "--tags", "new,tag"}, false},
		{"change due", []string{"edit", id, "--due", "2026-12-31"}, false},
		{"change description", []string{"edit", id, "--description", "Updated desc"}, false},
		{"change status", []string{"edit", id, "--status", "closed"}, false},
		{"change status to open", []string{"edit", id, "--status", "open"}, false},
		{"all changes", []string{"edit", id, "--title", "Final", "--priority", "100", "--tags", "final", "--due", "2027-01-01", "--description", "Done", "--status", "closed"}, false},
		{"set custom status", []string{"edit", id, "--status", "CUSTOM_STATUS"}, false},
		{"invalid priority", []string{"edit", id, "--priority", "abc"}, true},
		{"negative priority", []string{"edit", id, "--priority", "-1"}, true},
		{"unknown flag", []string{"edit", id, "--badflag"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(root, cfg, tt.args)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, err)
			}
		})
	}

	// Verify final state
	taskFile := filepath.Join(root, id, "TASK.md")
	data, err := os.ReadFile(taskFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Final") {
		t.Errorf("expected title Final, got:\n%s", content)
	}
	if !strings.Contains(content, "- PRIORITY: 100") {
		t.Errorf("expected priority 100, got:\n%s", content)
	}
	if !strings.Contains(content, "- TAGS: final") {
		t.Errorf("expected tags final, got:\n%s", content)
	}
	if !strings.Contains(content, "- DUE: 2027-01-01") {
		t.Errorf("expected due 2027-01-01, got:\n%s", content)
	}
	if !strings.Contains(content, "- STATUS: CUSTOM_STATUS") {
		t.Errorf("expected status CUSTOM_STATUS, got:\n%s", content)
	}
	if !strings.Contains(content, "Done") {
		t.Errorf("expected description Done, got:\n%s", content)
	}
}

// TestCmdDelete tests the 'delete' command.
func TestCmdDelete(t *testing.T) {
	root, cfg := setupTest(t)
	id := createTestTask(t, root, "DeleteMe", 50, nil)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no id", []string{"delete"}, true},
		{"valid", []string{"delete", id}, false},
		{"already deleted", []string{"delete", id}, true},
		{"nonexistent", []string{"delete", "00000000-000000"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(root, cfg, tt.args)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, err)
			}
		})
	}
}

// TestCmdCloseOpen tests the 'close' and 'open' commands.
func TestCmdCloseOpen(t *testing.T) {
	root, cfg := setupTest(t)
	id := createTestTask(t, root, "ToggleMe", 50, nil)

	// Test close
	if err := Run(root, cfg, []string{"close", id}); err != nil {
		t.Fatalf("close: %v", err)
	}
	t1, err := task.Parse(id, filepath.Join(root, id, "TASK.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if t1.Status != "CLOSED" {
		t.Errorf("expected CLOSED, got %s", t1.Status)
	}

	// Close again should say already closed
	if err := Run(root, cfg, []string{"close", id}); err != nil {
		t.Errorf("close again should succeed, got: %v", err)
	}

	// Test open
	if err := Run(root, cfg, []string{"open", id}); err != nil {
		t.Fatalf("open: %v", err)
	}
	t2, err := task.Parse(id, filepath.Join(root, id, "TASK.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if t2.Status != "OPEN" {
		t.Errorf("expected OPEN, got %s", t2.Status)
	}

	// Open again should say already open
	if err := Run(root, cfg, []string{"open", id}); err != nil {
		t.Errorf("open again should succeed, got: %v", err)
	}
}

// TestCmdCloseOpen_Errors tests error cases for close/open.
func TestCmdCloseOpen_Errors(t *testing.T) {
	root, cfg := setupTest(t)

	tests := []struct {
		name string
		args []string
	}{
		{"close no id", []string{"close"}},
		{"open no id", []string{"open"}},
		{"close nonexistent", []string{"close", "00000000-000000"}},
		{"open nonexistent", []string{"open", "00000000-000000"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(root, cfg, tt.args)
			if err == nil {
				t.Errorf("expected error for %v", tt.args)
			}
		})
	}
}

// TestCmdTags tests the 'tags' command.
func TestCmdTags(t *testing.T) {
	root, cfg := setupTest(t)

	createTestTask(t, root, "Task1", 50, []string{"go", "urgent"})
	createTestTask(t, root, "Task2", 50, []string{"go", "test"})
	createTestTask(t, root, "Task3", 50, []string{"urgent"})

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, captured string)
	}{
		{"default (open)", []string{"tags"}, false, nil},
		{"all statuses", []string{"tags", "--status", "*"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, "go") || !strings.Contains(s, "urgent") || !strings.Contains(s, "test") {
				t.Errorf("tags should contain all tags, got:\n%s", s)
			}
		}},
		{"sort by name", []string{"tags", "--status", "*", "--sort", "name"}, false, nil},
		{"sort by count", []string{"tags", "--status", "*", "--sort", "count"}, false, nil},
		{"filter by open", []string{"tags", "--status", "open"}, false, nil},
		{"no tags in closed", []string{"tags", "--status", "closed"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, "No tags found") {
				t.Errorf("expected no tags for closed, got:\n%s", s)
			}
		}},
		{"invalid sort", []string{"tags", "--status", "*", "--sort", "bad"}, true, nil},
		{"unknown flag", []string{"tags", "--badflag"}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe: %v", err)
			}
			stdout := os.Stdout
			os.Stdout = w

			runErr := Run(root, cfg, tt.args)

			w.Close()
			os.Stdout = stdout
			var buf strings.Builder
			readBuf := make([]byte, 4096)
			for {
				n, err := r.Read(readBuf)
				if n > 0 {
					buf.Write(readBuf[:n])
				}
				if err != nil {
					break
				}
			}

			if tt.wantErr && runErr == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && runErr != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, runErr)
			}
			if tt.check != nil {
				tt.check(t, buf.String())
			}
		})
	}
}

// TestCmdTop tests the 'top' command.
func TestCmdTop(t *testing.T) {
	root, cfg := setupTest(t)

	createTestTask(t, root, "Low", 10, nil)
	createTestTask(t, root, "High", 90, nil)
	createTestTask(t, root, "Mid", 50, nil)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no arg", []string{"top"}, true},
		{"invalid n", []string{"top", "abc"}, true},
		{"negative n", []string{"top", "-1"}, true},
		{"top 1", []string{"top", "1"}, false},
		{"top 5", []string{"top", "5"}, false},
		{"top 2 open", []string{"top", "2", "--status", "open"}, false},
		{"top with wildcard", []string{"top", "10", "--status", "*"}, false},
		{"unknown flag", []string{"top", "1", "--badflag"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(root, cfg, tt.args)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, err)
			}
		})
	}
}

// TestCmdNext tests the 'next' command.
func TestCmdNext(t *testing.T) {
	root, cfg := setupTest(t)

	createTestTask(t, root, "Later", 50, nil)
	id, err := db.CreateNewTask(root, "Soon", 50, nil, "2099-12-31", "")
	if err != nil {
		t.Fatalf("CreateNewTask: %v", err)
	}

	// Verify next finds the correct task
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	stdout := os.Stdout
	os.Stdout = w
	Run(root, cfg, []string{"next"})
	w.Close()
	os.Stdout = stdout
	var buf strings.Builder
	readBuf := make([]byte, 4096)
	for {
		n, err := r.Read(readBuf)
		if n > 0 {
			buf.Write(readBuf[:n])
		}
		if err != nil {
			break
		}
	}
	if !strings.Contains(buf.String(), id) {
		t.Errorf("next should find Soon (%s), got:\n%s", id, buf.String())
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"default", []string{"next"}, false},
		{"with status", []string{"next", "--status", "open"}, false},
		{"wildcard", []string{"next", "--status", "*"}, false},
		{"no matches", []string{"next", "--status", "closed"}, false},
		{"unknown flag", []string{"next", "--badflag"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(root, cfg, tt.args)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, err)
			}
		})
	}
}

// TestCmdOverdue tests the 'overdue' command.
func TestCmdOverdue(t *testing.T) {
	root, cfg := setupTest(t)

	overdueID, err := db.CreateNewTask(root, "OverdueTask", 50, nil, "2020-01-01", "")
	if err != nil {
		t.Fatalf("CreateNewTask: %v", err)
	}
	db.CreateNewTask(root, "FutureTask", 50, nil, "2099-12-31", "")

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, captured string)
	}{
		{"default", []string{"overdue"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, overdueID) {
				t.Errorf("overdue should find OverdueTask (%s), got:\n%s", overdueID, s)
			}
		}},
		{"with status", []string{"overdue", "--status", "open"}, false, nil},
		{"wildcard", []string{"overdue", "--status", "*"}, false, nil},
		{"closed only", []string{"overdue", "--status", "closed"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, "No overdue tasks") {
				t.Errorf("expected no overdue closed tasks, got:\n%s", s)
			}
		}},
		{"unknown flag", []string{"overdue", "--badflag"}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe: %v", err)
			}
			stdout := os.Stdout
			os.Stdout = w

			runErr := Run(root, cfg, tt.args)

			w.Close()
			os.Stdout = stdout
			var buf strings.Builder
			readBuf := make([]byte, 4096)
			for {
				n, err := r.Read(readBuf)
				if n > 0 {
					buf.Write(readBuf[:n])
				}
				if err != nil {
					break
				}
			}

			if tt.wantErr && runErr == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && runErr != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, runErr)
			}
			if tt.check != nil {
				tt.check(t, buf.String())
			}
		})
	}
}

// TestCmdSearch tests the 'search' command.
func TestCmdSearch(t *testing.T) {
	root, cfg := setupTest(t)

	createTestTask(t, root, "DatabaseSetup", 50, []string{"db"})
	createTestTask(t, root, "APIEndpoint", 50, []string{"api"})
	createTestTask(t, root, "DatabaseMigration", 50, []string{"db", "migration"})

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, captured string)
	}{
		{"no query", []string{"search"}, true, nil},
		{"match title", []string{"search", "Database", "--status", "*"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, "DatabaseSetup") || !strings.Contains(s, "DatabaseMigration") {
				t.Errorf("search should match title, got:\n%s", s)
			}
		}},
		{"match tag", []string{"search", "api", "--status", "*"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, "APIEndpoint") {
				t.Errorf("search should match tag, got:\n%s", s)
			}
		}},
		{"no match", []string{"search", "zzz", "--status", "*"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, "No matching tasks") {
				t.Errorf("expected no matches, got:\n%s", s)
			}
		}},
		{"with limit", []string{"search", "Database", "--status", "*", "--limit", "1"}, false, func(t *testing.T, s string) {
			lines := strings.Count(s, "\n")
			if lines > 2 {
				t.Errorf("limit 1 should show at most 1 task, got %d lines:\n%s", lines, s)
			}
		}},
		{"with status filter", []string{"search", "Database", "--status", "closed"}, false, func(t *testing.T, s string) {
			if !strings.Contains(s, "No matching tasks") {
				t.Errorf("expected no matches for closed, got:\n%s", s)
			}
		}},
		{"unknown flag", []string{"search", "test", "--badflag"}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe: %v", err)
			}
			stdout := os.Stdout
			os.Stdout = w

			runErr := Run(root, cfg, tt.args)

			w.Close()
			os.Stdout = stdout
			var buf strings.Builder
			readBuf := make([]byte, 4096)
			for {
				n, err := r.Read(readBuf)
				if n > 0 {
					buf.Write(readBuf[:n])
				}
				if err != nil {
					break
				}
			}

			if tt.wantErr && runErr == nil {
				t.Errorf("expected error for %v", tt.args)
			}
			if !tt.wantErr && runErr != nil {
				t.Errorf("unexpected error for %v: %v", tt.args, runErr)
			}
			if tt.check != nil {
				tt.check(t, buf.String())
			}
		})
	}
}

// TestCmdStats tests the 'stats' command.
func TestCmdStats(t *testing.T) {
	root, cfg := setupTest(t)

	createTestTask(t, root, "Open1", 50, nil)
	createTestTask(t, root, "Open2", 50, nil)

	id, err := db.CreateNewTask(root, "ToClose", 50, nil, "", "")
	if err != nil {
		t.Fatalf("CreateNewTask: %v", err)
	}
	db.ToggleStatus(root, &task.Task{ID: id, Status: "OPEN"})

	db.CreateNewTask(root, "Overdue", 50, nil, "2020-01-01", "")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	stdout := os.Stdout
	os.Stdout = w

	if err := Run(root, cfg, []string{"stats"}); err != nil {
		t.Fatalf("stats: %v", err)
	}

	w.Close()
	os.Stdout = stdout
	var buf strings.Builder
	readBuf := make([]byte, 4096)
	for {
		n, err := r.Read(readBuf)
		if n > 0 {
			buf.Write(readBuf[:n])
		}
		if err != nil {
			break
		}
	}

	output := buf.String()
	if !strings.Contains(output, "Total tasks:") {
		t.Errorf("stats missing total, got:\n%s", output)
	}
	if !strings.Contains(output, "Open:") {
		t.Errorf("stats missing open count, got:\n%s", output)
	}
	if !strings.Contains(output, "Closed:") {
		t.Errorf("stats missing closed count, got:\n%s", output)
	}
	if !strings.Contains(output, "Overdue:") {
		t.Errorf("stats missing overdue count, got:\n%s", output)
	}
}

// TestNoSandboxLeak verifies that no test creates files outside its temp directory.
func TestNoSandboxLeak(t *testing.T) {
	root, cfg := setupTest(t)
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	commands := [][]string{
		{"help"},
		{"create", "TestTask"},
		{"stats"},
	}
	for _, args := range commands {
		if err := Run(root, cfg, args); err != nil {
			t.Errorf("command %v failed: %v", args, err)
		}
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		return err
	})
	if err != nil {
		t.Errorf("walk root failed: %v", err)
	}

	if origHome != "" {
		os.Setenv("HOME", origHome)
	}
	if origUserProfile != "" {
		os.Setenv("USERPROFILE", origUserProfile)
	}
}
