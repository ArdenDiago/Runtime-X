# Phase 5: Scheduler Data Structures and Log Buffer - Research

**Researched:** 2026-03-01
**Domain:** Go stdlib concurrency — sync primitives, ring buffer, FSM state transitions, struct design
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Process definition shape:**
- Include env vars and working dir on ProcessDef from the start (name, command, args, restart policy, env, working dir)
- Add a `DependsOn []string` field now on ProcessDef — scheduler ignores it until Phase 7 wires up ordering
- Restart policy is a struct (`RestartPolicy`) with Mode (always/on-failure/never), MaxRetries, and Delay fields
- Process names are validated: slug-like format (lowercase, alphanumeric, hyphens) — prevents issues with API paths, logging, and display

**Log buffer behavior:**
- Ring buffer size configurable per process via a `LogBufferSize` field on ProcessDef, with a sane default
- Capture combined stdout + stderr with stream tags — single buffer, each entry tagged with source (stdout/stderr)
- Structured log entries: each entry is a struct with Timestamp, Stream (stdout/stderr), and Text fields
- Default buffer size: 1000 lines (~100KB per process)

**State lifecycle:**
- Moderate granularity (6 states): Idle, Starting, Running, Stopping, Stopped, Failed
- Validated state transitions — define a transition table, invalid transitions return an error
- Include runtime metadata from the start: StartedAt, StoppedAt, ExitCode, RestartCount on ManagedProcess
- Polling only for now — consumers read current state when needed. Events/channels added in later phases if needed

**Registration and storage:**
- Programmatic API only in this phase — `scheduler.Register(def)`. Config file parsing comes in a later phase
- Enforce unique process names — Register returns error if name already exists. Names are the primary identifier
- Instantiable struct — `scheduler.New()` returns a `*Scheduler`. Testable, composable, no global state
- Processes are removable — `scheduler.Remove(name)` unregisters a stopped process. Enables dynamic management via the API

### Claude's Discretion
- Exact mutex strategy (sync.Mutex vs sync.RWMutex on the ring buffer)
- Internal slice vs array backing for the ring buffer
- Whether to expose log buffer as a separate type or embed it in ManagedProcess
- Error message wording for invalid transitions and duplicate registrations

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SCH-01 | User can register a process definition (name, command, args, restart policy) with the scheduler and it is stored | ProcessDef struct design, Scheduler.Register() API pattern, name validation with regexp, unique name enforcement via map lookup |
| SCH-05 | Each process's stdout and stderr are captured in a per-process ring buffer (not direct fd to parent) | LogBuffer ring buffer design with mutex, LogEntry struct (Timestamp/Stream/Text), Write() concurrency safety |
| SCH-06 | User can retrieve recent log lines from a process's ring buffer | LogBuffer.Lines() method returning snapshot slice, mutex protection for concurrent reads, most-recent-N semantics |
</phase_requirements>

---

## Summary

This phase is a pure data modeling and synchronization phase — no external libraries needed, no I/O, no goroutine launching. The work is entirely in `internal/scheduler/` and uses only Go stdlib (`sync`, `regexp`, `time`, `errors`). Three distinct problems must be solved correctly: (1) a mutex-safe ring buffer that evicts oldest entries on overflow, (2) validated finite-state-machine transitions for the 6-state process lifecycle, and (3) a Scheduler struct that stores ProcessDef and ManagedProcess pairs under a single top-level RWMutex.

The critical insight from STATE.md is that the log buffer needs its own independent `sync.Mutex` — separate from the Scheduler's top-level RWMutex — because log writes arrive from goroutines launched by `cmd.Start()` in Phase 6. If the log buffer shared the scheduler lock, Phase 6 would face deadlock: the scheduler holds the write lock while launching a process, but the goroutine reading stdout needs to call `Write()` on the log buffer. The independent mutex on the log buffer makes this safe.

