# Codebase Concerns

**Analysis Date:** 2026-02-27

## Tech Debt

**Infinite Goroutine Loops - No Shutdown Mechanism:**
- Issue: `StartDockerScheduler()` and `StartScheduler()` spawn infinite loops without stop signals
- Files: `internal/worker/docker_scheduler.go`, `internal/worker/scheduler.go`
- Impact: Scheduler cannot be gracefully shut down; leaks goroutines on application exit
- Fix approach: Add a context-based cancellation mechanism or channel-based stop signal. Wrap `for { ... }` in `for { select { case <-ctx.Done(): return default: } }` pattern

**Unused Queue Implementation:**
- Issue: `internal/queue/queue.go` and `internal/queue/runner.go` exist but are not used by the Docker scheduler
- Files: `internal/queue/queue.go`, `internal/queue/runner.go` (not found but referenced)
- Impact: Code duplication and maintenance burden; unclear which queue implementation is the canonical one
- Fix approach: Consolidate to use only `DockerJobQueue` from `internal/queue/docker_queue.go` and remove generic queue

**Hardcoded Timeout Values:**
- Issue: Hardcoded 10-minute timeout in main and 10-second timeout in HTTP validators
- Files: `cmd/main.go` (line 19), `internal/docker/validator.go` (lines 204, 226)
- Impact: Cannot adjust timeouts without recompilation; may timeout too aggressively or allow slow operations
- Fix approach: Load timeouts from environment variables or configuration file

**Bare Logging with log Package:**
- Issue: Uses Go's standard `log` package directly instead of structured logging
- Files: `internal/logging/logger.go`, `internal/worker/docker_runner.go`, `cmd/main.go`
- Impact: Logs lack structure; difficult to parse, filter, or aggregate; no severity levels
- Fix approach: Migrate to structured logger (slog, zap, or logrus); add context fields like trace IDs

**Path Traversal Vulnerability in Dockerfile Retrieval:**
- Issue: `GetDockerfile()` uses `filepath.Base()` which prevents traversal, but validates insufficiently
- Files: `cmd/api/docker/dockerfile/create.go` (lines 158-177)
- Impact: Malicious ID like `../../../etc/passwd` could theoretically bypass validation
- Fix approach: Add explicit UUID validation before file lookup; reject IDs not matching UUID pattern

**Unsafe Dockerfile Generation (Command Injection Risk):**
- Issue: `GenerateDockerfile()` concatenates user input directly into RUN and CMD instructions without escaping
- Files: `cmd/api/docker/dockerfile/create.go` (lines 222-243)
- Impact: If `RunCommands` or `CmdCommand` contain shell metacharacters, they could inject arbitrary Docker instructions
- Fix approach: Properly escape/quote user inputs or use parameterized Dockerfile generation

**No Validation of Git Repository URL:**
- Issue: Git repo URLs are not validated before insertion into RUN instructions
- Files: `cmd/api/docker/dockerfile/create.go` (line 210)
- Impact: Can accept malformed URLs that cause Docker build failures; no URL format validation
- Fix approach: Validate URL format and allowed schemes before accepting

**Missing Error Propagation from Cleanup Operations:**
- Issue: `cleanup()` function silently logs cleanup failures but doesn't propagate errors
- Files: `internal/worker/docker_runner.go` (lines 181-199)
- Impact: Image and container cleanup failures are logged but don't affect job status; orphaned resources accumulate
- Fix approach: Track cleanup errors in config status or return errors for monitoring

**Race Condition in Worker Pool Semaphore:**
- Issue: `WorkerPool` uses unbuffered channel; potential deadlock if `Release()` called without `Acquire()`
- Files: `internal/worker/pool.go`
- Impact: Incorrect usage could deadlock the scheduler; no way to detect misuse
- Fix approach: Add assertions or use a counter-based pool with bounds checking

---

## Known Bugs

