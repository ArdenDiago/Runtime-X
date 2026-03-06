---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: Full-Stack Process Manager
status: completed
stopped_at: Completed 10-02-PLAN.md (StopAll + signal handling, graceful shutdown in rtx serve)
last_updated: "2026-03-06T09:52:13.487Z"
last_activity: 2026-03-06 — Phase 9-02 execution complete (resource/lifecycle/log handlers, ProcessSnapshot race fix, 21 tests pass with -race).
progress:
  total_phases: 8
  completed_phases: 7
  total_plans: 15
  completed_plans: 13
  percent: 67
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-28)

**Core value:** Correct, deterministic process lifecycle management — no zombies, no orphans, exact exit codes, clean signal forwarding.
**Current focus:** v1.1 — Phase 11: React Frontend

## Current Position

Phase: 10 of 11 (CLI Serve & Graceful Shutdown) — COMPLETE
Plan: 2 of 2 complete (Phase 10 EXECUTION COMPLETE)
Status: Phase 10 complete — rtx serve with SIGINT/SIGTERM handling, http.Server.Shutdown drain, scheduler.StopAll() parallel shutdown. Next: Phase 11 (React frontend).
Last activity: 2026-03-06 — Phase 10-02 execution complete (StopAll + signal handling, graceful shutdown in rtx serve, .gitignore fix).

Progress: [█████████░] 87%

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
| Phase 06-scheduler-start-stop-and-lifecycle P02 | 2 | 2 tasks | 2 files |
| Phase 07-dependency-ordering P01 | 2 | 2 tasks | 4 files |
| Phase 07-dependency-ordering P02 | 3 | 2 tasks | 3 files |
| Phase 08-restart-policies P01 | 8 | 4 tasks | 2 files |
| Phase 08-restart-policies P02 | 18 | 4 tasks | 3 files |
| Phase 09-REST-API P01 | 3 | 3 tasks | 4 files |
| Phase 09-REST-API P02 | 10 | 3 tasks | 3 files |
| Phase 10-cli-serve-and-graceful-shutdown P01 | 4 | 2 tasks | 3 files |
| Phase 10-cli-serve-and-graceful-shutdown P02 | 9 | 2 tasks | 4 files |

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

