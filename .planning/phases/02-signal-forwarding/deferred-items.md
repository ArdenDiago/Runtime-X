# Deferred Items — Phase 02 Signal Forwarding

## Pre-existing Build Failures (Out of Scope)

**Discovered during:** 02-01 Task 2 verification (`go build ./...`)

**Issue:** `internal/api/handlers.go` has pre-existing compilation errors unrelated to the process runner:
- `h.Scheduler undefined (type *TaskHandler has no field or method Scheduler)`
- `undefined: models`
- `h.Queue undefined (type *TaskHandler has no field or method Queue, but does have field queue)`

**Verified pre-existing:** Confirmed by stashing Phase 02-01 changes and reproducing the same errors.

**Impact on current plan:** None — the plan only modifies `internal/process/runner.go` and `cmd/rtx`. Both packages build and vet cleanly.

**Action required:** Fix `internal/api/handlers.go` in a future phase or separate task dedicated to the API layer.
