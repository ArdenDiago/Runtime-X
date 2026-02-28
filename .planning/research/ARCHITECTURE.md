# Architecture Research

**Domain:** Go process runner CLI (rtx binary)
**Researched:** 2026-02-27
**Confidence:** HIGH — based on official Go stdlib documentation (os/exec, os/signal, syscall) and verified community patterns

---

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     cmd/rtx/main.go                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │               CLI Layer (flag/os.Args)                │   │
│  │   Parse "rtx run <command> [args...]"                 │   │
│  └────────────────────────┬─────────────────────────────┘   │
└───────────────────────────┼─────────────────────────────────┘
                            │ calls
┌───────────────────────────▼─────────────────────────────────┐
│                  internal/process/                            │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐  │
│  │  Executor   │  │SignalHandler │  │  OutputStreamer     │  │
│  │             │  │              │  │                    │  │
│  │ exec.Cmd    │  │signal.Notify │  │ cmd.Stdout = os.   │  │
│  │ cmd.Start() │  │cmd.Process.  │  │   Stdout           │  │
│  │ cmd.Wait()  │  │  Signal(sig) │  │ cmd.Stderr = os.   │  │
│  └──────┬──────┘  └──────┬───────┘  │   Stderr           │  │
│         │                │          └────────────────────┘  │
│         └────────────────┘                                   │
│              ↓ returns ExitCode                               │
└─────────────────────────────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                    OS Process Layer                           │
│  ┌──────────────────────────────────────────────────────┐   │
│  │       Child Process (arbitrary command)               │   │
│  │   PID tracked via cmd.Process.Pid                     │   │
│  │   stdout/stderr → inherited from rtx parent           │   │
│  │   signals → forwarded via cmd.Process.Signal()        │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| CLI Parser (cmd/rtx/main.go) | Parse subcommand and args; validate that "run" is given; delegate to process package | `os.Args` slicing or `flag` package; `os.Exit(1)` on bad args |
| Executor (internal/process/) | Spawn child via exec.Command; call Start(); track PID; call Wait() unconditionally | `exec.Command(name, args...)` with Start/Wait split pattern |
| Signal Handler (internal/process/) | Intercept SIGINT/SIGTERM; forward to child process; prevent premature rtx exit | `signal.Notify(chan, syscall.SIGINT, syscall.SIGTERM)` goroutine loop |
| Output Streamer (internal/process/) | Wire child stdout/stderr directly to parent's stdout/stderr; no buffering | `cmd.Stdout = os.Stdout; cmd.Stderr = os.Stderr` |
| Exit Code Propagator (cmd/rtx/main.go) | Extract child exit code from ExitError; call os.Exit with child's code | Type-assert `*exec.ExitError`, call `ExitCode()` |

---

## Recommended Project Structure

```
Runtime-X/
├── cmd/
│   ├── main.go                     # Existing API server (unchanged)
│   ├── worker/
│   │   └── main.go                 # Existing worker process (unchanged)
│   └── rtx/
│       └── main.go                 # NEW: rtx CLI entrypoint
│
├── internal/
│   ├── core/                       # Existing domain models (unchanged)
│   ├── queue/                      # Existing queue (unchanged)
│   ├── worker/                     # Existing worker pool (unchanged)
│   ├── docker/                     # Existing Docker validation (unchanged)
│   ├── logging/                    # Existing logger (unchanged)
│   └── process/                    # NEW: process execution package
│       ├── runner.go               # ProcessRunner struct + Run() method
│       └── runner_test.go          # Unit tests for process runner
```

### Structure Rationale

- **cmd/rtx/main.go:** Follows the existing convention of one entry point per binary (cmd/main.go, cmd/worker/main.go). Only handles CLI argument parsing and os.Exit; no business logic lives here.
- **internal/process/:** Parallel to internal/worker/ but separate because rtx has no queue/pool/scheduler — it is a direct executor. The separation keeps Docker-specific code in internal/worker/ and pure OS process execution in internal/process/. This boundary is explicit in PROJECT.md.
- **runner_test.go in same directory:** Follows the project convention of pairing _test.go alongside implementation (see internal/docker/validator_test.go).

---

## Architectural Patterns

### Pattern 1: Start/Wait Split (not Run())

**What:** Call `cmd.Start()` to launch the child process, then `cmd.Wait()` in a separate step — never `cmd.Run()`.

