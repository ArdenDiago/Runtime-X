# Phase 6: Scheduler Start, Stop, and Lifecycle - Research

**Researched:** 2026-03-01
**Domain:** Go process lifecycle management — os/exec, syscall, sync, bufio
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Stop signal behavior:**
- SIGTERM first, then escalate to SIGKILL after a timeout — guarantees the process dies
- Stop timeout is configurable per process via a `StopTimeout` field on ProcessDef, with a sane default
- Stop() blocks until the process has fully exited and state is updated — caller knows it's done when Stop() returns
- Send signals to the entire process group (using process group / setpgid) — prevents orphan child processes

**Start failure handling:**
- Failed start transitions to StateFailed with error stored — user can see why and retry
- Process that exits on its own: exit code 0 -> Stopped, non-zero -> Failed — distinguishes clean exit from crash
- Start() is callable from Idle, Stopped, or Failed states — re-startable without Remove+Register ceremony
- Let exec fail naturally — don't pre-check with LookPath. Handle the error from cmd.Start()

**Output capture wiring:**
- Line-buffered using bufio.Scanner — each LogEntry is one line of output, clean for display
- Continue capturing output until EOF (process exit) — captures shutdown messages and cleanup output
- Separate goroutines for stdout and stderr — one goroutine per stream, each writing to shared logBuffer with its stream tag
- Truncate lines at a limit (e.g., 8KB) to prevent memory issues from binary data dumps

**Concurrent start/stop safety:**
- Start() on already-running process returns ErrAlreadyRunning — clear and predictable
- Stop() during Starting state: reject with error — caller must wait for Running state
- StopAll() deferred to Phase 10 (CLI serve and Graceful Shutdown) — keeps this phase focused
- Remove() blocked during Starting/Stopping transitions — only allowed from Idle, Stopped, or Failed

### Claude's Discretion
- Exact goroutine structure for the process monitor (wait for exit + update state)
- Whether to use cmd.StdoutPipe vs custom io.Writer for capture
- Scanner buffer size for line reading
- Exact error types and messages for edge cases
- Whether StopTimeout default is 5s or 10s

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SCH-02 | User can start a registered process — scheduler spawns it and tracks its PID and status | exec.Cmd.Start() pattern, SysProcAttr.Setpgid=true, PID capture from cmd.Process.Pid, state transition Idle/Stopped/Failed → Starting → Running |
| SCH-03 | User can stop a running process — scheduler sends SIGTERM and waits for exit | syscall.Kill(-pgid, SIGTERM), timer-based SIGKILL escalation, cmd.Wait() in monitor goroutine, blocking Stop() |
| SCH-04 | User can list all registered processes with their current status (stopped/running/restarting/failed) | Scheduler.List() already exists from Phase 5; needs State to reflect live values from Start/Stop |
</phase_requirements>

---

## Summary

Phase 6 wires the existing Scheduler skeleton (types, FSM, logBuffer from Phase 5) into real OS process execution. The work centers on three Go stdlib packages: `os/exec` for spawning processes, `syscall` for process group signals, and `bufio` for line-buffered output capture.

The critical architectural decision (already locked in CONTEXT.md) is the **single monitor goroutine** as the authoritative state authority. After `cmd.Start()` succeeds, one goroutine calls `cmd.Wait()` and owns all state transitions to Running → Stopped/Failed. This is the only design that avoids races between `Stop()` and natural process exit — neither `Stop()` nor the monitor can mutate state without coordination.

The process group pattern (`SysProcAttr.Setpgid: true` + `syscall.Kill(-pid, sig)`) is mandatory for zombie/orphan prevention. Without it, spawning a shell script that forks children leaves orphans alive after the parent shell exits. The `exec.Cmd` struct cannot be reused — a fresh `*exec.Cmd` must be created on each `Start()` call, including restarts.

