# Phase 11: React Frontend - Research

**Researched:** 2026-03-05
**Domain:** React 19, Vite 7, TypeScript, API Polling, UI Design
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| UI-01 | User sees a dashboard listing all processes with name, status, and action buttons | React component for `ProcessList`; use `useState` for the process array; poll `GET /api/processes` every 2s |
| UI-02 | User can create a new process definition via a form | `ProcessForm` component; `POST /api/processes` on submit; handle validation for slug names |
| UI-03 | User can start and stop processes via buttons in the UI | Action buttons in `ProcessList`; `POST /api/processes/:id/start` and `/stop` |
| UI-04 | User can view a process's recent logs in a polling-based log viewer (auto-refreshes) | `LogViewer` component; poll `GET /api/processes/:id/logs` every 2s when process is selected |
| UI-05 | User can edit a stopped process's definition | `ProcessForm` used for editing; `PUT /api/processes/:id` (requires API implementation in Phase 9) |
| UI-06 | User can delete a stopped process | Delete button with confirmation; `DELETE /api/processes/:id` |
| UI-07 | Process status indicators update via polling (2-second interval) | Part of the global `usePollingProcesses` hook |
</phase_requirements>

---

## Summary

Phase 11 delivers the visual interface for Runtime X. Following the "Vanilla CSS first" and "No extra libraries" mandates, we will build a clean, functional SPA using React 19 and Vite 7. The UI will focus on real-time visibility (via 2s polling) and direct control of the process lifecycle.

The frontend will be scaffolded into the `web/` directory. During development, Vite's proxy will handle requests to the Go API. In production, the Go server (from Phase 10) will serve the `web/dist` folder.

**Primary recommendation:** Use a single-page layout with a sidebar or list for process navigation and a detail view for logs and status. Implement a custom hook `useInterval` for robust polling. Use standard `fetch` with a small wrapper for API calls.

---

## Standard Stack

### Core

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| React | 19.2.x | UI Library | Modern, fast, and project standard |
| TypeScript | 5.9.x | Type safety | Catches API contract mismatches |
| Vite | 7.x | Build Tool | Fast HMR and simple proxy setup |
| Vanilla CSS | - | Styling | Maximum flexibility, no build-time overhead |

### Supporting

| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| `fetch` | Built-in | API calls | Zero-dependency, modern standard |
| `setInterval` | Built-in | Polling | Simple and effective for 2s updates |

---

## Architecture Patterns

### Recommended File Structure

```
web/
├── src/
│   ├── api/
│   │   └── client.ts        # Fetch wrappers and types
│   ├── components/
│   │   ├── Dashboard.tsx    # Main layout
│   │   ├── ProcessList.tsx  # Sidebar/List of processes
│   │   ├── ProcessForm.tsx  # Create/Edit form
│   │   ├── LogViewer.tsx    # Polling log viewer
│   │   └── StatusBadge.tsx  # Visual indicator
│   ├── hooks/
│   │   └── usePolling.ts    # Custom interval hook
│   ├── App.tsx              # Root component
│   └── index.css            # Global Vanilla CSS
├── vite.config.ts           # Proxy configuration
└── package.json
```

### Pattern 1: Robust Polling with `useEffect`

**What:** Use `setInterval` inside `useEffect` with a proper cleanup function to avoid memory leaks and stale closures.

**Example:**
```typescript
function useInterval(callback: () => void, delay: number | null) {
  const savedCallback = useRef(callback);

  useEffect(() => {
    savedCallback.current = callback;
  }, [callback]);

  useEffect(() => {
    if (delay !== null) {
      const id = setInterval(() => savedCallback.current(), delay);
      return () => clearInterval(id);
    }
  }, [delay]);
}
```

### Pattern 2: API Client Types

**What:** Define TypeScript interfaces that match the Go structs in `internal/scheduler/types.go` to ensure end-to-end type safety.

**Example:**
```typescript
export interface Process {
  id: string;
  name: string;
  state: string; // "idle", "running", "stopped", etc.
  pid: number;
  restarts: number;
}
```

---

## Common Pitfalls

### Pitfall 1: Stale Polling

**What goes wrong:** Polling continues for a process that was deleted or after the component unmounts.

**How to avoid:** Always return a cleanup function (`clearInterval`) from `useEffect`. Ensure the polling hook depends on the `processId`.

### Pitfall 2: CORS Issues

**What goes wrong:** The browser blocks requests from `localhost:5173` (Vite) to `localhost:8080` (Go).

**How to avoid:** Use Vite's `server.proxy` configuration to make API calls appear as same-origin during development. Ensure the Go server has a basic CORS middleware for production/external access.

---

## Sources

### Primary (HIGH confidence)
- React 19 documentation
- Vite 7 documentation
- `STACK.md` and `FEATURES.md` research

### Secondary (MEDIUM confidence)
- [Polling in React - Patterns and Pitfalls](https://overreacted.io/making-setinterval-declarative-with-react-hooks/)
