# Testing Patterns

**Analysis Date:** 2026-02-27

## Test Framework

**Runner:**
- Go standard `testing` package (built-in)
- No external testing framework (not `testify`, `ginkgo`, etc.)
- Tests run with: `go test ./...`

**Run Commands:**
```bash
go test ./...              # Run all tests
go test -v ./...           # Verbose output with test names
go test -run TestName      # Run specific test by name
go test -count=1 ./...     # Disable test caching
```

**Test Output Mode:**
Standard Go test output format:
- `=== RUN   TestName` - indicates test start
- `--- PASS: TestName (0.00s)` - indicates test completion
- Subtests shown with indentation: `--- PASS: TestName/subtest_name (0.00s)`

## Test File Organization

**Location:**
- Co-located with source files in same package
- Example: `internal/docker/validator.go` with `internal/docker/validator_test.go`

**Naming:**
- Files: `*_test.go` suffix required by Go
- Test functions: `Test<Function>` convention (PascalCase)
- Example test files:
  - `internal/api/tasks_test.go`
  - `internal/docker/validator_test.go`

**Test Package:**
- Same package as code being tested
- Example: `validator_test.go` is in `package docker` (same as `validator.go`)

## Test Structure

**Basic Test Function Pattern:**
```go
func TestValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"valid image", "golang", false},
		{"empty image", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageName(tt.image)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateImageName(%q) error = %v, wantErr %v", tt.image, err, tt.wantErr)
			}
		})
	}
}
```

**Setup Pattern (from `tasks_test.go`):**
```go
func TestCreateTask(t *testing.T) {
	// Setup
	sched := scheduler.NewScheduler()
	queue := worker.NewJobQueue(5)

	handler := &TaskHandler{
		Scheduler: sched,
		Queue:     queue,
	}

	// Test execution follows setup
}
```

**HTTP Handler Testing Pattern (from `tasks_test.go`):**
```go
func TestCreateTask(t *testing.T) {
	// Setup handler and dependencies
	handler := &TaskHandler{...}

	// Create request with body
	body := []byte(`{"command":"echo test"}`)
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.CreateTask(rr, req)

	// Assertions
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}
```

**Assertion Pattern:**
- Use `t.Fatalf()` for fatal test failures (stops test immediately)
- Use `t.Errorf()` for non-fatal assertion failures (test continues)
- Use `t.Run()` for subtests with table-driven testing

## Test Structure Patterns Observed

**Table-Driven Tests:**
All validation tests use table-driven pattern:
```go
tests := []struct {
	name    string
	image   string
	wantErr bool
}{
	{"valid image", "golang", false},
	{"empty image", "", true},
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		// test implementation
	})
}
```

**Error Assertion Pattern:**
```go
if (err != nil) != tt.wantErr {
	t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
}
```

**Value Assertion Pattern:**
```go
if task.Command != "echo test" {
	t.Fatalf("expected command 'echo test', got '%s'", task.Command)
}
```

**Response Assertion Pattern (HTTP handlers):**
```go
if rr.Code != http.StatusOK {
	t.Fatalf("expected status 200, got %d", rr.Code)
}

var task models.Task
if err := json.NewDecoder(rr.Body).Decode(&task); err != nil {
	t.Fatalf("failed to decode response: %v", err)
}
```

## Mocking

**Framework:** None - uses manual mocking and dependency injection

**Patterns:**
- Pass interfaces or concrete types to functions for testing
- Create test instances directly: `sched := scheduler.NewScheduler()`
- Use dependency injection in handlers: `TaskHandler{Scheduler: sched, Queue: queue}`
- No mocking library used (no `gomock`, `testify/mock`, etc.)

**What to Mock:**
- External services that have side effects (implied but not seen in current tests)
- Long-running operations
- Network calls
- Concurrent operations that need controlled timing

**What NOT to Mock:**
- Core business logic (validate directly)
- In-memory data structures used for testing
- Constructor functions
- Helper functions that don't have external dependencies

**Dependency Injection for Testing:**
- Handlers accept dependencies in struct fields: `Queue *queue.JobQueue`, `Scheduler *scheduler.Scheduler`
- Allow test code to inject test doubles by creating instances with test values

## Fixtures and Test Data

