# Stack Research

**Domain:** Go process runner CLI (minimal, stdlib-only)
**Researched:** 2026-02-27
**Confidence:** HIGH — all recommendations verified against official Go documentation and pkg.go.dev

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `os/exec` | Go stdlib (1.25.x) | Spawn and manage child processes | The canonical Go API for subprocess execution. `exec.Command` / `exec.CommandContext` give you `Cmd.Start()` + `Cmd.Wait()` for non-blocking lifecycle control, direct `Stdout`/`Stderr` assignment for zero-copy streaming, and `SysProcAttr` for Linux-specific process attributes. No third-party wrapper adds value here. |
| `os/signal` | Go stdlib (1.25.x) | Intercept OS signals (SIGINT, SIGTERM) | The only Go stdlib way to catch asynchronous signals. `signal.Notify(ch, ...)` + buffered `chan os.Signal` is the canonical pattern. `signal.NotifyContext` (Go 1.16+) is the modern alternative when context propagation is needed. Both are official, stable, and zero-dependency. |
| `syscall` | Go stdlib (1.25.x) | Access Linux-specific process primitives | Required for `SysProcAttr` fields (`Setpgid`, `Pdeathsig`) and signal constants (`syscall.SIGTERM`, `syscall.SIGINT`, `syscall.SIGKILL`). Also used for `WaitStatus` when you need to distinguish between a normally-exited process and one killed by signal. Not cross-platform, but the project specifies Linux first-class. |
| `os` | Go stdlib (1.25.x) | Process handle, exit, stdin/stdout wiring | `os.Stdin`, `os.Stdout`, `os.Stderr` assigned to `cmd.Stdin/Stdout/Stderr` gives real-time streaming with no buffering. `os.Exit(code)` propagates child exit code to the calling shell. `cmd.Process` (type `*os.Process`) exposes `Signal()` for manual forwarding. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `flag` (stdlib) | Go stdlib | Parse `rtx run <cmd> [args...]` subcommand + flags | Use for the CLI entry point. `flag.Parse()` + `flag.Args()` correctly splits flags from positional args. `flag.Args()[0]` = subcommand, `flag.Args()[1:]` = command and its args. Stops at `--` for explicit pass-through. No external dep needed for a two-subcommand CLI. |
| `fmt` (stdlib) | Go stdlib | Minimal PID/signal/exit-code logging to stderr | Use `fmt.Fprintf(os.Stderr, ...)` for operator-facing messages. Keeps rtx output on stderr, leaving stdout clean for the child process. |
| `os/exec` `ExitError` | Go stdlib | Extract child exit code on failure | Type-assert `err.(*exec.ExitError)` after `cmd.Wait()`, then call `.ExitCode()`. Always check this before falling back to exit code 1. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `go test ./internal/process/...` | Unit test the process package | Use `exec.Command("true")` / `exec.Command("false")` as portable test fixtures. Test signal forwarding with `exec.Command("sleep", "10")` + `cmd.Process.Signal(syscall.SIGTERM)`. |
| `go build -o bin/rtx ./cmd/rtx` | Produce the rtx binary | Single-binary output; no install dependencies. Add to `Makefile` alongside existing targets. |
| `go vet ./...` | Catch common mistakes | Will catch goroutine leaks from missing `cmd.Wait()`, unchecked error returns on `Signal()`. |

---

## Installation

