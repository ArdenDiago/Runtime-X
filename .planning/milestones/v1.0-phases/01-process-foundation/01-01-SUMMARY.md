---
phase: 01-process-foundation
plan: 01
subsystem: process
tags: [go, exec, process, zombie-prevention, exit-codes, fd-inheritance]

# Dependency graph
requires: []
provides:
  - "internal/process package: Run(name string, args []string) int"
  - "Direct fd inheritance for real-time stdout/stderr streaming"
  - "doneCh goroutine pattern preventing zombie processes"
  - "Exact exit code extraction via *exec.ExitError type assertion"
  - "Command-not-found detection returning 127 via exec.ErrNotFound"
  - "Child process group isolation via Setpgid: true"
affects: [02-process-foundation, signal-forwarding, cli-wrapper]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "cmd.Start() + doneCh buffered channel + goroutine for zombie-safe wait"
    - "Direct fd assignment (cmd.Stdout = os.Stdout) instead of StdoutPipe()"
    - "errors.Is(err, exec.ErrNotFound) for command-not-found detection"
    - "errors.As(err, &exitErr) for exact exit code extraction"
    - "Setpgid: true isolating child in own process group for Phase 2 signal forwarding"

key-files:
  created:
    - internal/process/runner.go
  modified: []

key-decisions:
  - "Used cmd.Start() not cmd.Run() — mandatory for doneCh goroutine pattern and future signal forwarding"
  - "Direct fd inheritance (cmd.Stdout = os.Stdout) instead of StdoutPipe() — avoids pipe goroutine race conditions"
  - "Setpgid: true from Phase 1 — child in own group so Phase 2 can forward signals explicitly"
  - "errors.Is(exec.ErrNotFound) not string matching — correct API for command-not-found (ERR-01)"
  - "errors.As(*exec.ExitError) not err != nil check — exact exit code preserved (EXIT-01)"
  - "No os.Exit() inside Run() — caller (main.go) must call os.Exit() for deferred cleanup (EXIT-02)"

patterns-established:
  - "doneCh pattern: make(chan error, 1) + goroutine writing cmd.Wait() + blocking receive — ensures Wait() called on every code path"
  - "fd inheritance pattern: cmd.Stdin/Stdout/Stderr = os.Stdin/Stdout/Stderr — real-time streaming without pipe buffering"
  - "resolveExitCode helper: nil=0, ExitError=exact code, other=1 with log — complete exit code handling"

requirements-completed: [PROC-01, PROC-02, PROC-03, PROC-04, PROC-05, EXIT-01, EXIT-02, ERR-01, ERR-02, LOG-01, LOG-03]

# Metrics
duration: 5min
completed: 2026-02-28
---

# Phase 1 Plan 01: Process Runner Core Summary

**Go process runner using cmd.Start()+doneCh goroutine pattern with direct fd inheritance for real-time streaming and exact exit code extraction via ExitError type assertion**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-02-28T03:40:47Z
- **Completed:** 2026-02-28T03:45:00Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Created `internal/process/runner.go` with `Run(name, args)` as the sole exported function
- Implemented zombie-safe wait using buffered doneCh channel + goroutine pattern (PROC-01/PROC-04)
- Direct fd inheritance for real-time streaming without buffering (PROC-02/PROC-03)
- Exact exit code extraction: 127 for command-not-found, ExitError code for child exits, 1 for unexpected failures
- Child process group isolation via Setpgid: true (PROC-05) enabling Phase 2 signal forwarding
- stderr logging: `[rtx] spawned PID <n>` immediately after start, `[rtx] exited with code <n>` before return

## Task Commits

Each task was committed atomically:

1. **Task 1: Create internal/process package with Run function** - `009ba36` (feat)

**Plan metadata:** (docs commit pending)

## Files Created/Modified

- `internal/process/runner.go` - Core process runner: Run() and resolveExitCode() functions, 65 lines

## Decisions Made

- Used `cmd.Start()` not `cmd.Run()`: mandatory to enable the doneCh goroutine pattern and leave room for Phase 2 signal forwarding select case
- Direct fd assignment `cmd.Stdout = os.Stdout`: avoids pipe goroutines and race conditions (PROC-02/PROC-03)
- `Setpgid: true` from Phase 1 even though signal forwarding is Phase 2: Phase 1 comment documents the orphan risk, Phase 2 closes the gap
- `errors.Is(err, exec.ErrNotFound)` for command-not-found: correct Go API, not fragile string matching
- `errors.As(err, &exitErr)` to extract ExitCode(): preserves exact child exit code instead of always returning 1

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/process` package compiles cleanly (`go build` + `go vet` both exit 0)
- Plan 02 (CLI wrapper) can now import `runtimex/internal/process` and call `process.Run()`
- No blockers for Phase 2 signal forwarding — doneCh pattern and Setpgid already in place

---
*Phase: 01-process-foundation*
*Completed: 2026-02-28*
