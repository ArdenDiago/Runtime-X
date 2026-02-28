# Pitfalls Research

**Domain:** Go process runner CLI (os/exec, signal handling, zombie prevention, exit code propagation, real-time streaming)
**Researched:** 2026-02-27
**Confidence:** HIGH — primary sources are pkg.go.dev official docs, tracked Go issue tracker bugs, and verified cross-source patterns

---

## Critical Pitfalls

### Pitfall 1: Calling cmd.Run() or cmd.Wait() Before Draining StdoutPipe/StderrPipe

**What goes wrong:**
Using `cmd.StdoutPipe()` or `cmd.StderrPipe()` and then calling `cmd.Run()` (instead of `cmd.Start()`) causes a deadlock. `Run()` internally calls `Wait()`, which waits for the command to finish — but the command won't finish until the pipes are drained, and nothing drains them until after `Run()` returns. Classic circular wait.

Similarly, calling `cmd.Wait()` before all goroutines reading from the pipes have finished causes a data race (detected by `-race`).

**Why it happens:**
Developers see `StdoutPipe()` as a simple "get output" call and assume `Run()` still works. The Go docs warn against this but the API doesn't prevent it.

**How to avoid:**
- Use `cmd.Start()` (never `cmd.Run()`) when using `StdoutPipe()` or `StderrPipe()`
- Start goroutines to read each pipe before calling `cmd.Wait()`
- Call `cmd.Wait()` only after all pipe-reading goroutines have finished
- Alternatively, skip pipes entirely: set `cmd.Stdout = os.Stdout` and `cmd.Stderr = os.Stderr` directly for pass-through streaming — no goroutines needed, no race condition possible

For this project (real-time streaming with no buffering), the simplest correct approach is direct assignment:
```go
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
```

**Warning signs:**
- Process hangs indefinitely after output stops
- `-race` detector reports data race on pipe reads
- Output appears only after the command finishes (buffered, not real-time)

**Phase to address:** Process execution foundation — must be settled before any other work

---

### Pitfall 2: Forgetting cmd.Wait() After cmd.Start() — Zombie Processes

**What goes wrong:**
If `cmd.Start()` is called but `cmd.Wait()` is never called, the child process becomes a zombie once it exits. The zombie persists in the process table for the entire lifetime of the Go process. On POSIX systems, the OS requires the parent to call `wait()` (or equivalent) to reap the child's exit status. Go's `cmd.Wait()` maps to `syscall.Wait4`.

**Why it happens:**
Developers focus on the "happy path" — the command runs and finishes. Edge cases like signal interruption, early return on error, or panic paths cause `Wait()` to be skipped.

**How to avoid:**
- Use `defer cmd.Wait()` immediately after a successful `cmd.Start()`
- If the exit code matters (it does for `rtx`), don't use naked defer — capture the error: `waitErr = cmd.Wait()` in a deferred closure
- Never use `Process.Release()` as a substitute — it explicitly leaves a zombie on the system
- In cleanup/signal handler paths, ensure `Wait()` is still called even if the process was killed

**Warning signs:**
- `ps aux | grep Z` shows zombie entries with the child command name
- `cmd.Process` is non-nil but `Wait()` was never called
- Process table entry persists after the child exits

**Phase to address:** Process execution foundation — every code path through the runner must call `Wait()`

---

### Pitfall 3: Signal Handling With an Unbuffered Channel — Dropped Signals

**What goes wrong:**
`signal.Notify(c, ...)` requires a buffered channel. If the channel is unbuffered (capacity 0), the signal package does a non-blocking send — if the goroutine is not immediately ready to receive, the signal is silently dropped. The process runner appears to ignore signals.

**Why it happens:**
Go's `make(chan os.Signal)` creates an unbuffered channel by default. This is idiomatic for most Go channel uses, but `signal.Notify` is a documented exception. The error is subtle — it compiles and usually works until a signal arrives at the wrong moment.

**How to avoid:**
```go
// WRONG — unbuffered, signals can be dropped
sigCh := make(chan os.Signal)

// CORRECT — buffered size 1 is sufficient for a single signal type
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
```

