# Architecture Research

**Domain:** Multi-process scheduler, Go REST API, React frontend (Runtime X v1.1)
**Researched:** 2026-02-28
**Confidence:** HIGH — based on existing codebase analysis, Go stdlib net/http documentation, and verified patterns for process supervisors and REST APIs

---

## Context: What Exists vs. What Is New

### Existing (Keep Exactly As-Is)

| File | Role | v1.1 Touch? |
|------|------|-------------|
| `cmd/rtx/main.go` | CLI entry point — `rtx run <cmd>` | Add `serve` subcommand, no other changes |
| `internal/process/runner.go` | Single-process `Run(name, args) int` | No changes — scheduler REUSES this |
| `internal/process/runner_test.go` | Unit tests for runner | No changes |

### To Delete (Legacy Docker Code)

| Path | Why Remove |
|------|-----------|
| `cmd/main.go` | Legacy API server entry — superseded by new `cmd/rtx/` serve subcommand |
| `cmd/worker/main.go` | Legacy worker entry — Docker job worker, no longer needed |
| `cmd/api/` (all files) | Docker-specific handlers and router |
| `internal/api/` (all files) | Legacy handlers with broken build (references undefined `h.Scheduler`) |
| `internal/worker/` (all files) | Docker worker pool, scheduler, runner |
| `internal/queue/` (all files) | Docker job queue |
| `internal/core/` (all files) | Docker Job/JobStatus domain model |
| `internal/docker/` (all files) | Dockerfile validator |
| `internal/logging/` (all files) | Legacy logger |
| `frontend/` (existing Go templates) | Legacy Go-template frontend — replaced by React |

### New Components (v1.1)

| Path | Role |
|------|------|
| `internal/scheduler/` | Multi-process lifecycle manager — owns process state, dependency graph, restart logic |
| `internal/api/` (rewritten) | Go REST API — HTTP handlers, JSON marshaling, CORS |
| `cmd/rtx/main.go` (extended) | Add `serve` subcommand to launch API server |
| `web/` | React frontend — built separately, served as static files by the API |

---

## Standard Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                          User Interfaces                              │
│                                                                       │
│   Browser (React SPA)                  Terminal (rtx CLI)            │
│   ┌──────────────────────────┐         ┌───────────────────┐         │
│   │  Process List View       │         │  rtx run <cmd>    │         │
│   │  Create / Edit Form      │         │  (single-process, │         │
│   │  Start / Stop Buttons    │         │   unchanged)      │         │
│   │  Log Viewer (polling)    │         └───────────────────┘         │
│   └───────────┬──────────────┘                                       │
└───────────────┼──────────────────────────────────────────────────────┘
                │ HTTP REST (JSON) — polling for logs
┌───────────────▼──────────────────────────────────────────────────────┐
│                      Go REST API (net/http)                           │
│   cmd/rtx/main.go  →  rtx serve  →  internal/api/                   │
│                                                                       │
│   ┌──────────────┐  ┌──────────────┐  ┌────────────────────────┐    │
│   │  Process     │  │  Process     │  │  Log                   │    │
│   │  CRUD        │  │  Control     │  │  Retrieval             │    │
│   │  Handlers    │  │  Handlers    │  │  Handlers              │    │
│   │  GET/POST/   │  │  POST        │  │  GET                   │    │
│   │  DELETE      │  │  start/stop  │  │  /logs                 │    │
│   └──────┬───────┘  └──────┬───────┘  └──────────┬─────────────┘    │
└──────────┼─────────────────┼────────────────────-─┼──────────────────┘
           │                 │                       │
           └─────────────────┼───────────────────────┘
                             │ method calls (same process)
