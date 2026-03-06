---
phase: 10-cli-serve-and-graceful-shutdown
plan: "02"
subsystem: cli
tags: [go, signal-handling, graceful-shutdown, os/signal, http.Server, sync.WaitGroup]

# Dependency graph
requires:
  - phase: 09-REST-API
    provides: scheduler.Scheduler, api.Server, api.NewServer, scheduler.Stop()
  - phase: 10-01
    provides: cmdServe skeleton, serve.go, http.ListenAndServe baseline
provides:
  - scheduler.StopAll() — parallel goroutine shutdown with sync.WaitGroup
  - SIGINT/SIGTERM signal handling in rtx serve
  - http.Server.Shutdown(ctx) graceful HTTP drain
  - 10-second shutdown timeout context
affects: [11-react-frontend, integration tests, manual QA]

# Tech tracking
tech-stack:
  added: [os/signal, syscall (SIGINT/SIGTERM), context.WithTimeout, http.Server.Shutdown]
  patterns:
    - StopAll uses RLock snapshot then goroutines with WaitGroup — prevents holding lock during blocking Stop() calls
    - Signal goroutine pattern: server in goroutine, main goroutine selects on quit and serverErrCh
    - http.ErrServerClosed guard — ListenAndServe returns this on Shutdown(), must not be treated as error

key-files:
  created: []
  modified:
    - internal/scheduler/deps.go
    - internal/scheduler/lifecycle_test.go
    - cmd/rtx/serve.go
    - .gitignore

key-decisions:
  - "StopAll snapshots process names under RLock before goroutine launch — prevents lock inversion since Stop() acquires write lock"
  - "StopAll silently ignores Stop() errors — ErrNotRunning is a valid race (process exits between snapshot and Stop call)"
  - "StopAll includes StateStarting and StateRestarting alongside StateRunning — all three are live states that need cleanup"
  - "gracefulShutdownTimeout = 10s shared by httpServer.Shutdown and StopAll is enforced via context; StopAll itself uses per-process StopTimeout (5s default)"
  - "[Rule 1 - Bug] .gitignore bare 'rtx' matched cmd/rtx/ directory — fixed to '/rtx' to only match root-level binary"

patterns-established:
  - "Signal handler pattern: server goroutine + main selects on quit channel and serverErrCh"
  - "StopAll parallel-shutdown: snapshot names under RLock, then WaitGroup goroutines calling Stop()"

requirements-completed: []

# Metrics
duration: 9min
completed: 2026-03-06
---

# Phase 10 Plan 02: Graceful Shutdown and StopAll() Summary

**scheduler.StopAll() with parallel SIGTERM+SIGKILL per process, plus SIGINT/SIGTERM signal handling in rtx serve using http.Server.Shutdown(ctx) and 10-second drain timeout**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-06T09:41:24Z
- **Completed:** 2026-03-06T09:50:48Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Added `StopAll()` to `internal/scheduler/deps.go`: snapshots Running/Starting/Restarting process names under RLock, stops each in a separate goroutine, waits via `sync.WaitGroup`
- Added `TestStopAll()` and `TestStopAll_AlreadyStopped()` — both pass with `-race`
- Refactored `cmdServe` to use `http.Server` instead of `http.ListenAndServe`, enabling `Shutdown(ctx)` for graceful HTTP drain
- On SIGINT/SIGTERM: `httpServer.Shutdown(ctx)` drains in-flight requests, then `sched.StopAll()` terminates all managed processes in parallel
- Fixed `.gitignore` bug where bare `rtx` matched `cmd/rtx/` source directory instead of just the root binary

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement scheduler.StopAll()** - `aeac275` (feat)
2. **Task 2: Signal handling + graceful shutdown in rtx serve** - `dc84d5e` (feat)

## Files Created/Modified
- `internal/scheduler/deps.go` - Added `StopAll()` method and `sync` import
- `internal/scheduler/lifecycle_test.go` - Added `TestStopAll()` and `TestStopAll_AlreadyStopped()`
- `cmd/rtx/serve.go` - Refactored to `http.Server`, added signal handling, graceful drain, and StopAll call
- `.gitignore` - Fixed `rtx` to `/rtx` to prevent accidentally ignoring `cmd/rtx/` source directory

## Decisions Made
- StopAll snapshots names under RLock before launching goroutines — avoids lock inversion since `Stop()` acquires the write lock internally
- StopAll silently ignores `Stop()` errors — `ErrNotRunning` is a valid race condition when a process exits naturally between snapshot and stop call
- StopAll targets StateRunning, StateStarting, and StateRestarting — all three represent live OS processes that need cleanup
- The 10-second context timeout covers `httpServer.Shutdown` drain; `StopAll` uses each process's own `StopTimeout` (default 5s) for SIGTERM→SIGKILL escalation

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed .gitignore matching cmd/rtx/ source directory**
- **Found during:** Task 2 (staging serve.go)
- **Issue:** `.gitignore` had bare `rtx` which Git interprets as matching any path component named `rtx`, including the `cmd/rtx/` source directory. `git add cmd/rtx/serve.go` was rejected.
- **Fix:** Changed `rtx` to `/rtx` in `.gitignore` — the leading slash anchors the pattern to the repo root, so only the root-level compiled binary is ignored
- **Files modified:** `.gitignore`
- **Verification:** `git add cmd/rtx/serve.go` succeeds after fix
- **Committed in:** `dc84d5e` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** The .gitignore fix was necessary to stage new files in cmd/rtx/. No scope creep.

## Issues Encountered
None beyond the .gitignore auto-fix above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 10 is complete: `rtx serve` starts the REST API + frontend file server and shuts down cleanly on Ctrl+C
- `scheduler.StopAll()` is available for any future integration or Phase 11 frontend polling scenarios
- Phase 11 (React frontend) can proceed — `rtx serve` serves `web/dist/` at root and the API at `/api/`

## Self-Check: PASSED

- FOUND: internal/scheduler/deps.go
- FOUND: internal/scheduler/lifecycle_test.go
- FOUND: cmd/rtx/serve.go
- FOUND: .gitignore
- FOUND commit: aeac275 (Task 1: StopAll)
- FOUND commit: dc84d5e (Task 2: signal handling)

---
*Phase: 10-cli-serve-and-graceful-shutdown*
*Completed: 2026-03-06*
