# Phase 3: Tests and Validation - Research

**Researched:** 2026-02-28
**Domain:** Go unit testing for os/exec subprocess behavior — exit codes, zombie prevention, signal delivery, table-driven patterns
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

None. All implementation choices are left to Claude's discretion.

### Claude's Discretion

- Test file organization and naming (standard Go `_test.go` conventions)
- Table-driven vs individual test functions
- Test execution speed targets
- Whether to use `go test -race`
- Test helper patterns (if any)
- Manual validation checklist depth and documentation format
- CI integration (if any — likely out of scope for v0)
- Edge case selection and coverage priorities

User trusts standard Go testing patterns and research recommendations.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TEST-01 | Unit test: `rtx run false` returns exit code 1 | `exec.Command("false")` + `ExitError.ExitCode()` assertion — trivially testable with real binary |
| TEST-02 | Unit test: `rtx run sh -c 'exit 42'` returns exit code 42 | `exec.Command("sh", "-c", "exit 42")` — tests exact code propagation via `resolveExitCode` |
| TEST-03 | Unit test: process spawning does not leave zombie processes | Start child, wait, inspect `/proc/<pid>/status` for `State: Z` while parent is alive |
| TEST-04 | Unit test: signal forwarding delivers signal to child | Spawn sleeping child via re-exec helper, send `syscall.SIGTERM` via `cmd.Process.Signal`, verify exit code 143 |
| TEST-05 | Unit test: "command not found" returns exit code 127 | Call `process.Run("nonexistent-xyz", nil)` directly — no real binary needed |
| TEST-06 | Manual validation: `rtx run yes` outputs line-by-line (real-time, not buffered) | Not automatable without a TTY; document as manual checklist item with exact command |
</phase_requirements>

---

## Summary

Phase 3 adds `internal/process/runner_test.go` — the only new file this phase creates. The implementation in `runner.go` is complete and correct (verified end-to-end in Phases 1 and 2). The test's job is to exercise `process.Run()` at its public boundary and confirm the five automatable behaviors hold. One requirement (TEST-06, real-time streaming) cannot be verified in `go test` without a PTY and is explicitly a manual check.

The project already uses table-driven tests with subtests (`internal/docker/validator_test.go` pattern). This phase follows the same convention. The key challenge is TEST-03 (zombie detection) and TEST-04 (signal delivery) — both require subprocess timing coordination but no external test libraries. The re-exec helper pattern (spawning the test binary itself as the subprocess) is the Go-idiomatic solution for controlled signal-target processes.

**Primary recommendation:** Write `runner_test.go` in `internal/process/` using table-driven tests for exit codes, `/proc/<pid>/status` inspection for zombie detection, and the `TestHelperProcess` re-exec pattern for signal delivery. Run with `go test -race ./internal/process/...`. Document TEST-06 as a manual step.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `testing` (stdlib) | Go 1.25.7 (project version) | Test runner, T.Run subtests, assertions | The only test package needed; project has zero test deps |
| `os/exec` (stdlib) | Go 1.25.7 | Spawn real binaries (`false`, `sh`, `true`) as test fixtures | Direct — same package under test uses it |
| `syscall` (stdlib) | Go 1.25.7 | Send `SIGTERM`/`SIGINT` to test subprocesses | Required for `syscall.SIGTERM` constant and signal delivery |
| `os` (stdlib) | Go 1.25.7 | Read `/proc/<pid>/status` for zombie check; `os.Args[0]` for re-exec | Linux-native, no third-party dep |
| `strings` (stdlib) | Go 1.25.7 | Parse `/proc/<pid>/status` content | Trivial string scan |
| `time` (stdlib) | Go 1.25.7 | Sleep briefly after signal send to let child exit | Signal delivery is async |
| `fmt` (stdlib) | Go 1.25.7 | Format error messages in test failures | Standard |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `bufio` (stdlib) | Go 1.25.7 | Scan `/proc/<pid>/status` line by line | Used in zombie detection helper |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `testing` only | `testify/assert` | testify adds an external dep; the project is stdlib-only by decision. Assertion failures with `t.Fatalf` are sufficient. |
| `/proc/<pid>/status` for zombie check | `ps aux \| grep Z` via `exec.Command` | Shelling out to `ps` works but is slower and less reliable in sandboxed CI. Direct `/proc` read is instant and Linux-native. |
| Re-exec helper for signal target | `exec.Command("sleep", "100")` | `sleep` would work but makes tests dependent on external binary location and timing. Re-exec pattern is self-contained. |
| `go test -timeout 30s` | Explicit per-test timeouts via context | Per-test contexts add boilerplate. `-timeout` flag is simpler for this scope. |

