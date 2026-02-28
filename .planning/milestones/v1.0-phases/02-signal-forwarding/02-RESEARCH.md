# Phase 2: Signal Forwarding - Research

**Researched:** 2026-02-28
**Domain:** Go signal interception, forwarding, and POSIX exit code emulation (os/signal, syscall)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

None — all implementation choices are delegated to Claude's Discretion.

### Claude's Discretion

- Shutdown timeout behavior (wait forever vs force-kill after timeout)
- Double Ctrl+C / signal escalation strategy
- Signal log message format and verbosity
- Whether to forward to child PID only or process group
- Signal channel buffer size and goroutine structure
- Error handling for forwarding to already-dead process
- All implementation patterns and architecture choices

User trusts research recommendations and standard patterns. Research already covers this domain with HIGH confidence.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SIG-01 | Parent intercepts SIGINT and forwards it to child process | `signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)` + `cmd.Process.Signal(sig)` after catching signal |
| SIG-02 | Parent intercepts SIGTERM and forwards it to child process | Same channel and forward call as SIG-01 — both signals on one `signal.Notify` call |
| SIG-03 | Graceful shutdown: forward signal → wait for child to finish → exit with child's code | `select` block: signal case forwards then blocks on `<-doneCh`; exit code extraction via `resolveExitCode` |
| SIG-04 | Signal channel is buffered (capacity 1) to prevent dropped signals | `make(chan os.Signal, 1)` — documented requirement in `os/signal` pkg; `go vet` enforces this |
| EXIT-03 | Signal-killed child produces correct POSIX exit code (128 + signal number) | `ExitCode()` returns -1 for signal-killed processes; must inspect `cmd.ProcessState.Sys().(syscall.WaitStatus)` |
| ERR-03 | Signal forwarding to already-dead process is handled gracefully (swallow `os.ErrProcessDone`) | `errors.Is(err, os.ErrProcessDone)` check after `cmd.Process.Signal(sig)` |
| LOG-02 | Minimal logging to stderr: `[rtx] received signal %s` on signal | `fmt.Fprintf(os.Stderr, "[rtx] received signal %s\n", sig)` before forwarding |
</phase_requirements>

---

## Summary

Phase 2 adds signal interception and forwarding on top of the Phase 1 process runner. The implementation is a minimal extension of `internal/process/runner.go` — specifically, the existing `doneCh` + `select` structure was designed for this exact purpose. Phase 2 requires: adding a `sigCh` (buffered os.Signal channel), calling `signal.Notify`, and inserting one new `case` into the existing `select`. No restructuring of Phase 1 code is needed.

The critical design decision is already resolved: `Setpgid: true` was set in Phase 1 (PROC-05), which means the kernel does NOT auto-deliver Ctrl+C to the child. Explicit forwarding via `cmd.Process.Signal(sig)` is therefore mandatory — if the signal case fires and the forward is omitted, the child will run forever. This is the primary correctness requirement for this phase.

EXIT-03 (128+N exit codes for signal-killed children) requires special handling because `(*exec.ExitError).ExitCode()` returns -1 when a process was killed by a signal. The correct exit code must be computed by inspecting `cmd.ProcessState.Sys().(syscall.WaitStatus)` using the `Signaled()` and `Signal()` methods from the `syscall` package. This computation must be added to `resolveExitCode()` before the generic ExitError case.

