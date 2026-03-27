# Runtime-X (rtx) - Project Context

This document provides a high-level overview of the Runtime-X project. It is designed to act as a primary context file for AI agents or new developers onboarding onto the codebase.

## Overview
Runtime-X is a full-stack process manager written in Go. It handles the entire process lifecycle (preventing zombie processes, capturing exact exit codes, and cleanly forwarding signals). It features both a CLI for executing single processes and a long-running Server mode featuring a multi-process scheduler and a React-based web dashboard.

## Core Architecture

### Components Layer
1. **CLI / Entrypoint (`cmd/rtx`)**
   - Commands routing between single-process CLI (`run`) and multi-process server (`serve`).
2. **Process Runner (`internal/process`)**
   - Responsible for launching the actual OS-level process, capturing stdout/stderr into ring buffers, tracking exit codes, and handling OS signals.
3. **Scheduler (`internal/scheduler`)**
   - Manages multiple concurrent processes logic. 
   - Uses Topological sorting (`deps.go`) for verifying `depends_on` relationships.
   - Houses the deterministic State Machine (FSM) dictating valid process states (e.g. `Idle`, `Starting`, `Running`, etc).
4. **Authentication & API (`internal/auth`, `internal/api`)**
   - The Go standard library (`net/http`) serves the JSON REST API.
   - Contains handlers (`handlers.go`, `auth_handlers.go`), server configuration (`server.go`), and route protection logic (`middleware.go`, `auth.go`).
5. **Web Dashboard (`web`)**
   - A Vite + React + TypeScript frontend that polls the API for live process statuses, presents logs, and manages scheduled jobs.

## Directory Structure
```text
Runtime-X/
├── cmd/rtx/                    # CLI entry points
├── internal/
│   ├── auth/                   # Authentication & Session logic
│   ├── api/                    # REST HTTP Server, Middlewares, and Handlers
│   ├── process/                # Single process runner & os/exec bindings
│   └── scheduler/              # Core Domain: lifecycle, state FSM, Types, DAG Deps Check
├── web/                        # React / Vite Frontend
├── Dockerfile                  # Multi-stage production build
├── ProjectContext.md           # This Context File!
└── NewTask.md                  # Active AI/Developer task directives
```

## State Machine (FSM)
Every `ManagedProcess` inside `internal/scheduler` transitions between carefully guarded states:
- `Idle`: Registered, but never executed.
- `Starting`: In the process of being launched.
- `Running`: Actively executing.
- `Stopping`: Received stop signal, shutting down gracefully.
- `Stopped`: Clean zero exit code or cleanly halted.
- `Failed`: Crashed or exited with non-zero code.
- `Restarting`: Waiting through an exponential backoff before the next attempt.

## Important Types (`internal/scheduler/types.go`)
- `ProcessDef`: Immutable configuration (Command, Args, Env, Dependencies, Restart Policies).
- `ManagedProcess`: The active runtime wrapper, holding the state, exit code, restart count, and Log Buffers.

## Recent Context & Notes
There are currently *3 features* being planned/implemented, documented inside `NewTask.md`:
1. **Dry-Run Registration**: Validing a payload without permanent persistence.
2. **Automated Restart Count**: *(Note: Code analysis confirms this is actually already implemented internally!)*
3. **Environment Variables Validation**: Enforcing standard `KEY=VALUE` formatting on `Env` arrays during `Register()`.
