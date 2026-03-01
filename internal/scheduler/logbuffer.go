package scheduler

import (
	"sync"
	"time"
)

// Stream identifies which output stream a log entry came from.
type Stream string

const (
	// StreamStdout indicates the entry came from standard output.
	StreamStdout Stream = "stdout"
	// StreamStderr indicates the entry came from standard error.
	StreamStderr Stream = "stderr"
)

// LogEntry is a single captured output line with metadata.
type LogEntry struct {
	// Timestamp records when the line was captured.
	Timestamp time.Time
	// Stream is the source stream (stdout or stderr).
	Stream Stream
	// Text is the raw output line (without trailing newline).
	Text string
}

// logBuffer is a bounded ring buffer of LogEntry values.
//
// It has its own mutex, independent of the Scheduler's mutex,
// so goroutines writing stdout/stderr can call Write() concurrently
// without acquiring the Scheduler lock (prevents Phase 6 deadlocks).
type logBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	size    int
	head    int // index of next write position (wraps with modulo)
	count   int // number of valid entries in [0, size]
}

// newLogBuffer allocates a logBuffer with the given capacity.
// If size <= 0, the default capacity of 1000 entries is used.
func newLogBuffer(size int) *logBuffer {
	if size <= 0 {
		size = 1000
	}
	return &logBuffer{
		entries: make([]LogEntry, size),
		size:    size,
	}
}

// Write appends an entry to the buffer. If the buffer is full, the oldest
// entry is silently overwritten. Safe to call from multiple goroutines.
func (lb *logBuffer) Write(entry LogEntry) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.entries[lb.head] = entry
	lb.head = (lb.head + 1) % lb.size
	if lb.count < lb.size {
		lb.count++
	}
}

// Lines returns a snapshot of all entries in chronological order (oldest first).
// Returns nil if no entries have been written. The returned slice is a new
// allocation and is safe to retain and mutate without affecting the buffer.
// Safe to call concurrently with Write().
func (lb *logBuffer) Lines() []LogEntry {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if lb.count == 0 {
		return nil
	}
	out := make([]LogEntry, lb.count)
	if lb.count < lb.size {
		// Buffer not yet wrapped: all valid entries are at [0, count).
		copy(out, lb.entries[:lb.count])
	} else {
		// Buffer is full: oldest entry is at head (the next write position).
		// Reconstruct chronological order: [head:] then [:head].
		n := copy(out, lb.entries[lb.head:])
		copy(out[n:], lb.entries[:lb.head])
	}
	return out
}

// Len returns the number of entries currently stored in the buffer.
func (lb *logBuffer) Len() int {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.count
}
