# Research: Phase 09 - REST API

## Goal
Implement a Go REST API for process management (CRUD, lifecycle, logs) using Go 1.22+ `http.ServeMux` and the existing `internal/scheduler` package.

## Requirements Mapping
- **API-01**: `GET /api/processes` -> `scheduler.List()`
- **API-02**: `POST /api/processes` -> `scheduler.Register(def)`
- **API-03**: `GET /api/processes/{name}` -> `scheduler.Get(name)`
- **API-04**: `PUT /api/processes/{name}` -> Update definition (only if stopped)
- **API-05**: `DELETE /api/processes/{name}` -> `scheduler.Remove(name)`
- **API-06**: `POST /api/processes/{name}/start` -> `scheduler.Start(name)`
- **API-07**: `POST /api/processes/{name}/stop` -> `scheduler.Stop(name)`
- **API-08**: `GET /api/processes/{name}/logs` -> `scheduler.Logs(name)`
- **API-09**: CORS middleware for React frontend.

## Technical Decisions

### 1. Router: `http.ServeMux` (Go 1.22+)
Go 1.22 introduced enhanced routing patterns:
- `GET /api/processes`
- `POST /api/processes`
- `GET /api/processes/{name}`
- `PUT /api/processes/{name}`
- `DELETE /api/processes/{name}`
- `POST /api/processes/{name}/start`
- `POST /api/processes/{name}/stop`
- `GET /api/processes/{name}/logs`

This satisfies all v1.1 requirements without external dependencies like `chi` or `gorilla/mux`.

### 2. Response Envelope (from 09-CONTEXT.md)
```json
{
  "data": <payload_or_null>,
  "error": <message_or_null>
}
```
All responses will use this envelope. Errors will return appropriate HTTP status codes (400, 404, 500, etc.) but still include the envelope.

### 3. Error Handling
Create a helper `sendJSON(w, status, data, err)` to ensure consistency.

### 4. JSON Models
We need to map `internal/scheduler` types to JSON-friendly structures.
- `ProcessDef`: `RestartPolicy.Delay` and `StopTimeout` are `time.Duration` (int64 nanoseconds). In JSON, we should probably use seconds (int) or strings ("5s").
- **Decision**: Use seconds (integers) in the API for simplicity in v1.1. `time.Duration(seconds) * time.Second`.
- `State`: Needs to be converted to its string representation ("running", "stopped", etc.).

### 5. Codebase Cleanup (Urgent)
The `api-service/` directory is legacy code that doesn't compile and uses `github.com/google/uuid`. It must be removed to restore a clean build.

## Proposed Strategy

### Plan 01: API Foundation & Cleanup
- `git rm -rf api-service/` to remove legacy code.
- Create `internal/api/server.go` and `internal/api/handlers.go`.
- Implement `Server` struct that holds the `*scheduler.Scheduler`.
- Implement JSON envelope helpers.
- Implement `GET /api/processes` and `POST /api/processes`.

### Plan 02: Full Lifecycle & Log Handlers
- Implement remaining handlers (Get, Put, Delete, Start, Stop, Logs).
- Add CORS middleware.
- Add unit tests for handlers using `net/http/httptest`.

## Verification Plan
- `go build ./...` must exit 0.
- `go test ./internal/api/...` must pass with high coverage.
- Manual verification using `curl` or `postman` once `rtx serve` is added in Phase 10.
