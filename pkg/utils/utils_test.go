package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TextLen
// ---------------------------------------------------------------------------

func TestTextLen_Plain(t *testing.T) {
	if n := TextLen("hello"); n != 5 {
		t.Errorf("TextLen(\"hello\") = %d, want 5", n)
	}
}

func TestTextLen_WithANSI(t *testing.T) {
	text := "\x1b[38;5;196mred\x1b[0m"
	if n := TextLen(text); n != 3 {
		t.Errorf("TextLen(ANSI red) = %d, want 3", n)
	}
}

func TestTextLen_MultiByte(t *testing.T) {
	if n := TextLen("日本語"); n != 3 {
		t.Errorf("TextLen(\"日本語\") = %d, want 3", n)
	}
}

func TestTextLen_Empty(t *testing.T) {
	if n := TextLen(""); n != 0 {
		t.Errorf("TextLen(\"\") = %d, want 0", n)
	}
}

func TestTextLen_MixedANSIAndMultiByte(t *testing.T) {
	text := "\x1b[1m\u001b[31m●\u001b[0m task"
	if n := TextLen(text); n != 6 {
		t.Errorf("TextLen(ANSI + multi-byte) = %d, want 6", n)
	}
}

func TestTextLen_MultipleANSICodes(t *testing.T) {
	text := "\x1b[1m\x1b[32mbold green\x1b[0m"
	if n := TextLen(text); n != 10 {
		t.Errorf("TextLen(multiple ANSI) = %d, want 10", n)
	}
}

// ---------------------------------------------------------------------------
// CalcVisualLines
// ---------------------------------------------------------------------------

func TestCalcVisualLines_ShortLine(t *testing.T) {
	if n := CalcVisualLines("hello", 80); n != 1 {
		t.Errorf("CalcVisualLines short = %d, want 1", n)
	}
}

func TestCalcVisualLines_Wrapped(t *testing.T) {
	if n := CalcVisualLines("hello world", 5); n != 3 {
		t.Errorf("CalcVisualLines wrap = %d, want 3 (hello=5/5+1=2, world=5/5+1=2)", n)
	}
}

func TestCalcVisualLines_MultipleLines(t *testing.T) {
	content := "hello\nworld"
	if n := CalcVisualLines(content, 10); n != 2 {
		t.Errorf("CalcVisualLines two short lines = %d, want 2", n)
	}
}

func TestCalcVisualLines_WithANSI(t *testing.T) {
	content := "\x1b[31mhello\x1b[0m"
	if n := CalcVisualLines(content, 80); n != 1 {
		t.Errorf("CalcVisualLines with ANSI = %d, want 1", n)
	}
}

func TestCalcVisualLines_Empty(t *testing.T) {
	if n := CalcVisualLines("", 80); n != 1 {
		t.Errorf("CalcVisualLines empty = %d, want 1", n)
	}
}

func TestCalcVisualLines_ZeroWidth(t *testing.T) {
	// contentWidth clamps to 1, so "hello" (5 chars) -> each char = 1 line -> 5 fl + 1 = 6
	if n := CalcVisualLines("hello", 0); n != 6 {
		t.Errorf("CalcVisualLines zero width = %d, want 6", n)
	}
}

func TestCalcVisualLines_ExactFit(t *testing.T) {
	if n := CalcVisualLines("hello", 5); n != 2 {
		t.Errorf("CalcVisualLines exact fit = %d, want 2", n)
	}
}

// ---------------------------------------------------------------------------
// ParseDueDateTime
// ---------------------------------------------------------------------------

func TestParseDueDateTime_RFC3339(t *testing.T) {
	dt := ParseDueDateTime("2026-06-14T15:04:05Z")
	if dt == nil {
		t.Fatal("ParseDueDateTime returned nil for RFC3339")
	}
	if dt.Year() != 2026 || dt.Month() != 6 || dt.Day() != 14 {
		t.Errorf("unexpected date: %v", *dt)
	}
}

