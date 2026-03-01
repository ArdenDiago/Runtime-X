# Pitfalls Research

**Domain:** Multi-process scheduler, Go REST API, React frontend (Runtime X v1.1)
**Researched:** 2026-03-01
**Confidence:** HIGH — based on direct codebase analysis of existing runner.go, Go stdlib official docs, and verified patterns from process supervisor implementations

> **Scope:** This document covers v1.1 additions only. v1.0 pitfalls (StdoutPipe deadlock, zombie prevention, signal channel buffer size, exit code extraction) are already solved in `internal/process/runner.go` and are not repeated here. Every pitfall below is specific to adding: multi-process scheduler, per-process log capture, Go REST API, React frontend, exponential backoff restart policies.

---

## Critical Pitfalls

### Pitfall 1: Calling process.Run() From the Scheduler — Blocking the Goroutine

**What goes wrong:**
The existing `process.Run(name, args)` in `internal/process/runner.go` is a blocking function. It holds the calling goroutine until the child exits, owns signal handling for the parent process, and returns an `int`. If the scheduler calls `process.Run()` in a goroutine per managed process, it loses all control: it cannot access `cmd.Process` to call `Signal()` for `Stop()`, cannot redirect stdout/stderr to the per-process log buffer, and cannot inspect or update process state while the child is running. Signal handling in `runner.go` also installs `signal.Notify` on the parent — running this in multiple goroutines races on the signal channel and is incorrect.

**Why it happens:**
`process.Run()` exists and handles signals and exit codes correctly. Reusing it seems like the obvious choice for DRY reasons.

**How to avoid:**
The scheduler must reimplement the process lifecycle directly using the patterns from `runner.go` — not the function itself. Specifically:
```go
// In scheduler — do NOT call process.Run()
cmd := exec.Command(def.Command, def.Args...)
cmd.Stdout = p.logBuf          // per-process log capture
cmd.Stderr = p.logBuf
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

if err := cmd.Start(); err != nil { ... }
p.cmd = cmd
p.pid = cmd.Process.Pid

// Non-blocking: goroutine waits; Stop() signals externally
go func() {
    err := cmd.Wait()
    exitCode := resolveExitCode(err, cmd.ProcessState)
    // update state, schedule restart
}()
```
`process.Run()` continues to be called only by `cmd/rtx/main.go` for the `rtx run` subcommand.

**Warning signs:**
- Scheduler has no field for `*exec.Cmd` — it has nowhere to store the handle for `Stop()`
- `Stop()` implementation sends a signal to a PID rather than calling `cmd.Process.Signal()` directly
- Log output from managed processes appears on the terminal instead of being captured

**Phase to address:** Scheduler process start/stop implementation (Step 3 in build order)

---

### Pitfall 2: Direct fd Assignment (cmd.Stdout = os.Stdout) in the Multi-Process Scheduler

**What goes wrong:**
The v1.0 pattern `cmd.Stdout = os.Stdout` / `cmd.Stderr = os.Stderr` is correct for a single transparent process runner. In the multi-process scheduler, it breaks in two ways:

1. **Output from all managed processes interleaves on the terminal** with no identification — there is no way to associate a log line with its source process.
2. **The HTTP log endpoint has nothing to return** — logs that flow directly to `os.Stdout` are not captured anywhere the API can read them.

Additionally, Go's `cmd.Wait()` does not spawn internal copy goroutines when `Stdout`/`Stderr` are `*os.File` — this was an advantage in v1.0. If instead an `io.Writer` (e.g., `io.MultiWriter`) is used without synchronization, `cmd.Wait()` does spawn internal copy goroutines that write to the writer concurrently from two goroutines (stdout copier and stderr copier). If the writer is not goroutine-safe, this is a data race.

**Why it happens:**
The v1.0 pattern is explicitly documented in the codebase comments as correct. Developers copy it into the scheduler without recognizing that the scheduler's requirements are different.

**How to avoid:**
Each `ManagedProcess` owns a `logBuffer` implementing `io.Writer` with its own `sync.Mutex`. Assign it to both `cmd.Stdout` and `cmd.Stderr`:
```go
buf := &logBuffer{cap: 1000}
cmd.Stdout = buf
cmd.Stderr = buf
```
The `logBuffer.Write()` must acquire its mutex before modifying `lines`. When both stdout and stderr point to the same writer, Go's internal copy goroutines (spawned by `cmd.Start()`) write to it concurrently — the mutex makes this safe.

