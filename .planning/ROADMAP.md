# Roadmap: Runtime X (rtx)

## Milestones

- ✅ **v1.0 MVP** — Phases 1-3 (shipped 2026-02-28)
- 🚧 **v1.1 Full-Stack Process Manager** — Phases 4-11 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-3) — SHIPPED 2026-02-28</summary>

- [x] **Phase 1: Process Foundation** (2/2 plans) — completed 2026-02-28
- [x] **Phase 2: Signal Forwarding** (2/2 plans) — completed 2026-02-28
- [x] **Phase 3: Tests and Validation** (2/2 plans) — completed 2026-02-28

</details>

### 🚧 v1.1 Full-Stack Process Manager (In Progress)

**Milestone Goal:** Transform rtx from a single-process CLI runner into a multi-process manager with a web UI for full browser-based process management.

- [x] **Phase 4: Codebase Cleanup** (1/1 plans) — Remove legacy Docker/API code and restore a clean, compiling baseline — completed 2026-03-01
- [ ] **Phase 5: Scheduler Data Structures and Log Buffer** — Define core types and mutex-safe ring buffer that all later scheduler logic depends on
- [ ] **Phase 6: Scheduler Start, Stop, and Lifecycle** — Implement the core process lifecycle methods on a race-free foundation
- [ ] **Phase 7: Dependency Ordering** — Add topological start ordering with cycle detection as the primary v1.1 differentiator
- [ ] **Phase 8: Restart Policies** — Add exponential backoff restart with cancellable goroutines as the second v1.1 differentiator
- [ ] **Phase 9: REST API** — Expose the full scheduler as a thin HTTP adapter with CORS support
- [ ] **Phase 10: CLI serve and Graceful Shutdown** — Wire everything into `rtx serve` with clean signal handling
- [ ] **Phase 11: React Frontend** — Deliver the browser-based process management UI

## Phase Details

### Phase 4: Codebase Cleanup
**Goal**: The project builds cleanly with only the v1.0 CLI layer — no legacy Docker or API packages remain
**Depends on**: Nothing (first phase of v1.1)
**Requirements**: CLN-01, CLN-02, CLN-03
**Success Criteria** (what must be TRUE):
  1. `go build ./...` exits 0 with no errors or warnings
  2. `go test ./...` exits 0 — existing v1.0 runner tests still pass
  3. No files remain under `cmd/api/`, `internal/api/`, or `internal/worker/`
  4. Project directory structure reflects the scheduler + API + frontend layout for v1.1
**Plans:** 1/1 plans complete
Plans:
- [x] 04-01-PLAN.md — Remove legacy code, update config, verify clean build

### Phase 5: Scheduler Data Structures and Log Buffer
**Goal**: The `ManagedProcess`, `ProcessDef`, `State`, and `logBuffer` types exist with a mutex-safe ring buffer that can be written from goroutines and read from HTTP handlers concurrently without races
**Depends on**: Phase 4
**Requirements**: SCH-01, SCH-05, SCH-06
**Success Criteria** (what must be TRUE):
  1. User can register a process definition (name, command, args, restart policy) with the scheduler and it is stored
  2. A process's log buffer captures output and evicts oldest lines when the ring is full
  3. `go test -race ./internal/scheduler/...` passes with concurrent log writes and reads — no data races detected
  4. Retrieved log lines from the ring buffer reflect the most recent output, not overwritten history
**Plans:** 1/2 plans executed
Plans:
- [ ] 05-01-PLAN.md — TDD: Log buffer ring buffer (mutex-safe, evict-oldest, concurrent read/write)
- [ ] 05-02-PLAN.md — TDD: Scheduler types and registration (ProcessDef, State FSM, Register/Remove/Get/List/Logs)

### Phase 6: Scheduler Start, Stop, and Lifecycle
**Goal**: Users can start and stop registered processes — the scheduler tracks PID and status transitions correctly with zombie prevention and race-free state management
**Depends on**: Phase 5
**Requirements**: SCH-02, SCH-03, SCH-04
**Success Criteria** (what must be TRUE):
  1. User can start a registered process — its PID is tracked and status transitions to running
  2. User can stop a running process — SIGTERM is sent, exit is waited, status transitions to stopped
  3. User can list all processes and see each one's current status (stopped / running / failed)
  4. Stopping an already-stopped process returns a clear error, not a panic or hang
  5. `go test -race ./internal/scheduler/...` passes — no data races in start/stop concurrent paths
**Plans**: TBD

