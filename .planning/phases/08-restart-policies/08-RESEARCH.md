# Phase 8: Restart Policies - Research

**Researched:** 2026-03-05
**Domain:** Process supervisor, exponential backoff, state machines
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| RST-01 | User can configure a process with restart-on-failure policy (restart when exit code != 0) | `RestartMode` already exists. `monitorProcess` needs to check exit code and policy. |
| RST-02 | Restart uses exponential backoff (initial delay, max delay, max retries configurable per process) | Formula: `min(Delay * (2 ^ RestartCount), MaxDelay)`. Need to add `MaxDelay` to `RestartPolicy`. |
| RST-03 | Restart attempts stop after reaching max retries — process status becomes "failed" | `RestartCount` already exists. `monitorProcess` or restart loop must check this. |
| RST-04 | User can stop a process during a backoff wait period (cancels pending restart) | `Stop()` must be able to cancel the sleep. Use `time.Timer` + `doneCh` or `select` with a dedicated cancel channel. |
</phase_requirements>

---

## Summary

Phase 8 implements automatic recovery for failed processes. This transforms `rtx` into a real process supervisor that can maintain uptime.

The core addition is the `StateRestarting` lifecycle state. When a process crashes, instead of moving directly to `StateFailed`, the scheduler evaluates the `RestartPolicy`. If a restart is warranted, the process enters `StateRestarting` and a goroutine is launched to wait for the backoff period before calling `Start()` again.

**Primary recommendation:**
1. Update `RestartPolicy` to include `MaxDelay` and `Factor`.
2. Add `StateRestarting` to the FSM.
3. Update `monitorProcess` to launch a `waitAndRestart` goroutine instead of just transitioning to `StateFailed`.
4. Update `Stop()` to handle `StateRestarting` by closing the backoff wait immediately.

---

## Standard Stack

### Core

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| `time.Timer` | Go Stdlib | Non-blocking sleep with cancellation | Allows `select` to wait for either the timeout or a stop signal. |
| `sync.Mutex` | Go Stdlib | Thread-safe state updates | Reuses existing `Scheduler.mu`. |

---

## Architecture Patterns

### State Machine Updates

New state: `StateRestarting`.

Valid transitions:
- `Running` → `Restarting`: On crash/exit if policy matches and retries not exhausted.
- `Restarting` → `Starting`: After backoff timer fires.
- `Restarting` → `Stopping`: When `Stop()` is called during wait.
- `Restarting` → `Failed`: If `MaxRetries` is reached (handled in `monitorProcess`).

### Pattern: Cancellable Backoff

Instead of `time.Sleep()`, we use a `select` block to wait for the backoff timer OR a stop signal.

```go
func waitAndRestart(s *Scheduler, mp *ManagedProcess) {
    // Calculate delay
    delay := calculateBackoff(mp)
    timer := time.NewTimer(delay)
    defer timer.Stop()

    select {
    case <-timer.C:
        // Proceed to Start()
        s.Start(mp.Def.Name)
    case <-mp.restartCancelCh:
        // Cancelled by Stop()
        return
    }
}
```

**Wait, `ManagedProcess` needs a way to cancel this.** I should use `mp.doneCh` or a new `restartCancelCh`. Since `doneCh` is closed by `monitorProcess` or created by `Stop()`, using it for both might be complex. A dedicated `restartCancelCh` is cleaner.

---

## Common Pitfalls

### Pitfall 1: Leaking Goroutines
If `Stop()` is called, the `waitAndRestart` goroutine must exit. If it doesn't check for cancellation, it will wait out the full backoff and then try to call `Start()`, which might fail or restart the process unexpectedly after it was stopped.

### Pitfall 2: Race on RestartCount
`RestartCount` must be updated correctly. It should be incremented *before* the backoff to calculate the delay for the *next* attempt.

### Pitfall 3: `Stop()` deadlocks
If `Stop()` is called while in `StateRestarting`, it must not wait for the process to exit (since it's already exited), but it MUST transition the state to `Stopped` and ensure no new process is spawned.

---

## Sources
- [Go by Example: Timers](https://gobyexample.com/timers)
- [Exponential Backoff Algorithm](https://en.wikipedia.org/wiki/Exponential_backoff)
