# Phase 1: Process Foundation - Research

**Researched:** 2026-02-28
**Domain:** Go stdlib process execution — os/exec, os, syscall, flag (Linux-first, no external deps)
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**CLI invocation design:**
- Subcommand pattern: `rtx run <command> [args...]`
- Everything after `run` is the child command and its arguments
- rtx-level flags go before `run`: `rtx -v run sleep 10`
- Support short and long flag forms: `-v` / `--verbose`
- `rtx run` with no command is an error

### Claude's Discretion

- No-args behavior (usage help vs error message) — pick the standard CLI approach
- `--version` and `--help` flags — include if standard, skip if not worth v0 effort
- Log format and prefix style (`[rtx]` prefix, formatting details)
- Error message wording for "command not found" and other failures
- Process group isolation decision (`Setpgid: true` vs default) — research recommends Setpgid: true

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-01 | User can run `rtx run <command> [args...]` to spawn a child process | `flag.NewFlagSet` global-flags-first + switch on subcommand; `flag.Args()` gives the subcommand and everything after it |
| CLI-02 | User sees PID displayed immediately after process spawns | `fmt.Fprintf(os.Stderr, "[rtx] spawned PID %d\n", cmd.Process.Pid)` after `cmd.Start()` returns nil |
| PROC-01 | Child process is spawned via `cmd.Start()` (not `cmd.Run()`) to allow concurrent signal handling | `cmd.Start()` returns immediately with `cmd.Process` set; `cmd.Run()` blocks and prevents signal interposition |
| PROC-02 | Child process stdout streams to parent stdout in real-time (direct fd assignment, no buffering) | `cmd.Stdout = os.Stdout` — zero-copy direct file descriptor inheritance; no goroutine, no buffering |
| PROC-03 | Child process stderr streams to parent stderr in real-time (direct fd assignment, no buffering) | `cmd.Stderr = os.Stderr` — same mechanism; also assign `cmd.Stdin = os.Stdin` for interactive commands |
| PROC-04 | Child process is always reaped via `cmd.Wait()` on every code path — no zombie processes | `doneCh := make(chan error, 1); go func() { doneCh <- cmd.Wait() }()` — select always drains doneCh |
| PROC-05 | Child process runs in its own process group (`Setpgid: true`) for clean signal interposition | `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` — Linux-only; set before `cmd.Start()` |
| EXIT-01 | Parent captures child's exact exit code via `ExitError.ExitCode()` | Type-assert `waitErr` as `*exec.ExitError`; call `.ExitCode()` — returns exact integer, not 0/1 |
| EXIT-02 | Parent exits with child's exact exit code via `os.Exit(code)` | Return int from inner function; `main()` calls `os.Exit(run())` to prevent defer-skip issue |
| ERR-01 | "Command not found" produces clear error message and exits with code 127 | `cmd.Start()` returns error wrapping `exec.ErrNotFound`; detect with `errors.Is(err, exec.ErrNotFound)`, emit message, return 127 |
| ERR-02 | Child that crashes immediately has its exit code propagated as-is | `cmd.Wait()` returns `*exec.ExitError`; `ExitCode()` gives the exact code — same path as normal exit |
| LOG-01 | Minimal logging to stderr: `[rtx] spawned PID %d` on start | `fmt.Fprintf(os.Stderr, "[rtx] spawned PID %d\n", cmd.Process.Pid)` after successful `cmd.Start()` |
| LOG-03 | Minimal logging to stderr: `[rtx] exited with code %d` on exit | `fmt.Fprintf(os.Stderr, "[rtx] exited with code %d\n", code)` before returning from runner |
</phase_requirements>

---

## Summary

Phase 1 delivers a working `rtx run <command> [args...]` binary that correctly spawns a child process, streams its stdout/stderr in real time with zero buffering, captures the child's exact exit code, propagates it to the parent shell via `os.Exit()`, and prevents zombie processes by calling `cmd.Wait()` on every code path. Signal forwarding is explicitly out of scope — that is Phase 2. The kernel will deliver Ctrl+C to both `rtx` and the child simultaneously (same process group default), which is acceptable for this phase because the child will die and `rtx` will wait for it and exit correctly.

