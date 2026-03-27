package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"runtimex/internal/scheduler"
)

// restartPolicyJSON is the JSON representation of a RestartPolicy.
// Durations are expressed in seconds (float64) for API compatibility.
type restartPolicyJSON struct {
	Mode          string  `json:"mode"`
	MaxRetries    int     `json:"max_retries"`
	DelaySecs     float64 `json:"delay_secs"`
	MaxDelaySecs  float64 `json:"max_delay_secs"`
	BackoffFactor float64 `json:"backoff_factor"`
}

// processJSON is the JSON representation of a ProcessDef / ManagedProcess.
// Durations are expressed in seconds (float64) for API compatibility.
type processJSON struct {
	Name            string            `json:"name"`
	Command         string            `json:"command"`
	Args            []string          `json:"args,omitempty"`
	Env             []string          `json:"env,omitempty"`
	WorkDir         string            `json:"work_dir,omitempty"`
	DryRun          bool              `json:"dry_run,omitempty"`
	RestartPolicy   restartPolicyJSON `json:"restart_policy"`
	DependsOn       []string          `json:"depends_on,omitempty"`
	LogBufferSize   int               `json:"log_buffer_size,omitempty"`
	StopTimeoutSecs float64           `json:"stop_timeout_secs,omitempty"`

	// Runtime fields — present in GET responses only.
	State        string `json:"state,omitempty"`
	RestartCount int    `json:"restart_count,omitempty"`
}

// snapshotToJSON converts a ProcessSnapshot (safe value copy) into its JSON
// representation. Using a snapshot avoids data races with the monitor goroutine
// that may concurrently write to the live ManagedProcess after scheduler methods
// like Start() return.
func snapshotToJSON(snap scheduler.ProcessSnapshot) processJSON {
	def := snap.Def
	return processJSON{
		Name:    def.Name,
		Command: def.Command,
		Args:    def.Args,
		Env:     def.Env,
		WorkDir: def.WorkDir,
		RestartPolicy: restartPolicyJSON{
			Mode:          string(def.RestartPolicy.Mode),
			MaxRetries:    def.RestartPolicy.MaxRetries,
			DelaySecs:     def.RestartPolicy.Delay.Seconds(),
			MaxDelaySecs:  def.RestartPolicy.MaxDelay.Seconds(),
			BackoffFactor: def.RestartPolicy.BackoffFactor,
		},
		DependsOn:       def.DependsOn,
		LogBufferSize:   def.LogBufferSize,
		StopTimeoutSecs: def.StopTimeout.Seconds(),
		State:           snap.State.String(),
		RestartCount:    snap.RestartCount,
	}
}

// fromProcessJSON converts a processJSON body into a ProcessDef.
func fromProcessJSON(p processJSON) scheduler.ProcessDef {
	return scheduler.ProcessDef{
		Name:    p.Name,
		Command: p.Command,
		Args:    p.Args,
		Env:     p.Env,
		WorkDir: p.WorkDir,
		RestartPolicy: scheduler.RestartPolicy{
			Mode:          scheduler.RestartMode(p.RestartPolicy.Mode),
			MaxRetries:    p.RestartPolicy.MaxRetries,
			Delay:         time.Duration(p.RestartPolicy.DelaySecs * float64(time.Second)),
			MaxDelay:      time.Duration(p.RestartPolicy.MaxDelaySecs * float64(time.Second)),
			BackoffFactor: p.RestartPolicy.BackoffFactor,
		},
		DependsOn:     p.DependsOn,
		LogBufferSize: p.LogBufferSize,
		StopTimeout:   time.Duration(p.StopTimeoutSecs * float64(time.Second)),
	}
}

// ListProcesses handles GET /api/processes.
// Returns a JSON array of all registered processes with their current state.
// Uses SnapshotAll() to ensure race-safe reads of live state fields.
func (s *Server) ListProcesses(w http.ResponseWriter, r *http.Request) {
	snaps := s.Scheduler.SnapshotAll()
	out := make([]processJSON, 0, len(snaps))
	for _, snap := range snaps {
		out = append(out, snapshotToJSON(snap))
	}
	send(w, http.StatusOK, out, nil)
}

// CreateProcess handles POST /api/processes.
// Decodes a processJSON body, validates it, and registers it with the scheduler.
func (s *Server) CreateProcess(w http.ResponseWriter, r *http.Request) {
	var body processJSON
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		send(w, http.StatusBadRequest, nil, errors.New("invalid JSON body"))
		return
	}

	def := fromProcessJSON(body)
	if err := s.Scheduler.Register(def); err != nil {
		// ErrAlreadyExists → 409 Conflict; validation errors → 422.
		if errors.Is(err, scheduler.ErrAlreadyExists) {
			send(w, http.StatusConflict, nil, err)
			return
		}
		send(w, http.StatusUnprocessableEntity, nil, err)
		return
	}

	snap, _ := s.Scheduler.Snapshot(def.Name)
	if body.DryRun {
		if err := s.Scheduler.Remove(def.Name); err != nil {
			send(w, http.StatusInternalServerError, nil, err)
			return
		}
	}
	send(w, http.StatusCreated, snapshotToJSON(snap), nil)
}

