---
phase: 06-scheduler-start-stop-and-lifecycle
plan: 01
subsystem: scheduler
tags: [os/exec, syscall, bufio, goroutines, process-lifecycle, FSM, race-detector]

# Dependency graph
requires:
  - phase: 05-scheduler-data-structures-and-log-buffer
    provides: ManagedProcess, ProcessDef, logBuffer, State FSM, Scheduler with Register/Remove/Get/List/Logs
provides:
  - Start() method that spawns real OS processes with PID tracking and state transitions
  - captureOutput() that writes stdout/stderr lines to logBuffer line-by-line via bufio.Scanner
  - monitorProcess() goroutine that owns state transitions on process exit (Stopped/Failed)
  - ErrAlreadyRunning and ErrNotRunning sentinel errors
  - cmd *exec.Cmd and doneCh chan struct{} fields on ManagedProcess for Phase 6 Stop()
  - StopTimeout time.Duration field on ProcessDef for per-process stop configuration
  - FSM correction: Running -> Stopped transition for natural clean exit
  - 9-test TDD suite covering Start(), monitor, and output capture under race detector
affects:
  - 06-02 (Stop() depends on mp.cmd, mp.doneCh, monitorProcess's doneCh close pattern)
  - 07 (dependency ordering depends on Start() working correctly)
  - 08 (restart policy depends on StateFailed and StateStopped terminal states)

# Tech tracking
tech-stack:
  added:
    - os/exec (exec.Command, StdoutPipe, StderrPipe, cmd.Start, cmd.Wait)
    - syscall (SysProcAttr.Setpgid, Kill with negative PID for process group)
    - bufio (Scanner with Scanner.Buffer for 8KB line cap)
    - io (io.ReadCloser for pipe parameters)
  patterns:
    - Lock/validate/transition/capture-def/unlock-before-exec/relock pattern for Start()
    - Single monitor goroutine as sole authority for terminal state transitions
    - Process group kill pattern (Setpgid:true + Kill(-pid, sig)) for zombie prevention
    - Same-package test helpers using s.mu directly to read mp fields race-safely

key-files:
  created:
    - internal/scheduler/lifecycle.go
    - internal/scheduler/lifecycle_test.go
  modified:
    - internal/scheduler/types.go (cmd, doneCh, StopTimeout fields; FSM Running->Stopped)
    - internal/scheduler/scheduler.go (ErrAlreadyRunning, ErrNotRunning)
    - internal/scheduler/scheduler_test.go (added Running->Stopped valid transition test)

key-decisions:
  - "FSM must allow Running->Stopped for natural clean exit: Phase 5 FSM only allowed Running->{Stopping,Failed}. Monitor goroutine correctly transitions Running->Stopped (exit 0) but the FSM silently rejected it, leaving state stuck at Running. Fixed by adding StateStopped to StateRunning valid transitions."
  - "Test helpers must hold scheduler lock when reading mp fields: getState/getExitCode/getPID helpers use s.mu.RLock() directly (same-package privilege) to avoid race detector failures when monitorProcess writes under s.mu.Lock()."
  - "Do not check nolint:errcheck silently — use proper error logging or fix the FSM: the original lifecycle.go suppressed transition errors which hid the FSM bug. Discovered by race detector and goroutine stack analysis."

patterns-established:
  - "Lock-unlock-exec-relock pattern: acquire write lock, validate+transition to Starting, capture def, UNLOCK, exec, relock, set Running+cmd, unlock, launch goroutines"
  - "monitorProcess owns terminal transitions: only goroutine that transitions Running->Stopped or Running->Failed; Stop() uses Stopping->Stopped via doneCh handshake"
  - "captureOutput is fire-and-forget: scanner reads until EOF (pipe close on process exit), writes to logBuffer with independent mutex"
  - "Same-package test lock access: tests in package scheduler use s.mu.RLock() directly for race-safe field reads on live ManagedProcess"

requirements-completed: [SCH-02, SCH-04]

# Metrics
duration: 9min
completed: 2026-03-02
---

# Phase 6 Plan 01: Start(), Monitor Goroutine, and Output Capture Summary

**Start() spawns real OS processes via exec.Command with Setpgid process group isolation, captureOutput() feeds stdout/stderr to logBuffer via bufio.Scanner, and monitorProcess() goroutine is the sole authority for Stopped/Failed state transitions**

## Performance

- **Duration:** ~9 min
- **Started:** 2026-03-02T02:07:49Z
- **Completed:** 2026-03-02T02:16:58Z
- **Tasks:** 2 (RED + GREEN TDD)
- **Files modified:** 5

## Accomplishments

- Start() spawns real OS processes with PID tracking, state transitions (Idle/Stopped/Failed -> Starting -> Running), and process group isolation via Setpgid
- captureOutput() goroutines capture stdout/stderr line-by-line via bufio.Scanner with 8KB line cap, writing to the independent logBuffer mutex
- monitorProcess() goroutine calls cmd.Wait() and is the single source of truth for terminal state transitions to Stopped (exit 0) or Failed (non-zero)
- 9 TDD tests all pass under go test -race with zero data races
- SCH-02 (start + PID tracking + status) and SCH-04 (List() reflects live state) requirements satisfied

## Task Commits

1. **Task 1: RED - Add type extensions and write failing tests** - `d2a6a64` (test)
2. **Task 2: GREEN - Implement Start(), captureOutput(), monitorProcess()** - `5acb2d9` (feat)

## Files Created/Modified

- `internal/scheduler/lifecycle.go` - Start(), captureOutput(), monitorProcess() implementations
- `internal/scheduler/lifecycle_test.go` - 9 TDD tests with race-safe helpers
- `internal/scheduler/types.go` - cmd, doneCh, StopTimeout fields; FSM Running->Stopped transition
- `internal/scheduler/scheduler.go` - ErrAlreadyRunning, ErrNotRunning sentinel errors
- `internal/scheduler/scheduler_test.go` - Added Running->Stopped as valid FSM transition

## Decisions Made

- FSM Running->Stopped correction: Phase 5 FSM only permitted Running->{Stopping,Failed}. Natural clean exit (code 0) requires Running->Stopped directly, bypassing the Stopping state. Fixed by adding StateStopped to StateRunning valid transitions table.
- Test helper lock access: Since tests are in package scheduler (same package), getState/getExitCode/getPID helpers access s.mu directly for race-safe reads on live ManagedProcess fields.
- Leave mp.cmd set after exit: post-mortem inspection of cmd.ProcessState is valuable; monitorProcess clears mp.doneCh but not mp.cmd.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] FSM missing Running->Stopped transition for natural clean exit**
- **Found during:** Task 2 (GREEN - implementing monitorProcess)
- **Issue:** Phase 5 FSM only listed StateRunning: {StateStopping, StateFailed}. When a process exits cleanly (code 0), monitorProcess correctly calls transition(mp, StateStopped) but the FSM rejected it silently (nolint:errcheck), leaving State stuck at StateRunning permanently.
- **Diagnosis:** Race detector showed monitorProcess goroutine ran and completed (goroutine count returned to baseline), but State remained Running. Goroutine stack dump confirmed the transition call was executing but failing silently.
- **Fix:** Added StateStopped to StateRunning's valid transitions: StateRunning: {StateStopping, StateStopped, StateFailed}. Updated scheduler_test.go to cover the new valid Running->Stopped edge.
- **Files modified:** internal/scheduler/types.go, internal/scheduler/scheduler_test.go
- **Verification:** TestMonitor_CleanExitToStopped and all 9 lifecycle tests pass under -race
- **Committed in:** 5acb2d9 (Task 2 commit)

