# Project Research Summary

**Project:** Runtime X v1.1
**Domain:** Multi-process scheduler, Go REST API, React frontend
**Researched:** 2026-03-01
**Confidence:** HIGH

## Executive Summary

Runtime X v1.1 extends a proven v1.0 single-process CLI runner into a multi-process supervisor with a browser UI. The recommended approach is a zero-new-Go-dependencies architecture: `net/http` stdlib for the REST API (Go 1.22+ routing is fully capable for 8 endpoints), `sync.RWMutex` for in-memory scheduler state, and React 19 + Vite 7 for the frontend. The scheduler reuses patterns from `internal/process/runner.go` (Setpgid isolation, non-blocking Start, doneCh goroutine) but never calls `process.Run()` directly — that function is blocking and incompatible with concurrent multi-process management. The key differentiators over existing tools (supervisord, PM2, overmind) are dependency-aware start ordering via topological sort, exponential backoff restart policies per process, and a self-hosted REST API with a web log viewer in a single binary — a combination no reference tool provides.

The build must begin with codebase cleanup. The legacy Docker-related code (`internal/api/handlers.go`, `internal/worker/`, `cmd/main.go`, and 8 other packages) contains a broken build — `h.Scheduler` references an undefined type from a deleted models package. No new feature can be built until `go build ./...` passes cleanly on just the v1.0 CLI layer. After cleanup, the build order is strictly layered: scheduler types and log buffer first, then start/stop lifecycle, then dependency ordering, then restart policies, then the API layer, then the `serve` subcommand, then the React frontend. Each step compiles and tests independently before the next begins.

The critical risks are all concurrency-related. The log buffer must have its own `sync.Mutex` independent of the scheduler's top-level `RWMutex` — log writes come from goroutines spawned by `cmd.Start()`, not from the scheduler's own code paths, and they race with HTTP handler reads. The scheduler's write lock must be released before calling `cmd.Start()` to prevent deadlock. Restart goroutines must be cancellable via `context.CancelFunc` so that `Stop()` can abort a pending backoff sleep. Integer overflow in exponential backoff must be capped at shift operand 30 with the result capped at `MaxWait`. Run `go test -race ./...` after every scheduler step — the race detector is the single most effective verification tool.

---

## Key Findings

### Recommended Stack

The Go backend requires no new `go.mod` entries. All new server-side packages are stdlib: `net/http`, `encoding/json`, `net/http/httptest`, `sync`, `time`, and `strings`. The only third-party dependency — `github.com/google/uuid v1.6.0` — is already present in `go.mod`. Go 1.22+ `ServeMux` supports method-scoped path patterns (`"POST /api/processes/{id}/start"`) and `r.PathValue("id")` for path parameter extraction; no external router is needed for 8 endpoints. External options (chi, gin) add nothing at this scale and must not be introduced. Do not use the experimental `encoding/json/v2` — it requires `GOEXPERIMENT=jsonv2` and is not covered by the Go 1 compatibility promise.

For the frontend, `npm create vite@latest web -- --template react-ts` scaffolds a production-ready setup with React 19.2, TypeScript 5.9, and Vite 7. The Vite dev server proxy (`proxy: { '/api': 'http://localhost:8080' }`) eliminates CORS friction during development without modifying the Go server. No state management library (Redux, Zustand, Jotai), data-fetching library (TanStack Query, SWR, Axios), or streaming protocol (WebSocket, SSE) is needed — `useState` + `useEffect` + native `fetch` + `setInterval` covers all frontend requirements. Vite 7 requires Node.js 20+ (Node 18 is EOL).

**Core technologies:**
- `net/http` stdlib (Go 1.25.5): HTTP server and routing — no external router needed for 8 flat endpoints with Go 1.22+ method+path patterns
- `encoding/json` stdlib: JSON request/response marshaling — use standard v1, not experimental v2
- `net/http/httptest` stdlib: Handler unit testing — `NewRecorder()` + real Scheduler instance, no running port needed
- `sync.RWMutex` stdlib: Scheduler concurrency — `RLock` for reads (list, status, logs), `Lock` for writes (add, start, stop)
- `github.com/google/uuid` v1.6.0: Process ID generation — already in `go.mod`, no changes needed
- React 19.2 + TypeScript 5.9 + Vite 7: Frontend — scaffold once, then native `fetch` for all API calls; no additional libraries

