# Runtime X

## What This Is

Runtime X is a user-space process runner written in Go. The `rtx` CLI binary executes arbitrary commands as child processes with correct signal forwarding, exact exit code propagation, real-time I/O streaming, and zombie prevention. v1.0 proves deterministic process lifecycle management on Linux.

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

(None — next milestone requirements TBD)

### Out of Scope

- Config files / YAML parsing — v0 philosophy: earn trust through correctness
- Daemon mode / background mode — not needed for process correctness proof
- Restart policies — v1+ concern, requires understanding real failure patterns
- Multiple process support — single process focus for v0 correctness proof
- State persistence — in-memory PID only during the run
- Lifecycle FSM — keep it simple, no state machine needed
- Isolation / Networking / HTTP API — Docker system handles this
- Metrics / Observability frameworks — minimal logging only
- Containers / Plugins — future versions
- Signal rewriting / remapping — forward as-is
- Structured/JSON logging — plain stderr sufficient
- Shell wrapping (sh -c) — direct exec.Command for correctness
- StdoutPipe/StderrPipe goroutines — race condition risk, use direct fd assignment

## Context

- Brownfield project: existing Docker orchestration system in `cmd/`, `internal/`
- `rtx` binary entry point at `cmd/rtx/main.go`
- Process execution logic in `internal/process/runner.go` (97 lines)
- Unit tests in `internal/process/runner_test.go` (171 lines)
- Go stdlib only (os/exec, os/signal, syscall) — zero external dependencies
- Linux first-class, cross-platform compatible
- 1,976 Go LOC total
- v1.0 shipped 2026-02-28

## Constraints

- **Language**: Go (latest stable) — project is already Go
- **Dependencies**: Standard library only for rtx — no external frameworks
- **Platform**: Linux first-class, cross-platform compatible
- **Scope**: Single process execution only — no multi-process
- **Logging**: Minimal (PID, signals, exit code) — no structured/JSON logging

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
*Last updated: 2026-02-28 after v1.0 milestone*