**Installation:** No new packages. All stdlib.

---

## Architecture Patterns

### Recommended Project Structure

```
internal/process/
├── runner.go          # Existing — process.Run() implementation (unchanged)
└── runner_test.go     # NEW — all Phase 3 unit tests live here
```

No new directories. No new binaries. One file added to an existing package.

### Pattern 1: Direct Call to `process.Run()` for Exit Code Tests

**What:** Call `process.Run(name, args)` directly from the test and compare the returned int to the expected exit code.

**When to use:** TEST-01 (exit 1 from `false`), TEST-02 (exit 42 from `sh -c 'exit 42'`), TEST-05 (exit 127 from nonexistent command).

**Why this works:** `runner.go` is in `package process`. Test file is in `package process` too (same-package test). `process.Run()` returns `int`. No subprocess indirection needed.

**Example:**

```go
// Source: inferred from process.Run() signature in runner.go
func TestRunExitCodes(t *testing.T) {
    tests := []struct {
        name     string
        command  string
        args     []string
        wantCode int
    }{
        {"false exits 1",      "false",  nil,                    1},
        {"exit 42",            "sh",     []string{"-c", "exit 42"}, 42},
        {"command not found",  "nonexistent-cmd-xyz-rtx-test", nil, 127},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Run(tt.command, tt.args)
            if got != tt.wantCode {
                t.Errorf("Run(%q) = %d, want %d", tt.command, got, tt.wantCode)
            }
        })
    }
}
```

**Confidence:** HIGH — this directly calls the function under test with real system binaries.

---

### Pattern 2: `TestHelperProcess` Re-exec for Signal Target

**What:** The test binary re-launches itself with `-test.run=TestHelperProcess` and a sentinel env var. The helper subprocess sleeps or runs a real command. The parent test sends a signal and asserts on exit code.

**When to use:** TEST-04 (signal delivery). We need a subprocess we can signal — `process.Run()` wraps the child internally so we test `process.Run()` indirectly via a subprocess that IS the test binary running in helper mode.

**The core insight for TEST-04:** We cannot call `process.Run()` directly and also signal its internal child from the same goroutine — `Run()` blocks until the child exits. Instead, spawn `process.Run()` *in its own subprocess* (via the test binary), then signal that subprocess from the parent test. Observe exit code 143 (SIGTERM).

**Two-subprocess chain for TEST-04:**

```
test (parent) --SIGTERM--> [test binary in helper mode] --SIGTERM--> sleep
                            (runs process.Run("sleep","100"))
```

**Example:**

```go
// TestHelperProcess is the re-exec helper. It only runs when the sentinel
// env var is set. In normal test runs it returns immediately (0ms overhead).
func TestHelperProcess(t *testing.T) {
    if os.Getenv("RTX_TEST_HELPER") != "1" {
        return
    }
    args := os.Args
    for i, arg := range args {
        if arg == "--" {
            args = args[i+1:]
            break
        }
    }
    if len(args) == 0 {
        fmt.Fprintln(os.Stderr, "helper: no command")
        os.Exit(1)
    }
    // Run the real process.Run — this IS what we're testing via signal.
    code := Run(args[0], args[1:])
    os.Exit(code)
}

func TestSignalDelivery(t *testing.T) {
    // Spawn the test binary in helper mode running "sleep 10"
    cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "sleep", "10")
    cmd.Env = append(os.Environ(), "RTX_TEST_HELPER=1")
    if err := cmd.Start(); err != nil {
        t.Fatalf("start helper: %v", err)
    }

    // Give process.Run() time to start sleep and register signal handler.
    time.Sleep(200 * time.Millisecond)

    // Send SIGTERM to the helper (process.Run will forward to sleep).
    if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
        t.Fatalf("signal: %v", err)
    }

    // Wait and check exit code: 143 = 128 + SIGTERM(15).
    err := cmd.Wait()
    var exitErr *exec.ExitError
    if !errors.As(err, &exitErr) {
        t.Fatalf("expected ExitError, got %T: %v", err, err)
    }
    if got := exitErr.ExitCode(); got != 143 {
        t.Errorf("exit code = %d, want 143 (128+SIGTERM)", got)
    }
}
```

