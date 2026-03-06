# Phase 10: CLI serve and Graceful Shutdown - Context

## Objective
Enable `rtx serve` to start the API and frontend, and ensure all processes are terminated gracefully on exit.

## Phase Constraints
- Must depend on Phase 9 being completed or having a stable API `Server` implementation.
- Must maintain `rtx run` backward compatibility.
- Use Go standard library only for CLI and signal handling.

## Known Dependencies
- `internal/api/server.go` (from Phase 9)
- `internal/scheduler/scheduler.go` (from Phases 5-7)

## Strategic Decisions
- **Subcommand Handling**: Use `flag.NewFlagSet` instead of a heavy library like Cobra. This fits the "stdlib only" and "minimal dependencies" constraints of the project.
- **Frontend Serving**: Use `http.FileServer` with `http.Dir("web/dist")` as recommended in `PROJECT.md`. This is simple and effective for v1.1.
- **Graceful Shutdown**: Prioritize stopping the API server first (to prevent new commands) then stopping managed processes. This avoids race conditions during shutdown.
- **StopAll() Parallelism**: Stop processes in parallel to minimize total shutdown time, especially if multiple processes have long `StopTimeout` periods.

## Success Criteria
1. `rtx serve` starts the API and responds to requests.
2. `rtx serve` serves the frontend at the root path.
3. `rtx run <cmd>` still works as before.
4. Sending `Ctrl+C` to `rtx serve` stops all managed processes before exiting.
5. No orphaned processes remain after CLI exit.
