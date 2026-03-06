---
phase: 11-react-frontend
plan: "01"
subsystem: ui
tags: [react, typescript, vite, polling, dashboard]

requires:
  - phase: 10-cli-serve-and-graceful-shutdown
    provides: rtx serve HTTP server with /api/processes endpoints and static file serving from web/dist

provides:
  - React 19 + Vite 7 + TypeScript dashboard at web/
  - API client (client.ts) wrapping all 8 REST endpoints with envelope unwrapping
  - ProcessList with 2-second polling showing live process state
  - ProcessForm for creating/editing processes via POST/PUT
  - LogViewer with 2-second polling for live log output
  - StatusBadge component with color-coded state indicators
  - usePolling hook for consistent interval-based data fetching

affects:
  - Any future frontend feature work (v2 WebSocket, SSE)

tech-stack:
  added:
    - react@19.2.0
    - react-dom@19.2.0
    - vite@7.3.1
    - typescript@5.9.3
    - "@vitejs/plugin-react@5.1.1"
  patterns:
    - Vite dev proxy for /api to localhost:8080 (avoids CORS in dev)
    - APIEnvelope unwrapping in client.ts (all API responses wrapped in {data, error})
    - usePolling hook pattern: ref-based callback to avoid stale closure, immediate first call
    - snake_case TypeScript types matching Go JSON serialization exactly

key-files:
  created:
    - web/src/api/types.ts
    - web/src/api/client.ts
    - web/src/hooks/usePolling.ts
    - web/src/components/StatusBadge.tsx
    - web/src/components/ProcessList.tsx
    - web/src/components/ProcessForm.tsx
    - web/src/components/LogViewer.tsx
    - web/src/components/Dashboard.tsx
    - web/vite.config.ts
    - web/package.json
    - web/src/index.css
  modified:
    - web/src/App.tsx
    - web/src/main.tsx
    - .gitignore

key-decisions:
  - "Snake_case TypeScript types: ProcessJSON uses snake_case fields (work_dir, restart_policy, depends_on) to match Go JSON serialization exactly -- no mapping layer needed"
  - "APIEnvelope unwrapping in client.ts: all API responses are {data, error} envelopes; unwrapping at client layer keeps components clean"
  - "Vite proxy for /api: dev server proxies /api to localhost:8080; production uses http.FileServer from web/dist served by rtx serve"
  - "ProcessList owns polling: list self-fetches every 2s via usePolling, no prop drilling of data; parent only controls selection"
  - "Dashboard as separate component: App.tsx is a thin wrapper importing Dashboard, keeping route entry point clean"
  - "web/dist excluded from git: build artifacts excluded via .gitignore; web/dist/index.html was previously tracked and removed from index"

patterns-established:
  - "usePolling pattern: savedCallback ref avoids stale closure; immediate first call then interval; dependency array is intervalMs only"
  - "API client pattern: all functions return unwrapped data (not envelope); throw Error with envelope.error on failure"

requirements-completed: []

duration: 11min
completed: 2026-03-06
---

# Phase 11 Plan 01: React Scaffolding and Core Dashboard Summary

**React 19 + Vite 7 dashboard with 2-second polling ProcessList, ProcessForm, and LogViewer wired to the Runtime-X REST API**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-03-06T13:52:05Z
- **Completed:** 2026-03-06T14:03:24Z
- **Tasks:** 4 (+ 1 chore commit for dist cleanup)
- **Files created/modified:** 15

## Accomplishments

- Scaffolded React 19 + Vite 7 + TypeScript application in `web/` with API proxy to localhost:8080
- Implemented API client with full envelope unwrapping for all 8 REST endpoints
- Built ProcessList with 2-second polling (self-managing, no prop drilling) and inline Start/Stop/Delete actions
- Built ProcessForm with all fields (name, command, args, work_dir, depends_on, restart policy with delay_secs)
- Built LogViewer with 2-second polling for live log output with stderr color coding
- `npm run build` produces `dist/` successfully (36 modules, 203KB gzipped 63KB)

## Task Commits

1. **Task 1: Scaffold React Application** - `0abf0b5` (feat)
2. **Task 2: API Client and Types** - `898e271` (feat)
3. **Task 3: Core Components (hooks, StatusBadge, ProcessList, LogViewer, App)** - `5bc4234` (feat)
4. **Task 4: ProcessForm + Dashboard integration** - `34a0514` (feat)
5. **Chore: remove web/dist from git tracking** - `4b26d60` (chore)