**Dockerfile Generation Missing Quotes in ENV Variables:**
- Symptoms: Environment variables with spaces are written without quotes; `ENV FOO=bar baz` creates invalid Dockerfile
- Files: `cmd/api/docker/dockerfile/create.go` (line 192)
- Trigger: Submit EnvVars with values containing spaces (e.g., `{"GREETING": "hello world"}`)
- Workaround: Client must pre-escape values

**Broken Handler Reference in handlers.go:**
- Symptoms: `ExecuteTask()` references undefined `h.Scheduler` and `models.TaskRunning`
- Files: `internal/api/handlers.go` (lines 63, 69)
- Trigger: Any call to ExecuteTask endpoint
- Impact: Code does not compile; referenced types/fields do not exist
- Workaround: Code path likely untested; would panic at runtime

**Docker Context Path Issues:**
- Symptoms: Docker build command runs in `docker_files/` directory but references relative paths
- Files: `internal/worker/docker_runner.go` (line 109)
- Trigger: Any job with COPY instructions using relative paths
- Impact: COPY operations fail because context is `docker_files/` not project root
- Workaround: Require absolute paths in COPY instructions

**Test File Imports Non-Existent Packages:**
- Symptoms: `internal/api/tasks_test.go` imports packages that don't exist (`runtimex/api-service/...`)
- Files: `internal/api/tasks_test.go` (lines 10-12)
- Impact: Test file cannot compile; tests are skipped/broken
- Workaround: None; code is unmaintainable

---

## Security Considerations

**Shell Injection in RunJob:**
- Risk: `exec.Command("sh", "-c", job.Command)` executes arbitrary shell commands from queue
- Files: `internal/worker/runner.go` (line 16)
- Current mitigation: None; API validation is weak
- Recommendations:
  - Don't use shell (`-c` flag); parse command into argv array
  - Whitelist allowed commands/scripts
  - Add audit logging for executed commands
  - Sandbox execution environment

**Docker Hub API Unauthenticated Requests:**
- Risk: Rate limiting (100 requests/hour unauthenticated); image validation calls waste quota
- Files: `internal/docker/validator.go` (lines 201-241), `cmd/api/docker/utility/dockerhub.go`
- Current mitigation: 10-second timeout only
- Recommendations:
  - Add Docker Hub API credentials for authenticated requests
  - Implement request caching/memoization
  - Add fallback behavior when rate limited
  - Monitor request counts

**No Input Size Limits:**
- Risk: Arbitrarily large RunCommands, CopyPaths arrays could cause memory exhaustion
- Files: `cmd/api/docker/dockerfile/create.go` (lines 25-26)
- Current mitigation: None
- Recommendations:
  - Add max limits: e.g., `len(RunCommands) <= 50`, total payload size <= 1MB
  - Validate at API handler level

**Dockerfile Written with Permissive Permissions:**
- Risk: Dockerfiles contain user-submitted git repo URLs and commands; written as 0644 (world-readable)
- Files: `cmd/api/docker/dockerfile/create.go` (line 123)
- Current mitigation: None
- Recommendations:
  - Consider using 0600 (owner-only readable) if files may contain sensitive data
  - Move docker_files to non-web-accessible directory

**No Access Control on Dockerfile Retrieval:**
- Risk: Anyone can retrieve generated Dockerfiles by guessing or brute-forcing UUIDs
- Files: `cmd/api/docker/dockerfile/create.go` (lines 151-181)
- Current mitigation: UUIDs are cryptographically random
- Recommendations:
  - Add authentication/authorization to GetDockerfile endpoint
  - Consider requiring API key or session token

---

## Performance Bottlenecks

**Synchronous Docker Hub API Calls on Every Request:**
- Problem: Each CreateDockerfile request makes 2 blocking HTTP calls to Docker Hub (image + tag validation)
- Files: `cmd/api/docker/dockerfile/create.go` (lines 100-108)
- Cause: No caching, no async batching, 10-second timeout per call
- Improvement path:
  - Add in-memory cache with TTL (cache for 1 hour)
  - Make validation async/optional (warnings only)
  - Use skipHubValidation flag default to true for faster responses