**Confidence:** HIGH — this is the canonical Go subprocess testing pattern used in the Go stdlib's own `os/exec` and `os/signal` test suites.

---

### Pattern 3: `/proc/<pid>/status` Zombie Detection

**What:** After `process.Run()` returns (child has exited and been waited on), check whether the child PID shows `State: Z` in `/proc/<pid>/status`. If `runner.go` calls `cmd.Wait()` correctly, the status file will be gone or show a non-zombie state before Run() returns.

**When to use:** TEST-03 (zombie prevention).

**Key insight:** The zombie check must happen AFTER `process.Run()` returns and BEFORE the test process exits. The child PID recorded from the log output (or captured via custom stderr writer) is checked in `/proc`. If `cmd.Wait()` was called, the kernel has reaped the child and `/proc/<pid>/status` will either not exist or not show `Z`.

**Practical approach:** Capture stderr from `process.Run()` using a bytes.Buffer assigned to the cmd run in the re-exec helper. Parse the PID from `[rtx] spawned PID <n>`. Then read `/proc/<n>/status`. Alternatively, run `process.Run()` directly and capture stderr to extract the PID.

**Simpler approach for TEST-03:** Since `process.Run()` blocks until Wait() completes, zombie prevention is guaranteed by design. The test can verify the structural guarantee: that after `process.Run()` returns, no zombie with the known PID exists in `/proc`.

```go
func TestZombiePrevention(t *testing.T) {
    // Redirect stderr to capture PID log.
    var stderr bytes.Buffer
    // process.Run uses os.Stderr directly — need to test via subprocess
    // or temporarily redirect. Use helper subprocess approach.
    cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "true")
    cmd.Env = append(os.Environ(), "RTX_TEST_HELPER=1")
    var helperStderr bytes.Buffer
    cmd.Stderr = &helperStderr
    if err := cmd.Run(); err != nil {
        t.Fatalf("helper: %v", err)
    }

    // Parse PID from "[rtx] spawned PID 12345"
    pid := extractPID(t, helperStderr.String())

    // After Run() returns, child should be reaped. Check /proc.
    statusPath := fmt.Sprintf("/proc/%d/status", pid)
    data, err := os.ReadFile(statusPath)
    if err != nil {
        // /proc/<pid>/status gone entirely = reaped = no zombie. PASS.
        return
    }
    // File exists — check state is not Z.
    for _, line := range strings.Split(string(data), "\n") {
        if strings.HasPrefix(line, "State:") {
            if strings.Contains(line, "Z") {
                t.Errorf("PID %d is zombie: %s", pid, line)
            }
            return
        }
    }
}
```

**Note on stderr capture:** `process.Run()` calls `fmt.Fprintf(os.Stderr, ...)` with `os.Stderr` hardcoded. To observe the PID in a test, either: (a) use the re-exec helper with `cmd.Stderr = &buf`, or (b) add an optional stderr writer parameter to `Run()` (interface change — avoid for now), or (c) parse `/proc` of the direct child of the test binary. Approach (a) is cleanest.

**Confidence:** HIGH for the `/proc` check mechanism. MEDIUM for PID extraction — depends on log format stability (`[rtx] spawned PID %d`).

---

### Pattern 4: Table-Driven Subtests (Project Convention)

**What:** All tests use `t.Run(tt.name, func(t *testing.T) {...})` with a slice of test cases.

