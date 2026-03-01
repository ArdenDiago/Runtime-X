# Phase 4: Codebase Cleanup - Research

**Researched:** 2026-03-01
**Domain:** Go project restructuring — file/package deletion, go.mod hygiene, directory reorganization
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **Command structure:** Single `rtx` binary with subcommands: `rtx run` (v1.0 single-process runner), `rtx serve` (API server + frontend), and a combined start command that can launch components individually or together. One entry point at `cmd/rtx/main.go` — all subcommands route through it. Remove `cmd/api/`, `cmd/worker/`, `cmd/main.go` (legacy entry points).
- **Target directory layout:** Remove `cmd/docker_files/` entirely. Keep `cmd/rtx/` as the sole entry point.
- **Docker artifact handling:** Remove all Docker artifacts: `Dockerfile`, `docker-compose.yml`, `.dockerignore`, `.air.toml`, `docker_files/`, `frontend/Dockerfile`. Remove `run.sh` (legacy development script). Remove `RUNTIME_X_IMPLEMENTATION_GUIDE.md` (outdated). Clean slate — no Docker remnants.
- **Legacy frontend disposition:** Remove the current Go-based `frontend/` directory entirely (Go templates + Dockerfile). Phase 11 will create a fresh React app from scratch.
- **Shared code retention:** `internal/process/` (runner.go, runner_test.go) stays — v1.0 runner, must remain functional. Remove `internal/api/`, `internal/worker/`, `internal/docker/`, `internal/queue/`, `internal/core/`. Remove `internal/logging/` — evaluate during later phases.
- **Binary artifacts:** Remove tracked binaries (`./rtx`, `./bin/rtx`) from the repository. Update `.gitignore` to exclude compiled binaries.
- **README:** Minimal update — remove Docker orchestration references, keep it brief.

### Claude's Discretion

- Internal package structure for v1.1 (feature-based vs layered packages)
- Whether to create placeholder directories for future phases or only create them when needed
- `go.mod` module path — keep or update based on current state
- Exact `.gitignore` updates needed

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLN-01 | Legacy Docker orchestration code (`cmd/api/`, `internal/api/`, `internal/worker/`) is removed | File inventory below maps exact files to remove; git rm handles tracked files; untracked binary also needs removal |
| CLN-02 | Project directory structure is reorganized for scheduler + API + frontend architecture | Target layout section below defines the post-cleanup structure; `cmd/rtx/main.go` stays, placeholder dirs are Claude's discretion |
| CLN-03 | `go build ./...` succeeds with no build errors after cleanup | Build failure root cause identified (handlers.go references undefined `models` + `h.Scheduler`); removing the entire `internal/api/` package resolves it; uuid dependency must be kept or pruned correctly |
</phase_requirements>

---

## Summary

Phase 4 is a pure deletion and reorganization phase. No new Go code needs to be written — the v1.0 runner in `internal/process/` is complete and its tests pass. The entire effort is: remove the right files via `git rm`, remove untracked files via `rm`, prune any now-orphaned `go.mod` dependencies, update `.gitignore`, and verify that `go build ./...` and `go test ./...` pass after cleanup.

The current build is broken because `internal/api/handlers.go` references `models.TaskRunning` (an undefined symbol — the `models` package was deleted separately) and field names `h.Scheduler` / `h.Queue` that don't match the actual struct definition. This file is being deleted entirely, so the build failure is resolved by deletion — no patching required.

The `github.com/google/uuid` dependency in `go.mod` is used by `internal/api/handlers.go` (the file being deleted) and potentially by other legacy packages. After cleanup, run `go mod tidy` to drop any unused dependencies and keep `go.sum` accurate.

**Primary recommendation:** Use `git rm -r` for tracked legacy directories, `rm -rf` for untracked artifacts, then `go mod tidy` + `go build ./...` + `go test ./internal/process/...` to verify.

---

## Standard Stack

### Core (all stdlib — no new dependencies)

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| `git rm` | (git built-in) | Remove tracked files from index + working tree atomically | Single command that stages the deletion; avoids `rm` + `git add` two-step |
| `go mod tidy` | Go 1.25.7 | Remove unused module dependencies, update go.sum | Canonical Go tool for keeping go.mod minimal |
| `go build ./...` | Go 1.25.7 | Verify no compilation errors across all packages | The success criterion per CLN-03 |
| `go test ./...` | Go 1.25.7 | Verify existing process tests still pass | Non-negotiable backwards-compat check |

### No New Libraries Needed

This phase adds zero Go packages. If `github.com/google/uuid` is not referenced by any remaining package after deletion, `go mod tidy` will remove it from `go.mod` and `go.sum` automatically. The `cmd/rtx/main.go` currently imports only `runtimex/internal/process` and stdlib — no uuid dependency.