┌────────────────────────────▼─────────────────────────────────────────┐
│                    internal/scheduler/                                 │
│                                                                       │
│   ┌──────────────────────────────────────────────────────────────┐   │
│   │  Scheduler                                                    │   │
│   │  mu sync.RWMutex                                              │   │
│   │  processes map[string]*ManagedProcess                         │   │
│   │                                                               │   │
│   │  Add(def ProcessDef) error                                    │   │
│   │  Start(id string) error       ← resolves deps, calls runner   │   │
│   │  Stop(id string) error        ← SIGTERM → wait               │   │
│   │  Status(id string) ProcessStatus                              │   │
│   │  Logs(id string) []string                                     │   │
│   └──────────────────────────────────────────────────────────────┘   │
│                                                                       │
│   ┌────────────────────────────────────────────────────────────────┐ │
│   │  ManagedProcess                                                 │ │
│   │  def     ProcessDef   (name, command, args, deps, restart)     │ │
│   │  state   State        (idle/starting/running/stopping/stopped) │ │
│   │  cmd     *exec.Cmd    (nil when not running)                   │ │
│   │  logs    []string     (ring buffer, capped)                    │ │
│   │  pid     int                                                   │ │
│   │  exitCode int                                                  │ │
│   │  restarts int                                                  │ │
│   └────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────┬───────────────────────────────┘
                                       │ exec.Command (reuses runner patterns)
┌──────────────────────────────────────▼───────────────────────────────┐
│                          OS Process Layer                              │
│                                                                       │
│   ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐    │
│   │ Child PID  │  │ Child PID  │  │ Child PID  │  │ Child PID  │    │
│   │ (process A)│  │ (process B)│  │ (process C)│  │ ...        │    │
│   │ logs→buf   │  │ logs→buf   │  │ logs→buf   │  │            │    │
│   └────────────┘  └────────────┘  └────────────┘  └────────────┘    │
└───────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | New or Modified |
|-----------|----------------|-----------------|
| `cmd/rtx/main.go` | CLI dispatch — `run` delegates to `process.Run()`, `serve` starts API server | Modified: add `serve` subcommand |
| `internal/process/runner.go` | Single-process `Run()` — unchanged v1.0 implementation | Unchanged |
| `internal/scheduler/` | Own all multi-process state; resolve dependency ordering; enforce restart policies; capture logs | New |
| `internal/api/` | HTTP handlers; JSON encode/decode; route to scheduler methods; CORS headers | New (full rewrite of deleted legacy) |
| `web/` (React) | Browser UI — CRUD, start/stop, log polling | New |

---

## Recommended Project Structure

```
Runtime-X/
├── cmd/
│   └── rtx/
│       └── main.go              # Extended: `run` + `serve` subcommands
│
├── internal/
│   ├── process/
│   │   ├── runner.go            # Unchanged v1.0 single-process runner
│   │   └── runner_test.go       # Unchanged v1.0 tests
│   ├── scheduler/
│   │   ├── scheduler.go         # Scheduler struct, Add/Start/Stop/Status/Logs
│   │   ├── process.go           # ManagedProcess, ProcessDef, State enum
│   │   ├── deps.go              # Dependency resolution (topological sort)
│   │   ├── restart.go           # RestartPolicy, backoff logic
│   │   └── scheduler_test.go    # Unit tests
│   └── api/
│       ├── server.go            # http.Server setup, route registration
│       ├── handlers.go          # HTTP handlers calling scheduler methods
│       ├── middleware.go        # CORS, logging middleware
│       └── api_test.go          # Handler tests (httptest.NewRecorder)
│
├── web/                         # React app (separate from Go module)
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/
│   │   │   ├── ProcessList.tsx
│   │   │   ├── ProcessForm.tsx
│   │   │   └── LogViewer.tsx
│   │   └── api/
│   │       └── client.ts        # fetch wrappers for REST endpoints
│   ├── package.json
│   └── vite.config.ts
│
├── bin/                         # Compiled binaries
│   └── rtx                      # Single binary: handles both `run` and `serve`
│
├── go.mod
└── go.sum
```

### Structure Rationale

- **Single binary (`bin/rtx`):** `rtx run` and `rtx serve` share the same binary. This matches the v1.0 convention and avoids a separate API server binary. The `serve` subcommand starts the HTTP server in-process.
- **`internal/scheduler/`:** Isolated from `internal/api/` so the scheduler can be unit-tested without HTTP. The API layer is a thin adapter; all logic lives in the scheduler.
- **`internal/api/` (rewritten):** The legacy `internal/api/handlers.go` is a broken stub (references undefined `h.Scheduler` and `models` package). Delete and rewrite from scratch. The package name stays `api` for import clarity.
- **`web/`:** React lives outside the Go module entirely. The Go API serves built static files in production; in development the React dev server proxies to the Go API. The `web/` directory is at the project root, parallel to `cmd/` and `internal/`.
- **`deps.go` and `restart.go` separate files:** Dependency resolution and restart policy are non-trivial logic. Keeping them in their own files makes them independently testable and reviewable.

