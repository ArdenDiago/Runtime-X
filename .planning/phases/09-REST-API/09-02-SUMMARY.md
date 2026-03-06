---
phase: 09-REST-API
plan: "02"
subsystem: api
tags: [go, rest-api, http, cors, net/http, httptest, scheduler]

# Dependency graph
requires:
  - phase: 09-REST-API plan 01
    provides: internal/api/server.go with Server struct, Routes(), send() helper, ListProcesses, CreateProcess, CORS middleware
  - phase: 08-restart-policies
    provides: Scheduler.Start(), Stop(), Register(), Remove(), Get(), List(), Logs() — full lifecycle engine
provides:
  - GET /api/processes/{name} — GetProcess handler (200/404)
  - PUT /api/processes/{name} — UpdateProcess handler (200/404/409/400)
  - DELETE /api/processes/{name} — DeleteProcess handler (200/404/409)
  - POST /api/processes/{name}/start — StartProcess handler (202/404/409)
  - POST /api/processes/{name}/stop — StopProcess handler (200/404/409)
  - GET /api/processes/{name}/logs — GetLogs handler with logsEnvelope (200/404)
  - scheduler.ProcessSnapshot value type for race-safe state reads
  - scheduler.Snapshot(name) and SnapshotAll() methods
  - Comprehensive handler tests (21 total, all pass with -race)
affects: [10-cli-serve-and-graceful-shutdown, 11-react-frontend]

# Tech tracking
tech-stack:
  added: [net/http/httptest for handler testing]
  patterns:
    - ProcessSnapshot value type for race-safe reads across goroutine boundaries
    - Snapshot()/SnapshotAll() pattern instead of Get()/List() in HTTP handlers
    - 202 Accepted for async lifecycle operations (Start)
    - 409 Conflict for state violations (already running, not stopped)
    - logsEnvelope struct for structured log responses

key-files:
  created:
    - internal/api/handlers.go — all 8 API handlers (complete)
    - internal/scheduler/scheduler.go — ProcessSnapshot, Snapshot(), SnapshotAll()
  modified:
    - internal/api/handlers_test.go — expanded from 7 to 21 tests covering all handlers + CORS

key-decisions:
  - "ProcessSnapshot value type: handlers use Snapshot()/SnapshotAll() instead of Get()/List() to prevent DATA RACE between monitorProcess goroutine writes and HTTP handler reads of mp.State"
  - "202 Accepted for StartProcess: semantics match async operation — process may still be starting when handler returns"
  - "UpdateProcess uses Remove+Register pattern: cleanest atomic swap within existing scheduler API without needing new UpdateDef method"
  - "logsEnvelope wraps entries with name field: clients know which process logs belong to without parsing URL"

patterns-established:
  - "Snapshot pattern: always use Scheduler.Snapshot()/SnapshotAll() in HTTP handlers, never Get()/List() when live fields (State, ExitCode) are needed post-goroutine-spawn"
  - "doRouteRequest test helper: sends full Routes() handler requests for integration-style tests with path params"
  - "mustRegister test helper: registers a process or fatals, reduces boilerplate in handler tests"

requirements-completed: [API-03, API-04, API-05, API-06, API-07, API-08, API-09]

# Metrics
duration: 10min
completed: 2026-03-06
---

# Phase 09 Plan 02: Lifecycle Handlers & CORS Summary

**Full REST API for process management: 8 handlers (GetProcess, UpdateProcess, DeleteProcess, StartProcess, StopProcess, GetLogs, CORS) with race-safe ProcessSnapshot pattern and 21 passing -race tests**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-06T09:23:55Z
- **Completed:** 2026-03-06T09:33:55Z
- **Tasks:** 3 (resource handlers, lifecycle+logs handlers, CORS — plus race fix)
- **Files modified:** 3

## Accomplishments

