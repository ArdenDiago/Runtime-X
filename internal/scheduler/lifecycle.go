package scheduler

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"
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
//
// Architecture: the write lock is released before cmd.Start() (blocking fork+exec)
// to prevent deadlocking readers and the logBuffer's independent mutex. State is
// set to Starting before unlock so external callers observe the transition.
func (s *Scheduler) Start(name string) error {
	s.mu.Lock()

	mp, exists := s.processes[name]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	// Validate the state allows starting.
	switch mp.State {
	case StateIdle, StateStopped, StateFailed:
		// allowed — proceed
	case StateRunning:
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAlreadyRunning, name)
	default:
		// Starting, Stopping — transient states, reject with descriptive error.
		s.mu.Unlock()
		return fmt.Errorf("cannot start process %q in state %s", name, mp.State)
	}

	// Transition to Starting while holding lock — makes the state visible immediately
	// to concurrent callers of Get() and List().
	if err := transition(mp, StateStarting); err != nil {
		s.mu.Unlock()
		return err
	}

	// Reset runtime metadata for restart from Stopped or Failed.
	mp.StartedAt = time.Now()
	mp.StoppedAt = time.Time{}
	mp.ExitCode = 0

	// Capture immutable local copy of definition before releasing the lock.
	def := mp.Def

	// CRITICAL: release write lock before cmd.Start() (blocking OS fork+exec).
	// See STATE.md [v1.1 arch]: "Release scheduler write lock before cmd.Start()".
	s.mu.Unlock()

	// Build a fresh *exec.Cmd — exec.Cmd cannot be reused after Start().
	cmd := exec.Command(def.Command, def.Args...)
	if def.WorkDir != "" {
		cmd.Dir = def.WorkDir
	}
	if len(def.Env) > 0 {
		cmd.Env = def.Env
	}
	// Set Setpgid so the child and its children get a new PGID == child PID.
	// This allows Stop() to send signals to the entire process group, preventing
	// orphan grandchildren when a shell script forks sub-processes.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Attach pipes before Start(). They must be created before cmd.Start() is called.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		s.mu.Lock()
		transition(mp, StateFailed) //nolint:errcheck — only fails on logic bugs
		s.mu.Unlock()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		s.mu.Lock()
		transition(mp, StateFailed) //nolint:errcheck
		s.mu.Unlock()
		return fmt.Errorf("stderr pipe: %w", err)
	}

	// Fork+exec the process. On failure, transition to Failed so the caller
	// can inspect the error and retry or remove the process.
	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		transition(mp, StateFailed) //nolint:errcheck
		s.mu.Unlock()
		return fmt.Errorf("start %q: %w", def.Command, err)
	}

	// cmd.Start() succeeded — PID is now valid. Re-acquire lock to update state.
	s.mu.Lock()
	transition(mp, StateRunning) //nolint:errcheck — Starting → Running is always valid
	mp.cmd = cmd
	s.mu.Unlock()

	// Launch output capture goroutines. They write to mp.logs which has its own
	// independent mutex, so they never contend with the scheduler write lock.
	go captureOutput(mp.logs, stdoutPipe, StreamStdout)
	go captureOutput(mp.logs, stderrPipe, StreamStderr)

	// Launch the monitor goroutine — single source of truth for terminal state
	// transitions (Running → Stopped or Running → Failed).
	go monitorProcess(s, mp, cmd)

	return nil
}

// captureOutput reads lines from r using a bufio.Scanner and writes each line
// as a LogEntry to lb with the given stream tag. It runs until EOF or pipe close.
//
// Scanner.Buffer sets the maximum line size to maxLineBytes (8KB). Lines
// exceeding this will cause the scanner to stop (ErrTooLong), which is treated
// as benign — binary data dumps should not block normal log capture.
func captureOutput(lb *logBuffer, r io.ReadCloser, stream Stream) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), maxLineBytes)

	for scanner.Scan() {
		lb.Write(LogEntry{
			Timestamp: time.Now(),
			Stream:    stream,
			Text:      scanner.Text(),
		})
	}
	// scanner.Err() is nil on EOF (normal process exit) — ignore it.
	// On unexpected pipe close, it returns an io error — also benign; we stop capturing.
}

// monitorProcess blocks on cmd.Wait() until the process exits (and all pipe
// goroutines have finished draining output), then transitions the process to
// Stopped or Failed and closes mp.doneCh to unblock a pending Stop() call.
//
// monitorProcess is the single authority for terminal state transitions. Only
// Stop() writes Stopping, and only monitorProcess transitions from there to Stopped.
func monitorProcess(s *Scheduler, mp *ManagedProcess, cmd *exec.Cmd) {
	// cmd.Wait() blocks until the process exits AND all pipe goroutines complete.
	// This ensures all output is captured in mp.logs before we update state.
	err := cmd.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	mp.StoppedAt = time.Now()

	// Determine exit code. ProcessState is always set after Wait() returns,
	// but guard defensively; -1 indicates signal-killed with unknown code.
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	mp.ExitCode = code

	if mp.State == StateStopping {
		// Stop() requested — transition to Stopped regardless of exit code.
		transition(mp, StateStopped) //nolint:errcheck
	} else if err == nil || code == 0 {
		// Natural clean exit.
		transition(mp, StateStopped) //nolint:errcheck
	} else {
		// Crash or non-zero exit without a Stop() request.
		transition(mp, StateFailed) //nolint:errcheck
	}

	// Signal Stop() that the process has fully exited (if Stop() is waiting).
	if mp.doneCh != nil {
		close(mp.doneCh)
		mp.doneCh = nil
	}
}
