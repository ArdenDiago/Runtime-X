# Runtime X (rtx)

A process manager for running, monitoring, and scheduling processes reliably.

## What Is rtx

Runtime X (rtx) is a Go-based process manager focused on correct process lifecycle management — no zombies, no orphans, exact exit codes, and clean signal forwarding.

## Versions

### v1.0 — Single-Process CLI Runner (complete)

A CLI tool that runs a single process and handles it correctly.

```
rtx run <command> [args...]
```

Features:
- Real-time stdout/stderr streaming via direct fd inheritance
- SIGINT/SIGTERM forwarded to the child process
- Exact POSIX exit code returned (128+N for signal-killed processes)

### v1.1 — Multi-Process Scheduler (in progress)

A scheduler that manages multiple named processes with a REST API and web UI.

Planned features:
- Named process definitions with restart policies
- REST API for process management
- Web UI for status and control
- Polling-based status updates

## Build

```
go build ./cmd/rtx
```

## Test

```
go test ./...
```

## Project Structure

```
cmd/rtx/             CLI entry point
internal/process/    v1.0 process runner
```