For local debugging, `io.MultiWriter(buf, os.Stdout)` can be used, but `os.Stdout` is itself goroutine-safe (it is an `*os.File`), so this is fine.

**Warning signs:**
- All managed process output appears in the server terminal mixed together
- `GET /api/processes/{id}/logs` returns an empty array
- `-race` detector reports a data race in `logBuffer.Write()`

**Phase to address:** Scheduler process struct and log buffer (Step 2 in build order) — must be solved before any process start logic

---

### Pitfall 3: Shared logBuffer Without Per-Buffer Mutex — Data Race on Every Write

**What goes wrong:**
The `logBuf.lines` slice is written by the goroutine copying from the child process's stdout/stderr pipe, and simultaneously read by HTTP handlers serving `GET /api/processes/{id}/logs`. Without synchronization, this is a textbook data race. Go's slice header (pointer, length, capacity) is not atomically readable — a concurrent append that triggers a reallocation can cause the reader to see a partially-updated pointer. The race detector flags this immediately. In practice it causes corrupted log output or a panic.

**Why it happens:**
The log buffer is typically written by one goroutine and read by another. Developers add a lock to the `Scheduler`'s top-level `mu` for other operations but forget that the log buffer has its own separate access pattern — writes happen from the process's copy goroutine (started by `cmd.Start()`), not from the scheduler's own goroutines.

**How to avoid:**
Give `logBuffer` its own independent `sync.Mutex`. Do not share the scheduler's `mu` for log buffer access — it would force serialization of all log writes with all scheduler operations:
```go
type logBuffer struct {
    mu    sync.Mutex
    lines []string
    cap   int
}

func (lb *logBuffer) Write(p []byte) (n int, err error) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    // split p into lines, append with eviction
    return len(p), nil
}

func (lb *logBuffer) Lines() []string {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    result := make([]string, len(lb.lines))
    copy(result, lb.lines)  // return a copy — caller cannot mutate internal state
    return result
}
```

**Warning signs:**
- `-race ./...` reports a data race mentioning `logBuffer` or its `lines` field
- Log lines appear out of order or truncated
- Occasional panic: `runtime error: slice bounds out of range`

**Phase to address:** Scheduler process struct and log buffer (Step 2 in build order)

---

### Pitfall 4: Growing Log Slice Without Eviction — Unbounded Memory

**What goes wrong:**
`logs = append(logs, newLine)` indefinitely causes memory to grow proportional to the total lifetime output of every managed process. A verbose process running `yes` or a log-heavy server can exhaust available memory. Since log persistence to disk is out of scope for v1.1, there is no alternative drain path.

**Why it happens:**
It feels premature to cap logs when the initial goal is just "capture some output." Developers plan to add a cap "later" and it never happens.

**How to avoid:**
Implement a ring buffer from the start with a configurable cap (1,000 lines is a reasonable default). When at capacity, evict the oldest line before appending:
```go
if len(lb.lines) >= lb.cap {
    lb.lines = lb.lines[1:]  // shift out oldest
}
lb.lines = append(lb.lines, line)
```
Note: `lb.lines[1:]` keeps the backing array allocated. For a simple ring buffer at this scale, this is acceptable. The oldest 1 element is dropped per append; the slice length never exceeds `cap`. No reallocation happens after the first fill.

**Warning signs:**
- Go process RSS memory grows continuously during a long-running managed process
- `runtime.MemStats.HeapAlloc` grows monotonically
- Managed process with verbose output causes the API server to OOM

**Phase to address:** Scheduler process struct and log buffer (Step 2 in build order)

---

### Pitfall 5: Holding the Scheduler Mutex While Calling cmd.Start() — Deadlock Risk

**What goes wrong:**
`cmd.Start()` can block briefly (it forks the OS process, which involves syscalls). More critically, if the lock is held while `cmd.Start()` runs, and the `Wait()` goroutine (spawned inside `cmd.Start()` for pipe copying) tries to acquire a lock on the same mutex to update process state, a deadlock occurs. Even without a deadlock, holding a write lock during `cmd.Start()` serializes all API requests (including reads) for the duration of process startup — which on a loaded system can be measurable.

