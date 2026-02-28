# Feature Research

**Domain:** User-space process runner CLI (Go)
**Researched:** 2026-02-27
**Confidence:** HIGH — features grounded in os/exec official docs, tini/dumb-init source analysis, and Go stdlib behavior

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = the process runner is broken, not incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Process spawning via `exec.Command` | Core function — it's a process runner | LOW | Use `cmd.Start()` not `cmd.Run()` so parent can remain in control; args passed verbatim, no shell wrapping |
| PID display on spawn | Every process runner shows PID immediately; used for external kill, debugging, logging | LOW | `cmd.Process.Pid` available after `cmd.Start()` returns |
| Real-time stdout passthrough | Buffered output is unusable for long-running commands; users expect to see output as it happens | LOW | Set `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr` — direct assignment, no goroutines, no pipes; this is the simplest correct approach |
| Real-time stderr passthrough | Same as stdout — stderr must flow immediately (build errors, crash logs) | LOW | Set `cmd.Stderr = os.Stderr` — same pattern as stdout |
| Exit code capture | Shell scripts, CI, orchestrators all check exit codes; returning 0 on failure is a critical bug | LOW | After `cmd.Wait()`, cast error to `*exec.ExitError`, call `.ExitCode()`, then `os.Exit(exitCode)` |
| Exit code propagation to parent | `rtx` itself must exit with the child's code — not always 0, not always 1 | LOW | `os.Exit(exitCode)` with the captured code; `os.Exit(0)` on clean success |
| SIGINT forwarding to child | Ctrl-C in terminal sends SIGINT to entire process group by default; must be intercepted and explicitly forwarded if process groups differ | MEDIUM | `signal.Notify` on `os.Interrupt`; send to `cmd.Process` via `cmd.Process.Signal(sig)` |
| SIGTERM forwarding to child | Process managers (systemd, Docker, Kubernetes) send SIGTERM for graceful stop | MEDIUM | Same pattern as SIGINT — `signal.Notify(c, syscall.SIGTERM)` and forward |
| Graceful shutdown sequence | Forward signal → wait for child → exit with child's code; killing child abruptly defeats the purpose | MEDIUM | After forwarding signal, block on `cmd.Wait()`; do NOT call `cmd.Process.Kill()` unless timeout exceeded |
| Zombie prevention via `cmd.Wait()` | On POSIX, if a parent never calls `wait()`, the exited child stays in the process table as a zombie indefinitely — resource leak, PID table exhaustion | LOW | `cmd.Wait()` is **mandatory** after `cmd.Start()` per Go issue #52580 and POSIX semantics; must be called even if the child crashes immediately |
| "Command not found" error handling | `exec.LookPath` returns `exec.ErrNotFound` when binary doesn't exist in PATH; must surface a clear error, not a panic | LOW | Check error from `exec.Command`/`exec.LookPath` before `cmd.Start()`; print user-friendly message, exit with code 1 (or 127 to match shell convention) |
| Invalid args / immediate crash handling | Child may exit with code 1 or 2 immediately; runner must not hang or swallow the error | LOW | `cmd.Wait()` returns `*exec.ExitError` — propagate exit code as-is; no special-casing needed |
| Minimal structured logging | PID on start, signal received, exit code on exit — operators need to know what happened | LOW | `log.Printf` or `fmt.Fprintf(os.Stderr, ...)` — no JSON, no log levels, no timestamps required in v0 |

### Differentiators (Competitive Advantage)

