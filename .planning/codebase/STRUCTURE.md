# Codebase Structure

**Analysis Date:** 2026-02-27

## Directory Layout

```
Runtime-X/
├── cmd/                           # Executable entrypoints and API route handlers
│   ├── main.go                    # API server entrypoint
│   ├── worker/                    # Standalone worker process
│   │   └── main.go                # Worker process entrypoint
│   └── api/                       # API route handlers
│       └── docker/                # Docker-related API routes
│           ├── router.go          # Route registration
│           ├── dockerfile/        # Dockerfile creation endpoint
│           │   └── create.go      # POST/GET dockerfile handlers
│           ├── image/             # Docker image discovery
│           │   └── images.go      # GET official images handler
│           ├── tags/              # Docker image tags discovery
│           │   └── tags.go        # GET image tags handler
│           └── utility/           # Docker Hub API utilities
│               ├── dockerhub.go   # Official image list API
│               └── dockerhub_tags.go # Image tags API
├── internal/                      # Private packages not importable from outside
│   ├── core/                      # Domain models and constants
│   │   ├── job.go                 # Job struct and status constants
│   │   ├── dockerfile_config.go   # DockerfileConfig struct
│   │   └── errors.go              # Error definitions
│   ├── api/                       # API utilities and handlers
│   │   ├── handlers.go            # Generic HTTP handler utilities
│   │   └── tasks_test.go          # Handler tests
│   ├── queue/                     # Job queue implementation
│   │   ├── queue.go               # Generic JobQueue
│   │   └── docker_queue.go        # Docker-specific DockerJobQueue
│   ├── worker/                    # Worker and scheduler logic
│   │   ├── pool.go                # WorkerPool semaphore
│   │   ├── scheduler.go           # Generic job scheduler
│   │   ├── docker_scheduler.go    # Docker job scheduler
│   │   ├── runner.go              # Generic job runner
│   │   └── docker_runner.go       # Docker container executor
│   ├── docker/                    # Docker validation
│   │   ├── validator.go           # Configuration and image validation
│   │   └── validator_test.go      # Validator tests
│   └── logging/                   # Centralized logging
│       └── logger.go              # Job event logger
├── frontend/                      # Frontend web server
│   ├── cmd/
│   │   └── main.go                # Frontend server entrypoint
│   ├── Dockerfile                 # Frontend container image
│   └── templates/
│       └── index.html             # HTML template
├── docker_files/                  # Generated Dockerfiles (git ignored)
├── go.mod                         # Go module definition
├── go.sum                         # Go module checksums
├── Dockerfile                     # Backend container image
├── .air.toml                      # Live reload configuration
├── .dockerignore                  # Files to exclude from Docker builds
├── .gitignore                     # Git ignore patterns
├── docker-compose.yml             # Multi-container orchestration
└── .planning/                     # GSD planning documents
    └── codebase/
        ├── ARCHITECTURE.md        # Architecture patterns and layers
        ├── STRUCTURE.md           # This file
        ├── CONVENTIONS.md         # Code style and patterns (if exists)
        ├── TESTING.md             # Testing strategy (if exists)
        └── CONCERNS.md            # Known issues and tech debt (if exists)
```

## Directory Purposes

**`cmd/`:**
- Purpose: Executable programs and route handler definitions
- Contains: `main.go` entry points, HTTP route handlers for API endpoints
- Key files: `cmd/main.go` (API server), `cmd/worker/main.go` (worker), `cmd/api/docker/router.go` (routes)

**`internal/core/`:**
- Purpose: Domain models and business logic constants
- Contains: Job, DockerfileConfig structs, status enums, error definitions
- Key files: `job.go`, `dockerfile_config.go`, `errors.go`

**`internal/queue/`:**
- Purpose: Thread-safe job queue implementations
- Contains: Generic and Docker-specific job queues with blocking semantics
- Key files: `queue.go` (base), `docker_queue.go` (Docker-specific)

**`internal/worker/`:**
- Purpose: Job execution infrastructure (pool, scheduler, runner)
- Contains: Semaphore-based worker pool, scheduler event loops, job/Docker runners
- Key files: `pool.go`, `scheduler.go`, `docker_runner.go`, `docker_scheduler.go`

**`internal/docker/`:**
- Purpose: Docker-specific validation and verification
- Contains: Field validation, Docker Hub API checks, security constraint enforcement
- Key files: `validator.go`

**`internal/logging/`:**
- Purpose: Centralized job logging
- Contains: Job event logging functions
- Key files: `logger.go`

**`internal/api/`:**
- Purpose: API-specific utilities and handlers
- Contains: Generic HTTP handler code and API tests
- Key files: `handlers.go`, `tasks_test.go`

**`frontend/`:**
- Purpose: Web UI server and templates
- Contains: HTML templates, frontend entrypoint
- Key files: `cmd/main.go`, `templates/index.html`

