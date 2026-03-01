# Stack Research

**Domain:** Multi-process scheduler, Go REST API, React frontend (Runtime X v1.1)
**Researched:** 2026-03-01
**Confidence:** HIGH — all Go recommendations verified against official stdlib docs and pkg.go.dev; React/Vite versions verified against npm/vite.dev release pages

---

## Scope: What This Document Covers

This document covers ONLY the new stack additions for v1.1. The v1.0 stdlib-only stack (`os/exec`, `os/signal`, `syscall`, `os`, `flag`, `fmt`) is validated and unchanged — see v1.0 STACK.md for those decisions.

**What is new for v1.1:**

1. `internal/scheduler/` — multi-process lifecycle, dependency ordering, restart policies
2. `internal/api/` — Go REST API (HTTP server, routing, JSON, CORS)
3. `cmd/rtx/main.go` — add `serve` subcommand
4. `web/` — React frontend (TypeScript, Vite, fetch-based polling)

---

## Recommended Stack

### Go Backend — New Additions

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `net/http` (stdlib) | Go 1.25.5 | HTTP server, routing, middleware | Go 1.22 added method-scoped patterns (`"POST /api/processes/{id}/start"`) and `r.PathValue("id")`. This project is on Go 1.25.5 — all routing features are available and production-ready with zero external dependencies. The API surface is small (8 endpoints); chi or gin add nothing. |
| `encoding/json` (stdlib) | Go 1.25.5 | JSON marshal/unmarshal for API request/response bodies | Go 1.25 ships `encoding/json/v2` as an experimental package (requires `GOEXPERIMENT=jsonv2`) — do NOT use it yet; it is not subject to the Go 1 compatibility promise and may break in 1.26. Standard `encoding/json` is correct for this scale. `json.NewDecoder(r.Body).Decode(&req)` and `json.NewEncoder(w).Encode(resp)` cover all needs. |
| `net/http/httptest` (stdlib) | Go 1.25.5 | Handler unit testing without a real server | `httptest.NewRecorder()` + `httptest.NewRequest()` is the canonical Go pattern for testing HTTP handlers. Every handler in `internal/api/handlers.go` is testable with no running port, no goroutines, no teardown. |
| `sync` (stdlib) | Go 1.25.5 | `sync.RWMutex` protecting the scheduler's process map | The scheduler is accessed from both HTTP handler goroutines and per-process wait goroutines simultaneously. `RLock/RUnlock` for reads (list, status, logs) and `Lock/Unlock` for writes (add, start, stop). This is the only concurrency primitive needed — no channels between scheduler and API layer. |
| `time` (stdlib) | Go 1.25.5 | Exponential backoff delays for restart policies | `time.Sleep(delay)` in a goroutine spawned at process exit. `time.Duration` arithmetic for `initialWait * (1 << restarts)`. No timer libraries needed. |
| `strings` (stdlib) | Go 1.25.5 | Line-splitting in the log ring buffer `Write()` method | `strings.Split(strings.TrimRight(string(p), "\n"), "\n")` converts raw `[]byte` writes from stdout/stderr into discrete log lines for the ring buffer. No third-party parsing needed. |
| `github.com/google/uuid` | v1.6.0 | Generate process IDs on `POST /api/processes` | Already in `go.mod`. `uuid.NewString()` returns a random UUID v4 string on creation. String IDs are more URL-safe and debuggable than auto-increment integers. Do not swap to a different ID library — this is already validated in the project. |

