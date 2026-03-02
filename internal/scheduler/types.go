package scheduler

import (
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// RestartMode defines when a process should be restarted.
type RestartMode string

const (
	// RestartAlways restarts the process regardless of exit status.
	RestartAlways RestartMode = "always"
	// RestartOnFailure restarts the process only when it exits with a non-zero code.
	RestartOnFailure RestartMode = "on-failure"
	// RestartNever disables automatic restarts.
	RestartNever RestartMode = "never"
)

// RestartPolicy configures automatic restart behavior for a process.
type RestartPolicy struct {
	// Mode determines when to restart.
	Mode RestartMode
	// MaxRetries limits the number of restart attempts (0 = unlimited when Mode is RestartAlways).
	MaxRetries int
	// Delay is the pause between restart attempts.
	Delay time.Duration
}

// ProcessDef is the immutable definition of a process to be managed.
// All fields are set at registration time; the shape is locked for Phase 5.
type ProcessDef struct {
	// Name is the unique slug identifier for the process.
	// Must match ^[a-z0-9][a-z0-9-]*$ (lowercase, alphanumeric, hyphens, no leading hyphen).
	Name string
	// Command is the executable path to run.
	Command string
	// Args are the command-line arguments passed to Command.
	Args []string
	// Env is a list of KEY=VALUE environment variable pairs.
	// An empty slice causes the child process to inherit the parent environment.
	Env []string
	// WorkDir is the working directory for the process.
	// An empty string means inherit the working directory from the parent process.
	WorkDir string
	// RestartPolicy configures automatic restart behaviour after the process exits.
	RestartPolicy RestartPolicy
	// DependsOn lists process names that must be running before this process starts.
	// This field is stored but ignored until Phase 7 wires up dependency ordering.
	DependsOn []string
	// LogBufferSize is the number of log lines to retain per process.
	// A value <= 0 is replaced with the default of 1000 at registration time.
	LogBufferSize int
	// StopTimeout is how long Stop() waits after SIGTERM before escalating to SIGKILL.
	// A zero value uses the scheduler default (5 seconds).
	StopTimeout time.Duration
}

// State is the lifecycle state of a managed process.
type State int

const (
	// StateIdle is the initial state after registration — process has never started.
	StateIdle State = iota
	// StateStarting is the transient state while cmd.Start() is being called.
	StateStarting
	// StateRunning means the process is alive and its PID is valid.
	StateRunning
	// StateStopping means a stop signal has been sent and we are waiting for exit.
	StateStopping
	// StateStopped means the process exited and was explicitly stopped.
	StateStopped
	// StateFailed means the process exited with a non-zero code or crashed.
	StateFailed
)

// String returns the lowercase name of the state for display and error messages.
func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ErrInvalidTransition is returned when a requested state change is not permitted
// by the FSM transition table.
var ErrInvalidTransition = errors.New("invalid state transition")

// validTransitions maps each state to the set of states it may transition into.
// Transitions not listed here are forbidden.
var validTransitions = map[State][]State{
	StateIdle:     {StateStarting},
	StateStarting: {StateRunning, StateFailed},
	StateRunning:  {StateStopping, StateFailed},
	StateStopping: {StateStopped, StateFailed},
	StateStopped:  {StateStarting}, // allows restart
	StateFailed:   {StateStarting}, // allows retry
}

// canTransition reports whether the FSM permits moving from → to.
func canTransition(from, to State) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// transition validates the from→to edge and, on success, updates mp.State.
// Returns an error wrapping ErrInvalidTransition with from/to context if the
// transition is not allowed. Called by Scheduler methods that hold the write
// lock; tests call it directly since they are in the same package.
func transition(mp *ManagedProcess, to State) error {
	from := mp.State
	if !canTransition(from, to) {
		return fmt.Errorf("transition %s → %s: %w", from, to, ErrInvalidTransition)
	}
	mp.State = to
	return nil
}

// ManagedProcess is the runtime representation of a registered process,
// combining its immutable definition with mutable lifecycle state.
type ManagedProcess struct {
	// Def is the immutable process definition supplied at registration.
	Def ProcessDef
	// State is the current lifecycle state.
	State State
	// StartedAt records when the most recent start occurred.
	StartedAt time.Time
	// StoppedAt records when the most recent stop or failure occurred.
	StoppedAt time.Time
	// ExitCode is the exit code from the most recent run (0 until set by Phase 6).
	ExitCode int
	// RestartCount is the number of times the process has been automatically restarted.
	RestartCount int
	// logs is the per-process ring buffer; unexported — accessed via Scheduler.Logs().
	logs *logBuffer

	// Phase 6 runtime fields — zeroed between restarts.
	// cmd is the active exec.Cmd; nil when not running. Set by Start(), read by Stop().
	cmd *exec.Cmd
	// doneCh is nil unless Stop() is pending; closed by the monitor goroutine on exit.
	doneCh chan struct{}
}
