---
phase: 06-scheduler-start-stop-and-lifecycle
plan: 02
subsystem: scheduler
tags: [syscall, SIGTERM, SIGKILL, process-group, goroutines, process-lifecycle, FSM, race-detector, doneCh]

# Dependency graph
requires:
  - phase: 06-scheduler-start-stop-and-lifecycle (plan 01)
    provides: Start(), captureOutput(), monitorProcess(), mp.cmd, mp.doneCh pattern, Setpgid process group isolation
provides:
  - Stop() method with SIGTERM-first, SIGKILL-escalation, process group signaling
  - doneCh handshake between Stop() and monitorProcess() for race-free exit coordination
  - ErrNotRunning coverage for Stopped, Idle, Failed states
  - 8-test TDD suite covering normal stop, error cases, SIGKILL escalation, concurrent safety, output capture
  - Full process lifecycle: register -> start -> running -> stop -> stopped
affects:
  - 07 (dependency ordering can now sequence starts and stops)
  - 08 (restart policy depends on StateFailed/StateStopped terminal states being reached correctly via Stop)
  - 09 (REST API Stop endpoint depends on s.Stop())

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Lock-validate-transition-createDoneCh-captureLocals-unlock-signal-wait pattern for Stop()
    - doneCh handshake: Stop() creates while holding lock; monitorProcess closes after cmd.Wait() under lock
    - Process group SIGTERM+SIGKILL escalation via syscall.Kill(-pid, sig) with negative PID
    - select on doneCh vs time.After(timeout) for non-blocking SIGKILL escalation
    - SIGKILL fallback always blocks on doneCh (SIGKILL is unblockable — process always exits)

key-files:
  created: []
  modified:
    - internal/scheduler/lifecycle.go (Stop() implementation added — SIGTERM/SIGKILL escalation)
    - internal/scheduler/lifecycle_test.go (8 Stop() tests + getStoppedAt/getStartedAt helpers)

key-decisions:
  - "Stop() creates doneCh while holding the write lock before releasing it: ensures monitorProcess always finds doneCh != nil when it acquires the lock after cmd.Wait(). This eliminates the race window where Stop() releases the lock, monitor runs and closes doneCh, then Stop() tries to create it (would create unreachable channel)."
  - "Default StopTimeout of 5 seconds applied when ProcessDef.StopTimeout <= 0: balances responsiveness with grace period for well-behaved processes."
  - "SIGKILL escalation blocks unconditionally on doneCh: SIGKILL cannot be caught or ignored by the OS, so the second wait is safe and will always complete."

patterns-established:
  - "Stop() lock protocol: acquire write lock -> validate state -> transition to Stopping -> create doneCh (assigned to mp.doneCh) -> capture pid+timeout locals -> unlock -> syscall.Kill(-pid, SIGTERM) -> select doneCh/timeout"
  - "doneCh creation must precede signal delivery: the lock guarantees monitorProcess cannot close doneCh before Stop() assigns it, preventing a nil-channel deadlock"
  - "Process group SIGTERM then SIGKILL: Stop() always targets the PGID (set by Setpgid in Start()) ensuring child processes spawned by shell scripts are also terminated"

requirements-completed: [SCH-03]

# Metrics
duration: 2min
completed: 2026-03-02
---

# Phase 6 Plan 02: Stop() with SIGTERM/SIGKILL Escalation Summary

**Stop() sends SIGTERM to the process group, waits for exit via doneCh handshake with monitorProcess, escalates to SIGKILL after StopTimeout — completing the full start/stop process lifecycle**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-02T02:20:09Z
- **Completed:** 2026-03-02T02:21:59Z
- **Tasks:** 2 (RED + GREEN TDD)
- **Files modified:** 2

## Accomplishments

- Stop() sends SIGTERM to the entire process group (negative PID) via Setpgid established in Plan 01, killing all child processes and preventing orphans
- doneCh handshake: Stop() creates the channel while holding the write lock before releasing it; monitorProcess closes it after cmd.Wait() — race-free coordination with zero shared mutable state after unlock
- SIGKILL escalation fires after StopTimeout (default 5s); second block on doneCh is unconditional since SIGKILL cannot be ignored
- All 8 Stop() tests pass; full 17-test suite (Plan 01 + Plan 02) passes under go test -race with zero data races
- SCH-03 requirement satisfied; combined with Plan 01, full lifecycle (register -> start -> running -> stop -> stopped) works correctly

## Task Commits

1. **Task 1: RED - Add Stop() stub and write failing tests** - `8ac90d3` (test)
2. **Task 2: GREEN - Implement Stop() with SIGTERM/SIGKILL escalation** - `b5e05f8` (feat)

## Files Created/Modified

- `internal/scheduler/lifecycle.go` - Stop() implementation with doneCh handshake and SIGTERM/SIGKILL escalation
- `internal/scheduler/lifecycle_test.go` - 8 Stop() TDD tests + getStoppedAt/getStartedAt race-safe helpers

## Decisions Made

- doneCh created while holding the write lock before releasing it: ensures monitorProcess always sees doneCh != nil when it acquires the lock post-cmd.Wait(). Without this, a race window exists where monitor runs between unlock and channel creation, closes a nil doneCh (nil close panics), or Stop() waits on a channel that will never be closed.
- Default StopTimeout of 5 seconds when ProcessDef.StopTimeout <= 0: consistent with common process manager defaults, short enough for tests with 500ms override.
- SIGKILL fallback blocks unconditionally: unlike the SIGTERM select, there is no second timer because SIGKILL is unblockable — the process will always exit, so a timeout would only add latency without benefit.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 7 (dependency ordering) is unblocked: Start() and Stop() both work correctly; dependency graph can safely sequence start/stop calls
- Phase 8 (restart policy) is unblocked: StateFailed and StateStopped terminal states are reached correctly via both natural exit and explicit Stop()
- Phase 9 (REST API) is unblocked: s.Start(name) and s.Stop(name) are the two primary mutation endpoints; both are fully implemented and tested

---
*Phase: 06-scheduler-start-stop-and-lifecycle*
*Completed: 2026-03-02*