```bash
# No external dependencies for rtx — stdlib only.
# Existing go.mod requires no changes.
# Verify Go version:
go version  # Should be >= 1.25.x (project go.mod specifies go 1.25.5)

# Build rtx binary:
go build -o bin/rtx ./cmd/rtx

# Run tests for the new process package:
go test ./internal/process/...
```

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `os/exec` direct | `os.StartProcess` | Never for this use case. `StartProcess` is the low-level primitive that `exec.Command` wraps. `exec.Command` adds PATH resolution, `SysProcAttr` wiring, and the `Cmd` lifecycle methods. Only use `StartProcess` if you need byte-level control over argv that `exec.Command` doesn't expose. |
| `signal.Notify(ch, ...)` | `signal.NotifyContext` | Use `NotifyContext` if you need to plumb a context into `exec.CommandContext`. For rtx v0, which runs a single foreground process, the bare `Notify` + channel pattern is simpler and equally correct. `NotifyContext` shines in servers with multiple concurrent goroutines. |
| `flag` stdlib | `cobra` / `spf13/cobra` | Use Cobra when you have 5+ subcommands, persistent flags, shell completion, and a need for help text generation. For `rtx run <cmd> [args...]` — one subcommand, no persistent flags, stdlib-only constraint — Cobra adds ~20KB of transitive dependencies for zero functional gain. |
| `flag` stdlib | `urfave/cli` | Same rationale as Cobra. Lightweight, but still an external dependency that the spec explicitly prohibits. |
| Direct `cmd.Stdout = os.Stdout` | `StdoutPipe` + `bufio.Scanner` | Use `StdoutPipe` only when you need to process/transform output line-by-line before printing. For pass-through streaming (rtx requirement), `cmd.Stdout = os.Stdout` is zero-copy and has no buffering delay. `StdoutPipe` adds a goroutine and a pipe buffer that can introduce latency. |
| `fmt.Fprintf(os.Stderr, ...)` | `log` stdlib | `log` adds timestamps and a configurable prefix by default — appropriate for daemons, not for a CLI that wants "PID, signal, exit code only". `fmt.Fprintf` to stderr gives full control over output format with no overhead. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `cmd.Run()` for the main execution path | `Run()` is `Start()` + `Wait()` in sequence — it blocks the calling goroutine until the child exits, making it impossible to intercept signals and forward them to the child concurrently. You cannot do `cmd.Process.Signal(sig)` from another goroutine while `Run()` is holding the lock. | `cmd.Start()` + goroutine calling `cmd.Wait()` + `select` on the done channel and signal channel. |
| `cmd.Output()` / `cmd.CombinedOutput()` | These buffer all output into memory before returning. They prevent real-time streaming and will OOM on large output. They also block signal handling for the same reason as `Run()`. | `cmd.Stdout = os.Stdout` + `cmd.Stderr = os.Stderr` + `cmd.Start()` + `cmd.Wait()`. |
| `sh -c <command>` shell wrapping | The existing `runner.go` uses `exec.Command("sh", "-c", job.Command)` — this is correct for Docker jobs where the command is a shell string, but wrong for rtx. Shell wrapping: (1) interposes a shell process between rtx and the real command, breaking direct signal delivery to the target process; (2) creates an extra PID to track; (3) is a security anti-pattern when args come from user input. | `exec.Command(args[0], args[1:]...)` — direct exec, no shell. |
| `cmd.Process.Release()` instead of `Wait()` | `Release()` releases OS resources but does NOT reap the process. The child will remain a zombie in the process table until the parent exits. The Go docs explicitly state: "If Wait has not been called, Release should be called to release associated OS resources." — but that still leaves the zombie. | Always call `cmd.Wait()`. If you don't need the exit status, call `cmd.Wait()` in a goroutine and discard the error. |
| `syscall.Kill(pid, syscall.SIGKILL)` as first signal | SIGKILL cannot be caught, blocked, or ignored by the child — it gives the child zero chance to flush buffers, close files, or log. Sending SIGKILL first breaks correctness for any child that needs cleanup. | Send `syscall.SIGTERM` first. If a timeout elapses, escalate to SIGKILL. For rtx v0 (no timeout requirement), SIGTERM + Wait is sufficient. |
| External process management libraries (`go-reaper`, `supervisord` bindings) | These solve the PID 1 zombie reaping problem (where init reaping is absent, e.g. inside containers). rtx v0 is not PID 1 and does not need init-level reaping. These libraries add complexity and dependencies the spec prohibits. | `cmd.Wait()` is sufficient for a non-PID-1 process runner. |

---

## Stack Patterns by Variant

**If the child must be isolated from Ctrl+C (detached process group):**
- Set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`
- This prevents keyboard SIGINT from propagating to the child automatically
- You then control signal delivery explicitly: `cmd.Process.Signal(sig)`
- Use this when you want rtx to intercept Ctrl+C, log "received SIGINT", forward to child, then wait

**If the child should inherit Ctrl+C (same process group):**
- Do NOT set `Setpgid`
- The kernel delivers SIGINT to the whole process group (rtx + child) simultaneously
- Signal forwarding code becomes a no-op (child already got the signal)
- Simpler but less controllable — you cannot interpose your logging before the child exits

**Recommended for rtx v0:** Use `Setpgid: true` + explicit forwarding. This gives you the log line "received SIGINT, forwarding to PID X" before the child exits, which is the observable behavior the spec requires ("Minimal logging: PID, signal received, exit code only").

**If exit code propagation to the parent shell is required (it is):**
```go
// After cmd.Wait() returns:
exitCode := 0
if err != nil {
    if exitErr, ok := err.(*exec.ExitError); ok {
        exitCode = exitErr.ExitCode()
    } else {
        // Non-exit error (e.g., command not found)
        fmt.Fprintf(os.Stderr, "rtx: error: %v\n", err)
        exitCode = 1
    }
}
os.Exit(exitCode)
```
Note: `os.Exit()` skips deferred functions. Ensure all cleanup (signal.Stop, log flush) happens before this call.

---

## Version Compatibility

| Package | Go Version Required | Notes |
|---------|---------------------|-------|
| `os/exec` (core API) | Go 1.0+ | Stable; no breaking changes in scope |
| `exec.Cmd.Cancel` field | Go 1.20+ | For custom context cancellation behavior — not needed for v0 |
| `exec.Cmd.WaitDelay` field | Go 1.20+ | For timeout after context cancel — not needed for v0 |
| `signal.NotifyContext` | Go 1.16+ | Available; project uses Go 1.25.5 |
| `os.ProcessState.ExitCode()` | Go 1.12+ | Use this, not `Sys().(syscall.WaitStatus).ExitStatus()` |
| `syscall.SysProcAttr.Setpgid` | Linux (all versions) | Linux-specific; not available on Windows |
| `syscall.SysProcAttr.Pdeathsig` | Linux-only | Sends signal to child if parent dies; useful but optional for v0 |

---

## Key Implementation Pattern (Canonical rtx v0 Loop)

This is the complete, verified pattern for `internal/process/runner.go`:

```go
package process

