package scheduler

import (
	"errors"
	"sync"
)

var validName = errors.New("not implemented")

var (
	ErrNotFound     = errors.New("process not found")
	ErrAlreadyExists = errors.New("process already exists")
	ErrNotStopped   = errors.New("process is not stopped")
)

// Scheduler manages the registry of process definitions and their runtime state.
type Scheduler struct {
	mu        sync.RWMutex
	processes map[string]*ManagedProcess
}

// New returns a ready-to-use Scheduler. STUB.
func New() *Scheduler {
	return &Scheduler{
		processes: make(map[string]*ManagedProcess),
	}
}

// Register STUB — always returns nil (tests will fail on invalid name checks).
func (s *Scheduler) Register(def ProcessDef) error {
	return nil
}

// Remove STUB — always returns nil.
func (s *Scheduler) Remove(name string) error {
	return nil
}

// Get STUB — always returns ErrNotFound.
func (s *Scheduler) Get(name string) (*ManagedProcess, error) {
	return nil, ErrNotFound
}

// List STUB — always returns empty slice.
func (s *Scheduler) List() []*ManagedProcess {
	return nil
}

// Logs STUB — always returns ErrNotFound.
func (s *Scheduler) Logs(name string) ([]LogEntry, error) {
	return nil, ErrNotFound
}

func validateName(name string) error {
	return nil
}