### Phase 7: Dependency Ordering
**Goal**: Processes start in topological order — a process waits for its dependencies to be running before it starts, and circular dependencies are rejected at registration time
**Depends on**: Phase 6
**Requirements**: DEP-01, DEP-02, DEP-03
**Success Criteria** (what must be TRUE):
  1. User can register process B with a dependency on process A — starting B starts A first
  2. A diamond dependency (B and C both depend on A, D depends on B and C) starts all processes in a valid order without starting A twice
  3. Registering a circular dependency (A → B → A) returns an error immediately and the definition is rejected
  4. A missing dependency reference (process B depends on nonexistent process A) returns a clear error
**Plans**: TBD

### Phase 8: Restart Policies
**Goal**: Processes with a restart policy automatically restart after exit according to their configured mode and exponential backoff — and pending restarts can be cancelled by an explicit stop
**Depends on**: Phase 7
**Requirements**: RST-01, RST-02, RST-03, RST-04
**Success Criteria** (what must be TRUE):
  1. A process configured with on-failure restart automatically restarts after a non-zero exit code
  2. A process configured with never restart does not restart after any exit
  3. Restart delays grow exponentially (e.g., 1s, 2s, 4s, 8s) and are capped at the configured max delay
  4. After reaching max retries the process status becomes failed and no further restart is attempted
  5. Calling stop on a process that is waiting in a backoff delay cancels the pending restart immediately
**Plans**: TBD

### Phase 9: REST API
**Goal**: All process management operations are reachable over HTTP — the scheduler is fully accessible to external clients including the React frontend, with correct HTTP semantics and CORS support
**Depends on**: Phase 8
**Requirements**: API-01, API-02, API-03, API-04, API-05, API-06, API-07, API-08, API-09
**Success Criteria** (what must be TRUE):
  1. `GET /api/processes` returns the full process list with current status for each
  2. `POST /api/processes` creates a process definition; `GET /api/processes/:id` returns it; `DELETE /api/processes/:id` removes a stopped process
  3. `PUT /api/processes/:id` updates a stopped process definition and returns 409 if the process is running
  4. `POST /api/processes/:id/start` and `/stop` trigger lifecycle transitions and return 202 Accepted
  5. `GET /api/processes/:id/logs` returns recent log lines from the process ring buffer
  6. A cross-origin `OPTIONS` preflight from `http://localhost:5173` receives the correct CORS headers and 204 response
**Plans**: TBD

### Phase 10: CLI serve and Graceful Shutdown
**Goal**: `rtx serve` starts the API server and serves the React frontend — Ctrl+C and SIGTERM stop all managed processes before the server exits, leaving no orphaned children
**Depends on**: Phase 9
**Requirements**: CMD-01, CMD-02, CMD-03
**Success Criteria** (what must be TRUE):
  1. `rtx serve` starts the HTTP server and all API endpoints respond correctly via curl
  2. `rtx run <command>` still works as the v1.0 single-process runner (backwards compatible)
  3. Pressing Ctrl+C causes the server to stop all managed processes and exit within 10 seconds
  4. After Ctrl+C, `ps aux` shows no orphaned child processes that were managed by rtx
**Plans**: TBD

### Phase 11: React Frontend
**Goal**: Users can manage all their processes entirely from a browser — create, start, stop, monitor status, view logs, edit definitions, and delete processes without touching the CLI
**Depends on**: Phase 10
**Requirements**: UI-01, UI-02, UI-03, UI-04, UI-05, UI-06, UI-07
**Success Criteria** (what must be TRUE):
  1. User sees a dashboard listing all processes with name, current status badge, and start/stop action buttons
  2. User can create a new process via a form (name, command, args, restart policy, dependencies) and it appears in the list
  3. User can start and stop processes via buttons — status updates automatically every 2 seconds without a page reload
  4. User can open a log viewer for any process and see recent output that refreshes every 2 seconds while the process is running
  5. User can edit a stopped process's definition and delete a stopped process from the UI
  6. `./bin/rtx serve` (without Vite running) serves both the React app and the API at the same origin
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Process Foundation | v1.0 | 2/2 | Complete | 2026-02-28 |
| 2. Signal Forwarding | v1.0 | 2/2 | Complete | 2026-02-28 |
| 3. Tests and Validation | v1.0 | 2/2 | Complete | 2026-02-28 |
| 4. Codebase Cleanup | v1.1 | Complete    | 2026-03-01 | 2026-03-01 |
| 5. Scheduler Data Structures and Log Buffer | 1/2 | In Progress|  | - |
| 6. Scheduler Start, Stop, and Lifecycle | v1.1 | 0/TBD | Not started | - |
| 7. Dependency Ordering | v1.1 | 0/TBD | Not started | - |
| 8. Restart Policies | v1.1 | 0/TBD | Not started | - |
| 9. REST API | v1.1 | 0/TBD | Not started | - |
| 10. CLI serve and Graceful Shutdown | v1.1 | 0/TBD | Not started | - |
| 11. React Frontend | v1.1 | 0/TBD | Not started | - |