### React Frontend — New Project

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| React | 19.2.x | UI component framework | React 19.2 is the current stable release (latest as of March 2026 is 19.2.4). React 18 LTS is still safe but React 19 is stable and production-ready. No server components or React Router needed — this is a single-page admin UI with no routing beyond one page. |
| TypeScript | 5.9.x | Type-safe component/fetch code | Latest stable as of March 2026 (6.0 is in beta, not stable). TypeScript enforces the API contract between `web/src/api/client.ts` and backend response shapes. Catches mismatched field names before runtime. |
| Vite | 7.x | Build tool and dev server | Vite 7 is the current stable major version (latest stable: 7.3.1 as of March 2026). Use Vite for the dev proxy to the Go API (`proxy: { '/api': 'http://localhost:8080' }`) — this eliminates CORS friction during development without changing the Go server. React `create-react-app` is abandoned; Vite is the 2026 standard. |
| `fetch` (browser built-in) | Browser API | HTTP calls from React to Go API | No library needed. Native `fetch()` is available in all modern browsers. For a polling log viewer hitting `GET /api/processes/{id}/logs` every 2 seconds, a wrapper function in `web/src/api/client.ts` is sufficient. No Axios, no TanStack Query, no SWR needed. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@types/react` | ^19.0.0 | TypeScript types for React 19 | Required alongside React 19. Types are separate from the runtime — must match major version. `npm install --save-exact @types/react@^19.0.0 @types/react-dom@^19.0.0`. |
| `@types/react-dom` | ^19.0.0 | TypeScript types for ReactDOM | Same as above — installed together with `@types/react`. |
| `@vitejs/plugin-react` | ^4.x | Vite plugin for React JSX/Fast Refresh | Required in `vite.config.ts` for React support. Uses Babel transform with React Fast Refresh (HMR). This is the standard plugin; `@vitejs/plugin-react-swc` is faster but SWC adds complexity for no meaningful gain in a small project. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `go test ./internal/scheduler/...` | Unit test scheduler without HTTP | Test `Add/Start/Stop/Status/Logs` with real OS commands (`sleep 5`, `true`, `false`). No mocking needed — these are real processes. |
| `go test ./internal/api/...` | Unit test HTTP handlers | Use `httptest.NewRecorder()` + a real `Scheduler` instance. Handler tests verify JSON encoding, status codes, and CORS headers without a real network. |
| `go build -o bin/rtx ./cmd/rtx` | Build single binary | Same build target as v1.0 — the `serve` subcommand is additive. |
| `npm create vite@latest web -- --template react-ts` | Scaffold React app | Scaffolds `web/` with React 19 + TypeScript + Vite 7. Run once. Sets up `vite.config.ts` with the dev proxy config. |
| `go run -race ./cmd/rtx serve` | Run API server with race detector | `-race` catches data races in the scheduler's concurrent goroutines (wait goroutines + HTTP handler goroutines). Run during development, not in production. |

---

## Installation

```bash
# Go backend — no new go.mod changes required.
# uuid is already in go.mod v1.6.0.
# All new packages (net/http, sync, time, strings, encoding/json) are stdlib.

# Verify existing uuid dependency:
cat go.mod
# Should show: github.com/google/uuid v1.6.0

# Build with the serve subcommand:
go build -o bin/rtx ./cmd/rtx

# React frontend — scaffold once into web/
npm create vite@latest web -- --template react-ts
cd web

# Install runtime dependencies
npm install react@^19.2.0 react-dom@^19.2.0

# Install type definitions
npm install --save-dev @types/react@^19.0.0 @types/react-dom@^19.0.0

# Vite plugin is scaffolded automatically — verify:
cat package.json | grep "@vitejs/plugin-react"
# Should show @vitejs/plugin-react ^4.x

# Dev server (with proxy to Go API on :8080):
npm run dev