**Why it happens:**
Developers acquire `s.mu.Lock()` at the top of `Start()` to protect the `processes` map, and keep it locked throughout the function body including the `cmd.Start()` call and goroutine spawn.

**How to avoid:**
Use a two-phase pattern: lock to read/validate process state, unlock before calling `cmd.Start()`, then lock again to write the updated state (pid, running status):
```go
func (s *Scheduler) Start(id string) error {
    s.mu.Lock()
    p, ok := s.processes[id]
    if !ok { s.mu.Unlock(); return ErrNotFound }
    if p.state == StateRunning { s.mu.Unlock(); return ErrAlreadyRunning }
    p.state = StateStarting
    s.mu.Unlock()  // release before syscall

    cmd := exec.Command(p.def.Command, p.def.Args...)
    cmd.Stdout = p.logBuf
    cmd.Stderr = p.logBuf
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    if err := cmd.Start(); err != nil {
        s.mu.Lock(); p.state = StateIdle; s.mu.Unlock()
        return err
    }

    s.mu.Lock()
    p.cmd = cmd
    p.pid = cmd.Process.Pid
    p.state = StateRunning
    s.mu.Unlock()

    go s.waitProcess(p)  // non-blocking wait goroutine
    return nil
}
```

**Warning signs:**
- `go test -race` hangs on scheduler tests
- High API latency when starting multiple processes simultaneously
- Deadlock in test: goroutine blocked on mutex acquisition inside `cmd.Wait()` callback

**Phase to address:** Scheduler start/stop implementation (Step 3 in build order)

---

### Pitfall 6: Dependency Cycle Not Detected — Silent Deadlock on Start

**What goes wrong:**
If Process A depends on B, and B depends on A, and the cycle is not detected at `Start()` time, the dependency resolution loops forever or (with a visited-set approach) produces a wrong start order. The scheduler tries to start A, waits for B to be running, starts B, waits for A to be running — neither ever starts. No error is returned to the API caller; the HTTP response hangs until a timeout.

**Why it happens:**
Developers implement a simple DFS for dependency ordering without distinguishing between "visited" and "currently in the recursion stack," which is the necessary distinction for cycle detection (vs. DAG traversal).

**How to avoid:**
Use two maps: `visited` (fully processed) and `inStack` (currently being recursed into). If `inStack[id]` is true when `visit(id)` is called again, a cycle is detected:
```go
func topoSort(start string, processes map[string]*ManagedProcess) ([]string, error) {
    visited := map[string]bool{}
    inStack := map[string]bool{}
    order   := []string{}

    var visit func(id string) error
    visit = func(id string) error {
        if inStack[id] {
            return fmt.Errorf("dependency cycle detected: %s", id)
        }
        if visited[id] {
            return nil
        }
        inStack[id] = true
        p, ok := processes[id]
        if !ok {
            return fmt.Errorf("unknown dependency: %s", id)
        }
        for _, dep := range p.def.DependsOn {
            if err := visit(dep); err != nil {
                return err
            }
        }
        inStack[id] = false
        visited[id] = true
        order = append(order, id)
        return nil
    }
    return order, visit(start)
}
```

**Warning signs:**
- `POST /api/processes/{id}/start` hangs and eventually returns a timeout error
- Scheduler has no entry in its logs for "starting dependency X"
- Two processes defined with each other in their `dependsOn` lists cause the API to become unresponsive

**Phase to address:** Dependency ordering implementation (Step 4 in build order)

---

### Pitfall 7: Restart Goroutine Leaks When Stop() Is Called During Backoff Wait

**What goes wrong:**
When a process exits and the restart policy fires, `scheduleRestart()` spawns a goroutine that sleeps for the backoff duration then calls `s.Start(p.def.ID)`. If the user calls `Stop()` on the process (setting its state to `stopped`) while the restart goroutine is sleeping, the goroutine wakes up after the sleep, ignores the stop, and restarts the process anyway. The process appears to ignore the stop command and keeps restarting. Multiple rapid stop/start cycles can accumulate multiple leaked restart goroutines that all eventually fire.

**Why it happens:**
The restart goroutine is spawned and forgotten — it has no cancellation mechanism. The sleep is a blind `time.Sleep(delay)` with no select on a stop signal.

