# Requirements: Runtime X (rtx)

**Defined:** 2026-02-28
**Core Value:** Correct, deterministic process lifecycle management — no zombies, no orphans, exact exit codes, clean signal forwarding.

## v1 Requirements

Requirements for v0 release. Each maps to roadmap phases.

### CLI

- [ ] **CLI-01**: User can run `rtx run <command> [args...]` to spawn a child process
- [ ] **CLI-02**: User sees PID displayed immediately after process spawns

### Process Execution

- [ ] **PROC-01**: Child process is spawned via `cmd.Start()` (not `cmd.Run()`) to allow concurrent signal handling
- [ ] **PROC-02**: Child process stdout streams to parent stdout in real-time (direct fd assignment, no buffering)
- [ ] **PROC-03**: Child process stderr streams to parent stderr in real-time (direct fd assignment, no buffering)
- [ ] **PROC-04**: Child process is always reaped via `cmd.Wait()` on every code path — no zombie processes
- [ ] **PROC-05**: Child process runs in its own process group (`Setpgid: true`) for clean signal interposition

### Exit Codes

- [ ] **EXIT-01**: Parent captures child's exact exit code via `ExitError.ExitCode()`
- [ ] **EXIT-02**: Parent exits with child's exact exit code via `os.Exit(code)`
- [ ] **EXIT-03**: Signal-killed child produces correct POSIX exit code (128 + signal number)

### Signal Handling

- [ ] **SIG-01**: Parent intercepts SIGINT and forwards it to child process
- [ ] **SIG-02**: Parent intercepts SIGTERM and forwards it to child process
- [ ] **SIG-03**: Graceful shutdown: forward signal → wait for child to finish → exit with child's code
- [ ] **SIG-04**: Signal channel is buffered (capacity 1) to prevent dropped signals

### Error Handling

- [ ] **ERR-01**: "Command not found" produces clear error message and exits with code 127
- [ ] **ERR-02**: Child that crashes immediately has its exit code propagated as-is
- [ ] **ERR-03**: Signal forwarding to already-dead process is handled gracefully (swallow `os.ErrProcessDone`)

### Logging

- [ ] **LOG-01**: Minimal logging to stderr: `[rtx] spawned PID %d` on start
- [ ] **LOG-02**: Minimal logging to stderr: `[rtx] received signal %s` on signal
- [ ] **LOG-03**: Minimal logging to stderr: `[rtx] exited with code %d` on exit

### Testing

- [ ] **TEST-01**: Unit test: `rtx run false` returns exit code 1
- [ ] **TEST-02**: Unit test: `rtx run sh -c 'exit 42'` returns exit code 42
- [ ] **TEST-03**: Unit test: process spawning does not leave zombie processes
- [ ] **TEST-04**: Unit test: signal forwarding delivers signal to child
- [ ] **TEST-05**: Unit test: "command not found" returns exit code 127
- [ ] **TEST-06**: Manual validation: `rtx run yes` outputs line-by-line (real-time, not buffered)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Restart Policies

- **RST-01**: User can configure restart-on-failure behavior
- **RST-02**: User can configure backoff strategy for restarts

### Multi-Process

- **MULTI-01**: User can run multiple processes with dependency ordering
- **MULTI-02**: User can define process groups with shared lifecycle

### Configuration

- **CFG-01**: User can define process configuration via YAML/TOML file
- **CFG-02**: User can set timeout/watchdog for child process

## Out of Scope

| Feature | Reason |
|---------|--------|
| Config files / YAML parsing | v0 philosophy: earn trust through correctness first |
| Daemon mode / background mode | Not needed for process correctness proof |
| Restart policies | v1+ concern — requires understanding real failure patterns |
| Multiple process support | Single process focus for v0 correctness proof |
| State persistence | In-memory PID only during the run |
| Lifecycle FSM | Keep it simple — no state machine needed |
| Web UI / HTTP API | Docker system handles this |
| Metrics / Observability | Minimal logging only — no Prometheus/OpenTelemetry |
| Containers / Plugins | Future versions |
| Signal rewriting / remapping | Forward as-is; dumb-init handles this if needed |
| Structured/JSON logging | Plain stderr logging sufficient for v0 |
| Shell wrapping (sh -c) | Direct exec.Command for correctness |
| StdoutPipe/StderrPipe goroutines | Race condition risk — use direct fd assignment |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| (populated by roadmapper) | | |

**Coverage:**
- v1 requirements: 22 total
- Mapped to phases: 0
- Unmapped: 22 (pending roadmap)

---
*Requirements defined: 2026-02-28*
*Last updated: 2026-02-28 after initial definition*
