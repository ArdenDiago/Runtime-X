# Roadmap: Runtime X (rtx)

## Overview

Three phases deliver a correct, deterministic process runner. Phase 1 proves the basic process lifecycle — spawn, stream I/O, propagate exit code, prevent zombies — without signal complexity. Phase 2 layers signal forwarding and graceful shutdown on top of the proven foundation. Phase 3 validates correctness across all edge cases with unit tests and manual verification. Each phase completes a verifiable capability before the next begins.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Process Foundation** - CLI entry point, process spawning, I/O streaming, exit code propagation, and error handling
- [x] **Phase 2: Signal Forwarding** - SIGINT/SIGTERM interception, forwarding to child, graceful shutdown sequence
- [ ] **Phase 3: Tests and Validation** - Unit tests, signal-killed exit code emulation, manual validation

## Phase Details

### Phase 1: Process Foundation
**Goal**: Users can run arbitrary commands via `rtx run` with correct I/O streaming and exact exit code propagation
**Depends on**: Nothing (first phase)
**Requirements**: CLI-01, CLI-02, PROC-01, PROC-02, PROC-03, PROC-04, PROC-05, EXIT-01, EXIT-02, ERR-01, ERR-02, LOG-01, LOG-03
**Success Criteria** (what must be TRUE):
  1. User runs `rtx run echo hello` and sees `hello` printed to stdout with PID displayed on stderr immediately after spawn
  2. User runs `rtx run sh -c 'exit 42'` and the shell reports exit code 42 from `echo $?`
  3. User runs `rtx run yes` and output appears line-by-line in real time (not buffered until the process ends)
  4. User runs `rtx run nonexistent-command` and sees a clear "command not found" message with exit code 127
  5. After any `rtx run` invocation completes, no zombie processes appear in `ps aux | grep Z`
**Plans**: 2 plans

Plans:
- [x] 01-01-PLAN.md — Process runner core package (internal/process/runner.go)
- [x] 01-02-PLAN.md — CLI entry point, binary build, and Phase 1 end-to-end verification

### Phase 2: Signal Forwarding
**Goal**: Users can interrupt or terminate `rtx`-managed processes and receive correct exit behavior in all signal scenarios
**Depends on**: Phase 1
**Requirements**: SIG-01, SIG-02, SIG-03, SIG-04, EXIT-03, ERR-03, LOG-02
**Success Criteria** (what must be TRUE):
  1. User presses Ctrl+C while `rtx run sleep 100` is running and sees `[rtx] received signal interrupt` logged to stderr before the process exits
  2. User sends SIGTERM to the `rtx` process and the child terminates cleanly with `rtx` exiting using the child's exit code
  3. User presses Ctrl+C during `rtx run sleep 100` and `rtx` exits with code 130 (128 + SIGINT signal number)
  4. User sends a signal to an already-dead child process and `rtx` handles it gracefully without crashing
**Plans**: 2 plans

Plans:
- [x] 02-01-PLAN.md — Signal interception, forwarding, and POSIX 128+N exit code emulation in runner.go
- [x] 02-02-PLAN.md — Binary rebuild and behavioral verification (signal log, exit codes 130/143, regressions)

### Phase 3: Tests and Validation
**Goal**: The `rtx` binary is verified correct across all edge cases by automated unit tests and manual validation
**Depends on**: Phase 2
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06
**Success Criteria** (what must be TRUE):
  1. Running `go test ./internal/process/...` passes with tests covering exit code 1, exit code 42, zombie prevention, signal delivery, and command-not-found (exit 127)
  2. Manual run of `rtx run yes` confirms output appears line-by-line with no buffering delay visible to the user
  3. All test cases in the "Looks Done But Isn't" checklist from PITFALLS.md pass without manual intervention
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Process Foundation | 2/2 | Complete   | 2026-02-28 |
| 2. Signal Forwarding | 2/2 | Complete    | 2026-02-28 |
| 3. Tests and Validation | 0/TBD | Not started | - |
