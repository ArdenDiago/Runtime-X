---
phase: 08-restart-policies
plan: "01"
subsystem: scheduler
tags: [go, fsm, state-machine, restart-policy, backoff]

# Dependency graph
requires:
  - phase: 07-dependency-ordering
    provides: ManagedProcess struct, validTransitions FSM, State enum in types.go
provides:
  - StateRestarting lifecycle state with FSM edges
  - RestartPolicy.MaxDelay and BackoffFactor fields for exponential backoff
  - restartCancelCh channel on ManagedProcess for Stop() interrupt of pending restart
  - FSM transitions Running->Restarting, Restarting->{Starting,Stopping,Failed}
affects: [08-02-restart-loop-implementation, 09-REST-API, 11-react-frontend]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "restartCancelCh (chan struct{}) closed by Stop() to interrupt backoff sleep — same pattern as doneCh for process exit"
    - "StateRestarting sits between StateRunning exit and StateStarting retry — models the backoff delay as a first-class lifecycle state"

key-files:
  created: []
  modified:
    - internal/scheduler/types.go
    - internal/scheduler/scheduler_test.go

key-decisions:
  - "[08-01 arch]: StateRestarting placed after StateFailed in iota so existing numeric values of Idle/Starting/Running/Stopping/Stopped/Failed are unchanged — safe for any serialization that used previous values"
  - "[08-01 arch]: BackoffFactor zero-value defaults to 2.0 at runtime (enforced by restart loop in 08-02), not at struct level — keeps zero-value RestartPolicy meaningful (never restart)"
  - "[08-01 arch]: MaxDelay zero-value means no cap — avoids sentinel value, consistent with Go zero-value idiom"
  - "[08-01 arch]: restartCancelCh added to ManagedProcess alongside doneCh — closed by Stop() to signal pending backoff goroutine to abort, same close-to-cancel pattern already used for doneCh"

patterns-established:
  - "close(ch) as cancellation: both doneCh and restartCancelCh use channel close (not send) for broadcast cancellation across goroutines"

requirements-completed: []

# Metrics
duration: 8min
completed: 2026-03-06
---

# Phase 08 Plan 01: Types and FSM Updates Summary

**StateRestarting lifecycle state added to FSM with backoff fields (MaxDelay, BackoffFactor) and restartCancelCh cancel channel on ManagedProcess**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-06T08:55:25Z
- **Completed:** 2026-03-06T09:03:00Z
- **Tasks:** 4
- **Files modified:** 2

## Accomplishments
- Added `MaxDelay time.Duration` and `BackoffFactor float64` to `RestartPolicy` for exponential backoff support in Phase 8 restart loop
- Added `StateRestarting` to the `State` iota enum with `String()` returning `"restarting"`
- Extended `validTransitions`: `StateRunning` now allows `StateRestarting`; `StateRestarting` allows `StateStarting` (backoff elapsed), `StateStopping` (Stop() interrupt), and `StateFailed` (MaxRetries exhausted)
- Added `restartCancelCh chan struct{}` to `ManagedProcess` for Stop() to interrupt pending backoff sleep
- Updated `TestStateTransitions` with 3 new valid and 3 new invalid `StateRestarting` cases; updated `TestStateString` to include `"restarting"` — all 37 tests pass with `-race`

## Task Commits

Each task was committed atomically:

1. **Tasks 1-4: RestartPolicy fields, StateRestarting enum, FSM transitions, restartCancelCh** - `c7acffe` (feat)

## Files Created/Modified
- `/home/ardendiago/Coding/go_tutorials/Runtime-X/internal/scheduler/types.go` - Added MaxDelay/BackoffFactor to RestartPolicy, StateRestarting to State enum with String(), extended validTransitions, added restartCancelCh to ManagedProcess
- `/home/ardendiago/Coding/go_tutorials/Runtime-X/internal/scheduler/scheduler_test.go` - Added StateRestarting valid/invalid FSM test cases and String() coverage

## Decisions Made
- `StateRestarting` placed after `StateFailed` in iota so existing state numeric values are unchanged
- `BackoffFactor` zero-value defaults to 2.0 at runtime (enforced by restart loop in 08-02), not at the struct definition level — keeps zero-value `RestartPolicy{}` meaningful as "never restart"
- `MaxDelay` zero-value means no cap — consistent with Go zero-value idiom rather than using a sentinel
- `restartCancelCh` follows the same close-to-cancel pattern as `doneCh` already established in Phase 6

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- FSM is fully prepared for the Phase 8-02 restart loop implementation
- `restartCancelCh` channel and `StateRestarting` transitions give the restart loop everything it needs to implement safe backoff with Stop() cancellation
- All tests pass; compile is clean

---
*Phase: 08-restart-policies*
*Completed: 2026-03-06*
