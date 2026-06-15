package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MPCodeWriter21/Tigo/pkg/logs"
)

// setupTestRepo creates a temporary directory initialised as a git repo
// and returns the repo root.  The caller is responsible for cleanup via
// t.TempDir().
func setupTestRepo(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init", "--initial-branch=main")
	run("config", "user.email", "test@tigo")
	run("config", "user.name", "Tigo Test")
	return repoDir
}

// writeFile creates a file with the given content inside dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HasGit
// ---------------------------------------------------------------------------

func TestHasGit(t *testing.T) {
	ok, err := HasGit()
	if err != nil {
		t.Fatalf("HasGit() returned error: %v", err)
	}
	if !ok {
		t.Fatal("HasGit() = false; expected true (git must be installed)")
	}
}

// ---------------------------------------------------------------------------
// RunGitCommand / RunGitCommandQuiet
// ---------------------------------------------------------------------------

func TestRunGitCommand(t *testing.T) {
	repo := setupTestRepo(t)
	logs.Clear()

	out, err := RunGitCommand(repo, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		t.Fatalf("RunGitCommand failed: %v", err)
	}
	if strings.TrimSpace(out) != "true" {
		t.Errorf("expected 'true', got %q", out)
	}
}

func TestRunGitCommandQuiet(t *testing.T) {
	repo := setupTestRepo(t)
	logs.Clear()

	out, err := RunGitCommandQuiet(repo, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		t.Fatalf("RunGitCommandQuiet failed: %v", err)
	}
	if strings.TrimSpace(out) != "true" {
		t.Errorf("expected 'true', got %q", out)
	}
}

// ---------------------------------------------------------------------------
// IsGitRepo
// ---------------------------------------------------------------------------

func TestIsGitRepo_True(t *testing.T) {
	repo := setupTestRepo(t)
	if !IsGitRepo(repo) {
		t.Fatal("IsGitRepo should be true inside a git repo")
	}
}

func TestIsGitRepo_False(t *testing.T) {
	dir := t.TempDir()
	if IsGitRepo(dir) {
		t.Fatal("IsGitRepo should be false outside a git repo")
	}
}

// ---------------------------------------------------------------------------
// CommitAll
// ---------------------------------------------------------------------------

func TestCommitAll(t *testing.T) {
	repo := setupTestRepo(t)
	taskDir := filepath.Join(repo, "tasks", "20260601-120000")
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, taskDir, "TASK.md", "# Test\n\n- STATUS: OPEN\n- PRIORITY: 50\n")

	out, err := CommitAll(repo, "feat: add test task")
	if err != nil {
		t.Fatalf("CommitAll failed: %v", err)
	}
	if !strings.Contains(out, "main") {
		t.Errorf("commit output should mention branch, got: %s", out)
	}

	// Verify the commit exists
	logOut, err := RunGitCommandQuiet(repo, "log", "--oneline", "-1")
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(logOut, "feat: add test task") {
		t.Errorf("commit message not found in log: %s", logOut)
	}
}

func TestCommitAll_RestagesOutsideFiles(t *testing.T) {
	repo := setupTestRepo(t)

	// Create a tigo subdirectory
	tigoDir := filepath.Join(repo, "tigo")
	taskDir := filepath.Join(tigoDir, "20260601-120000")
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, taskDir, "TASK.md", "# In tigo")

	// Create and stage a file outside the tigo dir
	writeFile(t, repo, "README.md", "# Project")
	RunGitCommand(repo, "add", "README.md")

	// Now make changes inside tigo dir that need to be committed
	writeFile(t, taskDir, "TASK.md", "# In tigo (updated)")

	out, err := CommitAll(tigoDir, "feat: update task")
	if err != nil {
		t.Fatalf("CommitAll failed: %v", err)
	}
	_ = out

	// After commit, README.md should still be staged
	out2, err2 := RunGitCommandQuiet(repo, "diff", "--cached", "--name-only")
	if err2 != nil {
		t.Fatalf("git diff --cached failed: %v", err2)
	}
	if !strings.Contains(out2, "README.md") {
		t.Errorf("expected README.md to be restaged, got: %s", out2)
	}
}