The ring buffer backing should use a `[]LogEntry` slice with head/tail indices and an explicit length counter. The `sync.Mutex` (not RWMutex) is correct for the log buffer because writes from goroutines and reads from HTTP handlers have roughly equal frequency — RWMutex overhead is not justified and can actually hurt performance at balanced read/write ratios. The Scheduler's top-level map guard (processes by name) uses `sync.RWMutex` because list/get operations (reads) heavily outnumber register/remove operations (writes).

**Primary recommendation:** Build three files in `internal/scheduler/`: `types.go` (all struct definitions and constants), `logbuffer.go` (the ring buffer with its own mutex), and `scheduler.go` (Scheduler struct with Register/Remove/Get/List). Write table-driven race tests before implementing; let `-race` be the arbiter of correctness.

---

## Standard Stack

### Core
| Package | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `sync` (stdlib) | Go 1.25.5 | Mutex, RWMutex for concurrent access control | No external dependency; correct primitives for the access pattern |
| `regexp` (stdlib) | Go 1.25.5 | Compile and validate process name slug pattern | Thread-safe compiled regexp; MustCompile at init time |
| `time` (stdlib) | Go 1.25.5 | Timestamps on LogEntry, StartedAt/StoppedAt on ManagedProcess | Standard Go time type |
| `errors` (stdlib) | Go 1.25.5 | sentinel errors for invalid transitions, duplicate names | errors.Is() compatible sentinel pattern |
| `testing` (stdlib) | Go 1.25.5 | Race-tested unit tests | `go test -race ./internal/scheduler/...` |

### Supporting
| Package | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `fmt` (stdlib) | Go 1.25.5 | Error message formatting | Error wording in Register, Transition |

### Alternatives Considered
| Standard Choice | Could Use | Tradeoff |
|-----------------|-----------|----------|
| `sync.Mutex` on LogBuffer | `sync.RWMutex` | Writes and reads are roughly equal in frequency; RWMutex overhead is higher at balanced ratios — Mutex is simpler and faster here |
| `sync.RWMutex` on Scheduler map | `sync.Mutex` | Scheduler map is read-heavy (list/get >> register/remove) — RWMutex is justified |
| Slice-backed ring buffer | channel-based ring buffer | Channel approach is elegant but adds buffered-channel semantics, blocking on full; slice + mutex gives explicit overwrite-oldest behavior without blocking writers |
| Separate `logBuffer` type | Embedded fields in ManagedProcess | Separate type keeps it testable in isolation, composable, and independently locked |
| Sentinel error vars | Custom error types | Sufficient for this phase; callers use errors.Is() |

**Installation:** No external packages needed. All work is Go stdlib only.

---

## Architecture Patterns

### Recommended Project Structure
```
internal/scheduler/
├── types.go          # ProcessDef, RestartPolicy, RestartMode, ManagedProcess, State, LogEntry
├── logbuffer.go      # logBuffer type, Write(), Lines(), Len(), Reset()
├── scheduler.go      # Scheduler struct, New(), Register(), Remove(), Get(), List()
└── scheduler_test.go # Table-driven tests including concurrent race tests
```

### Pattern 1: Independent Mutex on logBuffer

**What:** The log buffer carries its own `sync.Mutex` completely independent of the Scheduler's `sync.RWMutex`. The Scheduler stores a `*logBuffer` inside `ManagedProcess`.

**When to use:** Any time a nested type will be written by goroutines that don't hold the outer lock (here: goroutines reading process stdout in Phase 6).

**Why:** STATE.md explicitly records: "logBuffer needs its own sync.Mutex independent of scheduler RWMutex (log writes come from cmd.Start() goroutines)." This prevents the Phase 6 deadlock scenario where the scheduler holds a write lock during `cmd.Start()` while a log-reader goroutine tries to call `logBuffer.Write()`.

```go
// Source: Go sync package official docs + STATE.md architectural decision
type logBuffer struct {
    mu      sync.Mutex  // independent of Scheduler.mu
    entries []LogEntry
    size    int
    head    int // index of next write position
    count   int // number of valid entries (0..size)
}

type ManagedProcess struct {
    Def         ProcessDef
    State       State
    StartedAt   time.Time
    StoppedAt   time.Time
    ExitCode    int
    RestartCount int
    logs        *logBuffer // unexported; accessed via scheduler methods
}
```