**When to use:** TEST-01, TEST-02, TEST-05 (exit code tests fit naturally). TEST-03 and TEST-04 are single-scenario tests — they can be standalone functions without a table.

**Project precedent:** `internal/docker/validator_test.go` uses this pattern consistently. Follow it.

---

### Anti-Patterns to Avoid

- **Testing process.Run() by checking its stderr output strings:** The log format is an implementation detail. Assert only on return codes.
- **Using `time.Sleep` for signal timing without bounds:** Use a fixed 200ms delay for signal tests. If timing is flaky, the test design is wrong, not the sleep duration.
- **Calling `os.Exit()` in the test helper accidentally:** The `TestHelperProcess` guard (`if os.Getenv("RTX_TEST_HELPER") != "1" { return }`) MUST be the very first thing in the function — otherwise normal test runs call `os.Exit` and crash.
- **Testing the `resolveExitCode` function in isolation:** It is an unexported function. Test behavior via `Run()`, not by calling the private function directly (even though same-package tests can access it). Behavioral tests are more robust.
- **Using `exec.Command("sh", "-c", "exit 42")` for TEST-04:** Signal test needs a process that lives long enough to receive a signal. `sh -c 'exit 42'` exits immediately. Use `sleep 10` or `sleep 100`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Subprocess timing coordination | Custom waitgroup or channel-based sync | `time.Sleep(200ms)` + `cmd.Wait()` | Signal tests only need to outlive the send; 200ms is reliable for a local sleep process |
| Zombie detection | Parse `ps aux` output | Read `/proc/<pid>/status` directly | No external binary, no parsing ambiguity, instant |
| Signal test subprocess | Mock or fake process | Re-exec test binary with `TestHelperProcess` | Real process, real signal delivery, no external dep |
| Exit code extraction | Custom wait loop | `errors.As(err, &exitErr)` + `.ExitCode()` | Already used in runner.go; same pattern in tests |
| Race condition detection | Manual mutex analysis | `go test -race ./internal/process/...` | Built into Go toolchain; zero-cost to enable |

**Key insight:** Go's stdlib `testing` package plus real system binaries (`false`, `sh`, `true`, `sleep`) provide everything needed. No mocking framework, no assertion library, no subprocess management library.

---

## Common Pitfalls

### Pitfall 1: TestHelperProcess Runs in Normal Test Invocations

**What goes wrong:** `TestHelperProcess` is a real test function. If the env var guard is missing or placed after any code that calls `os.Exit`, the function runs as a normal test and either passes vacuously or panics.

**Why it happens:** Developers forget the guard or misplace it.

**How to avoid:** The guard must be the FIRST statement:
```go
func TestHelperProcess(t *testing.T) {
    if os.Getenv("RTX_TEST_HELPER") != "1" {
        return  // ← MUST be first; empty return, not t.Skip()
    }
    // ... rest of helper
}
```
Using `t.Skip()` instead of `return` prints a SKIP line in test output on every run — `return` is correct.

**Warning signs:** Every `go test` run prints `--- SKIP: TestHelperProcess`.

---

### Pitfall 2: Zombie Check Races With Kernel Reaping

**What goes wrong:** Check `/proc/<pid>/status` too soon after `cmd.Wait()` and the kernel hasn't updated the proc entry yet — or the entry is already gone but the code treats "missing file" as an error.

**Why it happens:** `/proc/<pid>/status` disappearing is the CORRECT outcome after reaping. Treating `os.ReadFile` error as a test failure is wrong.

**How to avoid:** If `os.ReadFile` returns an error (file not found), that IS the zombie-prevention success case. Only fail if the file exists AND contains `State: Z`.

```go
data, err := os.ReadFile(statusPath)
if err != nil {
    return // file gone = reaped = PASS
}
```

---

### Pitfall 3: Signal Test Flakiness From Insufficient Startup Time

**What goes wrong:** Signal sent before `process.Run()` has called `signal.Notify` (which happens after `cmd.Start()`). Signal is delivered to the Go runtime default handler before the signal channel is set up, and the behavior is unpredictable.