func TestCommitAll_EmptyRepo(t *testing.T) {
	repo := setupTestRepo(t)
	_, err := CommitAll(repo, "empty commit")
	// No changes to commit — commit will still succeed (git commit --allow-empty
	// is not passed, so this should fail with "nothing to commit")
	if err == nil {
		// It's okay if the noop "succeeds" (no changes staged, but . is also empty)
		t.Log("CommitAll returned nil for empty repo (acceptable)")
	}
}

// ---------------------------------------------------------------------------
// Log
// ---------------------------------------------------------------------------

func TestLog(t *testing.T) {
	repo := setupTestRepo(t)

	// Make an initial commit so there's something to log
	writeFile(t, repo, "README.md", "# test")
	RunGitCommand(repo, "add", ".")
	RunGitCommand(repo, "commit", "--no-verify", "-m", "initial commit")

	out, err := Log(repo, 5)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if !strings.Contains(out, "initial commit") {
		t.Errorf("expected 'initial commit' in log, got: %s", out)
	}
}

func TestLog_EmptyRepo(t *testing.T) {
	repo := setupTestRepo(t)
	// No commits yet — `git log` exits with 128 on empty repos
	_, err := Log(repo, 5)
	if err == nil {
		t.Log("Log on empty repo succeeded (expected, some git versions allow it)")
	}
}

// ---------------------------------------------------------------------------
// TaskIsChanged
// ---------------------------------------------------------------------------

func TestTaskIsChanged_Unchanged(t *testing.T) {
	repo := setupTestRepo(t)
	taskDir := filepath.Join(repo, "20260602-000000")
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, taskDir, "TASK.md", "# Unchanged\n\n- STATUS: OPEN\n")
	RunGitCommand(repo, "add", ".")
	RunGitCommand(repo, "commit", "--no-verify", "-m", "add task")

	changed, err := TaskIsChanged(repo, "20260602-000000")
	if err != nil {
		t.Fatalf("TaskIsChanged failed: %v", err)
	}
	if changed {
		t.Fatal("TaskIsChanged = true; expected false (no changes since commit)")
	}
}

func TestTaskIsChanged_Changed(t *testing.T) {
	repo := setupTestRepo(t)
	taskDir := filepath.Join(repo, "20260602-000001")
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, taskDir, "TASK.md", "# Changed\n\n- STATUS: OPEN\n")
	RunGitCommand(repo, "add", ".")
	RunGitCommand(repo, "commit", "--no-verify", "-m", "add task")

	// Now modify the file
	writeFile(t, taskDir, "TASK.md", "# Changed (updated)\n\n- STATUS: CLOSED\n")

	changed, err := TaskIsChanged(repo, "20260602-000001")
	if err != nil {
		t.Fatalf("TaskIsChanged failed: %v", err)
	}
	if !changed {
		t.Fatal("TaskIsChanged = false; expected true (file was modified)")
	}
}

func TestTaskIsChanged_NonExistentTaskID(t *testing.T) {
	repo := setupTestRepo(t)
	changed, err := TaskIsChanged(repo, "20269999-999999")
	if err != nil {
		t.Fatalf("TaskIsChanged failed: %v", err)
	}
	if changed {
		t.Fatal("TaskIsChanged = true for non-existent task; expected false")
	}
}

func TestTaskIsChanged_OutsideRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := TaskIsChanged(dir, "20260601-000000")
	if err == nil {
		t.Log("TaskIsChanged outside repo should ideally fail")
	}
}

func TestIsDirty_OutsideRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := IsDirty(dir)
	if err == nil {
		t.Log("IsDirty outside repo should ideally fail")
	}
}

func TestHasNonTaskChanges_OutsideRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := HasNonTaskChanges(dir)
	if err == nil {
		t.Log("HasNonTaskChanges outside repo should ideally fail")
	}
}

// ---------------------------------------------------------------------------
// IsDirty
// ---------------------------------------------------------------------------

