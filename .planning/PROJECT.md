# Runtime X

## What This Is

Runtime X is a user-space process runner written in Go. v0 answers one question: can it spawn, monitor, and terminate real OS processes correctly and deterministically? The `rtx` CLI binary executes arbitrary commands as child processes with proper signal forwarding, exit code propagation, and zombie prevention.

## Core Value

Correct, deterministic process lifecycle management — no zombies, no orphans, exact exit codes, clean signal forwarding.

## Requirements

### Validated

- ✓ Docker job orchestration system — existing
- ✓ HTTP API for Dockerfile configuration — existing
- ✓ Worker pool with bounded parallelism — existing
- ✓ Job queue with blocking semantics — existing

### Active

- [ ] `rtx run <command> [args...]` CLI that spawns real OS processes
- [ ] PID tracking and display
- [ ] Real-time stdout/stderr streaming (no buffering)
- [ ] Exit code capture and propagation to parent
- [ ] SIGINT/SIGTERM interception and forwarding to child
- [ ] Graceful shutdown (forward signal → wait for child → exit with child's code)
- [ ] Zombie prevention (always call cmd.Wait())
- [ ] Graceful error handling (command not found, invalid args, immediate crash)
- [ ] Minimal logging: PID, signal received, exit code only
- [ ] Unit tests and manual test coverage

### Out of Scope

- Config files / YAML parsing — v0 philosophy: earn trust through correctness
- Daemon mode / background mode — not needed for process correctness proof
- Restart policies — v1+ concern
- Multiple process support — single process focus for v0
- State persistence — in-memory PID only
- Lifecycle FSM — keep it simple
- Isolation / Networking / HTTP API — Docker system handles this
- Metrics / Observability frameworks — minimal logging only
- Containers / Plugins — future versions

## Context

- Brownfield project: existing Docker orchestration system in `cmd/`, `internal/`
- `rtx` binary is a new entry point at `cmd/rtx/main.go`
- Process execution logic goes in `internal/process/` — new package
- Go stdlib only (os/exec, os/signal, syscall) — no external dependencies
- Linux first-class, cross-platform compatible
- The existing worker/runner pattern in `internal/worker/` is related but separate

## Constraints

- **Language**: Go (latest stable) — project is already Go
- **Dependencies**: Standard library only for rtx — no external frameworks
- **Platform**: Linux first-class, cross-platform compatible
- **Scope**: Single process execution only — no multi-process
- **Logging**: Minimal (PID, signals, exit code) — no structured/JSON logging

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| New `cmd/rtx/` entry point | rtx is a standalone CLI, not part of the API server | — Pending |
| New `internal/process/` package | Separation from Docker-specific worker code | — Pending |
| No shell wrapping | Direct exec.Command for correctness | — Pending |
| Stdlib only | Spec requires no external runtime frameworks | — Pending |

---
*Last updated: 2026-02-27 after initialization*
