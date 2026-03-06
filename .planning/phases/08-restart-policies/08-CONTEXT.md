# Phase 8: Restart Policies - Context

## Objective
Implement automatic process restarts with exponential backoff and explicit cancellation.

## Phase Constraints
- **State Machine Integrity**: All transitions must be valid and race-free.
- **Zero Orphans**: Restarts must not spawn multiple instances of the same process.
- **Cancellation**: `Stop()` must immediately cancel any pending restart backoff.

## Known Dependencies
- `internal/scheduler/types.go` (existing FSM and types)
- `internal/scheduler/lifecycle.go` (existing Start/Stop/Monitor logic)

## Strategic Decisions
- **`StateRestarting`**: A dedicated state for backoff periods.
- **`restartCancelCh`**: A dedicated channel in `ManagedProcess` to signal the backoff timer to stop.
- **Deterministic Backoff**: `Delay * (2 ^ RestartCount)`, capped at `MaxDelay`.
- **Atomic State Updates**: All state changes must occur within `Scheduler.mu`.

## Success Criteria
- [ ] A crashed process (non-zero exit) restarts automatically if policy is `on-failure`.
- [ ] A clean exit (zero exit) restarts automatically if policy is `always`.
- [ ] Restart delay increases exponentially (1s, 2s, 4s...) until `MaxDelay`.
- [ ] After `MaxRetries`, the process transitions to `StateFailed`.
- [ ] `Stop()` while in `StateRestarting` moves state to `StateStopped` and cancels the restart.
