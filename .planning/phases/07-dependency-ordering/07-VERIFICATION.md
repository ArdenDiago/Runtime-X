---
phase: 07-dependency-ordering
verified: 2026-03-02T12:00:00Z
status: passed
score: 9/9 must-haves verified
re_verification: false
---

# Phase 7: Dependency Ordering Verification Report

**Phase Goal:** Processes start in topological order — a process waits for its dependencies to be running before it starts, and circular dependencies are rejected at registration time
**Verified:** 2026-03-02
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Registering a process with DependsOn referencing a nonexistent name returns ErrDependencyNotFound | VERIFIED | `TestTopoCheck_MissingDependency` + `TestRegister_RejectsMissingDependency` PASS; `topoCheck` first-pass validates all names before Kahn's BFS |
| 2 | Registering a process that depends on itself returns ErrDependencyCycle | VERIFIED | `TestTopoCheck_SelfDependency` + `TestRegister_RejectsSelfDependency` PASS; self-dep caught in first loop before graph build |
| 3 | Registering a valid chain (A -> B -> C) succeeds without error | VERIFIED | `TestTopoCheck_ValidChain` + `TestRegister_AcceptsValidDependency` PASS |
| 4 | topoOrder returns processes grouped into correct dependency layers | VERIFIED | `TestTopoOrder_Chain` and `TestTopoOrder_Independent` PASS; alphabetical sort within layers confirmed |
| 5 | A diamond dependency (A -> B,C -> D) produces 3 layers with A first and D last | VERIFIED | `TestTopoOrder_Diamond` PASS; layer[0]=["a"], layer[1]=["b","c"], layer[2]=["d"] |
| 6 | StartAll() starts all registered processes in topological order — dependencies run before dependents | VERIFIED | `TestStartAll_Chain` + `TestStartAll_DiamondDependency` PASS with real `sleep 30` processes; waitRunning guards each layer |
| 7 | A diamond dependency (A -> B,C -> D) starts A first, then B and C, then D — A is not started twice | VERIFIED | `TestStartAll_DiamondDependency` PASS; all 4 processes reach StateRunning; StartAll iterates topoOrder layers, never re-visits a node |
| 8 | StartAll() with independent processes starts them all successfully | VERIFIED | `TestStartAll_IndependentProcesses` + `TestStartAll_EmptyScheduler` + `TestStartAll_SingleProcess` PASS |
| 9 | Start() on a process whose dependencies are not running returns a clear error | VERIFIED | `TestStart_RejectsDependencyNotRunning` PASS; returns `ErrDependencyNotReady`; api remains StateIdle after rejection |

**Score:** 9/9 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/scheduler/deps.go` | topoCheck, topoOrder, sentinel errors | VERIFIED | 215 lines; exports `ErrDependencyCycle`, `ErrDependencyNotFound`, `ErrDependencyNotReady`; contains `topoCheck`, `topoOrder`, `checkDepsRunning`, `StartAll`, `waitRunning` — all substantive implementations |
| `internal/scheduler/deps_test.go` | Unit + integration tests; min 80 lines (plan 01), min 150 (plan 02) | VERIFIED | 499 lines; 20 test functions covering all scenarios |
| `internal/scheduler/scheduler.go` | Register() with topoCheck integration | VERIFIED | Line 70: `if err := topoCheck(s.processes, def); err != nil` — called inside write lock after duplicate check |

**Artifact level check:**

- deps.go: EXISTS (215 lines) — SUBSTANTIVE (full Kahn's BFS, layer ordering, error wrapping) — WIRED (called by Register and Start)
- deps_test.go: EXISTS (499 lines, well above both 80 and 150 minimums) — SUBSTANTIVE (5 topoCheck, 5 topoOrder, 3 Register integration, 7 StartAll integration tests) — WIRED (same package, directly calls unexported functions)
- scheduler.go: EXISTS — SUBSTANTIVE — WIRED (topoCheck called at line 70 inside Register under write lock)

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/scheduler/scheduler.go` | `internal/scheduler/deps.go` | `Register()` calls `topoCheck()` when `len(def.DependsOn) > 0` | VERIFIED | Pattern `topoCheck(s.processes, def)` confirmed at scheduler.go line 70; guarded by `if len(def.DependsOn) > 0` |
| `internal/scheduler/deps.go` | `internal/scheduler/lifecycle.go` | `StartAll()` calls `s.Start()` per layer; `waitRunning` polls state | VERIFIED | `s.Start(name)` called in StartAll() line 181; `waitRunning(s, name, 10*time.Second)` called line 187 |
| `internal/scheduler/deps.go` | `internal/scheduler/deps.go` | `StartAll()` calls `topoOrder()` to get layer ordering | VERIFIED | Pattern `topoOrder(` confirmed at deps.go line 173: `layers, err := topoOrder(processesCopy)` |
| `internal/scheduler/lifecycle.go` | `internal/scheduler/deps.go` | `Start()` calls `checkDepsRunning()` before StateStarting transition | VERIFIED | lifecycle.go lines 49-53: `checkDepsRunning(s.processes, mp.Def)` called under write lock before `transition(mp, StateStarting)` |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| DEP-01 | 07-01, 07-02 | User can specify that process B depends on process A (B starts only after A is running) | SATISFIED | `DependsOn []string` on `ProcessDef` is validated at Register() time via `topoCheck()`; enforced at Start() time via `checkDepsRunning()`; `TestStart_RejectsDependencyNotRunning` and `TestStart_AcceptsDependencyRunning` prove this contract |
| DEP-02 | 07-02 | Scheduler starts processes in topological order respecting all dependency edges | SATISFIED | `StartAll()` calls `topoOrder()` to get layers, starts each layer sequentially, calls `waitRunning()` before proceeding to next layer; `TestStartAll_Chain` and `TestStartAll_DiamondDependency` prove correct ordering with real processes |
| DEP-03 | 07-01 | Circular dependencies are detected and rejected at registration time with a clear error | SATISFIED | `topoCheck()` performs self-dep check then Kahn's BFS processed-count check; called in `Register()` before the process is accepted; `TestRegister_RejectsSelfDependency` and `TestRegister_RejectsMissingDependency` prove rejection with sentinel errors |

