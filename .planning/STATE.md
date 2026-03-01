# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-28)

**Core value:** Correct, deterministic process lifecycle management — no zombies, no orphans, exact exit codes, clean signal forwarding.
**Current focus:** v1.1 — Phase 4: Codebase Cleanup

## Current Position

Phase: 4 of 11 (Codebase Cleanup)
Plan: — of — (not yet planned)
Status: Ready to plan
Last activity: 2026-03-01 — v1.1 roadmap created (8 phases, 35 requirements mapped)

Progress: [░░░░░░░░░░] 0% (v1.1) — v1.0 complete

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

### Pending Todos

None.

### Blockers/Concerns

- [Phase 4 prerequisite]: internal/api/handlers.go has a pre-existing build failure (references undefined type from deleted models package). Phase 4 exists specifically to fix this — go build ./... is currently broken.
- [Phase 9/10 gap]: Production static file serving strategy (http.FileServer vs go:embed) needs a decision during Phase 10 planning. Recommendation: http.FileServer(http.Dir("web/dist")) for v1.1 simplicity.
- [Phase 11 gap]: Restart policy form UX (duration input format — seconds as integer vs "5s" string) must match JSON API body format. Decide during Phase 11 planning.

## Session Continuity

Last session: 2026-03-01
Stopped at: Roadmap created for v1.1 (Phases 4-11)
Resume file: None
Next: `/gsd:plan-phase 4` — plan Phase 4: Codebase Cleanup
