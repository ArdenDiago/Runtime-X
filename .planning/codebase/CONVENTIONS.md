# Coding Conventions

**Analysis Date:** 2026-02-27

## Naming Patterns

**Files:**
- Lowercase with underscores for multi-word names: `docker_runner.go`, `docker_queue.go`, `dockerfile_config.go`
- Test files use `_test.go` suffix: `validator_test.go`, `tasks_test.go`
- Package names are single words: `worker`, `queue`, `docker`, `core`, `logging`

**Functions:**
- PascalCase for exported functions: `NewDockerRunner()`, `ValidateImageName()`, `RunDockerJob()`
- camelCase for unexported functions: `buildImage()`, `runContainer()`, `waitAndGetLogs()`
- Constructor functions follow pattern `New<Type>()`: `NewWorkerPool()`, `NewJobQueue()`
- Validation functions prefixed with `Validate`: `ValidateImageName()`, `ValidateCopyPath()`, `ValidateDockerfileConfig()`
- Handler methods follow HTTP convention: `CreateTask()`, `ListTasks()`, `ExecuteTask()`

**Variables:**
- Short descriptive names: `ctx`, `err`, `job`, `config`, `imageName`, `containerID`
- Loop variables: `i`, `j` for indices; `tt` for test table cases
- Interface parameters: `r *http.Request`, `w http.ResponseWriter`
- Receiver names are single letters or short abbreviations: `(dr *DockerRunner)`, `(p *WorkerPool)`, `(h *TaskHandler)`

**Types:**
- PascalCase for struct and interface names: `DockerRunner`, `ValidationError`, `JobQueue`, `WorkerPool`
- Type aliases for simple types are PascalCase: `JobStatus`
- Interface names end with 'er' when representing a behavior: (not explicitly used but would follow Go conventions)

**Constants:**
- PascalCase or SCREAMING_SNAKE_CASE for constants: `StatusQueued`, `StatusRunning` (exported); dangerous patterns stored as string slice elements

## Code Style

**Formatting:**
- Standard Go formatting (gofmt conventions implied)
- Imports organized in groups: standard library, then third-party, then internal packages
- Imports from `runtimex` packages use full module path: `"runtimex/internal/core"`
- Blank line separates import groups

**Linting:**
- No explicit linter config files detected
- Follows standard Go idioms (error handling, defer patterns)
- Unused variable prevention pattern: `var _ = os.Getenv` (see `docker_runner.go` line 202)

**Import Organization Example** (from `docker_runner.go`):
```go
import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"runtimex/internal/core"
	"runtimex/internal/logging"
)
```

## Import Organization

**Order:**
1. Standard library imports (alphabetical): `context`, `fmt`, `log`, `net/http`, `os`, `sync`, `time`, etc.
2. Blank line separator
3. Third-party imports: `"github.com/google/uuid"`
4. Blank line separator
5. Internal imports from `runtimex` package: `"runtimex/internal/..."`, `"runtimex/cmd/..."`

**Path Aliases:**
- Explicit aliasing for disambiguation (seen in `dockerfile/create.go`):
  ```go
  dockerValidator "runtimex/internal/docker"
  imageRoutes "runtimex/cmd/api/docker/image"
  ```

## Error Handling

**Patterns:**
- Errors returned as last return value: `func (q *JobQueue) Enqueue(job *core.Job) error`
- Error wrapping with context using `fmt.Errorf()`: `fmt.Errorf("docker build failed: %w - %s", err, stderr.String())`
- Errors logged with `log.Printf()` for operational events
- Simple HTTP error responses with `http.Error(w, message, statusCode)`
- Custom error types for domain-specific validation: `ValidationError` struct with `Field` and `Message` fields
- Predefined package-level errors in `core/errors.go`: `ErrQueueFull`, `ErrJobNotFound`, `ErrTimeout`, etc.
- Error handling patterns:
  - Check error immediately: `if err := someFunc(); err != nil { return err }`
  - Defer cleanup on error: `defer pool.Release()`
  - Log and continue on non-critical errors: `log.Printf("Warning: failed to remove: %v", err)`

## Logging

**Framework:** Standard Go `log` package

