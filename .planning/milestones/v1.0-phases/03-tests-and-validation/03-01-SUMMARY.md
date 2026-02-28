---
phase: 03-tests-and-validation
plan: 01
subsystem: testing
tags: [go, testing, process, exit-codes, zombie-prevention, signal-forwarding, subprocess, re-exec]

# Dependency graph
requires:
  - phase: 01-process-foundation
    provides: process.Run() implementation with cmd.Start()+doneCh zombie-safe pattern
  - phase: 02-signal-forwarding
    provides: SIGINT/SIGTERM forwarding via cmd.Process.Signal() producing 128+N exit codes
provides:
  - Automated unit tests for all 5 testable process runner behaviors (TEST-01 through TEST-05)
  - TestHelperProcess re-exec scaffold for controlled subprocess testing
  - go test -race clean on internal/process package
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "TestHelperProcess re-exec: spawn test binary itself with RTX_TEST_HELPER=1 sentinel to create controlled subprocesses for signal/zombie tests"
    - "Table-driven exit code tests: direct call to Run() in same package, assert on returned int"
    - "/proc/<pid>/status zombie check: missing file = reaped = PASS; file present + non-Z = PASS; State:Z = FAIL"
    - "200ms sleep before signal send: gives process.Run() time to register signal.Notify after cmd.Start()"

key-files:
  created:
    - internal/process/runner_test.go
  modified: []

key-decisions:
  - "TestHelperProcess guard uses plain return (not t.Skip()) to avoid SKIP noise in normal test runs"
  - "Zombie test uses re-exec helper with cmd.Stderr = &buf to capture PID from [rtx] spawned PID log"
  - "Signal test targets helper subprocess with SIGTERM; process.Run() inside helper forwards to sleep grandchild"
  - "go vet scoped to ./internal/process/... (not ./...) — pre-existing api package build failures are out of scope"
  - "All 5 tests use stdlib only — zero new dependencies added"

patterns-established:
  - "Re-exec helper pattern: exec.Command(os.Args[0], \"-test.run=TestHelperProcess\", \"--\", cmd...) with RTX_TEST_HELPER=1"
  - "Subprocess exit code chain: SIGTERM -> helper (process.Run) -> sleep -> 143 propagated up"

requirements-completed: [TEST-01, TEST-02, TEST-03, TEST-04, TEST-05]

# Metrics
duration: 6min
completed: 2026-02-28
---

# Phase 3 Plan 01: Tests and Validation Summary

**Five automated Go unit tests covering exit code propagation (codes 1, 42, 127), zombie prevention via /proc inspection, and SIGTERM forwarding producing exit code 143, all using the TestHelperProcess re-exec pattern and stdlib only**

## Performance

- **Duration:** 6 min
- **Started:** 2026-02-28T15:15:53Z
- **Completed:** 2026-02-28T15:21:38Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Created `internal/process/runner_test.go` with 4 test functions covering all 5 automated requirements
- TestRunExitCodes: table-driven tests for exit codes 1 (`false`), 42 (`sh -c 'exit 42'`), 127 (nonexistent command) — TEST-01, TEST-02, TEST-05
- TestZombiePrevention: re-exec helper running `true`, PID extracted from stderr, /proc/<pid>/status verified non-zombie after Run() returns — TEST-03
- TestSignalDelivery: re-exec helper running `sleep 10`, SIGTERM sent after 200ms, exit code 143 (128+15) asserted — TEST-04
- `go test -race -v ./internal/process/...` passes all tests with no data races
- `go vet ./internal/process/...` passes with no warnings

## Task Commits

Each task was committed atomically:

1. **Task 1: Create runner_test.go with exit code tests, zombie prevention, signal delivery, and TestHelperProcess scaffold** - `1461ca9` (feat)

**Plan metadata:** (pending final commit)

## Files Created/Modified

- `internal/process/runner_test.go` - All Phase 3 automated unit tests: TestHelperProcess re-exec scaffold, TestRunExitCodes (table-driven, TEST-01/02/05), TestZombiePrevention (TEST-03), TestSignalDelivery (TEST-04)

## Decisions Made

- TestHelperProcess guard uses `return` (not `t.Skip()`) — avoids printing SKIP on every normal test run
- Zombie test uses re-exec helper subprocess with `cmd.Stderr = &helperStderr` to capture the `[rtx] spawned PID <n>` log line; direct call to `process.Run()` would write to `os.Stderr` (hardcoded, uncapturable in same process)
- Signal test spawns `sleep 10` as grandchild via helper so there is a long-lived process to receive SIGTERM; `process.Run()` inside helper forwards the signal; exit code 143 propagates back through the helper subprocess
- `go vet` scoped to `./internal/process/...` — pre-existing build failures in `internal/api/` are out of scope (documented in Phase 2 deferred-items.md)
- No new dependencies added; all stdlib

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Self-Check: PASSED

All artifacts verified:
- `internal/process/runner_test.go` — exists
- `.planning/phases/03-tests-and-validation/03-01-SUMMARY.md` — exists
- Commit `1461ca9` — exists in git log

## Next Phase Readiness

- All 5 automated test requirements (TEST-01 through TEST-05) are complete and passing
- Phase 3 automated testing is complete; Phase 3 is done (TEST-06 is explicitly documented as a manual-only validation step in the research)
- The process runner implementation is fully verified: exit codes, zombie prevention, and signal forwarding all confirmed by automated tests with race detector

---
*Phase: 03-tests-and-validation*
*Completed: 2026-02-28*
