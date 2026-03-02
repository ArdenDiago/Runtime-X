package scheduler

import (
	"errors"
	"io"
	"os/exec"
)

// maxLineBytes is the maximum number of bytes per output line.
// Lines exceeding this length are truncated by the scanner.
const maxLineBytes = 8 * 1024

// Start spawns the named process as a real OS process, transitions it through
// Starting → Running, and launches output capture and monitor goroutines.
//
// Start is callable from Idle, Stopped, or Failed states. Returns ErrNotFound
// if the process is not registered, ErrAlreadyRunning if already running, or
// a descriptive error for transient states (Starting, Stopping).
func (s *Scheduler) Start(name string) error {
	return errors.New("not implemented")
}

// captureOutput reads lines from r using a bufio.Scanner and writes each line
// as a LogEntry to lb with the given stream tag. It runs until EOF or pipe close.
func captureOutput(lb *logBuffer, r io.ReadCloser, stream Stream) {
	// stub — no-op
}

// monitorProcess blocks on cmd.Wait() until the process exits, then updates
// the process state and closes mp.doneCh if Stop() is waiting.
func monitorProcess(s *Scheduler, mp *ManagedProcess, cmd *exec.Cmd) {
	// stub — no-op
}