### Pattern 2: Map-Based FSM Transition Table

**What:** A package-level `validTransitions` map encodes which state transitions are allowed. `Transition()` looks up the current state, checks if the target state is in the allowed set, and either advances state or returns a sentinel error.

**When to use:** When invalid transitions must be caught and reported rather than silently allowed. The table makes all valid paths explicit and visible.

```go
// Source: FSM pattern from venilnoronha.io/a-simple-state-machine-framework-in-go
// verified as standard Go idiom

type State int

const (
    StateIdle State = iota
    StateStarting
    StateRunning
    StateStopping
    StateStopped
    StateFailed
)

var ErrInvalidTransition = errors.New("invalid state transition")

// validTransitions maps each State to the set of States it can transition to.
var validTransitions = map[State][]State{
    StateIdle:     {StateStarting},
    StateStarting: {StateRunning, StateFailed},
    StateRunning:  {StateStopping, StateFailed},
    StateStopping: {StateStopped, StateFailed},
    StateStopped:  {StateStarting}, // allows restart
    StateFailed:   {StateStarting}, // allows retry
}

func canTransition(from, to State) bool {
    for _, allowed := range validTransitions[from] {
        if allowed == to {
            return true
        }
    }
    return false
}
```

### Pattern 3: Scheduler as Instantiable Struct (http.NewServeMux idiom)

**What:** `scheduler.New()` returns a `*Scheduler` value with an initialized map. No package-level global state. Passed as a dependency to HTTP handlers in Phase 9.

**When to use:** Always — the user decision locked this pattern.

```go
// Source: Go standard library pattern (http.NewServeMux), STATE.md decision
type Scheduler struct {
    mu        sync.RWMutex
    processes map[string]*ManagedProcess
}

func New() *Scheduler {
    return &Scheduler{
        processes: make(map[string]*ManagedProcess),
    }
}
```

### Pattern 4: Ring Buffer with Head Index and Count

**What:** A fixed-capacity slice where `head` points to the next write position and wraps around with modulo. `count` tracks how many valid entries exist (0..size). On overflow, `head` advances past the oldest entry automatically.

**When to use:** When you need bounded memory for streaming output with evict-oldest semantics.

```go
// Source: standard ring buffer algorithm, verified against multiple Go implementations
func (lb *logBuffer) Write(entry LogEntry) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    lb.entries[lb.head] = entry
    lb.head = (lb.head + 1) % lb.size
    if lb.count < lb.size {
        lb.count++
    }
    // When count == size, head has advanced past the oldest entry (overwrite)
}

// Lines returns a snapshot of all entries in chronological order.
func (lb *logBuffer) Lines() []LogEntry {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    if lb.count == 0 {
        return nil
    }
    out := make([]LogEntry, lb.count)
    if lb.count < lb.size {
        // Buffer not yet full: entries start at index 0
        copy(out, lb.entries[:lb.count])
    } else {
        // Buffer full: oldest entry is at head (next write position)
        n := copy(out, lb.entries[lb.head:])
        copy(out[n:], lb.entries[:lb.head])
    }
    return out
}
```

### Pattern 5: Pre-compiled Name Validation Regexp

**What:** A package-level compiled `*regexp.Regexp` for slug validation. Compiled once at init, used in `Register()`. Thread-safe for concurrent use.

```go
// Source: pkg.go.dev/regexp official docs
var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func validateName(name string) error {
    if name == "" {
        return errors.New("process name cannot be empty")
    }
    if !validName.MatchString(name) {
        return fmt.Errorf("process name %q is invalid: must match ^[a-z0-9][a-z0-9-]*$", name)
    }
    return nil
}
```

Note: The pattern `^[a-z0-9][a-z0-9-]*$` requires at least one character, starts with alphanumeric (not hyphen), and allows hyphens in the middle. This is stricter than `^[a-z0-9-]+$` to prevent names like `-foo`.