Features that set rtx apart in the process runner space. Not expected for table stakes, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Exact exit code transparency | Most naive wrappers return 1 for any failure; rtx propagates the precise code (2, 125, 126, 127, 130, etc.) | LOW | Already required for table stakes — doing it precisely (including signal-killed codes) is the differentiator; `cmd.ProcessState.ExitCode()` returns -1 for signal-killed — handle with `syscall.WaitStatus` if needed |
| Signal-killed exit code emulation | If child is killed by SIGTERM, a proper runner exits with `128+signal_number` (e.g., 130 for SIGINT) to match shell behavior | MEDIUM | Check `cmd.ProcessState.Sys().(syscall.WaitStatus).Signaled()` and compute `128 + int(ws.Signal())` |
| No buffering guarantee | Many runners buffer output, breaking streaming tools (progress bars, log tails); direct fd assignment never buffers | LOW | Direct `cmd.Stdout = os.Stdout` assignment inherits OS-level buffering behavior; contrast with `StdoutPipe()` which adds Go-level buffering |
| Process group isolation option | Prevents Ctrl-C from killing child before rtx can clean up; allows rtx to control shutdown order | MEDIUM | `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` — note: with this set, Ctrl-C NO LONGER auto-kills child; rtx MUST forward the signal explicitly |
| Clean error messages for operator errors | "command not found: foo" instead of "exec: no such file or directory" stack trace | LOW | Detect `exec.ErrNotFound`, emit human-readable message, use exit code 127 (POSIX convention for command not found) |

### Anti-Features (Things to Deliberately NOT Build in v0)

Features that seem good but create scope, complexity, or design debt disproportionate to v0's goal.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Restart policies (autorestart, backoff) | Adds state machine complexity; v0 is about running once correctly — restart logic changes the failure model entirely | Let caller (shell script, systemd, k8s) handle restarts; rtx exits with exact code so callers can decide |
| Config files / YAML parsing | Config parsing brings format decisions, validation, versioning; v0 must earn trust through correct bare-metal behavior first | Accept command and args from CLI flags only: `rtx run <cmd> [args...]` |
| Daemon / background mode | Daemonizing requires PID files, log redirection, session management, and double-fork; orthogonal to correctness proof | Use `&` in shell or a real init system (systemd) for background execution |
| Multi-process management | Multiple processes need a dependency graph, startup ordering, and group signaling; complexity explosion for v0 | Single process only; add multi-process in v1+ after v0 is proven |
| State persistence (beyond in-memory PID) | Writing PID files, databases, or state files adds failure modes (stale PID files, disk full, permissions) | Track PID in-memory only during the run; it disappears on exit by design |
| Web UI / HTTP control API | A control API requires auth, a server lifecycle, port management — completely orthogonal to process correctness | Docker orchestration system in existing codebase already handles HTTP API |
| Structured/JSON logging | JSON logging requires log library decisions, field naming conventions, log levels — premature for v0 | Plain `fmt.Fprintf(os.Stderr, ...)` with PID, signal, exit code only |
| Metrics / Observability frameworks | Prometheus, OpenTelemetry, etc. add external dependencies; v0 is stdlib-only by constraint | Minimal stderr logging; metrics are v2+ concern |
| Signal rewriting / remapping | dumb-init offers this; it adds a mapping config and a new failure mode (wrong mapping) | Forward signals as-is; callers that need rewriting use dumb-init in front of rtx |
| Timeout / watchdog | Forces decisions about what to do on timeout (kill, SIGKILL, log); adds complexity and new error states | Let callers use `timeout(1)` from coreutils or implement in v1+ |
| StdoutPipe / StderrPipe goroutine pattern | Using `StdoutPipe()` then copying in goroutines introduces a race condition detected by `go test -race` and is explicitly warned against in the os/exec docs | Assign `cmd.Stdout = os.Stdout` directly — simpler, race-free, no goroutines needed |
| Shell wrapping (sh -c) | Shell wrapping changes argument quoting semantics, introduces a shell PID between rtx and the real process, and breaks exact signal delivery to the target | Use `exec.Command(args[0], args[1:]...)` directly — no shell |
| Isolation / namespaces / cgroups | Container-level isolation; requires root or capabilities; completely out of scope for a user-space CLI | Docker handles isolation; rtx runs inside the existing Docker job system |

---

## Feature Dependencies