- All 6 remaining API endpoints implemented (API-03 through API-08) with correct HTTP semantics (202 for async start, 409 for state violations, 404 for not found)
- CORS middleware (API-09) wraps all routes: Access-Control-Allow-Origin: *, handles OPTIONS preflight with 204
- Discovered and fixed a DATA RACE: `monitorProcess` goroutine writes `mp.State` concurrently with HTTP handlers reading it via `Get()` after `Start()`; resolved by introducing `ProcessSnapshot` value type and `Snapshot()`/`SnapshotAll()` scheduler methods
- 21 handler tests cover all happy paths and error paths; all pass with `-race` and `-v`

## Task Commits

1. **Task 1+2+3: Resource, Lifecycle, Log Handlers + CORS** — already committed in 09-01 session (722a79f, 4d75b2a)
2. **Tests: Comprehensive handler tests for 09-02** — `8711024` (test)
3. **Race fix: ProcessSnapshot + Snapshot/SnapshotAll** — `3016bb9` (fix)

## Files Created/Modified

- `/home/ardendiago/Coding/go_tutorials/Runtime-X/internal/api/handlers.go` — GetProcess, UpdateProcess, DeleteProcess, StartProcess, StopProcess, GetLogs; refactored to use snapshotToJSON(ProcessSnapshot) for race safety
- `/home/ardendiago/Coding/go_tutorials/Runtime-X/internal/scheduler/scheduler.go` — Added ProcessSnapshot struct, Snapshot(), SnapshotAll() methods
- `/home/ardendiago/Coding/go_tutorials/Runtime-X/internal/api/handlers_test.go` — Expanded from 7 to 21 tests; added doRouteRequest, mustRegister helpers; covers all handlers + CORS

## Decisions Made

- **ProcessSnapshot over live pointer**: Handlers must never read `mp.State` or other fields from a `*ManagedProcess` returned by `Get()` after `Start()` has launched goroutines. Using a value-type snapshot taken under the RLock is the correct pattern.
- **202 Accepted for StartProcess**: Start is asynchronous — the process is spawning when the handler returns. 202 is more accurate than 200.
- **UpdateProcess = Remove + Register**: Rather than adding a dedicated `UpdateDef` scheduler method, Remove+Register atomically replaces the definition. Acceptable because the state check (must be stopped) precedes the update.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] DATA RACE: mp.State read without lock after Start()**
- **Found during:** Task verification (running `go test -race ./internal/api/...`)
- **Issue:** `StartProcess` and `StopProcess` handlers called `s.Scheduler.Get(name)` after `Start()`/`Stop()` returned, then passed the live `*ManagedProcess` pointer to `toProcessJSON()`. The `monitorProcess` goroutine concurrently writes `mp.State` and other fields. Race detector flagged a DATA RACE.
- **Fix:** Added `ProcessSnapshot` value type and `Snapshot()`/`SnapshotAll()` scheduler methods that copy all observable fields under the RLock. Replaced all `Get()` → `toProcessJSON(mp)` calls with `Snapshot()` → `snapshotToJSON(snap)` in all handlers. Renamed `toProcessJSON` to `snapshotToJSON` to accept the value type.
- **Files modified:** `internal/scheduler/scheduler.go`, `internal/api/handlers.go`
- **Verification:** `go test -race ./...` — all 62 tests pass, zero DATA RACE warnings
- **Committed in:** `3016bb9`

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug: data race in handler state reads)
**Impact on plan:** Required for correctness. The Snapshot pattern is an architectural improvement that prevents future races as new handlers are added.

## Issues Encountered

- The 09-01 session had already committed `handlers.go` with all 8 handler implementations (GetProcess through GetLogs) and `server.go` with all routes + CORS. This was discovered when `git status` showed no changes to `handlers.go`. The 09-02 work remaining was exclusively the test expansion and the race fix that -race testing revealed.

## Next Phase Readiness

- All 8 REST API endpoints are implemented, tested, and race-free
- `internal/api/server.go` exports `NewServer(s *scheduler.Scheduler) *Server` and `Routes() http.Handler`
- Phase 10 (CLI serve + graceful shutdown) can wire `http.ListenAndServe` to `srv.Routes()`
- No blockers

---
*Phase: 09-REST-API*
*Completed: 2026-03-06*