**Queue.Dequeue() Blocks All Worker Threads:**
- Problem: Single blocking call to `q.Dequeue()` in scheduler goroutine; if queue is empty, scheduler thread blocks
- Files: `internal/worker/docker_scheduler.go` (line 16)
- Cause: `sync.Cond.Wait()` releases lock but blocks thread
- Improvement path:
  - Add timeout to Wait() call
  - Implement heartbeat/polling to detect hung scheduler
  - Monitor goroutine count

**Dockerfile Generation String Concatenation:**
- Problem: Uses `content += ...` in loop for Dockerfile generation (247 lines total)
- Files: `cmd/api/docker/dockerfile/create.go` (lines 184-246)
- Cause: Each append allocates new string; O(n²) behavior for large configs
- Improvement path: Use `strings.Builder` for concatenation; ~50x faster

**Unbounded Image Fetching from Docker Hub:**
- Problem: `GetOfficialDockerImages(0)` fetches ALL official images; pagination loop has no limits
- Files: `cmd/api/docker/utility/dockerhub.go` (lines 34-60)
- Cause: No max result limit; can fetch 50000+ images
- Improvement path:
  - Add hard limit (e.g., 1000 max)
  - Add timeout for entire operation
  - Cache results per process

---

## Fragile Areas

**Docker Build Context Assumptions:**
- Files: `internal/worker/docker_runner.go` (lines 98-121)
- Why fragile:
  - Assumes Dockerfile is in `docker_files/` directory
  - Uses `filepath.Dir()` and `.` as build context
  - If relative paths in COPY don't match actual filesystem, builds silently fail
- Safe modification:
  - Add validation that COPY sources exist before build
  - Log actual build context path
  - Test with various relative path scenarios
- Test coverage: No tests for docker_runner.go

**Scheduler Goroutine Never Recovers from Panic:**
- Files: `internal/worker/docker_scheduler.go` (lines 14-24)
- Why fragile:
  - Infinite loop `for { ... }` without panic recovery
  - If Dequeue() panics or RunDockerJob() panics, entire scheduler dies
  - No goroutine monitoring/restart
- Safe modification:
  - Wrap goroutine body in recover() defer
  - Log panics and restart scheduler
  - Implement health check endpoint
- Test coverage: No tests

**Queue Capacity Not Enforced on Edge Cases:**
- Files: `internal/queue/docker_queue.go` (lines 27-38)
- Why fragile:
  - Fixed 10-item capacity hardcoded in main
  - If all workers are busy, queue fills up and rejects new jobs
  - No backpressure mechanism or retry guidance
- Safe modification:
  - Add metrics for queue depth
  - Implement exponential backoff for client retries
  - Document queue behavior in API responses
- Test coverage: Basic unit tests exist but no concurrency tests

**Response Body Close Pattern Inconsistency:**
- Files: `cmd/api/docker/utility/dockerhub.go`, `cmd/api/docker/utility/dockerhub_tags.go`
- Why fragile:
  - Some paths close body in error handling, some don't
  - Line 41 closes before decode, then line 46-50 close after
  - Easy to add new error path and forget close
- Safe modification:
  - Use `defer resp.Body.Close()` immediately after Get()
  - Add linter rule (e.g., go-staticcheck)
- Test coverage: No tests for HTTP error paths

---

## Scaling Limits

**Single-Machine Worker Pool:**
- Current capacity: 3 concurrent jobs (hardcoded in main)
- Limit: Cannot exceed 3 parallel Docker builds on single machine
- Scaling path:
  - Make pool size configurable
  - Add worker process distribution (multiple machines)
  - Implement job queue persistence (Redis/RabbitMQ)

**Docker Images Stored Locally:**
- Current capacity: Limited by disk space on single machine
- Limit: After ~100 builds, disk fills up with image layers
- Scaling path:
  - Implement image registry cleanup (oldest images first)
  - Push built images to Docker Hub/private registry
  - Add image retention policies

**In-Memory Queue No Persistence:**
- Current capacity: Lost on process restart
- Limit: Jobs enqueued but not yet executed are lost on crash
- Scaling path:
  - Persist queue to database
  - Implement job recovery on startup
  - Add durability guarantees