## Files Created/Modified

- `web/vite.config.ts` - Vite config with /api proxy and dist outDir
- `web/package.json` - React 19, react-dom 19, TypeScript, Vite 7 dependencies
- `web/src/api/types.ts` - ProcessJSON, RestartPolicyJSON, APIEnvelope, LogEntry, LogsEnvelope interfaces
- `web/src/api/client.ts` - Typed fetch wrapper unwrapping {data, error} envelope for all 8 endpoints
- `web/src/hooks/usePolling.ts` - Ref-based polling hook with immediate first call
- `web/src/components/StatusBadge.tsx` - Color-coded state badges (7 states) with inline styles
- `web/src/components/ProcessList.tsx` - Self-polling process table with Start/Stop/Delete actions
- `web/src/components/ProcessForm.tsx` - Create/edit form with restart policy fields
- `web/src/components/LogViewer.tsx` - Polling log viewer with stderr color coding
- `web/src/components/Dashboard.tsx` - Dashboard composing ProcessList + ProcessForm + LogViewer
- `web/src/App.tsx` - Thin entry point wrapping Dashboard
- `web/src/index.css` - Global styles with light/dark mode support
- `.gitignore` - Added web/node_modules/ and web/dist/ exclusions

## Decisions Made

- **snake_case types**: TypeScript interfaces use snake_case (`work_dir`, `restart_policy`, `delay_secs`) matching Go JSON serialization exactly — no camelCase-to-snake_case mapping layer needed.
- **APIEnvelope unwrapping at client layer**: client.ts unwraps `{data, error}` envelopes; components receive plain typed data, not envelopes.
- **ProcessList self-manages data**: ProcessList owns its own fetch+poll state rather than receiving processes as props — keeps parent (Dashboard) simple.
- **Vite proxy for dev**: `/api` proxied to `localhost:8080` in `vite.config.ts`; production uses `http.FileServer` already configured in `rtx serve`.
- **web/dist gitignored**: Build artifacts excluded; removed previously-tracked `web/dist/index.html` from git index.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Pre-existing component files had wrong types (ProcessSnapshot vs ProcessJSON)**
- **Found during:** Task 2 (API client/types)
- **Issue:** Pre-existing `web/src/` files used `ProcessSnapshot` (with nested `def` object, camelCase, PID field) and `LogLine` types that don't match the actual Go API which returns flat `processJSON` structs with snake_case fields and `{data, error}` envelope.
- **Fix:** Rewrote `types.ts` to match actual API format (`ProcessJSON` with snake_case, `APIEnvelope<T>`, `LogsEnvelope` with `entries` not `lines`). Rewrote all components to use correct types.
- **Files modified:** `web/src/api/types.ts`, `web/src/api/client.ts`, all component files
- **Verification:** `npx tsc -b` exits 0; `npm run build` succeeds
- **Committed in:** `898e271` (Task 2), `5bc4234` (Task 3)

---

**Total deviations:** 1 auto-fixed (Rule 1 - type mismatch bug)
**Impact on plan:** Fix was essential for correctness — components would have failed at runtime against the real API without it.

## Issues Encountered

- `npm create vite@latest web` failed because `web/` directory already existed with `dist/` placeholder. Scaffolded to `tmp/web-scaffold` then copied files over.
- `web/dist/index.html` was tracked in git from a previous placeholder commit. Removed from tracking after adding `web/dist/` to `.gitignore`.

## Next Phase Readiness

- Frontend is ready to run: `cd web && npm run dev` (with rtx serve running on :8080)
- `npm run build` produces `web/dist/` which is served by `rtx serve` at `/`
- Manual verification steps from plan can now be performed:
  1. Start `rtx serve`
  2. Open http://localhost:8080 in browser
  3. Register a process via curl or the form
  4. Verify it appears in the list within 2 seconds
  5. Start/stop and verify state updates auto-refresh

## Self-Check: PASSED

All key files verified present. All task commits verified in git log.

- Files: 10/10 FOUND
- Commits: 5/5 FOUND (0abf0b5, 898e271, 5bc4234, 34a0514, 4b26d60)
- Build: npm run build exits 0, produces dist/ (36 modules, 203KB JS)

---
*Phase: 11-react-frontend*
*Completed: 2026-03-06*