The entire implementation requires zero external dependencies. The project's `go.mod` specifies Go 1.25.5 with the only dep being `github.com/google/uuid` (for existing services). No changes to `go.mod` are needed. The critical design choices for Phase 1 are: (1) use `cmd.Start()` + goroutine + `cmd.Wait()` split pattern — never `cmd.Run()`, (2) assign `cmd.Stdout = os.Stdout` and `cmd.Stderr = os.Stderr` directly — never `StdoutPipe()`/`StderrPipe()`, (3) return int from an inner `run()` function in `main()` and call `os.Exit()` on the result — never `os.Exit()` inside the runner function where deferred `signal.Stop()` would be skipped.

Note: PROC-05 (`Setpgid: true`) is listed as a Phase 1 requirement in the traceability table. For Phase 1 (no signal forwarding yet), setting `Setpgid: true` means the child will NOT receive the kernel's automatic Ctrl+C — it will only receive signals explicitly forwarded by the runner, which won't happen until Phase 2. The recommendation is to set `Setpgid: true` from Phase 1 to avoid restructuring in Phase 2, but be aware that Ctrl+C will terminate the runner (which exits) without the child receiving a signal — the child becomes an orphan until the OS's orphan reaping runs. This is acceptable for Phase 1 since signal correctness is Phase 2's goal.

**Primary recommendation:** Build `internal/process/runner.go` first with the canonical Start/Wait/doneCh pattern, then build `cmd/rtx/main.go` as a thin CLI wrapper that delegates to it and calls `os.Exit()` on the returned code.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os/exec` | Go stdlib (1.25.5) | Spawn and manage child processes | Official Go API for subprocess execution; provides `Cmd.Start()`, `Cmd.Wait()`, direct `Stdout`/`Stderr` assignment, and `SysProcAttr` for Linux-specific attributes |
| `os` | Go stdlib (1.25.5) | File descriptors, exit, process handle | `os.Stdout`, `os.Stderr`, `os.Stdin` for direct fd inheritance; `os.Exit(code)` for exit code propagation; `os.ErrProcessDone` for dead-process detection |
| `syscall` | Go stdlib (1.25.5) | Linux-specific process group control | `SysProcAttr{Setpgid: true}` to isolate child into new process group; signal constants `syscall.SIGINT`, `syscall.SIGTERM` |
| `fmt` | Go stdlib (1.25.5) | Stderr logging | `fmt.Fprintf(os.Stderr, ...)` for operator messages — keeps stdout clean for child output |
| `errors` | Go stdlib (1.25.5) | Error type inspection | `errors.Is(err, exec.ErrNotFound)` for command-not-found detection; `errors.As(err, &exitErr)` for exit code extraction |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `flag` | Go stdlib (1.25.5) | CLI argument parsing | Parse global flags (e.g., `-v`/`--verbose`) before subcommand; `flag.NewFlagSet` with `flag.ContinueOnError` for global flagset; switch on first positional arg as subcommand |
| `os/signal` | Go stdlib (1.25.5) | Signal channel (Phase 2 prep) | `signal.Stop(sigCh)` needed in defer even in Phase 1 if channel is set up; NOT used for forwarding yet in Phase 1 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `flag` stdlib | `cobra` / `urfave/cli` | External deps, 20KB+ transitive; unnecessary for one subcommand; spec prohibits external CLI deps |
| `cmd.Stdout = os.Stdout` | `StdoutPipe()` + goroutine | Pipe adds buffering, goroutine leak risk, race condition on Wait(); direct assignment is zero-copy and goroutine-free |
| `fmt.Fprintf(os.Stderr, ...)` | `log` stdlib | `log` adds timestamps by default; `fmt.Fprintf` gives exact output control for "PID, signal, exit code only" logging |
| `exec.Command(args[0], args[1:]...)` | `exec.Command("sh", "-c", userInput)` | Shell wrapping interposes extra process, breaks signal delivery, breaks PID display, is a security anti-pattern |

**Installation:**
```bash
# No external dependencies. go.mod requires no changes.
go build -o bin/rtx ./cmd/rtx
go test ./internal/process/...
go vet ./...
```

---

## Architecture Patterns

### Recommended Project Structure

```
Runtime-X/
├── cmd/
│   ├── main.go                    # Existing API server (DO NOT TOUCH)
│   ├── worker/
│   │   └── main.go               # Existing worker (DO NOT TOUCH)
│   └── rtx/
│       └── main.go               # NEW: CLI entry point (thin wrapper)
│
├── internal/
│   ├── core/                     # Existing (DO NOT TOUCH)
│   ├── queue/                    # Existing (DO NOT TOUCH)
│   ├── worker/                   # Existing (DO NOT TOUCH)
│   ├── docker/                   # Existing (DO NOT TOUCH)
│   ├── logging/                  # Existing (DO NOT TOUCH)
│   └── process/                  # NEW: process execution package
│       ├── runner.go             # Run(name string, args []string) int
│       └── runner_test.go        # Unit tests
```

### Pattern 1: Start/Wait Split with doneCh (Canonical)

**What:** Call `cmd.Start()`, then immediately start a goroutine that calls `cmd.Wait()` and sends to a buffered done channel. The main goroutine selects on the done channel (and in Phase 2, also on the signal channel). This ensures `cmd.Wait()` is called on every code path.

**When to use:** Always — this is the only correct pattern for a process runner that needs concurrent signal handling.

**Example:**
```go
// Source: https://pkg.go.dev/os/exec + STACK.md canonical pattern
package process

