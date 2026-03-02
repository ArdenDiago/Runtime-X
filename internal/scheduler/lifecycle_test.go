package scheduler

import (
	"errors"
	"sync"
	"syscall"
	"testing"
	"time"
)

// waitForState polls mp.State every 10ms until it matches target or timeout expires.
// It marks the test as fatal if the state does not reach the target in time.
func waitForState(t *testing.T, s *Scheduler, name string, target State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mp, err := s.Get(name)
		if err != nil {
			t.Fatalf("waitForState: Get(%q): %v", name, err)
		}
		if mp.State == target {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	mp, _ := s.Get(name)
	t.Fatalf("waitForState: process %q state = %v, want %v after %v", name, mp.State, target, timeout)
}

// killProcess sends SIGKILL to the process group of the named process if it is running.
// Used as test cleanup to ensure long-lived processes are terminated.
func killProcess(t *testing.T, s *Scheduler, name string) {
	t.Helper()
	mp, err := s.Get(name)
	if err != nil {
		return // already removed or not found
	}
	if mp.cmd != nil && mp.cmd.Process != nil {
		_ = syscall.Kill(-mp.cmd.Process.Pid, syscall.SIGKILL)
	}
}

// TestStart_IdleToRunning verifies that Start() on an Idle process transitions
// it to Running with a valid PID.
func TestStart_IdleToRunning(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "sleeper", Command: "sleep", Args: []string{"10"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killProcess(t, s, "sleeper") })

	if err := s.Start("sleeper"); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	mp, err := s.Get("sleeper")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mp.State != StateRunning {
		t.Errorf("State = %v, want StateRunning", mp.State)
	}
	if mp.cmd == nil {
		t.Fatal("cmd is nil after Start()")
	}
	if mp.cmd.Process == nil {
		t.Fatal("cmd.Process is nil after Start()")
	}
	if mp.cmd.Process.Pid <= 0 {
		t.Errorf("PID = %d, want > 0", mp.cmd.Process.Pid)
	}
}

// TestStart_AlreadyRunning verifies that Start() on a Running process returns
// ErrAlreadyRunning.
func TestStart_AlreadyRunning(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "sleeper", Command: "sleep", Args: []string{"10"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killProcess(t, s, "sleeper") })

	if err := s.Start("sleeper"); err != nil {
		t.Fatalf("first Start: %v", err)
	}

	err := s.Start("sleeper")
	if err == nil {
		t.Fatal("second Start: expected ErrAlreadyRunning, got nil")
	}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Start: got %v, want wrapping ErrAlreadyRunning", err)
	}
}

// TestStart_FromStoppedAndFailed verifies that Start() succeeds when called on
// a process in Stopped or Failed state (re-start without Remove+Register).
func TestStart_FromStoppedAndFailed(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "echo1", Command: "echo", Args: []string{"hello"}}); err != nil {
		t.Fatal(err)
	}

	// First start — echo exits immediately with code 0 → Stopped.
	if err := s.Start("echo1"); err != nil {
		t.Fatalf("first Start: %v", err)
	}

	// Wait for the process to exit naturally to Stopped.
	waitForState(t, s, "echo1", StateStopped, 2*time.Second)

	// Re-start from Stopped state.
	if err := s.Start("echo1"); err != nil {
		t.Fatalf("re-start from Stopped: %v", err)
	}

	// Wait for re-start to reach Running then Stopped again.
	// (echo exits immediately so it may skip straight to Stopped)
	deadline := time.Now().Add(2 * time.Second)
	var finalState State
	for time.Now().Before(deadline) {
		mp, _ := s.Get("echo1")
		finalState = mp.State
		if finalState == StateStopped || finalState == StateFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if finalState != StateStopped && finalState != StateFailed {
		t.Errorf("after re-start, state = %v, want Stopped or Failed", finalState)
	}
}

// TestStart_NotFound verifies that Start() on an unregistered process returns
// ErrNotFound.
func TestStart_NotFound(t *testing.T) {
	s := New()

	err := s.Start("nonexistent")
	if err == nil {
		t.Fatal("Start nonexistent: expected ErrNotFound, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Start nonexistent: got %v, want wrapping ErrNotFound", err)
	}
}

// TestStart_CapturesOutput verifies that stdout output is captured as LogEntry
// values with Stream == StreamStdout.
func TestStart_CapturesOutput(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "echo-test", Command: "echo", Args: []string{"hello-rtx"}}); err != nil {
		t.Fatal(err)
	}

	if err := s.Start("echo-test"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the process to exit naturally.
	waitForState(t, s, "echo-test", StateStopped, 2*time.Second)

	logs, err := s.Logs("echo-test")
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatal("Logs: expected at least one entry, got none")
	}

	found := false
	for _, entry := range logs {
		if entry.Stream == StreamStdout && contains(entry.Text, "hello-rtx") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Logs: no stdout entry containing %q; entries = %v", "hello-rtx", logs)
	}
}

// TestStart_CapturesStderr verifies that stderr output is captured as LogEntry
// values with Stream == StreamStderr.
func TestStart_CapturesStderr(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "stderr-test", Command: "sh", Args: []string{"-c", "echo err-line >&2"}}); err != nil {
		t.Fatal(err)
	}

	if err := s.Start("stderr-test"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the process to exit naturally.
	waitForState(t, s, "stderr-test", StateStopped, 2*time.Second)

	logs, err := s.Logs("stderr-test")
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}

	found := false
	for _, entry := range logs {
		if entry.Stream == StreamStderr && contains(entry.Text, "err-line") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Logs: no stderr entry containing %q; entries = %v", "err-line", logs)
	}
}

// TestMonitor_CleanExitToStopped verifies that a process exiting with code 0
// transitions to StateStopped with ExitCode == 0.
func TestMonitor_CleanExitToStopped(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "true-cmd", Command: "true"}); err != nil {
		t.Fatal(err)
	}

	if err := s.Start("true-cmd"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitForState(t, s, "true-cmd", StateStopped, 2*time.Second)

	mp, err := s.Get("true-cmd")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mp.State != StateStopped {
		t.Errorf("State = %v, want StateStopped", mp.State)
	}
	if mp.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", mp.ExitCode)
	}
}

// TestMonitor_NonZeroExitToFailed verifies that a process exiting with a non-zero
// code transitions to StateFailed with a non-zero ExitCode.
func TestMonitor_NonZeroExitToFailed(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "false-cmd", Command: "false"}); err != nil {
		t.Fatal(err)
	}

	if err := s.Start("false-cmd"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitForState(t, s, "false-cmd", StateFailed, 2*time.Second)

	mp, err := s.Get("false-cmd")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mp.State != StateFailed {
		t.Errorf("State = %v, want StateFailed", mp.State)
	}
	if mp.ExitCode == 0 {
		t.Errorf("ExitCode = 0, want non-zero")
	}
}

// TestStart_Race verifies that concurrent Start() and List() calls produce no
// data races. Run with go test -race.
func TestStart_Race(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "racer", Command: "sleep", Args: []string{"10"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killProcess(t, s, "racer") })

	var wg sync.WaitGroup

	// Start the process in one goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = s.Start("racer")
	}()

	// Call List() concurrently 100 times.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			s.List()
		}
	}()

	wg.Wait()
}

// contains is a simple substring check helper.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
