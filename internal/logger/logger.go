package logger

import (
	"sync"
	"sync/atomic"
	"time"
)

// Log levels
const (
	LevelInfo    = "info"
	LevelSuccess = "success"
	LevelWarning = "warning"
	LevelError   = "error"
)

// DefaultMaxCapacity limits memory consumption per ADR 0002.
const DefaultMaxCapacity = 1000

// LogEntry represents an individual log line.
type LogEntry struct {
	ID        int64  `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// LogBuffer stores a bounded list of log entries in memory.
type LogBuffer struct {
	mu       sync.RWMutex
	entries  []LogEntry
	capacity int
	seq      int64
}

// NewLogBuffer initializes a thread-safe bounded log buffer.
func NewLogBuffer(capacity int) *LogBuffer {
	if capacity <= 0 {
		capacity = DefaultMaxCapacity
	}
	return &LogBuffer{
		entries:  make([]LogEntry, 0, capacity),
		capacity: capacity,
	}
}

// Add creates and appends a new LogEntry, evicting the oldest if capacity is exceeded.
func (b *LogBuffer) Add(level, message string) LogEntry {
	id := atomic.AddInt64(&b.seq, 1)
	entry := LogEntry{
		ID:        id,
		Timestamp: time.Now().Format("15:04:05.000"),
		Level:     level,
		Message:   message,
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.entries) >= b.capacity {
		// Evict oldest entries by slicing
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, entry)

	return entry
}

// GetAll returns a copy of all current log entries.
func (b *LogBuffer) GetAll() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]LogEntry, len(b.entries))
	copy(result, b.entries)
	return result
}

// Clear removes all log entries from the buffer.
func (b *LogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries = b.entries[:0]
}