### Anti-Patterns to Avoid

- **Sharing the Scheduler's RWMutex with the log buffer:** Causes deadlock in Phase 6. The log buffer MUST have its own independent mutex.
- **Using `sync.RWMutex` for the log buffer:** Log writes and log reads happen at similar rates (every output line is a write; every HTTP /logs request is a read). RWMutex performs worse than Mutex when writes are frequent.
- **Using a package-level `var processes map[string]*ManagedProcess`:** Prevents testing multiple schedulers in parallel, creates implicit coupling. Use `New()` constructor.
- **Holding the Scheduler write lock across `cmd.Start()` (Phase 6 concern, but design for it now):** STATE.md flags this — release the write lock before calling `cmd.Start()`. Relevant to how Phase 6 will use these types.
- **Returning log entries as a live slice (not a snapshot):** Callers could mutate the buffer's internal state. `Lines()` must return a copy (new `[]LogEntry` allocated with `make()`).
- **Not testing the full-buffer overwrite path:** The ring buffer's most subtle behavior is when `count == size`. Tests must fill the buffer past capacity and verify that oldest entries are evicted.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Name validation | Custom string-walking code | `regexp.MustCompile` + `.MatchString()` | One line, thread-safe, correct edge cases |
| Concurrent map access | Unguarded map reads/writes | `sync.RWMutex` guarding the processes map | Go maps are NOT concurrency-safe; unguarded access causes race detector failures and runtime panics |
| Sentinel errors | Custom error structs with codes | `var ErrX = errors.New("...")` package-level sentinels | Compatible with `errors.Is()`; sufficient for this phase |

**Key insight:** The ring buffer is the one piece that must be custom-built because it needs evict-oldest semantics with struct-typed entries. There is no stdlib ring buffer. Do not reach for a third-party package (`smallnest/ringbuffer` etc.) — the requirement is a handful of methods on a small struct, and adding a dependency for ~50 lines of code is unnecessary complexity.

---

## Common Pitfalls

### Pitfall 1: Shared Lock Between Scheduler and LogBuffer Causes Deadlock (Phase 6 Preview)

**What goes wrong:** If `logBuffer.Write()` acquires the Scheduler's `sync.RWMutex` (even as a read lock), then Phase 6 code that holds the write lock while starting a process will deadlock: the scheduler holds write, the goroutine reading stdout calls `Write()` which tries to acquire write lock — blocks forever.

**Why it happens:** Nested locking with the same mutex, or incorrectly embedding the log buffer under the scheduler lock hierarchy.

**How to avoid:** The log buffer struct owns its own `sync.Mutex` field. The Scheduler's `mu` is never held when calling any `logBuffer` method. Design enforced by the independent struct type.

**Warning signs:** Tests involving concurrent `Write()` + `scheduler.Get()` hang indefinitely instead of completing.

---

### Pitfall 2: Ring Buffer Returns Stale Ordering When Full

**What goes wrong:** When the ring buffer is full, the oldest entry is at `entries[head]` (the next write position wraps around). If `Lines()` returns `entries[0:]` directly, entries are in wrong chronological order.

**Why it happens:** Forgetting that the head pointer has lapped position 0 after the buffer fills.

**How to avoid:** In `Lines()`, always check if `count < size`. If equal (full), reconstruct chronological order by reading from `head` to end, then `0` to `head-1`. Return a new allocated slice.

**Warning signs:** Tests that verify "latest N lines" get entries out of order after filling past capacity.

---

### Pitfall 3: Race Detector Fails on LogBuffer with Concurrent Write + Lines

**What goes wrong:** `go test -race` reports a race when one goroutine calls `Write()` while another calls `Lines()` if the mutex is missing or incorrectly deferred.

**Why it happens:** Missing `defer lb.mu.Unlock()` or using a value receiver (copies the mutex). The race detector catches memory accesses to `lb.entries`, `lb.head`, and `lb.count` that happen concurrently.

