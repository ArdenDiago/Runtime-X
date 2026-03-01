package scheduler

import (
	"errors"
	"time"
)

// RestartMode defines when a process should be restarted.
type RestartMode string

const (
	RestartAlways    RestartMode = "always"
	RestartOnFailure RestartMode = "on-failure"
	RestartNever     RestartMode = "never"
)

// RestartPolicy configures automatic restart behavior for a process.
type RestartPolicy struct {
	Mode       RestartMode
	MaxRetries int
	Delay      time.Duration
}

// ProcessDef is the immutable definition of a process to be managed.
type ProcessDef struct {
	Name          string
	Command       string
	Args          []string
	Env           []string
	WorkDir       string
	RestartPolicy RestartPolicy
	DependsOn     []string
	LogBufferSize int
}

// State is the lifecycle state of a managed process.
type State int

const (
	StateIdle     State = iota
	StateStarting
	StateRunning
	StateStopping
	StateStopped
	StateFailed
)

// String returns the lowercase name of the state. STUB — returns wrong values.
func (s State) String() string {
	return "unknown"
}

// ErrInvalidTransition is returned when a state change violates the FSM.
var ErrInvalidTransition = errors.New("invalid state transition")

// validTransitions defines the allowed FSM edges.
var validTransitions = map[State][]State{}

// canTransition returns true if transitioning from -> to is allowed.
func canTransition(from, to State) bool {
	return false
}

// transition validates and applies a state change. STUB — always returns error.
func transition(mp *ManagedProcess, to State) error {
	return ErrInvalidTransition
}

// ManagedProcess is the runtime representation of a registered process.
type ManagedProcess struct {
	Def          ProcessDef
	State        State
	StartedAt    time.Time
	StoppedAt    time.Time
	ExitCode     int
	RestartCount int
	logs         *logBuffer
}
