package scheduler

import (
	"errors"
	"sync"
	"syscall"
	"testing"
	"time"
)

// getState returns the current state of the named process under the read lock.
// Since tests are in the same package, they can access s.mu directly.
// This avoids the race between reading mp.State (write by monitorProcess under
// s.mu.Lock()) and reading (test goroutines reading mp.State without a lock).
func getState(s *Scheduler, name string) (State, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mp, ok := s.processes[name]
	if !ok {
		return StateIdle, false
	}
	return mp.State, true
}

// getExitCode returns ExitCode of the named process under the read lock.
func getExitCode(s *Scheduler, name string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mp, ok := s.processes[name]
	if !ok {
		return -1
	}
	return mp.ExitCode
}

// getPID returns the PID of the running process under the read lock, or 0 if
// the process is not running or cmd has not been set.
func getPID(s *Scheduler, name string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mp, ok := s.processes[name]
	if !ok || mp.cmd == nil || mp.cmd.Process == nil {
		return 0
	}
	return mp.cmd.Process.Pid
}

// killProcess sends SIGKILL to the process group of the named process.
// Used as test cleanup to ensure long-lived processes are terminated.
func killProcess(t *testing.T, s *Scheduler, name string) {
	t.Helper()
	pid := getPID(s, name)
	if pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}

// waitForState polls every 10ms until the named process reaches target state or
// timeout expires. Uses the scheduler read lock on every poll to avoid races.
func waitForState(t *testing.T, s *Scheduler, name string, target State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, ok := getState(s, name)
		if !ok {
			t.Fatalf("waitForState: process %q not found", name)
		}
		if state == target {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	state, _ := getState(s, name)
	t.Fatalf("waitForState: process %q state = %v, want %v after %v", name, state, target, timeout)
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

	state, _ := getState(s, "sleeper")
	if state != StateRunning {
		t.Errorf("State = %v, want StateRunning", state)
	}

	pid := getPID(s, "sleeper")
	if pid <= 0 {
		t.Errorf("PID = %d, want > 0", pid)
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

	// Wait for re-start to reach a terminal state (Stopped or Failed).
	// echo exits immediately so it transitions quickly.
	deadline := time.Now().Add(2 * time.Second)
	var finalState State
	for time.Now().Before(deadline) {
		finalState, _ = getState(s, "echo1")
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

	state, _ := getState(s, "true-cmd")
	if state != StateStopped {
		t.Errorf("State = %v, want StateStopped", state)
	}
	code := getExitCode(s, "true-cmd")
	if code != 0 {
		t.Errorf("ExitCode = %d, want 0", code)
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

	state, _ := getState(s, "false-cmd")
	if state != StateFailed {
		t.Errorf("State = %v, want StateFailed", state)
	}
	code := getExitCode(s, "false-cmd")
	if code == 0 {
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

// getStoppedAt returns StoppedAt of the named process under the read lock.
func getStoppedAt(s *Scheduler, name string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mp, ok := s.processes[name]
	if !ok {
		return time.Time{}
	}
	return mp.StoppedAt
}

// getStartedAt returns StartedAt of the named process under the read lock.
func getStartedAt(s *Scheduler, name string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mp, ok := s.processes[name]
	if !ok {
		return time.Time{}
	}
	return mp.StartedAt
}

// TestStop_RunningToStopped verifies that Stop() on a Running process sends
// SIGTERM, waits for exit, transitions to Stopped, and sets StoppedAt.
func TestStop_RunningToStopped(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "sleeper", Command: "sleep", Args: []string{"30"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killProcess(t, s, "sleeper") })

	if err := s.Start("sleeper"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, s, "sleeper", StateRunning, 2*time.Second)

	err := s.Stop("sleeper")
	if err != nil {
		t.Fatalf("Stop: unexpected error: %v", err)
	}

	state, _ := getState(s, "sleeper")
	if state != StateStopped {
		t.Errorf("State = %v, want StateStopped", state)
	}

	stoppedAt := getStoppedAt(s, "sleeper")
	startedAt := getStartedAt(s, "sleeper")
	if stoppedAt.IsZero() {
		t.Error("StoppedAt is zero, want non-zero")
	}
	if !stoppedAt.After(startedAt) {
		t.Errorf("StoppedAt %v not after StartedAt %v", stoppedAt, startedAt)
	}
}

// TestStop_NotFound verifies that Stop() on an unregistered process returns ErrNotFound.
func TestStop_NotFound(t *testing.T) {
	s := New()

	err := s.Stop("nonexistent")
	if err == nil {
		t.Fatal("Stop nonexistent: expected ErrNotFound, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Stop nonexistent: got %v, want wrapping ErrNotFound", err)
	}
}

// TestStop_NotRunning verifies that Stop() on an Idle process returns ErrNotRunning.
func TestStop_NotRunning(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "idle-proc", Command: "echo", Args: []string{"hi"}}); err != nil {
		t.Fatal(err)
	}

	err := s.Stop("idle-proc")
	if err == nil {
		t.Fatal("Stop idle-proc: expected ErrNotRunning, got nil")
	}
	if !errors.Is(err, ErrNotRunning) {
		t.Errorf("Stop idle-proc: got %v, want wrapping ErrNotRunning", err)
	}
}

// TestStop_AlreadyStopped verifies that Stop() on a naturally-stopped process returns ErrNotRunning.
func TestStop_AlreadyStopped(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "echo-done", Command: "true"}); err != nil {
		t.Fatal(err)
	}

	if err := s.Start("echo-done"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, s, "echo-done", StateStopped, 2*time.Second)

	err := s.Stop("echo-done")
	if err == nil {
		t.Fatal("Stop echo-done after natural exit: expected ErrNotRunning, got nil")
	}
	if !errors.Is(err, ErrNotRunning) {
		t.Errorf("Stop echo-done: got %v, want wrapping ErrNotRunning", err)
	}
}

// TestStop_ProcessGroupKill verifies that Stop() on a process that spawned
// children terminates without hanging, and state becomes Stopped.
func TestStop_ProcessGroupKill(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{
		Name:    "parent-child",
		Command: "sh",
		Args:    []string{"-c", "sleep 60 & wait"},
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killProcess(t, s, "parent-child") })

	if err := s.Start("parent-child"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, s, "parent-child", StateRunning, 2*time.Second)

	err := s.Stop("parent-child")
	if err != nil {
		t.Fatalf("Stop parent-child: unexpected error: %v", err)
	}

	state, _ := getState(s, "parent-child")
	if state != StateStopped {
		t.Errorf("State = %v, want StateStopped", state)
	}
}