**Primary recommendation:** Implement Start() as: create new exec.Cmd → set SysProcAttr.Setpgid=true → attach StdoutPipe/StderrPipe → release scheduler write lock → call cmd.Start() → re-acquire write lock → update state and PID → release lock → launch three goroutines (stdout scanner, stderr scanner, process monitor).

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os/exec` | Go stdlib | Spawn child processes, access Process.Pid, Wait for exit | Only stdlib API for executing subprocesses |
| `syscall` | Go stdlib | SysProcAttr.Setpgid, Kill(-pgid, sig), SIGTERM/SIGKILL constants | Required for process group control on Linux |
| `bufio` | Go stdlib | bufio.Scanner for line-buffered stdout/stderr reading | Splits output into LogEntry lines cleanly |
| `sync` | Go stdlib | RWMutex (already on Scheduler), already on logBuffer | All concurrency is stdlib; no third-party needed |
| `time` | Go stdlib | StopTimeout timer, StartedAt/StoppedAt timestamps | Zero-dependency timing |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `errors` | Go stdlib | errors.Is() for sentinel error checking, fmt.Errorf %w wrapping | All error handling already follows this pattern |
| `io` | Go stdlib | io.ReadCloser from StdoutPipe/StderrPipe | Pipe lifecycle management |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `cmd.StdoutPipe` | `cmd.Stdout = &bytes.Buffer` or custom io.Writer | Pipe is cleaner for streaming; buffer accumulates entire output in memory. Pipe gives per-line timing. Use Pipe. |
| `bufio.Scanner` | `bufio.NewReader` + ReadString('\n') | Scanner handles edge cases (partial lines, EOF) more cleanly; Scanner.Buffer() for size cap. Use Scanner. |
| `syscall.Kill(-pgid, sig)` | `cmd.Process.Signal(sig)` | Signal(sig) sends to PID only, not process group — orphan children survive. Must use Kill(-pgid). |
| `time.AfterFunc` | select + time.After channel | Both work; time.After leaks timer if process exits fast; AfterFunc with timer.Stop() is cleaner. Either is fine. |

**Installation:** No external packages needed. Pure Go stdlib.

---

## Architecture Patterns

### Recommended File Structure

```
internal/scheduler/
├── types.go          # ProcessDef, ManagedProcess, State FSM (Phase 5, add StopTimeout field)
├── scheduler.go      # Scheduler struct, Register/Remove/Get/List/Logs (Phase 5)
├── logbuffer.go      # logBuffer ring buffer (Phase 5)
├── start.go          # NEW: Scheduler.Start() implementation
├── stop.go           # NEW: Scheduler.Stop() implementation
├── monitor.go        # NEW: monitor goroutine (monitorProcess), capture goroutines
├── start_test.go     # NEW: TDD tests for Start(), including race tests
└── stop_test.go      # NEW: TDD tests for Stop(), including race tests
```

Alternatively the three new files can be collapsed into `lifecycle.go` — either is fine. Splitting by concern (start.go / stop.go / monitor.go) keeps diffs readable.

### Pattern 1: Start() — Lock, Check, Unlock, Exec, Relock

The critical rule: **release the scheduler write lock before calling cmd.Start()**.
`cmd.Start()` is a blocking OS call (fork+exec) that can take milliseconds. Holding the write lock during this time blocks all concurrent reads/gets. More critically, if output capture goroutines are launched before cmd.Start() returns, they will call `mp.logs.Write()` — which is fine since logBuffer has its own mutex — but any code path that re-acquires the scheduler lock from a goroutine while the main goroutine still holds it will deadlock.

```go
// Source: verified pattern based on STATE.md [v1.1 arch] decisions and os/exec docs
func (s *Scheduler) Start(name string) error {
    s.mu.Lock()

    mp, exists := s.processes[name]
    if !exists {
        s.mu.Unlock()
        return fmt.Errorf("%w: %s", ErrNotFound, name)
    }

    // Check allowed start states: Idle, Stopped, Failed
    switch mp.State {
    case StateIdle, StateStopped, StateFailed:
        // allowed
    case StateRunning:
        s.mu.Unlock()
        return fmt.Errorf("%w: %s", ErrAlreadyRunning, name)
    default:
        s.mu.Unlock()
        return fmt.Errorf("cannot start process %q in state %s", name, mp.State)
    }

    // Transition to Starting while holding lock — makes the state visible immediately
    if err := transition(mp, StateStarting); err != nil {
        s.mu.Unlock()
        return err
    }

    // Reset runtime metadata for restart
    mp.StartedAt = time.Now()
    mp.StoppedAt = time.Time{}
    mp.ExitCode = 0

    // Capture local copies of what we need after unlock
    def := mp.Def

    s.mu.Unlock() // CRITICAL: release before cmd.Start()

    // Build cmd — must create fresh *exec.Cmd on every start (Cmd cannot be reused)
    cmd := exec.Command(def.Command, def.Args...)
    cmd.Dir = def.WorkDir
    if len(def.Env) > 0 {
        cmd.Env = def.Env
    }
    // Process group: child and its children get a new PGID == child's PID
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    // Attach pipes before Start()
    stdoutPipe, err := cmd.StdoutPipe()
    if err != nil {
        s.mu.Lock()
        transition(mp, StateFailed) // nolint: errcheck — only fails on logic bugs
        s.mu.Unlock()
        return fmt.Errorf("stdout pipe: %w", err)
    }
    stderrPipe, err := cmd.StderrPipe()
    if err != nil {
        s.mu.Lock()
        transition(mp, StateFailed)
        s.mu.Unlock()
        return fmt.Errorf("stderr pipe: %w", err)
    }

    if err := cmd.Start(); err != nil {
        s.mu.Lock()
        transition(mp, StateFailed)
        s.mu.Unlock()
        return fmt.Errorf("start %q: %w", def.Command, err)
    }

    // cmd.Start() succeeded — PID is now valid
    s.mu.Lock()
    transition(mp, StateRunning) // Starting → Running
    mp.cmd = cmd                 // store cmd for Stop() to use
    s.mu.Unlock()

    // Launch output capture goroutines (they write to mp.logs, not scheduler state)
    go captureOutput(mp.logs, stdoutPipe, StreamStdout)
    go captureOutput(mp.logs, stderrPipe, StreamStderr)

    // Launch monitor goroutine — single source of truth for terminal state
    go monitorProcess(s, mp, cmd)

    return nil
}
```

**Key points:**
- `mp.cmd` must be added to `ManagedProcess` struct as `cmd *exec.Cmd` (unexported)
- `ErrAlreadyRunning` is a new sentinel error for this phase
- Transition to `StateStarting` happens before unlock — external callers can observe the transition immediately

### Pattern 2: Monitor Goroutine — Single Source of Truth

The monitor goroutine calls `cmd.Wait()` and is the **only** code that transitions to `StateStopped` or `StateFailed` on natural process exit. `Stop()` sends a signal and blocks on a done channel; the monitor does the actual state update and closes the channel.

```go
// Source: verified pattern from os/exec docs, STATE.md arch decisions
func monitorProcess(s *Scheduler, mp *ManagedProcess, cmd *exec.Cmd) {
    // cmd.Wait() blocks until process exits AND all pipe goroutines complete.
    // This is correct: output capture goroutines read from the pipes, and
    // Wait() waits for them to finish — so logs are fully captured before
    // state transitions to Stopped/Failed.
    err := cmd.Wait()

    s.mu.Lock()
    defer s.mu.Unlock()

    mp.StoppedAt = time.Now()

    if mp.State == StateStopping {
        // Stop() requested — transition to Stopped regardless of exit code
        mp.ExitCode = cmd.ProcessState.ExitCode()
        transition(mp, StateStopped)
    } else {
        // Natural exit (not requested by Stop())
        code := cmd.ProcessState.ExitCode()
        mp.ExitCode = code
        if err == nil || code == 0 {
            transition(mp, StateStopped) // clean exit
        } else {
            transition(mp, StateFailed)  // crash / non-zero exit
        }
    }

    // Signal Stop() that the process is done (if it's waiting)
    if mp.doneCh != nil {
        close(mp.doneCh)
        mp.doneCh = nil
    }
}
```

**doneCh lifecycle:** `mp.doneCh` is a `chan struct{}` created by `Stop()` before signaling the process. The monitor goroutine closes it when `cmd.Wait()` returns. `Stop()` blocks on `<-mp.doneCh`. This is safe because `doneCh` is created and assigned while `Stop()` holds the write lock, and `monitorProcess` only reads/closes it while holding the same write lock.

### Pattern 3: Stop() — Signal Process Group, Block for Exit

```go
// Source: verified pattern from syscall docs, bigkevmcd.github.io article
func (s *Scheduler) Stop(name string) error {
    s.mu.Lock()

    mp, exists := s.processes[name]
    if !exists {
        s.mu.Unlock()
        return fmt.Errorf("%w: %s", ErrNotFound, name)
    }

    if mp.State != StateRunning {
        s.mu.Unlock()
        return fmt.Errorf("cannot stop process %q in state %s: %w", name, mp.State, ErrNotRunning)
    }

    // Transition to Stopping — blocks new Start() calls and Remove()
    transition(mp, StateStopping)

    // Create the done channel before signaling — monitor will close it
    doneCh := make(chan struct{})
    mp.doneCh = doneCh

    // Capture pid and timeout before releasing lock
    pid := mp.cmd.Process.Pid
    timeout := mp.Def.StopTimeout
    if timeout <= 0 {
        timeout = 5 * time.Second // default
    }

    s.mu.Unlock()

    // Send SIGTERM to entire process group (-pid = kill process group)
    _ = syscall.Kill(-pid, syscall.SIGTERM)

    // Wait for monitor goroutine to close doneCh, with SIGKILL escalation
    select {
    case <-doneCh:
        // Process exited cleanly within timeout
        return nil
    case <-time.After(timeout):
        // Escalate: send SIGKILL to process group
        _ = syscall.Kill(-pid, syscall.SIGKILL)
        // Still wait for monitor — SIGKILL will definitely terminate the process
        <-doneCh
        return nil
    }
}
```

### Pattern 4: captureOutput — Line-buffered Scanner

```go
// Source: bufio.Scanner official docs, CONTEXT.md decision
const maxLineBytes = 8 * 1024 // 8KB truncation limit per line