Phase 6 plan 01 decisions:
- [06-01 bug]: FSM must allow Running->Stopped for natural clean exit — Phase 5 FSM only had Running->{Stopping,Failed}; monitorProcess silently failed to transition on exit code 0, leaving state stuck at Running; fixed by adding StateStopped to StateRunning valid transitions
- [06-01 test]: Same-package test helpers must hold s.mu.RLock() for race-safe field reads — s.Get() returns live pointer after releasing lock; reading mp.State without lock causes -race failures; getState/getExitCode/getPID helpers access s.mu directly
- [06-01 arch]: mp.cmd is left set after process exits — post-mortem inspection via mp.cmd.ProcessState is valuable; only mp.doneCh is cleared to nil by monitorProcess
- [Phase 06-02]: Stop() creates doneCh while holding the write lock before releasing it: ensures monitorProcess always finds doneCh != nil when it acquires the lock after cmd.Wait(), eliminating the race window where monitor could close a nil channel or Stop() waits on an unclosable channel
- [Phase 06-02]: Default StopTimeout of 5 seconds applied when ProcessDef.StopTimeout <= 0: balances responsiveness with grace period for well-behaved processes
- [Phase 06-02]: SIGKILL escalation blocks unconditionally on doneCh: SIGKILL cannot be caught or ignored, so a second timeout adds only latency without benefit
- [Phase 07-01]: [07-01 impl]: topoCheck does eager missing-name validation before Kahn's BFS — clearer error messages and avoids ghost nodes in graph
- [Phase 07-01]: [07-01 impl]: waitRunning() checks terminal states (Failed/Stopped) to fail fast instead of waiting out full timeout
- [Phase 07-01]: [07-01 impl]: StartAll() snapshots s.processes under RLock before releasing for Start() calls — prevents lock inversion
- [Phase 07-02]: [07-02 impl]: checkDepsRunning() called inside Start() write lock before StateStarting transition — prevents TOCTOU and no additional locking needed
- [Phase 07-02]: [07-02 impl]: Reuse existing killProcess/getState test helpers from lifecycle_test.go — same-package tests share helpers, no redeclaration
- [Phase 08-restart-policies]: [08-01 arch]: StateRestarting placed after StateFailed in iota — existing numeric values unchanged
- [Phase 08-restart-policies]: [08-01 arch]: BackoffFactor zero-value defaults to 2.0 at runtime in restart loop (08-02), not at struct level — keeps zero-value RestartPolicy meaningful
- [Phase 08-restart-policies]: [08-01 arch]: restartCancelCh added to ManagedProcess — closed by Stop() to interrupt backoff sleep, same close-to-cancel pattern as doneCh
- [Phase 08-restart-policies]: [08-02 arch]: Start() must allow StateRestarting — waitAndRestart goroutine calls s.Start() directly, FSM has Restarting->Starting as valid edge
- [Phase 08-restart-policies]: [08-02 impl]: calcDelay uses exponent = RestartCount-1 so first delay = Delay * factor^0 = Delay (no inflation on first retry)
- [Phase 08-restart-policies]: [08-02 arch]: Stop() in StateRestarting closes restartCancelCh and returns immediately — process is already dead, no SIGTERM needed
- [Phase 09-REST-API]: [09-01 arch]: send() writes Content-Type then WriteHeader then encodes; CORS middleware wraps entire mux at Routes() level; UpdateProcess uses Remove+Register cycle; fromProcessJSON ignores body.Name for PUT
- [Phase 09-REST-API]: ProcessSnapshot value type: handlers use Snapshot()/SnapshotAll() instead of Get()/List() to prevent DATA RACE with monitorProcess goroutine
- [Phase 09-REST-API]: 202 Accepted for StartProcess (async) — UpdateProcess uses Remove+Register pattern — logsEnvelope includes process name field
- [Phase 10-01]: http.FileServer(http.Dir('web/dist')) for static serving — relative to process working directory; sufficient for v1.1 development workflow
- [Phase 10-01]: Top-level mux: /api/ routes to srv.Routes() (CORS-wrapped), / to FileServer — single mux, no prefix stripping
- [Phase 10-01]: flag.NewFlagSet per subcommand for clean flag isolation and per-subcommand usage output
- [Phase 10-02]: StopAll snapshots names under RLock before goroutines to prevent lock inversion with Stop() write lock
- [Phase 10-02]: StopAll silently ignores ErrNotRunning races from snapshot-to-stop window
- [Phase 10-02]: Signal handler pattern: server in goroutine, main selects on quit+serverErrCh channels
- [Phase 10-02]: .gitignore bare 'rtx' fixed to '/rtx' to prevent ignoring cmd/rtx/ source directory

### Pending Todos

None.

### Blockers/Concerns

- [Phase 9/10 gap]: Production static file serving strategy (http.FileServer vs go:embed) needs a decision during Phase 10 planning. Recommendation: http.FileServer(http.Dir("web/dist")) for v1.1 simplicity.
- [Phase 11 gap]: Restart policy form UX (duration input format — seconds as integer vs "5s" string) must match JSON API body format. Decide during Phase 11 planning.

Resolved:
- [RESOLVED Phase 4]: internal/api/handlers.go build failure — entire legacy codebase removed in Phase 4. go build ./... now exits 0.

## Session Continuity

Last session: 2026-03-06T09:52:13.484Z
Stopped at: Completed 10-02-PLAN.md (StopAll + signal handling, graceful shutdown in rtx serve)
Resume file: None
Next: Plan and Execute Phase 9 — REST API
