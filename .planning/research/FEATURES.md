# Feature Research

**Domain:** Multi-process scheduler, Go REST API, React frontend (Runtime X v1.1)
**Researched:** 2026-03-01
**Confidence:** HIGH — grounded in codebase analysis, supervisord docs, PM2 feature set, systemd restart policy design, React polling patterns, and Go net/http stdlib patterns

> **Note:** This file supersedes the v1.0 FEATURES.md (single-process CLI runner). v1.0 features are complete and shipped. This document covers ONLY the delta for v1.1. See ARCHITECTURE.md for how new features integrate with `internal/process/runner.go`.

---

## Capability Areas

v1.1 adds four distinct capability areas. Each area has its own table-stakes/differentiators/anti-features analysis.

1. **Multi-Process Scheduler** — internal state machine managing N processes concurrently
2. **Dependency Ordering** — start order resolution with cycle detection
3. **Restart Policies with Exponential Backoff** — per-process failure recovery
4. **Go REST API** — HTTP layer exposing scheduler operations
5. **React Frontend** — browser UI for all of the above
6. **Polling-Based Log Viewer** — in-browser access to captured output

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist in a multi-process manager with a web UI. Missing these = product feels incomplete or broken.

| Feature | Why Expected | Complexity | Depends On (runner.go) |
|---------|--------------|------------|------------------------|
| Register multiple named processes | Users define what to run — without this there's nothing to manage | LOW | None (new ProcessDef struct) |
| Start a named process | Core action — can't manage what you can't start | LOW | Reuses `cmd.Start()` + `Setpgid` patterns from runner.go |
| Stop a named process with SIGTERM | Core action — graceful stop is table stakes for any supervisor | LOW | Reuses `cmd.Process.Signal(syscall.SIGTERM)` from runner.go |
| Per-process status (idle/running/stopped) | Without status, users can't tell if anything worked | LOW | New state field on ManagedProcess struct |
| List all processes with status | PM2, supervisord, every manager shows a process list | LOW | New Scheduler.List() reading process map |
| Real-time PID tracking | Every manager shows PID; needed for external kill, debugging | LOW | `cmd.Process.Pid` after Start(), same as runner.go |
| Exit code recording | Operators need to know why a process stopped | LOW | `resolveExitCode()` pattern from runner.go, stored in ManagedProcess |
| Log capture to in-memory buffer | Logs must be retrievable via API — direct fd to os.Stdout won't work for multi-process | MEDIUM | Replaces `cmd.Stdout = os.Stdout` with `cmd.Stdout = logBuffer` — critical change from runner.go |
| REST API: GET /api/processes | List endpoint is the foundation for any UI | LOW | None (API adapter over scheduler) |
| REST API: POST /api/processes | Create process definition | LOW | None |
| REST API: POST /api/processes/{id}/start | Start action | LOW | None |
| REST API: POST /api/processes/{id}/stop | Stop action | LOW | None |
| REST API: GET /api/processes/{id}/logs | Log retrieval for polling | LOW | None |
| REST API: DELETE /api/processes/{id} | Cleanup — must be able to remove a stopped process | LOW | None |
| CORS middleware | React dev server on port 5173 calling Go API on port 8080 requires CORS headers — without this nothing works | LOW | None |
| React: process list view | Primary UI — shows all processes and their statuses | LOW | None |
| React: start/stop buttons per process | Fundamental controls — users expect to click to act | LOW | None |
| React: log viewer with auto-refresh | Users expect to see logs without manually refreshing | MEDIUM | None |
| React: process creation form | Users need to define new processes | MEDIUM | None |
| Zombie prevention for all children | Every managed process needs `cmd.Wait()` called — same discipline as runner.go but for N processes | LOW | Same doneCh goroutine pattern from runner.go, per-process |

### Differentiators (Competitive Advantage)