import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "syscall"
)

// Run spawns name with args, streams I/O in real time, waits for exit,
// and returns the child's exact exit code. No signal forwarding in Phase 1.
func Run(name string, args []string) int {
    cmd := exec.Command(name, args...)
    cmd.Stdin  = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    // Isolate child in its own process group (PROC-05).
    // Phase 1: Ctrl+C reaches only the runner, not the child.
    // Phase 2: explicit signal forwarding makes this correct.
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    if err := cmd.Start(); err != nil {
        if errors.Is(err, exec.ErrNotFound) {
            fmt.Fprintf(os.Stderr, "[rtx] command not found: %s\n", name)
            return 127
        }
        fmt.Fprintf(os.Stderr, "[rtx] failed to start: %v\n", err)
        return 1
    }
    fmt.Fprintf(os.Stderr, "[rtx] spawned PID %d\n", cmd.Process.Pid)

    // Wait in goroutine so Phase 2 can add a signal select without restructuring.
    doneCh := make(chan error, 1)
    go func() { doneCh <- cmd.Wait() }()

    waitErr := <-doneCh  // blocks until child exits

    code := exitCode(waitErr, name)
    fmt.Fprintf(os.Stderr, "[rtx] exited with code %d\n", code)
    return code
}

func exitCode(err error, name string) int {
    if err == nil {
        return 0
    }
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        return exitErr.ExitCode()
    }
    fmt.Fprintf(os.Stderr, "[rtx] wait error: %v\n", err)
    return 1
}
```

### Pattern 2: Inner Function Returns Int (os.Exit Safety)

**What:** `main()` contains only a single `os.Exit(run())` call. All real work happens in `run()` which returns an int. This allows `run()` to use `defer` freely — those defers run when `run()` returns, before `os.Exit()` is called.

**When to use:** Any time `os.Exit()` must be called from main and cleanup is needed.

**Example:**
```go
// Source: PITFALLS.md + STACK.md
// cmd/rtx/main.go

package main

import (
    "flag"
    "fmt"
    "os"
    "runtimex/internal/process"
)

func main() {
    os.Exit(run())
}