### Expected Features

**Must have (table stakes):**
- Process registration, start, stop, and status tracking — the foundation without which there is nothing to manage
- Per-process state machine (idle/starting/running/stopping/stopped) — users need to know what changed
- Log capture to in-memory ring buffer (1,000-line cap) — the log API endpoint has nothing to return without this
- REST API — all 8 endpoints (list, create, get, delete, start, stop, logs, health) — the frontend depends on every one
- CORS middleware — without this the React dev server on port 5173 cannot reach the Go API on port 8080 at all
- `rtx serve` subcommand — entry point for the entire API and frontend layer
- React: process list, create form, start/stop buttons, polling log viewer — minimum viable browser UI
- Dependency ordering via topological sort with cycle detection — the primary differentiator; without cycle detection the scheduler can deadlock silently
- Restart policies (never/always/on-failure) with exponential backoff — the second differentiator, configurable per process

**Should have (competitive):**
- Per-process restart counter surfaced in API response and UI — makes failure patterns visible
- React process list auto-refresh (poll `/api/processes` on same 2-second interval as log polling) — without this the process list goes stale after state changes
- React log viewer polling disabled for stopped processes — avoids unnecessary network traffic
- Health check endpoint (`GET /api/health`) — enables external monitoring and load balancer checks
- Graceful shutdown: `rtx serve` sends SIGTERM to all managed processes before exiting — prevents orphaned children

**Defer (v2+):**
- Config file loading (YAML/TOML) — requires format, schema, and validation design
- State persistence to disk — requires SQLite or JSON file with corruption handling
- WebSocket/SSE log streaming — explicitly out of scope per PROJECT.md
- Interactive process console (stdin forwarding) — requires terminal emulation
- Process metrics (CPU, memory) — requires `/proc` parsing on Linux
- Multi-user auth — requires session management and token refresh

### Architecture Approach

The architecture is a strictly layered single binary: React static files (served by the Go binary in production) → Go REST API (`internal/api/` — thin adapter) → Scheduler (`internal/scheduler/` — all business logic) → OS process layer (exec.Cmd). The scheduler is completely decoupled from the API — no import of `internal/api/` and no import of `internal/process/`. HTTP handlers do exactly three things: decode request body, call a scheduler method, encode the response. All process lifecycle logic — exec.Cmd setup, Setpgid, doneCh goroutine, log buffer assignment, restart scheduling — lives exclusively in the scheduler. The codebase must be cleaned before any new code is written: 10+ legacy Docker packages must be deleted to restore `go build ./...`.

**Major components:**
1. `internal/scheduler/` — owns all multi-process state; `sync.RWMutex`-protected `map[string]*ManagedProcess`; dependency graph resolution; restart policies; per-process log ring buffers
2. `internal/api/` — Go 1.22+ `net/http` handlers (thin adapters); CORS middleware; JSON encode/decode; route registration via `ServeMux`
3. `web/` (React) — process list, create form, start/stop controls, polling log viewer; built separately via Vite; served as static files by the Go binary in production
4. `cmd/rtx/main.go` — extended with `serve` subcommand; runs `http.ListenAndServe` in a goroutine; signal handling and graceful `srv.Shutdown()` in the main goroutine

### Critical Pitfalls

1. **Calling `process.Run()` from the scheduler** — it is blocking, owns signal handling, and forfeits all control (cannot call Signal(), cannot capture logs to a buffer). The scheduler must reimplement the non-blocking patterns from `runner.go` directly using `cmd.Start()` + doneCh goroutine, and must never call `process.Run()`.

2. **Log buffer without its own mutex** — `cmd.Start()` spawns internal goroutines to copy stdout and stderr; they write to the log buffer concurrently with HTTP handler goroutines reading it. The `logBuffer` struct needs its own `sync.Mutex` independent of the scheduler's `RWMutex`. `go test -race` detects this immediately.