func captureOutput(lb *logBuffer, r io.ReadCloser, stream Stream) {
    scanner := bufio.NewScanner(r)
    // Set custom buffer: initial 4KB, max 8KB per line
    buf := make([]byte, 4096)
    scanner.Buffer(buf, maxLineBytes)

    for scanner.Scan() {
        lb.Write(LogEntry{
            Timestamp: time.Now(),
            Stream:    stream,
            Text:      scanner.Text(),
        })
    }
    // scanner.Err() is nil on EOF (normal exit) — ignore it.
    // The pipe is closed by cmd.Wait() after process exits.
}
```

**Why not check scanner.Err():** On normal process exit, the pipe EOF produces `nil` from `scanner.Err()`. On unexpected pipe close (killed process), it may return `io.ErrClosedPipe` or similar — this is expected and benign; we simply stop capturing. No need to log the error.

### Pattern 5: ManagedProcess cmd field

Add to `ManagedProcess` in `types.go`:

```go
type ManagedProcess struct {
    Def          ProcessDef
    State        State
    StartedAt    time.Time
    StoppedAt    time.Time
    ExitCode     int
    RestartCount int
    logs         *logBuffer

    // Phase 6 runtime fields — zeroed between restarts
    cmd    *exec.Cmd    // nil when not running; set by Start(), cleared after Wait()
    doneCh chan struct{} // nil unless Stop() is pending; closed by monitor goroutine
}
```

### Pattern 6: ProcessDef StopTimeout field

Add to `ProcessDef` in `types.go`:

```go
type ProcessDef struct {
    // ... existing fields ...

    // StopTimeout is how long Stop() waits after SIGTERM before escalating to SIGKILL.
    // A zero value uses the scheduler default (5 seconds).
    StopTimeout time.Duration
}
```

### Anti-Patterns to Avoid

- **Holding scheduler write lock during cmd.Start():** Blocks all readers and risks deadlock with output capture goroutines that attempt to acquire logBuffer mutex. Always release the write lock before calling cmd.Start().
- **Calling cmd.Wait() from Stop():** cmd.Wait() must be called by exactly one goroutine. The monitor goroutine owns Wait(). Stop() blocks on doneCh — it never calls Wait() directly.
- **Reusing exec.Cmd:** A `*exec.Cmd` cannot be reused after Start(). Always create a fresh `exec.Cmd` in Start(), including on restart from Stopped/Failed state.
- **Signaling only cmd.Process.Pid (not the process group):** `cmd.Process.Signal(syscall.SIGTERM)` kills only the process, not its children. Spawned shells that fork children will leave orphan grandchildren. Use `syscall.Kill(-pid, syscall.SIGTERM)` to kill the entire process group.
- **Creating doneCh after signaling:** Race condition — if the process exits very fast (before the monitor goroutine runs), the monitor might see `doneCh == nil` and skip closing it. Always create and assign doneCh before sending the signal.
- **Transitioning state from two goroutines:** Only the monitor goroutine transitions from Running → Stopped/Failed. Stop() only does Running → Stopping. Mixing these creates races.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Line splitting | Manual byte scanning, strings.Split | `bufio.Scanner` with `ScanLines` | Handles partial lines, EOF, newline variants (\n, \r\n), buffer reuse |
| Process exit code | Parse error string | `cmd.ProcessState.ExitCode()` | ProcessState.ExitCode() returns -1 for signal-killed, 0 for success, N for non-zero exit |
| Process liveness check | Send signal(0) and check error | `cmd.Wait()` result in monitor goroutine | Wait() is the definitive source — signal(0) has TOCTOU races |
| Timer escalation | Goroutine with sleep + channel | `time.After(timeout)` in select | Simple, GC'd when case fires, no goroutine leak if process exits fast |

**Key insight:** Go's os/exec + syscall are complete for this use case. No external library adds value here.

---

## Common Pitfalls

### Pitfall 1: Lock Held During cmd.Start()

**What goes wrong:** If you call cmd.Start() while holding the scheduler write lock, any code path that tries to acquire the lock (directly or indirectly) from another goroutine will deadlock. Even if no deadlock occurs, it blocks List() and Get() calls for the duration of the fork+exec syscall.

**Why it happens:** It looks natural to hold the lock "for the whole operation" — but cmd.Start() is not a pure in-memory operation.

**How to avoid:** The three-phase pattern: (1) lock → validate → transition to Starting → capture def → unlock, (2) create cmd and call cmd.Start() with no lock held, (3) re-lock → update PID and state → unlock. This is documented in STATE.md as `[v1.1 arch]: Release scheduler write lock before cmd.Start() to prevent deadlock`.

**Warning signs:** Deadlock in integration tests where List() is called while Start() is running. Race detector may catch this as a lock-order cycle.

### Pitfall 2: Race Between Stop() and Natural Process Exit

**What goes wrong:** Stop() sends SIGTERM and then tries to read mp.ExitCode — but the monitor goroutine is also updating ExitCode from cmd.Wait(). Without coordination, both goroutines write to shared fields simultaneously.

**Why it happens:** The monitor goroutine runs independently. The process may exit (naturally or from signal) between when Stop() sends SIGTERM and when Stop() reads the state.

**How to avoid:** The monitor goroutine is the **single source of truth**. Stop() never reads ExitCode or State directly after sending the signal. It blocks on `<-doneCh` and only reads result after the monitor has closed the channel and fully updated state under the write lock.

**Warning signs:** `go test -race` will flag concurrent writes to `mp.ExitCode` or `mp.State`. Test: have Stop() read mp.State after doneCh closes — it should always be StateStopped.

### Pitfall 3: cmd.Wait() Waits for Pipe Goroutines, Not Just Process Exit

**What goes wrong:** You think "cmd.Wait() returns when the process exits" but it actually waits for all pipe-copying goroutines to finish too. If a pipe-reading goroutine is blocked (e.g., large output buffer not drained), Wait() blocks indefinitely.

**Why it happens:** cmd.Wait() is designed to ensure all output is captured before returning. This is the correct behavior for our use case (we want full log capture), but it can surprise if you expect immediate return.

**How to avoid:** This is actually desirable for our use case — we want all output captured before transitioning to Stopped/Failed. The captureOutput goroutines read until EOF, then exit. Wait() returns after they're done. No action needed; just understand the contract.

**Warning signs:** Stop() hangs indefinitely after process dies. Symptom: process is dead (PID not in ps) but Stop() hasn't returned. Cause: a pipe-reading goroutine is blocked.

### Pitfall 4: Zombie Processes Without Setpgid

**What goes wrong:** Without `Setpgid: true`, child processes (spawned by the managed process, e.g., a shell script that forks) inherit the parent's process group. When you send SIGTERM to the managed process PID, the children survive as orphans, eventually reparented to init. The managed process becomes a zombie until its grandchildren exit.

**Why it happens:** Signal(sig) on a PID only kills that one process, not its children.

**How to avoid:** Always set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` in Start(). Use `syscall.Kill(-pid, sig)` to kill the entire process group. This is the approach the CONTEXT.md has locked in.