```
Process Spawning (cmd.Start)
    └──requires──> Zombie Prevention (cmd.Wait)
                       └──required for──> Exit Code Capture (ExitError.ExitCode)
                                              └──required for──> Exit Code Propagation (os.Exit)

Signal Interception (signal.Notify SIGINT/SIGTERM)
    └──requires──> Process Spawning (need cmd.Process to forward to)
    └──enhances──> Graceful Shutdown (forward → wait → exit with child code)

Process Group Isolation (Setpgid: true)
    └──requires──> Signal Interception (without interception, Ctrl-C won't reach child at all)
    └──enables──> Graceful Shutdown (rtx controls when child dies)

Stdout/Stderr Passthrough
    └──requires──> Process Spawning (need cmd before setting Stdout/Stderr)
    └──conflicts──> StdoutPipe/StderrPipe pattern (mutually exclusive in os/exec)

Exit Code Capture
    └──requires──> Zombie Prevention (cmd.Wait must be called first)
    └──enables──> Signal-Killed Exit Code Emulation (inspect ProcessState after Wait)

Command Not Found Handling
    └──requires──> Nothing (checked before spawning via exec.LookPath or cmd.Start error)
    └──enhances──> Minimal Logging (log the specific error with human-readable message)
```

### Dependency Notes

- **Zombie Prevention requires Process Spawning:** `cmd.Wait()` only makes sense after `cmd.Start()`. If using `cmd.Run()`, Wait is called internally — but `cmd.Run()` cannot be used alongside signal interception because it blocks.
- **Signal Interception requires Process Spawning:** Cannot forward a signal until `cmd.Process` is populated, which happens after `cmd.Start()` returns.
- **Process Group Isolation conflicts with default Ctrl-C behavior:** When `Setpgid: true` is set, the OS no longer delivers Ctrl-C to the child automatically. Signal forwarding becomes mandatory, not optional.
- **Stdout/Stderr direct assignment conflicts with StdoutPipe:** Setting `cmd.Stdout` and calling `cmd.StdoutPipe()` on the same Cmd will panic. Pick one approach.
- **Exit Code Emulation for signal-killed processes:** `ExitCode()` returns -1 when the process was killed by a signal. To emit the correct POSIX exit code (128+N), must inspect `syscall.WaitStatus` — this is a post-Wait step.

---

## MVP Definition

### Launch With (v0)

The minimum to prove "correct, deterministic process lifecycle management":

- [ ] `rtx run <command> [args...]` CLI entry point — the user-facing interface
- [ ] Process spawning with `cmd.Start()` (not `cmd.Run()`) — required for signal interception
- [ ] PID display immediately after spawn — operators need to know the PID
- [ ] Real-time stdout/stderr passthrough via direct `cmd.Stdout = os.Stdout` assignment — unbuffered, race-free
- [ ] SIGINT + SIGTERM interception and forwarding to child — correct shutdown behavior
- [ ] Graceful shutdown: forward signal, block on `cmd.Wait()`, exit with child's code
- [ ] `cmd.Wait()` always called — zombie prevention, non-negotiable on POSIX
- [ ] Exit code capture and propagation: `*exec.ExitError.ExitCode()` → `os.Exit(code)`
- [ ] "Command not found" error: detect `exec.ErrNotFound`, emit clear message, exit 127
- [ ] Immediate crash error handling: child exits quickly → propagate its exit code as-is
- [ ] Minimal logging: `[rtx] spawned PID %d`, `[rtx] received signal %s`, `[rtx] exited with code %d`

### Add After Validation (v0.x)

Add once the core correctness proof is working:

- [ ] Signal-killed exit code emulation (128+N) — polish; only needed if consumers check for signal-kill vs normal exit
- [ ] Process group isolation (`Setpgid: true`) — adds control but complicates signal delivery; validate basic signal forwarding first
- [ ] Unit tests for exit code propagation, zombie prevention, signal routing — add alongside or immediately after implementation

### Future Consideration (v1+)

Defer until v0 proves correctness and real usage patterns emerge:

- [ ] Restart policies — requires understanding of failure patterns from actual use
- [ ] Multi-process support — needs dependency graph design
- [ ] Config file support — needs format and schema decisions
- [ ] Timeout / watchdog — needs graceful kill sequence design
- [ ] Daemon mode — needs PID file, log redirect, double-fork design

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Process spawning | HIGH | LOW | P1 |
| Zombie prevention (cmd.Wait) | HIGH | LOW | P1 |
| Exit code propagation | HIGH | LOW | P1 |
| Stdout/stderr passthrough | HIGH | LOW | P1 |
| SIGINT/SIGTERM forwarding | HIGH | MEDIUM | P1 |
| Graceful shutdown sequence | HIGH | MEDIUM | P1 |
| PID display | MEDIUM | LOW | P1 |
| Command not found handling | MEDIUM | LOW | P1 |
| Minimal logging | MEDIUM | LOW | P1 |
| Signal-killed exit code (128+N) | MEDIUM | MEDIUM | P2 |
| Process group isolation | MEDIUM | MEDIUM | P2 |
| Unit tests | HIGH | MEDIUM | P2 |
| Restart policies | HIGH | HIGH | P3 |
| Multi-process support | HIGH | HIGH | P3 |
| Config file parsing | MEDIUM | MEDIUM | P3 |

