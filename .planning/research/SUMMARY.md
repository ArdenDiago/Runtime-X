# Project Research Summary

**Project:** Runtime-X / rtx
**Domain:** Go process runner CLI (stdlib-only, Linux-first)
**Researched:** 2026-02-27
**Confidence:** HIGH

## Executive Summary

The `rtx` binary is a transparent process runner CLI in the tradition of `tini` — its sole responsibility is to spawn one child process, stream its I/O without buffering, forward SIGINT and SIGTERM, reap the child to prevent zombies, and propagate the child's exact exit code. The closest production analog is `tini` (krallin/tini), but implemented as a Go CLI rather than a PID-1 init process. Research across all four domains is in strong agreement: the correct approach is a minimal, stdlib-only implementation using `cmd.Start()` + `cmd.Wait()` with a signal-forwarding goroutine and direct `os.Stdout`/`os.Stderr` file descriptor inheritance. No external dependencies are needed or appropriate.

The recommended implementation is two Go packages: `internal/process/` (the process lifecycle logic) and `cmd/rtx/` (the CLI entry point). This follows the existing project convention of one `cmd/` subdirectory per binary and keeps all OS-process logic isolated from the Docker orchestration system already in the codebase. The entire feature set for v0 fits in roughly 60-80 lines of Go using only `os/exec`, `os/signal`, `syscall`, `os`, and `fmt`. The canonical implementation loop is already verified in STACK.md.

The primary risks are all implementation pitfalls, not design unknowns. The most dangerous are: (1) using `cmd.Run()` instead of `cmd.Start()` + `cmd.Wait()`, which makes signal forwarding impossible; (2) forgetting to call `cmd.Wait()` on every code path, which creates zombie processes; and (3) using an unbuffered signal channel, which silently drops signals. All three pitfalls are caught early and avoided with the patterns in STACK.md. Research confidence is HIGH across all four domains because every recommendation traces to official Go stdlib documentation.

---

## Key Findings

### Recommended Stack

The entire implementation uses only Go's standard library on Go 1.25.5 (the project's existing go.mod version). No changes to `go.mod` are required. The core packages are `os/exec` for process lifecycle, `os/signal` for signal interception, `syscall` for Linux-specific process attributes and signal constants, `os` for file descriptor wiring and exit, and `flag` for CLI argument parsing.

The critical version-specific note: use `(*exec.ExitError).ExitCode()` (Go 1.12+) rather than the legacy `syscall.WaitStatus.ExitStatus()` approach. `signal.NotifyContext` (Go 1.16+) is available but not needed for v0's single-process, foreground execution model.

**Core technologies:**
- `os/exec`: Spawn and manage the child process — `cmd.Start()` + `cmd.Wait()` split pattern is mandatory for signal interception.
- `os/signal`: Intercept SIGINT and SIGTERM with a buffered channel — `signal.Notify(ch, ...)` where `ch` has capacity 1.
- `syscall`: Linux-specific `SysProcAttr{Setpgid: true}` for process group isolation; signal constants; `WaitStatus` for signal-killed exit code emulation.
- `os`: Wire `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr`, `cmd.Stdin = os.Stdin` for zero-copy, unbuffered I/O passthrough; `os.Exit(code)` for exit code propagation.
- `flag`: Parse `rtx run <cmd> [args...]` — one subcommand, no external CLI library needed.

See `.planning/research/STACK.md` for the complete verified implementation loop and alternatives analysis.

### Expected Features

The v0 feature surface is deliberately narrow. All P1 features are required for correctness — missing any one of them means the process runner is broken, not incomplete. P2 features add polish but are not blocking.

**Must have (table stakes / P1):**
- Process spawning via `cmd.Start()` (not `cmd.Run()`) — required for concurrent signal forwarding
- PID display on spawn — `[rtx] spawned PID %d` to stderr immediately after Start()
- Real-time stdout/stderr passthrough via direct `cmd.Stdout = os.Stdout` assignment — unbuffered, race-free
- SIGINT + SIGTERM interception and forwarding to child process
- Graceful shutdown: forward signal, block on `cmd.Wait()`, exit with child's code
- Zombie prevention: `cmd.Wait()` called on every code path after a successful `cmd.Start()`
- Exit code capture and propagation: `*exec.ExitError.ExitCode()` → `os.Exit(code)`
- "Command not found" handling: detect `exec.ErrNotFound`, emit clear message, exit 127
- Minimal logging to stderr: PID, signal received, exit code

**Should have (competitive / P2):**
- Signal-killed exit code emulation: `128 + signal_number` (e.g., 130 for SIGINT) to match shell behavior
- Process group isolation: `Setpgid: true` + explicit signal forwarding for clean interposition
- Unit tests for exit code propagation, zombie prevention, signal routing