3. **Holding the scheduler write lock during `cmd.Start()`** — `cmd.Start()` involves a fork syscall; holding the lock during it serializes all API operations and risks deadlock if a Wait goroutine tries to acquire the same lock before `Start()` returns. Use a two-phase pattern: acquire lock to validate state, release before `cmd.Start()`, re-acquire to write pid/state.

4. **Restart goroutine not cancellable** — `time.Sleep(delay)` in a goroutine with no cancellation means `Stop()` cannot abort a pending backoff restart; the process appears to ignore stop commands. Each `ManagedProcess` needs a `context.CancelFunc`; `Stop()` calls it before sending SIGTERM.

5. **`http.ListenAndServe` blocking the main goroutine** — signal handling code placed after `ListenAndServe` is never reached; Ctrl+C kills the server hard, leaving all managed processes as orphans. Run `ListenAndServe` in a goroutine; handle signals in main; call `srv.Shutdown()` with a timeout context.

---

## Implications for Roadmap

The dependency graph in ARCHITECTURE.md, feature priorities in FEATURES.md, and pitfall phase mappings in PITFALLS.md converge on a clear 8-phase structure. There are no ambiguous sequencing decisions — architecture forces the order.

### Phase 1: Codebase Cleanup
**Rationale:** Hard prerequisite for everything else. The existing `internal/api/handlers.go` references an undefined type and the build fails. No new code can be written or tested until `go build ./...` passes on the clean v1.0 baseline. Attempting to build on a broken foundation causes confusion throughout all later phases.
**Delivers:** Clean, compiling codebase containing only `cmd/rtx/main.go`, `internal/process/runner.go`, and their tests. All 10+ legacy Docker packages deleted.
**Addresses:** "Codebase cleanup" P1 prerequisite from FEATURES.md (unblocks everything).
**Avoids:** Build failures that block compilation of any new package.
**Research flag:** No research needed — this is mechanical deletion of identified file paths.

### Phase 2: Scheduler — Data Structures and Log Buffer
**Rationale:** All scheduler functionality depends on the `ManagedProcess`, `ProcessDef`, `State` enum, and `logBuffer` types existing first. The log buffer's mutex design must be correct from the start — retrofitting it after start/stop logic is built creates a data race that is harder to isolate and debug.
**Delivers:** `internal/scheduler/process.go` with `ManagedProcess`, `ProcessDef`, `State`, and a mutex-safe `logBuffer` implementing `io.Writer`. Unit tests verifying ring buffer eviction at capacity and race-safe concurrent writes via `go test -race`.
**Implements:** Architecture Pattern 1 (Scheduler as In-Memory Registry) and Pattern 2 (Logs via io.Writer to Ring Buffer).
**Avoids:** Pitfalls 3 (logBuffer data race), 4 (unbounded log memory).
**Research flag:** No research needed — patterns are fully specified with working code examples in ARCHITECTURE.md and PITFALLS.md.

### Phase 3: Scheduler — Start, Stop, and Basic Lifecycle
**Rationale:** With types established, implement the core lifecycle without the complexity of dependency ordering or restart policies. Verifying start/stop/status with simple test commands (`sleep 5`, `true`, `false`) provides a stable, race-free foundation before layering on more complex features.
**Delivers:** `internal/scheduler/scheduler.go` with `Add`, `Start`, `Stop`, `Status`, `Logs` methods. `go test -race ./internal/scheduler/...` passing. Zombie prevention verified (Wait goroutine for every started process).
**Implements:** Architecture build order Step 3; reuses non-blocking process patterns from `runner.go`.
**Avoids:** Pitfalls 1 (calling `process.Run()`), 2 (direct fd assignment — logs go to ring buffer), 5 (lock held during `cmd.Start()`).
**Research flag:** No research needed — patterns are fully specified with code in ARCHITECTURE.md and PITFALLS.md.

