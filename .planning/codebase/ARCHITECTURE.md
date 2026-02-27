# Architecture

**Analysis Date:** 2026-02-27

## Pattern Overview

**Overall:** Layered architecture with job queue processing and worker pool pattern

**Key Characteristics:**
- HTTP API server accepts Dockerfile configuration requests
- Job queue decouples request acceptance from execution
- Worker pool manages concurrent job execution with bounded parallelism
- Docker-focused: generates, builds, and runs Docker containers
- Dual entry points: API server and standalone worker process

## Layers

**API Layer:**
- Purpose: Handles HTTP requests, validates input, responds to clients
- Location: `cmd/main.go`, `cmd/api/docker/*`
- Contains: HTTP handlers, route definitions, request/response serialization
- Depends on: Core types, Queue, Docker validator
- Used by: HTTP clients, Docker API consumers

**Handler/Controller Layer:**
- Purpose: Process API requests, orchestrate business logic, queue jobs
- Location: `cmd/api/docker/dockerfile/create.go`, `cmd/api/docker/image/images.go`, `cmd/api/docker/tags/tags.go`
- Contains: HTTP handler functions, request validation, Dockerfile generation
- Depends on: Core types, Validator, Queue, UUID generation
- Used by: Router/Mux

**Core Domain Layer:**
- Purpose: Define domain models and business constants
- Location: `internal/core/`
- Contains: `Job`, `DockerfileConfig`, status constants, error definitions
- Depends on: Go stdlib only
- Used by: All other layers

**Queue Layer:**
- Purpose: Thread-safe job queue with blocking semantics
- Location: `internal/queue/`
- Contains: `JobQueue` (generic), `DockerJobQueue` (Docker-specific)
- Depends on: Core types, sync primitives
- Used by: API handlers, worker scheduler

**Worker Pool Layer:**
- Purpose: Manage bounded concurrent execution
- Location: `internal/worker/pool.go`
- Contains: `WorkerPool` with semaphore-based concurrency control
- Depends on: None (uses channels)
- Used by: Scheduler, Docker scheduler

**Scheduler Layer:**
- Purpose: Dequeue jobs and assign to available workers
- Location: `internal/worker/scheduler.go`, `internal/worker/docker_scheduler.go`
- Contains: Generic scheduler and Docker-specific scheduler
- Depends on: Queue, Worker pool, Job execution logic
- Used by: Main processes

**Execution/Runner Layer:**
- Purpose: Execute jobs, manage Docker container lifecycle
- Location: `internal/worker/runner.go`, `internal/worker/docker_runner.go`
- Contains: Shell command execution, Docker build/run/cleanup operations
- Depends on: Core types, Logging, OS/exec
- Used by: Scheduler

**Validation Layer:**
- Purpose: Validate Docker configuration and verify images exist
- Location: `internal/docker/validator.go`
- Contains: Field validation, Docker Hub image/tag verification, security checks
- Depends on: Core types, HTTP for Docker Hub API
- Used by: Dockerfile handler

**Logging Layer:**
- Purpose: Centralized job logging
- Location: `internal/logging/logger.go`
- Contains: Job event logging
- Depends on: Core types
- Used by: Workers, runners

## Data Flow

**Dockerfile Creation and Execution Flow:**

1. Client sends POST to `/docker/dockerfile` with `DockerfileConfig` JSON
2. Handler validates request JSON into `CreateDockerfileRequest`
3. Validator checks field constraints (image name, tag format, copy paths, etc.)
4. Validator makes HTTP calls to Docker Hub API (unless `skipHubValidation=true`)
5. Handler generates Dockerfile content using `GenerateDockerfile()`
6. Handler writes Dockerfile to `docker_files/{id}.dockerfile`
7. If `execute=true`, handler enqueues `DockerfileConfig` into `DockerJobQueue`
8. Handler responds with `CreateDockerfileResponse` including config and any warnings
9. Scheduler continuously dequeues jobs from `DockerJobQueue`
10. Scheduler acquires worker from pool (blocks if none available)
11. Docker runner builds image: `docker build -t runtimex-job-{id} -f {dockerfile} .`
12. Docker runner runs container in detached mode: `docker run -d --rm=false {image}`
13. Docker runner waits for container with `docker wait {containerID}`
14. Docker runner captures logs: `docker logs {containerID}`
15. Runner updates `DockerfileConfig.Status` and `DockerfileConfig.Error`
16. Runner logs job completion/failure via `logging.LogJob()`
17. Runner cleans up: `docker rm -f {containerID}` and `docker rmi -f {image}`
18. Worker released back to pool