**Defer (v1+):**
- Restart policies — requires understanding of real failure patterns first
- Multi-process support — dependency graph design needed
- Config file (YAML/TOML) — format and schema decisions not ready
- Timeout / watchdog — graceful kill sequence design needed
- Daemon mode — PID file, log redirect, double-fork out of scope

The closest analog is `tini`, which does exactly what v0 targets: spawn one child, forward signals, reap zombies, propagate exit code. rtx v0 deliberately avoids everything `supervisord` adds (config files, restart policies, web UI, multi-process).

See `.planning/research/FEATURES.md` for the full feature dependency graph and competitor analysis.

### Architecture Approach

The architecture is two-layer. `cmd/rtx/main.go` handles argument parsing and `os.Exit()`. `internal/process/runner.go` contains all process lifecycle logic (spawn, stream, signal, wait, exit code). These packages share no state and have no coupling to the existing Docker orchestration packages (`internal/worker/`, `internal/core/`, `internal/queue/`). The boundary is enforced by design: the process runner is a standalone, self-contained unit.

**Major components:**
1. `cmd/rtx/main.go` (CLI Layer) — Parse `rtx run <cmd> [args...]`, validate args, delegate to `process.Run()`, call `os.Exit()` with returned code
2. `internal/process/runner.go` (Executor + Signal Handler + Output Streamer) — `exec.Command` lifecycle, `signal.Notify` goroutine loop, direct fd wiring, exit code extraction
3. OS Process Layer — child process inherits parent's stdout/stderr file descriptors; signals forwarded via `cmd.Process.Signal(sig)`

Build order is forced by the dependency graph: `internal/process/` first (no external deps), `cmd/rtx/main.go` second (depends on process package only), tests third (alongside each component). The project adds a third binary alongside `bin/api` and `bin/worker`.

See `.planning/research/ARCHITECTURE.md` for the full data flow diagram, component boundary table, and anti-pattern analysis.

### Critical Pitfalls

All 7 pitfalls documented in PITFALLS.md are known failure modes with verified prevention strategies. The top 5 by severity:

1. **Calling `cmd.Run()` instead of `cmd.Start()` + `cmd.Wait()`** — `Run()` blocks and makes signal forwarding impossible. Prevention: always use the Start/Wait split pattern with a signal goroutine between them.

2. **Missing `cmd.Wait()` on any code path after `cmd.Start()`** — Creates zombie processes that persist in the process table. Prevention: structure the runner so `cmd.Wait()` is called unconditionally; prefer a goroutine sending to `doneCh` and a select that always drains `doneCh` before returning.

3. **Unbuffered signal channel** — `make(chan os.Signal)` (capacity 0) silently drops signals; `signal.Notify` does a non-blocking send. Prevention: `make(chan os.Signal, 1)`. `go vet` catches this — run it.

4. **Exit code swallowed — always returning 0 or 1** — Type-asserting `cmd.Wait()` error as generic `error` loses the real exit code. Prevention: `errors.As(err, &exitErr)` then `exitErr.ExitCode()`. Never `os.Exit(1)` on a non-nil `cmd.Wait()` error without checking first.

5. **Process group conflict — child receives signal twice or zero times** — Default process group: kernel delivers Ctrl+C to both runner and child simultaneously; `cmd.Process.Signal(sig)` then hits an already-dead process. Setpgid: child never gets the signal unless explicitly forwarded. Prevention: decide the model upfront. For rtx (transparent forwarder without Setpgid), swallow `os.ErrProcessDone` from `Signal()`. For rtx with `Setpgid: true`, explicit forwarding is mandatory.

Secondary pitfalls to address at implementation time:
- `os.Exit()` skips deferred functions — use the inner-function-returns-int pattern in `main()`
- `cmd.Wait()` can block forever if a grandchild inherits the pipe — avoided entirely by using direct `*os.File` assignment instead of `StdoutPipe()`

See `.planning/research/PITFALLS.md` for the complete pitfall-to-phase mapping and "Looks Done But Isn't" checklist.

---

## Implications for Roadmap

Based on research, the component dependency chain dictates a clear build order. There are no ambiguous sequencing decisions — the architecture forces the order. Three phases are sufficient for v0.

### Phase 1: Process Execution Foundation

**Rationale:** `cmd/rtx/main.go` cannot be built until `internal/process/` exists. The output streamer and exit code extractor are prerequisites for signal handling (you need a running process before you can forward signals to it). This phase proves the basic process lifecycle is correct before adding signal complexity.

**Delivers:** A working `rtx run <cmd> [args...]` binary that spawns a child, streams I/O in real time, captures exit code, propagates it to the shell, and prevents zombies. No signal handling yet — Ctrl+C kills both parent and child via the kernel (default behavior), which is acceptable for this phase.