### Phase 4: Scheduler — Dependency Ordering
**Rationale:** Dependency ordering is Runtime X v1.1's primary differentiator. It enhances `Start()` and must be built on a working start/stop foundation. Cycle detection is mandatory — without it, circular dependencies cause silent hangs that are difficult to debug and result in unresponsive HTTP endpoints.
**Delivers:** `internal/scheduler/deps.go` with `topoSort` using `visited` + `inStack` DFS. `Start()` resolves and starts dependencies in order. Tests covering: linear chain, diamond dependency, missing dependency error, and cycle detection returning an error immediately.
**Implements:** Architecture Pattern 3 (Dependency Ordering via Topological Sort).
**Avoids:** Pitfall 6 (dependency cycle not detected — silent deadlock on `POST /start`).
**Research flag:** No research needed — algorithm is fully specified with code in PITFALLS.md and ARCHITECTURE.md.

### Phase 5: Scheduler — Restart Policies with Exponential Backoff
**Rationale:** Restart policies are the second differentiator. They hook into the existing exit goroutine from Phase 3. Both the goroutine cancellation pattern (for Stop() during backoff) and the integer overflow cap must be implemented together — they interact and are easy to miss independently.
**Delivers:** `internal/scheduler/restart.go` with `RestartPolicy` struct, `scheduleRestart()`, `computeBackoff()`. `ManagedProcess` gets `restartCtx`/`restartCancel` fields. Tests for: `never` policy (no restart), `always` policy (restarts after exit 0), `on-failure` policy (restarts after exit 1, not after exit 0), stop-during-backoff cancellation, and overflow cap at 30 shifts.
**Implements:** Architecture Pattern 4 (Restart Policy with Exponential Backoff).
**Avoids:** Pitfalls 7 (restart goroutine not cancellable by Stop()), 8 (backoff integer overflow after many restarts).
**Research flag:** No research needed — both patterns are fully specified with working code in PITFALLS.md.

### Phase 6: REST API — HTTP Server, Handlers, and CORS
**Rationale:** The API layer depends on a complete scheduler. With all scheduler features in place, the API is a thin adapter — decode, call scheduler method, encode. CORS must be implemented in this phase because it is required for any browser-based integration testing. Handlers must use `202 Accepted` for start/stop since process lifecycle is asynchronous.
**Delivers:** `internal/api/server.go`, `handlers.go`, `middleware.go`. All 8 endpoints registered and tested with `httptest.NewRecorder()`. CORS preflight returning 204 verified with `curl -X OPTIONS -v`. Error responses mapped from scheduler errors to correct HTTP status codes (404, 409, 400).
**Implements:** Architecture Pattern 5 (REST API as Thin Adapter) and the full API endpoint contract from ARCHITECTURE.md.
**Avoids:** Pitfalls 9 (HTTP handler blocking on process start — use `202 Accepted`), 10 (missing CORS middleware blocks all browser requests).
**Research flag:** No research needed — all patterns documented with code examples in ARCHITECTURE.md and STACK.md.

### Phase 7: CLI — `rtx serve` Subcommand and Graceful Shutdown
**Rationale:** Wires everything together into a runnable binary. The signal handling pattern is critical here — `ListenAndServe` must run in a goroutine; the main goroutine handles signals; `srv.Shutdown()` is called with a timeout context; managed processes receive SIGTERM before the server exits to prevent orphaned children.
**Delivers:** Extended `cmd/rtx/main.go` with `serve [--addr :8080]` subcommand. End-to-end verification via curl: all 8 endpoints respond correctly. Ctrl+C triggers graceful shutdown within 10 seconds and `ps aux` shows no orphaned managed processes.
**Avoids:** Pitfall 12 (ListenAndServe blocking signal handler — managed processes become orphans on hard kill).
**Research flag:** No research needed — the `http.Server` goroutine + signal handling + Shutdown pattern is fully specified with code in PITFALLS.md.