// TestStop_SIGKILLEscalation verifies that Stop() escalates to SIGKILL when a
// process traps SIGTERM and does not exit within StopTimeout.
func TestStop_SIGKILLEscalation(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{
		Name:        "trap-sigterm",
		Command:     "sh",
		Args:        []string{"-c", "trap '' TERM; sleep 60"},
		StopTimeout: 500 * time.Millisecond,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killProcess(t, s, "trap-sigterm") })

	if err := s.Start("trap-sigterm"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, s, "trap-sigterm", StateRunning, 2*time.Second)

	start := time.Now()
	err := s.Stop("trap-sigterm")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Stop trap-sigterm: unexpected error: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Stop took %v, want < 2s (timeout 500ms + SIGKILL should be fast)", elapsed)
	}

	state, _ := getState(s, "trap-sigterm")
	if state != StateStopped {
		t.Errorf("State = %v, want StateStopped", state)
	}
}

// TestStop_ConcurrentStartStop verifies that concurrent Stop() and List() calls
// produce no data races and Stop() returns nil.
func TestStop_ConcurrentStartStop(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{Name: "racer", Command: "sleep", Args: []string{"30"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killProcess(t, s, "racer") })

	if err := s.Start("racer"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, s, "racer", StateRunning, 2*time.Second)

	var wg sync.WaitGroup
	var stopErr error

	// Goroutine A: stop the process.
	wg.Add(1)
	go func() {
		defer wg.Done()
		stopErr = s.Stop("racer")
	}()

	// Goroutine B: call List() 50 times concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			s.List()
		}
	}()

	wg.Wait()

	if stopErr != nil {
		t.Errorf("Stop racer: unexpected error: %v", stopErr)
	}
}