func run() int {
    // Global flags
    verbose := flag.Bool("v", false, "verbose output")
    flag.BoolVar(verbose, "verbose", false, "verbose output")
    flag.Parse()

    args := flag.Args()
    if len(args) == 0 {
        fmt.Fprintf(os.Stderr, "usage: rtx run <command> [args...]\n")
        return 1
    }

    subcommand := args[0]
    switch subcommand {
    case "run":
        if len(args) < 2 {
            fmt.Fprintf(os.Stderr, "[rtx] error: 'run' requires a command\n")
            return 1
        }
        return process.Run(args[1], args[2:])
    default:
        fmt.Fprintf(os.Stderr, "[rtx] unknown subcommand: %s\n", subcommand)
        return 1
    }
}
```

### Pattern 3: Direct fd Inheritance (Real-Time Streaming)

**What:** Assign `cmd.Stdout`, `cmd.Stderr`, `cmd.Stdin` directly to the parent's `os.File` values before `cmd.Start()`. The OS inherits the file descriptors — the child writes directly to the terminal with zero Go buffering.

**When to use:** Whenever transparent pass-through is needed. This is the only correct approach for PROC-02 and PROC-03.

**Example:**
```go
// Source: https://pkg.go.dev/os/exec — Cmd.Stdout field documentation
cmd.Stdin  = os.Stdin
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
// That's it. No goroutines. No pipes. No bufio.Scanner. No race conditions.
```

### Anti-Patterns to Avoid

- **`cmd.Run()` instead of `cmd.Start()`:** `Run()` blocks until exit — impossible to set up doneCh goroutine between start and wait. Defeats Phase 2 signal forwarding before it exists.
- **`cmd.Output()` / `cmd.CombinedOutput()`:** Buffers all output into a `bytes.Buffer` in memory; output appears only after exit; OOM risk for long-running commands.
- **`StdoutPipe()` / `StderrPipe()`:** Introduces a Go-managed pipe and goroutine; `cmd.Wait()` won't return until the pipe goroutine drains, which can deadlock if not started before Wait; direct `*os.File` assignment avoids this entirely.
- **`os.Exit(1)` on any `cmd.Wait()` error:** Swallows the real exit code. Always type-assert as `*exec.ExitError` and call `.ExitCode()`.
- **`os.Exit()` inside the process runner:** Skips deferred `signal.Stop()` cleanup. Return int; call `os.Exit()` only in `main()`.
- **`exec.Command("sh", "-c", userInput)`:** Shell interposition breaks PID display, signal delivery to real process, and exit code accuracy.
- **`cmd.Process.Release()` instead of `cmd.Wait()`:** `Release()` explicitly leaves a zombie.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exit code extraction | Custom error parsing | `(*exec.ExitError).ExitCode()` | Built-in since Go 1.12; handles all platform cases |
| Real-time output streaming | Manual goroutine + pipe + scanner | `cmd.Stdout = os.Stdout` (direct assignment) | Zero-copy, zero-buffering, zero goroutines, zero race conditions |
| "Command not found" detection | String matching on error.Error() | `errors.Is(err, exec.ErrNotFound)` | Stable API; error string matching is fragile |
| Zombie prevention | Manual process table inspection | `cmd.Wait()` called unconditionally | OS-level reaping; the goroutine-doneCh pattern ensures it runs on all paths |
| Process group isolation | Manual `setpgid()` syscall | `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` | Integrated with exec.Cmd lifecycle; set before Start() |
| Subcommand CLI | Hand-parsed `os.Args` | `flag.NewFlagSet` + `flag.Parse()` + `flag.Args()` | Handles `-v`, `--verbose`, `--`, and help text automatically |

**Key insight:** The entire process runner core is ~50 lines of stdlib-only Go. Any custom solution for the above problems will be both more complex and less correct than what the stdlib provides.

---

## Common Pitfalls

### Pitfall 1: cmd.Wait() Skipped on Error Path

**What goes wrong:** `cmd.Start()` succeeds; signal setup or PID logging panics or early-returns; `cmd.Wait()` is never called; child becomes a zombie.

**Why it happens:** Developer structures code as "start, then set up, then wait" — any panic or return between start and wait creates a zombie.

**How to avoid:** Use the goroutine-doneCh pattern: `go func() { doneCh <- cmd.Wait() }()` immediately after `cmd.Start()` returns nil. The goroutine calls Wait() unconditionally. The main function drains `doneCh` before returning.

**Warning signs:** `ps aux | grep Z` shows zombie entries; child command name appears as `<defunct>` in ps output.

### Pitfall 2: Exit Code Always 0 or 1

**What goes wrong:** `cmd.Wait()` returns `*exec.ExitError` but code does `if err != nil { return 1 }`. Child exits 42; caller sees 1.

**Why it happens:** `*exec.ExitError` is not the only non-nil error from `cmd.Wait()`; the natural generic check loses the real code.

**How to avoid:**
```go
if errors.As(waitErr, &exitErr) {
    return exitErr.ExitCode()  // exact code: 42, 127, 2, etc.
}
// Only reach here for non-exit errors (I/O error, etc.)
return 1
```

**Warning signs:** `echo $?` always returns 1 regardless of child behavior; `rtx run sh -c 'exit 42'` returns 1 instead of 42.

### Pitfall 3: Direct Assignment + StdoutPipe Mixed

**What goes wrong:** Developer sets `cmd.Stdout = os.Stdout` AND calls `cmd.StdoutPipe()`. `cmd.Start()` panics with "Stdout already set".

**Why it happens:** Both methods configure the same Stdout field in the Cmd struct; they are mutually exclusive.

**How to avoid:** Choose ONE method per stream. For rtx: always use direct assignment (`cmd.Stdout = os.Stdout`). Never use StdoutPipe.

**Warning signs:** Panic at `cmd.Start()`: "exec: Stdout already set".

### Pitfall 4: ErrNotFound Detection via Error String

**What goes wrong:** `strings.Contains(err.Error(), "not found")` — fragile; different OS versions produce different strings.

**Why it happens:** Developer doesn't know about `exec.ErrNotFound`.

**How to avoid:** `errors.Is(err, exec.ErrNotFound)` — stable API-level check that works regardless of error message wording.

**Warning signs:** "command not found" error handling breaks on Alpine Linux or in Docker containers with minimal PATH.

### Pitfall 5: os.Exit Skips Deferred Cleanup

**What goes wrong:** `defer signal.Stop(sigCh)` inside the runner function — but `os.Exit()` is also called from the runner. Defer never runs; signal channel is never cleaned up.

**Why it happens:** `os.Exit()` does NOT run deferred functions. This is Go's documented behavior.

**How to avoid:** Inner-function-returns-int pattern. The runner function returns an int. `main()` calls `os.Exit(run())`. All deferred functions in `run()` execute before `run()` returns, before `os.Exit()` is called.

**Warning signs:** Resource leak warnings; unexpected behavior in tests that import the runner directly.

### Pitfall 6: Setpgid + Phase 1 Ctrl+C = Orphan Child

**What goes wrong:** `Setpgid: true` is set (PROC-05). User presses Ctrl+C. Kernel delivers SIGINT only to `rtx` (child is in new process group). `rtx` exits. Child continues running as an orphan.

**Why it happens:** This is the correct behavior of Setpgid — it's WHY you set it, so you can control signal delivery. But in Phase 1, there's no signal forwarding yet.

**How to avoid:** Two options: (a) Set `Setpgid: true` in Phase 1 and document that Ctrl+C may orphan the child until Phase 2 — acceptable because Phase 1 success criteria don't test signal scenarios; (b) Don't set Setpgid in Phase 1 and add it in Phase 2. The canonical pattern (STACK.md) sets it from the start. The planner MUST document this behavior gap in the Phase 1 verification checklist.

**Warning signs:** `rtx run sleep 100`, press Ctrl+C — rtx exits, `ps aux` still shows `sleep 100` running.

---

## Code Examples

Verified patterns from official sources and project-level research:

### Complete runner.go for Phase 1

```go
// Source: STACK.md canonical pattern + https://pkg.go.dev/os/exec
package process