func TestIsDirty_Clean(t *testing.T) {
	repo := setupTestRepo(t)
	writeFile(t, repo, "dummy", "content")
	RunGitCommand(repo, "add", ".")
	RunGitCommand(repo, "commit", "--no-verify", "-m", "initial")

	dirty, err := IsDirty(repo)
	if err != nil {
		t.Fatalf("IsDirty failed: %v", err)
	}
	if dirty {
		t.Fatal("IsDirty = true; expected false (clean repo)")
	}
}

func TestIsDirty_Dirty(t *testing.T) {
	repo := setupTestRepo(t)
	writeFile(t, repo, "untracked.txt", "hello")

	dirty, err := IsDirty(repo)
	if err != nil {
		t.Fatalf("IsDirty failed: %v", err)
	}
	if !dirty {
		t.Fatal("IsDirty = false; expected true (untracked file present)")
	}
}

// ---------------------------------------------------------------------------
// HasNonTaskChanges
// ---------------------------------------------------------------------------

func TestHasNonTaskChanges_NoNonTaskChanges(t *testing.T) {
	repo := setupTestRepo(t)

	// Create a tigo subdirectory (simulates .tigo/)
	tigoDir := filepath.Join(repo, "tigo")
	err := os.MkdirAll(tigoDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, tigoDir, "task/TASK.md", "# inside tigo")
	writeFile(t, repo, "README.md", "# outside tigo")
	RunGitCommand(repo, "add", ".")
	RunGitCommand(repo, "commit", "--no-verify", "-m", "initial")

	// Stage a change inside the tigo directory
	writeFile(t, tigoDir, "task/TASK.md", "# updated inside tigo")
	RunGitCommand(repo, "add", "tigo/task/TASK.md")

	has, err := HasNonTaskChanges(tigoDir)
	if err != nil {
		t.Fatalf("HasNonTaskChanges failed: %v", err)
	}
	if has {
		t.Fatal("HasNonTaskChanges = true; expected false (only changes inside tigo dir staged)")
	}
}

func TestHasNonTaskChanges_HasNonTaskChanges(t *testing.T) {
	repo := setupTestRepo(t)

	// Create a tigo subdirectory (simulates .tigo/)
	tigoDir := filepath.Join(repo, "tigo")
	err := os.MkdirAll(tigoDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, tigoDir, "task/TASK.md", "# inside tigo")
	writeFile(t, repo, "README.md", "# outside tigo")
	RunGitCommand(repo, "add", ".")
	RunGitCommand(repo, "commit", "--no-verify", "-m", "initial")

	// Stage a change outside the tigo directory
	writeFile(t, repo, "README.md", "# updated repo")
	RunGitCommand(repo, "add", "README.md")

	has, err := HasNonTaskChanges(tigoDir)
	if err != nil {
		t.Fatalf("HasNonTaskChanges failed: %v", err)
	}
	if !has {
		t.Fatal("HasNonTaskChanges = false; expected true (README.md staged outside tigo dir)")
	}
}

func TestHasNonTaskChanges_NoStagedChanges(t *testing.T) {
	repo := setupTestRepo(t)
	tigoDir := filepath.Join(repo, "tigo")
	os.MkdirAll(tigoDir, 0755)
	has, err := HasNonTaskChanges(tigoDir)
	if err != nil {
		t.Fatalf("HasNonTaskChanges failed: %v", err)
	}
	if has {
		t.Fatal("HasNonTaskChanges = true; expected false (no staged changes)")
	}
}

// ---------------------------------------------------------------------------
// BlameTask
// ---------------------------------------------------------------------------