**Why it happens:** `time.Sleep(50 * time.Millisecond)` is not enough on a loaded system.

**How to avoid:** Use 200ms minimum sleep after `cmd.Start()` and before `cmd.Process.Signal()`. The `sleep 10` child process gives 10 seconds of window — no tightness. For deterministic startup, the helper can write a ready signal to a pipe, but that's over-engineering for this scope.

---

### Pitfall 4: `process.Run()` Writes to `os.Stderr` Directly

**What goes wrong:** Tests that try to capture stderr output from a direct `Run()` call fail because `Run()` hardcodes `os.Stderr` via `fmt.Fprintf(os.Stderr, ...)`. `cmd.Stderr = &buf` on the test's own exec.Cmd does not affect `os.Stderr` inside the same process.

**Why it happens:** `os.Stderr` is a package-level `*os.File`. Assigning `cmd.Stderr` on the outer cmd does not redirect the inner `os.Stderr`.

**How to avoid:** For tests that need to capture stderr (zombie PID extraction), use the re-exec helper approach where the helper subprocess's stderr IS captured by `cmd.Stderr = &buf`. For exit code tests, no stderr capture is needed at all.

---

### Pitfall 5: `go vet` Flags Unbuffered Signal Channel (Pre-existing Issue)

**What goes wrong:** If any test code uses `make(chan os.Signal)` (unbuffered) and passes it to `signal.Notify`, `go vet` will fail.

**Why it matters:** `go vet ./...` is the project's linting step. Signal channel in test code must be buffered.

**How to avoid:** `sigCh := make(chan os.Signal, 1)` in all test code.

---

### Pitfall 6: TEST-06 Cannot Be Automated in `go test`

**What goes wrong:** Attempting to test "output appears line-by-line" via piping in `go test` — the test harness may buffer or the pipe semantics differ from a real TTY.

**Why it happens:** Real-time streaming validation requires observing time-sequenced output, which `bytes.Buffer` captures atomically after completion.

**How to avoid:** Declare TEST-06 as a manual validation step. Document the exact command and expected behavior in PLAN.md as a human-verify checkpoint:

```bash
# Manual TEST-06 check — must be run interactively, not in go test
timeout 2 ./bin/rtx run yes 2>/dev/null | head -5
# Expected: 5 lines of "y" appear quickly (sub-second), not all at exit
```

---

## Code Examples

Verified patterns for `runner_test.go`:

### Exit Code Table-Driven Test (TEST-01, TEST-02, TEST-05)

```go
// Source: direct call to process.Run() — same package
package process

import (
    "testing"
)

func TestRunExitCodes(t *testing.T) {
    tests := []struct {
        name     string
        command  string
        args     []string
        wantCode int
    }{
        // TEST-01: false exits with code 1
        {"false exits 1", "false", nil, 1},
        // TEST-02: sh -c 'exit 42' propagates exact code
        {"exit 42", "sh", []string{"-c", "exit 42"}, 42},
        // TEST-05: nonexistent command returns 127
        {"command not found", "nonexistent-rtx-test-xyz", nil, 127},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Run(tt.command, tt.args)
            if got != tt.wantCode {
                t.Errorf("Run(%q, %v) = %d, want %d",
                    tt.command, tt.args, got, tt.wantCode)
            }
        })
    }
}
```

### TestHelperProcess (re-exec scaffold)

```go
// Source: Go stdlib os/exec/exec_test.go pattern
package process

import (
    "fmt"
    "os"
    "testing"
)

// TestHelperProcess is the subprocess entry point for signal and zombie tests.
// It only activates when RTX_TEST_HELPER=1 is set.
func TestHelperProcess(t *testing.T) {
    if os.Getenv("RTX_TEST_HELPER") != "1" {
        return
    }
    // Parse args after "--" separator
    args := os.Args
    for i, a := range args {
        if a == "--" {
            args = args[i+1:]
            break
        }
    }
    if len(args) == 0 {
        fmt.Fprintln(os.Stderr, "[rtx-helper] no args")
        os.Exit(1)
    }
    os.Exit(Run(args[0], args[1:]))
}
```