**Concurrent HTTP Requests Unbounded:**
- Current capacity: Go's http server has no limits
- Limit: Thousands of concurrent requests cause memory exhaustion
- Scaling path:
  - Add max concurrent request limiting
  - Implement request queuing/backpressure
  - Add timeout for slow clients

---

## Dependencies at Risk

**Google UUID Package (github.com/google/uuid):**
- Risk: External dependency for UUID generation; adds build complexity
- Impact: Breaking changes in future versions could require code updates
- Migration plan: Switch to Go 1.16+ built-in `crypto/rand` with `encoding/hex` (eliminates dependency)

---

## Missing Critical Features

**No Job Persistence:**
- Problem: Job state and results are lost on service restart; no audit trail
- Blocks: Cannot guarantee job execution; impossible to debug failures
- Potential solution: Add PostgreSQL integration for job history and state

**No Monitoring/Observability:**
- Problem: No metrics, traces, or structured logging; impossible to debug production issues
- Blocks: Cannot diagnose performance problems or detect failures
- Potential solution: Add Prometheus metrics + OpenTelemetry tracing

**No Authentication/Authorization:**
- Problem: All APIs are public; anyone can submit arbitrary Docker builds
- Blocks: Unsafe for multi-tenant or production use
- Potential solution: Add API key validation or OAuth2

**No Rate Limiting:**
- Problem: Users can spam API with unlimited requests
- Blocks: Vulnerable to DoS attacks
- Potential solution: Add token bucket or sliding window rate limiter

**No Retry Logic:**
- Problem: Failed jobs are marked failed once; no automatic or manual retry
- Blocks: Transient failures (network timeouts) permanently fail jobs
- Potential solution: Add exponential backoff retry with configurable policies

**No Job Cancellation:**
- Problem: Running Docker builds cannot be stopped; must wait for timeout
- Blocks: Stuck builds waste resources
- Potential solution: Add graceful shutdown signal and cleanup on cancellation

---

## Test Coverage Gaps

**Docker Runner Untested:**
- What's not tested: Docker build/run/cleanup operations, timeout handling, context setup
- Files: `internal/worker/docker_runner.go` (202 lines, 0 tests)
- Risk: Changes to build process silently break; container cleanup failures undetected
- Priority: High - this is the critical execution path

**API Handlers Minimally Tested:**
- What's not tested: CreateDockerfile validation flow, GetDockerfile retrieval, error responses
- Files: `cmd/api/docker/dockerfile/create.go` (247 lines, 0 tests)
- Risk: Filesystem operations and Dockerfile generation bugs undetected
- Priority: High - this is the API entry point

**Worker Scheduler Untested:**
- What's not tested: Goroutine startup, job dequeue/execution, panic recovery
- Files: `internal/worker/docker_scheduler.go` (25 lines, 0 tests)
- Risk: Race conditions and deadlocks in scheduler loop
- Priority: High - affects all job execution

**Worker Pool Concurrency Untested:**
- What's not tested: Concurrent Acquire/Release, deadlock scenarios, edge cases
- Files: `internal/worker/pool.go` (20 lines, 0 tests)
- Risk: Semaphore bugs cause deadlocks or starvation
- Priority: Medium - affects concurrency guarantees

**Docker Hub API Calls Untested:**
- What's not tested: Rate limiting, network failures, pagination, retry behavior
- Files: `cmd/api/docker/utility/dockerhub.go` (64 lines, 0 tests)
- Risk: Unexpected Docker Hub API responses cause panics or hangs
- Priority: Medium - external dependency, production risk

**Integration Tests Missing:**
- What's not tested: End-to-end: API call → queue → build → cleanup → response
- Files: All files together
- Risk: Individual components work but integration fails silently
- Priority: High - blocks production deployment

**Error Path Testing:**
- What's not tested: Network timeouts, invalid Docker builds, cleanup failures, context cancellation
- Files: All error handling paths
- Risk: Error handling bugs (leaks, panics) only discovered in production
- Priority: High - reliability depends on error paths

---

*Concerns audit: 2026-02-27*