`go vet` now flags unbuffered channels passed to `signal.Notify` — run it.

**Warning signs:**
- `go vet` reports `misuse of unbuffered os.Signal channel`
- Ctrl+C sometimes kills the runner but other times has no effect
- Signal handling is reliable in tests but intermittent in real use

**Phase to address:** Signal handling phase — the very first line of signal code

---

### Pitfall 4: Signals Not Forwarded to Child — Child Ignores Ctrl+C

**What goes wrong:**
Intercepting SIGINT in the Go parent with `signal.Notify` stops the OS from delivering that signal to the child. The Go process catches it, but the child never sees it. The child keeps running, and the runner hangs waiting for a process that will never exit.

**Why it happens:**
This is the correct behavior of `signal.Notify` — it stops the default action (which would have terminated the program) and routes the signal to the channel. But it also prevents propagation down the process tree unless the developer explicitly re-sends it.

**How to avoid:**
After catching the signal, explicitly forward it to the child:
```go
cmd.Process.Signal(sig)  // forward the caught signal
```
Then call `cmd.Wait()` to wait for the child to finish its cleanup before the runner exits.

**Warning signs:**
- Ctrl+C in the terminal causes the runner to print "signal received" but hangs indefinitely afterward
- Child process remains running (`ps`) after Ctrl+C
- runner exits but child process lives on as an orphan

**Phase to address:** Signal handling phase

---

### Pitfall 5: Process Group Conflict — Child Receives Signal Twice (or Zero Times)

**What goes wrong:**
Two conflicting approaches produce opposite bugs:

1. **No Setpgid (default):** Child is in the same process group as the runner. Ctrl+C sends SIGINT to the entire foreground process group — both runner and child receive it simultaneously. The child exits, then the runner tries to forward the signal to an already-dead process (returns "process already finished" error).

2. **Setpgid: true:** Child is placed in a new process group. Ctrl+C only reaches the runner. The runner must explicitly forward signals. If it forgets, the child never receives any signal.

For `rtx` (a transparent process runner, not a daemon), the correct model is: runner forwards signal to child, then waits for child to finish, then exits with child's exit code. Neither blind Setpgid nor ignoring the issue produces this correctly.

**Why it happens:**
The behavior of Setpgid is subtle and depends on the runner's intended role. Daemon-style runners want Setpgid. Transparent forwarders (like `rtx`) generally do not — but they must handle the case where the child has already exited when the forward attempt occurs.

**How to avoid:**
For `rtx` (transparent forwarder):
- Do NOT set Setpgid — let child inherit the process group
- Intercept signals, forward to child, swallow "process already finished" errors from Signal()
- Always call `cmd.Wait()` regardless

```go
if err := cmd.Process.Signal(sig); err != nil {
    // process may have already exited — not an error worth propagating
    if !errors.Is(err, os.ErrProcessDone) {
        log.Printf("signal forward failed: %v", err)
    }
}
```

**Warning signs:**
- Double output of signal-related log lines
- "os: process already finished" errors in logs
- Ctrl+C kills runner but child continues running

**Phase to address:** Signal handling phase — requires deliberate design decision before coding

---

### Pitfall 6: Exit Code Lost — Always Returning 0 or 1

**What goes wrong:**
When the child process exits with a non-zero code, `cmd.Wait()` returns a non-nil error of type `*exec.ExitError`. If the runner does `if err != nil { os.Exit(1) }`, it swallows the real exit code and always reports 1. Callers (CI systems, shell scripts) that rely on exact exit codes receive wrong information.

**Why it happens:**
The `*exec.ExitError` type isn't well-known. Many developers know `cmd.Run()` returns an error but don't know how to extract the integer code from it. Before Go 1.12, extraction required importing `syscall` and a platform-specific cast. Now `ExitCode()` is available but not widely documented.