**Primary recommendation:** Extend `runner.go` with a buffered `sigCh`, `signal.Notify`, and a signal `case` in the existing `select`; update `resolveExitCode` to emit 128+N for signal-killed processes; swallow `os.ErrProcessDone` from forwarding calls.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os/signal` | Go stdlib (1.25.x) | Intercept OS signals | Only Go stdlib mechanism to catch async signals; `signal.Notify` routes signals to a channel instead of default termination |
| `syscall` | Go stdlib (1.25.x) | Signal constants, WaitStatus, process group | Required for `SIGINT`/`SIGTERM` constants, `WaitStatus.Signaled()`/`Signal()` for 128+N exit code emulation |
| `os/exec` | Go stdlib (1.25.x) | `cmd.Process.Signal()` for forwarding | Already in use from Phase 1; `Signal()` on `*os.Process` delivers a signal to the child |
| `errors` | Go stdlib | `errors.Is(err, os.ErrProcessDone)` | Distinguishes "already dead" from real forwarding failures |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `fmt` | Go stdlib | Logging `[rtx] received signal %s` | Always — same logging pattern as Phase 1 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `signal.Notify(ch, ...)` | `signal.NotifyContext` | NotifyContext is better for multi-goroutine servers; for a single foreground process, bare Notify + select is simpler and equally correct |
| `cmd.Process.Signal(sig)` | `syscall.Kill(cmd.Process.Pid, sig.(syscall.Signal))` | Both work; `cmd.Process.Signal` is the Go stdlib abstraction and handles the `os.ErrProcessDone` sentinel correctly |
| 128+N via `WaitStatus.Signal()` | Hardcoding 130 for SIGINT | WaitStatus is correct for any signal; hardcoding breaks SIGTERM (143) and any future signals |

**No new dependencies.** No changes to `go.mod` required.

---

## Architecture Patterns

### What Phase 2 Changes in runner.go

Phase 1 `runner.go` has this structure:

```
cmd.Start()
doneCh goroutine: cmd.Wait()
waitErr = <-doneCh          ← Phase 2 replaces this bare receive
resolveExitCode(waitErr)
```

Phase 2 changes the bare `<-doneCh` receive into a `select` with a signal case:

```
cmd.Start()
sigCh setup + signal.Notify
doneCh goroutine: cmd.Wait()
select {
    case sig := <-sigCh:    ← NEW
        log + forward
        waitErr = <-doneCh  ← still waits (zombie prevention intact)
    case waitErr = <-doneCh:  ← natural exit path unchanged
}
resolveExitCode(waitErr)    ← extended for 128+N
```

This is the exact pattern the Phase 1 doneCh design was built to support.

### Pattern 1: Signal Interception and Forwarding

**What:** Intercept SIGINT and SIGTERM, log receipt, forward to child, then wait for child to finish before returning.

**When to use:** Always — this is the entire Phase 2 behavior.

**Example:**

```go
// Source: https://pkg.go.dev/os/signal + STACK.md Key Implementation Pattern
package process

import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "os/signal"
    "syscall"
)