### Signal Delivery Test (TEST-04)

```go
// Source: Go stdlib os/signal/signal_test.go pattern, adapted
package process

import (
    "errors"
    "os"
    "os/exec"
    "syscall"
    "testing"
    "time"
)

func TestSignalDelivery(t *testing.T) {
    // Spawn test binary in helper mode running "sleep 10"
    cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "sleep", "10")
    cmd.Env = append(os.Environ(), "RTX_TEST_HELPER=1")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Start(); err != nil {
        t.Fatalf("start helper subprocess: %v", err)
    }

    // Allow process.Run() to register signal.Notify before we send.
    time.Sleep(200 * time.Millisecond)

    // Send SIGTERM to the helper; process.Run() will forward it to sleep.
    if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
        t.Fatalf("send SIGTERM: %v", err)
    }

    err := cmd.Wait()
    var exitErr *exec.ExitError
    if !errors.As(err, &exitErr) {
        t.Fatalf("expected ExitError after SIGTERM, got %T: %v", err, err)
    }

    const wantCode = 143 // 128 + SIGTERM(15)
    if got := exitErr.ExitCode(); got != wantCode {
        t.Errorf("signal exit code = %d, want %d (128+SIGTERM)", got, wantCode)
    }
}
```

### Zombie Prevention Test (TEST-03)

```go
// Source: /proc filesystem inspection — Linux-native
package process

import (
    "bytes"
    "fmt"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "testing"
)

func TestZombiePrevention(t *testing.T) {
    // Use re-exec helper so we can capture stderr (where PID is logged).
    cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "true")
    cmd.Env = append(os.Environ(), "RTX_TEST_HELPER=1")
    var helperStderr bytes.Buffer
    cmd.Stderr = &helperStderr
    cmd.Stdout = os.Stdout

    if err := cmd.Run(); err != nil {
        t.Fatalf("helper run: %v", err)
    }

    // Extract PID from "[rtx] spawned PID <n>"
    pid := extractSpawnedPID(t, helperStderr.String())

    // After Run() returns, child must be reaped.
    statusPath := fmt.Sprintf("/proc/%d/status", pid)
    data, err := os.ReadFile(statusPath)
    if err != nil {
        // File gone = process was reaped = no zombie. PASS.
        return
    }
    // File still present — verify not in zombie state.
    for _, line := range strings.Split(string(data), "\n") {
        if strings.HasPrefix(line, "State:") {
            if strings.Contains(line, "Z") {
                t.Errorf("PID %d still zombie after Run() returned: %q", pid, line)
            }
            return
        }
    }
}

func extractSpawnedPID(t *testing.T, stderr string) int {
    t.Helper()
    // Log format: "[rtx] spawned PID 12345"
    for _, line := range strings.Split(stderr, "\n") {
        if strings.Contains(line, "spawned PID") {
            fields := strings.Fields(line)
            for i, f := range fields {
                if f == "PID" && i+1 < len(fields) {
                    pid, err := strconv.Atoi(fields[i+1])
                    if err == nil {
                        return pid
                    }
                }
            }
        }
    }
    t.Fatal("could not parse spawned PID from helper stderr:\n" + stderr)
    return 0
}
```

### Race Detector Usage

```bash
# Run all process package tests with race detector
go test -race -v ./internal/process/...
```

No special code needed — `go test -race` instruments the binary. If runner.go has a race condition, it will surface here. Given that `runner.go` uses `select` with a single signal goroutine and a `doneCh` goroutine, no races are expected, but enabling `-race` confirms this.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `errors.As(err, &exitErr)` requires `syscall.WaitStatus` cast for exit code | `exitErr.ExitCode()` method | Go 1.12 | Project already uses correct modern API |
| `cmd.Cancel` field for signal on context cancel | New in Go 1.20 | Go 1.20 | Not needed for these tests; using `cmd.Process.Signal()` directly |
| `testenv.Executable(t)` for test binary path | `os.Args[0]` | Always | `os.Args[0]` is simpler for same-package tests; `testenv` is stdlib internal |
| `signal.NotifyContext` for signal testing | `signal.Notify` + buffered channel | Go 1.16 | `signal.Notify` is correct here; `NotifyContext` adds context plumbing not needed |

