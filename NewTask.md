# NewTask: Implementation of Person A Features

This task involves implementing two high-impact, low-complexity backend features for the Runtime X project. 

## Features to Implement

### 1. API Request Logging Middleware
**Goal:** Track every incoming HTTP request to the API with its method, URL path, and processing duration.

#### Implementation Steps:
1.  **Modify [internal/api/server.go](internal/api/server.go):**
    *   Create a new function `logMiddleware(next http.Handler) http.Handler`.
    *   Inside the middleware, capture the starting time using `time.Now()`.
    *   Call `next.ServeHTTP(w, r)`.
    *   After the call returns, calculate the duration using `time.Since()`.
    *   Print the results to `os.Stdout` in a clear format, e.g., `[API] 2026/03/27 10:00:00 GET /api/processes took 1.2ms`.
2.  **Apply Middleware:**
    *   In the `Routes()` method of `Server`, wrap the existing `corsMiddleware` with the new `logMiddleware`.
    *   The final return should look like: `return logMiddleware(corsMiddleware(mux))`.

### 2. Case-Insensitive Process Search
**Goal:** Add a search capability to the `GET /api/processes` endpoint using a query parameter.

#### Implementation Steps:
1.  **Modify `ListProcesses` in [internal/api/handlers.go](internal/api/handlers.go):**
    *   Extract the search query from the request using `r.URL.Query().Get("q")` or `r.URL.Query().Get("search")`.
    *   If a query exists, convert it to lowercase using `strings.ToLower()`.
    *   During the loop that iterates over snapshots (`snaps`), add a check:
        *   Convert the process name to lowercase.
        *   Use `strings.Contains(lowerName, lowerQuery)` to determine if the process should be included in the response.
    *   If no query is provided, return all processes as before.
2.  **Verify:**
    *   Ensure that `GET /api/processes?q=web` returns only processes with "web" in their name (e.g., "web-server", "frontend-web").

---

## Instructions for AI Agent
*   Maintain the existing code style and naming conventions.
*   Ensure that `internal/api/server.go` and `internal/api/handlers.go` are the only files modified unless imports need adjustment.
*   Use standard library packages (`time`, `strings`, `log` or `fmt`) for implementation.
*   Run tests in `internal/api/handlers_test.go` after changes to ensure no regressions.