**Image/Tag Discovery Flow:**

1. Client calls GET `/docker/images` → returns official Docker images
2. Client calls GET `/docker/images/{image}/tags` → returns tags for image from Docker Hub

**State Management:**
- Configuration persisted to filesystem: `docker_files/{id}.dockerfile`
- Job status tracked in memory within `DockerfileConfig` object
- Job lifecycle events logged via stdlib logging
- No persistent database; state lost on process restart

## Key Abstractions

**Job and DockerfileConfig:**
- Purpose: Represent executable work units with metadata and status
- Examples: `internal/core/job.go`, `internal/core/dockerfile_config.go`
- Pattern: Data structures with string-typed status constants

**JobQueue and DockerJobQueue:**
- Purpose: Thread-safe FIFO queue with blocking dequeue
- Examples: `internal/queue/queue.go`, `internal/queue/docker_queue.go`
- Pattern: Mutex + condition variable for synchronization

**WorkerPool:**
- Purpose: Limit concurrent execution to N workers
- Examples: `internal/worker/pool.go`
- Pattern: Semaphore using buffered channel of size N

**Scheduler:**
- Purpose: Continuously dequeue and execute with worker pool
- Examples: `internal/worker/scheduler.go`, `internal/worker/docker_scheduler.go`
- Pattern: Goroutine-based event loop with goroutine spawning per job

**DockerRunner:**
- Purpose: Encapsulate Docker container build/run/cleanup lifecycle
- Examples: `internal/worker/docker_runner.go`
- Pattern: Struct with methods for each lifecycle phase

**ValidationResult:**
- Purpose: Collect validation errors with field context
- Examples: `internal/docker/validator.go`
- Pattern: Struct containing `Valid` boolean and `[]ValidationError`

## Entry Points

**API Server:**
- Location: `cmd/main.go`
- Triggers: Process startup
- Responsibilities:
  - Initialize `DockerJobQueue(capacity=10)` and `WorkerPool(size=3)`
  - Start Docker scheduler with 10-minute timeout per job
  - Register HTTP routes for health check, home, and Docker APIs
  - Listen on `:8080`
  - Block until HTTP server error

**Worker Process:**
- Location: `cmd/worker/main.go`
- Triggers: Standalone process startup
- Responsibilities:
  - Initialize `JobQueue(capacity=10)` and `WorkerPool(size=3)`
  - Start generic job scheduler
  - Block forever to keep scheduler running

**Frontend Server:**
- Location: `frontend/cmd/main.go`
- Triggers: Standalone process startup
- Responsibilities:
  - Load HTML template from `frontend/templates/index.html`
  - Serve template on GET `/`
  - Listen on `:3000`

## Error Handling

**Strategy:** Return validation errors or system errors in HTTP responses; log job failures; continue processing

**Patterns:**

**Validation Errors:**
- Return `400 Bad Request` with `{"error": "validation failed", "errors": [...]}`
- Errors include field name and message
- Example: `"field": "image", "message": "image name is required"`

**System Errors:**
- Return `500 Internal Server Error` with plain text error message
- Example: "failed to create docker_files directory: permission denied"
- Logged to stdout/stderr via stdlib log

**Job Execution Errors:**
- Caught and stored in `DockerfileConfig.Error` and `Job.Error`
- Job status set to `StatusFailed` or `ConfigFailed`
- Logged via `logging.LogJob()`
- Job cleanup still occurs (Docker container/image removal)

**Queue Full Error:**
- `Enqueue()` returns `core.ErrQueueFull` if capacity reached
- Handler catches and includes in response warnings
- Client can retry

## Cross-Cutting Concerns

**Logging:** Stdlib `log` package with printf-style messages to stdout. Job lifecycle events logged at each status change.

**Validation:** Multi-level validation at handler layer before queuing:
- Request body JSON parsing
- Field constraints (image name format, tag length, path safety)
- Docker Hub API checks for image/tag existence
- Dangerous command patterns (fork bomb, disk wipe, etc.)

**Authentication:** Not implemented. API is open to all HTTP clients.

**Concurrency:** Goroutine-based with mutex-protected queues. Worker pool enforces max concurrent docker operations.

---

*Architecture analysis: 2026-02-27*