**How to avoid:** Always use pointer receivers on methods that acquire the mutex. Always `defer mu.Unlock()` immediately after `mu.Lock()`. Never copy a struct containing a mutex (the `go vet` tool flags this as `sync.Mutex must not be copied after first use`).

**Warning signs:** `go vet ./internal/scheduler/...` reports "locks are passed by value" or race detector logs mention the `entries` slice field.

---

### Pitfall 4: Invalid State Transitions Silently Succeed

**What goes wrong:** Without a transition table, code in Phase 6 accidentally sets `ManagedProcess.State = StateRunning` from `StateStopped` without going through `StateStarting`. The process appears running but its internal state is inconsistent.

**Why it happens:** Direct field assignment instead of a `Transition(to State) error` method.

**How to avoid:** Make `State` unexported or enforce all mutations through a `Transition()` method that validates against the table and returns `ErrInvalidTransition` on invalid attempts. Tests should verify that `StateStopped -> StateRunning` returns an error.

**Warning signs:** Phase 6 sets state directly with `mp.State = StateRunning` instead of `mp.Transition(StateRunning)`.

---

### Pitfall 5: LogBufferSize of 0 Panics on Modulo

**What goes wrong:** `logBuffer.Write()` computes `lb.head = (lb.head + 1) % lb.size`. If `lb.size == 0` (zero-value or misconfigured), this is a divide-by-zero panic.

**Why it happens:** `ProcessDef.LogBufferSize` left at zero (Go zero value), no default applied.

**How to avoid:** In `Register()`, apply the default if `def.LogBufferSize == 0`: `def.LogBufferSize = 1000`. Validate that `LogBufferSize > 0` before constructing the log buffer.

**Warning signs:** Panic `runtime error: integer divide by zero` in `logBuffer.Write()` during tests.

---

### Pitfall 6: Process Name Regex Allows Leading Hyphen

**What goes wrong:** Pattern `^[a-z0-9-]+$` accepts names like `-my-process` which breaks API URL path construction and display.

**Why it happens:** Forgetting to anchor the first character separately.

**How to avoid:** Use `^[a-z0-9][a-z0-9-]*$` which requires the first character to be alphanumeric. Test with `-leading`, `trailing-`, `UPPER`, `under_score`, `valid-name-123`.

---

## Code Examples

Verified patterns from official sources and established Go idioms:

### Complete LogBuffer Type

```go
// Source: Standard Go ring buffer algorithm + sync package (pkg.go.dev/sync)
// Confidence: HIGH — verified against official sync docs and multiple implementations

package scheduler

import (
    "sync"
    "time"
)

// Stream identifies which output stream a log entry came from.
type Stream string

const (
    StreamStdout Stream = "stdout"
    StreamStderr Stream = "stderr"
)

// LogEntry is a single captured output line with metadata.
type LogEntry struct {
    Timestamp time.Time
    Stream    Stream
    Text      string
}

// logBuffer is a bounded ring buffer of LogEntry values.
// It has its own mutex, independent of the Scheduler's mutex,
// so goroutines writing stdout/stderr can call Write() concurrently
// without acquiring the Scheduler lock.
type logBuffer struct {
    mu      sync.Mutex
    entries []LogEntry
    size    int
    head    int // index of next write position (wraps)
    count   int // number of valid entries [0, size]
}

func newLogBuffer(size int) *logBuffer {
    if size <= 0 {
        size = 1000 // default
    }
    return &logBuffer{
        entries: make([]LogEntry, size),
        size:    size,
    }
}

// Write appends an entry. If the buffer is full, the oldest entry is overwritten.
// Safe to call from multiple goroutines concurrently.
func (lb *logBuffer) Write(entry LogEntry) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    lb.entries[lb.head] = entry
    lb.head = (lb.head + 1) % lb.size
    if lb.count < lb.size {
        lb.count++
    }
}

// Lines returns a snapshot of all entries in chronological order (oldest first).
// Safe to call concurrently with Write().
func (lb *logBuffer) Lines() []LogEntry {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    if lb.count == 0 {
        return nil
    }
    out := make([]LogEntry, lb.count)
    if lb.count < lb.size {
        // Buffer not yet wrapped: entries are at [0, count)
        copy(out, lb.entries[:lb.count])
    } else {
        // Buffer full: oldest entry is at head
        n := copy(out, lb.entries[lb.head:])
        copy(out[n:], lb.entries[:lb.head])
    }
    return out
}

// Len returns the number of entries currently stored.
func (lb *logBuffer) Len() int {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    return lb.count
}
```

