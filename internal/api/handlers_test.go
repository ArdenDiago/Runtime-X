package api

import (
	"bytes"
	"encoding/json"
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
// returns 422 (the scheduler's validateName returns a plain error, not ErrAlreadyExists).
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