import (
    "fmt"
    "os"
    "os/exec"
    "os/signal"
    "syscall"
)

// Run spawns cmd with args, streams stdout/stderr in real time, forwards
// SIGINT and SIGTERM to the child, waits for exit, and returns the exit code.
func Run(name string, args []string) int {
    cmd := exec.Command(name, args...)
    cmd.Stdin  = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    // Detach child into its own process group so we control signal delivery.
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    if err := cmd.Start(); err != nil {
        fmt.Fprintf(os.Stderr, "rtx: failed to start: %v\n", err)
        return 1
    }
    fmt.Fprintf(os.Stderr, "rtx: started PID %d\n", cmd.Process.Pid)

    // Catch SIGINT and SIGTERM; buffer=1 so the sender never blocks.
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    defer signal.Stop(sigCh)

    // Wait for child exit in a goroutine so we can also select on signals.
    doneCh := make(chan error, 1)
    go func() { doneCh <- cmd.Wait() }()

    var waitErr error
    select {
    case sig := <-sigCh:
        fmt.Fprintf(os.Stderr, "rtx: received %s, forwarding to PID %d\n", sig, cmd.Process.Pid)
        cmd.Process.Signal(sig) // forward; child may or may not handle it
        waitErr = <-doneCh     // always wait — zombie prevention
    case waitErr = <-doneCh:
        // child exited on its own
    }

    if waitErr == nil {
        fmt.Fprintf(os.Stderr, "rtx: exit code 0\n")
        return 0
    }
    if exitErr, ok := waitErr.(*exec.ExitError); ok {
        code := exitErr.ExitCode()
        fmt.Fprintf(os.Stderr, "rtx: exit code %d\n", code)
        return code
    }
    fmt.Fprintf(os.Stderr, "rtx: wait error: %v\n", waitErr)
    return 1
}
```

---

## Sources

- `https://pkg.go.dev/os/exec` — Official Go stdlib docs; `Cmd` struct fields, lifecycle methods, signal handling, zombie prevention. Confidence: HIGH.
- `https://pkg.go.dev/os/signal` — Official Go stdlib docs; `Notify`, `Stop`, `NotifyContext` signatures and behavior. Confidence: HIGH.
- `https://pkg.go.dev/os#Process` — Official Go stdlib docs; `Process.Signal`, `Process.Wait`, `ProcessState.ExitCode`. Confidence: HIGH.
- `https://pkg.go.dev/syscall` — Official Go stdlib docs; `SysProcAttr`, `WaitStatus`, `Kill`. Confidence: HIGH.
- `https://pkg.go.dev/flag` — Official Go stdlib docs; `flag.Args()`, `flag.Arg(i)`, FlagSet for subcommands. Confidence: HIGH.
- `https://go.dev/doc/go1.25` — Go 1.25 release notes; confirmed no breaking changes to os/exec, os/signal, syscall in this release. Confidence: HIGH.
- `https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/` — Practical os/exec patterns including process group isolation, signal handling. Confidence: MEDIUM (blog, but aligns with official docs).
- `https://calmops.com/programming/golang/go-process-management-subprocess/` — Signal forwarding pattern with `select` on signal channel and done channel. Confidence: MEDIUM (verified against official docs).
- `https://victoriametrics.com/blog/go-graceful-shutdown/` — Graceful shutdown patterns; `signal.NotifyContext` vs `signal.Notify` tradeoffs. Confidence: MEDIUM.
- `https://mezhenskyi.dev/posts/go-linux-processes/` — Linux process management in Go; zombie prevention, `Setpgid` usage. Confidence: MEDIUM.

---

*Stack research for: rtx process runner CLI (Go stdlib only)*
*Researched: 2026-02-27*