**How to avoid:**
```go
waitErr := cmd.Wait()
if waitErr != nil {
    var exitErr *exec.ExitError
    if errors.As(waitErr, &exitErr) {
        os.Exit(exitErr.ExitCode())
    }
    // Non-ExitError: runner infrastructure failure (e.g., I/O error)
    fmt.Fprintf(os.Stderr, "rtx: %v\n", waitErr)
    os.Exit(1)
}
os.Exit(0)
```

Also: `os.Exit()` bypasses `defer`. Structure the main function so the real exit code is returned from an inner function, and `main()` calls `os.Exit()` on the result — this preserves deferred cleanup.

**Warning signs:**
- `echo $?` always returns 0 or 1 regardless of what the child does
- Shell scripts treating `rtx` as a transparent wrapper behave incorrectly
- CI pipeline shows "success" when the wrapped command failed

**Phase to address:** Process execution foundation — exit code extraction must be built in from the start

---

### Pitfall 7: cmd.Wait() Blocks Forever When Grandchild Inherits Pipe

**What goes wrong:**
If the child process spawns its own child (grandchild) and passes the pipe file descriptors to it, and the child exits without waiting for the grandchild, the grandchild now holds an open write end of the stdout/stderr pipe. `cmd.Wait()` waits for EOF on the pipes before returning — it never gets EOF because the grandchild is still running. The runner hangs forever.

**Why it happens:**
`cmd.Wait()` spawns internal goroutines that copy from the pipe to the writer. These goroutines exit on EOF. If a grandchild keeps the pipe open, EOF never arrives.

**How to avoid:**
Go 1.20 added `WaitDelay` and the improved `CommandContext` cancel behavior to bound this case. For `rtx` in v0 (single-process focus, no grandchild concerns), this is lower priority — but still worth understanding:

```go
cmd.WaitDelay = 5 * time.Second  // unblock Wait() if pipes don't close
```

For v0, using `cmd.Stdout = os.Stdout` (direct `*os.File` assignment) avoids the internal goroutine entirely — `Wait()` does not spawn goroutines when Stdout/Stderr are `*os.File`. This sidesteps the problem completely.

**Warning signs:**
- Runner hangs after child process exits
- Child PID is no longer in `ps` but the runner process is still running
- No further output but process doesn't terminate

**Phase to address:** Process execution foundation — avoided by using direct file assignment for output

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `cmd.Output()` / `cmd.CombinedOutput()` for "simple" output capture | One line of code | Buffers all output in memory; no real-time streaming; no exit code distinction | Never for rtx — streaming is a core requirement |
| `os.Exit(1)` on any `cmd.Wait()` error | Simple error handling | Swallows real exit codes; breaks callers | Never — use ExitError.ExitCode() |
| Ignoring the error from `cmd.Process.Signal()` entirely | Less code | Hides legitimate forwarding failures vs. expected "already exited" cases | Acceptable if using `errors.Is(err, os.ErrProcessDone)` to distinguish |
| Using `cmd.Run()` instead of `cmd.Start()` + `cmd.Wait()` | Simpler call | Incompatible with pipe-based streaming; no interruptibility | Acceptable only for fire-and-forget with direct os.Stdout/os.Stderr assignment |
| Buffered output via `bytes.Buffer` | Simple to implement | Memory grows unbounded for long-running processes; no real-time output | Never for rtx |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Terminal / TTY | Setting `cmd.Stdout = os.Stdout` while also using `StdoutPipe()` — panics at Start | Choose one method: either direct assignment OR pipe, never both |
| Shell built-ins | Running `cd`, `source`, `export` directly via `exec.Command` — they don't exist as executables | Wrap in `sh -c "..."` if shell built-ins are needed; for rtx, document this limitation |
| PATH resolution | Relying on implicit `.` in PATH for local executables (Go 1.19+ ErrDot) | Use `./binary` explicit prefix for local executables |
| Signal to killed process | Forwarding signal after child is already dead returns `os.ErrProcessDone` | Swallow this specific error; it is expected, not a bug |
| Deferred cleanup + os.Exit | `defer` statements don't run when `os.Exit()` is called | Use the inner-function-returns-int pattern for main |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| `bytes.Buffer` for stdout capture | Memory grows proportional to command output | Use direct `cmd.Stdout = os.Stdout` assignment for pass-through | Any long-running command with verbose output |
| StdoutPipe + single goroutine reading both stdout and stderr sequentially | Deadlock when stderr fills pipe buffer while stdout goroutine is blocked | Separate goroutines for each stream, or use direct file assignment | When command writes > ~65KB to either stream before the other is consumed |
| Synchronous signal handling (no goroutine) | Signal arrives, handler blocks on `cmd.Process.Signal()`, next signal missed | Use goroutine for signal dispatch; keep the signal channel receive loop tight | In shutdown sequences where multiple signals arrive quickly |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Shell-wrapping user input: `exec.Command("sh", "-c", userInput)` | Shell injection — arbitrary command execution | For rtx, use `exec.Command(args[0], args[1:]...)` directly — no shell involved |
| Executing relative paths from PATH without validation | PATH hijacking if CWD is writable | Use `exec.LookPath` explicitly and validate the resolved absolute path if needed |
| Leaking parent environment to child unconditionally | Sensitive env vars (tokens, passwords) passed to untrusted child | Explicitly construct `cmd.Env` from a safe allowlist; don't blindly inherit `os.Environ()` for untrusted commands |