**How to avoid:**
Assign each `ManagedProcess` a `restartCancel context.CancelFunc`. When spawning the restart goroutine, derive a context from it. When `Stop()` is called, call `restartCancel()` to abort any pending restart:
```go
type ManagedProcess struct {
    // ...
    restartCtx    context.Context
    restartCancel context.CancelFunc
}

func (s *Scheduler) scheduleRestart(p *ManagedProcess, exitCode int) {
    // ... policy checks ...
    delay := computeBackoff(p)

    ctx, cancel := context.WithCancel(context.Background())
    p.restartCancel = cancel  // overwrite previous cancel (already called)

    go func() {
        select {
        case <-time.After(delay):
            s.Start(p.def.ID)
        case <-ctx.Done():
            // Stop() was called — do not restart
        }
    }()
}

func (s *Scheduler) Stop(id string) error {
    s.mu.Lock()
    p := s.processes[id]
    if p.restartCancel != nil {
        p.restartCancel()  // cancel pending restart goroutine
    }
    p.state = StateStopping
    s.mu.Unlock()
    // send SIGTERM to child...
}
```

**Warning signs:**
- `Stop()` returns success but the process restarts a few seconds later
- After repeated stop attempts, the process spawns more rapidly than expected
- `runtime.NumGoroutine()` grows proportionally to stop/start cycles

**Phase to address:** Restart policy implementation (Step 5 in build order)

---

### Pitfall 8: Exponential Backoff Integer Overflow After Many Restarts

**What goes wrong:**
The pattern `delay := initialWait * (1 << restartCount)` uses a bit shift to compute `2^n`. After 63 restarts (on a 64-bit system), the shift overflows to zero or a negative duration. After only ~30 restarts with a 1-second initial wait, the computed delay exceeds 30 years. Without a cap, the delay becomes effectively infinite or wraps around to negative, causing `time.Sleep` to either wait forever or return immediately and cause a rapid restart loop.

**Why it happens:**
Bit shift as `2^n` looks clean and correct for small values. The cap-at-maximum step is easy to forget or write incorrectly.

**How to avoid:**
Always apply a cap before the sleep, and apply it before the shift to prevent overflow:
```go
func computeBackoff(p *ManagedProcess) time.Duration {
    r := p.def.Restart
    // cap restarts used for shift to avoid overflow
    n := p.restarts
    if n > 30 {
        n = 30  // 2^30 * 1s = ~12 days, well above any MaxWait
    }
    delay := r.InitialWait * (1 << uint(n))
    if delay > r.MaxWait || delay < 0 {  // < 0 catches overflow
        delay = r.MaxWait
    }
    return delay
}
```

**Warning signs:**
- After many restarts, process restarts immediately without any delay
- `time.Sleep` with a very large `time.Duration` causes the goroutine to appear permanently blocked
- Restart count exceeds 32 and behavior becomes erratic

**Phase to address:** Restart policy implementation (Step 5 in build order)

---

### Pitfall 9: REST Handler Blocks on Process Startup — cmd.Run() in HTTP Handler

**What goes wrong:**
If a `POST /api/processes/{id}/start` handler calls any blocking process function (directly or indirectly), the HTTP handler goroutine is held for the lifetime of the child process. The HTTP client receives no response until the process exits. Other API requests that arrive during this time may also block if they attempt to acquire the scheduler's write lock. The server appears to hang.

**Why it happens:**
Developers migrating from a CLI pattern write the process start inline in the handler without recognizing that `net/http` runs each request in its own goroutine but expects that goroutine to return promptly.

**How to avoid:**
The HTTP handler calls `scheduler.Start(id)` which returns immediately after `cmd.Start()` succeeds. The response is `202 Accepted`, not `200 OK`. The process runs asynchronously; clients poll status via `GET /api/processes/{id}`:
```go
func (h *Handler) handleStart(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if err := h.scheduler.Start(id); err != nil {
        writeError(w, err)
        return
    }
    writeJSON(w, http.StatusAccepted, map[string]string{"status": "starting"})
}
```

**Warning signs:**
- `POST /start` hangs until the process exits
- API returns status `200` for start, not `202`
- Only one process can be "started" at a time (others time out waiting)

**Phase to address:** API handler implementation (Step 6 in build order)

---

### Pitfall 10: Missing CORS Middleware — React SPA Cannot Reach the API