**Warning signs:** `ps -eo pid,ppid,stat,comm` shows Z (zombie) next to process entries. Or child processes are still running after Stop() returns.

### Pitfall 5: doneCh Nil Check Race in Monitor

**What goes wrong:** Monitor goroutine checks `if mp.doneCh != nil { close(mp.doneCh) }` but Stop() hasn't run yet (process exited naturally before Stop() was called). This is fine — doneCh is nil, nothing to close. The problem arises if Stop() creates doneCh AFTER the monitor has already finished and returned: Stop() will block forever on `<-doneCh` because nobody will close it.

**Why it happens:** Natural process exit and Stop() are concurrent.

**How to avoid:** When the monitor goroutine finishes and sets `mp.State = StateStopped/StateFailed`, Stop() (which holds or tries to acquire the write lock after creating doneCh) must detect the terminal state. Solution: Stop() checks state again after acquiring lock — if state is already Stopped or Failed, return immediately without creating doneCh. Alternatively, design so that doneCh is always created before the signal is sent, and the monitor always closes it if non-nil.

**Warning signs:** Stop() blocks forever when called on a process that already exited naturally before Stop() ran. Integration test: register, start, wait for process to exit (sleep 0), then call Stop() — expect ErrNotRunning (because monitor updated state to Stopped before Stop() runs) or a clean immediate return.