**Requirements from REQUIREMENTS.md cross-check:**

REQUIREMENTS.md maps DEP-01, DEP-02, DEP-03 to Phase 7 — all three are claimed by the plans and confirmed verified above. No orphaned requirements.

---

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `internal/scheduler/lifecycle_test.go` | Flaky tests `TestStart_CapturesOutput`, `TestStart_CapturesStderr` (pre-existing) | Warning | Intermittent failures (~1 in 10 runs) due to timing race in log capture. Not caused by Phase 7. Documented in deferred-items.md. |

No blocker anti-patterns found in Phase 7 files:

- No `TODO`/`FIXME`/`PLACEHOLDER` comments in deps.go, scheduler.go, or lifecycle.go
- No stub return values (`return nil, nil` in topoOrder for empty map is correct per spec, not a stub)
- No empty handlers
- All sentinel errors are used (not just declared)
- `StartAll()` processes real layers, not hardcoded returns

---

### Human Verification Required

No items require human verification. All behaviors are verified programmatically:

- Registration rejection: verified via `errors.Is()` checks in tests
- Start rejection: verified via `errors.Is(err, ErrDependencyNotReady)` and state assertion
- StartAll ordering: verified via real process state checks (`StateRunning`) after `StartAll()` returns
- Race safety: verified via `go test -race -count=3` with zero data races across all runs

---

### Test Suite Results

```
go test -race -count=1 ./internal/scheduler/ → ok (1.159s)
go test -race -count=3 ./internal/scheduler/ → ok (1.359s)
go build ./...                               → OK
go vet ./...                                 → OK
```

Phase 7 tests (20 functions, all PASS):

- `TestTopoCheck_SelfDependency` PASS
- `TestTopoCheck_MissingDependency` PASS
- `TestTopoCheck_ValidChain` PASS
- `TestTopoCheck_ValidDiamond` PASS
- `TestTopoCheck_DuplicateDependency` PASS
- `TestTopoOrder_SingleProcess` PASS
- `TestTopoOrder_Chain` PASS
- `TestTopoOrder_Diamond` PASS
- `TestTopoOrder_Independent` PASS
- `TestTopoOrder_EmptyMap` PASS
- `TestRegister_RejectsSelfDependency` PASS
- `TestRegister_RejectsMissingDependency` PASS
- `TestRegister_AcceptsValidDependency` PASS
- `TestStartAll_IndependentProcesses` PASS
- `TestStartAll_Chain` PASS
- `TestStartAll_DiamondDependency` PASS
- `TestStartAll_EmptyScheduler` PASS
- `TestStartAll_SingleProcess` PASS
- `TestStart_RejectsDependencyNotRunning` PASS
- `TestStart_AcceptsDependencyRunning` PASS

Existing Phase 5/6 tests: all PASS (zero regressions)

---

### Gaps Summary

None. All must-haves are verified. Phase goal achieved.

---

_Verified: 2026-03-02_
_Verifier: Claude (gsd-verifier)_
