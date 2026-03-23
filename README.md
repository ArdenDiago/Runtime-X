# Runtime X (rtx)

A full-stack process manager written in Go with a React dashboard for running, monitoring, and scheduling processes reliably.

Runtime X handles the entire process lifecycle with precision — no zombies, no orphans, exact exit codes, and clean signal forwarding. It combines a lightweight CLI for single-process execution with a multi-process scheduler and web-based dashboard for managing complex workloads.

<!-- Add a screenshot of the dashboard here -->
<!-- ![Dashboard Overview](docs/screenshots/dashboard.png) -->

---

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Usage](#usage)
  - [CLI Mode (Single Process)](#cli-mode-single-process)
  - [Server Mode (Multi-Process)](#server-mode-multi-process)
- [Web Dashboard](#web-dashboard)
- [REST API Reference](#rest-api-reference)
- [Project Structure](#project-structure)
- [Running Tests](#running-tests)
- [Docker](#docker)
- [Authors](#authors)
- [License](#license)

---

## Features

- **Signal Forwarding** — `Ctrl+C` (SIGINT) and SIGTERM are forwarded cleanly to child processes, ensuring graceful shutdown every time.
- **Exact Exit Codes** — rtx preserves and returns the exact exit code of the managed process. No information is lost.
- **Zero-Dependency Core** — The single-process runner uses only the Go standard library. No external dependencies.
- **Multi-Process Scheduler** — Register, start, stop, and monitor multiple background processes simultaneously.
- **Dependency Ordering** — Define process dependencies and rtx will start them in the correct order using topological sorting with cycle detection.
- **Restart Policies** — Configure `never`, `on-failure`, or `always` restart modes with exponential backoff and max retry limits.
- **Per-Process Log Buffering** — Each process gets a configurable ring buffer (default 1000 lines) for stdout/stderr output, accessible via the API or dashboard.
- **REST API** — A JSON API for full process lifecycle management, from registration to log retrieval.
- **Web Dashboard** — A React-based UI with real-time polling, process controls, log viewing, and status indicators.
- **Process State Machine** — Seven well-defined states (Idle, Starting, Running, Stopping, Stopped, Failed, Restarting) with deterministic transitions.
- **Graceful Server Shutdown** — The server handles OS signals to cleanly stop all managed processes before exiting.

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                     Runtime X                        │
│                                                      │
│  ┌──────────┐    ┌───────────┐    ┌───────────────┐  │
│  │  CLI     │    │  REST API │    │ React Web UI  │  │
│  │ (rtx run)│    │  Server   │◄───│  Dashboard    │  │
│  └────┬─────┘    └─────┬─────┘    └───────────────┘  │
│       │                │                              │
│       │          ┌─────▼─────┐                        │
│       │          │ Scheduler │                        │
│       │          │           │                        │
│       │          │ ┌───────┐ │                        │
│       │          │ │ Deps  │ │                        │
│       │          │ │ Graph │ │                        │
│       │          │ └───────┘ │                        │
│       │          └─────┬─────┘                        │
│       │                │                              │
│  ┌────▼────────────────▼─────┐                        │
│  │     Process Runner        │                        │
│  │  (Signal Forwarding,      │                        │
│  │   Exit Code Tracking,     │                        │
│  │   Zombie Prevention)      │                        │
│  └───────────────────────────┘                        │
└──────────────────────────────────────────────────────┘
```

---

## Tech Stack

| Layer    | Technology                          |
| -------- | ----------------------------------- |
| Backend  | Go 1.25 (standard library only for core) |
| Frontend | React 19, TypeScript 5.9, Vite 7   |
| API      | REST over HTTP (net/http)           |
| Styling  | CSS with light/dark mode support    |
| Testing  | Go testing + race detector          |
| Container| Docker                              |

---

## Prerequisites

- **Go** 1.25 or later
- **Node.js** 18+ and **npm** (for building the web dashboard)
- **Git**

---

## Getting Started

### 1. Clone the Repository

```bash
git clone https://github.com/ArdenDiago/Runtime-X.git
cd Runtime-X
```

### 2. Build the Frontend

```bash
cd web
npm install
npm run build
cd ..
```

### 3. Build the Binary

```bash
go build -o rtx ./cmd/rtx
```

### 4. Run

```bash
# Single process mode
./rtx run ls -la

# Multi-process server with web dashboard
./rtx serve
```

The dashboard will be available at **http://localhost:8080**.

---

## Usage

### CLI Mode (Single Process)

Use the `run` subcommand to execute a single process with full signal management:

```bash
./rtx run <command> [args...]
```

**How it works:**
1. Finds the command in your system's `PATH`
2. Spawns the process in its own process group
3. Forwards `SIGINT` and `SIGTERM` to the child process
4. Waits for the process to exit and returns its exact exit code
5. Returns `127` if the command is not found

**Examples:**

```bash
# Run a simple command
./rtx run ls -la

# Run a long-lived process (Ctrl+C forwards cleanly)
./rtx run ping 8.8.8.8

# Wrap a development server
./rtx run npm run dev

# Run a Python HTTP server
./rtx run python3 -m http.server 8000
```

### Server Mode (Multi-Process)

Start the scheduler and web dashboard:

```bash
./rtx serve                  # Default: port 8080
./rtx serve -port 9000       # Custom port
```

This starts:
- A **process scheduler** that manages background processes
- A **REST API** at `/api/`
- A **web dashboard** at `/`

<!-- Add a screenshot of the running server terminal output here -->
<!-- ![Server Start](docs/screenshots/server-start.png) -->

---

## Web Dashboard

The web dashboard provides a visual interface for managing all your processes.

<!-- Add a screenshot of the process list here -->
<!-- ![Process List](docs/screenshots/process-list.png) -->

### Dashboard Features

- **Process List** — View all registered processes with real-time status indicators (Running, Stopped, Failed, Idle, Restarting)
- **Create Process** — Click "+ New Process" to register a command with environment variables, working directory, dependencies, and restart policies
- **Lifecycle Controls** — Start, stop, or delete any process with a single click
- **Log Viewer** — Click on a process name to open a real-time log viewer showing stdout and stderr output

<!-- Add a screenshot of the log viewer here -->
<!-- ![Log Viewer](docs/screenshots/log-viewer.png) -->

### Process States

| State       | Description                                         |
| ----------- | --------------------------------------------------- |
| Idle        | Registered but never started                        |
| Starting    | Process is being launched                           |
| Running     | Process is actively executing                       |
| Stopping    | Graceful shutdown in progress                       |
| Stopped     | Process has exited cleanly                          |
| Failed      | Process exited with a non-zero code                 |
| Restarting  | Waiting to restart based on the configured policy   |

---

## REST API Reference

All endpoints use JSON. Responses follow the envelope pattern: `{ "data": T }` on success, `{ "error": "message" }` on failure.

**Base URL:** `http://localhost:8080/api/`

### Register a Process

```bash
curl -X POST http://localhost:8080/api/processes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "web-server",
    "command": "python3",
    "args": ["-m", "http.server", "9001"],
    "env": {"PORT": "9001"},
    "dir": "/tmp",
    "depends_on": [],
    "restart_policy": {
      "mode": "on-failure",
      "max_retries": 5
    }
  }'
```

### List All Processes

```bash
curl http://localhost:8080/api/processes
```

### Get Process Details

```bash
curl http://localhost:8080/api/processes/web-server
```

### Start a Process

```bash
curl -X POST http://localhost:8080/api/processes/web-server/start
```

### Stop a Process

```bash
curl -X POST http://localhost:8080/api/processes/web-server/stop
```

### Delete a Process

```bash
curl -X DELETE http://localhost:8080/api/processes/web-server
```

### View Process Logs

```bash
curl http://localhost:8080/api/processes/web-server/logs
```

### Restart Policies

| Mode         | Behavior                                                            |
| ------------ | ------------------------------------------------------------------- |
| `never`      | Process stays stopped after exit                                    |
| `on-failure` | Restarts only on non-zero exit codes, up to `max_retries`          |
| `always`     | Always restarts after exit, with exponential backoff                |

---

## Project Structure

```
Runtime-X/
├── cmd/rtx/                    # CLI entry point
│   ├── main.go                 # Subcommand routing (run, serve)
│   └── serve.go                # Server startup and graceful shutdown
│
├── internal/
│   ├── process/                # Single-process runner
│   │   ├── runner.go           # Core execution with signal forwarding
│   │   └── runner_test.go      # Unit tests
│   │
│   ├── scheduler/              # Multi-process manager
│   │   ├── types.go            # ProcessDef, ManagedProcess, State FSM
│   │   ├── scheduler.go        # Process registry and lifecycle
│   │   ├── lifecycle.go        # Start, stop, and monitor goroutines
│   │   ├── restart.go          # Exponential backoff restart logic
│   │   ├── deps.go             # Topological sort and cycle detection
│   │   ├── logbuffer.go        # Ring buffer for process output
│   │   └── *_test.go           # Test suite
│   │
│   └── api/                    # REST API
│       ├── server.go           # HTTP server and CORS middleware
│       ├── handlers.go         # Route handlers
│       └── handlers_test.go    # Handler tests
│
├── web/                        # React frontend
│   ├── src/
│   │   ├── main.tsx            # React entry point
│   │   ├── App.tsx             # Root component
│   │   ├── index.css           # Global styles (light/dark mode)
│   │   ├── api/
│   │   │   ├── client.ts       # HTTP client for API calls
│   │   │   └── types.ts        # TypeScript types matching Go structs
│   │   ├── hooks/
│   │   │   └── usePolling.ts   # Polling hook for real-time updates
│   │   └── components/
│   │       ├── Dashboard.tsx   # Main container
│   │       ├── ProcessList.tsx # Process table with controls
│   │       ├── ProcessForm.tsx # Create/edit process form
│   │       ├── LogViewer.tsx   # Real-time log display
│   │       └── StatusBadge.tsx # Color-coded status indicator
│   ├── package.json
│   └── vite.config.ts
│
├── Dockerfile
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## Running Tests

```bash
# Run all tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run tests for a specific package
go test ./internal/scheduler/...
go test ./internal/process/...
go test ./internal/api/...
```

---

## Docker

The Dockerfile uses a multi-stage build — Node.js builds the frontend, Go compiles the binary, and a minimal Debian image runs it all. No local toolchain needed.

```bash
# Build the image
docker build -t rtx .

# Run the server (default) — dashboard at http://localhost:8080
docker run -p 8080:8080 rtx

# Run on a custom port
docker run -p 9000:9000 rtx serve -port 9000

# Use CLI mode to run a single command
docker run rtx run ls -la
docker run rtx run echo "hello from rtx"
```

---

## Authors

- **Arden Diago** — [@ArdenDiago](https://github.com/ArdenDiago)
- **Sooryananda** — [@sooryananda](https://github.com/sooryananda)

---

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

Copyright (c) 2026 Arden Diago & Sooryananda