### Pitfall 6: Scanner Default Buffer Size (64KB max token) is Too Large for Binary Output

**What goes wrong:** A process dumps binary data to stdout. bufio.Scanner's default `MaxScanTokenSize` is 64*1024 bytes. The `bufio.ErrTooLong` error fires when a single "line" (bytes before a newline) exceeds this. Scanner stops scanning on this error.

**Why it happens:** Binary data rarely contains newlines at regular intervals.

**How to avoid:** Use `scanner.Buffer(buf, maxLineBytes)` where `maxLineBytes = 8*1024`. Scanner.Buffer sets the maximum token size. Tokens longer than this are silently truncated (actually: Scanner returns `bufio.ErrTooLong` and stops). For our use case, we should handle the error by truncating. The CONTEXT.md decision says: "Truncate lines at a limit (e.g., 8KB)". The clearest approach: use a custom SplitFunc that truncates rather than errors.

**Warning signs:** Output capture goroutine exits early with scanner.Err() returning `bufio.ErrTooLong`. Logs stop appearing mid-stream from a process.

---

## Code Examples

Verified patterns from official sources and CONTEXT.md locked decisions:

### Starting a Process with Process Group Isolation

```go
// Source: os/exec docs (pkg.go.dev/os/exec), syscall docs (pkg.go.dev/syscall)
cmd := exec.Command(def.Command, def.Args...)
cmd.Dir = def.WorkDir
if len(def.Env) > 0 {
    cmd.Env = def.Env
}
// Create new process group so signal -pgid kills all children too
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

stdoutPipe, _ := cmd.StdoutPipe()
stderrPipe, _ := cmd.StderrPipe()

err := cmd.Start()
// cmd.Process.Pid is now valid (if err == nil)
```