**Patterns:**
- Use `log.Println()` for basic info messages
- Use `log.Printf()` for formatted messages with context
- Log job lifecycle events: creation, status changes, completion
- Include identifiers: `Job=%s Status=%s Error=%s` in LogJob function
- Docker operations logged with contextual info: `log.Printf("Building Docker image: %s from %s", imageName, filepath)`
- Container lifecycle events logged: start, wait, logs retrieval, cleanup
- Warning logs for non-fatal failures: `log.Printf("Warning: failed to remove image %s: %v", imageName, err)`

Example (from `logging/logger.go`):
```go
func LogJob(job *core.Job) {
	log.Printf(
		"Job=%s Status=%s Error=%s",
		job.ID,
		job.Status,
		job.Error,
	)
}
```

## Comments

**When to Comment:**
- Function documentation comments (unexported functions are not documented)
- Complex algorithm explanations
- Non-obvious business logic or validation reasons
- Security-related decisions (e.g., dangerous command patterns to detect)
- Step-by-step process breakdowns in multi-step operations

**Patterns Observed:**
- Comment blocks above type definitions explain purpose: `// DockerRunner handles Docker container lifecycle for jobs`
- Comments describe WHY not WHAT: `// Wait for container to complete and capture logs` explains purpose
- Step comments in complex functions: `// Step 1: Build the Docker image`, `// Step 2: Run the container`
- Inline comments for non-obvious code: `// Detached mode`, `// Fork bomb`, `// Docker Hub limit is 128`
- Comments on struct fields explain purpose: (not extensively used but follow pattern when present)

**JSDoc/TSDoc:**
- Not applicable (this is Go, not TypeScript/JavaScript)
- Go documentation uses comment convention above exported identifiers (standard Go doc)

## Function Design

**Size:**
- Small focused functions: typically 15-50 lines
- Complex operations (like `RunDockerJob`) broken into helper methods: `buildImage()`, `runContainer()`, `waitAndGetLogs()`
- Validation functions are single-responsibility: each validates one aspect
- Handler functions kept under 25 lines

**Parameters:**
- Prefer passing initialized objects over primitives: `(config *core.DockerfileConfig)`
- Use time.Duration for timeout parameters: `NewDockerRunner(timeout time.Duration)`
- HTTP handlers always receive `(w http.ResponseWriter, r *http.Request)`
- Context passed as first parameter: `(ctx context.Context, config ...)`

**Return Values:**
- Single return value for simple operations: `func (p *WorkerPool) Acquire()`
- Error as last return value: `func Enqueue(job *core.Job) error`
- Multiple meaningful returns: `func waitAndGetLogs() (string, int, error)` - logs, exitCode, error
- Return custom types for complex results: `ValidationResult` contains both `Valid` bool and error `[]ValidationError`
- Pointer receivers return `*Type` to allow method calls: `NewWorkerPool() *WorkerPool`

## Module Design

**Exports:**
- All exported functions and types start with uppercase (Go convention)
- Unexported helper functions use lowercase
- Each package has single responsibility:
  - `core`: data types and errors
  - `worker`: job execution and pooling
  - `queue`: job queue management
  - `docker`: Docker validation
  - `logging`: structured logging

**Barrel Files:**
- No barrel/index files observed
- Each package defines its own types and functions
- Internal structure documented through directory layout

**Package Organization:**
- `internal/`: Contains reusable internal logic (core, worker, queue, docker, logging)
- `cmd/`: Contains application entry points and route handlers
- Clear separation between internal API (`internal/api`) and external API (`cmd/api/docker`)
- Imports reference `runtimex/` module prefix for all packages

## Type Organization

**Receiver Pattern:**
All methods use value or pointer receivers consistently:
- Pointer receivers for types that maintain state: `(dr *DockerRunner)`, `(q *JobQueue)`, `(p *WorkerPool)`
- Methods that acquire locks use pointer receivers: `(q *JobQueue) Enqueue()`
- Constructor functions return pointers: `func NewWorkerPool() *WorkerPool`

**Struct Field Organization:**
- Public fields first, alphabetically: `CmdCommand`, `CopyPaths`, `Image`, `Tag`, `WorkDir`
- Mutex fields named `mu` placed early in struct: `mu sync.Mutex`
- Condition variables follow mutex: `cond *sync.Cond`
- Slices and channels for internal state: `jobs []*core.Job`, `sem chan struct{}`

---

*Convention analysis: 2026-02-27*
