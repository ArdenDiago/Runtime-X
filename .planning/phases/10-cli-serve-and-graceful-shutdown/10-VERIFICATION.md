# Phase 10: CLI serve and Graceful Shutdown - Verification

## Task Completion Status

| Task | Plan | Status |
|------|------|--------|
| Refactor `main.go` for subcommands | 10-01 | Pending |
| Implement `rtx serve` logic | 10-01 | Pending |
| Implement `scheduler.StopAll()` | 10-02 | Pending |
| Implement Graceful Shutdown | 10-02 | Pending |

## Acceptance Criteria Verification

| ID | Requirement | Test Result |
|----|-------------|-------------|
| CMD-01 | `rtx serve` starts API and frontend | Pending |
| CMD-02 | `rtx serve` handles SIGTERM/SIGINT | Pending |
| CMD-03 | `rtx run` is backward compatible | Pending |

## Automated Verification

### Unit Tests
- `TestStopAll()` in `internal/scheduler/lifecycle_test.go`
- Other subcommand-related tests if possible.

### Integration Tests
- Verify that `rtx serve` starts up and responds to `curl`.
- Verify that processes started via `rtx serve` are terminated when `rtx serve` is stopped.

## Manual Verification Log

- [ ] Check `rtx run echo "hello"` works.
- [ ] Check `rtx serve` starts on default port (8080).
- [ ] Check `rtx serve -port 9000` works.
- [ ] Check `curl localhost:8080/api/processes` returns 200.
- [ ] Check `Ctrl+C` logs "Shutting down..." and "Stopping all processes...".
- [ ] Check `ps aux` after shutdown to ensure no orphans.