### Killing a Process Group

```go
// Source: sigmoid.at/post/2023/08/kill_process_descendants_golang/,
//         bigkevmcd.github.io article
// Negative PID = kill entire process group with PGID == pid
pid := cmd.Process.Pid
_ = syscall.Kill(-pid, syscall.SIGTERM)
// After timeout:
_ = syscall.Kill(-pid, syscall.SIGKILL)
```

### Line-Buffered Output Capture

```go
// Source: pkg.go.dev/bufio#Scanner.Buffer
const maxLineBytes = 8 * 1024

func captureOutput(lb *logBuffer, r io.ReadCloser, stream Stream) {
    scanner := bufio.NewScanner(r)
    scanner.Buffer(make([]byte, 4096), maxLineBytes)
    for scanner.Scan() {
        lb.Write(LogEntry{
            Timestamp: time.Now(),
            Stream:    stream,
            Text:      scanner.Text(),
        })
    }
    // EOF or pipe closed — both are expected; no error logging needed
}
```

### Monitor Goroutine with doneCh Coordination

```go
// Source: os/exec docs — cmd.Wait() waits for pipe goroutines to finish too
func monitorProcess(s *Scheduler, mp *ManagedProcess, cmd *exec.Cmd) {
    err := cmd.Wait() // blocks until process exits AND all pipe goroutines done

    s.mu.Lock()
    defer s.mu.Unlock()

    mp.StoppedAt = time.Now()
    code := -1
    if cmd.ProcessState != nil {
        code = cmd.ProcessState.ExitCode()
    }
    mp.ExitCode = code

    wasStopping := mp.State == StateStopping
    if wasStopping || err == nil || code == 0 {
        transition(mp, StateStopped)
    } else {
        transition(mp, StateFailed)
    }

    if mp.doneCh != nil {
        close(mp.doneCh)
        mp.doneCh = nil
    }
}
```