**Addresses (from FEATURES.md P1):**
- Process spawning via `cmd.Start()`
- Real-time stdout/stderr passthrough (direct fd assignment)
- Zombie prevention (`cmd.Wait()` on all paths)
- Exit code capture and propagation
- "Command not found" error handling (exit 127)
- PID display and minimal logging
- CLI entry point (`rtx run <cmd> [args...]`)

**Avoids (from PITFALLS.md):**
- `cmd.Run()` anti-pattern (use Start/Wait split from the start)
- Missing `cmd.Wait()` on error paths
- `os.Exit()` skipping defer (inner-function-returns-int pattern in main)
- `cmd.Wait()` blocking on grandchild pipe (direct `*os.File` assignment from day one)
- Exit code swallowing (ExitError type assertion built in from the start)

**Research flag:** Standard patterns — no deeper research needed. The canonical implementation is fully specified in STACK.md (Key Implementation Pattern section).

---

### Phase 2: Signal Forwarding and Graceful Shutdown

**Rationale:** Signal handling depends on Phase 1 producing a running `cmd.Process`. The signal goroutine pattern is independent enough to layer on top of the Phase 1 foundation without refactoring. The process group isolation decision (Setpgid vs. no Setpgid) must be made before coding this phase.

**Delivers:** Ctrl+C and SIGTERM are intercepted by `rtx`, forwarded to the child, and `rtx` waits for the child to finish its cleanup before exiting. The log line "received SIGINT, forwarding to PID X" appears before child exit. `rtx` exits with the child's exit code in all signal scenarios.