---

## Architecture Patterns

### Target Directory Layout After Phase 4

```
Runtime-X/
├── cmd/
│   └── rtx/
│       └── main.go          # sole entry point — rtx run subcommand (v1.0 kept)
├── internal/
│   └── process/
│       ├── runner.go        # v1.0 runner (kept)
│       └── runner_test.go   # v1.0 tests (kept)
├── go.mod                   # module: runtimex, go 1.25.5 (or tidy'd)
├── go.sum                   # updated after go mod tidy
├── .gitignore               # updated to exclude rtx, bin/rtx binaries
├── LICENSE                  # kept
├── README.md                # minimal update
└── .planning/               # kept (planning docs)
```

No placeholder directories for future phases (Phase 5–11 packages). Create them only when their phases start — avoids empty directories that Go tooling ignores anyway, and keeps the working tree clear of speculative structure.

### Pattern: Atomic Deletion via git rm

For tracked files/directories, use `git rm -r` instead of `rm + git add`:
```bash
# Removes files from both working tree and git index in one step
git rm -r cmd/api/ cmd/worker/ cmd/main.go cmd/docker_files/
git rm -r internal/api/ internal/worker/ internal/docker/ internal/queue/ internal/core/ internal/logging/
git rm -r frontend/
git rm Dockerfile docker-compose.yml .dockerignore .air.toml run.sh
git rm RUNTIME_X_IMPLEMENTATION_GUIDE.md
git rm bin/rtx
```

### Pattern: Untracked Artifacts Removed Separately

The `./rtx` binary in the repo root is untracked (appears in `git status` as `??`). Remove it with plain `rm`:
```bash
rm rtx
```

The `docker_files/` directory in the repo root is partially gitignored (only `go.dockerfile` was tracked; uuid-named files were not tracked per .gitignore). The `cmd/docker_files/` uuid-named `.dockerfile` files ARE tracked — `git rm -r` above handles those.

### Pattern: go mod tidy After Deletion

After removing all legacy Go source files:
```bash
go mod tidy
```

This will:
1. Scan remaining source files (`cmd/rtx/main.go`, `internal/process/*.go`)
2. Drop `require github.com/google/uuid v1.6.0` if nothing imports it
3. Update `go.sum` to remove the uuid checksum entries

Current `go.mod`:
```
module runtimex

go 1.25.5

require github.com/google/uuid v1.6.0
```

After cleanup with `go mod tidy`, expected result:
```
module runtimex

go 1.25.5
```
(uuid dependency dropped — no remaining source file imports it)

### Pattern: .gitignore Binary Exclusion

