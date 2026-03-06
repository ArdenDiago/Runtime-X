package api

import (
	"encoding/json"
	"net/http"

	"runtimex/internal/scheduler"
)

// Server holds the HTTP server dependencies. Construct with NewServer and call
// Routes() to obtain the handler to pass to http.ListenAndServe.
type Server struct {
	Scheduler *scheduler.Scheduler
}

// NewServer creates a new Server backed by the given Scheduler.
func NewServer(s *scheduler.Scheduler) *Server {
	return &Server{Scheduler: s}
}

// Routes returns an http.Handler with all API routes registered.
// Uses Go 1.22+ ServeMux method+path patterns. CORS middleware wraps the mux.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Collection routes (API-01, API-02)
	mux.HandleFunc("GET /api/processes", s.ListProcesses)
	mux.HandleFunc("POST /api/processes", s.CreateProcess)

	// Resource routes (API-03, API-04, API-05)
	mux.HandleFunc("GET /api/processes/{name}", s.GetProcess)
	mux.HandleFunc("PUT /api/processes/{name}", s.UpdateProcess)
	mux.HandleFunc("DELETE /api/processes/{name}", s.DeleteProcess)

	// Lifecycle routes (API-06, API-07)
	mux.HandleFunc("POST /api/processes/{name}/start", s.StartProcess)
	mux.HandleFunc("POST /api/processes/{name}/stop", s.StopProcess)

	// Log route (API-08)
	mux.HandleFunc("GET /api/processes/{name}/logs", s.GetLogs)

	return corsMiddleware(mux)
}

// envelope is the standard JSON response wrapper.
type envelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// send writes a JSON response with the given status code.
// If err is non-nil the Error field is populated; otherwise Data is used.
func send(w http.ResponseWriter, status int, data any, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	var resp envelope
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Data = data
	}

	_ = json.NewEncoder(w).Encode(resp)
}