import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "syscall"
)

// Run spawns name with args, streams stdout/stderr in real time,
// waits for exit, and returns the child's exact exit code.
// Phase 1: no signal forwarding — Ctrl+C behavior depends on Setpgid.
func Run(name string, args []string) int {
    cmd := exec.Command(name, args...)
    cmd.Stdin  = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    if err := cmd.Start(); err != nil {
        if errors.Is(err, exec.ErrNotFound) {
            fmt.Fprintf(os.Stderr, "[rtx] command not found: %s\n", name)
            return 127
        }
        fmt.Fprintf(os.Stderr, "[rtx] failed to start: %v\n", err)
        return 1
    }
    fmt.Fprintf(os.Stderr, "[rtx] spawned PID %d\n", cmd.Process.Pid)

    doneCh := make(chan error, 1)
    go func() { doneCh <- cmd.Wait() }()

    waitErr := <-doneCh

    code := resolveExitCode(waitErr, name)
    fmt.Fprintf(os.Stderr, "[rtx] exited with code %d\n", code)
    return code
}

func resolveExitCode(err error, name string) int {
    if err == nil {
        return 0
    }
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        return exitErr.ExitCode()
    }
    fmt.Fprintf(os.Stderr, "[rtx] wait error: %v\n", err)
    return 1
}
```

### Complete main.go for Phase 1

```go
// Source: https://pkg.go.dev/flag (FlagSet subcommand pattern)
package main