**When to use:** Any time signals must be forwarded, because you need an active `cmd.Process` reference between start and wait. `cmd.Run()` is a blocking call that returns only after exit, making it impossible to forward signals mid-execution.

**Trade-offs:** Slightly more code than `cmd.Run()`; however it is the only correct approach for signal forwarding and zombie prevention.

**Example:**
```go
cmd := exec.Command(name, args...)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr

if err := cmd.Start(); err != nil {
    return fmt.Errorf("failed to start process: %w", err)
}

log.Printf("pid=%d", cmd.Process.Pid)

// Signal forwarding goroutine runs here (see Pattern 2)

if err := cmd.Wait(); err != nil {
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        return exitErr // caller inspects ExitCode()
    }
    return err
}
return nil
```

### Pattern 2: Signal Forwarding via Goroutine Loop

**What:** Register a buffered signal channel with `signal.Notify`, spin a goroutine that receives signals and forwards them to the child via `cmd.Process.Signal(sig)`, then unblock when `cmd.Wait()` returns.

**When to use:** Any process wrapper CLI that must not absorb Ctrl+C or SIGTERM for itself — signals must flow through to the child.

**Trade-offs:** Requires coordination between the goroutine and Wait() completion; a done channel or context cancellation is used to stop the goroutine cleanly.

**Example:**
```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
defer signal.Stop(sigChan)

done := make(chan struct{})
go func() {
    defer close(done)
    for {
        select {
        case sig, ok := <-sigChan:
            if !ok {
                return
            }
            log.Printf("signal received: %s", sig)
            if cmd.Process != nil {
                cmd.Process.Signal(sig) // forward; ignore error if already exited
            }
        }
    }
}()

err := cmd.Wait()
signal.Stop(sigChan)
<-done // wait for forwarding goroutine to exit
```

### Pattern 3: Direct stdout/stderr Inheritance (No Pipes)

**What:** Assign `cmd.Stdout = os.Stdout` and `cmd.Stderr = os.Stderr` instead of using `StdoutPipe()`/`StderrPipe()`.

**When to use:** When the goal is transparent pass-through with zero buffering and no programmatic consumption of output. This is the correct choice for rtx v0 — the spec requires real-time streaming with no buffering.

**Trade-offs:** Cannot capture or transform output. For v0 this is a feature, not a limitation — the process runner must be transparent.

**Example:**
```go
cmd := exec.Command(name, args...)
cmd.Stdout = os.Stdout   // real-time, unbuffered pass-through
cmd.Stderr = os.Stderr   // real-time, unbuffered pass-through
cmd.Stdin  = os.Stdin    // pass stdin through for interactive commands
```

Contrast: `StdoutPipe()` is used only when the Go parent must read and process output programmatically (e.g., docker_runner.go captures logs for storage). For rtx, output is the user's concern.

---

## Data Flow

### Process Execution Flow

```
User runs: rtx run sleep 5
    ↓
cmd/rtx/main.go
    Parse os.Args → subcommand="run", command="sleep", args=["5"]
    Validate: len(args) >= 1 after "run"
    ↓
internal/process.Run("sleep", []string{"5"})
    ↓
exec.Command("sleep", "5")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin  = os.Stdin
    ↓
cmd.Start()
    → OS spawns child PID=12345
    → log.Printf("pid=12345")
    → child stdout/stderr flow directly to terminal (no buffering)
    ↓
Signal forwarding goroutine starts
    → signal.Notify(sigChan, SIGINT, SIGTERM)
    → blocks on select
    ↓
[If user presses Ctrl+C]
    sigChan receives SIGINT
    → log.Printf("signal received: interrupt")
    → cmd.Process.Signal(syscall.SIGINT)
    → child receives SIGINT, terminates
    ↓
cmd.Wait() returns
    → ExitError with ExitCode=130 (SIGINT convention)
    ↓
Signal goroutine exits (done channel closed)
    ↓
cmd/rtx/main.go
    Extract exit code from *exec.ExitError
    log.Printf("exit code: 130")
    os.Exit(130)
```

### Error Paths

```
Command Not Found:
    exec.Command("nonexistent")
    cmd.Start() → error: "exec: nonexistent: not found"
    → log error, os.Exit(1)

Immediate Crash:
    cmd.Start() succeeds
    cmd.Wait() returns ExitError{ExitCode: N}
    → extract code, os.Exit(N)

Invalid Args (no command given):
    cmd/rtx/main.go detects missing args
    → print usage, os.Exit(1)
    (never reaches internal/process)
```