---

## "Looks Done But Isn't" Checklist

- [ ] **Signal handling:** Signals are intercepted AND forwarded to child — verify child actually exits on Ctrl+C, not just the runner
- [ ] **Exit code:** `echo $?` after `rtx run false` returns exactly `1`; after `rtx run sh -c 'exit 42'` returns exactly `42`
- [ ] **Zombie prevention:** After child exits, verify no zombie in `ps aux | grep Z` while runner is still alive
- [ ] **Real-time streaming:** Long-running commands (e.g., `rtx run yes`) show output line-by-line, not all at once at exit
- [ ] **Error vs. exit:** `rtx run nonexistent-command` reports "command not found" with a non-zero exit code, not a panic or generic error
- [ ] **Graceful shutdown order:** On SIGTERM, runner waits for child to fully exit before exiting itself — not the other way around
- [ ] **os.Exit skips defer:** Any deferred cleanup (file closes, log flushes) runs before the runner exits — use the inner-function pattern
- [ ] **Second signal handling:** If child ignores SIGTERM, a second Ctrl+C should escalate (or at minimum not hang) — decide the policy and implement it

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Zombie processes discovered in production | LOW | Add `defer cmd.Wait()` after `cmd.Start()` in all code paths; deploy fix |
| Exit codes always 1 — callers broken | MEDIUM | Replace `os.Exit(1)` with `ExitError.ExitCode()` extraction; test with `sh -c 'exit N'` cases |
| Signals not forwarded — child orphaned on Ctrl+C | MEDIUM | Add explicit `cmd.Process.Signal(sig)` in signal handler; test with `sleep 100` as child |
| Deadlock on large command output | MEDIUM | Switch from `bytes.Buffer` capture to direct `os.Stdout`/`os.Stderr` assignment |
| cmd.Wait() hangs due to grandchild pipe inheritance | HIGH | Add `WaitDelay`; switch to direct `*os.File` assignment to avoid internal copy goroutines entirely |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| StdoutPipe + Run() deadlock | Phase 1: Process execution foundation | Test: real-time output appears line-by-line for `rtx run yes | head -5` |
| Zombie prevention (missing Wait) | Phase 1: Process execution foundation | Test: `ps aux | grep Z` after child exits with runner still alive |
| Unbuffered signal channel | Phase 2: Signal handling | `go vet ./...` passes; rapid Ctrl+C never drops signal |
| Signal not forwarded to child | Phase 2: Signal handling | Test: `rtx run sleep 100`, press Ctrl+C, verify sleep exits |
| Process group / double signal | Phase 2: Signal handling | Test: verify no "process already finished" errors on normal Ctrl+C |
| Exit code lost | Phase 1: Process execution foundation | Test: `rtx run sh -c 'exit 42'; echo $?` outputs `42` |
| cmd.Wait() blocks (grandchild pipe) | Phase 1: Process execution foundation | Avoided entirely by using `cmd.Stdout = os.Stdout` (direct *os.File assignment) |
| os.Exit() skips defer cleanup | Phase 1: Process execution foundation | Use inner-function-returns-int pattern from the start in main |
| Windows SIGINT not sendable | Cross-platform phase (post-Linux) | Document Linux-first constraint; use build tags for platform-specific signal code |

