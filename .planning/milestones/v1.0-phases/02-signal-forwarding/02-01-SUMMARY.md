---
phase: 02-signal-forwarding
plan: 01
subsystem: process
tags: [go, os/signal, syscall, SIGINT, SIGTERM, exit-codes, posix]

# Dependency graph
requires:
  - phase: 01-process-foundation
    provides: "cmd.Start()+doneCh goroutine pattern with Setpgid:true — designed to receive signal select case"
provides:
  - "SIGINT/SIGTERM interception via buffered os.Signal channel in Run()"
  - "Signal forwarding to child via cmd.Process.Signal(sig) with os.ErrProcessDone guard"
  - "POSIX-correct exit codes: 128+N for signal-killed children (130=SIGINT, 143=SIGTERM)"
  - "Signal receipt logging: '[rtx] received signal %s' to stderr"
affects: [03-scheduler-foundation, any phase testing signal behavior]

# Tech tracking
tech-stack:
  added: [os/signal]
  patterns:
    - "signal.Notify after cmd.Start() (not before — no child target before start)"
    - "buffered os.Signal channel capacity 1 (signal.Notify non-blocking send requirement)"
    - "select with sigCh case + doneCh case for concurrent signal/exit handling"
    - "errors.Is(err, os.ErrProcessDone) guard on Signal() to handle natural-exit race"
    - "waitErr = <-doneCh in signal case for zombie prevention after forwarding"
    - "WaitStatus.Signaled() + 128+int(ws.Signal()) for POSIX exit code emulation"

key-files:
  created: []
  modified:
    - internal/process/runner.go

key-decisions:
  - "Signal channel setup after cmd.Start() (not before) — no child to forward to before process exists"
  - "Forward to child PID only (cmd.Process.Signal), not process group — Setpgid:true isolates child, PID-targeted forward is correct"
  - "Block on <-doneCh after signal forward — zombie prevention and correct exit code extraction"
  - "Swallow os.ErrProcessDone from Signal() call — handles natural-exit race without spurious error log"
  - "resolveExitCode accepts *os.ProcessState parameter for 128+N computation"
  - "Tasks 1 and 2 committed atomically (same file, single write) — functionally equivalent to sequential commits"

patterns-established:
  - "Signal forwarding pattern: Notify → select → forward → wait (SIG-01, SIG-02, SIG-03, SIG-04)"
  - "POSIX exit code pattern: ExitCode()==-1 check → WaitStatus.Signaled() → 128+N (EXIT-03)"
  - "Error guard pattern: errors.Is(err, os.ErrProcessDone) for forwarding to potentially-dead process (ERR-03)"

requirements-completed: [SIG-01, SIG-02, SIG-03, SIG-04, EXIT-03, ERR-03, LOG-02]

# Metrics
duration: 3min
completed: 2026-02-28
---

# Phase 2 Plan 01: Signal Forwarding Summary

**SIGINT/SIGTERM forwarding to child via buffered os.Signal channel select with POSIX 128+N exit code emulation via WaitStatus.Signaled()**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-28T11:06:13Z
- **Completed:** 2026-02-28T11:08:56Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- Extended Run() with buffered os.Signal channel, signal.Notify, and a select block handling both signal arrival and natural exit concurrently
- Signal forwarding to child PID with os.ErrProcessDone guard prevents spurious errors on natural-exit race
- resolveExitCode() now accepts *os.ProcessState and emits 128+N via WaitStatus.Signaled() for signal-killed children
- All Phase 1 behavior preserved: natural exit codes, command-not-found 127, PID logging

## Task Commits

Each task was committed atomically:

1. **Task 1 + Task 2: Signal interception, forwarding, and 128+N exit codes** - `1a38c5e` (feat)

Note: Both tasks modify the same file; they were implemented in a single atomic write and committed together. All done criteria for both tasks verified before commit.

**Plan metadata:** (to be recorded after state update commit)

## Files Created/Modified

- `internal/process/runner.go` - Extended Run() with signal forwarding select block; resolveExitCode() updated with *os.ProcessState parameter and WaitStatus.Signaled() for 128+N exit codes

## Decisions Made

- Signal channel initialized AFTER cmd.Start() returns — before that, there is no child process to forward to; signals received during this window would be lost anyway
- Used cmd.Process.Signal(sig) targeting child PID, not syscall.Kill(-pgid, sig) for process group — Setpgid:true means child is isolated; PID-targeted forward is the correct single-process approach
- Blocked on waitErr = <-doneCh in signal case before returning — ensures zombie prevention (PROC-04) and gives resolveExitCode the populated cmd.ProcessState
- Swallowed os.ErrProcessDone from Signal() without logging — this is a benign race (child exits naturally just as signal arrives); logging it would be misleading noise

## Deviations from Plan

None — plan executed exactly as written.

Pre-existing build failures in `internal/api/handlers.go` discovered during `go build ./...` verification. Confirmed pre-existing (not caused by this plan's changes). Logged to `deferred-items.md` in phase directory. Plan verification scoped to `./internal/process/...` and `./cmd/rtx/...` which build and vet cleanly.

## Issues Encountered

None beyond pre-existing out-of-scope API build failures documented in deferred-items.md.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Signal forwarding fully implemented and verified with live binary tests (SIGINT exit 130, SIGTERM exit 143, natural exit unchanged)
- Phase 02-02 (if it exists) can build directly on this runner foundation
- Phase 3 (Scheduler) has a clean, signal-aware process runner to orchestrate

## Self-Check: PASSED

- internal/process/runner.go: FOUND
- .planning/phases/02-signal-forwarding/02-01-SUMMARY.md: FOUND
- commit 1a38c5e: FOUND
- go build ./internal/process/... ./cmd/rtx/...: PASSED
- go vet ./internal/process/... ./cmd/rtx/...: PASSED
- SIGINT exit code 130: VERIFIED (live binary test)
- SIGTERM exit code 143: VERIFIED (live binary test)
- Natural exit code propagation: VERIFIED (sh -c 'exit 42' -> 42)

---
*Phase: 02-signal-forwarding*
*Completed: 2026-02-28*
