package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"runtimex/internal/scheduler"
)

// newTestServer creates a Server backed by a fresh Scheduler for test isolation.
func newTestServer() *Server {
	return NewServer(scheduler.New())
}

// decodeResponse decodes the JSON envelope data field into v.
func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	var env struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if v != nil && env.Data != nil {
		if err := json.Unmarshal(env.Data, v); err != nil {
			t.Fatalf("decode data: %v", err)
		}
	}
}

// mustRegister registers a process or fatals the test.
func mustRegister(t *testing.T, s *scheduler.Scheduler, name string) {
	t.Helper()
	if err := s.Register(scheduler.ProcessDef{Name: name, Command: "/bin/echo", Args: []string{"hello"}}); err != nil {
		t.Fatalf("Register(%q): %v", name, err)
	}
}

// doRouteRequest sends a request through the full Routes() handler and returns the recorder.
func doRouteRequest(t *testing.T, srv *Server, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

// ── ListProcesses ─────────────────────────────────────────────────────────────

// TestListProcesses_Empty verifies that GET /api/processes on an empty scheduler
// returns 200 and an empty JSON array.
func TestListProcesses_Empty(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/processes", nil)
	rec := httptest.NewRecorder()

	srv.ListProcesses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var procs []processJSON
	decodeResponse(t, rec, &procs)

	if len(procs) != 0 {
		t.Fatalf("expected empty slice, got %d elements", len(procs))
	}
}

// TestListProcesses_NonEmpty verifies that GET /api/processes returns registered processes.
func TestListProcesses_NonEmpty(t *testing.T) {
	srv := newTestServer()
	def := scheduler.ProcessDef{
		Name:    "web",
		Command: "/usr/bin/sleep",
		Args:    []string{"10"},
	}
	if err := srv.Scheduler.Register(def); err != nil {
		t.Fatalf("register: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/processes", nil)
	rec := httptest.NewRecorder()

	srv.ListProcesses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var procs []processJSON
	decodeResponse(t, rec, &procs)

	if len(procs) != 1 {
		t.Fatalf("expected 1 process, got %d", len(procs))
	}
	if procs[0].Name != "web" {
		t.Errorf("expected name 'web', got %q", procs[0].Name)
	}
	if procs[0].State != "idle" {
		t.Errorf("expected state 'idle', got %q", procs[0].State)
	}
}

// ── CreateProcess ─────────────────────────────────────────────────────────────

// TestCreateProcess_Valid verifies that POST /api/processes with a valid payload
// returns 201 and the created process.
func TestCreateProcess_Valid(t *testing.T) {
	srv := newTestServer()
	body := processJSON{
		Name:    "worker",
		Command: "/usr/bin/sleep",
		Args:    []string{"5"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/processes", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.CreateProcess(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var proc processJSON
	decodeResponse(t, rec, &proc)

	if proc.Name != "worker" {
		t.Errorf("expected name 'worker', got %q", proc.Name)
	}
	if proc.State != "idle" {
		t.Errorf("expected state 'idle', got %q", proc.State)
	}
}

// TestCreateProcess_InvalidJSON verifies that POST /api/processes with malformed JSON
// returns 400.
func TestCreateProcess_InvalidJSON(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/processes", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.CreateProcess(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestCreateProcess_Duplicate verifies that registering the same name twice returns 409.
func TestCreateProcess_Duplicate(t *testing.T) {
	srv := newTestServer()
	body := processJSON{Name: "api", Command: "/bin/sh"}
	b, _ := json.Marshal(body)

	// First registration — must succeed.
	req1 := httptest.NewRequest(http.MethodPost, "/api/processes", bytes.NewReader(b))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	srv.CreateProcess(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d", rec1.Code)
	}

	// Second registration with same name — must conflict.
	req2 := httptest.NewRequest(http.MethodPost, "/api/processes", bytes.NewReader(b))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.CreateProcess(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("duplicate create: expected 409, got %d", rec2.Code)
	}
}

// TestCreateProcess_InvalidName verifies that a name violating the slug regex
// returns 422.
func TestCreateProcess_InvalidName(t *testing.T) {
	srv := newTestServer()
	invalid := []string{"-starts-with-hyphen", "UPPERCASE", "under_score", ""}
	for _, name := range invalid {
		body := processJSON{Name: name, Command: "/bin/sh"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/processes", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.CreateProcess(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("name %q: expected 422, got %d", name, rec.Code)
		}
	}
}

// TestCreateProcess_DryRun verifies that POST /api/processes with dry_run=true
// validates and returns success, but does not keep the process registered.
func TestCreateProcess_DryRun(t *testing.T) {
	srv := newTestServer()
	body := processJSON{
		Name:    "preview-worker",
		Command: "/usr/bin/sleep",
		Args:    []string{"1"},
		DryRun:  true,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/processes", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.CreateProcess(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var proc processJSON
	decodeResponse(t, rec, &proc)
	if proc.Name != "preview-worker" {
		t.Errorf("expected name 'preview-worker', got %q", proc.Name)
	}
	if proc.State != "idle" {
		t.Errorf("expected state 'idle', got %q", proc.State)
	}

	if _, err := srv.Scheduler.Snapshot("preview-worker"); !errors.Is(err, scheduler.ErrNotFound) {
		t.Fatalf("expected process to be removed after dry-run, got err=%v", err)
	}
}

// ── GetProcess ────────────────────────────────────────────────────────────────

// TestGetProcess_Found verifies that a registered process is returned with 200.
func TestGetProcess_Found(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	mustRegister(t, srv.Scheduler, "my-proc")

	rec := doRouteRequest(t, srv, http.MethodGet, "/api/processes/my-proc", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var proc processJSON
	decodeResponse(t, rec, &proc)
	if proc.Name != "my-proc" {
		t.Errorf("name = %q, want my-proc", proc.Name)
	}
	if proc.State != "idle" {
		t.Errorf("state = %q, want idle", proc.State)
	}
}

// TestGetProcess_NotFound verifies that an unknown process returns 404.
func TestGetProcess_NotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer()

	rec := doRouteRequest(t, srv, http.MethodGet, "/api/processes/no-such", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}

	// Response should contain an error message.
	var env struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error == "" {
		t.Error("expected non-empty error field")
	}
}

// ── UpdateProcess ─────────────────────────────────────────────────────────────

// TestUpdateProcess_NotFound verifies that updating a non-existent process returns 404.
func TestUpdateProcess_NotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer()

	body, _ := json.Marshal(processJSON{Command: "/bin/sh"})
	rec := doRouteRequest(t, srv, http.MethodPut, "/api/processes/ghost", body)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestUpdateProcess_Idle verifies that an idle process can be updated successfully.
func TestUpdateProcess_Idle(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	mustRegister(t, srv.Scheduler, "updateme")

	body, _ := json.Marshal(processJSON{Command: "/bin/sh", Args: []string{"-c", "echo updated"}})
	rec := doRouteRequest(t, srv, http.MethodPut, "/api/processes/updateme", body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var proc processJSON
	decodeResponse(t, rec, &proc)
	if proc.Name != "updateme" {
		t.Errorf("name = %q, want updateme", proc.Name)
	}
}

// ── DeleteProcess ─────────────────────────────────────────────────────────────

// TestDeleteProcess_Found verifies that a stopped/idle process is deleted with 200.
func TestDeleteProcess_Found(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	mustRegister(t, srv.Scheduler, "to-delete")

	rec := doRouteRequest(t, srv, http.MethodDelete, "/api/processes/to-delete", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// Confirm it no longer exists.
	rec2 := doRouteRequest(t, srv, http.MethodGet, "/api/processes/to-delete", nil)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("Get after Delete: status = %d, want 404", rec2.Code)
	}
}

// TestDeleteProcess_NotFound verifies that deleting a non-existent process returns 404.
func TestDeleteProcess_NotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer()

	rec := doRouteRequest(t, srv, http.MethodDelete, "/api/processes/ghost", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ── StartProcess ──────────────────────────────────────────────────────────────

// TestStartProcess_NotFound verifies that starting a non-existent process returns 404.
func TestStartProcess_NotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer()

	rec := doRouteRequest(t, srv, http.MethodPost, "/api/processes/ghost/start", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestStartProcess_Valid verifies that starting a valid process returns 202 Accepted.
func TestStartProcess_Valid(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	if err := srv.Scheduler.Register(scheduler.ProcessDef{
		Name:    "short-lived",
		Command: "/bin/echo",
		Args:    []string{"hi"},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	rec := doRouteRequest(t, srv, http.MethodPost, "/api/processes/short-lived/start", nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body: %s)", rec.Code, rec.Body.String())
	}

	var proc processJSON
	decodeResponse(t, rec, &proc)
	if proc.Name != "short-lived" {
		t.Errorf("name = %q, want short-lived", proc.Name)
	}
}

// TestStartProcess_AlreadyRunning verifies that starting an already-running process returns 409.
func TestStartProcess_AlreadyRunning(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	// Register a long-running process.
	if err := srv.Scheduler.Register(scheduler.ProcessDef{
		Name:    "long-proc",
		Command: "/bin/sleep",
		Args:    []string{"60"},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Start it once.
	if err := srv.Scheduler.Start("long-proc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = srv.Scheduler.Stop("long-proc") }()

	// Second start must return 409.
	rec := doRouteRequest(t, srv, http.MethodPost, "/api/processes/long-proc/start", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

// ── StopProcess ───────────────────────────────────────────────────────────────

// TestStopProcess_NotFound verifies that stopping a non-existent process returns 404.
func TestStopProcess_NotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer()

	rec := doRouteRequest(t, srv, http.MethodPost, "/api/processes/ghost/stop", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestStopProcess_NotRunning verifies that stopping an idle process returns 409.
func TestStopProcess_NotRunning(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	mustRegister(t, srv.Scheduler, "idle-stop")

	rec := doRouteRequest(t, srv, http.MethodPost, "/api/processes/idle-stop/stop", nil)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

// TestStopProcess_Running verifies that stopping a running process returns 200.
func TestStopProcess_Running(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	if err := srv.Scheduler.Register(scheduler.ProcessDef{
		Name:    "sleeper",
		Command: "/bin/sleep",
		Args:    []string{"60"},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := srv.Scheduler.Start("sleeper"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec := doRouteRequest(t, srv, http.MethodPost, "/api/processes/sleeper/stop", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// ── GetLogs ───────────────────────────────────────────────────────────────────

// TestGetLogs_NotFound verifies that logs for a non-existent process return 404.
func TestGetLogs_NotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer()

	rec := doRouteRequest(t, srv, http.MethodGet, "/api/processes/ghost/logs", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestGetLogs_EmptyBuffer verifies that a process with no log entries returns an empty
// envelope with the correct name field and an empty (not null) entries array.
func TestGetLogs_EmptyBuffer(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	mustRegister(t, srv.Scheduler, "quiet")

	rec := doRouteRequest(t, srv, http.MethodGet, "/api/processes/quiet/logs", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var envelope logsEnvelope
	decodeResponse(t, rec, &envelope)
	if envelope.Name != "quiet" {
		t.Errorf("name = %q, want quiet", envelope.Name)
	}
	if envelope.Entries == nil {
		t.Error("entries should be an empty slice, not nil")
	}
	if len(envelope.Entries) != 0 {
		t.Errorf("entries length = %d, want 0", len(envelope.Entries))
	}
}

// ── CORS middleware ───────────────────────────────────────────────────────────

// TestCORSHeaders_Present verifies that CORS headers are present on all responses.
func TestCORSHeaders_Present(t *testing.T) {
	t.Parallel()
	srv := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/processes", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods header missing")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("Access-Control-Allow-Headers header missing")
	}
}

// TestCORSPreflight_Handled verifies that OPTIONS preflight requests return 204
// with CORS headers and without hitting any route handler.
func TestCORSPreflight_Handled(t *testing.T) {
	t.Parallel()
	srv := newTestServer()

	req := httptest.NewRequest(http.MethodOptions, "/api/processes", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
}

// ── Integration (Routes) ──────────────────────────────────────────────────────

// TestRoutes_Integration verifies that the ServeMux routes requests correctly.
func TestRoutes_Integration(t *testing.T) {
	srv := newTestServer()
	handler := srv.Routes()

	// Register a process first.
	_ = srv.Scheduler.Register(scheduler.ProcessDef{Name: "svc", Command: "/bin/sh"})

	req := httptest.NewRequest(http.MethodGet, "/api/processes", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/processes via router: expected 200, got %d", rec.Code)
	}
}
