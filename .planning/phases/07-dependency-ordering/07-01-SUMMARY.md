---
phase: 07-dependency-ordering
plan: 01
subsystem: scheduler
tags: [go, dag, kahn, topological-sort, cycle-detection]

# Dependency graph
requires:
  - phase: 06-scheduler-start-stop-and-lifecycle
    provides: "Scheduler with Register(), Start(), Stop(), ManagedProcess, DependsOn []string already on ProcessDef"
provides:
  - "ErrDependencyCycle and ErrDependencyNotFound sentinel errors in deps.go"
  - "topoCheck() — Kahn's BFS cycle detection and missing-dependency validation at Register() time"
  - "topoOrder() — layer-based topological sort returning [][]string for ordered startup"
  - "StartAll() — public method that starts all processes in topological layer order"
  - "Register() now validates DependsOn edges (self-deps and missing deps rejected)"
affects: [08-restart-policy, 09-http-api, 10-cli]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Kahn's BFS for DAG cycle detection: processed-count check catches all cycle types including self-loops"
    - "topoCheck called inside Register() write lock — no separate locking needed"
    - "topoOrder operates on local in-degree and dependents maps only — never mutates s.processes"
    - "StartAll() takes a snapshot under RLock before releasing for Start() calls to prevent lock inversion"

key-files:
  created:
    - internal/scheduler/deps.go
    - internal/scheduler/deps_test.go
  modified:
    - internal/scheduler/scheduler.go
    - internal/scheduler/types.go

key-decisions:
  - "[07-01 impl]: topoCheck does eager missing-name validation before Kahn's BFS — clearer error messages and avoids ghost nodes in graph"
  - "[07-01 impl]: waitRunning checks for terminal states (Failed/Stopped) to fail fast instead of waiting out the full timeout"
  - "[07-01 impl]: StartAll() snapshots s.processes under RLock before calling Start() — prevents holding RLock during blocking Start() operations"

patterns-established:
  - "Pattern: Kahn's in-degree BFS for cycle detection — used in topoCheck and topoOrder"
  - "Pattern: Layer-based topological ordering with sort.Strings per layer for determinism"

requirements-completed: [DEP-01, DEP-03]

# Metrics
duration: 2min
completed: 2026-03-02
---

# Phase 7 Plan 01: Dependency Ordering Summary

**Kahn's BFS cycle detection at Register() time and layer-based topological ordering in deps.go, wired into Register() and StartAll()**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-02T00:02:33Z
- **Completed:** 2026-03-02T00:04:20Z
- **Tasks:** 2 (RED + GREEN TDD)
- **Files modified:** 4

## Accomplishments
- Created `deps.go` with `ErrDependencyCycle`, `ErrDependencyNotFound`, `topoCheck()`, `topoOrder()`, and `StartAll()`
- Integrated `topoCheck()` into `Register()` — self-deps and missing deps rejected at registration time
- 13 new tests in `deps_test.go` covering all cycle, missing-dep, chain, diamond, and Register() integration scenarios
- All 30 scheduler tests pass with `-race` flag, zero data races

## Task Commits

Each task was committed atomically:

1. **Task 1: RED — Write failing tests for topoCheck and topoOrder** - `4ece643` (test)
2. **Task 2: GREEN — Implement topoCheck, topoOrder, and wire Register()** - `7e46a05` (feat)

## Files Created/Modified
- `internal/scheduler/deps.go` — ErrDependencyCycle, ErrDependencyNotFound, topoCheck(), topoOrder(), StartAll(), waitRunning()
- `internal/scheduler/deps_test.go` — 13 tests: 5 topoCheck, 5 topoOrder, 3 Register integration
- `internal/scheduler/scheduler.go` — Register() gains topoCheck() call after duplicate check
- `internal/scheduler/types.go` — DependsOn comment updated (removed "ignored until Phase 7")

## Decisions Made
- **Eager missing-name check before Kahn's BFS:** topoCheck validates all DependsOn names exist before building the graph. Provides clearer `ErrDependencyNotFound` messages vs a generic cycle error from ghost nodes in Kahn's count.
- **waitRunning() fast-fails on terminal states:** Instead of waiting out the full 10-second timeout when a dependency crashes on start, it immediately returns an error when it sees Failed or Stopped state.
- **StartAll() snapshot pattern:** Takes a copy of `s.processes` under RLock before releasing, then calls `Start()` without holding any lock. This prevents lock inversion between the scheduler RLock and Start()'s write lock.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `topoOrder()` is ready for Plan 02's `StartAll()` integration tests with real processes
- `ErrDependencyCycle` and `ErrDependencyNotFound` are ready for Phase 9 HTTP handler error responses
- Phase 8 restart policy can reference DependsOn ordering without changes to deps.go

---
*Phase: 07-dependency-ordering*
*Completed: 2026-03-02*