func Run(name string, args []string) int {
    cmd := exec.Command(name, args...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // already set, Phase 1

    if err := cmd.Start(); err != nil {
        if errors.Is(err, exec.ErrNotFound) {
            fmt.Fprintf(os.Stderr, "[rtx] command not found: %s\n", name)
            return 127
        }
        fmt.Fprintf(os.Stderr, "[rtx] failed to start: %v\n", err)
        return 1
    }
    fmt.Fprintf(os.Stderr, "[rtx] spawned PID %d\n", cmd.Process.Pid)

    // SIG-04: buffered capacity 1 — signal.Notify does a non-blocking send
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM) // SIG-01, SIG-02
    defer signal.Stop(sigCh)

    doneCh := make(chan error, 1)
    go func() { doneCh <- cmd.Wait() }()

    var waitErr error
    select {
    case sig := <-sigCh: // SIG-01/SIG-02: signal received
        // LOG-02: log before forwarding
        fmt.Fprintf(os.Stderr, "[rtx] received signal %s\n", sig)
        // SIG-03 + ERR-03: forward; swallow "already finished"
        if err := cmd.Process.Signal(sig); err != nil && !errors.Is(err, os.ErrProcessDone) {
            fmt.Fprintf(os.Stderr, "[rtx] signal forward failed: %v\n", err)
        }
        waitErr = <-doneCh // PROC-04: always wait — zombie prevention
    case waitErr = <-doneCh: // natural exit
    }

    code := resolveExitCode(waitErr, cmd.ProcessState)
    fmt.Fprintf(os.Stderr, "[rtx] exited with code %d\n", code)
    return code
}
```

### Pattern 2: Signal-Killed Exit Code Emulation (EXIT-03)

**What:** When a process is killed by a signal, `(*exec.ExitError).ExitCode()` returns -1. POSIX shells emit 128+N (e.g., 130 for SIGINT=2, 143 for SIGTERM=15). This behavior is required for correct shell scripting integration.

**Why ExitCode() returns -1:** The `ExitCode()` method checks `ProcessState.Sys().(syscall.WaitStatus).Exited()` — a signal-killed process has `Exited() == false`, so `ExitCode()` falls back to -1.

**How to compute 128+N:**

```go
// Source: https://pkg.go.dev/syscall#WaitStatus
func resolveExitCode(err error, state *os.ProcessState) int {
    if err == nil {
        return 0
    }
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        code := exitErr.ExitCode()
        // EXIT-03: ExitCode() returns -1 for signal-killed processes
        if code == -1 && state != nil {
            if ws, ok := state.Sys().(syscall.WaitStatus); ok {
                if ws.Signaled() {
                    return 128 + int(ws.Signal())
                }
            }
        }
        return code
    }
    fmt.Fprintf(os.Stderr, "[rtx] wait error: %v\n", err)
    return 1
}
```

**Signal number reference:**
- SIGINT = 2 → exit code 130
- SIGTERM = 15 → exit code 143
- SIGKILL = 9 → exit code 137

**Note:** `cmd.ProcessState` is populated by `cmd.Wait()` and is accessible after `waitErr = <-doneCh`. Pass it to `resolveExitCode` as a parameter (or access via the existing `exitErr.ProcessState` field on the `*exec.ExitError`).

### Pattern 3: Graceful Shutdown Ordering (SIG-03)

**What:** Signal case MUST block on `<-doneCh` after forwarding. This ensures:
1. Zombie prevention: `cmd.Wait()` is always called (PROC-04)
2. Correct exit code: `waitErr` is populated from the child's actual exit
3. No race: runner doesn't exit before child has fully terminated

```go
// CORRECT: forward → then block waiting for child
case sig := <-sigCh:
    fmt.Fprintf(os.Stderr, "[rtx] received signal %s\n", sig)
    cmd.Process.Signal(sig)
    waitErr = <-doneCh  // blocks until child exits

// WRONG: exit immediately after forwarding (zombie + race)
case sig := <-sigCh:
    cmd.Process.Signal(sig)
    return 130  // child not waited; zombie created
