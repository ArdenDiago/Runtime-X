# Phase 6: Scheduler Start, Stop, and Lifecycle - Context

**Gathered:** 2026-03-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement `Start()` and `Stop()` on the Scheduler so registered processes actually run as OS processes. PID tracking, state transitions (Starting/Running/Stopping/Stopped/Failed), output capture to the logBuffer from Phase 5, and zombie prevention. Dependency ordering is Phase 7, restart policies are Phase 8, graceful shutdown is Phase 10.

</domain>

<decisions>
## Implementation Decisions

### Stop signal behavior
- SIGTERM first, then escalate to SIGKILL after a timeout — guarantees the process dies
- Stop timeout is configurable per process via a `StopTimeout` field on ProcessDef, with a sane default
- Stop() blocks until the process has fully exited and state is updated — caller knows it's done when Stop() returns
- Send signals to the entire process group (using process group / setpgid) — prevents orphan child processes

### Start failure handling
- Failed start transitions to StateFailed with error stored — user can see why and retry
- Process that exits on its own: exit code 0 -> Stopped, non-zero -> Failed — distinguishes clean exit from crash
- Start() is callable from Idle, Stopped, or Failed states — re-startable without Remove+Register ceremony
- Let exec fail naturally — don't pre-check with LookPath. Handle the error from cmd.Start()

### Output capture wiring
- Line-buffered using bufio.Scanner — each LogEntry is one line of output, clean for display
- Continue capturing output until EOF (process exit) — captures shutdown messages and cleanup output
- Separate goroutines for stdout and stderr — one goroutine per stream, each writing to shared logBuffer with its stream tag
- Truncate lines at a limit (e.g., 8KB) to prevent memory issues from binary data dumps

### Concurrent start/stop safety
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

</decisions>

<specifics>
## Specific Ideas

- Process group kill pattern: use `syscall.SysProcAttr{Setpgid: true}` on the exec.Cmd, then `syscall.Kill(-pid, signal)` to signal the group
- The monitor goroutine (waiting for process exit) should be the single source of truth for state transitions — avoids races between Stop() and natural exit
- Start() should reset runtime metadata (PID, ExitCode, StartedAt, etc.) when re-starting from Stopped/Failed

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-scheduler-start-stop-and-lifecycle*
*Context gathered: 2026-03-01*
