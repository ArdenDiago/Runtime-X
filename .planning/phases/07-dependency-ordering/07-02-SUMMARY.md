---
phase: 07-dependency-ordering
plan: 02
subsystem: scheduler
tags: [go, dag, topological-sort, dependency-check, integration-test]

# Dependency graph
requires:
  - phase: 07-dependency-ordering-plan-01
    provides: "StartAll(), waitRunning(), topoOrder(), ErrDependencyCycle, ErrDependencyNotFound in deps.go"
  - phase: 06-scheduler-start-stop-and-lifecycle
    provides: "Start(), Stop(), ManagedProcess, StateRunning lifecycle states"
provides:
  - "ErrDependencyNotReady sentinel error in deps.go"
  - "checkDepsRunning() helper that validates all DependsOn are StateRunning before process starts"
  - "Start() dependency-readiness check — rejects start if any dependency is not Running"
  - "7 integration tests with real sleep processes covering chain, diamond, independent, and single-process StartAll scenarios"
  - "2 integration tests for Start() dependency validation (reject + accept paths)"
affects: [08-restart-policy, 09-http-api, 10-cli]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Dependency check injected into Start() before StateStarting transition — uses write lock already held"
    - "checkDepsRunning() reads live process map under caller's write lock — no additional locking"
    - "Integration tests use sleep 30 as long-lived process; killProcess for cleanup uses SIGKILL to process group"

key-files:
  created: []
  modified:
    - internal/scheduler/deps.go
    - internal/scheduler/deps_test.go
    - internal/scheduler/lifecycle.go

key-decisions:
  - "[07-02 impl]: checkDepsRunning() called inside Start() write lock section before transition to StateStarting — no additional locking needed and prevents TOCTOU between check and transition"
  - "[07-02 impl]: Reuse existing killProcess/getState helpers from lifecycle_test.go — no redeclaration; same-package tests share all helpers"

patterns-established:
  - "Pattern: Dependency pre-check before state transition — check inside write lock, return error before mutating state"
  - "Pattern: Integration tests for real process orchestration — register multiple processes, call StartAll, verify state under RLock"

requirements-completed: [DEP-01, DEP-02]

# Metrics
duration: 3min
completed: 2026-03-02
---

# Phase 7 Plan 02: Dependency Ordering Summary

**ErrDependencyNotReady sentinel and Start() dependency-readiness check, validated by 7 integration tests using real sleep processes covering chain, diamond, and independent StartAll scenarios**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-02T03:06:54Z
- **Completed:** 2026-03-02T03:09:54Z
- **Tasks:** 2 (RED + GREEN TDD)
- **Files modified:** 3

## Accomplishments
- Added `ErrDependencyNotReady` sentinel and `checkDepsRunning()` helper to `deps.go`
- Wired dependency check into `Start()` in `lifecycle.go` before `StateStarting` transition — processes with un-running dependencies are rejected with a clear error
- 7 new integration tests with real `sleep 30` processes proving correct layer-by-layer startup ordering
- Diamond dependency test (A, B->A, C->A, D->B,C) passes, confirming A starts exactly once and D last
- All 37 scheduler tests pass with `-race` flag, zero data races

## Task Commits

Each task was committed atomically:

1. **Task 1: RED — Write failing tests for StartAll and Start dependency check** - `3506d33` (test)
2. **Task 2: GREEN — Implement StartAll, waitRunning, and Start dependency check** - `1bf9501` (feat)

## Files Created/Modified
- `internal/scheduler/deps.go` — Added ErrDependencyNotReady, checkDepsRunning()
- `internal/scheduler/deps_test.go` — 7 new integration tests using real processes
- `internal/scheduler/lifecycle.go` — Start() now calls checkDepsRunning() under write lock

## Decisions Made
- **Dependency check inside write lock before StateStarting:** checkDepsRunning() is called after the state validation switch and before transition(mp, StateStarting), all under the same write lock. This prevents TOCTOU between the dependency check and state mutation.
- **Reuse lifecycle_test.go helpers:** killProcess and getState were already defined in lifecycle_test.go with compatible signatures. My initial redeclarations in deps_test.go caused build errors. Removed redeclarations and adapted test code to existing helper signatures.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed redeclared killProcess/getState helpers from deps_test.go**
- **Found during:** Task 1 (RED test writing)
- **Issue:** Plan instructed adding killProcess/getState helpers to deps_test.go, but identical helpers already existed in lifecycle_test.go — both in package scheduler, causing "redeclared in this block" build errors
- **Fix:** Removed duplicate declarations from deps_test.go and adapted all test calls to use existing helper signatures (getState returns (State, bool) not (State, error))
- **Files modified:** internal/scheduler/deps_test.go
- **Verification:** Build succeeds, no redeclaration errors
- **Committed in:** 3506d33 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — removed conflicting helper redeclarations)
**Impact on plan:** Required to fix build; no behavior change, same test coverage.

## Issues Encountered
- Pre-existing flaky tests `TestStart_CapturesOutput` and `TestStart_CapturesStderr` fail intermittently due to a timing race in log capture. This is out of scope for this plan — logged to deferred-items.md.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `ErrDependencyNotReady` is ready for Phase 9 HTTP handler error responses
- `Start()` dependency guard is complete — Phase 8 restart policy can rely on dependency ordering being enforced at start time
- All Phase 5, 6, and 7 tests pass with -race flag

---
*Phase: 07-dependency-ordering*
*Completed: 2026-03-02*
