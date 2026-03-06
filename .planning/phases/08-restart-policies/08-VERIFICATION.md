---
phase: 08-restart-policies
verified: 2026-03-06T10:30:00Z
status: passed
score: 5/5 must-haves verified
re_verification: false
gaps: []
human_verification: []
---

# Phase 8: Restart Policies Verification Report

**Phase Goal:** Processes with a restart policy automatically restart after exit according to their configured mode and exponential backoff — and pending restarts can be cancelled by an explicit stop
**Verified:** 2026-03-06T10:30:00Z
**Status:** PASSED
**Re-verification:** No — initial verification (previous file was a stub template with all-Pending status)

---

## Goal Achievement

### Observable Truths (from ROADMAP.md Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A process configured with on-failure restart automatically restarts after a non-zero exit code | VERIFIED | `TestRestartOnFailure` passes: `false` command (exit 1), `RestartOnFailure` mode, `RestartCount >= 1` confirmed. `monitorProcess` checks `err != nil || code != 0` for `RestartOnFailure`. |
| 2 | A process configured with never restart does not restart after any exit | VERIFIED | `monitorProcess` defaults `shouldRestart = false` for any mode not `RestartAlways` or `RestartOnFailure`. Zero-value `RestartPolicy{}` (`Mode = ""`) hits the default case — no restart. Covered by `TestMonitor_CleanExitToStopped` and `TestMonitor_NonZeroExitToFailed`. |
| 3 | Restart delays grow exponentially (e.g., 1s, 2s, 4s, 8s) and are capped at the configured max delay | VERIFIED | `calcDelay` in `restart.go` uses `math.Pow(factor, restartCount-1)`. Verified programmatically: Delay=1s produces 1s, 2s, 4s, 8s (capped to MaxDelay). `BackoffFactor` defaults to 2.0 when zero. |
| 4 | After reaching max retries the process status becomes failed and no further restart is attempted | VERIFIED | `TestRestartMaxRetries`: `MaxRetries=3`, process exits with code 1 each time. Test confirms `RestartCount == 3` and final state is `StateFailed`. `monitorProcess` uses `withinBudget` check: `MaxRetries == 0` (unlimited) or `RestartCount < MaxRetries`. |
| 5 | Calling stop on a process that is waiting in a backoff delay cancels the pending restart immediately | VERIFIED | `TestStopDuringRestart`: 2s backoff delay, `Stop()` called while in `StateRestarting`. Elapsed time < 500ms confirmed. `Stop()` closes `restartCancelCh` and transitions `Stopping -> Stopped` without SIGTERM. |

