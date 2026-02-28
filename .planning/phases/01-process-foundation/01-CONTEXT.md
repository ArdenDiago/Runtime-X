# Phase 1: Process Foundation - Context

**Gathered:** 2026-02-28
**Status:** Ready for planning

<domain>
## Phase Boundary

CLI that spawns a child process via `rtx run <command> [args...]`, streams stdout/stderr in real time with direct fd assignment, captures the child's exact exit code and propagates it to the parent shell, and prevents zombie processes by always calling `cmd.Wait()`. No signal handling in this phase — that's Phase 2.

</domain>

<decisions>
## Implementation Decisions

### CLI invocation design
- Subcommand pattern: `rtx run <command> [args...]`
- Everything after `run` is the child command and its arguments
- rtx-level flags go before `run`: `rtx -v run sleep 10`
- Support short and long flag forms: `-v` / `--verbose`
- `rtx run` with no command is an error

### Claude's Discretion
- No-args behavior (usage help vs error message) — pick the standard CLI approach
- `--version` and `--help` flags — include if standard, skip if not worth v0 effort
- Log format and prefix style (`[rtx]` prefix, formatting details)
- Error message wording for "command not found" and other failures
- Process group isolation decision (`Setpgid: true` vs default) — research recommends Setpgid: true

</decisions>

<specifics>
## Specific Ideas

- User wants it simple — minimal flags, clean invocation
- Follows the `tini` analog: do one thing correctly, nothing more
- The `run` subcommand leaves room for future subcommands without breaking changes

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-process-foundation*
*Context gathered: 2026-02-28*