### State Type and Transition Table

```go
// Source: FSM map-pattern from venilnoronha.io + errors package (pkg.go.dev/errors)
// Confidence: HIGH

package scheduler

import (
    "errors"
    "fmt"
)

// State represents the lifecycle state of a managed process.
type State int

const (
    StateIdle     State = iota // Registered, never started
    StateStarting              // cmd.Start() called, not yet confirmed running
    StateRunning               // Process is alive
    StateStopping              // SIGTERM sent, awaiting exit
    StateStopped               // Exited cleanly (exit code 0 or expected)
    StateFailed                // Exited unexpectedly or max retries exceeded
)

func (s State) String() string {
    switch s {
    case StateIdle:     return "idle"
    case StateStarting: return "starting"
    case StateRunning:  return "running"
    case StateStopping: return "stopping"
    case StateStopped:  return "stopped"
    case StateFailed:   return "failed"
    default:            return "unknown"
    }
}

// ErrInvalidTransition is returned when Transition() is called with a
// state that is not reachable from the current state.
var ErrInvalidTransition = errors.New("invalid state transition")

// validTransitions defines the allowed state machine edges.
var validTransitions = map[State][]State{
    StateIdle:     {StateStarting},
    StateStarting: {StateRunning, StateFailed},
    StateRunning:  {StateStopping, StateFailed},
    StateStopping: {StateStopped, StateFailed},
    StateStopped:  {StateStarting},
    StateFailed:   {StateStarting},
}

// canTransition reports whether moving from -> to is allowed.
func canTransition(from, to State) bool {
    for _, allowed := range validTransitions[from] {
        if allowed == to {
            return true
        }
    }
    return false
}
```

### ProcessDef and ManagedProcess Structs

```go
// Source: decisions from 05-CONTEXT.md
// Confidence: HIGH (exact shape locked by user decisions)

package scheduler

import "time"

// RestartMode controls when a process is restarted after exit.
type RestartMode string

const (
    RestartAlways    RestartMode = "always"
    RestartOnFailure RestartMode = "on-failure"
    RestartNever     RestartMode = "never"
)

// RestartPolicy configures automatic restart behavior.
type RestartPolicy struct {
    Mode       RestartMode
    MaxRetries int           // 0 = unlimited (for RestartAlways/RestartOnFailure)
    Delay      time.Duration // initial delay between restarts
}

// ProcessDef is the static definition of a process (what to run).
// Immutable after registration.
type ProcessDef struct {
    Name          string
    Command       string
    Args          []string
    Env           []string          // additional environment variables (KEY=VALUE)
    WorkDir       string            // working directory; "" = inherit from server
    RestartPolicy RestartPolicy
    DependsOn     []string          // ignored until Phase 7
    LogBufferSize int               // 0 = use default (1000)
}

// ManagedProcess is the runtime state of a registered process.
// The logs field has its own mutex — do NOT hold Scheduler.mu when calling
// any logs method, or Phase 6 goroutines will deadlock.
type ManagedProcess struct {
    Def          ProcessDef
    State        State
    StartedAt    time.Time
    StoppedAt    time.Time
    ExitCode     int
    RestartCount int
    logs         *logBuffer
}
```

### Scheduler Struct and Registration

