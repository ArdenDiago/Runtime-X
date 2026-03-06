# Phase 11: React Frontend - Context

## Objective
Deliver a production-ready React 19 application in the `web/` directory that provides full process management capabilities over the Go REST API.

## Phase Constraints
- React 19 + Vite 7 + TypeScript.
- No heavy state management (use `useState` / `useEffect`).
- Zero-dependency styling (Vanilla CSS).
- Polling-only updates (2s interval).
- Must work with `rtx serve` (serving `web/dist`).

## Known Dependencies
- Go REST API (Phase 9) for endpoints.
- `rtx serve` (Phase 10) for static file serving.
- Node.js 20+ for building the frontend.

## Strategic Decisions
- **Styling**: Stick to Vanilla CSS to keep the build simple and fast.
- **API Communication**: Use native `fetch` with a small `client.ts` wrapper.
- **State**: Lift state to `Dashboard.tsx` to share the process list between `ProcessList` and `LogViewer`.
- **Polling**: Implement a robust `useInterval` hook to ensure clean up on unmount.

## Success Criteria
1. Dashboard displays all registered processes.
2. Status updates are visible via polling without refresh.
3. Users can create, start, stop, edit, and delete processes from the UI.
4. Log viewer scrolls to the bottom and auto-refreshes.
5. `npm run build` produces a static site that `rtx serve` correctly delivers.
