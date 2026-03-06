package scheduler

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

// validName matches the required process name format: a lowercase alphanumeric
// character followed by zero or more lowercase alphanumeric characters or hyphens.
// Valid: "my-app", "web1", "a". Invalid: "", "-foo", "FOO", "under_score".
var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Sentinel errors returned by Scheduler methods. Callers should use errors.Is()
// to check for these, not string comparison, since the messages include context.
var (
	// ErrNotFound is returned when a process name is not registered.
	ErrNotFound = errors.New("process not found")
	// ErrAlreadyExists is returned when Register is called with a name already in use.
	ErrAlreadyExists = errors.New("process already exists")
	// ErrNotStopped is returned when Remove is called on a running or starting process.
	ErrNotStopped = errors.New("process is not stopped")
	// ErrAlreadyRunning is returned when Start is called on a process that is already running.
	ErrAlreadyRunning = errors.New("process is already running")
	// ErrNotRunning is returned when Stop is called on a process that is not running.
	ErrNotRunning = errors.New("process is not running")
)

// Scheduler manages the registry of process definitions and their runtime state.
// Create one with New(). All methods are safe for concurrent use from multiple
// goroutines. The Scheduler follows the http.NewServeMux() instantiation pattern:
// it holds no global state and can be constructed, passed, and tested freely.
type Scheduler struct {
	mu        sync.RWMutex
	processes map[string]*ManagedProcess
}

// New returns a ready-to-use Scheduler with an empty process registry.
func New() *Scheduler {
	return &Scheduler{
		processes: make(map[string]*ManagedProcess),
	}
}

// Register adds a new process definition to the scheduler.
//
// The process name must satisfy ^[a-z0-9][a-z0-9-]*$ (lowercase slug format).
// If LogBufferSize is <= 0 it is defaulted to 1000.
// Returns a descriptive error if the name is invalid.
// Returns fmt.Errorf("%w: %s", ErrAlreadyExists, name) if the name is already registered.
func (s *Scheduler) Register(def ProcessDef) error {
	if err := validateName(def.Name); err != nil {
		return err
	}
	if def.LogBufferSize <= 0 {
		def.LogBufferSize = 1000
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.processes[def.Name]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, def.Name)
	}

	// Phase 7: validate dependency edges before accepting the definition.
	// topoCheck reads s.processes directly — safe because we hold the write lock.
	if len(def.DependsOn) > 0 {
		if err := topoCheck(s.processes, def); err != nil {
			return err
		}
	}

	s.processes[def.Name] = &ManagedProcess{
		Def:   def,
		State: StateIdle,
		logs:  newLogBuffer(def.LogBufferSize),
	}
	return nil
}

// Remove unregisters a process from the scheduler.
//
// The process must be in the Stopped, Idle, or Failed state. Attempting to
// remove a process in any other state returns ErrNotStopped. Returns ErrNotFound
// if no process with the given name is registered.
func (s *Scheduler) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mp, exists := s.processes[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	switch mp.State {
	case StateStopped, StateIdle, StateFailed:
		// allowed — fall through to delete
	default:
		return fmt.Errorf("%w: %s (state: %s)", ErrNotStopped, name, mp.State)
	}

	delete(s.processes, name)
	return nil
}

// Get returns the ManagedProcess for the given name.
//
// The returned pointer is the live object stored in the scheduler.
// Phase 6 and later phases should not mutate it without holding the scheduler
// lock; use the unexported transition() helper when mutation is needed.
// Returns ErrNotFound if no process with the given name is registered.
func (s *Scheduler) Get(name string) (*ManagedProcess, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mp, exists := s.processes[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return mp, nil
}

// List returns a snapshot slice of all registered ManagedProcess pointers.
//
// The slice itself is a fresh allocation on each call; the pointers within it
// reference the live objects in the scheduler. The order of entries is
// non-deterministic (map iteration). Callers must not modify the pointed-to
// objects without holding appropriate locks.
func (s *Scheduler) List() []*ManagedProcess {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*ManagedProcess, 0, len(s.processes))
	for _, mp := range s.processes {
		out = append(out, mp)
	}
	return out
}

// Logs returns a point-in-time snapshot of all log entries for the named process.
//
// The scheduler read lock is released before accessing the log buffer so that
// Phase 6 log-writing goroutines never contend with the scheduler lock.
// Returns nil (not an error) if the process has no log entries yet.
// Returns ErrNotFound if no process with the given name is registered.
func (s *Scheduler) Logs(name string) ([]LogEntry, error) {
	// Acquire the read lock only long enough to locate the ManagedProcess.
	s.mu.RLock()
	mp, exists := s.processes[name]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	// The log buffer has its own independent mutex, so this call is safe
	// without holding the scheduler lock.
	return mp.logs.Lines(), nil
}

// ProcessSnapshot is a point-in-time copy of a ManagedProcess's observable fields.
// All fields are value types — no pointers into the live scheduler state — so the
// snapshot is safe to read after the scheduler lock has been released.
type ProcessSnapshot struct {
	Def          ProcessDef
	State        State
	StartedAt    time.Time
	StoppedAt    time.Time
	ExitCode     int
	RestartCount int
}

// Snapshot returns a race-safe copy of the named process's state under the read
// lock. Unlike Get(), the returned struct contains no pointers into the live
// scheduler state, so callers may read it after the lock is released without
// data races with the monitor goroutine.
//
// Returns ErrNotFound if no process with the given name is registered.
func (s *Scheduler) Snapshot(name string) (ProcessSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mp, exists := s.processes[name]
	if !exists {
		return ProcessSnapshot{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return ProcessSnapshot{
		Def:          mp.Def,
		State:        mp.State,
		StartedAt:    mp.StartedAt,
		StoppedAt:    mp.StoppedAt,
		ExitCode:     mp.ExitCode,
		RestartCount: mp.RestartCount,
	}, nil
}

// SnapshotAll returns a point-in-time slice of ProcessSnapshot for every
// registered process. Like Snapshot(), all values are safe to read without
// holding the lock after this call returns.
func (s *Scheduler) SnapshotAll() []ProcessSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]ProcessSnapshot, 0, len(s.processes))
	for _, mp := range s.processes {
		out = append(out, ProcessSnapshot{
			Def:          mp.Def,
			State:        mp.State,
			StartedAt:    mp.StartedAt,
			StoppedAt:    mp.StoppedAt,
			ExitCode:     mp.ExitCode,
			RestartCount: mp.RestartCount,
		})
	}
	return out
}

// validateName returns a descriptive error if name does not satisfy the slug format.
func validateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid process name %q: must match ^[a-z0-9][a-z0-9-]*$", name)
	}
	return nil
}
