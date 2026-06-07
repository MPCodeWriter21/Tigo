/* Package logs provides a simple in-memory logging mechanism for the application.

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
	mu      sync.Mutex
	entries []Entry
	maxSize = 500
)

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
}

func Clear() {
	mu.Lock()
	defer mu.Unlock()
	entries = nil
}

func Entries() []Entry {
	mu.Lock()
	defer mu.Unlock()
	cp := make([]Entry, len(entries))
	copy(cp, entries)
	return cp
}