### Stop() with SIGTERM → SIGKILL Escalation

```go
// Source: CONTEXT.md locked decisions, mezhenskyi.dev escalation pattern
func (s *Scheduler) Stop(name string) error {
    s.mu.Lock()
    mp, exists := s.processes[name]
    if !exists {
        s.mu.Unlock()
        return fmt.Errorf("%w: %s", ErrNotFound, name)
    }

    switch mp.State {
    case StateRunning:
        // proceed
    case StateStopped, StateIdle, StateFailed:
        s.mu.Unlock()
        return fmt.Errorf("%w: %s (state: %s)", ErrNotRunning, name, mp.State)
    case StateStarting, StateStopping:
        s.mu.Unlock()
        return fmt.Errorf("cannot stop process %q in transient state %s", name, mp.State)
    }

    transition(mp, StateStopping)
    doneCh := make(chan struct{})
    mp.doneCh = doneCh
    pid := mp.cmd.Process.Pid
    timeout := mp.Def.StopTimeout
    if timeout <= 0 {
        timeout = 5 * time.Second
    }
    s.mu.Unlock()

    _ = syscall.Kill(-pid, syscall.SIGTERM)

    select {
    case <-doneCh:
        return nil
    case <-time.After(timeout):
        _ = syscall.Kill(-pid, syscall.SIGKILL)
        <-doneCh
        return nil
    }
}
```

### Testing Strategy: Real Process Tests

For Phase 6 TDD tests, use real short-lived processes — not mocks. Go's testing infrastructure supports this cleanly:

```go
// Test Start() with a real process
s := scheduler.New()
s.Register(scheduler.ProcessDef{Name: "sleeper", Command: "sleep", Args: []string{"10"}})

if err := s.Start("sleeper"); err != nil {
    t.Fatal(err)
}

mp, _ := s.Get("sleeper")
if mp.State != scheduler.StateRunning {
    t.Errorf("state = %v, want Running", mp.State)
}
if mp.cmd.Process.Pid == 0 {
    t.Error("PID is 0 after Start()")
}

// Test Stop()
if err := s.Stop("sleeper"); err != nil {
    t.Fatal(err)
}
mp2, _ := s.Get("sleeper")
if mp2.State != scheduler.StateStopped {
    t.Errorf("state = %v after Stop(), want Stopped", mp2.State)
}
```

**Note:** `mp.cmd` is unexported. Tests are in `package scheduler` (same package), so they have access to unexported fields. This is consistent with existing tests that call `transition()` directly.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Process.Kill() only | Process group kill with syscall.Kill(-pgid) | Required for zombie prevention | Orphan children no longer survive stop |
| cmd.Run() for simple scripts | cmd.Start() + monitor goroutine + cmd.Wait() | Required for async management | Enables non-blocking start with async exit notification |
| bufio.NewReader ReadString | bufio.Scanner with Scanner.Buffer() | Go 1.6 (Scanner.Buffer added) | Size cap prevents OOM on binary output |
| Check LookPath before Start | Let cmd.Start() fail naturally | CONTEXT.md decision | Simpler, no TOCTOU race between check and exec |

**Deprecated/outdated:**
- `exec.LookPath` pre-check: unnecessary, cmd.Start() returns a descriptive error if command is not found. CONTEXT.md locks this decision.
- `os.FindProcess(pid)` for monitoring: not needed — we hold the `*exec.Cmd` reference; cmd.Wait() is the authoritative exit signal.

---

## Open Questions