### Phase 8: React Frontend
**Rationale:** The frontend is the final consumer and depends on a fully functional API. The React app is scaffolded once with `npm create vite@latest web -- --template react-ts`. The Vite proxy handles CORS during development. The three components — ProcessList, ProcessForm, LogViewer — are built and tested against the real running API. Production build verification (Go binary serves `web/dist/`) is the final integration test.
**Delivers:** `web/` directory with ProcessList (status badges + 2-second auto-refresh), ProcessForm (name, command, args, dependsOn, restartPolicy fields), start/stop buttons per process, LogViewer with 2-second polling + AbortController cleanup on unmount. Production build: `./bin/rtx serve` serves React and API at same origin without Vite.
**Implements:** All React frontend MVP features from FEATURES.md.
**Avoids:** Pitfall 11 (React polling without `useEffect` cleanup — memory leak and state updates on unmounted component).
**Research flag:** React cleanup pattern and AbortController usage are fully specified with code in PITFALLS.md. No additional research needed.

### Phase Ordering Rationale

- **Cleanup is a hard prerequisite:** The broken build blocks compilation of any new package. Nothing can proceed while `go build ./...` fails.
- **Data structures before logic:** Types and the log buffer must exist before any lifecycle code is written; retrofitting the mutex after the fact creates racing bugs that are harder to isolate than if the mutex is designed in from the start.
- **Basic lifecycle before advanced features:** Start/stop without dependencies or restart policies provides a testable race-free baseline; dependency ordering and restart policies are layered on top without requiring refactoring.
- **Complete scheduler before API:** The API is a thin adapter — building it against an incomplete scheduler requires mocking that obscures real integration issues.
- **API before frontend:** The React app calls real endpoints; testing against the actual API rather than a stub catches contract mismatches before they become UX bugs.
- **Vite proxy is dev-only:** Production verification must confirm that the Go binary serves React static files correctly without Vite running — this is a required final check in Phase 8.

### Research Flags

All 8 phases have sufficient coverage in the research files to proceed directly to implementation planning. Every critical pattern has working code examples in ARCHITECTURE.md, PITFALLS.md, or STACK.md.

Phases with well-documented patterns — skip `/gsd:research-phase`:
- **Phase 1 (Cleanup):** Mechanical deletion of identified file paths.
- **Phase 2 (Data Structures):** Ring buffer design and logBuffer mutex — code examples in PITFALLS.md.
- **Phase 3 (Start/Stop):** Non-blocking exec.Cmd + doneCh goroutine — code examples in ARCHITECTURE.md and PITFALLS.md.
- **Phase 4 (Dependency Ordering):** DFS topological sort with visited/inStack — code examples in PITFALLS.md.
- **Phase 5 (Restart Policies):** Context cancellation + backoff cap — code examples in PITFALLS.md.
- **Phase 6 (REST API):** Go 1.22+ net/http, CORS middleware — code examples in STACK.md and ARCHITECTURE.md.
- **Phase 7 (serve subcommand):** http.Server goroutine + signal handling + Shutdown — code examples in PITFALLS.md.
- **Phase 8 (React Frontend):** React 19 + useEffect cleanup + AbortController — code examples in PITFALLS.md and STACK.md.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All Go recommendations verified against official stdlib docs (pkg.go.dev) and Go 1.25 release notes. React/Vite/TypeScript versions verified against official release pages. No technology decision rests solely on community sources. |
| Features | HIGH | Grounded in direct codebase analysis of `runner.go` plus competitor analysis of supervisord, PM2, and overmind. Table stakes reflect what reference tools actually implement. Differentiators (dependency ordering, REST API, backoff) are verified absent from all three reference tools. |
| Architecture | HIGH | Based on existing codebase analysis plus official Go stdlib docs. Component boundaries are explicit, verified against the no-circular-dependency constraint, and illustrated with a full system diagram. Code examples synthesized from real Go stdlib patterns. |
| Pitfalls | HIGH | Every pitfall maps to a specific build phase, has a prevention code example, has warning signs for detection, and has a recovery cost estimate. Critical pitfalls (logBuffer race, lock-during-Start, restart goroutine leak) are derived from Go stdlib behavior documented in official sources and golang/go issue tracker. |

**Overall confidence:** HIGH

### Gaps to Address

