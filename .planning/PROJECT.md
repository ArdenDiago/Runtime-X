# Runtime X

## What This Is

Runtime X is a full-stack process manager written in Go with a React frontend. The `rtx` CLI spawns and manages multiple processes with dependency-aware start ordering, restart policies with exponential backoff, and real-time log streaming. A Go REST API and React UI provide browser-based process management — create, start/stop, monitor, and view logs. Built on the v1.0 single-process runner foundation.

## Core Value

Correct, deterministic process lifecycle management — no zombies, no orphans, exact exit codes, clean signal forwarding.

## Requirements

### Validated

- ✓ Docker job orchestration system — existing
- ✓ HTTP API for Dockerfile configuration — existing
- ✓ Worker pool with bounded parallelism — existing
- ✓ Job queue with blocking semantics — existing
- ✓ `rtx run <command> [args...]` CLI that spawns real OS processes — v1.0
- ✓ PID tracking and display — v1.0
- ✓ Real-time stdout/stderr streaming (no buffering) — v1.0
- ✓ Exit code capture and propagation to parent — v1.0
- ✓ SIGINT/SIGTERM interception and forwarding to child — v1.0
- ✓ Graceful shutdown (forward signal → wait for child → exit with child's code) — v1.0
- ✓ Zombie prevention (always call cmd.Wait()) — v1.0
- ✓ Graceful error handling (command not found, invalid args, immediate crash) — v1.0
- ✓ Minimal logging: PID, signal received, exit code only — v1.0
- ✓ Unit tests and manual test coverage — v1.0

### Active

- [ ] Codebase refactor: remove old Docker/API code, clean project structure
- [ ] Internal scheduler managing multiple processes with lifecycle tracking
- [ ] Dependency-aware process start ordering (Process B waits for A)
- [ ] Restart policies with exponential backoff (configurable per process)
- [ ] Go REST API for process CRUD, start/stop, status, log retrieval
- [ ] React frontend: full management UI (create/edit/start/stop/monitor/logs)
- [ ] Log viewer with polling-based refresh

### Out of Scope

- Config files / YAML parsing — process definitions come from API, not files
- Daemon mode / background mode — API server handles lifecycle
- State persistence to disk — in-memory process state only
- Containers / Plugins — future versions
- Signal rewriting / remapping — forward as-is
- Structured/JSON logging for process output — plain stderr sufficient
- Shell wrapping (sh -c) — direct exec.Command for correctness
- StdoutPipe/StderrPipe goroutines — race condition risk, use direct fd assignment
- WebSocket/SSE log streaming — polling is sufficient for v1.1
- OAuth / user authentication — single-user for now
- Process metrics / observability dashboards — basic status only

## Current Milestone: v1.1 Full-Stack Process Manager

**Goal:** Transform rtx from a single-process CLI runner into a multi-process manager with a web UI for full browser-based process management.

**Target features:**
- Codebase cleanup (remove legacy Docker orchestration code)
- Multi-process scheduler with dependency ordering
- Restart policies with exponential backoff
- Go REST API backend
- React frontend for full process management
- Polling-based log viewer

## Context

- v1.0 shipped 2026-02-28: single-process runner with signal forwarding, exit codes, zombie prevention
- `rtx` binary entry point at `cmd/rtx/main.go`
- Process execution logic in `internal/process/runner.go` (97 lines)
- Unit tests in `internal/process/runner_test.go` (171 lines)
- Legacy Docker orchestration code in `cmd/api/`, `internal/api/`, `internal/worker/` — to be removed
- Go stdlib only for core runner — API and frontend will add dependencies (net/http router, React)
- Linux first-class, cross-platform compatible
- 1,976 Go LOC total (pre-refactor)

## Constraints

- **Language**: Go (latest stable) — project is already Go
- **Dependencies**: Stdlib for core runner; minimal dependencies for API (standard net/http or chi); React for frontend
- **Platform**: Linux first-class, cross-platform compatible
- **Scope**: Multi-process management with web UI
- **Logging**: Minimal process logging (PID, signals, exit code); API uses standard HTTP logging

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| New `cmd/rtx/` entry point | rtx is a standalone CLI, not part of the API server | ✓ Good |
| New `internal/process/` package | Separation from Docker-specific worker code | ✓ Good |
| No shell wrapping | Direct exec.Command for correctness | ✓ Good |
| Stdlib only | Spec requires no external runtime frameworks | ✓ Good |
| `cmd.Start()` + `doneCh` goroutine pattern | Enables concurrent signal handling + zombie-safe wait | ✓ Good |
| Direct fd assignment (`cmd.Stdout = os.Stdout`) | Avoids pipe goroutine race conditions and buffering | ✓ Good |
| `Setpgid: true` from Phase 1 | Child in own process group for clean signal interposition | ✓ Good |
| Buffered signal channel (capacity 1) | signal.Notify non-blocking send requirement | ✓ Good |
| `cmd.Process.Signal(sig)` not process group | Single-process runner targets child PID directly | ✓ Good |
| Swallow `os.ErrProcessDone` silently | Benign natural-exit race, logging would be noise | ✓ Good |
| Inner-function exit pattern (`os.Exit(run())`) | Ensures deferred cleanup always executes | ✓ Good |
| Re-exec helper pattern for signal/zombie tests | Canonical Go stdlib pattern, no mocking needed | ✓ Good |

---
*Last updated: 2026-02-28 after v1.1 milestone start*
