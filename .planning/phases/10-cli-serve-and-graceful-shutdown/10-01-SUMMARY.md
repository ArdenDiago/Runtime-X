---
phase: 10-cli-serve-and-graceful-shutdown
plan: "01"
subsystem: cli
tags: [go, net/http, http.FileServer, flag, subcommands, cli]

# Dependency graph
requires:
  - phase: 09-REST-API
    provides: api.Server, api.NewServer(), srv.Routes() CORS-wrapped handler
  - phase: 06-scheduler-start-stop-and-lifecycle
    provides: scheduler.New() constructor
provides:
  - rtx serve subcommand that starts REST API and serves React frontend from web/dist
  - rtx run isolated into cmdRun(args []string) with its own FlagSet
  - cmd/rtx/serve.go with cmdServe() function
  - web/dist/ directory with placeholder index.html
affects: [11-react-frontend, future-cli-improvements]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "flag.NewFlagSet per subcommand — each subcommand owns its own FlagSet"
    - "top-level mux pattern: /api/ routes to API handler, / to http.FileServer"

key-files:
  created:
    - cmd/rtx/serve.go
    - web/dist/index.html
  modified:
    - cmd/rtx/main.go

key-decisions:
  - "http.FileServer(http.Dir('web/dist')) for static serving — relative to process working directory"
  - "Top-level mux: /api/ routes to srv.Routes() (CORS-wrapped), / to FileServer — no prefix stripping needed"
  - "cmdRun uses flag.NewFlagSet('run', ...) for clean per-subcommand flag isolation"

patterns-established:
  - "subcommand dispatch: switch args[0] -> cmdRun / cmdServe, each with own FlagSet"
  - "mux layering: API routes under /api/, static files at /"

requirements-completed: []

# Metrics
duration: 4min
completed: 2026-03-06
---

# Phase 10 Plan 01: CLI Subcommands and `rtx serve` Summary

**Multi-subcommand CLI with `rtx serve` starting the REST API on a configurable port and serving React frontend from web/dist via http.FileServer**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-06T09:41:14Z
- **Completed:** 2026-03-06T09:45:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Refactored `main.go` to dispatch `run` and `serve` subcommands with per-subcommand FlagSet
- Implemented `cmdServe()` in `cmd/rtx/serve.go` with `-port` flag (default 8080)
- API mounted under `/api/` using `srv.Routes()` (CORS-wrapped); static frontend served at `/` from `web/dist`
- Created `web/dist/index.html` placeholder; manually verified `/api/processes` returns `{"data":[]}` and `/` returns HTML

## Task Commits

Each task was committed atomically:

1. **Task 1: Refactor main.go for subcommands** - `bf42915` (feat)
2. **Task 2: Implement rtx serve with static file serving** - `e95f5ad` (feat)

## Files Created/Modified

- `cmd/rtx/main.go` - Refactored with cmdRun(args) + serve dispatch, updated usage message
- `cmd/rtx/serve.go` - cmdServe(): -port flag, scheduler.New(), api.NewServer(), /api/ + / mux
- `web/dist/index.html` - Placeholder frontend HTML for static file serving verification

## Decisions Made

- `http.FileServer(http.Dir("web/dist"))` for static serving — resolves relative to process working directory at startup; sufficient for v1.1 development workflow
- Top-level mux: `/api/` routes to `srv.Routes()` (CORS-wrapped via corsMiddleware), `/` to FileServer — single mux with two handlers
- Each subcommand owns a `flag.NewFlagSet` for clean flag isolation and per-subcommand usage output

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected scheduler constructor name**
- **Found during:** Task 2 (implement rtx serve)
- **Issue:** Plan implied `scheduler.NewScheduler()` but actual constructor is `scheduler.New()` — build failed
- **Fix:** Changed `scheduler.NewScheduler()` to `scheduler.New()` in serve.go
- **Files modified:** `cmd/rtx/serve.go`
- **Verification:** `go build -o /tmp/rtx ./cmd/rtx` exits 0
- **Committed in:** e95f5ad (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — wrong constructor name)
**Impact on plan:** Fix was necessary for compilation. No scope creep.

## Issues Encountered

- Port 9000 was in use during manual verification — tested on 9099 instead. API and static serving both confirmed working.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `rtx serve` fully functional: starts REST API + static frontend serving from web/dist
- Phase 11 (React frontend) can now build to `web/dist` and be served immediately by `rtx serve`
- Graceful shutdown (SIGINT/SIGTERM handling with context cancellation) is the remaining Phase 10 work

---
*Phase: 10-cli-serve-and-graceful-shutdown*
*Completed: 2026-03-06*