### Signal Flow Direction

```
Terminal (Ctrl+C)
    ↓ SIGINT
os/signal channel (buffered, size=1)
    ↓ received by goroutine
cmd.Process.Signal(SIGINT)
    ↓ delivered to child PID
Child process receives SIGINT
    ↓ exits (with its own exit code)
cmd.Wait() unblocks
    ↓
rtx exits with child's exit code
```

---

## Build Order (Dependencies Between Components)

The components have a clear dependency chain. Build in this order:

### Phase 1: internal/process/ package

Build first because cmd/rtx/ depends on it. This package has zero external dependencies — only Go stdlib (os, os/exec, os/signal, syscall, log).

Components to build within the package:
1. **ProcessRunner struct definition** — holds nothing for v0 (stateless runner); defines the Run(name string, args []string) method signature
2. **Output wiring** — assign cmd.Stdout/Stderr/Stdin (simplest, no risk)
3. **Start/Wait split** — replace cmd.Run() with cmd.Start() + cmd.Wait()
4. **Exit code extraction** — handle *exec.ExitError, return code to caller
5. **Signal forwarding goroutine** — add after basic execution works

Each sub-step can be committed and tested independently.

### Phase 2: cmd/rtx/main.go

Build second. Depends only on internal/process/. Responsibilities:
1. **Argument parsing** — validate "run" subcommand exists, extract command+args
2. **Delegation** — call process.Run() with parsed values
3. **Exit code propagation** — call os.Exit() with the returned code

### Phase 3: Tests

Build alongside or immediately after each component:
- `internal/process/runner_test.go` — unit tests using real commands (echo, false, sleep)
- Manual test checklist from implementation guide (rtx run sleep 2, rtx run false, rtx run yes)

### Dependency Graph

```
cmd/rtx/main.go
    └── depends on → internal/process/
                         └── depends on → os/exec (stdlib)
                         └── depends on → os/signal (stdlib)
                         └── depends on → syscall (stdlib)
                         └── depends on → os (stdlib)
                         └── depends on → log (stdlib)
```

No circular dependencies. internal/process/ does NOT depend on internal/core/, internal/worker/, internal/queue/, or internal/logging/ — the process package is a standalone, self-contained unit.

---

## Component Boundaries

### What Talks to What

| From | To | Interface |
|------|----|-----------|
| cmd/rtx/main.go | internal/process | `process.Run(name string, args []string) error` returns typed exit error |
| internal/process | os/exec | `exec.Command()`, `cmd.Start()`, `cmd.Wait()` |
| internal/process | os/signal | `signal.Notify()`, `signal.Stop()` |
| internal/process | os/syscall | `syscall.SIGINT`, `syscall.SIGTERM` |
| internal/process | cmd.Process | `cmd.Process.Signal(sig)` for forwarding |
| cmd/rtx/main.go | os | `os.Exit(code)` for exit code propagation |

### What Does NOT Talk to What (Boundary Enforcement)

| Package | Must NOT depend on | Reason |
|---------|--------------------|--------|
| internal/process | internal/worker | Different abstraction; worker is queue-based Docker executor |
| internal/process | internal/core | No Job/DockerfileConfig needed; process runner is simpler |
| internal/process | internal/queue | No queue; rtx is synchronous single-process execution |
| cmd/rtx/main.go | internal/worker | rtx is not a worker-pool system |
| cmd/rtx/main.go | cmd/api | rtx is a CLI, not an HTTP server |

---

## Anti-Patterns

### Anti-Pattern 1: Using cmd.Run() Instead of Start()/Wait()

**What people do:** Call `cmd.Run()` for simplicity since it handles both start and wait.

**Why it's wrong:** `cmd.Run()` blocks until the process exits, so there is no opportunity to set up signal forwarding before the process is running. Signal interception must happen between Start and Wait.

**Do this instead:** Always use `cmd.Start()` then `cmd.Wait()` with a signal-forwarding goroutine active between the two calls.

---

### Anti-Pattern 2: Shell Wrapping (sh -c)

**What people do:** `exec.Command("sh", "-c", userCommand)` to run commands through a shell.

**Why it's wrong:** The actual process becomes the shell (`sh`), not the user's command. Signals sent to the shell may not propagate to the real child. Exit codes may differ. PIDs are wrong (shell's PID not the command's PID). PROJECT.md explicitly prohibits this.

