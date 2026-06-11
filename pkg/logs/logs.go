/*
	Package logs provides a simple in-memory logging mechanism for the application.

It allows appending log entries with different levels (INFO, WARN, ERROR, GIT) and retrieving them.
The logs are stored in a slice and are protected by a mutex to ensure thread safety.
The maximum number of log entries is limited to 500 to prevent excessive memory usage.
The package also provides a function to clear all log entries.
*/
package logs

import (
	"fmt"
	"sync"
	"time"
)

type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
	LevelGit
)

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INF"
	case LevelWarn:
		return "WRN"
	case LevelError:
		return "ERR"
	case LevelGit:
		return "GIT"
	default:
		return "???"
	}
}

type Entry struct {
	Time    time.Time
	Level   Level
	Message string
}

var (
	mu        sync.Mutex
	entries   []Entry
	callbacks map[string]func() = make(map[string]func())
	maxSize                     = 500
)

// Append adds a new log entry with the specified level and message.
func Append(level Level, msg string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	entries = append(entries, Entry{Time: time.Now(), Level: level, Message: msg})
	if len(entries) > maxSize {
		entries = entries[len(entries)-maxSize:]
	}
	notifyCallbacks()
}

// Clear removes all log entries from the in-memory log.
func Clear() {
	mu.Lock()
	defer mu.Unlock()
	entries = nil
	notifyCallbacks()
}

// Entries returns a copy of the current log entries.
func Entries() []Entry {
	mu.Lock()
	defer mu.Unlock()
	cp := make([]Entry, len(entries))
	copy(cp, entries)
	return cp
}

// SetMaxSize sets the maximum number of log entries to keep in memory.
func SetMaxSize(size int) {
	mu.Lock()
	defer mu.Unlock()
	maxSize = size
	if len(entries) > maxSize {
		entries = entries[len(entries)-maxSize:]
	}
}

// RegisterCallback registers a callback function that will be called whenever a new log entry is added.
func RegisterCallback(name string, cb func()) {
	mu.Lock()
	defer mu.Unlock()
	callbacks[name] = cb
}

// UnregisterCallback removes a previously registered callback function.
func UnregisterCallback(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(callbacks, name)
}

// notifyCallbacks calls all registered callback functions.
func notifyCallbacks() {
	for _, cb := range callbacks {
		cb()
	}
}