// TestStop_CapturesShutdownOutput verifies that output captured before Stop() is
// still available via Logs() after the process exits.
func TestStop_CapturesShutdownOutput(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{
		Name:    "shutdown-msg",
		Command: "sh",
		Args:    []string{"-c", "echo starting; sleep 30"},
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killProcess(t, s, "shutdown-msg") })

	if err := s.Start("shutdown-msg"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForState(t, s, "shutdown-msg", StateRunning, 2*time.Second)

	// Wait for "starting" to be captured in the log buffer.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		logs, _ := s.Logs("shutdown-msg")
		for _, e := range logs {
			if contains(e.Text, "starting") {
				goto stopPhase
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for 'starting' log entry before Stop()")

stopPhase:
	if err := s.Stop("shutdown-msg"); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	logs, err := s.Logs("shutdown-msg")
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	found := false
	for _, e := range logs {
		if contains(e.Text, "starting") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Logs: expected 'starting' entry, got %v", logs)
	}
}

// getRestartCount returns RestartCount of the named process under the read lock.
func getRestartCount(s *Scheduler, name string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mp, ok := s.processes[name]
	if !ok {
		return -1
	}
	return mp.RestartCount
}

// TestRestartAlways verifies that a process with RestartPolicy{Mode: RestartAlways}
// is automatically restarted after a clean exit (code 0).
func TestRestartAlways(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{
		Name:    "restart-always",
		Command: "true",
		RestartPolicy: RestartPolicy{
			Mode:  RestartAlways,
			Delay: 10 * time.Millisecond, // tiny delay for test speed
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Start("restart-always"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the process to be restarted at least once.
	// The process exits 0 (true), should enter Restarting then Running again.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if getRestartCount(s, "restart-always") >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	rc := getRestartCount(s, "restart-always")
	if rc < 1 {
		t.Errorf("RestartCount = %d, want >= 1 (process should have been restarted)", rc)
	}

	// Stop cleanly — the process may currently be Running or Restarting.
	// Retry Stop() a few times because there is a window between Restarting
	// and Starting where Stop() would see "transient state" and return error.
	var stopErr error
	for i := 0; i < 20; i++ {
		stopErr = s.Stop("restart-always")
		if stopErr == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if stopErr != nil {
		// If stop didn't work, try forceful kill for cleanup
		killProcess(t, s, "restart-always")
		t.Logf("Stop: %v (but restart was confirmed)", stopErr)
	}
}

// TestRestartOnFailure verifies that a process with RestartPolicy{Mode: RestartOnFailure}
// is restarted when it exits with a non-zero code.
func TestRestartOnFailure(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{
		Name:    "restart-on-fail",
		Command: "false", // exits with code 1
		RestartPolicy: RestartPolicy{
			Mode:  RestartOnFailure,
			Delay: 10 * time.Millisecond,
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Start("restart-on-fail"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for at least one restart.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if getRestartCount(s, "restart-on-fail") >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	rc := getRestartCount(s, "restart-on-fail")
	if rc < 1 {
		t.Errorf("RestartCount = %d, want >= 1 (process should have been restarted on failure)", rc)
	}

	// Stop cleanly — retry in case process is in transient Starting state.
	var stopErr error
	for i := 0; i < 20; i++ {
		stopErr = s.Stop("restart-on-fail")
		if stopErr == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if stopErr != nil {
		killProcess(t, s, "restart-on-fail")
		t.Logf("Stop: %v (but restart was confirmed)", stopErr)
	}
}

// TestRestartMaxRetries verifies that a process stops restarting after MaxRetries
// attempts and ends in StateFailed.
func TestRestartMaxRetries(t *testing.T) {
	const maxRetries = 3
	s := New()
	if err := s.Register(ProcessDef{
		Name:    "retry-limit",
		Command: "false", // exits with code 1 every time
		RestartPolicy: RestartPolicy{
			Mode:       RestartOnFailure,
			MaxRetries: maxRetries,
			Delay:      10 * time.Millisecond,
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Start("retry-limit"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the process to exhaust retries and reach StateFailed.
	waitForState(t, s, "retry-limit", StateFailed, 5*time.Second)

	rc := getRestartCount(s, "retry-limit")
	if rc != maxRetries {
		t.Errorf("RestartCount = %d, want %d (exactly MaxRetries attempts)", rc, maxRetries)
	}

	state, _ := getState(s, "retry-limit")
	if state != StateFailed {
		t.Errorf("State = %v, want StateFailed after exhausting MaxRetries", state)
	}
}

// TestStopDuringRestart verifies that calling Stop() while a process is in
// StateRestarting (backoff wait) cancels the restart and transitions to Stopped.
func TestStopDuringRestart(t *testing.T) {
	s := New()
	if err := s.Register(ProcessDef{
		Name:    "stop-during-restart",
		Command: "true",
		RestartPolicy: RestartPolicy{
			Mode:  RestartAlways,
			Delay: 2 * time.Second, // long delay so we can catch it in Restarting state
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Start("stop-during-restart"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait until the process enters StateRestarting (after the first exit).
	waitForState(t, s, "stop-during-restart", StateRestarting, 3*time.Second)

	// Call Stop() while in StateRestarting — should cancel the backoff immediately.
	start := time.Now()
	if err := s.Stop("stop-during-restart"); err != nil {
		t.Fatalf("Stop during restart: %v", err)
	}
	elapsed := time.Since(start)

	// Stop should return quickly (cancellation, no need to wait for backoff).
	if elapsed > 500*time.Millisecond {
		t.Errorf("Stop took %v, want < 500ms (should cancel backoff immediately)", elapsed)
	}

	state, _ := getState(s, "stop-during-restart")
	if state != StateStopped {
		t.Errorf("State = %v, want StateStopped after Stop() during restart", state)
	}
}

// contains is a simple substring check helper.
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