Current `.gitignore` does not exclude the `rtx` binary at the root or `bin/rtx`. Add entries after removal to prevent re-tracking:
```gitignore
# Compiled binaries
rtx
bin/
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Removing tracked files | Manual `rm` + `git add -A` | `git rm -r <path>` | Atomic: stages deletion in one step; `git add -A` can accidentally include unwanted files |
| Dependency cleanup | Manual `go.mod` editing | `go mod tidy` | go mod tidy handles transitive dependencies correctly; manual editing misses indirect entries in go.sum |
| Verifying build | Spot-checking individual packages | `go build ./...` | The `./...` wildcard catches all packages including nested ones; spot-checking misses broken sub-packages |

**Key insight:** This phase is a mechanical deletion task. The only judgment calls are (a) confirming the exact file inventory before running commands, and (b) verifying the `go.mod` module path is correct post-tidy.

---

## Common Pitfalls

### Pitfall 1: Deleting Files Without Checking git-tracked Status

**What goes wrong:** Running `rm -rf cmd/api/` removes the directory from disk but leaves it staged as a deletion with different semantics than `git rm`. Or the inverse — `git rm` on an untracked file fails.

**Why it happens:** Mix of tracked and untracked files in the same area.

**How to avoid:** Before deleting, confirm with `git ls-files <path>` whether the path is tracked. Use `git rm -r` for tracked paths and `rm` for untracked ones. The `./rtx` binary at root is untracked — `git rm rtx` would error.

**Warning signs:** `git status` shows `D` (deleted from working tree but not staged) instead of `D ` (staged deletion).

### Pitfall 2: go.sum Stale After Dependency Removal

**What goes wrong:** `go build ./...` passes locally but `go.sum` still contains uuid hash entries. CI or other tools may flag the stale `go.sum`.

**Why it happens:** Forgetting to run `go mod tidy` after source deletion.

**How to avoid:** Always run `go mod tidy` immediately after all source deletions, before the final `go build ./...` verification.

**Warning signs:** `go.sum` still contains `github.com/google/uuid` lines after cleanup.

### Pitfall 3: internal/api/tasks_test.go Causes Test Failure

**What goes wrong:** `go test ./...` fails because `internal/api/tasks_test.go` has broken imports (`runtimex/api-service/internal/models` — a non-existent module path).

**Why it happens:** The whole `internal/api/` package is being deleted — if only `handlers.go` is removed but `tasks_test.go` is left, compilation fails.

**How to avoid:** Delete the entire `internal/api/` directory, not individual files. `git rm -r internal/api/` removes both `handlers.go` and `tasks_test.go`.

**Warning signs:** Build error referencing `runtimex/api-service/internal/models` — this import path doesn't exist in the current module.

### Pitfall 4: cmd/docker_files/ Tracked .dockerfile Files Left Behind

**What goes wrong:** The three uuid-named `.dockerfile` files under `cmd/docker_files/` are tracked by git (confirmed via `git ls-files`). They are NOT covered by the `.gitignore` pattern `docker_files/` because the gitignore targets the root-level `docker_files/`, not `cmd/docker_files/`.

**Why it happens:** The gitignore rule is path-specific (`docker_files/` at root), but the tracked files are in `cmd/docker_files/` — a different path.

**How to avoid:** Use `git rm -r cmd/docker_files/` explicitly. After removal, the `cmd/docker_files/` directory no longer exists, so no gitignore update needed for that specific path.

**Warning signs:** After cleanup, `git ls-files cmd/docker_files/` returns any output.

### Pitfall 5: Binary `bin/rtx` is Tracked

**What goes wrong:** `bin/rtx` is tracked by git (confirmed in `git ls-files`). Simply deleting it with `rm` leaves a staged-but-not-committed deletion.

**Why it happens:** Compiled binaries were checked in at some point; current `.gitignore` has no rule for `bin/`.

**How to avoid:** Use `git rm bin/rtx` to stage the deletion, then update `.gitignore` to add `bin/` so future builds don't accidentally re-track.

**Warning signs:** After cleanup, `git ls-files bin/` returns any output.

### Pitfall 6: README Still References Docker After Cleanup

**What goes wrong:** README.md still contains Docker Compose instructions, Dockerfile references, or architecture diagrams showing the Docker setup. Phase's success criteria includes the directory structure reflecting v1.1 layout.

**Why it happens:** README update is easy to overlook — it's not a Go file so build tools don't catch it.

**How to avoid:** Include a README update task. The locked decision says "minimal update: remove Docker orchestration references, keep it brief until later phases flesh out v1.1 docs."

---

## Code Examples

### Current Build Failure (root cause confirmed)

```
# runtimex/internal/api
internal/api/handlers.go:63:20: h.Scheduler undefined (type *TaskHandler has no field or method Scheduler)
internal/api/handlers.go:69:16: undefined: models
internal/api/handlers.go:70:4: h.Scheduler undefined (type *TaskHandler has no field or method Scheduler)
internal/api/handlers.go:79:14: h.Queue undefined (type *TaskHandler has no field or method Queue, but does have field queue)
```

Resolution: `git rm -r internal/api/` — no patching needed.

### Verified: Process Tests Pass Now

```bash
$ go test ./internal/process/...
ok  runtimex/internal/process  0.211s
```

These tests must still pass after cleanup. They have zero dependencies on the legacy Docker packages — only stdlib imports.

### Complete File Inventory: Files to Remove

**Tracked by git** (use `git rm`):

```
# cmd/ legacy entry points and docker routes
cmd/api/docker/dockerfile/create.go
cmd/api/docker/image/images.go
cmd/api/docker/router.go
cmd/api/docker/tags/tags.go
cmd/api/docker/utility/dockerhub.go
cmd/api/docker/utility/dockerhub_tags.go
cmd/main.go
cmd/worker/main.go
cmd/docker_files/58c10e87-1d25-4f2a-8415-0e9239d5838e.dockerfile
cmd/docker_files/eec854bf-db5c-4034-ae06-1a038b0aea9f.dockerfile
cmd/docker_files/fabc09cf-4723-4ae0-a7ec-9ee3ee8b7d52.dockerfile

# internal/ legacy packages
internal/api/handlers.go
internal/api/tasks_test.go
internal/core/dockerfile_config.go
internal/core/errors.go
internal/core/job.go
internal/docker/validator.go
internal/docker/validator_test.go
internal/logging/logger.go
internal/queue/docker_queue.go
internal/queue/queue.go
internal/worker/docker_runner.go
internal/worker/docker_scheduler.go
internal/worker/pool.go
internal/worker/runner.go
internal/worker/scheduler.go

# frontend/ legacy Go template frontend
frontend/cmd/main.go
frontend/Dockerfile
frontend/templates/index.html

