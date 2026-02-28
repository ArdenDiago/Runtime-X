# Deferred Items - Phase 01

## Pre-existing Issues (Out of Scope)

### 1. internal/api/tasks_test.go import path mismatch
- **Found during:** Task 2 (01-02)
- **File:** internal/api/tasks_test.go
- **Issue:** Imports `runtimex/api-service/internal/models` and `runtimex/api-service/internal/scheduler` which don't exist under the current module structure. The file predates the Runtime-X refactor (first committed in `8b852ff`).
- **Impact:** `go vet ./...` fails; `go vet ./cmd/... ./internal/process/...` passes cleanly.
- **Action needed:** Fix import paths in tasks_test.go to match current module layout, or remove the file if the old api-service tests are no longer relevant.