# Production build (outputs to web/dist/ — Go API serves this):
npm run build
```

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `net/http` stdlib (Go 1.22+) | `go-chi/chi` v5 | Use chi if you need route groups with shared middleware, subrouters, or have 15+ endpoints that benefit from chi's composable structure. For this project's 8 endpoints on a simple flat API, `net/http`'s `"METHOD /path/{id}"` patterns are sufficient with no external dependency. |
| `net/http` stdlib | `gin-gonic/gin` | Use Gin if performance benchmarks show `encoding/json` reflection as a bottleneck at high request rates. This is a local process manager — it will never approach Gin's performance threshold. Gin's convenience APIs are not worth the 12KB import for this scope. |
| `encoding/json` stdlib | `encoding/json/v2` (experimental) | Use v2 when it becomes stable (planned Go 1.26). Currently requires `GOEXPERIMENT=jsonv2` and is not covered by Go 1 compatibility guarantee. Not production-safe for v1.1. |
| `fetch` built-in | `axios` | Use Axios if you need request interceptors, automatic JSON transform with error normalization, or request cancellation with AbortController polyfills for older browsers. For a local admin UI on modern browsers with 8 endpoints, `fetch` with a thin wrapper is 0KB vs Axios's ~13KB. |
| `fetch` built-in | TanStack Query v5 | Use TanStack Query if the UI has complex caching requirements, optimistic updates, or many independent query invalidations. For a polling log viewer with `setInterval` + `useEffect`, TanStack Query's 16KB overhead adds zero functional value — it solves stale-while-revalidate complexity this project doesn't have. |
| `fetch` built-in | SWR | Same rationale as TanStack Query. SWR (5KB) is lighter but still unnecessary. `useEffect` + `useRef` + `clearInterval` is 15 lines of idiomatic React 19 and teaches the pattern explicitly. |
| React 19 | React 18 LTS | Use React 18 if your organization requires LTS stability or if existing internal tooling is tested only against React 18. Both versions work identically for this use case. React 19's `use()` hook and Actions are unused — this project uses `useState` + `useEffect` + `fetch`. |
| Vite 7 | webpack / CRA | CRA is abandoned (last release 2022). webpack requires manual configuration. Vite 7 scaffolds and runs in under 30 seconds. No case for alternatives here. |
| Manual CORS middleware (stdlib) | `rs/cors` or `jub0bs/cors` | Use `rs/cors` or `jub0bs/cors` if you need per-route CORS policies, credential handling, or preflight caching headers. For a dev tool with `Access-Control-Allow-Origin: *` and `OPTIONS` preflight — 20 lines of stdlib middleware handles it. No dependency needed. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `encoding/json/v2` (Go 1.25 experimental) | Requires `GOEXPERIMENT=jsonv2` at build time and is explicitly not covered by Go 1 compatibility promise. API may change in Go 1.26. Using it in v1.1 creates a build-flag dependency and potential breakage on Go toolchain upgrade. | `encoding/json` — the standard v1 package. Adequate for a local process manager's API traffic. |
| Gorilla/mux | Unmaintained since 2022 (maintenance transferred, activity sparse). New Go 1.22 stdlib routing covers the same feature set. | `net/http` stdlib with Go 1.22+ method+path patterns. |
| `create-react-app` | Abandoned by Meta in 2022. Depends on webpack 4 with outdated defaults, slow HMR, no TypeScript-first setup. | `npm create vite@latest web -- --template react-ts` |
| Redux / Zustand / Jotai | Global state management for a UI with: one list view, one log viewer, one form. React's `useState` per component is the correct scope. Adding a state manager implies shared state across unrelated component trees — this UI has no such case. | `useState` + `useEffect` in each component that needs it. Lift state to `App.tsx` for the process list. |
| WebSocket / SSE for log streaming | PROJECT.md explicitly marks WebSocket and SSE as out-of-scope for v1.1. They add server-side connection management, browser reconnect logic, and CORS complications. | Polling: `setInterval(fetchLogs, 2000)` in `LogViewer.tsx` with `clearInterval` on unmount. |
| `gorm` or any ORM | There is no database. State is in-memory only (PROJECT.md "Out of Scope: State persistence to disk"). An ORM with no database is absurd. | `sync.RWMutex` + `map[string]*ManagedProcess` in `internal/scheduler/scheduler.go`. |
| `cobra` for CLI | Already decided in v1.0 STACK.md. For two subcommands (`run`, `serve`), a `switch os.Args[1]` in `main.go` is sufficient. Cobra adds ~20KB of transitive dependencies and auto-generated help text for a developer tool. | `switch` on `os.Args[1]` with manual help text in `main.go`. |
| `air` or `nodemon` for Go hot reload | Watch-and-rebuild tools are useful but not project dependencies. They are developer preferences, not shared tooling requirements. | `go build -o bin/rtx ./cmd/rtx && ./bin/rtx serve` is fast enough for a small project. Document it in a Makefile target. |

---

## Stack Patterns by Variant

**For the log ring buffer `Write()` method:**

The `logBuffer` struct must implement `io.Writer`. When `cmd.Stdout = logBuffer`, Go writes raw bytes that may contain partial lines or multiple lines per write. The pattern:

```go
func (lb *logBuffer) Write(p []byte) (n int, err error) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    lines := strings.Split(strings.TrimRight(string(p), "\n"), "\n")
    for _, line := range lines {
        if line == "" {
            continue
        }
        if len(lb.lines) >= lb.cap {
            lb.lines = lb.lines[1:]
        }
        lb.lines = append(lb.lines, line)
    }
    return len(p), nil
}
```

No third-party line scanner needed. `bufio.Scanner` is for reading from a stream; here we are writing into a buffer.

**For the CORS middleware (stdlib only):**

```go
// internal/api/middleware.go
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

No `rs/cors` needed. This handles all preflight requests from the React dev server on a different port.

**For the Vite dev proxy (eliminates CORS during development):**

```typescript
// web/vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
```

With this proxy, the React dev server (`localhost:5173`) forwards all `/api/*` requests to the Go API (`localhost:8080`) as same-origin requests. The CORS middleware on the Go side is still needed for production (where the React static files are served by the Go binary directly).

**For the log polling hook in React:**