```

### Anti-Patterns to Avoid

- **No `signal.Stop(sigCh)` after done:** Without `defer signal.Stop(sigCh)`, the signal package keeps sending to a channel nobody reads — goroutine leak risk. Always `defer signal.Stop(sigCh)` right after `signal.Notify`.
- **Forwarding to the process GROUP instead of the PID:** With `Setpgid: true`, the child is in its own process group. `cmd.Process.Signal(sig)` targets the PID, which is correct. Using `syscall.Kill(-pgid, sig)` would send to the entire group — unnecessary for single-process runners.
- **Using `ExitCode()` alone for EXIT-03:** Returns -1 for signal-killed children. Must check `WaitStatus.Signaled()` to emit 128+N.
- **Exiting the select after only one signal:** If the child ignores the first signal and a second arrives, the goroutine is no longer listening on `sigCh`. The second signal can be lost. For v0 (no escalation requirement), this is acceptable — but document it.
- **Calling `signal.Notify` before `cmd.Start()`:** Signals received before the child is running have no target to forward to. Initialize the signal channel after `cmd.Start()` succeeds.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Signal buffering | Custom ring buffer for signals | `make(chan os.Signal, 1)` | The Go runtime handles signal delivery; capacity-1 buffer is sufficient for a single-process runner and is the documented minimum |
| Signal-to-exit-code mapping | Hardcoded map of signal names to numbers | `syscall.WaitStatus.Signal()` cast to int | `Signal()` returns the OS signal number directly; no mapping table needed |
| Waiting for child after signal | Polling loop on `cmd.ProcessState` | `<-doneCh` (the goroutine calling `cmd.Wait()`) | `cmd.Wait()` is the correct POSIX wait call; polling is racy and burns CPU |
| Process liveness check before forwarding | `/proc/[pid]/status` check | `errors.Is(err, os.ErrProcessDone)` after `Signal()` | The OS may recycle PIDs between check and signal; the sentinel error is the race-free approach |

**Key insight:** The Go stdlib has designed the `os/signal` + `os/exec` APIs to compose correctly. The only custom logic needed is the 128+N exit code computation — everything else is direct API calls.

---

## Common Pitfalls

### Pitfall 1: Unbuffered Signal Channel (SIG-04)

**What goes wrong:** `signal.Notify` does a non-blocking send. If the goroutine isn't ready, the signal is silently dropped. `make(chan os.Signal)` (capacity 0) is the wrong default.

**Why it happens:** Go's default channel creation is unbuffered; `signal.Notify` is a documented exception.

**How to avoid:** `make(chan os.Signal, 1)`. Capacity 1 is sufficient — only one signal can be "in flight" at a time before the select processes it.

**Warning signs:** `go vet ./...` reports `misuse of unbuffered os.Signal channel`. Intermittent signal drops under load.

### Pitfall 2: Forwarding Fails Because Child Already Exited (ERR-03)

**What goes wrong:** Child exits on its own (natural death) right as a signal arrives. `doneCh` races with `sigCh` — the `select` picks `sigCh`, and `cmd.Process.Signal(sig)` returns `os.ErrProcessDone`. Without the `errors.Is` check, this is logged as an error.

**Why it happens:** Race between natural exit and signal arrival — both are real, concurrent events.

**How to avoid:** Always wrap `Signal()` call:
```go
if err := cmd.Process.Signal(sig); err != nil && !errors.Is(err, os.ErrProcessDone) {
    fmt.Fprintf(os.Stderr, "[rtx] signal forward failed: %v\n", err)
}
waitErr = <-doneCh // still drain doneCh regardless
```

**Warning signs:** `os: process already finished` in stderr during Ctrl+C of a short-lived process.

### Pitfall 3: Missing signal.Stop — Signal Channel Not Cleaned Up

**What goes wrong:** After the function returns, the signal channel is still registered. The signal package holds a reference to it. If the channel is garbage collected while still registered, the runtime panics or silently drops signals in subsequent calls.

**Why it happens:** Cleanup omitted; `signal.Stop` is not as obviously required as `defer close(ch)`.

**How to avoid:** `defer signal.Stop(sigCh)` immediately after `signal.Notify(sigCh, ...)`.

### Pitfall 4: ExitCode() Returns -1 for Signal-Killed Children (EXIT-03)

**What goes wrong:** After forwarding SIGINT, child is killed by signal. `cmd.Wait()` returns `*exec.ExitError`. `exitErr.ExitCode()` returns -1 (not -1 meaning "error" — it means "killed by signal"). Without the `WaitStatus` check, `resolveExitCode` returns -1 as the exit code, which `os.Exit(-1)` interprets as exit code 255 on Linux.

**Why it happens:** `ExitCode()` only reads the exit status for processes that called `exit()`. Signal-killed processes have a different WaitStatus bit pattern.

**How to avoid:** Check for -1 from `ExitCode()`, then inspect `cmd.ProcessState.Sys().(syscall.WaitStatus).Signaled()`. If true, return `128 + int(ws.Signal())`.

**Warning signs:** `echo $?` returns 255 after Ctrl+C instead of 130.

### Pitfall 5: Double Signal Delivery (Setpgid Interaction)

**What goes wrong:** If `Setpgid` were NOT set (it is set in Phase 1), the kernel delivers Ctrl+C to both the runner and child simultaneously. The runner also forwards via `cmd.Process.Signal(sig)`. Child receives SIGINT twice — may cause unexpected behavior in interactive programs.

**How this is avoided in this project:** `Setpgid: true` is already applied in Phase 1 (PROC-05). The child is in a separate process group and does NOT receive Ctrl+C from the keyboard automatically. Explicit forwarding via `cmd.Process.Signal(sig)` is the only delivery path. This is the correct model.

**Warning signs (if Setpgid were removed):** Child exits immediately and runner logs "process already finished" when trying to forward.

---

## Code Examples

Verified patterns from official sources:

### Complete Updated runner.go (Phase 2)

```go
// Source: internal/process/runner.go — extended from Phase 1
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
    // PROC-05: isolate child in its own process group — explicit forwarding mandatory
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

    // SIG-04: buffered channel — signal.Notify does a non-blocking send
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM) // SIG-01, SIG-02
    defer signal.Stop(sigCh)

    // PROC-04: doneCh ensures cmd.Wait() is called on every code path
    doneCh := make(chan error, 1)
    go func() { doneCh <- cmd.Wait() }()

    var waitErr error
    select {
    case sig := <-sigCh:
        // LOG-02: log signal receipt before forwarding
        fmt.Fprintf(os.Stderr, "[rtx] received signal %s\n", sig)
        // SIG-03 + ERR-03: forward signal; swallow "already finished"
        if err := cmd.Process.Signal(sig); err != nil && !errors.Is(err, os.ErrProcessDone) {
            fmt.Fprintf(os.Stderr, "[rtx] signal forward failed: %v\n", err)
        }
        waitErr = <-doneCh // SIG-03: wait for child before exiting
    case waitErr = <-doneCh:
        // natural exit — no signal handling needed
    }

    code := resolveExitCode(waitErr, cmd.ProcessState)
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
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        code := exitErr.ExitCode()
        // EXIT-03: ExitCode() returns -1 for signal-killed processes
        // Inspect WaitStatus to compute 128 + signal number
        if code == -1 && state != nil {
            if ws, ok := state.Sys().(syscall.WaitStatus); ok {
                if ws.Signaled() {
                    return 128 + int(ws.Signal())
                }
            }
        }
        return code
    }
    // Non-ExitError: runner infrastructure failure (I/O error etc.)
    fmt.Fprintf(os.Stderr, "[rtx] wait error: %v\n", err)
    return 1
}
```

### Signal Number to Exit Code Reference

```
SIGINT  (2)  → 128 + 2  = 130   (Ctrl+C)
SIGTERM (15) → 128 + 15 = 143   (kill default)
SIGKILL (9)  → 128 + 9  = 137   (kill -9, rtx cannot forward but child may receive)
SIGHUP  (1)  → 128 + 1  = 129   (terminal disconnect)
```

### Verifying Signal-Killed Exit Codes

```bash
# Verify SIGINT exit code = 130
rtx run sleep 100 &
kill -SIGINT $!
wait $!
echo $?   # should be 130

