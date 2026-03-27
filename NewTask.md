# Runtime-X Task Description: Feature Implementation

This document outlines the required changes to implement two new features for the Runtime-X project on the `Sooryananda_2547147` branch. 

## 1. API Request Logging Middleware
**Description:** A middleware that logs every incoming HTTP request method, path, and duration.
**Target File:** `internal/api/server.go`

### Implementation Steps:
1. Open `internal/api/server.go`.
2. Add `"log"` and `"time"` to the import block.
3. Create a new middleware function `loggingMiddleware`.
```go
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Serve the request
		next.ServeHTTP(w, r)
		
		// Log method, path, and duration
		log.Printf("method=%s path=%s duration=%s", r.Method, r.URL.Path, time.Since(start))
	})
}
```
4. Update the `Routes()` method to wrap the multiplexer with the `loggingMiddleware` before returning.
```go
func (s *Server) Routes() http.Handler {
	// ... (existing route definitions)

	// Wrap CORS and logging middleware
	return loggingMiddleware(corsMiddleware(mux))
}
```

## 2. Case-Insensitive Process Search
**Description:** Enhance `GET /api/processes` to support a query parameter (e.g., `?q=web`) for filtering.
**Target File:** `internal/api/handlers.go`

### Implementation Steps:
1. Open `internal/api/handlers.go`.
2. Add `"strings"` to the import declarations.
3. Locate the `ListProcesses` handler method.
4. Modify `ListProcesses` to extract the query parameter `q` and filter the `snaps` array (case-insensitively).

```go
// ListProcesses handles GET /api/processes.
func (s *Server) ListProcesses(w http.ResponseWriter, r *http.Request) {
	// 1. Get query parameter "q"
	query := r.URL.Query().Get("q")
	queryLower := strings.ToLower(query)

	snaps := s.Scheduler.SnapshotAll()
	out := make([]processJSON, 0, len(snaps))
	
	for _, snap := range snaps {
		// 2. If 'q' is given, check for case-insensitive match
		if query != "" {
			nameLower := strings.ToLower(snap.Def.Name)
			if !strings.Contains(nameLower, queryLower) {
				continue // Skip if it doesn't match
			}
		}
		
		out = append(out, snapshotToJSON(snap))
	}
	send(w, http.StatusOK, out, nil)
}
```

## Execution Instructions for AI Agents
- Verify that `Sooryananda_2547147` is checked out safely.
- Make the modifications clearly and verify that import references are complete.
- Do not remove any existing routes or handlers; only wrap the multiplexer and modify iteration logic as described.