import (
    "flag"
    "fmt"
    "os"
    "runtimex/internal/process"
)

func main() {
    os.Exit(run())
}

func run() int {
    verbose := flag.Bool("v", false, "verbose output")
    flag.BoolVar(verbose, "verbose", false, "verbose output")
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "usage: rtx [flags] run <command> [args...]\n")
        flag.PrintDefaults()
    }
    flag.Parse()

    args := flag.Args()
    if len(args) == 0 {
        flag.Usage()
        return 1
    }

    switch args[0] {
    case "run":
        if len(args) < 2 {
            fmt.Fprintf(os.Stderr, "[rtx] error: 'run' requires a command\n")
            return 1
        }
        return process.Run(args[1], args[2:])
    default:
        fmt.Fprintf(os.Stderr, "[rtx] unknown subcommand: %s\n", args[0])
        return 1
    }
}
```

### exit code 127 Detection

```go
// Source: https://pkg.go.dev/os/exec — ErrNotFound variable
// cmd.Start() returns exec.ErrNotFound (wrapped) when the binary is not in PATH.
// Use errors.Is, not string matching.
if err := cmd.Start(); err != nil {
    if errors.Is(err, exec.ErrNotFound) {
        fmt.Fprintf(os.Stderr, "[rtx] command not found: %s\n", name)
        return 127  // ERR-01: POSIX convention for "command not found"
    }
    fmt.Fprintf(os.Stderr, "[rtx] failed to start: %v\n", err)
    return 1
}
```

### Build Command

```bash
# Build the rtx binary alongside the existing API and worker binaries:
go build -o bin/rtx ./cmd/rtx

# Verify no vet warnings (catches unbuffered signal channels, etc.):
go vet ./...

# Run process package tests:
go test ./internal/process/...
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `cmd.Run()` for simple execution | `cmd.Start()` + `cmd.Wait()` split always | Go 1.0 — but best practices evolved | `Run()` is fine for fire-and-forget; never correct for signal-forwarding runners |
| `syscall.WaitStatus.ExitStatus()` for exit code | `(*exec.ExitError).ExitCode()` | Go 1.12 | Cross-platform; simpler; no syscall import needed for exit code extraction |
| `StdoutPipe()` + goroutine for streaming | `cmd.Stdout = os.Stdout` direct assignment | Go 1.0 — but best practices evolved | Direct assignment is zero-copy, race-free, and Wait()-safe |
| `exec.ErrNotFound` string matching | `errors.Is(err, exec.ErrNotFound)` | Go 1.13 (errors.Is) | API-stable; not fragile on different OS error messages |
| `signal.NotifyContext` (Go 1.16+) | `signal.Notify` + buffered channel | N/A — both valid | `NotifyContext` is better for servers; bare Notify is simpler for single-process forwarder |