**Priority key:**
- P1: Must have for v0 launch (correctness proof fails without it)
- P2: Should have; add when P1 is working
- P3: Future — v1+

---

## Competitor Feature Analysis

Reference tools surveyed to establish what "table stakes" means in the process runner space:

| Feature | tini (krallin/tini) | dumb-init (Yelp) | supervisord | rtx v0 approach |
|---------|---------------------|------------------|-------------|-----------------|
| Single-process spawning | Yes | Yes | Yes (multi) | Yes — single process only |
| Zombie reaping | Yes — primary feature | Yes | Yes | `cmd.Wait()` — always called |
| Signal forwarding | Yes — to child PID | Yes — to process group | Yes | Forward to `cmd.Process`; optionally to process group |
| Exit code propagation | Yes | Yes | Partial (restart masks codes) | Yes — exact code via `ExitError.ExitCode()` |
| Stdout/stderr passthrough | Transparent (fd inheritance) | Transparent | Log files / redirect | Direct fd assignment — same transparency |
| Config file | No | No | Yes (ini format) | No — v0 CLI only |
| Restart policy | No | No | Yes — primary feature | No — v0 out of scope |
| Web UI | No | No | Yes | No |
| Signal rewriting | No | Yes (`--rewrite`) | No | No — forward as-is |
| Process group mode | With `-g` flag | Default behavior | No | Optional (`Setpgid`) |
| Daemon mode | No | No | Yes | No |

**Key insight:** tini is the closest analog to rtx v0. It does one thing: spawn one child, forward signals, reap zombies, propagate exit code. rtx v0 targets this same feature surface but as a Go CLI with direct OS process control rather than as an init process (PID 1).

---

## Sources

- [os/exec package — pkg.go.dev](https://pkg.go.dev/os/exec) — official documentation; HIGH confidence
- [os/signal package — pkg.go.dev](https://pkg.go.dev/os/signal) — official documentation; HIGH confidence
- [Go issue #52580: cmd.Wait must be called](https://github.com/golang/go/issues/52580) — confirms Wait is mandatory on POSIX; HIGH confidence
- [tini README — krallin/tini](https://github.com/krallin/tini/blob/master/README.md) — defines the minimal process runner feature set; HIGH confidence
- [Introducing dumb-init — Yelp Engineering Blog](https://engineeringblog.yelp.com/2016/01/dumb-init-an-init-for-docker.html) — signal forwarding and zombie reaping patterns; MEDIUM confidence
- [Advanced command execution in Go — kowalczyk.info](https://blog.kowalczyk.info/article/wOYk/advanced-command-execution-in-go-with-osexec.html) — patterns including StdoutPipe race condition warning; MEDIUM confidence
- [Managing Linux Processes in Go — mezhenskyi.dev](https://mezhenskyi.dev/posts/go-linux-processes/) — zombie prevention, signal handling, process groups; MEDIUM confidence
- [Useful Patterns for Go's os/exec — DoltHub Blog](https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/) — Setpgid, errgroup, pipe lifecycle; MEDIUM confidence
- [supervisord introduction](https://supervisord.org/introduction.html) — establishes full-featured supervisor baseline (what NOT to build in v0); HIGH confidence
- [Graceful Shutdown in Go — VictoriaMetrics Blog](https://victoriametrics.com/blog/go-graceful-shutdown/) — shutdown patterns and grace period design; MEDIUM confidence
- [Exit Code 127 — spacelift.io](https://spacelift.io/blog/exit-code-127) — POSIX convention for command-not-found exit codes; MEDIUM confidence

---
*Feature research for: rtx — user-space process runner CLI*
*Researched: 2026-02-27*