```go
// Source: http.NewServeMux idiom, sync.RWMutex docs (pkg.go.dev/sync)
// Confidence: HIGH

package scheduler

import (
    "errors"
    "fmt"
    "regexp"
    "sync"
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ErrNotFound is returned when looking up a process name that is not registered.
var ErrNotFound = errors.New("process not found")

// ErrAlreadyExists is returned when registering a name that is already in use.
var ErrAlreadyExists = errors.New("process name already registered")

// ErrNotStopped is returned when attempting to remove a process that is not stopped.
var ErrNotStopped = errors.New("process is not stopped")

// Scheduler manages a set of named process definitions and their runtime state.
// Create with New(); do not use the zero value directly.
type Scheduler struct {
    mu        sync.RWMutex
    processes map[string]*ManagedProcess
}

// New returns a ready-to-use Scheduler.
func New() *Scheduler {
    return &Scheduler{
        processes: make(map[string]*ManagedProcess),
    }
}

// Register adds a new process definition to the scheduler.
// Returns ErrAlreadyExists if a process with that name is already registered.
// Returns an error if the name is invalid.
func (s *Scheduler) Register(def ProcessDef) error {
    if err := validateName(def.Name); err != nil {
        return err
    }
    if def.LogBufferSize <= 0 {
        def.LogBufferSize = 1000
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    if _, exists := s.processes[def.Name]; exists {
        return fmt.Errorf("%w: %s", ErrAlreadyExists, def.Name)
    }
    s.processes[def.Name] = &ManagedProcess{
        Def:  def,
        State: StateIdle,
        logs: newLogBuffer(def.LogBufferSize),
    }
    return nil
}

// Remove unregisters a stopped process. Returns ErrNotFound if the name
// is not registered. Returns ErrNotStopped if the process is still running.
func (s *Scheduler) Remove(name string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    mp, exists := s.processes[name]
    if !exists {
        return fmt.Errorf("%w: %s", ErrNotFound, name)
    }
    if mp.State != StateStopped && mp.State != StateIdle && mp.State != StateFailed {
        return fmt.Errorf("%w: %s", ErrNotStopped, name)
    }
    delete(s.processes, name)
    return nil
}

// Get returns the ManagedProcess for name, or ErrNotFound.
func (s *Scheduler) Get(name string) (*ManagedProcess, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    mp, exists := s.processes[name]
    if !exists {
        return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
    }
    return mp, nil
}

// List returns a snapshot of all registered processes.
// The returned slice is safe to iterate; individual ManagedProcess pointers
// still point into the scheduler — callers must not mutate State directly.
func (s *Scheduler) List() []*ManagedProcess {
    s.mu.RLock()
    defer s.mu.RUnlock()
    result := make([]*ManagedProcess, 0, len(s.processes))
    for _, mp := range s.processes {
        result = append(result, mp)
    }
    return result
}

func validateName(name string) error {
    if name == "" {
        return errors.New("process name cannot be empty")
    }
    if !validName.MatchString(name) {
        return fmt.Errorf("process name %q is invalid: must be lowercase alphanumeric with hyphens, starting with alphanumeric", name)
    }
    return nil
}
```

### Race-Tested Concurrent LogBuffer Test