```typescript
// web/src/components/LogViewer.tsx
function usePollingLogs(processId: string, enabled: boolean) {
  const [lines, setLines] = useState<string[]>([])

  useEffect(() => {
    if (!enabled) return

    const fetchLogs = async () => {
      const res = await fetch(`/api/processes/${processId}/logs`)
      if (res.ok) {
        const data = await res.json()
        setLines(data.lines)
      }
    }

    fetchLogs()  // immediate first fetch
    const id = setInterval(fetchLogs, 2000)
    return () => clearInterval(id)  // cleanup on unmount
  }, [processId, enabled])

  return lines
}
```

No TanStack Query, no SWR. `setInterval` + `clearInterval` in `useEffect` is the correct React idiom. The `return () => clearInterval(id)` cleanup prevents the memory leak on unmount.

---

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| React 19.2.x | @types/react ^19.0.0 | Types must match major. React 19 dropped some deprecated APIs (no legacy `createRef` as function, `useRef` always requires an argument). No impact on this project's patterns. |
| React 19.2.x | @vitejs/plugin-react ^4.x | Vite 7 + plugin-react 4 supports React 19 JSX transform without `import React` boilerplate. |
| Vite 7.x | Node.js 20.19+ or 22.12+ | Vite 7 dropped Node.js 18 support. Use Node 20 LTS or 22 LTS. Node 18 reached EOL October 2023. |
| TypeScript 5.9.x | React 19 | TypeScript 5.x is stable and compatible. TypeScript 6.0 is in beta (March 2026) — do NOT use the beta. |
| `github.com/google/uuid` v1.6.0 | Go 1.25.5 | Already in go.mod. No changes required. |
| `encoding/json` (stdlib) | Go 1.25.5 | Standard v1 JSON — stable, no compatibility concerns. Do not enable `GOEXPERIMENT=jsonv2`. |
| Go 1.22+ `net/http` routing | Go 1.25.5 | `r.PathValue("id")` and method-scoped mux patterns are available in Go 1.25.5. Production-ready since Go 1.22 (February 2024). |

---

## Go Module Impact

The `go.mod` requires no new `require` directives for the Go backend. All new Go packages are stdlib:

- `net/http` — stdlib
- `encoding/json` — stdlib
- `net/http/httptest` — stdlib
- `sync` — stdlib
- `time` — stdlib
- `strings` — stdlib
- `github.com/google/uuid` v1.6.0 — already present

The `web/` directory is NOT a Go module. It has its own `package.json` and `node_modules/`. The Go toolchain ignores it entirely. The Go binary serves `web/dist/` as static files after `npm run build`.

---

## Sources

- `https://pkg.go.dev/net/http` — `ServeMux` method+path routing, `r.PathValue()`, middleware wrapping — HIGH confidence, official Go stdlib
- `https://go.dev/blog/routing-enhancements` — Go 1.22 routing enhancements; method-scoped patterns and wildcards — HIGH confidence, official Go blog
- `https://go.dev/doc/go1.25` — Go 1.25 release notes; `encoding/json/v2` experimental status confirmed, `net/http` CrossOriginProtection added — HIGH confidence, official Go release notes
- `https://go.dev/blog/jsonv2-exp` — json/v2 experimental API announcement; explicitly not under Go 1 compatibility promise — HIGH confidence, official Go blog
- `https://pkg.go.dev/net/http/httptest` — `NewRecorder`, `NewRequest` for handler testing — HIGH confidence, official Go stdlib
- `https://pkg.go.dev/github.com/google/uuid` — `uuid.NewString()` for UUID v4 generation — HIGH confidence, official pkg.go.dev
- `https://vite.dev/releases` — Vite 7.3.1 confirmed as current stable; Vite 8 in beta — HIGH confidence, official Vite release page
- `https://react.dev/versions` — React 19.2 confirmed stable; React 19.2.4 latest — HIGH confidence, official React docs
- `https://devblogs.microsoft.com/typescript/` — TypeScript 5.9 is current stable; 6.0 in beta as of March 2026 — HIGH confidence, official Microsoft TypeScript blog
- `https://www.calhoun.io/go-servemux-vs-chi/` — Go 1.22+ ServeMux vs chi comparison; stdlib sufficient for flat APIs — MEDIUM confidence, community analysis
- `https://www.alexedwards.net/blog/which-go-router-should-i-use` — Router comparison; net/http 1.22+ recommended starting point — MEDIUM confidence, community analysis
- Runtime-X `go.mod` — uuid v1.6.0 already present; no new Go dependencies needed — HIGH confidence, direct file read

---

*Stack research for: Runtime X v1.1 — multi-process scheduler, Go REST API, React frontend*
*Researched: 2026-03-01*
