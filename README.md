# Runtime X (rtx)

A process manager for running, monitoring, and scheduling processes reliably.

## What Is rtx

Runtime X (rtx) is a Go-based process manager focused on correct process lifecycle management — no zombies, no orphans, exact exit codes, and clean signal forwarding.

---

## 1. Direct CLI Commands (Single Process)

Use the `run` subcommand to execute a single process directly through `rtx`. This ensures signals (like `Ctrl+C`) are forwarded correctly and the exit code is preserved.

### Usage
```bash
go run ./cmd/rtx run <command> [args...]
```

### Examples
- **Run a simple command:**
  ```bash
  go run ./cmd/rtx run ls -la
  ```
- **Run a long-running process:**
  ```bash
  go run ./cmd/rtx run ping 8.8.8.8
  ```

---

## 2. What Commands Can I Run?

You can run **any command available in your system's PATH**. `rtx` acts as a smart wrapper that manages the process lifecycle for you.

### System Commands
Run any standard terminal utility:
- `rtx run ls -la` (List files)
- `rtx run pwd` (Print working directory)
- `rtx run date` (Show current time)

### Long-Running Processes
`rtx` is ideal for these because it handles `Ctrl+C` (SIGINT) gracefully:
- `rtx run ping 8.8.8.8`
- `rtx run top`
- `rtx run tail -f /var/log/syslog`

### Development Tools
Wrap your development servers to ensure they shut down cleanly:
- `rtx run npm run dev`
- `rtx run python3 -m http.server 8000`
- `rtx run go run main.go`

### How it Works
When you use `rtx run <name> [args]`:
1. **Finds the command:** It searches your system's folders for `<name>`.
2. **Spawns the process:** It starts the command and logs its PID.
3. **Forwards Signals:** Pressing `Ctrl+C` sends a signal to the child process first, allowing it to shut down cleanly.
4. **Returns Exit Code:** `rtx` returns the exact same exit code as the command (or `127` if the command is not found).

---

## 3. Scheduler & Web UI (Multi-Process)

The `serve` subcommand starts a scheduler that can manage multiple background processes simultaneously, accessible via a REST API and a Web Dashboard.

### Step A: Build the Frontend
Before starting the server, you must compile the React-based web interface:
```bash
cd web
npm install
npm run build
cd ..
```

### Step B: Start the Server
```bash
go run ./cmd/rtx serve
```
*   **Web Dashboard:** [http://localhost:8080](http://localhost:8080)
*   **API Base:** [http://localhost:8080/api/](http://localhost:8080/api/)
*   **Custom Port:** `go run ./cmd/rtx serve -port 9000`

---

## 4. API Commands (REST API)

While the server is running, you can manage processes using `curl` or any HTTP client.

### Register a New Process
```bash
curl -X POST http://localhost:8080/api/processes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "web-server",
    "command": "python3",
    "args": ["-m", "http.server", "9001"],
    "restart_policy": {"mode": "always"}
  }'
```

### Manage Process Lifecycle
- **Start a process:**
  ```bash
  curl -X POST http://localhost:8080/api/processes/web-server/start
  ```
- **Stop a process:**
  ```bash
  curl -X POST http://localhost:8080/api/processes/web-server/stop
  ```
- **List all processes:**
  ```bash
  curl http://localhost:8080/api/processes
  ```

### View Logs
Retrieve the buffered logs for a specific process:
```bash
curl http://localhost:8080/api/processes/web-server/logs
```

---

## 5. Web Dashboard Actions

The Web UI provides a visual way to manage your processes:

1.  **Dashboard:** View the real-time status (Running, Stopped, Failed) of all registered processes.
2.  **Add Process:** Use the "+" button to register a new command with specific environment variables, working directories, and restart policies.
3.  **Lifecycle Controls:** Start, Stop, or Delete processes using the action buttons in the list.
4.  **Log Viewer:** Click on a process name to open a real-time log viewer to see stdout and stderr output.

---

## Build & Test

- **Build Binary:** `go build -o rtx ./cmd/rtx`
- **Run Tests:** `go test ./...`
