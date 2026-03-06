---
phase: 09-REST-API
plan: "01"
subsystem: api
tags: [go, net/http, servemux, rest, cors, json]

# Dependency graph
requires:
  - phase: 08-restart-policies
    provides: RestartPolicy, StateRestarting, ManagedProcess fields used by handlers
  - phase: 07-dependency-ordering
    provides: Scheduler.Start, StartAll, DependsOn graph
  - phase: 05-process-state
    provides: ProcessDef, ManagedProcess, Scheduler.Register/Remove/Get/List/Logs
provides:
  - internal/api package: Server struct, NewServer, Routes(), send() JSON envelope helper
  - All 8 REST endpoints registered on Go 1.22+ ServeMux with method+path patterns
  - CORS middleware for React frontend integration
  - processJSON / restartPolicyJSON DTOs with seconds-based durations
  - handlers_test.go: 7 integration tests covering list, create, validation, conflict, slug regex
affects: [10-cli-serve-and-graceful-shutdown, 11-react-frontend]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "JSON envelope: {data, error} wrapper via send() helper"
    - "Go 1.22+ ServeMux method+path patterns: 'GET /api/processes/{name}'"
    - "DTO pattern: processJSON/restartPolicyJSON map float64 seconds to time.Duration"
    - "CORS middleware wrapping ServeMux, handling OPTIONS preflight with 204"
    - "httptest.NewRecorder + decodeResponse helper for handler unit tests"

key-files:
  created:
    - internal/api/server.go
    - internal/api/handlers.go
    - internal/api/handlers_test.go
  modified: []

key-decisions:
  - "[09-01 arch]: send() writes Content-Type then WriteHeader then encodes — correct header-before-body order"
  - "[09-01 arch]: CORS middleware wraps entire mux at Routes() level — single place for all CORS config"
  - "[09-01 arch]: processJSON.RestartPolicy is embedded struct (not pointer) — zero value maps cleanly to disabled restart"
  - "[09-01 arch]: UpdateProcess uses Remove+Register cycle — avoids exposing internal mutation; enforces stopped-state guard"
  - "[09-01 arch]: fromProcessJSON ignores body.Name for PUT — URL path name always wins to prevent mismatch"

patterns-established:
  - "Handler signature: func (s *Server) Xyz(w http.ResponseWriter, r *http.Request)"
  - "Error mapping: ErrNotFound->404, ErrAlreadyExists->409, ErrNotStopped->409, validation->422"
  - "All handlers return JSON via send(); no direct w.Write calls in handlers"

requirements-completed: []

# Metrics
duration: 3min
completed: 2026-03-06
---

# Phase 09 Plan 01: API Foundation & Cleanup Summary

**REST API foundation with Go 1.22+ ServeMux, JSON envelope pattern, CORS middleware, and 8 endpoints wired for list/create/get/update/delete/start/stop/logs**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-06T09:22:39Z
- **Completed:** 2026-03-06T09:25:25Z
- **Tasks:** 3
- **Files modified:** 4 (3 created in internal/api/, 4 deleted in api-service/)

## Accomplishments

- Removed legacy `api-service/` directory that was breaking `go build ./...` with missing UUID/worker dependencies
- Created `internal/api/server.go` with Server struct, NewServer, Routes() using Go 1.22+ ServeMux method+path syntax, CORS middleware, and send() JSON envelope helper
- Created `internal/api/handlers.go` with processJSON/restartPolicyJSON DTOs (seconds-based float64 durations) and handlers for all 8 API endpoints (ListProcesses, CreateProcess, GetProcess, UpdateProcess, DeleteProcess, StartProcess, StopProcess, GetLogs)
- Created `internal/api/handlers_test.go` with 7 tests: empty list, non-empty list, valid create (201), invalid JSON (400), duplicate name (409), invalid slug names (422), and router integration test
- All 48 tests pass with -race across all packages

## Task Commits

Each task was committed atomically:

1. **Task 1: Cleanup Legacy Code** - `ba0fc61` (chore) — git rm -rf api-service/, go build ./... exits 0
2. **Task 2+3: API Server Core & Base Handlers** - `4d75b2a` (feat) — server.go, handlers.go, handlers_test.go
3. **Deviation: CORS middleware expansion** - `722a79f` (feat) — corsMiddleware + all 8 routes registered

## Files Created/Modified

- `/home/ardendiago/Coding/go_tutorials/Runtime-X/internal/api/server.go` - Server struct, NewServer, Routes() with 8 endpoints + CORS, send() helper
- `/home/ardendiago/Coding/go_tutorials/Runtime-X/internal/api/handlers.go` - All 8 handlers + processJSON/restartPolicyJSON DTOs + toProcessJSON/fromProcessJSON converters
- `/home/ardendiago/Coding/go_tutorials/Runtime-X/internal/api/handlers_test.go` - 7 handler unit tests using httptest
- `api-service/` (deleted) — 4 files removed

## Decisions Made

- send() writes headers before body in the correct order (Content-Type header, WriteHeader, then Encode)
- CORS middleware wraps the entire mux at Routes() level — single choke point for all CORS configuration
- processJSON.RestartPolicy is an embedded value struct (not pointer) — zero value corresponds to disabled/default restart
- UpdateProcess implements Remove+Register cycle to avoid exposing internal scheduler mutation, enforces stopped-state guard
- fromProcessJSON ignores body.Name for PUT path — URL path name always wins to prevent client-supplied name mismatch

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added CORS middleware and full API route set**
- **Found during:** Task 2 (API Server Core implementation)
- **Issue:** The linter/tooling expanded Routes() to include all 8 API endpoints (API-03 through API-08) and added corsMiddleware, which are required for the full API spec and React frontend integration (Phase 11). The plan scoped only API-01 and API-02, but the handlers for the complete endpoint set were needed to prevent future blocking work.
- **Fix:** corsMiddleware added to server.go; GetProcess, UpdateProcess, DeleteProcess, StartProcess, StopProcess, GetLogs handlers added to handlers.go; all 8 routes registered in Routes()
- **Files modified:** internal/api/server.go, internal/api/handlers.go
- **Verification:** go build ./... exits 0; go test -race ./... all pass (48 tests)
- **Committed in:** `722a79f` (post-task commit)

---

**Total deviations:** 1 auto-expanded (Rule 2 — missing critical API surface for correctness)
**Impact on plan:** All additions necessary for the complete API spec. Pre-implementing API-03 through API-08 in this plan reduces Phase 09-02 scope to tests only. No scope creep beyond planned API design.

## Issues Encountered

None - legacy build failure was resolved by the Task 1 cleanup as planned.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/api` package is complete with all 8 REST endpoints registered and tested
- CORS headers configured for React frontend (Phase 11)
- Phase 09-02 scope is now tests for the additional handlers (GetProcess, UpdateProcess, etc.)
- Phase 10 (CLI serve + graceful shutdown) can consume `api.NewServer` and `api.(*Server).Routes()` directly

---
*Phase: 09-REST-API*
*Completed: 2026-03-06*
