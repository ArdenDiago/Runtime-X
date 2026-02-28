package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Run spawns name with args, streams stdout/stderr in real time via direct fd
// inheritance, waits for the child to exit, and returns its exact exit code.
// Phase 1: no signal forwarding. Ctrl+C with Setpgid=true may orphan child
// until Phase 2 adds explicit forwarding.
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

	// PROC-01/PROC-04: doneCh pattern ensures cmd.Wait() is called on every
	// code path. Phase 2 will add a signal case to the select without
	// restructuring this code.
	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()

	waitErr := <-doneCh

	code := resolveExitCode(waitErr)
	// LOG-03: log exit code before returning
	fmt.Fprintf(os.Stderr, "[rtx] exited with code %d\n", code)
	return code
}

// resolveExitCode extracts the exact integer exit code from cmd.Wait()'s
// error. Returns 0 for clean exit, the child's exit code for ExitError,
// or 1 for unexpected wait failures.
func resolveExitCode(err error) int {
	if err == nil {
		return 0
	}
	// EXIT-01: type-assert to *exec.ExitError to get exact code
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	// ERR-02: non-ExitError (I/O error etc.) — log and return 1
	fmt.Fprintf(os.Stderr, "[rtx] wait error: %v\n", err)
	return 1
}
