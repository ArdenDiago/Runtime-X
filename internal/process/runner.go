package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Run spawns name with args, streams stdout/stderr in real time via direct fd
// inheritance, intercepts SIGINT/SIGTERM and forwards them to the child, waits
// for the child to exit, and returns its exact POSIX exit code (128+N for
// signal-killed processes).
func Run(name string, args []string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// PROC-05: isolate child in its own process group so Phase 2 can forward
	// signals explicitly without relying on kernel's automatic delivery.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		// ERR-01: command not found — use errors.Is not string matching
		if errors.Is(err, exec.ErrNotFound) {
			fmt.Fprintf(os.Stderr, "[rtx] command not found: %s\n", name)
			return 127
		}
		fmt.Fprintf(os.Stderr, "[rtx] failed to start: %v\n", err)
		return 1
	}
	// LOG-01: log PID immediately after successful Start()
	fmt.Fprintf(os.Stderr, "[rtx] spawned PID %d\n", cmd.Process.Pid)

	// SIG-04: buffered capacity 1 — signal.Notify does a non-blocking send.
	// signal.Notify MUST be called AFTER cmd.Start() succeeds — before that,
	// there is no child to forward to.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM) // SIG-01, SIG-02
	defer signal.Stop(sigCh)

	// PROC-01/PROC-04: doneCh pattern ensures cmd.Wait() is called on every
	// code path, preventing zombie processes.
	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()

	var waitErr error
	select {
	case sig := <-sigCh:
		// LOG-02: log signal receipt before forwarding
		fmt.Fprintf(os.Stderr, "[rtx] received signal %s\n", sig)
		// SIG-03 + ERR-03: forward signal; swallow "already finished" so a
		// natural-exit race does not produce a spurious error log.
		if err := cmd.Process.Signal(sig); err != nil && !errors.Is(err, os.ErrProcessDone) {
			fmt.Fprintf(os.Stderr, "[rtx] signal forward failed: %v\n", err)
		}
		waitErr = <-doneCh // SIG-03: always wait — zombie prevention intact
	case waitErr = <-doneCh:
		// natural exit — no signal handling needed
	}

	code := resolveExitCode(waitErr, cmd.ProcessState)
	// LOG-03: log exit code before returning
	fmt.Fprintf(os.Stderr, "[rtx] exited with code %d\n", code)
	return code
}

// resolveExitCode extracts the exact POSIX exit code from cmd.Wait()'s error.
// Returns 0 for clean exit, 128+N for signal-killed children (EXIT-03),
// the child's exit code for normal non-zero exit, or 1 for infrastructure failures.
func resolveExitCode(err error, state *os.ProcessState) int {
	if err == nil {
		return 0
	}
	// EXIT-01: type-assert to *exec.ExitError to get exact code
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		// EXIT-03: ExitCode() returns -1 for signal-killed processes.
		// WaitStatus.Signaled() detects this and provides the actual signal
		// number so we can compute the POSIX-correct 128+N exit code.
		if code == -1 && state != nil {
			if ws, ok := state.Sys().(syscall.WaitStatus); ok {
				if ws.Signaled() {
					return 128 + int(ws.Signal())
				}
			}
		}
		return code
	}
	// ERR-02: non-ExitError (I/O error etc.) — log and return 1
	fmt.Fprintf(os.Stderr, "[rtx] wait error: %v\n", err)
	return 1
}
