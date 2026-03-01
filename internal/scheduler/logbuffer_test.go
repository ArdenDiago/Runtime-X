package scheduler

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestLogBufferBasic covers the core read/write behavior of the ring buffer.
func TestLogBufferBasic(t *testing.T) {
	t.Parallel()

	now := time.Now()
	makeEntry := func(i int, stream Stream) LogEntry {
		return LogEntry{
			Timestamp: now.Add(time.Duration(i) * time.Millisecond),
			Stream:    stream,
			Text:      fmt.Sprintf("line-%d", i),
		}
	}

	tests := []struct {
		name       string
		size       int
		writes     []LogEntry
		wantLen    int
		wantLines  []LogEntry // expected chronological order; nil means nil
	}{
		{
			name: "three writes into size-5 buffer",
			size: 5,
			writes: []LogEntry{
				makeEntry(0, StreamStdout),
				makeEntry(1, StreamStdout),
				makeEntry(2, StreamStdout),
			},
			wantLen: 3,
			wantLines: []LogEntry{
				makeEntry(0, StreamStdout),
				makeEntry(1, StreamStdout),
				makeEntry(2, StreamStdout),
			},
		},
		{
			name: "five writes into size-3 buffer evicts oldest",
			size: 3,
			writes: []LogEntry{
				makeEntry(0, StreamStdout),
				makeEntry(1, StreamStdout),
				makeEntry(2, StreamStdout),
				makeEntry(3, StreamStdout),
				makeEntry(4, StreamStdout),
			},
			wantLen: 3,
			wantLines: []LogEntry{
				makeEntry(2, StreamStdout),
				makeEntry(3, StreamStdout),
				makeEntry(4, StreamStdout),
			},
		},
		{
			name: "size-1 buffer keeps only last entry",
			size: 1,
			writes: []LogEntry{
				makeEntry(0, StreamStdout),
				makeEntry(1, StreamStdout),
				makeEntry(2, StreamStdout),
			},
			wantLen: 1,
			wantLines: []LogEntry{
				makeEntry(2, StreamStdout),
			},
		},
		{
			name:      "empty buffer returns nil and zero len",
			size:      5,
			writes:    nil,
			wantLen:   0,
			wantLines: nil,
		},
		{
			name: "stream tags are preserved in order",
			size: 4,
			writes: []LogEntry{
				makeEntry(0, StreamStdout),
				makeEntry(1, StreamStderr),
				makeEntry(2, StreamStdout),
				makeEntry(3, StreamStderr),
			},
			wantLen: 4,
			wantLines: []LogEntry{
				makeEntry(0, StreamStdout),
				makeEntry(1, StreamStderr),
				makeEntry(2, StreamStdout),
				makeEntry(3, StreamStderr),
			},
		},
		{
			name: "exactly full buffer returns all entries in order",
			size: 3,
			writes: []LogEntry{
				makeEntry(0, StreamStdout),
				makeEntry(1, StreamStdout),
				makeEntry(2, StreamStdout),
			},
			wantLen: 3,
			wantLines: []LogEntry{
				makeEntry(0, StreamStdout),
				makeEntry(1, StreamStdout),
				makeEntry(2, StreamStdout),
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lb := newLogBuffer(tc.size)
			for _, e := range tc.writes {
				lb.Write(e)
			}

			gotLen := lb.Len()
			if gotLen != tc.wantLen {
				t.Errorf("Len() = %d, want %d", gotLen, tc.wantLen)
			}

			gotLines := lb.Lines()
			if tc.wantLines == nil {
				if gotLines != nil {
					t.Errorf("Lines() = %v, want nil", gotLines)
				}
				return
			}
			if len(gotLines) != len(tc.wantLines) {
				t.Errorf("Lines() returned %d entries, want %d", len(gotLines), len(tc.wantLines))
				return
			}
			for i, got := range gotLines {
				want := tc.wantLines[i]
				if got.Text != want.Text {
					t.Errorf("Lines()[%d].Text = %q, want %q", i, got.Text, want.Text)
				}
				if got.Stream != want.Stream {
					t.Errorf("Lines()[%d].Stream = %q, want %q", i, got.Stream, want.Stream)
				}
				if !got.Timestamp.Equal(want.Timestamp) {
					t.Errorf("Lines()[%d].Timestamp = %v, want %v", i, got.Timestamp, want.Timestamp)
				}
			}
		})
	}
}

// TestLogBufferDefaultSize verifies that size <= 0 defaults to 1000.
func TestLogBufferDefaultSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		size int
	}{
		{"zero size defaults to 1000", 0},
		{"negative size defaults to 1000", -1},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lb := newLogBuffer(tc.size)
			if lb == nil {
				t.Fatal("newLogBuffer returned nil")
			}
			// Write 1000 entries — should all fit (default size is 1000)
			for i := 0; i < 1000; i++ {
				lb.Write(LogEntry{
					Timestamp: time.Now(),
					Stream:    StreamStdout,
					Text:      fmt.Sprintf("line-%d", i),
				})
			}
			if lb.Len() != 1000 {
				t.Errorf("Len() = %d after 1000 writes, want 1000 (default size)", lb.Len())
			}
			// Write one more — should evict oldest, still 1000
			lb.Write(LogEntry{Timestamp: time.Now(), Stream: StreamStdout, Text: "overflow"})
			if lb.Len() != 1000 {
				t.Errorf("Len() = %d after 1001 writes, want 1000", lb.Len())
			}
		})
	}
}

// TestLogBufferLinesSnapshot verifies that Lines() returns a new slice
// and does not expose internal state to the caller.
func TestLogBufferLinesSnapshot(t *testing.T) {
	t.Parallel()

	lb := newLogBuffer(5)
	lb.Write(LogEntry{Timestamp: time.Now(), Stream: StreamStdout, Text: "original"})

	lines := lb.Lines()
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	// Mutate the returned slice — should not affect the buffer
	lines[0].Text = "mutated"

	// Write a second entry and retrieve again
	lb.Write(LogEntry{Timestamp: time.Now(), Stream: StreamStdout, Text: "second"})
	lines2 := lb.Lines()
	if len(lines2) < 1 {
		t.Fatalf("expected at least 1 line, got %d", len(lines2))
	}
	// First entry should still be "original", not "mutated"
	if lines2[0].Text != "original" {
		t.Errorf("Lines()[0].Text = %q after external mutation, want %q", lines2[0].Text, "original")
	}
}

// TestLogBufferConcurrentWriteAndRead verifies that concurrent Write and Lines
// calls produce no data races when run with go test -race.
func TestLogBufferConcurrentWriteAndRead(t *testing.T) {
	lb := newLogBuffer(100)
	const writers = 10
	const writesPerGoroutine = 50

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				lb.Write(LogEntry{
					Timestamp: time.Now(),
					Stream:    StreamStdout,
					Text:      "line",
				})
			}
		}()
	}

	// Concurrent reader goroutine
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				lb.Lines()
			}
		}
	}()

	wg.Wait()
	close(done)

	// After all writes complete, buffer should have exactly 100 entries
	// (10 writers × 50 writes = 500 total, buffer size 100 => last 100 retained)
	if lb.Len() != 100 {
		t.Errorf("Len() = %d after concurrent writes, want 100", lb.Len())
	}
}
