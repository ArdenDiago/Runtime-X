---
phase: 05-scheduler-data-structures-and-log-buffer
plan: 02
subsystem: scheduler
tags: [go, sync, fsm, tdd, process-management, mutex, state-machine]

# Dependency graph
requires:
  - phase: 05-01-scheduler-data-structures-and-log-buffer
    provides: "logBuffer ring buffer with Write/Lines/Len; LogEntry/Stream types"
provides:
  - "ProcessDef struct (Name, Command, Args, Env, WorkDir, RestartPolicy, DependsOn, LogBufferSize)"
  - "ManagedProcess struct with State FSM fields and embedded *logBuffer"
  - "6-state FSM (Idle/Starting/Running/Stopping/Stopped/Failed) with validated transitions"
  - "Scheduler struct with New/Register/Remove/Get/List/Logs methods (mutex-safe)"
  - "ErrNotFound, ErrAlreadyExists, ErrNotStopped sentinel errors"
  - "ErrInvalidTransition sentinel error with from/to context in messages"
affects:
  - "06-scheduler-process-lifecycle"
  - "07-dependency-ordering"
  - "08-restart-policies"
  - "09-http-api"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "FSM transition table: package-level validTransitions map[State][]State"
    - "canTransition(from, to State) bool — O(n) slice scan over allowed targets"
    - "transition(mp *ManagedProcess, to State) error — unexported, mutates under caller's write lock"
    - "validateName() using compiled regexp.MustCompile at package init"
    - "Scheduler.Logs() releases RLock before calling mp.logs.Lines() — avoids holding two locks simultaneously"
    - "TDD: RED (stubs + failing tests) then GREEN (full implementation) then REFACTOR (godoc)"

key-files:
  created:
    - "internal/scheduler/types.go"
    - "internal/scheduler/scheduler.go"
    - "internal/scheduler/scheduler_test.go"
  modified: []

key-decisions:
  - "transition() is unexported — Phase 6 calls it from Scheduler methods that hold the write lock; tests call it directly since they are in the same package"
  - "Remove() allows Idle, Stopped, and Failed states — not just Stopped — because an Idle process was never started and can always be unregistered cleanly"
  - "Scheduler.Logs() releases the RLock before calling mp.logs.Lines() — log buffer has its own mutex and holding both locks simultaneously would create ordering hazards with Phase 6 writer goroutines"
  - "validateName uses regexp ^[a-z0-9][a-z0-9-]*$ — first char cannot be hyphen, preventing URL path confusion"

patterns-established:
  - "FSM pattern: validTransitions map + canTransition helper + transition() mutator — Phase 6 uses same pattern for lifecycle"
  - "Sentinel errors wrapped with context: fmt.Errorf(\"%w: %s\", ErrNotFound, name) — errors.Is() still works, message includes name"
  - "Lock ordering discipline: scheduler RWMutex always acquired/released before accessing logBuffer mutex"
  - "All Scheduler methods use pointer receivers to avoid copying sync.RWMutex"

requirements-completed: [SCH-01, SCH-05, SCH-06]

# Metrics
duration: ~5min
completed: 2026-03-01
---

# Phase 5 Plan 02: Scheduler Types and Registration Summary

**ProcessDef/ManagedProcess/State FSM types with Register/Remove/Get/List/Logs Scheduler methods, TDD-verified with race-detector passing**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-01T19:59:46Z
- **Completed:** 2026-03-01T21:22:35Z
- **Tasks:** 3 (RED, GREEN, REFACTOR)
- **Files modified:** 3

## Accomplishments
- Implemented all scheduler core types: ProcessDef, RestartPolicy, RestartMode, ManagedProcess, State with 6-state FSM and validated transition table
- Implemented Scheduler struct with New/Register/Remove/Get/List/Logs methods, all mutex-safe for concurrent use
- 8 test functions covering 30+ subtests: name validation (7 cases), duplicate registration, default LogBufferSize, removal (5 subtests), list, logs, FSM transitions (16 edge cases), State.String(), concurrent access race detection

## Task Commits

Each task was committed atomically:

1. **RED — Failing tests + stubs** - `3685f94` (test)
2. **GREEN + REFACTOR — Full implementation with godoc** - `58640fd` (feat)

## Files Created/Modified
- `internal/scheduler/types.go` — RestartMode/RestartPolicy/ProcessDef types, State FSM with iota constants, String() method, validTransitions map, canTransition/transition helpers, ManagedProcess struct
- `internal/scheduler/scheduler.go` — validName regexp, ErrNotFound/ErrAlreadyExists/ErrNotStopped sentinels, Scheduler struct, New/Register/Remove/Get/List/Logs methods, validateName helper
- `internal/scheduler/scheduler_test.go` — TestSchedulerRegister (7 subtests), TestSchedulerRegisterDuplicate, TestSchedulerRegisterDefaultLogBufferSize, TestSchedulerRemove (5 subtests), TestSchedulerList (2 subtests), TestSchedulerLogs (2 subtests), TestStateTransitions (valid + invalid), TestStateString (6 subtests), TestSchedulerConcurrentAccess

## Decisions Made
- `transition()` is unexported: Phase 6 calls it from Scheduler methods that already hold the write lock; tests in the same package can call it directly without going through public API
- `Remove()` permits Idle, Stopped, and Failed states (not just Stopped): an Idle process was never started so removing it is always clean; Failed processes may need removal before retry
- `Scheduler.Logs()` releases the `RLock` before calling `mp.logs.Lines()`: the log buffer has its own independent mutex, so acquiring both simultaneously would create lock-ordering hazards with Phase 6 goroutines that write logs under the logBuffer mutex while the scheduler may hold its write lock
- Name regexp `^[a-z0-9][a-z0-9-]*$` ensures first character cannot be a hyphen, which would break URL path segments in Phase 9 HTTP handlers

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `Scheduler`, `New()`, `Register()`, `Remove()`, `Get()`, `List()`, `Logs()` are ready for Phase 6 to add Start/Stop lifecycle methods
- `ManagedProcess.logs` field is allocated on Register — Phase 6 goroutines can call `mp.logs.Write()` immediately after `cmd.Start()`
- `transition()` helper ready for Phase 6 to drive state changes (Idle→Starting, Starting→Running, Running→Stopping, etc.)
- `DependsOn []string` field stored on ProcessDef and ready for Phase 7 dependency ordering
- `RestartPolicy` struct stored and ready for Phase 8 restart policy enforcement
- All tests pass: `go test -race ./internal/scheduler/...`, `go vet ./internal/scheduler/...`, `go build ./...` all exit 0

## Self-Check: PASSED

- FOUND: internal/scheduler/types.go
- FOUND: internal/scheduler/scheduler.go
- FOUND: internal/scheduler/scheduler_test.go
- FOUND: 3685f94 (RED test commit)
- FOUND: 58640fd (GREEN + REFACTOR implementation commit)

---
*Phase: 05-scheduler-data-structures-and-log-buffer*
*Completed: 2026-03-01*