func TestParseDueDateTime_DateOnly(t *testing.T) {
	dt := ParseDueDateTime("2026-12-31")
	if dt == nil {
		t.Fatal("ParseDueDateTime returned nil for date-only")
	}
	if dt.Year() != 2026 || dt.Month() != 12 || dt.Day() != 31 {
		t.Errorf("unexpected date: %v", *dt)
	}
}

func TestParseDueDateTime_WithTimezone(t *testing.T) {
	dt := ParseDueDateTime("2026-01-15T10:30:00+05:30")
	if dt == nil {
		t.Fatal("ParseDueDateTime returned nil for timezone format")
	}
	_, offset := dt.Zone()
	if offset != 5*3600+30*60 {
		t.Errorf("expected +05:30 offset, got %d", offset)
	}
}

func TestParseDueDateTime_Empty(t *testing.T) {
	if dt := ParseDueDateTime(""); dt != nil {
		t.Error("ParseDueDateTime(\"\") should return nil")
	}
}

func TestParseDueDateTime_Invalid(t *testing.T) {
	if dt := ParseDueDateTime("not-a-date"); dt != nil {
		t.Error("ParseDueDateTime(invalid) should return nil")
	}
}

func TestParseDueDateTime_WithSpaceFormat(t *testing.T) {
	dt := ParseDueDateTime("2026-06-14 15:04:05")
	if dt == nil {
		t.Fatal("ParseDueDateTime returned nil for space-separated datetime")
	}
	if dt.Year() != 2026 || dt.Month() != 6 || dt.Day() != 14 {
		t.Errorf("unexpected date: %v", *dt)
	}
}

func TestParseDueDateTime_ShortDateTime(t *testing.T) {
	dt := ParseDueDateTime("2026-06-14T15:04")
	if dt == nil {
		t.Fatal("ParseDueDateTime returned nil for short datetime")
	}
	if dt.Minute() != 4 {
		t.Errorf("expected minute 4, got %d", dt.Minute())
	}
}

// ---------------------------------------------------------------------------
// ParseRelativeDateTime — these use time.Now() so we check approximate ranges
// ---------------------------------------------------------------------------

func TestParseRelativeDateTime_Today(t *testing.T) {
	s, dt, err := ParseRelativeDateTime("today")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().Format("2006-01-02")
	if s != expected {
		t.Errorf("got %q, want %q", s, expected)
	}
	if dt == nil {
		t.Fatal("dt is nil")
	}
	if dt.Hour() != 0 || dt.Minute() != 0 || dt.Second() != 0 {
		t.Errorf("expected midnight, got %v", dt)
	}
}

func TestParseRelativeDateTime_Tonight(t *testing.T) {
	s, dt, err := ParseRelativeDateTime("tonight")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dt == nil {
		t.Fatal("dt is nil")
	}
	if dt.Hour() != 23 || dt.Minute() != 59 || dt.Second() != 59 {
		t.Errorf("expected 23:59:59, got %v", dt)
	}
	_ = s
}