**Test Data Patterns:**
- Inline struct literals for simple test data:
  ```go
  task := models.Task{
      ID:      "1",
      Command: "echo hello",
      Status:  models.TaskPending,
  }
  ```

- Table-driven test structs for validation cases (seen extensively in `validator_test.go`):
  ```go
  config := &core.DockerfileConfig{
      Image:       "python",
      Tag:         "3.11",
      CopyPaths:   []core.CopyPath{{Source: "./app", Destination: "/app"}},
      RunCommands: []string{"pip install flask"},
  }
  ```

**HTTP Test Data:**
- JSON payloads created as byte slices:
  ```go
  body := []byte(`{"command":"echo test"}`)
  req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(body))
  ```

**Location:** Test data defined inline in test functions (no separate fixture files)

## Coverage

**Requirements:** No coverage requirements enforced

**View Coverage:**
```bash
go test -cover ./...              # Show coverage percentage per package
go test -coverprofile=coverage.out ./...  # Generate coverage profile
go tool cover -html=coverage.out  # View HTML coverage report
```

## Test Types

**Unit Tests:**
- Scope: Individual functions and validation logic
- Approach: Direct function calls with known inputs
- Examples:
  - `TestValidateImageName()` - tests image validation function
  - `TestValidateTagName()` - tests tag validation
  - `TestValidateCopyPath()` - tests path validation with multiple error cases
  - `TestValidateRunCommand()` - tests command safety validation
  - `TestValidateWorkDir()` - tests directory path validation
  - `TestValidateDockerfileConfig()` - tests comprehensive config validation

- No mocking used - validation functions are pure functions with no external dependencies

**Integration Tests:**
- Scope: HTTP handlers with queues and schedulers
- Approach: Create handler with injected dependencies, test full request/response cycle
- Examples:
  - `TestCreateTask()` - tests HTTP handler creating task with scheduler and queue
  - `TestListTasks()` - tests listing with pre-added tasks
  - `TestExecuteTask()` - tests task execution flow through scheduler
- Use `httptest.NewRequest()` and `httptest.NewRecorder()` for HTTP testing

**E2E Tests:**
- Not present in current codebase
- No integration test framework detected

## Common Patterns

**Table-Driven Test Execution:**
```go
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		// Use tt.field to access test case data
	})
}
```

**Subtest Naming:**
Test names automatically formatted as `TestName/subtest_name` from `t.Run(tt.name, ...)`

**Error Value Testing:**
```go
err := ValidateImageName(tt.image)
if (err != nil) != tt.wantErr {
	t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
}
```

**Multiple Error Testing:**
```go
errs := ValidateCopyPath(tt.copyPath, 0)
if len(errs) != tt.wantErrs {
	t.Errorf("ValidateCopyPath() errors = %d, wantErrs %d", len(errs), tt.wantErrs)
}
```

**Complex Result Assertion:**
```go
result := ValidateDockerfileConfig(tt.config)
if result.Valid != tt.wantValid {
	t.Errorf("ValidateDockerfileConfig() valid = %v, wantValid %v, errors = %v",
		result.Valid, tt.wantValid, result.Errors)
}
```

**HTTP Status Assertion:**
```go
if rr.Code != http.StatusOK {
	t.Fatalf("expected status 200, got %d", rr.Code)
}
```

**JSON Response Parsing:**
```go
var task models.Task
if err := json.NewDecoder(rr.Body).Decode(&task); err != nil {
	t.Fatalf("failed to decode response: %v", err)
}
```

**Assertions:**
- Use `t.Fatalf()` for assertion failures that should stop the test
- Use `t.Errorf()` for comparison errors that log but continue
- Error messages include both actual and expected: `expected status 200, got %d`
- Field information included: `expected command 'echo test', got '%s'`

## Test Package Dependencies

**Imports Used in Tests:**
- Standard: `bytes`, `encoding/json`, `net/http`, `net/http/httptest`, `testing`
- Internal: `runtimex/internal/...`, `runtimex/worker`, package-under-test

**Testing Utilities:**
- `httptest.NewRequest()` - create test HTTP requests
- `httptest.NewRecorder()` - capture HTTP responses
- `json.NewDecoder()` - decode response bodies
- Standard `*testing.T` methods only (no helper wrappers)

---

*Testing analysis: 2026-02-27*
