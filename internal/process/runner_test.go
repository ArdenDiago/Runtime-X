package process

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestHelperProcess is the subprocess entry point for signal and zombie tests.
// It only activates when RTX_TEST_HELPER=1 is set in the environment.
// In normal test runs it returns immediately — zero overhead, no t.Skip() noise.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("RTX_TEST_HELPER") != "1" {
		return
	}
	// Parse args after the "--" separator.
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "[rtx-helper] no command args provided")
		os.Exit(1)
	}
	// Delegate to the real process.Run — this is what the tests exercise.
	os.Exit(Run(args[0], args[1:]))
}

// TestRunExitCodes verifies exit code propagation for TEST-01, TEST-02, TEST-05.
// Calls process.Run() directly (same package — no subprocess indirection needed).
func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     []string
		wantCode int
	}{
		// TEST-01: `false` always exits with code 1.
		{"false exits 1", "false", nil, 1},
		// TEST-02: sh propagates the exact exit code from `exit 42`.
		{"exit 42", "sh", []string{"-c", "exit 42"}, 42},
		// TEST-05: a nonexistent command returns 127 (command not found).
		{"command not found", "nonexistent-rtx-test-xyz", nil, 127},
	}

	for _, tt := range tests {
		tt := tt // capture range var
		t.Run(tt.name, func(t *testing.T) {
			got := Run(tt.command, tt.args)
			if got != tt.wantCode {
				t.Errorf("Run(%q, %v) = %d, want %d", tt.command, tt.args, got, tt.wantCode)
			}
		})
	}
}

// TestZombiePrevention verifies that process.Run() reaps its child before
// returning, leaving no zombie processes (TEST-03).
//
// Strategy: spawn the test binary in helper mode running "true" (exits
// immediately). Capture stderr to extract the child PID from the
// "[rtx] spawned PID <n>" log line. After cmd.Run() returns, inspect
// /proc/<pid>/status — file missing means reaped (PASS), file present with
// non-Z state also means PASS.
func TestZombiePrevention(t *testing.T) {
	// Spawn test binary in helper mode; let it run "true" and exit.
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "true")
	cmd.Env = append(os.Environ(), "RTX_TEST_HELPER=1")
	var helperStderr bytes.Buffer
	cmd.Stderr = &helperStderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("helper subprocess failed: %v (stderr: %s)", err, helperStderr.String())
	}

	// Extract the PID of the grandchild ("true") from the rtx log.
	pid := extractSpawnedPID(t, helperStderr.String())

	// After Run() returns, the child must have been reaped by cmd.Wait().
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	data, err := os.ReadFile(statusPath)
	if err != nil {
		// /proc/<pid>/status is gone entirely — process was reaped. PASS.
		return
	}
	// File still exists — verify the process is not in zombie state (State: Z).
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "State:") {
			if strings.Contains(line, "Z") {
				t.Errorf("PID %d is still a zombie after Run() returned: %q", pid, line)
			}
			return
		}
	}
}

// extractSpawnedPID parses the child PID from the "[rtx] spawned PID <n>" log
// line that process.Run() writes to stderr.
func extractSpawnedPID(t *testing.T, stderr string) int {
	t.Helper()
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "spawned PID") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "PID" && i+1 < len(fields) {
					pid, err := strconv.Atoi(fields[i+1])
					if err == nil {
						return pid
					}
				}
			}
		}
	}
	t.Fatalf("could not parse spawned PID from helper stderr:\n%s", stderr)
	return 0
}

// TestSignalDelivery verifies that process.Run() forwards SIGTERM to its child
// and that the exit code is 143 (128 + SIGTERM(15)) (TEST-04).
//
// Strategy: spawn the test binary in helper mode running "sleep 10" (a process
// that outlives the signal). After 200ms (enough for process.Run() to call
// signal.Notify), send SIGTERM to the helper. process.Run() forwards it to
// sleep; sleep exits on signal; process.Run() computes 128+15=143 and the
// helper subprocess exits with that code.
func TestSignalDelivery(t *testing.T) {
	// Spawn test binary in helper mode running "sleep 10".
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "sleep", "10")
	cmd.Env = append(os.Environ(), "RTX_TEST_HELPER=1")
	// Signal test does not need to capture output.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper subprocess: %v", err)
	}

	// Allow process.Run() inside the helper to call cmd.Start() and
	// signal.Notify before we send the signal. 200ms is conservative but
	// reliable for a local re-exec.
	time.Sleep(200 * time.Millisecond)

	// Send SIGTERM to the helper process; process.Run() will forward it to
	// the "sleep" grandchild via cmd.Process.Signal().
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM to helper: %v", err)
	}

	// Wait for the helper to exit.
	err := cmd.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError after SIGTERM, got %T: %v", err, err)
	}

	const wantCode = 143 // 128 + SIGTERM(15)
	if got := exitErr.ExitCode(); got != wantCode {
		t.Errorf("signal exit code = %d, want %d (128+SIGTERM)", got, wantCode)
	}
}