---

## Architectural Patterns

### Pattern 1: Scheduler as In-Memory Registry with Mutex

**What:** A single `Scheduler` struct holds a `sync.RWMutex` and a `map[string]*ManagedProcess`. All API handlers call methods on a single scheduler instance passed at server startup. No database. No channels between scheduler and handlers.

**When to use:** Single-server, in-memory-only requirement (stated in PROJECT.md "Out of Scope: State persistence to disk"). The mutex protects the map; read-heavy operations (list, status, logs) use `RLock`; write operations (add, start, stop) use `Lock`.

**Trade-offs:** Simple and correct. Cannot survive server restart. Acceptable per spec.

**Example:**
```go
// internal/scheduler/scheduler.go
type Scheduler struct {
    mu        sync.RWMutex
    processes map[string]*ManagedProcess
}

func (s *Scheduler) Status(id string) (ProcessStatus, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    p, ok := s.processes[id]
    if !ok {
        return ProcessStatus{}, ErrNotFound
    }
    return p.snapshot(), nil  // returns a copy — safe to use outside lock
}
```

### Pattern 2: Logs Captured via io.Writer to Ring Buffer

**What:** Instead of direct `cmd.Stdout = os.Stdout` (v1.0 pattern), the scheduler configures each child's stdout/stderr to write into a capped in-memory ring buffer. The API then reads from this buffer for log retrieval. The `io.MultiWriter` can be used to also forward to `os.Stdout` for local debugging.

**When to use:** Any time log retrieval via HTTP is required. The v1.0 pattern of direct fd inheritance cannot be used for multi-process because (a) output from multiple processes would interleave on the terminal without identification, and (b) the API has no way to retrieve logs that have already scrolled past.

**Trade-offs:** Memory capped by ring buffer size. Old logs are evicted. Polling (not streaming) is acceptable per PROJECT.md spec.

**Example:**
```go
// internal/scheduler/process.go
type logBuffer struct {
    mu    sync.Mutex
    lines []string
    cap   int      // e.g. 1000 lines
}

func (lb *logBuffer) Write(p []byte) (n int, err error) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
        if len(lb.lines) >= lb.cap {
            lb.lines = lb.lines[1:] // evict oldest
        }
        lb.lines = append(lb.lines, line)
    }
    return len(p), nil
}

// In ManagedProcess.Start():
buf := &logBuffer{cap: 1000}
cmd.Stdout = buf
cmd.Stderr = buf
```

### Pattern 3: Dependency Ordering via Topological Sort