**What goes wrong:**
The React dev server runs on `localhost:5173` (Vite default). The Go API runs on `localhost:8080`. Any fetch from the React app to the Go API is cross-origin. Without `Access-Control-Allow-Origin` response headers, the browser blocks the response. Without handling `OPTIONS` preflight requests, any non-simple request (POST, DELETE, custom headers like `Content-Type: application/json`) fails silently with a CORS error in the browser console. The API works fine with `curl` and Postman (which do not enforce CORS), leading to confusion.

**Why it happens:**
CORS is a browser-only restriction. Backend developers test with curl and see success. The problem only surfaces when the browser client runs. The OPTIONS preflight is especially easy to miss — it is a separate HTTP request the browser sends automatically before the actual request.

**How to avoid:**
Add a CORS middleware that wraps all routes. Handle `OPTIONS` explicitly:
```go
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
During development, allow `*`. During production, restrict to the specific deployed origin.

Additionally, configure the Vite dev server proxy to forward `/api/*` to `:8080` to avoid CORS during development entirely:
```ts
// vite.config.ts
export default defineConfig({
  server: {
    proxy: {
      '/api': 'http://localhost:8080'
    }
  }
})
```
Note: the Vite proxy works only in `npm run dev` mode. Production builds served by the Go server do not use it — the API must still have correct CORS headers for any non-same-origin deployment.

**Warning signs:**
- Browser console shows: `Access to fetch at 'http://localhost:8080' from origin 'http://localhost:5173' has been blocked by CORS policy`
- `curl` works; browser does not
- DELETE and POST with `Content-Type: application/json` fail but GET succeeds (simple vs. preflighted requests)

**Phase to address:** API server setup (Step 6 in build order) — required before any frontend integration

---

### Pitfall 11: React Polling Without useEffect Cleanup — Memory Leak and State-on-Unmounted-Component

**What goes wrong:**
`setInterval(() => fetchLogs(), 2000)` in a `useEffect` without a cleanup function continues firing after the component unmounts (e.g., when the user navigates away). Each interval tick calls `fetch` and then attempts `setState(...)` on the unmounted component. React logs a warning ("Can't perform a React state update on an unmounted component") and the ongoing fetches cause a memory leak. In React 18+, the warning is suppressed but the leak persists.

Additionally, in-flight fetch requests from a previous render cycle complete after the component unmounts and attempt to update state — same problem.

**Why it happens:**
Developers familiar with imperative patterns write `useEffect(() => { setInterval(...) }, [])` without knowing that the return value of the `useEffect` callback is the cleanup function.

**How to avoid:**
Always return a cleanup function from polling `useEffect`s. Use `AbortController` to cancel in-flight fetches on unmount:
```tsx
useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    const poll = async () => {
        try {
            const res = await fetch(`/api/processes/${id}/logs`, {
                signal: controller.signal
            });
            if (!cancelled) {
                const data = await res.json();
                setLines(data.lines);
            }
        } catch (e) {
            if ((e as Error).name !== 'AbortError') {
                console.error(e);
            }
        }
    };

    poll();
    const interval = setInterval(poll, 2000);

    return () => {
        cancelled = true;
        controller.abort();
        clearInterval(interval);
    };
}, [id]);
```

**Warning signs:**
- Console warning: "Can't perform a React state update on an unmounted component"
- Network tab shows ongoing `/api/processes/{id}/logs` requests after navigating away from LogViewer
- Memory usage grows monotonically as the user opens/closes the log viewer repeatedly

**Phase to address:** React frontend log viewer (Step 8 in build order)

---

### Pitfall 12: http.ListenAndServe Blocks Main Goroutine — Signal Handler Never Runs

**What goes wrong:**
`http.ListenAndServe(addr, handler)` blocks the calling goroutine until the server exits. If the `rtx serve` subcommand calls `ListenAndServe` directly in the main goroutine and then tries to listen for OS signals on the same goroutine, the signal handler never runs — execution never reaches it. The server cannot be gracefully shut down and must be killed hard (`SIGKILL`), leaving managed processes as orphans.

**Why it happens:**
`ListenAndServe` looks like a final call — "start the server and run forever" — which makes developers forget it needs to be in a goroutine if anything else needs to happen concurrently (signal handling, context cancellation).

**How to avoid:**
Run the server in a goroutine. Listen for signals in the main goroutine. Use `server.Shutdown()` with a timeout context:
```go
srv := &http.Server{Addr: addr, Handler: mux}

go func() {
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Fatalf("[rtx] server error: %v", err)
    }
}()

sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
<-sigCh

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
    log.Printf("[rtx] shutdown error: %v", err)
}
// additional cleanup: stop all managed processes
```
`http.ErrServerClosed` is a normal return when `Shutdown()` is called — do not treat it as a fatal error.

**Warning signs:**
- Signal handling code after `ListenAndServe` is never reached
- Ctrl+C on `rtx serve` kills the server without graceful shutdown
- Managed child processes remain running after `rtx serve` exits

**Phase to address:** `rtx serve` subcommand implementation (Step 7 in build order)

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `cmd.Stdout = os.Stdout` in scheduler | No code change from v1.0 | Logs unavailable via API; all process output mixed in terminal | Never — log capture is a v1.1 requirement |
| Growing `[]string` log slice (no ring buffer) | Simple `append` | OOM for verbose long-running processes | Never — implement ring buffer from the start |
| Scheduler-level `mu` protecting log buffer reads/writes | One lock for everything | Forces serialization of log I/O with all scheduler operations; reduces throughput | Never — give logBuffer its own lock |
| `time.Sleep(delay)` restart goroutine with no cancellation | Simple to implement | Stop() cannot abort a pending restart; goroutine leaks accumulate | Never — use select on a cancellable context |
| `1 << restartCount` bit shift with no cap | Looks correct for small N | Integer overflow after ~30 restarts; silent rapid-restart loop | Never — cap both the shift operand and the result |
| CORS wildcard `*` origin in production | Works everywhere | Allows any origin to make authenticated requests | Acceptable for v1.1 single-user local deployment; not for multi-user production |
| No `OPTIONS` preflight handler | Works for GET requests | All POST/DELETE/PUT requests fail from browser with CORS error | Never — always handle OPTIONS if you have non-GET routes |
| `setInterval` without `clearInterval` cleanup | Simpler code | Memory leak; state updates on unmounted component | Never |
| `http.ListenAndServe` in main goroutine | One less goroutine | Signal handler unreachable; no graceful shutdown | Never — always run in goroutine |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| runner.go → scheduler | Calling `process.Run()` for process lifecycle | Reuse patterns (Setpgid, Start, doneCh goroutine), not the function |
| logBuffer → cmd.Stdout | Assigning non-goroutine-safe writer; race between stdout/stderr copy goroutines | `logBuffer` has its own `sync.Mutex`; both Stdout and Stderr point to same locked buffer |
| scheduler → HTTP handler | Returning pointer to `ManagedProcess` from scheduler methods | Return a value copy (`ProcessStatus`) — callers must not hold a pointer to internal state |
| React → Go API | Calling `fetch('/api/...')` without handling non-2xx responses | Check `response.ok`; surface error state in UI; don't silently ignore API errors |
| Vite proxy → Go API | Relying on Vite proxy in production build | Proxy only works in dev; Go API must serve built React static files in production |
| Stop() → restart goroutine | Calling Stop() while restart backoff sleep is in progress | Cancel the restart context in Stop() before sending SIGTERM to child |
| http.ErrServerClosed → error log | Treating ErrServerClosed as a fatal server error | Check `errors.Is(err, http.ErrServerClosed)` and ignore it — it is normal shutdown |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| No ring buffer cap on logBuffer | API server RSS grows indefinitely | Ring buffer with 1,000-line cap from the start | Any verbose long-running process |
| Scheduler-level write lock held during `cmd.Start()` | All API operations serialize with process startup | Release lock before `cmd.Start()`; re-acquire to write state after | When starting multiple processes simultaneously |
| React polling interval too short | Network tab shows constant requests; backend log shows flood of GET requests | 2-second polling interval is sufficient for log updates; use `useEffect` with `id` dep, not every render | Any polling interval under 500ms with multiple open LogViewers |
| `GET /api/processes` locking scheduler on every list request | List operations block all writes temporarily | `RLock` for reads; return snapshot copy of state, not live map reference | When list is called frequently (e.g., polled by process list UI) |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| `exec.Command("sh", "-c", userInput)` — shell wrapping user-supplied command | Shell injection — arbitrary command execution with the API server's privileges | Use `exec.Command(args[0], args[1:]...)` directly, as in v1.0; document that shell built-ins are not supported |
| CORS wildcard `*` with `Access-Control-Allow-Credentials: true` | Any origin can make credentialed requests to the process manager API | Never combine wildcard origin with credentials; for v1.1 single-user local, avoid credentials header entirely |
| No validation of process `command` field in API handler | User can register arbitrary commands (e.g., `rm -rf /`) via POST /api/processes | Document scope: v1.1 is single-user, single-machine; no remote access assumed. Add allowlist if remote access is added later |
| Child processes inheriting API server's full environment | Sensitive env vars (API keys, credentials in env) passed to all managed processes | Explicitly set `cmd.Env` in the scheduler; do not blindly pass `os.Environ()` |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Polling log viewer replaces all lines on each tick | Log viewer "flashes" on every poll; user loses scroll position | Track line count; append only new lines; preserve scroll position unless user has scrolled to bottom |
| No visual indicator for process state transitions | "starting" and "stopping" states look identical to "running" from the UI; user does not know if action is in progress | Show distinct visual states for idle/starting/running/stopping/stopped with appropriate colors |
| Error responses from API not surfaced in UI | User clicks "Start", nothing happens, no feedback | All fetch calls check `response.ok`; non-2xx responses show an error message in the UI |
| Log viewer always polls even for stopped processes | Unnecessary network requests for processes that are not running | Stop polling when `process.status === 'stopped'`; resume if status changes to `running` |
| Deleting a running process | Process continues running as unmanaged orphan | Validate in API: DELETE returns 409 Conflict if process is not stopped; UI disables Delete button for running processes |

---

## "Looks Done But Isn't" Checklist

- [ ] **Log capture:** `GET /api/processes/{id}/logs` returns actual output from the managed process — verify with `echo "hello"` as the process command
- [ ] **Log race safety:** `go test -race ./internal/scheduler/...` passes with no data race errors on the log buffer
- [ ] **Ring buffer eviction:** After 1,001 lines of output, the first line is gone and the buffer holds exactly 1,000 lines
- [ ] **Zombie prevention in scheduler:** After a managed process exits, no zombie appears in `ps aux | grep Z` while the API server is still running
- [ ] **Dependency ordering:** Process B (depends on A) does not start until A is in `running` state — verify start order in logs
- [ ] **Cycle detection:** Adding Process A → depends on B, B → depends on A; calling `POST /api/processes/A/start` returns 400 with "dependency cycle detected", not a hang
- [ ] **Stop cancels restart:** `Stop()` on a process with `restart: always` stops it and it does not restart — verify by checking status after 10 seconds
- [ ] **Backoff cap:** After 100 restart attempts, the delay between restarts is capped at `MaxWait` and does not grow further or overflow
- [ ] **CORS preflight:** `OPTIONS /api/processes` returns 204 with correct `Access-Control-Allow-*` headers — verify with `curl -X OPTIONS -v`
- [ ] **Graceful shutdown:** `Ctrl+C` on `rtx serve` stops the HTTP server and sends SIGTERM to all managed processes before exiting
- [ ] **React cleanup:** Opening and closing the LogViewer 10 times; `chrome://inspect` heap snapshot shows no growing listener or interval accumulation
- [ ] **Vite proxy not relied on in production:** The built React app served by Go (`/`) fetches `/api/...` and receives correct responses without Vite running

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| process.Run() called from scheduler — blocking design discovered | HIGH | Refactor scheduler to manage `*exec.Cmd` directly; add per-process logBuffer; re-test all lifecycle scenarios |
| Direct fd assignment discovered — no log capture | MEDIUM | Replace `cmd.Stdout = os.Stdout` with `cmd.Stdout = p.logBuf` in scheduler; add logBuffer type; re-test |
| Log buffer data race discovered by -race | LOW-MEDIUM | Add `sync.Mutex` to logBuffer; ensure `Write()` and `Lines()` both acquire it; re-run -race |
| Stop() not cancelling restart goroutine | MEDIUM | Add context.CancelFunc per process; cancel in Stop(); add test for stop-during-backoff scenario |
| CORS blocking React from API | LOW | Add corsMiddleware before all routes; test with browser; add OPTIONS handler |
| React polling not cleaning up | LOW | Add useEffect return cleanup function; clearInterval + controller.abort(); test by unmounting component |
| ListenAndServe blocking signal handler | LOW | Move ListenAndServe to goroutine; add signal loop in main; add Shutdown() call with timeout |
| Integer overflow in backoff | LOW | Cap shift operand at 30; cap result at MaxWait; add negative-duration guard |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Calling process.Run() from scheduler | Scheduler design (Step 3) | Scheduler has no import of `internal/process`; `Stop()` calls `cmd.Process.Signal()` |
| Direct fd assignment — no log capture | Scheduler process struct (Step 2) | `GET /logs` returns actual process output |
| logBuffer data race | Scheduler process struct (Step 2) | `go test -race ./internal/scheduler/...` passes |
| Unbounded log memory | Scheduler process struct (Step 2) | Buffer evicts at 1,000 lines; memory stable for verbose process |
| Lock held during cmd.Start() | Scheduler start/stop (Step 3) | `-race` test passes; simultaneous starts do not serialize |
| Dependency cycle deadlock | Dependency ordering (Step 4) | Cycle returns error immediately; no hang |
| Restart goroutine not cancellable | Restart policy (Step 5) | Stop during backoff prevents restart |
| Backoff integer overflow | Restart policy (Step 5) | Delay never exceeds MaxWait after many restarts |
| HTTP handler blocks on process start | API handlers (Step 6) | `POST /start` returns 202 immediately; process runs async |
| Missing CORS middleware | API server setup (Step 6) | OPTIONS returns 204; React dev server can reach API |
| React polling without cleanup | React frontend (Step 8) | No stale intervals; no state-on-unmounted-component warning |
| ListenAndServe blocks signal handler | rtx serve subcommand (Step 7) | Ctrl+C triggers graceful shutdown within 10 seconds |

---

## Sources

- `internal/process/runner.go` — v1.0 codebase, direct read — HIGH confidence (patterns being adapted)
- `.planning/research/ARCHITECTURE.md` — v1.1 architecture decisions — HIGH confidence
- [os/exec package — pkg.go.dev](https://pkg.go.dev/os/exec) — `Cmd.Start`, `Cmd.Wait`, internal copy goroutines — HIGH confidence
- [sync package — pkg.go.dev](https://pkg.go.dev/sync) — RWMutex, Mutex semantics — HIGH confidence
- [Go 1.22 net/http — pkg.go.dev](https://pkg.go.dev/net/http) — ServeMux, PathValue, ErrServerClosed — HIGH confidence
- [Go race detector — go.dev](https://go.dev/doc/articles/race_detector) — race detection methodology — HIGH confidence
- [Go issue #19804: data race with same writer for Stdout/Stderr](https://github.com/golang/go/issues/19804) — HIGH confidence (concurrent copy goroutines from cmd.Start)
- [Proper HTTP shutdown in Go — DEV Community](https://dev.to/mokiat/proper-http-shutdown-in-go-3fji) — ListenAndServe goroutine + Shutdown pattern — MEDIUM confidence
- [React useEffect cleanup — refine.dev](https://refine.dev/blog/useeffect-cleanup/) — polling cleanup pattern — MEDIUM confidence
- [AbortController in React — j-labs.pl](https://www.j-labs.pl/en/tech-blog/how-to-use-the-useeffect-hook-with-the-abortcontroller/) — fetch cancellation on unmount — MEDIUM confidence
- [CORS common mistakes — corsfix.com](https://corsfix.com/blog/common-cors-mistakes) — OPTIONS preflight, wildcard+credentials — MEDIUM confidence
- [Vite proxy CORS guide — dbi-services.com](https://www.dbi-services.com/blog/avoid-cors-requests-in-development-mode-with-vite/) — dev-only proxy limitation — MEDIUM confidence
- [cenkalti/backoff Go implementation](https://github.com/cenkalti/backoff) — backoff cap and overflow prevention patterns — MEDIUM confidence
- [Goroutine leak prevention — DEV Community](https://dev.to/serifcolakel/go-concurrency-mastery-preventing-goroutine-leaks-with-context-timeout-cancellation-best-1lg0) — context cancellation for background goroutines — MEDIUM confidence

---
*Pitfalls research for: Runtime X v1.1 — multi-process scheduler, Go REST API, React frontend*
*Researched: 2026-03-01*