---

## Cross-Platform Gotchas (Linux vs. macOS vs. Windows)

### Linux (First-Class Target)
- SIGINT, SIGTERM, SIGKILL all work as expected
- `setpgid` available via `syscall.SysProcAttr{Setpgid: true}`
- `/proc` filesystem available for PID validation
- Signal 0 (`process.Signal(syscall.Signal(0))`) can probe process existence

### macOS
- Signal behavior is largely identical to Linux for this use case
- `Setpgid` works the same way
- No `/proc` filesystem — cannot use `/proc/[pid]/status` for diagnostics
- `syscall` package has Darwin-specific constants — use `golang.org/x/sys/unix` for portable Unix code

### Windows — Significant Divergences
- **SIGINT is not sendable to other processes** — `process.Signal(os.Interrupt)` returns "not supported" (tracked in Go issue #6720, #28498)
- Windows uses `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT)` instead, which requires a separate console group
- SIGTERM does not exist on Windows — the only way to terminate a process is `TerminateProcess` (equivalent to SIGKILL)
- Process groups work differently — `Setpgid` behavior differs
- No POSIX-style signal inheritance
- **Recommendation for rtx v0:** Declare Linux as the target platform. Add build constraints if cross-platform support is added later. Do not attempt to abstract signal differences in v0 — the abstraction is non-trivial and correctness on Linux is the stated goal.

---

## Sources

- [os/exec package — pkg.go.dev official docs](https://pkg.go.dev/os/exec) — HIGH confidence
- [os/signal package — pkg.go.dev official docs](https://pkg.go.dev/os/signal) — HIGH confidence
- [Go issue #52580: cmd.Wait must be called — documentation unclear](https://github.com/golang/go/issues/52580) — HIGH confidence
- [Go issue #40467: cmd/go run does not relay signals to child process](https://github.com/golang/go/issues/40467) — HIGH confidence
- [Go issue #6720: os.Interrupt is not sendable on Windows](https://github.com/golang/go/issues/6720) — HIGH confidence
- [Go issue #28498: SIGINT not supported for other processes on Windows](https://github.com/golang/go/issues/28498) — HIGH confidence
- [Go issue #21135: proposal — allow user of CommandContext to specify kill signal](https://github.com/golang/go/issues/21135) — HIGH confidence
- [Go issue #19804: possible data race when using same writer for Stdout and Stderr](https://github.com/golang/go/issues/19804) — HIGH confidence
- [Go issue #45604: misuse of unbuffered os.Signal channel](https://github.com/golang/go/issues/45604) — HIGH confidence
- [DoltHub Blog: Useful Patterns for Go's os/exec](https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/) — MEDIUM confidence
- [Go Exec zombies and orphaned processes — SegmentFault](https://segmentfault.com/a/1190000041466423/en) — MEDIUM confidence
- [Managing Linux Processes in Go — mezhenskyi.dev](https://mezhenskyi.dev/posts/go-linux-processes/) — MEDIUM confidence
- [Killing a child process and all of its children in Go — Felix Geisendörfer](https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773) — MEDIUM confidence
- [SIGINT and SIGTERM Handling — go-task issue #75](https://github.com/go-task/task/issues/75) — MEDIUM confidence
- [Reading os/exec.Cmd Output Without Race Conditions — hackmysql.com](https://hackmysql.com/rand/reading-os-exec-cmd-output-without-race-conditions/) — MEDIUM confidence

---
*Pitfalls research for: Go process runner CLI (Runtime X / rtx)*
*Researched: 2026-02-27*