**What:** Before starting a process, resolve its `DependsOn` list into a start order. Use a simple topological sort (Kahn's algorithm) on the dependency graph. If a cycle is detected, return an error — do not start anything.

**When to use:** Any process has a non-empty `DependsOn` list. For processes with no dependencies, start immediately. The sort runs at `Start()` time, not at `Add()` time, so dependencies can be defined before referenced processes exist.

**Trade-offs:** In-memory graph walk is O(V+E) — negligible for single-machine process counts (< 100 processes). Cycle detection is mandatory; without it, the scheduler can deadlock.

**Example:**
```go
// internal/scheduler/deps.go
// topoSort returns process IDs in start order or error if cycle detected.
func topoSort(start string, processes map[string]*ManagedProcess) ([]string, error) {
    visited := map[string]bool{}
    order   := []string{}
    var visit func(id string) error
    visit = func(id string) error {
        if visited[id] {
            return nil
        }
        p, ok := processes[id]
        if !ok {
            return fmt.Errorf("unknown dependency: %s", id)
        }
        visited[id] = true
        for _, dep := range p.def.DependsOn {
            if err := visit(dep); err != nil {
                return err
            }
        }
        order = append(order, id)
        return nil
    }
    if err := visit(start); err != nil {
        return nil, err
    }
    return order, nil
}
```

### Pattern 4: Restart Policy with Exponential Backoff

**What:** When a managed process exits, check its `RestartPolicy`. If `always` or `on-failure`, schedule a restart after a delay. The delay doubles on each restart attempt up to a configured maximum. Track restart count per process. Run the restart timer in a goroutine spawned at process exit detection.

**When to use:** When `ManagedProcess.def.RestartPolicy != "never"`.

**Trade-offs:** The goroutine-per-restart approach is simple and correct for low process counts. A single restart-timer goroutine is cleaner than a ticker-based approach for sporadic restarts.

**Example:**
```go
// internal/scheduler/restart.go
type RestartPolicy struct {
    Mode        string        // "never", "always", "on-failure"
    MaxRestarts int           // 0 = unlimited
    InitialWait time.Duration // first retry delay (e.g. 1s)
    MaxWait     time.Duration // max backoff cap (e.g. 30s)
}

func (s *Scheduler) scheduleRestart(p *ManagedProcess, exitCode int) {
    if p.def.Restart.Mode == "never" {
        return
    }
    if p.def.Restart.Mode == "on-failure" && exitCode == 0 {
        return
    }
    if p.def.Restart.MaxRestarts > 0 && p.restarts >= p.def.Restart.MaxRestarts {
        return
    }
    delay := p.def.Restart.InitialWait * (1 << p.restarts) // 2^n backoff
    if delay > p.def.Restart.MaxWait {
        delay = p.def.Restart.MaxWait
    }
    go func() {
        time.Sleep(delay)
        p.restarts++
        s.Start(p.def.ID) // re-enters start logic
    }()
}
```

### Pattern 5: REST API as Thin Adapter Layer

**What:** HTTP handlers do only three things: (1) decode the request body, (2) call a scheduler method, (3) encode the response. No business logic in handlers. All validation is the scheduler's responsibility.

**When to use:** Always. Keeping handlers thin makes them independently testable with `httptest.NewRecorder` without needing a real scheduler.

**Example:**
```go
// internal/api/handlers.go
func (h *Handler) handleStart(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id") // Go 1.22+ net/http path value extraction
    if err := h.scheduler.Start(id); err != nil {
        writeError(w, err)
        return
    }
    writeJSON(w, http.StatusAccepted, map[string]string{"status": "starting"})
}
```

---

## Data Flow

### Process Create and Start Flow

```
React user fills form → POST /api/processes
    ↓
internal/api/handlers.go
    Decode JSON body → ProcessDef{name, command, args, dependsOn, restart}
    Call scheduler.Add(def)
    ↓
internal/scheduler/scheduler.go
    Lock; validate def; store in processes map; Unlock
    Return 201 Created with {id, status: "idle"}
    ↓
React receives 201 → show process in list with status "idle"
    ↓
React user clicks Start → POST /api/processes/{id}/start
    ↓
internal/api/handlers.go
    Extract id from URL path
    Call scheduler.Start(id)
    ↓
internal/scheduler/scheduler.go
    Lock; resolve dependency order via topoSort; Unlock
    For each dep in order: ensure dep is running (start if needed)
    Start target: exec.Command(cmd, args...)
        cmd.Stdout = logBuffer
        cmd.Stderr = logBuffer
        cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
        cmd.Start()
    Update state → "running"; record PID
    Spawn goroutine: wait for exit → update state; scheduleRestart if policy
    Return nil (success)
    ↓
API returns 202 Accepted
```

### Log Polling Flow

```
React LogViewer mounts → setInterval(fetchLogs, 2000)
    ↓
GET /api/processes/{id}/logs
    ↓
internal/api/handlers.go
    Call scheduler.Logs(id)
    ↓
internal/scheduler/scheduler.go
    RLock; copy logBuffer.lines; RUnlock
    Return []string
    ↓
API returns JSON array of log lines
    ↓
React renders log lines in scrollable div
    Updates on each poll tick
```

### Process State Machine

```
              Add()
idle ──────────────────────────────────┐
  │                                    │ (initial state on Add)
  ▼
Start() called
  │
  ▼                    dependency not ready
starting ──────────────────────────────► error returned, stays idle
  │
  │ cmd.Start() succeeds
  ▼
running
  │             │
  │ Stop()      │ process exits naturally
  ▼             ▼
stopping      stopped ──────► restart policy check
  │                               │
  │ cmd.Wait() returns             │ policy == "always" or "on-failure"+code!=0
  ▼                               ▼
stopped                        [wait backoff delay]
                                   │
                                   ▼
                                starting (restart loop)
```

### Signal Flow for Stop()

```
React clicks Stop → POST /api/processes/{id}/stop
    ↓
scheduler.Stop(id)
    ↓
Lock; set state = stopping; Unlock
    ↓
cmd.Process.Signal(syscall.SIGTERM)
    ↓
Wait goroutine (already running from Start) receives exit
    ↓
Sets state = stopped; exitCode recorded; scheduleRestart evaluated
```

---

## API Endpoints

These endpoints are what the React frontend calls. They define the contract between `internal/api/` and `web/`.

| Method | Path | Request Body | Response | Description |
|--------|------|-------------|----------|-------------|
| GET | `/api/processes` | — | `[]{id, name, status, pid, restarts}` | List all processes |
| POST | `/api/processes` | `{name, command, args, dependsOn, restart}` | `{id, name, status}` | Create process definition |
| GET | `/api/processes/{id}` | — | `{id, name, status, pid, restarts, exitCode}` | Get process detail |
| DELETE | `/api/processes/{id}` | — | 204 No Content | Remove process (must be stopped) |
| POST | `/api/processes/{id}/start` | — | `{status: "starting"}` | Start a process |
| POST | `/api/processes/{id}/stop` | — | `{status: "stopping"}` | Stop a process |
| GET | `/api/processes/{id}/logs` | — | `{lines: [string]}` | Get captured log lines |
| GET | `/api/health` | — | `{status: "ok"}` | Health check |

**Routing:** Use Go 1.22+ `net/http` native path parameters (`{id}` in pattern, `r.PathValue("id")`). No external router required. Go 1.22 `ServeMux` supports method-scoped patterns (`"POST /api/processes/{id}/start"`).

**CORS:** Add a CORS middleware that sets `Access-Control-Allow-Origin: *` (or the configured origin) and handles `OPTIONS` preflight requests. Required because the React dev server runs on a different port.

---

## Integration Points: Existing Runner Reused in Scheduler

This is the critical integration point. The scheduler does NOT call `process.Run()` from v1.0. `process.Run()` is a blocking function designed for the CLI use case — it holds signal handling and blocks until the child exits. The scheduler needs non-blocking process lifecycle management.

The scheduler REUSES the patterns from `runner.go` (not the function itself):
- `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` — isolate child
- `cmd.Start()` not `cmd.Run()` — non-blocking launch
- `doneCh` goroutine pattern — non-blocking wait
- `errors.Is(err, os.ErrProcessDone)` — handle signal-to-dead-process race
- `resolveExitCode()` logic — extract POSIX exit code

The `process` package's public API is unchanged. `process.Run()` continues to be called by `cmd/rtx/main.go` for the `rtx run` subcommand.

| v1.0 pattern | v1.1 adaptation |
|---|---|
| `cmd.Stdout = os.Stdout` | `cmd.Stdout = logBuffer` (capture for API) |
| `signal.Notify(sigCh, ...)` | Not used in scheduler — scheduler calls `cmd.Process.Signal()` directly on Stop() |
| Blocking select on `sigCh` / `doneCh` | Non-blocking: goroutine waits on `doneCh`; scheduler's `Stop()` signals externally |
| Returns `int` (exit code) | Goroutine stores exit code in `ManagedProcess.exitCode` |

---

## Component Boundaries (New vs. Existing)

### What Talks to What

| From | To | Interface | Notes |
|------|----|-----------|-------|
| `cmd/rtx/main.go` `run` case | `internal/process` | `process.Run(name, args) int` | Unchanged from v1.0 |
| `cmd/rtx/main.go` `serve` case | `internal/api` | `api.NewServer(sched).ListenAndServe(addr)` | New |
| `internal/api/handlers.go` | `internal/scheduler` | `scheduler.Add/Start/Stop/Status/Logs` methods | New |
| `internal/scheduler/scheduler.go` | `os/exec` | `exec.Command`, `cmd.Start`, goroutine `cmd.Wait` | New, uses v1.0 patterns |
| `web/` React | `internal/api` | HTTP REST / JSON over fetch() | New |

### What Must NOT Talk to What

| Package | Must NOT depend on | Reason |
|---------|--------------------|--------|
| `internal/scheduler` | `internal/process` | Scheduler reimplements process lifecycle — calling `process.Run()` would block |
| `internal/scheduler` | `internal/api` | One-way dependency: api depends on scheduler, not vice versa |
| `internal/api` | `os/exec` | API layer must not manage processes directly — all process logic is scheduler's |
| `cmd/rtx/main.go` | `internal/scheduler` | The `run` subcommand uses `process.Run()`, not the scheduler |

---

## Build Order (Dependencies Between Components)

The dependency graph forces this build sequence:

### Step 1: Codebase Cleanup

Delete all legacy Docker code before writing any new code. This eliminates the broken build in `internal/api/handlers.go` and removes confusion between old and new packages. After deletion, `go build ./...` should compile with only `cmd/rtx/` and `internal/process/`.

### Step 2: `internal/scheduler/` — Process Struct and State

Build the data structures first: `ProcessDef`, `ManagedProcess`, `State` enum, `logBuffer`. No scheduler logic yet. Just the types and the `logBuffer.Write()` implementation. Testable in isolation.

### Step 3: `internal/scheduler/` — Start and Stop (No Deps, No Restart)

Implement `Scheduler.Add()`, `Scheduler.Start()` (for a process with no dependencies), and `Scheduler.Stop()`. Uses `exec.Command`, `cmd.Start()`, goroutine `cmd.Wait()` pattern. Write unit tests using `sleep`, `true`, `false` as test commands.

### Step 4: `internal/scheduler/` — Dependency Ordering

Add `deps.go` topological sort. Add `DependsOn` to `ProcessDef`. Update `Start()` to resolve and start dependencies in order. Test with multi-process scenarios.

### Step 5: `internal/scheduler/` — Restart Policies

Add `restart.go` with `RestartPolicy` and backoff logic. Hook into the `cmd.Wait()` goroutine to call `scheduleRestart`. Test with `on-failure` and `always` policies.

### Step 6: `internal/api/` — HTTP Server and Handlers

Build the API layer. It depends only on `internal/scheduler/`. Use `httptest.NewRecorder` for handler tests without a real server. Test CORS headers and error responses.

### Step 7: `cmd/rtx/main.go` — Add `serve` Subcommand

Extend the CLI switch statement to handle `serve [--addr :8080]`. Instantiates the scheduler, instantiates the server, calls `ListenAndServe`. Minimal code — all logic is in `internal/api/` and `internal/scheduler/`.

### Step 8: `web/` — React Frontend

Build the React app. During development, run `vite` with a proxy to `:8080`. The API is fully testable at this point via curl / Postman. The frontend adds browser UI but does not block API verification.

### Dependency Graph

```
web/ (React)
    └── depends on → internal/api/ (HTTP endpoints)
            └── depends on → internal/scheduler/
                    └── depends on → os/exec (stdlib)
                    └── depends on → sync (stdlib)
                    └── depends on → time (stdlib)

cmd/rtx/main.go
    ├── depends on → internal/process/ (for `run` subcommand) ← UNCHANGED
    └── depends on → internal/api/ (for `serve` subcommand)

internal/process/ (UNCHANGED)
    └── depends on → os/exec, os/signal, syscall (stdlib)
```

No circular dependencies. `internal/scheduler/` has zero dependency on `internal/api/` or `internal/process/`.

---

## Anti-Patterns

### Anti-Pattern 1: Calling process.Run() from the Scheduler

**What people do:** Re-use the existing `process.Run(name, args)` function in the scheduler since it already handles signals, exit codes, and zombie prevention.

**Why it's wrong:** `process.Run()` is a blocking call that holds the goroutine until the child exits. The scheduler needs to track multiple processes concurrently, and calling `process.Run()` in goroutines per process forfeits all control — the scheduler cannot signal a specific process, cannot access the `*exec.Cmd` to call `Signal()`, and cannot capture logs to a buffer.

**Do this instead:** Reuse the patterns from `runner.go` (Start/doneCh goroutine/Setpgid), not the function. The scheduler manages `*exec.Cmd` directly.

---

### Anti-Pattern 2: Global Mutable State Without a Mutex

**What people do:** Use a package-level `var processes = map[string]*ManagedProcess{}` and access it from multiple goroutines (the HTTP server goroutines + the per-process wait goroutines).

**Why it's wrong:** The Go race detector will flag this immediately. Map reads and writes from concurrent goroutines without synchronization cause data races — undefined behavior, possible crashes, corrupted data.

**Do this instead:** Embed `sync.RWMutex` in the `Scheduler` struct. Use `RLock/RUnlock` for reads (list, status, logs) and `Lock/Unlock` for writes (add, start, stop). Return copies of process state rather than pointers, so callers cannot mutate state outside the lock.

---

### Anti-Pattern 3: Shared Log Buffer Without Lock

**What people do:** Write to the log ring buffer from the child process's stdout goroutine while the HTTP handler reads from it for log retrieval, with no synchronization.

**Why it's wrong:** Two goroutines accessing the same slice — the write goroutine appending lines, the read goroutine iterating — is a data race. Go's slice is not thread-safe.

**Do this instead:** Give `logBuffer` its own `sync.Mutex`. `Write()` acquires the lock before modifying `lines`. `Lines()` (the read accessor) acquires the lock and returns a copy of the slice.

---

### Anti-Pattern 4: Storing Log Lines as a Growing Slice

**What people do:** `logs = append(logs, line)` indefinitely, reasoning that the process will be stopped eventually.

**Why it's wrong:** A long-running process with verbose output (e.g., `rtx run yes`) fills memory unboundedly. Since persistence is out of scope, there is no way to drain old logs.

**Do this instead:** Implement a ring buffer capped at a configurable line count (e.g., 1,000 lines). When at capacity, evict the oldest line before appending the new one.

---

### Anti-Pattern 5: REST Handler Starting Blocking exec.Command.Run()

**What people do:** Call `cmd.Run()` inside an HTTP handler goroutine to start a process and wait for it.

**Why it's wrong:** The HTTP handler is blocked for the lifetime of the child process. Concurrent requests time out. The handler cannot respond "202 Accepted" — it can only respond after the process exits.

**Do this instead:** HTTP handler calls `scheduler.Start(id)` which returns immediately after `cmd.Start()`. The child is managed by the scheduler's goroutine. The handler returns 202 Accepted.

---

### Anti-Pattern 6: No CORS Middleware

**What people do:** Build the REST API without CORS headers, then discover the React SPA (running on `localhost:5173`) cannot talk to the Go API (running on `localhost:8080`).

**Why it's wrong:** Browsers enforce the Same-Origin Policy. A request from `localhost:5173` to `localhost:8080` is cross-origin and will be blocked without appropriate `Access-Control-Allow-*` headers.

**Do this instead:** Add a CORS middleware that wraps all API routes. In development, allow `*`. In production, allow only the specific origin. Handle `OPTIONS` preflight requests with the correct response headers and `204 No Content`.

---

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 1-50 processes | Current design — in-memory map + mutex is correct and sufficient |
| 50-500 processes | Consider read path optimization: snapshot the map to a slice at list time rather than locking for each status check |
| Restart persistence | Would require writing state to disk (SQLite or JSON file) — explicitly out of scope for v1.1 |
| Multiple servers | Would require distributed state — explicitly out of scope; single-user, single-server design |

The v1.1 design intentionally does not build for scale. A local process manager on a single machine will not approach the 50-process boundary in normal use.

---

## Sources

- `https://pkg.go.dev/os/exec` — `exec.Cmd` struct, `Start`, `Wait`, `SysProcAttr` — HIGH confidence, official Go stdlib
- `https://pkg.go.dev/net/http` — `ServeMux` pattern routing (Go 1.22+), `r.PathValue()`, middleware pattern — HIGH confidence, official Go stdlib
- `https://pkg.go.dev/sync` — `sync.RWMutex`, `RLock/RUnlock` — HIGH confidence, official Go stdlib
- `https://go.dev/doc/go1.22` — Confirmed `ServeMux` method+path routing and `PathValue()` available in Go 1.22+; project uses Go 1.25.5 which includes this — HIGH confidence
- `https://pkg.go.dev/net/http/httptest` — `NewRecorder` for handler unit tests — HIGH confidence, official Go stdlib
- Runtime-X v1.0 codebase analysis — `internal/process/runner.go`, `cmd/rtx/main.go`, `internal/api/handlers.go` (legacy) — HIGH confidence, direct code read
- Process supervisor reference architectures: `supervisord`, `foreman`, `overmind` — analyzed for scheduler state machine design — MEDIUM confidence

---

*Architecture research for: Runtime X v1.1 — multi-process scheduler, Go REST API, React frontend*
*Researched: 2026-02-28*
