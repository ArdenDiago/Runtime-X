# External Integrations

**Analysis Date:** 2026-02-27

## APIs & External Services

**Docker Hub API:**
- Service: Docker Hub Registry API
- What it's used for: Fetching available Docker images, validating image names, and checking tag availability
  - SDK/Client: `net/http` with custom HTTP clients
  - Auth: No authentication required (querying public library repository)
  - Endpoints used:
    - `https://hub.docker.com/v2/repositories/library/` - List official Docker images (paginated)
    - `https://hub.docker.com/v2/repositories/library/{image}/` - Validate image existence
    - `https://hub.docker.com/v2/repositories/library/{image}/tags/{tag}/` - Validate tag existence

**Files:**
- `cmd/api/docker/utility/dockerhub.go` - GetOfficialDockerImages() fetches paginated list
- `cmd/api/docker/utility/dockerhub_tags.go` - Fetches available tags for an image
- `internal/docker/validator.go` - ValidateImageExists() and ValidateTagExists() check registry

## Data Storage

**Databases:**
- None - In-memory queue only (no persistent database)

**File Storage:**
- Local filesystem only
- Dockerfile artifacts saved to `docker_files/` directory
- No cloud storage integration
- Files written via `os.WriteFile()` in `cmd/api/docker/dockerfile/create.go`

**Caching:**
- None detected

## Authentication & Identity

**Auth Provider:**
- Custom - No external auth provider
- Implementation: No authentication layer present
  - HTTP endpoints in `cmd/main.go` accept all requests
  - Docker Hub API access is anonymous (library repository is public)

## Monitoring & Observability

**Error Tracking:**
- None - No external error tracking service

**Logs:**
- Approach: Go stdlib `log` package only
- Logging file: `internal/logging/logger.go` with LogJob() function
- Output: Standard output (console logs)
- Log levels: Not implemented (info level only)

## CI/CD & Deployment

**Hosting:**
- Docker Compose for local development
- Dockerfiles available for containerized deployment
- No detected cloud platform integration

**CI Pipeline:**
- None - No GitHub Actions or CI service configured (.github directory not present)

## Environment Configuration

**Required env vars:**
- None detected - System uses hardcoded configuration

**Secrets location:**
- Not applicable - No secrets management

## Webhooks & Callbacks

**Incoming:**
- None detected

**Outgoing:**
- None detected

## Docker Integration

**Docker Execution:**
- Purpose: Job execution engine - each Dockerfile configuration is built and run as a Docker container
- Implementation: `os/exec` package in `internal/worker/docker_runner.go`
- Commands used:
  - `docker build` - Builds image from generated Dockerfile
  - `docker run` - Runs container in detached mode
  - `docker wait` - Waits for container completion
  - `docker logs` - Captures execution logs
  - `docker rm` - Cleans up containers
  - `docker rmi` - Cleans up images

**Files:**
- `internal/worker/docker_runner.go` - DockerRunner handles Docker lifecycle (buildImage, runContainer, waitAndGetLogs, cleanup)

## Job Queue & Worker Pool

**Queue:**
- In-memory job queue: `internal/queue/docker_queue.go`
- Capacity: Configurable (default: 10 jobs)
- Implementation: Thread-safe with sync.Cond for blocking Dequeue
- No external message queue (Kafka, RabbitMQ, etc.)

**Worker Pool:**
- In-memory worker pool: `internal/worker/pool.go`
- Pool size: Configurable (default: 3 workers)
- No external job scheduler

## Git Repository Access

**Git Integration:**
- Optional - Configuration field `GitRepo` and `GitBranch` in `core.DockerfileConfig`
- Implementation: Git cloned inside Docker container during Dockerfile generation
- Method: `RUN git clone --branch {branch} --single-branch {repo} .`
- Cloning via: Dockerfile RUN instruction (requires git in base image)

**Files:**
- `internal/core/dockerfile_config.go` - GitRepo and GitBranch fields
- `cmd/api/docker/dockerfile/create.go` - GenerateDockerfile() adds git clone RUN instruction

---

*Integration audit: 2026-02-27*