// GetProcess handles GET /api/processes/{name}.
// Returns the process definition and current runtime state, or 404 if not found.
func (s *Server) GetProcess(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	snap, err := s.Scheduler.Snapshot(name)
	if err != nil {
		if errors.Is(err, scheduler.ErrNotFound) {
			send(w, http.StatusNotFound, nil, err)
			return
		}
		send(w, http.StatusInternalServerError, nil, err)
		return
	}
	send(w, http.StatusOK, snapshotToJSON(snap), nil)
}

// UpdateProcess handles PUT /api/processes/{name}.
// Replaces the process definition. The process must be stopped before updating.
// Returns 404 if not found, 409 if the process is not stopped.
func (s *Server) UpdateProcess(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Verify the process exists and is in a stoppable state.
	snap, err := s.Scheduler.Snapshot(name)
	if err != nil {
		if errors.Is(err, scheduler.ErrNotFound) {
			send(w, http.StatusNotFound, nil, err)
			return
		}
		send(w, http.StatusInternalServerError, nil, err)
		return
	}

	// Process must be stopped (Idle, Stopped, or Failed) before updating.
	switch snap.State {
	case scheduler.StateIdle, scheduler.StateStopped, scheduler.StateFailed:
		// allowed — proceed
	default:
		send(w, http.StatusConflict, nil, errors.New("process must be stopped before updating"))
		return
	}

	var body processJSON
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		send(w, http.StatusBadRequest, nil, errors.New("invalid JSON body"))
		return
	}

	// Force the name from the URL path — body name is ignored.
	body.Name = name
	newDef := fromProcessJSON(body)

	// Remove the old registration and re-register with the new definition.
	if err := s.Scheduler.Remove(name); err != nil {
		send(w, http.StatusInternalServerError, nil, err)
		return
	}
	if err := s.Scheduler.Register(newDef); err != nil {
		send(w, http.StatusUnprocessableEntity, nil, err)
		return
	}

	updated, _ := s.Scheduler.Snapshot(name)
	send(w, http.StatusOK, snapshotToJSON(updated), nil)
}

// DeleteProcess handles DELETE /api/processes/{name}.
// Removes the process from the scheduler. The process must be stopped first.
// Returns 404 if not found, 409 if the process is still running.
func (s *Server) DeleteProcess(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.Scheduler.Remove(name); err != nil {
		if errors.Is(err, scheduler.ErrNotFound) {
			send(w, http.StatusNotFound, nil, err)
			return
		}
		if errors.Is(err, scheduler.ErrNotStopped) {
			send(w, http.StatusConflict, nil, err)
			return
		}
		send(w, http.StatusInternalServerError, nil, err)
		return
	}
	send(w, http.StatusOK, map[string]string{"message": "process deleted"}, nil)
}

// StartProcess handles POST /api/processes/{name}/start.
// Starts the named process. Returns 404 if not found, 409 if already running.
// Uses Snapshot() after Start() for a race-safe state read.
func (s *Server) StartProcess(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.Scheduler.Start(name); err != nil {
		if errors.Is(err, scheduler.ErrNotFound) {
			send(w, http.StatusNotFound, nil, err)
			return
		}
		if errors.Is(err, scheduler.ErrAlreadyRunning) {
			send(w, http.StatusConflict, nil, err)
			return
		}
		send(w, http.StatusInternalServerError, nil, err)
		return
	}
	snap, _ := s.Scheduler.Snapshot(name)
	send(w, http.StatusAccepted, snapshotToJSON(snap), nil)
}

// StopProcess handles POST /api/processes/{name}/stop.
// Stops the named process. Returns 404 if not found, 409 if already stopped.
// Uses Snapshot() after Stop() for a race-safe state read.
func (s *Server) StopProcess(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.Scheduler.Stop(name); err != nil {
		if errors.Is(err, scheduler.ErrNotFound) {
			send(w, http.StatusNotFound, nil, err)
			return
		}
		if errors.Is(err, scheduler.ErrNotRunning) {
			send(w, http.StatusConflict, nil, err)
			return
		}
		send(w, http.StatusInternalServerError, nil, err)
		return
	}
	snap, _ := s.Scheduler.Snapshot(name)
	send(w, http.StatusOK, snapshotToJSON(snap), nil)
}

// logEntryJSON is the JSON representation of a scheduler.LogEntry.
type logEntryJSON struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"`
	Text      string `json:"text"`
}

// logsEnvelope wraps the log entries and metadata in a structured response.
type logsEnvelope struct {
	Name    string         `json:"name"`
	Entries []logEntryJSON `json:"entries"`
}

// GetLogs handles GET /api/processes/{name}/logs.
// Returns a structured envelope with log entries. Returns 404 if not found.
func (s *Server) GetLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	lines, err := s.Scheduler.Logs(name)
	if err != nil {
		if errors.Is(err, scheduler.ErrNotFound) {
			send(w, http.StatusNotFound, nil, err)
			return
		}
		send(w, http.StatusInternalServerError, nil, err)
		return
	}

	entries := make([]logEntryJSON, 0, len(lines))
	for _, line := range lines {
		entries = append(entries, logEntryJSON{
			Timestamp: line.Timestamp.UTC().Format(time.RFC3339Nano),
			Stream:    string(line.Stream),
			Text:      line.Text,
		})
	}

	send(w, http.StatusOK, logsEnvelope{Name: name, Entries: entries}, nil)
}