**Score:** 5/5 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/scheduler/types.go` | `RestartPolicy` with `MaxDelay`/`BackoffFactor`; `StateRestarting` enum; `restartCancelCh` on `ManagedProcess`; FSM transitions | VERIFIED | All fields present (lines 23-36, 82-86, 184-187). FSM table at lines 115-129 includes all 4 `StateRestarting` edges. `String()` returns `"restarting"` at line 103. |
| `internal/scheduler/restart.go` | `waitAndRestart` goroutine with backoff + cancellation; `calcDelay` helper | VERIFIED | 77-line file. `waitAndRestart` reads policy+cancelCh under RLock, computes delay, selects on `timer.C`/`cancelCh`. `calcDelay` uses `math.Pow`, applies `MaxDelay` cap. |
| `internal/scheduler/lifecycle.go` | Updated `monitorProcess` (policy check, restart branching), `Stop()` (Restarting fast-path), `Start()` (StateRestarting allowed) | VERIFIED | `monitorProcess` at line 264: policy evaluation, `shouldRestart` logic, `withinBudget` check, `go waitAndRestart(s, mp)` launch. `Stop()` at line 167: `StateRestarting` case closes `restartCancelCh`, transitions directly to Stopped. `Start()` at line 39: `case StateRestarting` allowed. |
| `internal/scheduler/lifecycle_test.go` | 4 integration tests: RestartAlways, RestartOnFailure, MaxRetries, StopDuringRestart | VERIFIED | All 4 tests present and pass with `-race` flag. `TestRestartAlways` (line 595), `TestRestartOnFailure` (line 647), `TestRestartMaxRetries` (line 695), `TestStopDuringRestart` (line 730). |
| `internal/scheduler/scheduler_test.go` | `TestStateTransitions` updated with `StateRestarting` edges; `TestStateString` updated | VERIFIED | Lines 286-309: 4 valid transitions (`running->restarting`, `restarting->starting`, `restarting->stopping`, `restarting->failed`) and 3 invalid ones (`restarting->idle`, `restarting->running`, `restarting->stopped`). Line 367: `"restarting"` string case. |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `monitorProcess` | `waitAndRestart` | `go waitAndRestart(s, mp)` under write lock | WIRED | `lifecycle.go` line 313: `go waitAndRestart(s, mp)` launched after `transition(mp, StateRestarting)`. |
| `waitAndRestart` | `s.Start()` | `_ = s.Start(mp.Def.Name)` after `timer.C` | WIRED | `restart.go` line 41: calls `s.Start(mp.Def.Name)` when backoff timer fires. |
| `Stop()` | `restartCancelCh` | `close(cancelCh)` when `StateRestarting` | WIRED | `lifecycle.go` lines 170-179: captures channel, sets field to nil, transitions state, then closes channel. |
| `waitAndRestart` | `cancelCh` | `case <-cancelCh:` in select | WIRED | `restart.go` line 43: `case <-cancelCh: return` — goroutine exits without starting process. |
| `Start()` | `restartCancelCh` (reset) | `mp.restartCancelCh = make(chan struct{})` | WIRED | `lifecycle.go` line 74: fresh channel created on every `Start()` call, enabling repeated restarts. |
| `calcDelay` | `math.Pow` | `math.Pow(factor, float64(exponent))` | WIRED | `restart.go` line 70, imports `math` at line 5. Verified: produces 1s, 2s, 4s, 8s with default factor 2.0. |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| RST-01 | 08-02-PLAN.md | User can configure a process with restart-on-failure policy (restart when exit code != 0) | SATISFIED | `RestartOnFailure` mode in `RestartPolicy`. `monitorProcess` checks `err != nil \|\| code != 0`. `TestRestartOnFailure` passes. |
| RST-02 | 08-01-PLAN.md, 08-02-PLAN.md | Restart uses exponential backoff (initial delay, max delay, max retries configurable per process) | SATISFIED | `RestartPolicy.Delay`, `MaxDelay`, `BackoffFactor`, `MaxRetries` fields in `types.go`. `calcDelay` implements `Delay * (Factor^(N-1))` capped at `MaxDelay`. Programmatically verified: 1s,2s,4s,8s pattern. |
| RST-03 | 08-02-PLAN.md | Restart attempts stop after reaching max retries — process status becomes "failed" | SATISFIED | `withinBudget` check in `monitorProcess` (line 309). When exhausted: `transition(mp, StateFailed)`. `TestRestartMaxRetries` confirms `RestartCount == MaxRetries` and final state `StateFailed`. |
| RST-04 | 08-01-PLAN.md, 08-02-PLAN.md | User can stop a process during a backoff wait period (cancels pending restart) | SATISFIED | `restartCancelCh` field on `ManagedProcess`. `Stop()` `StateRestarting` fast-path closes channel and returns immediately. `TestStopDuringRestart` confirms < 500ms elapsed with 2s backoff. |

**SCH-04 note:** 08-01-PLAN.md also references SCH-04 ("User can list all registered processes with their current status — including restarting"). `StateRestarting.String()` returns `"restarting"`, and `List()` reads `mp.State` under RLock — SCH-04 is fully extended by this phase.

**No orphaned requirements:** All 4 RST requirements (RST-01 through RST-04) are claimed by phase 8 plans and verified as satisfied.

---

### Commit Verification

| Commit | Message | Files | Status |
|--------|---------|-------|--------|
| `c7acffe` | feat(08-01): add StateRestarting, backoff fields, and restartCancelCh | `types.go`, `scheduler_test.go` | EXISTS — confirmed in git log |
| `36b4018` | feat(08-02): implement waitAndRestart with exponential backoff | `restart.go` | EXISTS — confirmed in git log |
| `dee4bb9` | feat(08-02): update lifecycle for restart policies and cancellation | `lifecycle.go`, `lifecycle_test.go` | EXISTS — confirmed in git log |

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | None found | — | — |

No TODOs, FIXMEs, placeholders, empty implementations, or stub returns found in `restart.go`, `lifecycle.go`, or `types.go`.

---

### Test Results

**Full suite run with `-race` flag:**

```
ok  runtimex/internal/scheduler  1.305s
```

**41 tests total — all PASS, no data races detected.**

Restart-specific tests:
- `TestRestartAlways` — PASS (0.02s)
- `TestRestartOnFailure` — PASS (0.02s)
- `TestRestartMaxRetries` — PASS (0.08s)
- `TestStopDuringRestart` — PASS (0.01s)

FSM tests (StateRestarting coverage):
- `TestStateTransitions/valid_transitions_succeed/running->restarting` — PASS
- `TestStateTransitions/valid_transitions_succeed/restarting->starting` — PASS
- `TestStateTransitions/valid_transitions_succeed/restarting->stopping` — PASS
- `TestStateTransitions/valid_transitions_succeed/restarting->failed` — PASS
- `TestStateTransitions/invalid_transitions_return_ErrInvalidTransition/restarting->idle` — PASS
- `TestStateTransitions/invalid_transitions_return_ErrInvalidTransition/restarting->running` — PASS
- `TestStateTransitions/invalid_transitions_return_ErrInvalidTransition/restarting->stopped` — PASS
- `TestStateString/restarting` — PASS

---

### Build Status

`go build ./internal/scheduler/... ./cmd/...` exits 0 with no errors.

Note: `go build ./...` fails on the legacy `api-service/` directory (missing external dependencies — pre-existing issue outside phase 8 scope, present since Phase 4 cleanup). The scheduler package and all phase 8 artifacts compile cleanly.

---

### Human Verification Required

None. All success criteria are mechanically verifiable via tests and code inspection. The race detector confirms concurrent correctness; exponential delay math was verified programmatically.

---

## Summary

Phase 8 goal is fully achieved. The restart policy engine is substantively implemented across three files:

- **Types layer** (`types.go`): `RestartPolicy` extended with `MaxDelay`/`BackoffFactor`; `StateRestarting` added to FSM with all 4 valid edges; `restartCancelCh` added to `ManagedProcess`.
- **Restart goroutine** (`restart.go`): `waitAndRestart` performs exponential backoff via `calcDelay`, selects between timer expiry (triggers restart) and channel close (cancels restart). No stubs — fully functional.
- **Lifecycle integration** (`lifecycle.go`): `monitorProcess` evaluates restart policy after every exit and launches `waitAndRestart` under write lock; `Stop()` handles `StateRestarting` fast-path with immediate channel cancellation; `Start()` accepts `StateRestarting` as a valid caller state.

All 4 RST requirements are satisfied. The implementation passes 41 tests with the `-race` flag with zero data races detected. Three atomic commits are verified in git history.

---

_Verified: 2026-03-06T10:30:00Z_
_Verifier: Claude (gsd-verifier)_