Features that set Runtime X apart. Not expected baseline, but meaningful.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Dependency-aware start ordering | Process B waits for A before starting — most simple process managers (foreman, honcho) lack this | MEDIUM | Topological sort (Kahn's algorithm or DFS) on `DependsOn` list; cycle detection mandatory |
| Exponential backoff restart policy | `always` / `on-failure` / `never` with `initialWait`, `maxWait`, `maxRestarts` — configurable per process | MEDIUM | `2^n * initialWait` capped at `maxWait`; goroutine-per-restart pattern; systemd and Temporal use this model |
| Per-process restart counter | Shows how many times a process crashed and restarted — makes failure patterns visible | LOW | `restarts int` on ManagedProcess; increment on each restart; surfaced in API response |
| Log ring buffer (capped, not unbounded) | Prevents memory exhaustion from verbose long-running processes | LOW | Fixed-cap slice with FIFO eviction; 1,000 lines default |
| Process group isolation per child | `Setpgid: true` isolates each child; stop targets the exact PID, not a group | LOW | Direct reuse from runner.go; no new logic |
| Single binary for CLI and API server | `rtx run` and `rtx serve` from same binary — no separate daemon | LOW | Subcommand dispatch already in cmd/rtx/main.go |
| REST API health check endpoint | `GET /api/health` lets load balancers and monitoring detect when the server is up | LOW | Returns `{"status": "ok"}` — trivial but expected |

### Anti-Features (Explicitly NOT Built for v1.1)

Features that seem reasonable but are explicitly out of scope. Each has a documented reason.

| Anti-Feature | Why Requested | Why Problematic for v1.1 | What to Do Instead |
|--------------|---------------|--------------------------|-------------------|
| WebSocket / SSE log streaming | "Real-time" feels better than polling | Adds connection lifecycle management, reconnection handling, and a new protocol; polling at 2s interval is imperceptible to users and accepted in PROJECT.md scope | Use `setInterval(fetchLogs, 2000)` in React — sufficient and correct |
| Log offset tracking (tail from position N) | Avoids re-fetching lines already seen | Requires cursor/offset state on client, offset parameter on server, and atomicity guarantees when buffer evicts | Return all buffered lines on every poll; 1,000-line cap keeps payload small |
| State persistence to disk | Process definitions survive server restart | SQLite/file I/O adds failure modes (corrupt file, disk full, permissions); out of scope per PROJECT.md | In-memory only; users recreate definitions after restart |
| Config file (YAML/TOML) loading on startup | Processes auto-start without manual API calls | Requires format decisions, schema validation, versioning, and a separate code path from the API; out of scope per PROJECT.md | All process definitions come via API POST; no file parsing |
| Process dependency graph visualization | Shows dependency arrows in the browser | Requires a graph rendering library (D3, dagre, reactflow); significant frontend complexity for minimal utility | Show `dependsOn` as a text field in the process detail view |
| WebSocket-based process console (interactive stdin) | Allows users to type into a running process | Requires bidirectional streaming, session management, terminal emulation — complexity far beyond v1.1 | Not supported; processes get no stdin from the UI (cmd.Stdin = nil for managed processes) |
| OAuth / user authentication | Multi-user access control | Auth adds token management, session storage, refresh flow — single-user per PROJECT.md | No auth; single user, local network use |
| Process metrics (CPU %, memory) | Dashboards feel professional | Requires `/proc` parsing (Linux-specific) or platform API calls; adds polling loop separate from log polling; out of scope per PROJECT.md | Status (state + PID + restarts + exitCode) is sufficient |
| "Start all" / "Stop all" batch operations | Convenient shortcut | Dependency ordering makes "start all" non-trivial (ordering must be respected); without it, batch start is incorrect | Start each process individually in order; the dependency resolver handles ordering per-process |
| Restart count reset | Allow clearing the restart counter | Operational complexity; confusing semantics when process is running | Restart count is informational only; resets when process is deleted and recreated |
| Process groups / namespaces / isolation | Container-like isolation | Requires elevated privileges; completely out of scope for a user-space manager | Docker handles isolation; rtx runs real OS processes directly |
| Daemon mode / background mode | Run `rtx serve` as a background service | Double-fork, PID file, log redirection — out of scope per PROJECT.md; use systemd/launchd for production deployment | `rtx serve` runs in the foreground; the caller handles backgrounding if needed |
| Jitter on restart backoff | Prevents thundering herd when N processes all restart simultaneously | At single-machine process counts (< 50), thundering herd is not a real problem; adds complexity for no practical benefit | Pure exponential backoff (2^n) without jitter is correct for v1.1 |
| Per-process environment variable overrides | Fine-grained env config | Adds a new field to ProcessDef, JSON encoding complexity, and security considerations — not core | Out of scope; processes inherit the server's environment |

---

## Feature Dependencies

```
[Codebase Cleanup]
    └──required before──> [Scheduler] (removes broken internal/api/handlers.go that won't compile)
    └──required before──> [REST API] (eliminates name collision with legacy internal/api package)

[Scheduler: Process Struct + State Machine]
    └──required for──> [Scheduler: Start/Stop]
    └──required for──> [Scheduler: Log Buffer]
    └──required for──> [REST API Handlers]

[Scheduler: Start/Stop (no deps, no restart)]
    └──required for──> [Scheduler: Dependency Ordering]
    └──required for──> [Scheduler: Restart Policies]
    └──required for──> [REST API: start/stop endpoints]

[Scheduler: Dependency Ordering]
    └──enhances──> [Scheduler: Start] (Start() calls topoSort before launching)

[Scheduler: Log Buffer]
    └──required for──> [REST API: GET /logs endpoint]
    └──conflicts──> [runner.go direct fd pattern] (cmd.Stdout = logBuffer replaces cmd.Stdout = os.Stdout)

[REST API: All Endpoints]
    └──required for──> [React Frontend]

[REST API: GET /logs]
    └──required for──> [React: Log Viewer]
    └──required for──> [React: polling loop]

[Scheduler: Restart Policies]
    └──required for──> [REST API: restart fields in ProcessDef POST body]
    └──required for──> [React: restart policy form fields]

[CORS Middleware]
    └──required for──> [React Frontend] (dev server on different port)

[React: Process List]
    └──required for──> [React: Start/Stop Buttons]
    └──required for──> [React: Log Viewer navigation]

[React: Process Creation Form]
    └──required for──> [React: Start/Stop Buttons] (process must exist before it can be started)
```

### Dependency Notes

- **Codebase cleanup is a hard prerequisite.** The legacy `internal/api/handlers.go` references an undefined `h.Scheduler` from a deleted models package. This causes a build failure. All new code depends on `go build ./...` succeeding first.
- **Log buffer conflicts with runner.go fd pattern.** The v1.0 `cmd.Stdout = os.Stdout` is correct for a CLI runner. The v1.1 scheduler must use `cmd.Stdout = logBuffer` instead. This is a deliberate fork — runner.go is unchanged; the scheduler reimplements the exec.Cmd setup inline.
- **Dependency ordering enhances Start, not a separate operation.** `Scheduler.Start(id)` internally resolves the dependency graph. The API caller always calls a single `/start` endpoint — dependency resolution is invisible to the API layer.
- **Restart policies hook into the exit goroutine.** The `doneCh` goroutine (reused pattern from runner.go) detects process exit and calls `scheduleRestart()`. No separate restart goroutine is spawned until needed.
- **CORS is required before any React integration testing.** Without `Access-Control-Allow-Origin` headers on the Go API, every browser request from the React dev server will be blocked by the browser's same-origin policy enforcement.

---

## MVP Definition

### v1.1 Launch With

Minimum to call Runtime X a multi-process manager with a browser UI.

- [ ] Codebase cleanup (delete legacy Docker code) — required for build to succeed
- [ ] ProcessDef and ManagedProcess structs — data structures for all other scheduler features
- [ ] Scheduler Add/Start/Stop with state tracking — core lifecycle management
- [ ] Per-process log capture (ring buffer, 1,000 lines) — required for log viewer
- [ ] Dependency ordering via topological sort with cycle detection — the primary differentiator
- [ ] Restart policies: `never` / `always` / `on-failure` with exponential backoff (`initialWait`, `maxWait`, `maxRestarts`) — the second primary differentiator
- [ ] REST API: GET /api/processes, POST /api/processes, GET /api/processes/{id}, DELETE /api/processes/{id} — CRUD
- [ ] REST API: POST /api/processes/{id}/start, POST /api/processes/{id}/stop — control
- [ ] REST API: GET /api/processes/{id}/logs — log retrieval
- [ ] REST API: GET /api/health — health check
- [ ] CORS middleware — required for React dev and production
- [ ] `rtx serve` subcommand in cmd/rtx/main.go — entry point for the API server
- [ ] React: process list with status badges (idle/running/stopped) — primary UI
- [ ] React: create process form (name, command, args, dependsOn, restartPolicy) — required to populate the list
- [ ] React: start/stop buttons per process — primary controls
- [ ] React: log viewer with 2-second polling — required to observe process behavior

### Add After Validation (v1.1.x)

Features to add once v1.1 core is working and real usage patterns emerge:

- [ ] React: edit existing process definition (PUT /api/processes/{id}) — add after users confirm create/delete workflow is sufficient
- [ ] React: per-process restart count display — low effort, add when restart policies are validated
- [ ] React: visual status indicator that auto-refreshes (poll `/api/processes` on same interval as log polling) — process list should reflect status changes without manual refresh
- [ ] React: log viewer "clear" button — useful after confirmation that polling is working correctly

### Future Consideration (v2+)

Defer until v1.1 is proven:

- [ ] Config file loading (YAML/TOML) — requires format, schema, and validation design
- [ ] State persistence to disk — requires SQLite or JSON file design with corruption handling
- [ ] WebSocket / SSE log streaming — requires protocol upgrade, reconnection logic
- [ ] Interactive process console (stdin forwarding) — requires terminal emulation
- [ ] Process metrics (CPU, memory) — requires /proc parsing or cgroups
- [ ] Multi-user auth — requires session management, token refresh

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Codebase cleanup | HIGH (unblocks everything) | LOW | P1 |
| ProcessDef + ManagedProcess structs | HIGH | LOW | P1 |
| Scheduler Add/Start/Stop | HIGH | LOW | P1 |
| Per-process log buffer | HIGH | LOW | P1 |
| Dependency ordering (topoSort) | HIGH | MEDIUM | P1 |
| Restart policies (backoff) | HIGH | MEDIUM | P1 |
| REST API CRUD endpoints | HIGH | LOW | P1 |
| REST API control endpoints (start/stop) | HIGH | LOW | P1 |
| REST API log endpoint | HIGH | LOW | P1 |
| CORS middleware | HIGH (React won't work without it) | LOW | P1 |
| `rtx serve` subcommand | HIGH | LOW | P1 |
| React process list | HIGH | LOW | P1 |
| React create form | HIGH | MEDIUM | P1 |
| React start/stop buttons | HIGH | LOW | P1 |
| React log viewer (polling) | HIGH | MEDIUM | P1 |
| React status auto-refresh | MEDIUM | LOW | P2 |
| React restart count display | LOW | LOW | P2 |
| REST API health check | MEDIUM | LOW | P2 |
| WebSocket log streaming | MEDIUM | HIGH | P3 |
| State persistence | MEDIUM | HIGH | P3 |
| Config file support | MEDIUM | MEDIUM | P3 |
| Process metrics | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for v1.1 launch (product definition fails without it)
- P2: Should have; add when P1 is complete
- P3: Future consideration only

---

## Competitor Feature Analysis

Reference tools analyzed to establish what "table stakes" means at each capability layer.

| Feature | supervisord | PM2 | overmind | Runtime X v1.1 approach |
|---------|-------------|-----|----------|--------------------------|
| Multi-process management | Yes | Yes | Yes (Procfile) | Yes — in-memory scheduler |
| Process list with status | Yes (supervisorctl) | Yes (pm2 list) | Yes (tmux panes) | Yes — React list view + GET /api/processes |
| Start/stop per process | Yes | Yes | Yes | Yes — React buttons + POST /start, /stop |
| Dependency ordering | No | No (cluster mode only) | No | Yes — topological sort on DependsOn |
| Restart policy | Yes (autorestart) | Yes (--restart-delay) | Yes (Procfile restart) | Yes — never/always/on-failure with backoff |
| Exponential backoff | No (fixed interval) | Partial (--restart-delay) | No | Yes — 2^n * initialWait, capped at maxWait |
| Log capture | Yes (to files) | Yes (to files + pm2 logs) | Yes (to tmux) | Yes — in-memory ring buffer, 1,000 lines |
| Web UI for logs | Basic (supervisord web) | Yes (pm2 plus, paid) | No (tmux only) | Yes — React polling log viewer |
| Config file | Yes (ini) | Yes (ecosystem.config.js) | Yes (Procfile) | No — all config via API (by design) |
| REST API | No (XML-RPC only) | No (CLI only) | No | Yes — core feature of v1.1 |
| State persistence | Yes (to disk) | Yes | No | No — in-memory only (by design) |
| Auth | No | No | No | No |
| Single binary | No (supervisord + supervisorctl) | No (pm2 daemon + CLI) | Yes | Yes — rtx binary handles both run and serve |

**Key insight from competitor analysis:**

- supervisord is the closest full-featured analog. It has restart policies and a web UI but no REST API, no dependency ordering, and its web UI is notoriously minimal (the long-standing feature request for a proper log viewer is 10+ years old).
- PM2's log viewer requires the paid "PM2 Plus" cloud product. Runtime X providing this feature free and self-hosted is a meaningful differentiator.
- None of the reference tools combine REST API + dependency ordering + exponential backoff in a single binary. Runtime X v1.1 is unique in this combination.

---

## Integration Points with Existing runner.go

This section is specific to Runtime X. It documents which v1.0 patterns carry forward and which are adapted.

| v1.0 Pattern in runner.go | v1.1 Adaptation | Reason |
|---------------------------|-----------------|--------|
| `cmd.Stdout = os.Stdout` | `cmd.Stdout = logBuffer` | Multi-process output must be captured for API retrieval; direct fd inheritance to terminal cannot be retrieved |
| `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` | Unchanged — copied directly into scheduler Start() | Same reason: isolate child in own process group |
| `cmd.Start()` non-blocking launch | Unchanged | Scheduler needs non-blocking launch to manage N processes concurrently |
| `doneCh := make(chan error, 1); go func() { doneCh <- cmd.Wait() }()` | Unchanged pattern, extended to call `scheduleRestart()` on exit | Goroutine waits for exit, then evaluates restart policy |
| `errors.Is(err, os.ErrProcessDone)` on Signal() | Unchanged — same race exists in scheduler Stop() | Natural-exit race: process may exit between Stop() being called and Signal() being sent |
| `resolveExitCode(waitErr, cmd.ProcessState)` | Logic copied, result stored in `ManagedProcess.exitCode` instead of returned | Scheduler stores exit codes for API retrieval; runner.go returns to caller |
| `signal.Notify(sigCh, ...)` | NOT used in scheduler | Scheduler calls Stop() directly from API handler; no OS-level signal interception needed for multi-process management |
| `process.Run(name, args) int` | NOT called from scheduler | `process.Run()` is blocking and holds signal handling — incompatible with multi-process concurrent management |

**Critical distinction:** `process.Run()` remains in use only for `rtx run` (the single-process CLI). The scheduler never calls it. The scheduler reimplements the non-blocking subset of runner.go's patterns inline.

---

## Sources

- [supervisord introduction — supervisord.org](https://supervisord.org/introduction.html) — defines table stakes for process supervisors; HIGH confidence
- [supervisord logging — supervisord.org](https://supervisord.org/logging.html) — log file behavior and limitations; HIGH confidence
- [PM2 process management — pm2.keymetrics.io](https://pm2.keymetrics.io/docs/usage/process-management/) — PM2 feature set for process manager comparison; MEDIUM confidence
- [Overmind — github.com/DarthSim/overmind](https://github.com/DarthSim/overmind) — Procfile-based process manager with restart and tmux; MEDIUM confidence
- [Exponential backoff — betterstack.com](https://betterstack.com/community/guides/monitoring/exponential-backoff/) — essential parameters: initialDelay, backoff factor, maxRetries, maxDelay; HIGH confidence
- [systemd restart policy — michael.stapelberg.ch](https://michael.stapelberg.ch/posts/2024-01-17-systemd-indefinite-service-restarts/) — restart parameters (RestartSec, StartLimitIntervalSec, StartLimitBurst); HIGH confidence (2024)
- [systemd.service man page — man7.org](https://man7.org/linux/man-pages/man5/systemd.service.5.html) — restart mode values: no/always/on-success/on-failure/on-abnormal/on-abort/on-watchdog; HIGH confidence
- [Polling in React — dev.to](https://dev.to/tangoindiamango/polling-in-react-3h8a) — setInterval + useEffect + cleanup pattern; stale closure pitfall; MEDIUM confidence
- [API polling best practices — merge.dev](https://www.merge.dev/blog/api-polling-best-practices) — interval selection, error handling, cleanup; MEDIUM confidence
- [React useEffect — react.dev](https://react.dev/reference/react/useEffect) — official cleanup pattern for intervals; HIGH confidence
- [Topological sort for dependency resolution — various] — standard algorithm, HIGH confidence for cycle detection via DFS or Kahn's
- Runtime-X v1.0 codebase — `internal/process/runner.go`, `cmd/rtx/main.go` — direct analysis; HIGH confidence

---
*Feature research for: Runtime X v1.1 — multi-process scheduler, Go REST API, React frontend*
*Researched: 2026-03-01*