**2. [Rule 1 - Bug] Test helpers reading mp fields without lock caused data races**
- **Found during:** Task 2 (GREEN - running tests under -race)
- **Issue:** waitForState() and other test helpers called s.Get() (which releases RLock before returning) and then read mp.State from the returned pointer without any lock. monitorProcess writes mp.State under s.mu.Lock(). Race detector flagged this as a concurrent write/read.
- **Fix:** Rewrote test helpers to use lock-safe equivalents (getState, getExitCode, getPID) that access s.mu.RLock() directly (same-package privilege). All test field reads are now protected.
- **Files modified:** internal/scheduler/lifecycle_test.go
- **Verification:** All 9 tests pass under go test -race with zero data race warnings
- **Committed in:** 5acb2d9 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 Rule 1 bugs)
**Impact on plan:** Both fixes essential for correctness. The FSM fix was a latent design gap from Phase 5 that only became visible when Phase 6 tested natural process exit. The lock fix was a test-only correctness issue. No scope creep.

## Issues Encountered

- The FSM bug was subtle: monitorProcess goroutines ran and exited quickly (visible via runtime.NumGoroutine returning to baseline), but state stayed Running because transition() returned an error that was silently swallowed. Diagnosis required goroutine count analysis and understanding that the race detector was suppressing the test (failing fast) rather than blocking the goroutine.

## Next Phase Readiness

- Plan 02 (Stop()) is unblocked: mp.cmd is set by Start(), mp.doneCh pattern is established, monitorProcess closes doneCh and handles StateStopping->StateStopped
- The doneCh nil check in monitorProcess handles the race between Stop() and natural exit correctly
- Pitfall 5 from RESEARCH.md (doneCh nil check race) is resolved: monitorProcess clears doneCh to nil and Stop() will check state after acquiring the lock

---
*Phase: 06-scheduler-start-stop-and-lifecycle*
*Completed: 2026-03-02*
