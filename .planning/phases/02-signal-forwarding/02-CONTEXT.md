# Phase 2: Signal Forwarding - Context

**Gathered:** 2026-02-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Layer signal interception and forwarding on top of the Phase 1 process runner. SIGINT and SIGTERM are intercepted by rtx, forwarded to the child, and rtx waits for the child to finish before exiting with the child's code. Signal-killed exit code emulation (128+N) for correct POSIX behavior. No new CLI flags or subcommands in this phase.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
- Shutdown timeout behavior (wait forever vs force-kill after timeout)
- Double Ctrl+C / signal escalation strategy
- Signal log message format and verbosity
- Whether to forward to child PID only or process group
- Signal channel buffer size and goroutine structure
- Error handling for forwarding to already-dead process
- All implementation patterns and architecture choices

User trusts research recommendations and standard patterns. Research already covers this domain with HIGH confidence.

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. Research recommends:
- Setpgid: true already set in Phase 1 — explicit forwarding is mandatory
- Buffered signal channel (capacity 1) per research pitfalls
- Swallow `os.ErrProcessDone` when forwarding to dead process

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-signal-forwarding*
*Context gathered: 2026-02-28*