1. **StopTimeout default: 5s or 10s?**
   - What we know: CONTEXT.md says "sane default", marks this as Claude's discretion.
   - What's unclear: Neither 5s nor 10s has a strong technical reason over the other.
   - Recommendation: Use 5 seconds. Matches typical container orchestrator defaults (Kubernetes uses 30s for pods, but for a local process manager, 5s is snappy). Can be overridden per-process via ProcessDef.StopTimeout.

2. **captureOutput handling of bufio.ErrTooLong**
   - What we know: Scanner stops scanning when a single token exceeds the max size. The CONTEXT.md decision says "truncate lines at a limit (e.g., 8KB)".
   - What's unclear: Scanner with Buffer(buf, 8KB) returns ErrTooLong and STOPS when a line is longer than 8KB — it doesn't truncate and continue. To get truncate-and-continue, a custom SplitFunc is needed.
   - Recommendation: Use a custom SplitFunc that truncates at 8KB and continues scanning. This is ~10 lines of code. If complexity is a concern, using Scanner.Buffer with the max and accepting early termination on binary data is acceptable for v1.1.

3. **Error type for ErrAlreadyRunning and ErrNotRunning**
   - What we know: CONTEXT.md says "exact error types and messages for edge cases" is Claude's discretion.
   - What's unclear: Should these be sentinel vars in scheduler.go or defined inline?
   - Recommendation: Add two sentinel vars to `scheduler.go`: `ErrAlreadyRunning = errors.New("process is already running")` and `ErrNotRunning = errors.New("process is not running")`. This matches the existing pattern of `ErrNotFound`, `ErrAlreadyExists`, `ErrNotStopped`.

4. **Where to store cmd *exec.Cmd on ManagedProcess**
   - What we know: It must be on ManagedProcess since Stop() needs pid from cmd.Process.Pid.
   - What's unclear: Should it be cleared after process exits?
   - Recommendation: Leave `mp.cmd` set after exit (non-nil) to allow post-mortem inspection. The monitor goroutine clears `mp.doneCh` to nil but not `mp.cmd`. This is fine — cmd is a struct ref, not a resource that needs releasing; cmd.ProcessState is available via mp.cmd.ProcessState.

---

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/os/exec` — Cmd struct, Start(), Wait(), StdoutPipe(), StderrPipe(), SysProcAttr
- `pkg.go.dev/syscall#SysProcAttr` — Setpgid, Pgid fields
- `pkg.go.dev/bufio#Scanner` — Scanner.Buffer(), Scan(), Text(), ScanLines behavior
- `pkg.go.dev/os#Process` — Process.Signal(), ProcessState.ExitCode(), ExitCode() returns -1 for signal-killed
- Existing Phase 5 code in `internal/scheduler/` — verified current struct shapes, mutex strategy, lock-ordering decision

### Secondary (MEDIUM confidence)
- `sigmoid.at/post/2023/08/kill_process_descendants_golang/` — Setpgid + syscall.Kill(-pgid) verified pattern; also verifies that the issue #53199 (syscall.Kill negative pgid) was a user error, the call does work correctly
- `bigkevmcd.github.io/go/pgrp/context/2019/02/19/terminating-processes-in-go.html` — Setpgid pattern and negative-pid kill pattern
- `mezhenskyi.dev/posts/go-linux-processes/` — SIGTERM then SIGKILL escalation pattern with polling
- `github.com/golang/go/issues/53199` — Confirmed: syscall.Kill(-pid, sig) does work correctly for process groups on Linux; the issue was user error

### Tertiary (LOW confidence)
- WebSearch results on goroutine monitor patterns — no single definitive source; conclusions drawn from combining multiple sources with verification against official docs

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Pure Go stdlib; os/exec, syscall, bufio are stable and verified against official docs
- Architecture: HIGH — Lock-before-exec pattern, monitor goroutine as single state owner, and process group kill are all documented in official sources and STATE.md's arch decisions
- Pitfalls: HIGH — Race conditions verified by reading Go issue tracker; Setpgid/orphan behavior verified by syscall docs
- Testing approach: HIGH — Same-package test pattern already established in Phase 5

**Research date:** 2026-03-01
**Valid until:** 2027-03-01 (Go stdlib APIs are extremely stable; these patterns will not change)
