# Phase 4: Codebase Cleanup - Context

**Gathered:** 2026-03-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Remove all legacy Docker orchestration code (cmd/api/, internal/api/, internal/worker/, internal/docker/, internal/queue/, internal/core/) and reorganize the project for the v1.1 architecture: scheduler + API + frontend. The v1.0 CLI runner (`rtx run`) must remain fully functional with passing tests.

</domain>

<decisions>
## Implementation Decisions

### Command structure
- Single `rtx` binary with subcommands: `rtx run` (v1.0 single-process runner), `rtx serve` (API server + frontend), and a combined start command that can launch components individually or together
- One entry point at cmd/rtx/main.go — all subcommands route through it
- Remove cmd/api/, cmd/worker/, cmd/main.go (legacy entry points)

### Target directory layout
- Remove cmd/docker_files/ entirely
- Keep cmd/rtx/ as the sole entry point

### Docker artifact handling
- Remove all Docker artifacts: Dockerfile, docker-compose.yml, .dockerignore, .air.toml, docker_files/, frontend/Dockerfile
- Remove run.sh (legacy development script)
- Remove RUNTIME_X_IMPLEMENTATION_GUIDE.md (outdated, v1.1 planning docs supersede it)
- Clean slate — no Docker remnants

### Legacy frontend disposition
- Remove the current Go-based frontend/ directory entirely (Go templates + Dockerfile)
- Phase 11 will create a fresh React app from scratch
- Frontend directory naming and embedding strategy (go:embed vs static) deferred to Phase 11's planning

### Shared code retention
- internal/process/ (runner.go, runner_test.go) stays — this is the v1.0 runner, must remain functional
- Remove internal/api/, internal/worker/, internal/docker/, internal/queue/, internal/core/ — all Docker-specific
- Remove internal/logging/ — evaluate during later phases whether to use slog or custom logging

### Binary artifacts
- Remove tracked binaries (./rtx, ./bin/rtx) from the repository
- Update .gitignore to exclude compiled binaries

### README
- Minimal update: remove Docker orchestration references, keep it brief until later phases flesh out v1.1 docs

### Claude's Discretion
- Internal package structure for v1.1 (feature-based vs layered packages)
- Whether to create placeholder directories for future phases or only create them when needed
- go.mod module path — keep or update based on current state
- Exact .gitignore updates needed

</decisions>

<specifics>
## Specific Ideas

- User wants `rtx` to have a command that can start server and frontend individually, plus a command to run them together
- v1.0 `rtx run` backwards compatibility is non-negotiable — tests must pass after cleanup

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-codebase-cleanup*
*Context gathered: 2026-03-01*