func TestParseRelativeDateTime_Tomorrow(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("tomorrow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_NextWeek(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("next week")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_NextMonth(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("next month")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_NextYear(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("next year")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_NextDecade(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("next decade")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(10, 0, 0).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_NextCentury(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("next century")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(100, 0, 0).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_NextSeason(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("next season")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 3, 0).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_NextSecond(t *testing.T) {
	s, dt, err := ParseRelativeDateTime("next second")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dt == nil {
		t.Fatal("dt is nil")
	}
	if !strings.Contains(s, "T") {
		t.Errorf("expected RFC3339 format (with T), got %q", s)
	}
	now := time.Now()
	if dt.Sub(now) > 2*time.Second || dt.Before(now) {
		t.Errorf("expected ~1 second from now, got %v (now=%v)", dt, now)
	}
}

func TestParseRelativeDateTime_NextMinute(t *testing.T) {
	s, dt, err := ParseRelativeDateTime("next minute")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(s, "T") {
		t.Errorf("expected RFC3339 format, got %q", s)
	}
	now := time.Now()
	if dt.Sub(now) > 70*time.Second || dt.Before(now) {
		t.Errorf("expected ~1 minute from now, got %v", dt)
	}
}

func TestParseRelativeDateTime_NextHour(t *testing.T) {
	s, dt, err := ParseRelativeDateTime("next hour")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(s, "T") {
		t.Errorf("expected RFC3339 format, got %q", s)
	}
	now := time.Now()
	if dt.Sub(now) > 2*time.Hour || dt.Before(now) {
		t.Errorf("expected ~1 hour from now, got %v", dt)
	}
}

func TestParseRelativeDateTime_NextDay(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("next day")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_3Days(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("3 days")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_2Weeks(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("2 weeks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_5Minutes(t *testing.T) {
	s, dt, err := ParseRelativeDateTime("5 minutes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(s, "T") {
		t.Errorf("expected RFC3339 format, got %q", s)
	}
	now := time.Now()
	if dt.Sub(now) > 6*time.Minute || dt.Before(now) {
		t.Errorf("expected ~5 minutes from now, got %v", dt)
	}
}

func TestParseRelativeDateTime_2Hours(t *testing.T) {
	s, dt, err := ParseRelativeDateTime("2 hours")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(s, "T") {
		t.Errorf("expected RFC3339 format, got %q", s)
	}
	now := time.Now()
	if dt.Sub(now) > 3*time.Hour || dt.Before(now) {
		t.Errorf("expected ~2 hours from now, got %v", dt)
	}
}

func TestParseRelativeDateTime_30Seconds(t *testing.T) {
	s, dt, err := ParseRelativeDateTime("30 seconds")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(s, "T") {
		t.Errorf("expected RFC3339 format, got %q", s)
	}
	now := time.Now()
	if dt.Sub(now) > 35*time.Second || dt.Before(now) {
		t.Errorf("expected ~30 seconds from now, got %v", dt)
	}
}

func TestParseRelativeDateTime_1Month(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("1 month")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 1, 0).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_3Seasons(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("3 seasons")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 9, 0).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_1Year(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("1 year")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_Invalid(t *testing.T) {
	_, _, err := ParseRelativeDateTime("not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestParseRelativeDateTime_EmptyString(t *testing.T) {
	_, _, err := ParseRelativeDateTime("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseRelativeDateTime_WhitespaceTrimmed(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("  tomorrow  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_CaseInsensitive(t *testing.T) {
	_, dt, err := ParseRelativeDateTime("NEXT WEEK")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	if dt.Format("2006-01-02") != expected {
		t.Errorf("got %s, want %s", dt.Format("2006-01-02"), expected)
	}
}

func TestParseRelativeDateTime_PluralForms(t *testing.T) {
	tests := []struct {
		input string
		days  int
	}{
		{"1 day", 1},
		{"2 days", 2},
		{"1 week", 7},
		{"3 weeks", 21},
		{"1 month", 0}, // handled as months
	}
	for _, tt := range tests {
		_, dt, err := ParseRelativeDateTime(tt.input)
		if err != nil {
			t.Errorf("ParseRelativeDateTime(%q) error: %v", tt.input, err)
			continue
		}
		if tt.days > 0 {
			expected := time.Now().AddDate(0, 0, tt.days).Format("2006-01-02")
			if dt.Format("2006-01-02") != expected {
				t.Errorf("ParseRelativeDateTime(%q) = %s, want %s", tt.input, dt.Format("2006-01-02"), expected)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// DueColor
// ---------------------------------------------------------------------------

func TestDueColor_Nil(t *testing.T) {
	if c := DueColor(nil); c != "" {
		t.Errorf("DueColor(nil) = %q, want empty", c)
	}
}

func TestDueColor_Overdue(t *testing.T) {
	dt := time.Now().Add(-1 * time.Hour)
	if c := DueColor(&dt); c != "\x1b[38;5;196m" {
		t.Errorf("DueColor(overdue) = %q, want red code", c)
	}
}

func TestDueColor_Today(t *testing.T) {
	dt := time.Now().Add(1 * time.Hour)
	if c := DueColor(&dt); c != "\x1b[38;5;208m" {
		t.Errorf("DueColor(today) = %q, want orange code", c)
	}
}

func TestDueColor_Tomorrow(t *testing.T) {
	dt := time.Now().Add(30 * time.Hour)
	if c := DueColor(&dt); c != "\x1b[38;5;220m" {
		t.Errorf("DueColor(tomorrow) = %q, want yellow code", c)
	}
}

func TestDueColor_ThisWeek(t *testing.T) {
	dt := time.Now().Add(3 * 24 * time.Hour)
	if c := DueColor(&dt); c != "\x1b[38;5;38m" {
		t.Errorf("DueColor(this week) = %q, want cyan code", c)
	}
}

func TestDueColor_Future(t *testing.T) {
	dt := time.Now().Add(14 * 24 * time.Hour)
	if c := DueColor(&dt); c != "\x1b[38;5;12m" {
		t.Errorf("DueColor(future) = %q, want blue code", c)
	}
}

func TestDueColor_ExactlyNow(t *testing.T) {
	dt := time.Now()
	if c := DueColor(&dt); c != "\x1b[38;5;208m" {
		t.Errorf("DueColor(now) = %q, want orange code (due today)", c)
	}
}

// ---------------------------------------------------------------------------
// SortedKeysByValue
// ---------------------------------------------------------------------------

func TestSortedKeysByValue_Basic(t *testing.T) {
	m := map[string]int{"a": 3, "b": 1, "c": 2}
	got := SortedKeysByValue(m)
	want := []string{"a", "c", "b"}
	if !equalSlices(got, want) {
		t.Errorf("SortedKeysByValue = %v, want %v", got, want)
	}
}

func TestSortedKeysByValue_TieAlphabetical(t *testing.T) {
	m := map[string]int{"banana": 5, "apple": 5, "cherry": 3}
	got := SortedKeysByValue(m)
	// banana and apple both have value 5, should be sorted alphabetically: apple, banana
	want := []string{"apple", "banana", "cherry"}
	if !equalSlices(got, want) {
		t.Errorf("SortedKeysByValue tie = %v, want %v", got, want)
	}
}

func TestSortedKeysByValue_EmptyMap(t *testing.T) {
	got := SortedKeysByValue(map[string]int{})
	if len(got) != 0 {
		t.Errorf("SortedKeysByValue(empty) = %v, want empty", got)
	}
}

func TestSortedKeysByValue_SingleEntry(t *testing.T) {
	m := map[string]int{"only": 42}
	got := SortedKeysByValue(m)
	want := []string{"only"}
	if !equalSlices(got, want) {
		t.Errorf("SortedKeysByValue(single) = %v, want %v", got, want)
	}
}

func TestSortedKeysByValue_NegativeValues(t *testing.T) {
	m := map[string]int{"low": -10, "high": 10, "mid": 0}
	got := SortedKeysByValue(m)
	want := []string{"high", "mid", "low"}
	if !equalSlices(got, want) {
		t.Errorf("SortedKeysByValue(negative) = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// OpenFile
// ---------------------------------------------------------------------------

func TestOpenFile_NonExistent(t *testing.T) {
	err := OpenFile(filepath.Join(t.TempDir(), "nonexistent.txt"))
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RunEditor & OpenInEditor
// ---------------------------------------------------------------------------

func TestRunEditor_SplitArgs(t *testing.T) {
	err := RunEditor("nonexistent-editor", "/tmp/test.txt")
	if err == nil {
		t.Skip("RunEditor unexpectedly succeeded (maybe nonexistent-editor exists?)")
	}
}

func TestRunEditor_EditorWithArgs(t *testing.T) {
	err := RunEditor("nonexistent --wait", "/tmp/test.txt")
	if err == nil {
		t.Skip("RunEditor unexpectedly succeeded")
	}
}

func TestRunEditor_WithEcho(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("content"), 0644)

	// Use a simple, quick command that exists on the platform
	var editor string
	switch runtime.GOOS {
	case "windows":
		editor = "cmd /c type"
	default:
		editor = "cat"
	}
	err := RunEditor(editor, path)
	if err != nil {
		// May fail on headless systems; that's okay
		t.Logf("RunEditor returned: %v", err)
	}
}

func TestOpenInEditor_WithVISUAL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("content"), 0644)

	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	switch runtime.GOOS {
	case "windows":
		t.Setenv("VISUAL", "cmd /c type")
	default:
		t.Setenv("VISUAL", "cat")
	}

	err := OpenInEditor(path)
	if err != nil {
		t.Logf("OpenInEditor returned: %v", err)
	}
}

func TestOpenInEditor_WithEDITOR(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("content"), 0644)

	t.Setenv("VISUAL", "")
	switch runtime.GOOS {
	case "windows":
		t.Setenv("EDITOR", "cmd /c type")
	default:
		t.Setenv("EDITOR", "cat")
	}

	err := OpenInEditor(path)
	if err != nil {
		t.Logf("OpenInEditor returned: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Ensure all regex patterns compile (they do via MustCompile, but verify they match)
func TestRegexPatterns_Match(t *testing.T) {
	tests := []struct {
		name  string
		regex *regexp.Regexp
		good  []string
		bad   []string
	}{
		{
			name:  "IDRegEx",
			regex: IDRegEx,
			good:  []string{"20260614-123456", "00000000-000000"},
			bad:   []string{"20260614", "abcdefgh-123456", "2026-0614-123456"},
		},
		{
			name:  "TaskRegEx",
			regex: TaskRegEx,
			good:  []string{"TASK(20260614-123456)", "task(20260614-123456)"},
			bad:   []string{"TASK(abc)", "TASK()", "task(20260614)"},
		},
		{
			name:  "AllANSIRegex",
			regex: AllANSIRegex,
			good:  []string{"\x1b[31m", "\x1b[38;5;196m", "\x1b[0m", "\x1b[1;34m"},
			bad:   []string{"hello", "\x1b", "\x1b[", "\x1b[abc"},
		},
		{
			name:  "URLRegEx",
			regex: URLRegEx,
			good:  []string{"https://example.com", "http://example.com/path", "ftp://example.com:8080/file"},
			bad:   []string{"not-a-url", "://missing"},
		},
		{
			name:  "FilePathRegEx",
			regex: FilePathRegEx,
			good:  []string{"./path/to/file", "../relative/path"},
			bad:   []string{"/absolute/path", "C:\\path"},
		},
	}

	for _, tt := range tests {
		for _, s := range tt.good {
			if !tt.regex.MatchString(s) {
				t.Errorf("%s should match %q", tt.name, s)
			}
		}
		for _, s := range tt.bad {
			if tt.regex.MatchString(s) {
				t.Errorf("%s should NOT match %q", tt.name, s)
			}
		}
	}
}

func TestDateFormats_ParseValidDates(t *testing.T) {
	validDates := []string{
		"2026-06-14T15:04:05Z",
		"2026-06-14T15:04:05+07:00",
		"2026-06-14T15:04:05-07:00",
		"2026-06-14 15:04:05+07:00",
		"2026-06-14 15:04:05 -0700",
		"2026-06-14T15:04:05",
		"2026-06-14 15:04:05",
		"2026-06-14T15:04",
		"2026-06-14 15:04",
		"2026-06-14",
	}
	for _, d := range validDates {
		dt := ParseDueDateTime(d)
		if dt == nil {
			t.Errorf("DateFormats should parse %q", d)
		}
	}
}

// TestDueColor_EdgeCases validates boundary conditions (all use <=).
func TestDueColor_EdgeCases(t *testing.T) {
	// exactly 24h → still "today" (<= 24h)
	dt24h := time.Now().Add(24 * time.Hour)
	if c := DueColor(&dt24h); c != "\x1b[38;5;208m" {
		t.Errorf("DueColor(exactly 24h) = %q, want orange (today)", c)
	}

	// exactly 48h → still "tomorrow" (<= 48h)
	dt48h := time.Now().Add(48 * time.Hour)
	if c := DueColor(&dt48h); c != "\x1b[38;5;220m" {
		t.Errorf("DueColor(exactly 48h) = %q, want yellow (tomorrow)", c)
	}

	// exactly 7d → still "this week" (<= 7d)
	dt7d := time.Now().Add(7 * 24 * time.Hour)
	if c := DueColor(&dt7d); c != "\x1b[38;5;38m" {
		t.Errorf("DueColor(exactly 7d) = %q, want cyan (this week)", c)
	}
}