```go
// Source: go.dev/doc/articles/race_detector official pattern
// Run with: go test -race ./internal/scheduler/...

func TestLogBufferConcurrentWriteAndRead(t *testing.T) {
    lb := newLogBuffer(100)
    const writers = 10
    const writes = 50

    var wg sync.WaitGroup
    for i := 0; i < writers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < writes; j++ {
                lb.Write(LogEntry{
                    Timestamp: time.Now(),
                    Stream:    StreamStdout,
                    Text:      "line",
                })
            }
        }()
    }
    // Concurrent reader
    done := make(chan struct{})
    go func() {
        for {
            select {
            case <-done:
                return
            default:
                lb.Lines()
            }
        }
    }()

    wg.Wait()
    close(done)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Global `var processes map[string]*Process` | Instantiable `*Scheduler` with `New()` | Post-2018 Go idiom | Testable in parallel, no test pollution |
| Storing raw `[]byte` log lines | Structured `LogEntry{Timestamp, Stream, Text}` | Best practice from structured logging adoption | Enables filtering, timestamp display, stream separation in API |
| Unbounded log capture (write to file) | Bounded ring buffer with evict-oldest | Process manager maturity | Predictable memory usage per process |
| Direct state field assignment | FSM transition table with validation | Type-safe state machine patterns | Prevents impossible state combinations from reaching Phase 6+ |

**No deprecated items in this phase** — all patterns are current Go stdlib idiom.

---

## Open Questions

1. **Transition method placement: on `*ManagedProcess` or on `*Scheduler`?**
   - What we know: `ManagedProcess` holds the state field; `Scheduler` holds the mutex that protects it.
   - What's unclear: Should `Transition(to State) error` live on `ManagedProcess` (natural owner) or be a `Scheduler.Transition(name string, to State) error` method that holds the lock?
   - Recommendation: Put `Transition()` as an unexported method on `*ManagedProcess`, called only from within Scheduler methods that already hold the write lock. This keeps the transition logic testable via direct struct manipulation in unit tests while ensuring callers outside the package can't bypass the lock.

2. **What does `Remove()` do with the log buffer?**
   - What we know: `Remove()` deletes the `*ManagedProcess` from the map; the `*logBuffer` will be GC'd.
   - What's unclear: Phase 6 goroutines hold a reference to the `*logBuffer` via their goroutine closure. If `Remove()` runs while the goroutine is still writing, the goroutine writes to an unreferenced buffer — this is safe (no use-after-free in Go) but the writes go nowhere. The Scheduler must enforce that the process is stopped (goroutine done) before allowing Remove.
   - Recommendation: Remove already enforces `State in {Stopped, Idle, Failed}`. Phase 6 must set state to Stopped/Failed only after the goroutine has exited. This invariant keeps Remove safe.

3. **LogLines accessor: on Scheduler or exposed via ManagedProcess?**
   - What we know: HTTP handlers in Phase 9 will call something like `GET /api/processes/:id/logs`. The handler needs to call into the scheduler.
   - What's unclear: Should `Scheduler.Logs(name string) ([]LogEntry, error)` be a method, or should `Get()` return the `*ManagedProcess` and callers call `mp.Logs()`?
   - Recommendation: Add `Scheduler.Logs(name string) ([]LogEntry, error)` that internally calls `mp.logs.Lines()`. This keeps the log buffer field unexported on `ManagedProcess` and gives Phase 9 a clean single-entry-point API.

---

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/sync` — Mutex, RWMutex semantics, method signatures, RWMutex writer-priority rule
- `go.dev/doc/articles/race_detector` — Race detector flags, test patterns, `go test -race` usage
- `pkg.go.dev/regexp` — MustCompile, MatchString, thread-safety guarantee for compiled Regexp

### Secondary (MEDIUM confidence)
- `venilnoronha.io/a-simple-state-machine-framework-in-go` — Map-based FSM transition pattern, ErrEventRejected sentinel approach, verified against standard Go idiom
- Multiple Go ring buffer implementations cross-referenced — slice + mutex approach, head/count/size fields verified across medium.com/checker-engineering, joshrosso.com/c/ring-buffer

### Tertiary (LOW confidence — but all consistent with primary sources)
- `leapcell.io/blog/concurrency-control-in-go-mastering-mutex-and-rwmutex` — RWMutex performance degradation at 50/50 read-write ratio (supports Mutex choice for logBuffer); consistent with official sync docs

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib, no version ambiguity, Go 1.25.5 confirmed in go.mod
- Architecture patterns: HIGH — ring buffer algorithm is deterministic; mutex patterns verified against official docs; FSM table pattern verified against established Go implementations
- Pitfalls: HIGH — deadlock pitfall comes directly from STATE.md architectural decision; others are mechanically derived from the synchronization model
- Code examples: HIGH — code examples are consistent with verified official API signatures

**Research date:** 2026-03-01
**Valid until:** 2026-09-01 (60 days — stdlib is extremely stable; ring buffer algorithm is timeless)