**`docker_files/`:**
- Purpose: Generated Dockerfile storage
- Contains: Runtime-generated Dockerfile artifacts
- Generated: Yes
- Committed: No (git ignored)

## Key File Locations

**Entry Points:**
- `cmd/main.go`: API server listening on `:8080` with Docker routes and scheduler
- `cmd/worker/main.go`: Standalone worker process consuming generic job queue
- `frontend/cmd/main.go`: Frontend server listening on `:3000`

**Configuration:**
- `go.mod`: Go module definition with dependencies (google/uuid)
- `docker-compose.yml`: Multi-container orchestration (backend, frontend)
- `.air.toml`: Live reload settings for development

**Core Logic:**
- `internal/core/job.go`: Job domain model with status lifecycle
- `internal/core/dockerfile_config.go`: DockerfileConfig with all Docker build instructions
- `internal/queue/docker_queue.go`: Docker job queue with blocking dequeue
- `internal/worker/docker_runner.go`: Docker build/run/cleanup orchestration
- `internal/worker/docker_scheduler.go`: Scheduler for Docker job processing
- `internal/docker/validator.go`: Field validation and Docker Hub verification

**Testing:**
- `internal/api/tasks_test.go`: API handler tests
- `internal/docker/validator_test.go`: Validator unit tests

## Naming Conventions

**Files:**
- PascalCase for exported domain models: `JobQueue.go`, `DockerRunner.go`
- snake_case for internal/implementation: `docker_runner.go`, `dockerfile_config.go`
- Paired with `_test.go` suffix: `validator_test.go`, `tasks_test.go`
- Package-scoped implementations in same directory: validators in `internal/docker/`

**Directories:**
- lowercase for package directories: `queue`, `worker`, `core`, `docker`
- Functional grouping: `api/`, `docker/`, `logging/`
- Hierarchical: `cmd/api/docker/dockerfile/` mirrors logical organization

**Functions:**
- Exported: PascalCase: `NewDockerJobQueue()`, `Enqueue()`, `RunDockerJob()`
- Unexported: camelCase: `buildImage()`, `runContainer()`, `cleanup()`

**Types:**
- Structs: PascalCase: `DockerRunner`, `WorkerPool`, `ValidationResult`
- Interfaces: PascalCase with -er/-able suffix if applicable
- Constants: ALL_CAPS: `StatusRunning`, `ConfigBuilding`

**Variables:**
- Package-level unexported: camelCase: `dockerQueue`, `workerPool`
- Receiver: Short, single letter if simple: `(dr *DockerRunner)`, `(q *DockerJobQueue)`
- Loop/temp: Single letters or clear abbreviations: `i`, `err`, `config`

## Where to Add New Code

**New Feature (e.g., new Docker operation):**
- Primary code: New handler in `cmd/api/docker/{feature}/`
- Core logic: New runner/executor in `internal/worker/` if execution needed
- Validation: Add validators to `internal/docker/validator.go`
- Tests: Create `*_test.go` alongside implementation

**New Docker API Endpoint:**
- Route definition: Register in `cmd/api/docker/router.go` RouterWithQueue
- Handler implementation: Create new package under `cmd/api/docker/{resource}/`
- Example: GET endpoint → `image/images.go`, POST endpoint → `dockerfile/create.go`

**New Job Type (parallel to Docker jobs):**
- Queue: Create `internal/queue/{type}_queue.go` similar to `docker_queue.go`
- Runner: Create `internal/worker/{type}_runner.go` similar to `docker_runner.go`
- Scheduler: Create `internal/worker/{type}_scheduler.go`
- Domain model: Add to `internal/core/` alongside existing models
- Entry point: Add new `cmd/{type}/main.go` for standalone processor

**Shared Utilities:**
- Core domain models: `internal/core/`
- Validation logic: `internal/docker/validator.go` or new file in `internal/docker/`
- Job execution: `internal/worker/` for pool/scheduler/runner logic
- Logging: `internal/logging/logger.go`

**Tests:**
- Unit tests: `{file}_test.go` in same directory as implementation
- API tests: `internal/api/tasks_test.go`
- Location: Always in `internal/` for code under `internal/`, never put tests in `cmd/`

## Special Directories

**`.git/`:**
- Purpose: Version control metadata
- Generated: Yes
- Committed: No (never committed)

**`docker_files/`:**
- Purpose: Generated Dockerfile artifacts from API requests
- Generated: Yes (at runtime)
- Committed: No (git ignored)

**`.planning/`:**
- Purpose: GSD planning and analysis documents
- Generated: Yes (by GSD commands)
- Committed: Yes (part of repository planning)

**`vendor/`:**
- Purpose: Not present; using Go modules
- Generated: N/A
- Committed: No

---

*Structure analysis: 2026-02-27*