- **Production static file serving:** The research specifies that the Go binary serves `web/dist/` in production but does not specify whether to use `//go:embed` (embeds files into the binary) or `http.FileServer` (serves from disk). Decision needed during Phase 7/8 planning. Recommendation: use `http.FileServer(http.Dir("web/dist"))` for simplicity in v1.1; embed is better for distribution as a single self-contained binary.

- **Process `Remove()` method guard:** PITFALLS.md notes that deleting a running process should return `409 Conflict`, and ARCHITECTURE.md lists `DELETE /api/processes/{id}` in the endpoint contract. However, neither document specifies a `Remove()` method signature on the scheduler. This method needs to be designed (checking state before removal) and must be included in Phase 3 or Phase 6 implementation planning.

- **Restart policy form UX:** The `ProcessForm` React component must include fields for `restartPolicy.mode` (dropdown: never/always/on-failure), `restartPolicy.initialWait` (duration input), `restartPolicy.maxWait` (duration input), and `restartPolicy.maxRestarts` (number input). The UX for duration fields is not specified in the research — implementer must decide on input format (seconds as integer, or string like "5s") and ensure it matches the JSON API body format accepted by `POST /api/processes`.

---

## Sources

### Primary (HIGH confidence)
- `https://pkg.go.dev/net/http` — ServeMux routing, PathValue, middleware, ErrServerClosed
- `https://go.dev/blog/routing-enhancements` — Go 1.22 method+path pattern confirmation
- `https://go.dev/doc/go1.25` — json/v2 experimental status, compatibility warning
- `https://go.dev/blog/jsonv2-exp` — json/v2 explicitly not under Go 1 compatibility promise
- `https://pkg.go.dev/net/http/httptest` — NewRecorder, NewRequest for handler testing
- `https://pkg.go.dev/os/exec` — Cmd.Start, Cmd.Wait, internal copy goroutines
- `https://pkg.go.dev/sync` — RWMutex, Mutex semantics
- `https://pkg.go.dev/github.com/google/uuid` — uuid.NewString() for UUID v4
- `https://vite.dev/releases` — Vite 7.3.1 current stable confirmed
- `https://react.dev/versions` — React 19.2.4 current stable confirmed
- `https://devblogs.microsoft.com/typescript/` — TypeScript 5.9 stable, 6.0 in beta
- `https://go.dev/doc/articles/race_detector` — race detection methodology
- `https://github.com/golang/go/issues/19804` — concurrent copy goroutines from cmd.Start(), data race with shared writer
- `https://react.dev/reference/react/useEffect` — official cleanup pattern for intervals
- Runtime-X `go.mod` — uuid v1.6.0 already present, no new Go dependencies needed
- Runtime-X `internal/process/runner.go` — v1.0 patterns being reused and adapted in scheduler

### Secondary (MEDIUM confidence)
- `https://supervisord.org/introduction.html` — table stakes definition for process supervisors
- `https://pm2.keymetrics.io/docs/usage/process-management/` — PM2 feature set comparison
- `https://github.com/DarthSim/overmind` — Procfile-based manager; no REST API, no dependency ordering
- `https://michael.stapelberg.ch/posts/2024-01-17-systemd-indefinite-service-restarts/` — restart policy parameter design
- `https://betterstack.com/community/guides/monitoring/exponential-backoff/` — initialDelay, factor, maxRetries, maxDelay
- `https://www.alexedwards.net/blog/which-go-router-should-i-use` — net/http 1.22+ as recommended starting point for flat APIs
- `https://dev.to/mokiat/proper-http-shutdown-in-go-3fji` — ListenAndServe goroutine + Shutdown pattern
- `https://refine.dev/blog/useeffect-cleanup/` — React polling cleanup pattern with AbortController
- `https://github.com/cenkalti/backoff` — backoff cap and overflow prevention patterns
- `https://corsfix.com/blog/common-cors-mistakes` — OPTIONS preflight requirements, wildcard+credentials risks
- `https://www.dbi-services.com/blog/avoid-cors-requests-in-development-mode-with-vite/` — Vite proxy dev-only limitation

---
*Research completed: 2026-03-01*
*Ready for roadmap: yes*