# Verify SIGTERM exit code = 143
rtx run sleep 100 &
kill -SIGTERM $!
wait $!
echo $?   # should be 143

# Verify natural exit code unchanged
rtx run sh -c 'exit 42'
echo $?   # should be 42
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `syscall.WaitStatus.ExitStatus()` for exit code | `(*exec.ExitError).ExitCode()` | Go 1.12 | Cleaner API; still need WaitStatus for signal-killed cases (ExitCode returns -1) |
| `signal.Notify` + manual goroutine loop | `signal.NotifyContext` | Go 1.16 | NotifyContext integrates with context cancellation; for single-process foreground runner, bare Notify is sufficient |
| `exec.CommandContext` with `os.Kill` on cancel | `exec.Cmd.Cancel` + `Cmd.WaitDelay` | Go 1.20 | More precise control over kill signal and pipe drain timeout; not needed for v0 |

**No deprecated patterns in use.** All Phase 1 code uses current, idiomatic Go 1.25.x APIs.

---

## Open Questions

1. **Double Ctrl+C / Signal Escalation**
   - What we know: SIG-03 requires forwarding and waiting; no requirement for escalation
   - What's unclear: If the child ignores SIGTERM/SIGINT and hangs, rtx waits forever; no timeout requirement exists in v0
   - Recommendation: For v0, wait indefinitely (no timeout). The `select` only processes one signal — a second Ctrl+C will reach the default OS handler and kill rtx itself, which implicitly orphans the child. This is acceptable for v0. Document the behavior.