**Deprecated/outdated:**
- Using `syscall.WaitStatus.ExitStatus()` directly: replaced by `ProcessState.ExitCode()` since Go 1.12. The `resolveExitCode` function in runner.go already uses the modern API correctly.
- `Process.Kill()` as a test fixture: too blunt. Use `Process.Signal(syscall.SIGTERM)` for TEST-04 to match what process.Run() actually handles.

---

## Open Questions

1. **Is 200ms sleep reliable for signal timing on this machine?**
   - What we know: The system runs Linux 6.18.7. Process startup for a re-exec'd test binary typically takes 10-50ms.
   - What's unclear: Under extreme load (CI, resource-constrained), 200ms may still be too short.
   - Recommendation: Use 200ms for v0. If TEST-04 is flaky in CI, replace sleep with a pipe-based readiness signal from the helper.

2. **Does runner.go's use of `os.Stderr` directly complicate test observability?**
   - What we know: Capturing PID from stderr requires the re-exec helper with `cmd.Stderr = &buf`.
   - What's unclear: Whether the zombie test needs to extract PID from log output or can use another method.
   - Recommendation: Use re-exec helper + `cmd.Stderr = &buf` for the zombie test. For exit code tests, stderr output is irrelevant — just check the return value.

3. **Should `go vet ./...` be scoped to `./internal/process/...` to avoid the pre-existing build failure in `internal/api/tasks_test.go`?**
   - What we know: `go vet ./...` currently fails due to `runtimex/api-service/internal/models` import issues in the pre-existing `internal/api/tasks_test.go`. This is documented in `.planning/phases/02-signal-forwarding/deferred-items.md`.
   - What's unclear: Whether the plan should gate success on `go vet ./internal/process/...` (narrower scope) rather than `./...`.
   - Recommendation: Scope the success criterion to `go vet ./internal/process/...` and note the pre-existing API package issue as out of scope.

---

## Sources

### Primary (HIGH confidence)

- [pkg.go.dev/os/exec](https://pkg.go.dev/os/exec) — ProcessState.ExitCode(), cmd.Process.Signal(), Wait() behavior after signal
- [pkg.go.dev/os/signal](https://pkg.go.dev/os/signal) — signal.Notify buffered channel requirement
- [go.dev/src/os/exec/exec_test.go](https://go.dev/src/os/exec/exec_test.go) — helperCommands pattern, TestMain re-exec approach, exit code test patterns
- [go.dev/src/os/signal/signal_test.go](https://go.dev/src/os/signal/signal_test.go) — signal delivery test patterns
- `/proc/<pid>/status` man page (Linux) — `State: Z` format for zombie detection
- Direct codebase read: `internal/process/runner.go` (current implementation), `internal/docker/validator_test.go` (project test conventions)

### Secondary (MEDIUM confidence)

- [rednafi.com/go/test-subprocesses/](https://rednafi.com/go/test-subprocesses/) — Re-exec pattern explanation, TestHelperProcess guard pattern
- [segmentfault.com/a/1190000041466423/en](https://segmentfault.com/a/1190000041466423/en) — Go exec zombie process mechanics
- [mezhenskyi.dev/posts/go-linux-processes/](https://mezhenskyi.dev/posts/go-linux-processes/) — Linux process management in Go, `/proc` filesystem usage

### Tertiary (LOW confidence)

- None — all critical claims verified via primary or secondary sources.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib, verified against Go 1.25.7 on this machine
- Architecture patterns: HIGH — re-exec pattern confirmed from Go stdlib test source; direct `Run()` call for exit codes is trivially correct
- Pitfalls: HIGH — zombie check `/proc` semantics confirmed; signal timing is MEDIUM (200ms is empirical, not proven)
- TEST-06 (manual): HIGH — verified that real-time streaming cannot be automated without PTY in go test

**Research date:** 2026-02-28
**Valid until:** 2026-03-30 (stable domain — stdlib testing APIs do not change between Go minor releases)
