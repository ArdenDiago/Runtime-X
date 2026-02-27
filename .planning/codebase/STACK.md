# Technology Stack

**Analysis Date:** 2026-02-27

## Languages

**Primary:**
- Go 1.25.5 - Backend API server, Docker job orchestration, and frontend UI server

**Secondary:**
- HTML/CSS - Frontend templates

## Runtime

**Environment:**
- Go 1.25 (tip-trixie variant in Docker)
- Docker (for containerization and job execution)

**Package Manager:**
- Go Modules (go.mod/go.sum)
- Lockfile: Present

## Frameworks

**Core:**
- `net/http` (Go stdlib) - HTTP server and routing
- `html/template` (Go stdlib) - Frontend HTML templating

**Job Orchestration:**
- `os/exec` (Go stdlib) - Docker command execution
- `context` (Go stdlib) - Timeout management for job execution

**Testing:**
- Go built-in testing (no external test framework detected in current phase)

**Build/Dev:**
- Air (github.com/air-verse/air) - Hot reload development server
- Docker Compose - Local orchestration

## Key Dependencies

**Critical:**
- `github.com/google/uuid v1.6.0` - Job and Dockerfile ID generation (`runtimex/internal/core` and `runtimex/cmd/api/docker/dockerfile`)

**Infrastructure:**
- Docker - Job execution engine (invoked via `os/exec`)

## Configuration

**Environment:**
- Uses standard Go http.ListenAndServe on hardcoded ports:
  - Backend API: `:8080`
  - Frontend: `:3000`
- No environment variable configuration detected
- Docker-specific configuration passed via request payloads (GitRepo, EnvVars, RunCommands, etc.)

**Build:**
- `.air.toml` - Air development server configuration
- `go.mod` - Module definition
- Multiple Dockerfiles:
  - `Dockerfile` - Base Go container (commented-out build)
  - `docker_files/go.dockerfile` - Development backend with Air
  - `frontend/Dockerfile` - Multi-stage Go build for frontend
- `docker-compose.yml` - Local service orchestration

## Platform Requirements

**Development:**
- Go 1.25+
- Docker with CLI installed (for job execution)
- Docker Compose 3.8+
- Shell (bash) - run.sh script uses bash syntax

**Production:**
- Deployment target: Docker containers
- Dockerfile expects Docker daemon access (jobs executed via docker CLI)
- Go binary runs HTTP server bound to ports 8080 (backend) and 3000 (frontend)

## Build Artifacts

**Entry Points:**
- Backend: `go run cmd/main.go` or compiled binary from `go build -o ./tmp/main ./cmd/main.go`
- Frontend: `go run frontend/cmd/main.go` or compiled binary from `go build -o frontend ./cmd/main.go`

**Docker Build Context:**
- Backend image built from `/home/ardendiago/Coding/go_tutorials/Runtime-X` with `docker_files/go.dockerfile`
- Frontend image built from `frontend/` directory with `frontend/Dockerfile`

---

*Stack analysis: 2026-02-27*