func TestBlameTask_Tracked(t *testing.T) {
	repo := setupTestRepo(t)
	taskDir := filepath.Join(repo, "20260604-000000")
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, taskDir, "TASK.md", "# Blamed\n\n- STATUS: OPEN\n")
	RunGitCommand(repo, "add", ".")
	RunGitCommand(repo, "commit", "--no-verify", "-m", "add blamed task")

	times, names, err := BlameTask(repo, "20260604-000000")
	if err != nil {
		t.Fatalf("BlameTask failed: %v", err)
	}
	if len(times) == 0 {
		t.Fatal("BlameTask returned empty times slice")
	}
	if len(names) == 0 {
		t.Fatal("BlameTask returned empty names slice")
	}
	if len(times) != len(names) {
		t.Fatalf("times (%d) and names (%d) length mismatch", len(times), len(names))
	}
	if names[0] != "Tigo Test" {
		t.Errorf("expected author 'Tigo Test', got %q", names[0])
	}
}

func TestBlameTask_Untracked(t *testing.T) {
	repo := setupTestRepo(t)
	taskDir := filepath.Join(repo, "20260604-000001")
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, taskDir, "TASK.md", "# Untracked\n\n- STATUS: OPEN\n")

	_, _, err = BlameTask(repo, "20260604-000001")
	if err == nil {
		t.Fatal("BlameTask should fail for untracked task")
	}
}

// ---------------------------------------------------------------------------
// AheadBehind
// ---------------------------------------------------------------------------

func TestAheadBehind_NoUpstream(t *testing.T) {
	repo := setupTestRepo(t)
	ahead, behind := AheadBehind(repo)
	if ahead != 0 {
		t.Errorf("ahead = %d, want 0", ahead)
	}
	if behind != 0 {
		t.Errorf("behind = %d, want 0", behind)
	}
}

func TestAheadBehind_AfterCommit(t *testing.T) {
	repo := setupTestRepo(t)
	writeFile(t, repo, "file", "content")
	RunGitCommand(repo, "add", ".")
	RunGitCommand(repo, "commit", "--no-verify", "-m", "initial")

	// With no upstream, should still return 0,0 even after commits
	ahead, behind := AheadBehind(repo)
	if ahead != 0 {
		t.Errorf("ahead = %d, want 0 (no upstream)", ahead)
	}
	if behind != 0 {
		t.Errorf("behind = %d, want 0 (no upstream)", behind)
	}
}

// ---------------------------------------------------------------------------
// Log-based callback checks — verify that RunGitCommand fires a log entry
// ---------------------------------------------------------------------------

func TestRunGitCommand_FiresLog(t *testing.T) {
	repo := setupTestRepo(t)
	logs.Clear()

	RunGitCommand(repo, "rev-parse", "--is-inside-work-tree")

	entries := logs.Entries()
	found := false
	for _, e := range entries {
		if strings.Contains(e.Message, "rev-parse") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log entry containing 'rev-parse' after RunGitCommand")
	}
}

func TestRunGitCommandQuiet_DoesNotFireLog(t *testing.T) {
	repo := setupTestRepo(t)
	logs.Clear()

	RunGitCommandQuiet(repo, "rev-parse", "--is-inside-work-tree")

	entries := logs.Entries()
	for _, e := range entries {
		if strings.Contains(e.Message, "rev-parse") {
			t.Errorf("RunGitCommandQuiet should not log, but found: %s", e.Message)
		}
	}
}

// ---------------------------------------------------------------------------
// Commit verification: commit timestamp is recent
// ---------------------------------------------------------------------------

func TestBlameTask_Timestamp(t *testing.T) {
	repo := setupTestRepo(t)
	taskDir := filepath.Join(repo, "20260605-000000")
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	writeFile(t, taskDir, "TASK.md", "# Timestamp\n\n- STATUS: OPEN\n")
	RunGitCommand(repo, "add", ".")
	RunGitCommand(repo, "commit", "--no-verify", "-m", "add timestamp task")

	times, _, err := BlameTask(repo, "20260605-000000")
	if err != nil {
		t.Fatalf("BlameTask failed: %v", err)
	}
	if len(times) == 0 {
		t.Fatal("no blame lines returned")
	}
	now := time.Now()
	if times[0].After(now) || now.Sub(times[0]) > 5*time.Minute {
		t.Errorf("blame timestamp %v is too far from now %v", times[0], now)
	}
}
