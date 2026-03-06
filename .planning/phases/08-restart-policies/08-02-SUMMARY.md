---
phase: 08-restart-policies
plan: "02"
subsystem: scheduler
tags: [go, restart, backoff, exponential-backoff, goroutines, channels, fsm]

# Dependency graph
requires:
  - phase: 08-restart-policies plan 01
    provides: StateRestarting FSM state, restartCancelCh field, BackoffFactor/MaxDelay policy fields
  - phase: 07-dependency-ordering
    provides: Start() with dependency checks, lifecycle.go/monitorProcess foundation
provides:
  - Automatic process restart via waitAndRestart goroutine with exponential backoff
  - Stop() cancellation of pending restarts via restartCancelCh close
  - RestartAlways, RestartOnFailure, RestartNever modes with MaxRetries budget
affects: [09-REST-API, 10-cli-serve-and-graceful-shutdown, 11-react-frontend]

# Tech tracking
tech-stack:
  added: [math.Pow for exponential backoff calculation]
  patterns:
    - close-to-cancel channel pattern (restartCancelCh mirrors doneCh pattern)
    - monitorProcess as single FSM authority — no other goroutine writes Running->Restarting
    - waitAndRestart goroutine launched under lock, reads policy snapshot after release
    - Start() allowed from StateRestarting to support automatic restart loop

key-files:
  created:
    - internal/scheduler/restart.go
  modified:
    - internal/scheduler/lifecycle.go
    - internal/scheduler/lifecycle_test.go

key-decisions:
  - "[08-02 arch]: Start() must allow StateRestarting as caller state — waitAndRestart goroutine calls s.Start() directly, and FSM has Restarting->Starting as valid edge"
  - "[08-02 arch]: monitorProcess increments RestartCount before launching waitAndRestart — count reflects attempts in flight, not only completed restarts"
  - "[08-02 arch]: Stop() in StateRestarting closes restartCancelCh under the lock then returns immediately — process is already dead, no SIGTERM needed"
  - "[08-02 impl]: calcDelay uses math.Pow for float64 exponentiation — exponent = RestartCount-1 so first delay = Delay * factor^0 = Delay"
  - "[08-02 impl]: waitAndRestart reads policy and cancelCh under RLock snapshot — safe concurrent access, timer created outside lock"

patterns-established:
  - "close-to-cancel: creating a channel, storing it in the struct, then closing it from a different goroutine to signal cancellation"
  - "goroutine-safe restart loop: monitorProcess transitions state and launches goroutine under lock; goroutine reads snapshot then releases lock before sleeping"

requirements-completed: [RST-01, RST-02, RST-03, RST-04]

# Metrics
duration: 18min
completed: 2026-03-06
---

# Phase 8 Plan 02: Restart Logic and Integration Summary

**Exponential-backoff restart engine with per-process cancellation channels — processes restart automatically after exit per policy, Stop() during backoff cancels immediately**

## Performance

- **Duration:** 18 min
- **Started:** 2026-03-06T08:55:49Z
- **Completed:** 2026-03-06T09:13:00Z
- **Tasks:** 4
- **Files modified:** 3

## Accomplishments

- Implemented `waitAndRestart` goroutine with `math.Pow`-based exponential backoff and cancellable timer select
- Updated `monitorProcess` to evaluate restart policy, increment retry counter, and launch restart goroutine
- Updated `Stop()` to handle `StateRestarting` by closing `restartCancelCh` and returning immediately (no SIGTERM needed)
- Updated `Start()` to accept `StateRestarting` as a valid caller state (from `waitAndRestart` goroutine)
- 4 integration tests covering all restart scenarios pass with `-race`

## Task Commits

Each task was committed atomically:

1. **Task 08-01 types/FSM** - `c7acffe` (feat): StateRestarting, BackoffFactor, MaxDelay, restartCancelCh
2. **Task 1: waitAndRestart** - `36b4018` (feat): exponential backoff goroutine with calcDelay helper
3. **Tasks 2-4: monitorProcess/Stop/Start + tests** - `dee4bb9` (feat): restart policy evaluation, cancellation, StateRestarting in Start()

## Files Created/Modified

- `internal/scheduler/restart.go` - `waitAndRestart()` goroutine and `calcDelay()` helper
- `internal/scheduler/lifecycle.go` - Updated `monitorProcess` (policy check, restart branching), `Stop()` (Restarting fast path), `Start()` (StateRestarting allowed)
- `internal/scheduler/lifecycle_test.go` - `TestRestartAlways`, `TestRestartOnFailure`, `TestRestartMaxRetries`, `TestStopDuringRestart`

## Decisions Made

- **Start() from StateRestarting:** The plan said "call `s.Start(mp.Def.Name)`" from `waitAndRestart`. The FSM has `Restarting → Starting` as valid, but the original `Start()` only allowed `{Idle, Stopped, Failed}`. Fixed by adding `StateRestarting` as an allowed source state — this is the goroutine-internal restart path.
- **monitorProcess restartCount ordering:** `RestartCount` is incremented before launching `waitAndRestart` so the count reflects in-flight attempts. `calcDelay` uses `RestartCount - 1` as the exponent so the first delay equals `Delay * factor^0 = Delay` (no inflation on first retry).
- **Stop() in Restarting returns immediately:** When `StateRestarting`, the OS process is already dead. We close `restartCancelCh`, transition `Stopping → Stopped`, and return — no SIGTERM path needed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Start() must allow StateRestarting**
- **Found during:** Task 2 (TestRestartMaxRetries failing — process stuck in Restarting)
- **Issue:** `waitAndRestart` calls `s.Start()` but `Start()` rejected `StateRestarting` with "cannot start process in state restarting"
- **Fix:** Added `case StateRestarting:` to the allowed states switch in `Start()`
- **Files modified:** `internal/scheduler/lifecycle.go`
- **Verification:** `TestRestartMaxRetries` passes (0.08s), all 41 tests pass with `-race`
- **Committed in:** `dee4bb9` (Task 2-4 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — logic bug in Start() state guard)
**Impact on plan:** Essential fix for correctness. The plan's architecture requires Start() to be callable from waitAndRestart goroutine.

## Issues Encountered

None beyond the auto-fixed Start() state guard.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 8 complete: full restart policy engine with exponential backoff and cancellation
- Phase 9 (REST API) ready: scheduler has all required operations (Register, Start, Stop, List, Get, Logs)
- REST API handlers can expose RestartPolicy fields as JSON — BackoffFactor and MaxDelay are straightforward to serialize

---
*Phase: 08-restart-policies*
*Completed: 2026-03-06*
