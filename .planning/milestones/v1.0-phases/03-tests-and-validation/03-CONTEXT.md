# Phase 3: Tests and Validation - Context

**Gathered:** 2026-02-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Write unit tests for `internal/process/` covering exit code propagation, zombie prevention, signal delivery, and command-not-found handling. Run manual validation against the "Looks Done But Isn't" checklist from PITFALLS.md. No new features — this phase proves Phase 1 and Phase 2 are correct.

</domain>

<decisions>
## Implementation Decisions

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

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. Research PITFALLS.md has a "Looks Done But Isn't" checklist that should be the basis for manual validation.

Requirements to test:
- TEST-01: `rtx run false` returns exit code 1
- TEST-02: `rtx run sh -c 'exit 42'` returns exit code 42
- TEST-03: Process spawning does not leave zombie processes
- TEST-04: Signal forwarding delivers signal to child
- TEST-05: "Command not found" returns exit code 127
- TEST-06: Manual validation of `rtx run yes` real-time output

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 03-tests-and-validation*
*Context gathered: 2026-02-28*
