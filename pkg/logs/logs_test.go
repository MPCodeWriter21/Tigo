package logs

import (
	"testing"
)

func TestAppendAndEntries(t *testing.T) {
	Clear()

	Append(LevelInfo, "test message")
	entries := Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Level != LevelInfo {
		t.Errorf("expected LevelInfo, got %v", entries[0].Level)
	}
	if entries[0].Message != "test message" {
		t.Errorf("expected 'test message', got '%s'", entries[0].Message)
	}
}

func TestAppendFormatted(t *testing.T) {
	Clear()

	Append(LevelGit, "task %s created", "20260601-123456")
	entries := Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Message != "task 20260601-123456 created" {
		t.Errorf("unexpected message: %s", entries[0].Message)
	}
}

func TestClear(t *testing.T) {
	Clear()

	Append(LevelWarn, "something")
	if len(Entries()) != 1 {
		t.Fatal("expected 1 entry before clear")
	}
	Clear()
	if len(Entries()) != 0 {
		t.Fatal("expected 0 entries after clear")
	}
}

func TestMaxSize(t *testing.T) {
	Clear()

	maxSize = 10
	for i := range 20 {
		Append(LevelInfo, "msg %d", i)
	}
	entries := Entries()
	if len(entries) != 10 {
		t.Fatalf("expected 10 entries, got %d", len(entries))
	}
	if entries[0].Message != "msg 10" {
		t.Errorf("expected 'msg 10', got '%s'", entries[0].Message)
	}
	if entries[9].Message != "msg 19" {
		t.Errorf("expected 'msg 19', got '%s'", entries[9].Message)
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelInfo, "INF"},
		{LevelWarn, "WRN"},
		{LevelError, "ERR"},
		{LevelGit, "GIT"},
		{Level(99), "???"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q; want %q", tt.level, got, tt.want)
		}
	}
}