2. **`cmd.ProcessState` Access After Signal + doneCh**
   - What we know: `cmd.ProcessState` is populated after `cmd.Wait()` completes; `doneCh` receives the result of `cmd.Wait()`
   - What's unclear: Whether `cmd.ProcessState` is guaranteed to be set after the signal case's `waitErr = <-doneCh` receive completes
   - Recommendation: Yes — `cmd.Wait()` populates `cmd.ProcessState` before sending to `doneCh`. Access `cmd.ProcessState` after `waitErr = <-doneCh` is safe. Alternatively, access via `exitErr.ProcessState` on the `*exec.ExitError` (same pointer).

---

## Sources

### Primary (HIGH confidence)

- `https://pkg.go.dev/os/signal` — `Notify`, `Stop` signatures; buffered channel requirement documented explicitly
- `https://pkg.go.dev/syscall#WaitStatus` — `Signaled()`, `Signal()` methods for detecting and extracting signal number from process state
- `https://pkg.go.dev/os/exec#Cmd.Wait` — `ProcessState` populated after Wait; `ExitError.ExitCode()` returns -1 for signal-killed
- `https://pkg.go.dev/os#ProcessState` — `Sys()` returns `interface{}` castable to `syscall.WaitStatus` on Linux
- `.planning/research/STACK.md` — Canonical implementation pattern; signal goroutine loop with select; Setpgid decision; HIGH confidence across all domains
- `.planning/research/PITFALLS.md` — Pitfall 3 (unbuffered channel), Pitfall 4 (signal not forwarded), Pitfall 5 (process group conflict), Integration Gotcha (ErrProcessDone)
- `internal/process/runner.go` — Phase 1 implementation; confirms `Setpgid: true` is set, `doneCh` pattern is in place, `resolveExitCode` exists and needs 128+N extension

### Secondary (MEDIUM confidence)

- `https://github.com/golang/go/issues/7938` — Confirms `ExitCode()` returns -1 for signal-killed processes; WaitStatus inspection is the documented workaround
- `https://github.com/golang/go/issues/26539` — History of `ExitCode()` API addition; explains -1 sentinel for signal-killed
- `https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/` — Signal forwarding with select + doneCh pattern; process group isolation; aligns with official docs

### Tertiary (LOW confidence)

None used for critical claims.

---

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — All packages are Go stdlib; no version ambiguity. APIs verified against pkg.go.dev.
- Architecture: HIGH — Phase 1 was explicitly designed to receive Phase 2 with a single `select` case addition. The doneCh pattern is already in production code.
- Signal forwarding patterns: HIGH — Verified against official os/signal docs and STACK.md canonical implementation.
- EXIT-03 (128+N): HIGH — WaitStatus.Signaled() + Signal() is the documented approach; ExitCode() returning -1 for signal-killed is verified against Go issue tracker.
- Pitfalls: HIGH — Primary sources are official Go issue tracker and pkg.go.dev docs.

**Research date:** 2026-02-28
**Valid until:** 2026-04-28 (Go stdlib APIs are stable; 60-day window is conservative)