**Addresses (from FEATURES.md P1/P2):**
- SIGINT + SIGTERM interception and forwarding
- Graceful shutdown sequence (forward → wait → exit with child's code)
- Process group isolation decision (Setpgid: true recommended for observable logging)

**Avoids (from PITFALLS.md):**
- Unbuffered signal channel (use `make(chan os.Signal, 1)`)
- Signal not forwarded to child (explicit `cmd.Process.Signal(sig)`)
- Process group conflict (deliberate design decision: Setpgid: true + explicit forwarding)
- Signal forwarding to already-dead process (swallow `os.ErrProcessDone`)

**Design decision required before coding:** Whether to use `Setpgid: true` (recommended by STACK.md for observable behavior) or rely on default process group inheritance. STACK.md recommends Setpgid: true for v0. If chosen, explicit signal forwarding is mandatory — the OS no longer auto-delivers Ctrl+C to the child.

**Research flag:** Standard patterns — the signal goroutine loop and Setpgid decision are fully specified in STACK.md and ARCHITECTURE.md. No additional research needed.

---

### Phase 3: Polish, Tests, and Validation

**Rationale:** Once Phase 1 and Phase 2 are complete, the core correctness proof is done. Phase 3 adds the "Looks Done But Isn't" verification, signal-killed exit code emulation (128+N), and test coverage that makes the runner trustworthy for production use.

**Delivers:** A production-ready `rtx` binary with verified behavior across all edge cases, unit tests that serve as regression guards, and documented manual test results for the PITFALLS.md checklist.

**Addresses (from FEATURES.md P2):**
- Signal-killed exit code emulation (128 + signal_number)
- Unit tests: exit code propagation, zombie prevention, signal routing
- Manual validation against "Looks Done But Isn't" checklist

**Test targets:**
- `rtx run false` → `echo $?` returns exactly `1`
- `rtx run sh -c 'exit 42'` → returns exactly `42`
- `rtx run sleep 100` + Ctrl+C → child exits, `ps aux | grep Z` shows no zombie, runner exits with correct code
- `rtx run yes` → output appears line-by-line (real-time, not buffered)
- `rtx run nonexistent-command` → "command not found" message, exit 127

**Research flag:** Standard patterns — test fixtures using `exec.Command("true")`, `exec.Command("false")`, `exec.Command("sleep", "10")` are specified in STACK.md. Signal-killed exit code emulation using `syscall.WaitStatus.Signaled()` is documented in FEATURES.md.

---

### Phase Ordering Rationale

- **Phase 1 before Phase 2** is forced by the dependency graph: signal forwarding requires `cmd.Process`, which only exists after `cmd.Start()` succeeds. The output streamer and zombie prevention must be in place first, or signal testing is unreliable.
- **Phase 2 before Phase 3** is logical: you cannot validate signal behavior until the signal handler exists. The "Looks Done But Isn't" checklist tests features from both Phase 1 and Phase 2 together.
- **No Phase requires redesign** from a previous Phase: each phase layers on top of the previous one. Phase 2 adds a goroutine and a select statement to Phase 1's Start/Wait loop. Phase 3 adds tests.
- **This avoids the primary pitfall sequence:** building signal handling before proving basic lifecycle correctness (Phase 1) is a common source of compound bugs.

### Research Flags

Phases with standard patterns (skip additional research):
- **Phase 1:** Fully specified. The exact implementation is in STACK.md "Key Implementation Pattern." No design unknowns.
- **Phase 2:** Fully specified. The signal goroutine pattern and Setpgid decision are documented in STACK.md "Stack Patterns by Variant" and ARCHITECTURE.md "Pattern 2."
- **Phase 3:** Fully specified. Test fixtures and validation checklist are in PITFALLS.md.

No phases require a `/gsd:research-phase` call. All research is complete and HIGH confidence.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Every recommendation traces to official pkg.go.dev documentation. No external dependencies means no version compatibility risk. The canonical implementation loop is verified against Go 1.25.5 release notes. |
| Features | HIGH | Feature set grounded in os/exec official docs, tini/dumb-init source analysis, and Go stdlib behavior. The tini analog establishes clear table-stakes definition. |
| Architecture | HIGH | Based on official Go stdlib docs + direct read of the Runtime-X codebase (internal/worker/runner.go, .planning/codebase/ARCHITECTURE.md). Package boundaries match existing project conventions exactly. |
| Pitfalls | HIGH | Primary sources include official Go issue tracker entries (tracked bugs, not blog speculation). `go vet` catches the signal channel pitfall automatically. All 7 pitfalls have verified prevention strategies. |

**Overall confidence:** HIGH

### Gaps to Address

- **Process group decision:** STACK.md recommends `Setpgid: true` for v0. PITFALLS.md suggests that for a transparent forwarder, NOT using Setpgid may be simpler (child receives signal from kernel + from runner, but the runner should swallow `os.ErrProcessDone`). Both approaches are documented. Implementer must choose before coding Phase 2. Recommendation: start with `Setpgid: true` as specified in STACK.md for observable logging behavior; revisit if testing reveals issues.

- **Signal-killed exit code emulation (Phase 3):** `(*exec.ExitError).ExitCode()` returns -1 when the process was killed by a signal. Emitting `128 + signal_number` requires inspecting `cmd.ProcessState.Sys().(syscall.WaitStatus)`. This is a known P2 feature, not a gap in understanding — it needs implementation and testing but no additional research.

- **Windows support:** Out of scope for v0. `SIGINT` is not sendable to other processes on Windows (Go issue #6720). Linux is the declared first-class target. Build tags should gate the `syscall.SysProcAttr{Setpgid: true}` field assignment if cross-platform support is added post-v0.

---

## Sources

### Primary (HIGH confidence)
- `https://pkg.go.dev/os/exec` — Cmd struct, Start/Wait lifecycle, signal handling, zombie prevention, ExitError
- `https://pkg.go.dev/os/signal` — Notify, Stop, NotifyContext signatures and buffered channel requirement
- `https://pkg.go.dev/os#Process` — Process.Signal, ProcessState.ExitCode
- `https://pkg.go.dev/syscall` — SysProcAttr, WaitStatus, signal constants
- `https://pkg.go.dev/flag` — Args(), FlagSet for subcommands
- `https://go.dev/doc/go1.25` — Confirmed no breaking changes to os/exec, os/signal, syscall
- Go issue #52580 — cmd.Wait must be called (zombie prevention mandate)
- Go issue #45604 — go vet catches unbuffered os.Signal channel
- Go issue #6720, #28498 — SIGINT not sendable on Windows (Linux-first rationale)
- `https://github.com/krallin/tini/blob/master/README.md` — Defines minimal process runner feature set (the tini analog)
- Runtime-X codebase direct analysis — internal/worker/runner.go, .planning/codebase/ARCHITECTURE.md

### Secondary (MEDIUM confidence)
- `https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/` — Process group isolation, signal handling, pipe lifecycle patterns
- `https://victoriametrics.com/blog/go-graceful-shutdown/` — Graceful shutdown patterns, NotifyContext tradeoffs
- `https://mezhenskyi.dev/posts/go-linux-processes/` — Linux process management, zombie prevention, Setpgid
- `https://engineeringblog.yelp.com/2016/01/dumb-init-an-init-for-docker.html` — Signal forwarding and zombie reaping patterns (dumb-init comparison)
- `https://kevin.burke.dev/kevin/proxying-to-a-subcommand-with-go/` — Start/Wait/signal-goroutine pattern, syscall.Exec alternative
- `https://github.com/Netflix/signal-wrapper/blob/master/main.go` — Production signal forwarding reference implementation

### Tertiary (LOW confidence)
- `https://medium.com/@AlexanderObregon/signal-handling-in-go-applications-b96eb61ecb69` — Signal handling overview (single source, not official)

---

*Research completed: 2026-02-27*
*Ready for roadmap: yes*