# Docker and dev artifacts
Dockerfile
docker-compose.yml
.dockerignore
.air.toml
run.sh

# Tracked binary
bin/rtx

# Outdated docs
RUNTIME_X_IMPLEMENTATION_GUIDE.md
```

**Untracked** (use `rm`):
```
rtx           # compiled binary at repo root (git status shows ?? rtx)
```

**Keep (not deleted):**
```
cmd/rtx/main.go                  # sole entry point
internal/process/runner.go       # v1.0 runner
internal/process/runner_test.go  # v1.0 tests
go.mod                           # updated by go mod tidy
go.sum                           # updated by go mod tidy
.gitignore                       # updated to add bin/ and rtx
LICENSE
README.md                        # minimal update
.planning/                       # all planning docs
```

### Updated .gitignore

After cleanup, add to `.gitignore`:
```gitignore
# Compiled binaries
rtx
bin/
```

Remove or neutralize the now-irrelevant comment `# Generated Dockerfiles (keep sample)` and the `docker_files/` rule since the directory no longer exists (optional cleanup — harmless to leave).

### Execution Sequence

```bash
# Step 1: Remove tracked legacy directories/files
git rm -r cmd/api/ cmd/worker/ cmd/docker_files/
git rm cmd/main.go
git rm -r internal/api/ internal/core/ internal/docker/ internal/logging/ internal/queue/ internal/worker/
git rm -r frontend/
git rm Dockerfile docker-compose.yml .dockerignore .air.toml run.sh
git rm RUNTIME_X_IMPLEMENTATION_GUIDE.md
git rm bin/rtx

# Step 2: Remove untracked binary
rm rtx

# Step 3: Tidy dependencies
go mod tidy

# Step 4: Verify build
go build ./...

# Step 5: Verify tests
go test ./...

# Step 6: Update .gitignore (add bin/ and rtx rules)
# Step 7: Update README.md (remove Docker references)
# Step 8: Commit
```

---

## State of the Art

| Old Approach | Current Approach | Notes |
|--------------|-----------------|-------|
| `rm -rf path && git add -A` | `git rm -r path` | Atomic staged deletion |
| Manual `go.mod` editing | `go mod tidy` | Canonical dependency cleanup |
| Multiple entry points (`cmd/main.go`, `cmd/worker/main.go`) | Single entry point `cmd/rtx/main.go` with subcommands | Go convention for CLI tools |

---

## Open Questions

1. **go.mod module path: keep `runtimex` or rename?**
   - What we know: Current module is `module runtimex` (single-word, lowercase). All internal imports use `runtimex/internal/...`. This is a valid Go module name.
   - What's unclear: Whether the user wants to rename it to something like `github.com/user/runtime-x` for consistency with Go module conventions.
   - Recommendation: Keep `runtimex` for now — Claude's discretion says "keep or update based on current state." Renaming requires updating all import paths in `cmd/rtx/main.go` and `internal/process/runner.go`, adding friction. Leave for a future decision if the project is ever published publicly.

2. **Placeholder directories for v1.1?**
   - What we know: Claude's discretion. Future phases (5–11) will create `internal/scheduler/`, `internal/api/`, `web/` etc.
   - What's unclear: Whether empty placeholder dirs add value.
   - Recommendation: Do NOT create placeholders. Go tooling ignores empty directories, and empty dirs in git require a `.gitkeep` file — unnecessary ceremony. Phases 5+ create their own structure.

---

## Sources

### Primary (HIGH confidence)

- Direct codebase inspection via Read/Bash tools — file inventory, build errors, test results are ground truth
- `go build ./...` run live: confirmed exact error lines in `internal/api/handlers.go`
- `go test ./internal/process/...` run live: confirmed `ok runtimex/internal/process 0.211s`
- `git ls-files` run live: confirmed exact set of tracked files including `bin/rtx`, `cmd/docker_files/*.dockerfile`
- `go version` run live: confirmed Go 1.25.7

### Secondary (MEDIUM confidence)

- `go mod tidy` behavior for unused dependencies: well-established Go toolchain behavior, HIGH confidence based on training data cross-referenced with known stable Go toolchain semantics

---

## Metadata

**Confidence breakdown:**
- File inventory: HIGH — obtained from live `git ls-files` + `find` commands
- Build failure root cause: HIGH — from live `go build ./...` output
- Test baseline: HIGH — from live `go test ./internal/process/...` output
- go mod tidy behavior: HIGH — stable Go toolchain feature, verified behavior
- Target directory layout: HIGH — derived directly from locked CONTEXT.md decisions

**Research date:** 2026-03-01
**Valid until:** Not time-sensitive — this is a pure filesystem/git operation with no external API dependencies
