# Phase 10: CLI serve and Graceful Shutdown - Research

**Researched:** 2026-03-05
**Domain:** Go CLI subcommands, signal handling, graceful HTTP shutdown, static file serving
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CMD-01 | `rtx serve` starts the API server and serves the React frontend | `main.go` needs to be refactored to support subcommands; `http.FileServer` can serve the React build from `web/dist` |
| CMD-02 | `rtx serve` handles SIGTERM/SIGINT gracefully — stops all managed processes before shutting down | `os/signal` to catch signals; `http.Server.Shutdown()` for clean API exit; `scheduler.StopAll()` (to be implemented) for process cleanup |
| CMD-03 | `rtx run` continues to work as v1.0 single-process runner (backwards compatible) | Refactor `main.go` using `flag.NewFlagSet` to isolate subcommand flags; maintain existing `process.Run()` logic for the `run` subcommand |
</phase_requirements>

---

## Summary

Phase 10 wires the components built in previous phases (Scheduler, REST API) into a unified `rtx serve` command. It also ensures the tool remains a reliable CLI by preserving the original `rtx run` behavior and implementing robust lifecycle management for the long-running server process.

The primary challenge is **orchestrated graceful shutdown**. When `rtx serve` receives a termination signal (SIGINT/SIGTERM), it must:
1. Stop accepting new API requests (graceful HTTP shutdown).
2. Stop all currently managed processes in the scheduler (using a new `StopAll()` method).
3. Wait for all processes to exit before finally exiting the CLI.

For frontend delivery, we will use `http.FileServer` pointing to `web/dist`. While `go:embed` is a strong alternative for single-binary distribution, `http.FileServer` is simpler for v1.1 development and matches the recommended strategy in `PROJECT.md`.

**Primary recommendation:** Use `flag.NewFlagSet` for subcommand routing in `main.go`. Implement a `StopAll()` method on the `Scheduler` that sends SIGTERM to all running processes in parallel and waits for them. Use `context.WithTimeout` for the HTTP server shutdown.

---

## Standard Stack

### Core

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| Go stdlib (`flag`, `os/signal`, `net/http`) | Go 1.25.5 | Subcommands, signal handling, static serving | No external dependencies; Go's `flag` package is sufficient for simple subcommands; `os/signal` is the canonical way to handle termination |
| `http.Server.Shutdown(ctx)` | Go 1.8+ | Graceful API shutdown | Standard library support for draining active connections |

### Supporting

| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| New `StopAll()` method on Scheduler | Phase 10 | Stops all registered processes | Called during graceful shutdown in `rtx serve` |
| `http.FileServer(http.Dir("web/dist"))` | Go stdlib | Serving the React frontend | Mounted at `/` on the server's router |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `flag.NewFlagSet` | `spf13/cobra` | Cobra is powerful but adds external dependencies; `flag` is zero-dependency and sufficient for 2-3 subcommands |
| `http.FileServer` | `go:embed` | `embed` produces a single binary but requires a build step for the frontend; `FileServer` is simpler for v1.1 |

**Installation:** No new packages. Pure Go stdlib.

---

## Architecture Patterns

### Recommended File Structure

```
cmd/rtx/
├── main.go           # Refactored for subcommands (run, serve)
└── serve.go          # NEW: serve subcommand implementation logic
internal/scheduler/
├── ...
├── deps.go           # StartAll() (existing)
└── lifecycle.go      # StopAll() (NEW)
```

Refactoring `main.go` to delegate to subcommand handlers (e.g., `cmdRun()`, `cmdServe()`) keeps the entry point clean.

### Pattern 1: Subcommand Routing with `flag`

**What:** Use `flag.NewFlagSet` for each subcommand to isolate their flags and usage strings.

**When to use:** In `main.go` to handle `rtx run` and `rtx serve`.

**Example:**
```go
func run() int {
    if len(os.Args) < 2 {
        printGlobalUsage()
        return 1
    }

    switch os.Args[1] {
    case "run":
        return cmdRun(os.Args[2:])
    case "serve":
        return cmdServe(os.Args[2:])
    default:
        fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
        return 1
    }
}
```

### Pattern 2: Graceful Shutdown Orchestration

**What:** Use a signal channel and a context to manage the shutdown sequence.

**When to use:** In `cmdServe()`.

**Sequence:**
1. Catch SIGINT/SIGTERM.
2. Call `server.Shutdown(ctx)` (stop API).
3. Call `scheduler.StopAll()` (stop processes).
4. Exit.

### Pattern 3: Scheduler `StopAll()`

**What:** New method on `Scheduler` that stops all running processes in parallel and waits for completion.

**Example:**
```go
func (s *Scheduler) StopAll() {
    processes := s.List()
    var wg sync.WaitGroup
    for _, mp := range processes {
        if mp.State == StateRunning || mp.State == StateStarting {
            wg.Add(1)
            go func(name string) {
                defer wg.Done()
                s.Stop(name)
            }(mp.Def.Name)
        }
    }
    wg.Wait()
}
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Signal handling | Low-level syscalls | `os/signal` + `Notify` | Robust and cross-platform |
| HTTP Server | Custom socket listener | `http.Server` | Standard library handles timeouts and graceful shutdown |

---

## Common Pitfalls

### Pitfall 1: Orphaned Processes on CLI Crash

**What goes wrong:** If the CLI crashes or is force-killed (SIGKILL), the managed processes keep running as orphans.

**How to avoid:** Use `Setpgid: true` (already done in Phase 6) so managed processes are in their own groups. However, for a clean shutdown, the CLI *must* catch SIGINT/SIGTERM and call `StopAll()`. There is no way to catch SIGKILL.

### Pitfall 2: Blocking Shutdown

**What goes wrong:** A managed process refuses to exit (ignoring SIGTERM), causing `rtx serve` to hang indefinitely during shutdown.

**How to avoid:** `s.Stop()` already handles SIGKILL escalation after a timeout. Ensure `StopAll()` respects this by letting each `Stop()` call handle its own timeout/escalation.

---

## Sources

### Primary (HIGH confidence)
- Go Standard Library documentation (`os/signal`, `net/http`, `flag`)
- `PROJECT.md` and `ROADMAP.md` success criteria for Phase 10

### Secondary (MEDIUM confidence)
- [Graceful Shutdown in Go - Go by Example](https://gobyexample.com/signals)
- [Serving Static Files with Go - Alex Edwards](https://www.alexedwards.net/blog/serving-static-files-with-go)
