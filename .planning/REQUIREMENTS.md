# Requirements: Runtime X (rtx) v1.1

**Defined:** 2026-03-01
**Core Value:** Correct, deterministic process lifecycle management — no zombies, no orphans, exact exit codes, clean signal forwarding.

## v1.1 Requirements

Requirements for v1.1 Full-Stack Process Manager. Each maps to roadmap phases.

### Codebase Cleanup

- [x] **CLN-01**: Legacy Docker orchestration code (cmd/api/, internal/api/, internal/worker/) is removed
- [x] **CLN-02**: Project directory structure is reorganized for scheduler + API + frontend architecture
- [x] **CLN-03**: `go build ./...` succeeds with no build errors after cleanup

### Scheduler Core

- [x] **SCH-01**: User can register a process definition (name, command, args, restart policy) with the scheduler
- [x] **SCH-02**: User can start a registered process — scheduler spawns it and tracks its PID and status
- [x] **SCH-03**: User can stop a running process — scheduler sends SIGTERM and waits for exit
- [x] **SCH-04**: User can list all registered processes with their current status (stopped/running/restarting/failed)
- [x] **SCH-05**: Each process's stdout and stderr are captured in a per-process ring buffer (not direct fd to parent)
- [x] **SCH-06**: User can retrieve recent log lines from a process's ring buffer

### Dependency Ordering

- [ ] **DEP-01**: User can specify that process B depends on process A (B starts only after A is running)
- [ ] **DEP-02**: Scheduler starts processes in topological order respecting all dependency edges
- [ ] **DEP-03**: Circular dependencies are detected and rejected at registration time with a clear error

### Restart Policies

- [ ] **RST-01**: User can configure a process with restart-on-failure policy (restart when exit code != 0)
- [ ] **RST-02**: Restart uses exponential backoff (initial delay, max delay, max retries configurable per process)
- [ ] **RST-03**: Restart attempts stop after reaching max retries — process status becomes "failed"
- [ ] **RST-04**: User can stop a process during a backoff wait period (cancels pending restart)

### REST API

- [ ] **API-01**: GET /api/processes returns list of all registered processes with status
- [ ] **API-02**: POST /api/processes creates a new process definition (name, command, args, restart policy, depends_on)
- [ ] **API-03**: GET /api/processes/:id returns a single process definition and current status
- [ ] **API-04**: PUT /api/processes/:id updates a process definition (only when stopped)
- [ ] **API-05**: DELETE /api/processes/:id removes a process definition (only when stopped)
- [ ] **API-06**: POST /api/processes/:id/start starts a registered process
- [ ] **API-07**: POST /api/processes/:id/stop stops a running process
- [ ] **API-08**: GET /api/processes/:id/logs returns recent log lines from the process ring buffer
- [ ] **API-09**: API server includes CORS middleware for cross-origin requests from React frontend

### CLI

- [ ] **CMD-01**: `rtx serve` starts the API server and serves the React frontend
- [ ] **CMD-02**: `rtx serve` handles SIGTERM/SIGINT gracefully — stops all managed processes before shutting down
- [ ] **CMD-03**: `rtx run` continues to work as v1.0 single-process runner (backwards compatible)

### React Frontend

- [ ] **UI-01**: User sees a dashboard listing all processes with name, status, and action buttons
- [ ] **UI-02**: User can create a new process definition via a form (name, command, args, restart policy, dependencies)
- [ ] **UI-03**: User can start and stop processes via buttons in the UI
- [ ] **UI-04**: User can view a process's recent logs in a polling-based log viewer (auto-refreshes)
- [ ] **UI-05**: User can edit a stopped process's definition
- [ ] **UI-06**: User can delete a stopped process
- [ ] **UI-07**: Process status indicators update via polling (2-second interval)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Real-Time Streaming

- **STREAM-01**: WebSocket or SSE-based real-time log streaming
- **STREAM-02**: Live process output without polling delay

### Authentication

- **AUTH-01**: User authentication for API access
- **AUTH-02**: Role-based access control for process management

### Persistence

- **PERSIST-01**: Process definitions persist across server restarts
- **PERSIST-02**: Log history persists to disk

### Advanced Scheduling

- **ADV-01**: Health checks for process readiness (HTTP probe, TCP check)
- **ADV-02**: Process groups with shared lifecycle
- **ADV-03**: Resource limits (CPU, memory) per process

## Out of Scope

| Feature | Reason |
|---------|--------|
| WebSocket/SSE log streaming | Polling is sufficient for v1.1; real-time streaming is v2 |
| Config files / YAML | Process definitions come from API, not files |
| State persistence to disk | In-memory only; restart server = processes re-registered via API |
| User authentication | Single-user for v1.1 |
| Process metrics / dashboards | Basic status only; Prometheus/OpenTelemetry is v2+ |
| Container isolation / cgroups | Docker handles this; rtx is user-space |
| Daemon mode | `rtx serve` runs in foreground; use systemd for background |
| Interactive stdin forwarding | Multi-process runner doesn't support interactive input |
| Log offset tracking / pagination | Ring buffer with recent lines is sufficient for v1.1 |
| Batch start/stop operations | Individual process control only |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CLN-01 | Phase 4 | Complete |
| CLN-02 | Phase 4 | Complete |
| CLN-03 | Phase 4 | Complete |
| SCH-01 | Phase 5 | Complete |
| SCH-05 | Phase 5 | Complete |
| SCH-06 | Phase 5 | Complete |
| SCH-02 | Phase 6 | Complete |
| SCH-03 | Phase 6 | Complete |
| SCH-04 | Phase 6 | Complete |
| DEP-01 | Phase 7 | Pending |
| DEP-02 | Phase 7 | Pending |
| DEP-03 | Phase 7 | Pending |
| RST-01 | Phase 8 | Pending |
| RST-02 | Phase 8 | Pending |
| RST-03 | Phase 8 | Pending |
| RST-04 | Phase 8 | Pending |
| API-01 | Phase 9 | Pending |
| API-02 | Phase 9 | Pending |
| API-03 | Phase 9 | Pending |
| API-04 | Phase 9 | Pending |
| API-05 | Phase 9 | Pending |
| API-06 | Phase 9 | Pending |
| API-07 | Phase 9 | Pending |
| API-08 | Phase 9 | Pending |
| API-09 | Phase 9 | Pending |
| CMD-01 | Phase 10 | Pending |
| CMD-02 | Phase 10 | Pending |
| CMD-03 | Phase 10 | Pending |
| UI-01 | Phase 11 | Pending |
| UI-02 | Phase 11 | Pending |
| UI-03 | Phase 11 | Pending |
| UI-04 | Phase 11 | Pending |
| UI-05 | Phase 11 | Pending |
| UI-06 | Phase 11 | Pending |
| UI-07 | Phase 11 | Pending |

**Coverage:**
- v1.1 requirements: 35 total
- Mapped to phases: 35
- Unmapped: 0

---
*Requirements defined: 2026-03-01*
*Last updated: 2026-03-01 after Phase 4 completion (CLN-01, CLN-02, CLN-03 complete)*