**Deprecated/outdated:**
- `cmd.Run()` for process runners: Blocks signal interposition. Use `cmd.Start()` + `cmd.Wait()`.
- `os.Process.Release()`: Does not reap zombie. Use `cmd.Wait()`.
- `sh -c` wrapping in rtx: Use direct `exec.Command(args[0], args[1:]...)`.

---

## Open Questions

1. **Setpgid in Phase 1 vs. Phase 2**
   - What we know: Setting `Setpgid: true` in Phase 1 creates an observable behavior gap — Ctrl+C will orphan the child until Phase 2 adds explicit signal forwarding.
   - What's unclear: The CONTEXT.md marks Setpgid as "Claude's Discretion" and says "research recommends Setpgid: true." REQUIREMENTS.md lists PROC-05 (`Setpgid: true`) as a Phase 1 requirement.
   - Recommendation: Set `Setpgid: true` in Phase 1 as PROC-05 requires. Note the orphan-on-Ctrl+C behavior in verification. Phase 1 success criteria do not include signal tests — this is acceptable. Phase 2 will close the gap.

2. **ExitCode() returns -1 for signal-killed child in Phase 1**
   - What we know: If the child is killed by a signal (SIGKILL, etc.) before Phase 2, `exitErr.ExitCode()` returns -1. The runner would log "exited with code -1" and exit with code -1.
   - What's unclear: Is exit code -1 an acceptable Phase 1 behavior for signal-killed children? (EXIT-03, which handles this correctly with 128+N, is a Phase 2 requirement.)
   - Recommendation: Acceptable for Phase 1. EXIT-03 is explicitly listed as Phase 2. Phase 1 success criteria only test normal exits and "command not found."

3. **Log format prefix**
   - What we know: REQUIREMENTS.md uses `[rtx]` prefix: `[rtx] spawned PID %d`, `[rtx] exited with code %d`. CONTEXT.md leaves log format to Claude's Discretion.
   - Recommendation: Use `[rtx]` prefix consistently. `fmt.Fprintf(os.Stderr, "[rtx] spawned PID %d\n", pid)`. All logging to stderr. No timestamps. LOG-01 and LOG-03 are the Phase 1 log requirements; LOG-02 (signal received) is Phase 2.

---

## Sources

### Primary (HIGH confidence)
- `https://pkg.go.dev/os/exec` — Cmd struct fields, Start/Wait lifecycle, ExitError, ErrNotFound, SysProcAttr
- `https://pkg.go.dev/os#ProcessState.ExitCode` — ExitCode() returns -1 for signal-killed; +N for normal exit
- `https://pkg.go.dev/flag` — NewFlagSet, Parse, Args() subcommand pattern
- `https://pkg.go.dev/os/signal` — Notify, Stop, buffered channel requirement
- `https://pkg.go.dev/syscall` — SysProcAttr{Setpgid: true}, signal constants
- `.planning/research/STACK.md` — Complete canonical implementation loop, verified 2026-02-27
- `.planning/research/ARCHITECTURE.md` — Component structure, data flow, anti-patterns, verified 2026-02-27
- `.planning/research/PITFALLS.md` — All 7 pitfalls with Go issue tracker citations, verified 2026-02-27

### Secondary (MEDIUM confidence)
- `https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/` — Process group isolation patterns (aligns with official docs)
- `https://abhinavg.net/2022/08/13/flag-subcommand/` — flag.NewFlagSet subcommand pattern (aligns with official docs)

### Tertiary (LOW confidence)
- None for this phase — all critical claims verified against official Go documentation.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages are Go stdlib; behavior verified against pkg.go.dev official documentation
- Architecture: HIGH — based on existing codebase structure (cmd/main.go, cmd/worker/main.go patterns) and official Go stdlib docs
- Pitfalls: HIGH — primary sources include Go issue tracker entries (tracked bugs); canonical implementation from project-level STACK.md research

**Research date:** 2026-02-28
**Valid until:** 2026-03-28 (Go stdlib is stable; no breaking changes expected in this domain)
