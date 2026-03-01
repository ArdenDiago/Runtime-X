# Phase 5: Scheduler Data Structures and Log Buffer - Context

**Gathered:** 2026-03-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Define the core data types for the process scheduler: `ProcessDef` (what to run), `ManagedProcess` (runtime state + log buffer), `State` (lifecycle enum), and a mutex-safe `logBuffer` ring buffer. This phase delivers types and registration — actual process lifecycle (start/stop) is Phase 6, dependency ordering is Phase 7, restart policies are Phase 8.

</domain>

<decisions>
## Implementation Decisions

### Process definition shape
- Include env vars and working dir on ProcessDef from the start (name, command, args, restart policy, env, working dir)
- Add a `DependsOn []string` field now on ProcessDef — scheduler ignores it until Phase 7 wires up ordering
- Restart policy is a struct (`RestartPolicy`) with Mode (always/on-failure/never), MaxRetries, and Delay fields
- Process names are validated: slug-like format (lowercase, alphanumeric, hyphens) — prevents issues with API paths, logging, and display

### Log buffer behavior
- Ring buffer size configurable per process via a `LogBufferSize` field on ProcessDef, with a sane default
- Capture combined stdout + stderr with stream tags — single buffer, each entry tagged with source (stdout/stderr)
- Structured log entries: each entry is a struct with Timestamp, Stream (stdout/stderr), and Text fields
- Default buffer size: 1000 lines (~100KB per process)

### State lifecycle
- Moderate granularity (6 states): Idle, Starting, Running, Stopping, Stopped, Failed
- Validated state transitions — define a transition table, invalid transitions return an error
- Include runtime metadata from the start: StartedAt, StoppedAt, ExitCode, RestartCount on ManagedProcess
- Polling only for now — consumers read current state when needed. Events/channels added in later phases if needed

### Registration and storage
- Programmatic API only in this phase — `scheduler.Register(def)`. Config file parsing comes in a later phase
- Enforce unique process names — Register returns error if name already exists. Names are the primary identifier
- Instantiable struct — `scheduler.New()` returns a `*Scheduler`. Testable, composable, no global state
- Processes are removable — `scheduler.Remove(name)` unregisters a stopped process. Enables dynamic management via the API

### Claude's Discretion
- Exact mutex strategy (sync.Mutex vs sync.RWMutex on the ring buffer)
- Internal slice vs array backing for the ring buffer
- Whether to expose log buffer as a separate type or embed it in ManagedProcess
- Error message wording for invalid transitions and duplicate registrations

</decisions>

<specifics>
## Specific Ideas

- Scheduler should feel like Go's `http.NewServeMux()` pattern — instantiable, testable, passed as dependency
- Ring buffer should be its own well-tested type that ManagedProcess composes
- State transition table approach similar to how finite state machines are typically implemented in Go (map of valid transitions)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-scheduler-data-structures-and-log-buffer*
*Context gathered: 2026-03-01*