**Do this instead:** `exec.Command(name, args...)` with direct exec — no shell wrapper. The existing `internal/worker/runner.go` uses `sh -c` but that is the Docker worker pattern; rtx must NOT replicate it.

---

### Anti-Pattern 3: Swallowing Exit Codes

**What people do:** Check only `if err != nil` after `cmd.Wait()` and return a generic error or always exit 1.

**Why it's wrong:** The core rtx requirement is exit code propagation. If the child exits 2, rtx must exit 2. A generic error loses that information.

**Do this instead:**
```go
if err := cmd.Wait(); err != nil {
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        os.Exit(exitErr.ExitCode())
    }
    log.Printf("process error: %v", err)
    os.Exit(1)
}
os.Exit(0)
```

---

### Anti-Pattern 4: Buffered Output (StdoutPipe + goroutine reader)

**What people do:** Use `cmd.StdoutPipe()` with a goroutine that copies to os.Stdout, thinking this is equivalent to direct assignment.

**Why it's wrong:** Introduces unnecessary complexity and a potential goroutine leak if the goroutine doesn't drain before Wait(). Direct assignment (`cmd.Stdout = os.Stdout`) is Go stdlib's canonical approach for transparent pass-through and is lower risk.

**Do this instead:** Direct assignment. The os/exec package guarantees Wait() handles I/O cleanup when Stdout/Stderr are set to file-backed writers like os.Stdout.

---

### Anti-Pattern 5: Not Calling cmd.Wait() on Error Paths

**What people do:** Return early after cmd.Start() if signal setup fails, skipping cmd.Wait().

**Why it's wrong:** If the process was started (Start returned nil), Wait() MUST be called to reap the child. Skipping it creates a zombie process. This is the zombie-prevention requirement from the spec.

**Do this instead:** Use `defer` for cleanup, or ensure Wait() is called on all code paths where Start() succeeded.

---

## Integration with Existing Project

### What Does Not Change

The entire existing Docker orchestration system (cmd/main.go, internal/worker/, internal/queue/, internal/core/, internal/docker/) is untouched. rtx is an additive binary that shares only:

- The Go module (`runtimex`) — new packages are imported normally
- Build tooling — go build ./cmd/rtx/ builds the new binary

### Coexistence

The project will have three binaries after this milestone:

```
go build -o bin/api     ./cmd/
go build -o bin/worker  ./cmd/worker/
go build -o bin/rtx     ./cmd/rtx/     ← new
```

Each binary is independent. The API server and worker process share internal/core/, internal/queue/, internal/worker/ — rtx shares none of these.

---

## Scaling Considerations

This section is minimal by design. rtx v0 is a single-process executor — scaling is not applicable.

| Scale | Architecture Adjustment |
|-------|--------------------------|
| v0: Single process | Current design — correct and complete |
| v1: Multiple processes | Would require process group tracking and a supervisor struct (out of v0 scope) |
| v1+: Restart policies | Would require state machine around process lifecycle (explicitly excluded) |

The design intentionally does not build for v1 concerns. Over-engineering here violates the v0 philosophy of earning trust through correctness.

---

## Sources

- [os/exec package — pkg.go.dev](https://pkg.go.dev/os/exec) — HIGH confidence, official Go stdlib docs
- [os/signal package — pkg.go.dev](https://pkg.go.dev/os/signal) — HIGH confidence, official Go stdlib docs
- [Netflix signal-wrapper — reference implementation of signal forwarding pattern](https://github.com/Netflix/signal-wrapper/blob/master/main.go) — MEDIUM confidence, well-known production pattern
- [Proxying to a subcommand with Go — Kevin Burke](https://kevin.burke.dev/kevin/proxying-to-a-subcommand-with-go/) — MEDIUM confidence, documents the Start/Wait/signal-goroutine pattern and syscall.Exec alternative
- [Some Useful Patterns for Go's os/exec — DoltHub Blog](https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/) — MEDIUM confidence, verified against stdlib docs
- [Signal Handling in Go Applications — Medium](https://medium.com/@AlexanderObregon/signal-handling-in-go-applications-b96eb61ecb69) — LOW confidence (single source, not official)
- Runtime-X codebase analysis — internal/worker/runner.go, internal/worker/docker_runner.go, .planning/codebase/ARCHITECTURE.md — HIGH confidence, direct code read

---

*Architecture research for: Go process runner CLI (rtx binary), Runtime X project*
*Researched: 2026-02-27*
