# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-28)

**Core value:** Correct, deterministic process lifecycle management — no zombies, no orphans, exact exit codes, clean signal forwarding.
**Current focus:** v1.1 — Phase 5: Scheduler Data Structures and Log Buffer

## Current Position

Phase: 5 of 11 (Scheduler Data Structures and Log Buffer)
Plan: 2 of 3 complete
Status: Phase 5 plan 02 complete — scheduler types and registration implemented
Last activity: 2026-03-01 — Phase 5 plan 02 executed (ProcessDef/ManagedProcess/State FSM/Scheduler with TDD)

Progress: [███░░░░░░░] 20% (v1.1) — v1.0 complete, Phase 4 done, Phase 5.1 done, Phase 5.2 done

## Performance Metrics

**Velocity (from v1.0):**
- Total plans completed: 6
- Average duration: 5.75 min
- Total execution time: ~0.6 hours

**By Phase (v1.0):**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 1. Process Foundation | 2 | ~12 min | ~6 min |
| 2. Signal Forwarding | 2 | ~11 min | ~5.5 min |
| 3. Tests and Validation | 2 | ~11 min | ~5.5 min |

**Recent Trend:**
- Last 5 plans: ~6, ~6, ~5.5, ~5.5, ~5.5 min
- Trend: Stable

## Accumulated Context

### Decisions

Full v1.0 decision log in PROJECT.md Key Decisions table (12 decisions, all Good).

Recent decisions relevant to v1.1:
- [v1.1 scope]: net/http stdlib only — no external Go router (Go 1.22+ ServeMux is sufficient for 8 endpoints)
- [v1.1 scope]: React 19 + Vite 7 + TypeScript — no state management or data-fetching libraries
- [v1.1 scope]: Polling only (2-second interval) — WebSocket/SSE deferred to v2
- [v1.1 arch]: Scheduler never calls process.Run() — must use cmd.Start() + doneCh goroutine directly
- [v1.1 arch]: logBuffer needs its own sync.Mutex independent of scheduler RWMutex (log writes come from cmd.Start() goroutines)
- [v1.1 arch]: Release scheduler write lock before cmd.Start() to prevent deadlock

Phase 4 decisions:
- [04 cleanup]: Keep module path as runtimex — renaming adds friction with no benefit
- [04 cleanup]: Removed commented-out uuid require from go.mod — go mod tidy doesn't strip comments

Phase 5 plan 01 decisions:
- [05-01 arch]: sync.Mutex (not RWMutex) on logBuffer — writes and reads at similar frequency; Mutex simpler and faster at balanced ratios
- [05-01 arch]: logBuffer has its own independent mutex separate from Scheduler's RWMutex — prevents Phase 6 deadlock where cmd.Start() goroutines call Write() while scheduler may hold write lock
- [05-01 impl]: Default buffer size 1000 for size <= 0 — prevents divide-by-zero panic in modulo, matches ProcessDef.LogBufferSize zero-value behavior

Phase 5 plan 02 decisions:
- [05-02 arch]: transition() is unexported — Phase 6 calls it from Scheduler methods holding the write lock; same-package tests call it directly
- [05-02 arch]: Remove() permits Idle, Stopped, and Failed states — Idle was never started; Failed may need removal before retry
- [05-02 arch]: Scheduler.Logs() releases RLock before calling mp.logs.Lines() — prevents lock-ordering hazard with Phase 6 writer goroutines
- [05-02 impl]: validateName regexp ^[a-z0-9][a-z0-9-]*$ — first char cannot be hyphen, prevents URL path segment confusion in Phase 9 HTTP handlers

### Pending Todos

None.

### Blockers/Concerns

- [Phase 9/10 gap]: Production static file serving strategy (http.FileServer vs go:embed) needs a decision during Phase 10 planning. Recommendation: http.FileServer(http.Dir("web/dist")) for v1.1 simplicity.
- [Phase 11 gap]: Restart policy form UX (duration input format — seconds as integer vs "5s" string) must match JSON API body format. Decide during Phase 11 planning.

Resolved:
- [RESOLVED Phase 4]: internal/api/handlers.go build failure — entire legacy codebase removed in Phase 4. go build ./... now exits 0.

## Session Continuity

Last session: 2026-03-01
Stopped at: Completed 05-02-PLAN.md (Scheduler types and registration — ProcessDef/ManagedProcess/State FSM with Register/Remove/Get/List/Logs)
Resume file: None
Next: Execute Phase 5 Plan 03 — remaining Phase 5 work (if any) or proceed to Phase 6 Scheduler Process Lifecycle
