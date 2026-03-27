package scheduler

import (
	"errors"
	"sync"
	"testing"
)

// TestSchedulerRegister covers the Register method: name validation,
// uniqueness enforcement, default LogBufferSize, and happy-path storage.
func TestSchedulerRegister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		def     ProcessDef
		wantErr bool // true means expect a non-nil error
	}{
		// --- valid names ---
		{
			name:    "valid name my-app",
			def:     ProcessDef{Name: "my-app", Command: "/bin/sh"},
			wantErr: false,
		},
		{
			name:    "valid name web1",
			def:     ProcessDef{Name: "web1", Command: "/bin/sh"},
			wantErr: false,
		},
		{
			name:    "valid single char name a",
			def:     ProcessDef{Name: "a", Command: "/bin/sh"},
			wantErr: false,
		},
		// --- invalid names ---
		{
			name:    "empty name is rejected",
			def:     ProcessDef{Name: "", Command: "/bin/sh"},
			wantErr: true,
		},
		{
			name:    "leading hyphen is rejected",
			def:     ProcessDef{Name: "-foo", Command: "/bin/sh"},
			wantErr: true,
		},
		{
			name:    "uppercase is rejected",
			def:     ProcessDef{Name: "FOO", Command: "/bin/sh"},
			wantErr: true,
		},
		{
			name:    "underscore is rejected",
			def:     ProcessDef{Name: "under_score", Command: "/bin/sh"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := New()
			err := s.Register(tc.def)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Register(%q) expected error, got nil", tc.def.Name)
				}
				return
			}
			if err != nil {
				t.Fatalf("Register(%q) unexpected error: %v", tc.def.Name, err)
			}
			// Verify the process is retrievable and in StateIdle
			mp, getErr := s.Get(tc.def.Name)
			if getErr != nil {
				t.Fatalf("Get(%q) after Register: %v", tc.def.Name, getErr)
			}
			if mp.State != StateIdle {
				t.Errorf("after Register, State = %v, want StateIdle", mp.State)
			}
		})
	}
}

// TestSchedulerRegisterDuplicate verifies that registering the same name twice
// returns ErrAlreadyExists.
func TestSchedulerRegisterDuplicate(t *testing.T) {
	t.Parallel()

	s := New()
	def := ProcessDef{Name: "worker", Command: "/bin/sh"}

	if err := s.Register(def); err != nil {
		t.Fatalf("first Register: unexpected error: %v", err)
	}
	err := s.Register(def)
	if err == nil {
		t.Fatal("second Register: expected ErrAlreadyExists, got nil")
	}
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("second Register: got %v, want wrapping ErrAlreadyExists", err)
	}
}

// TestSchedulerRegisterEnvValidation verifies KEY=VALUE validation for Env entries.
func TestSchedulerRegisterEnvValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid env entries are accepted", func(t *testing.T) {
		t.Parallel()
		s := New()
		err := s.Register(ProcessDef{
			Name:    "env-ok",
			Command: "/bin/sh",
			Env:     []string{"A=1", "B=two=parts", "EMPTY="},
		})
		if err != nil {
			t.Fatalf("Register valid env: unexpected error: %v", err)
		}
	})

	t.Run("missing equals is rejected", func(t *testing.T) {
		t.Parallel()
		s := New()
		err := s.Register(ProcessDef{
			Name:    "env-no-equals",
			Command: "/bin/sh",
			Env:     []string{"NOVALUE"},
		})
		if err == nil {
			t.Fatal("Register missing equals: expected error, got nil")
		}
	})

	t.Run("empty key is rejected", func(t *testing.T) {
		t.Parallel()
		s := New()
		err := s.Register(ProcessDef{
			Name:    "env-empty-key",
			Command: "/bin/sh",
			Env:     []string{"=value"},
		})
		if err == nil {
			t.Fatal("Register empty key: expected error, got nil")
		}
	})
}

// TestSchedulerRegisterDefaultLogBufferSize verifies that LogBufferSize <= 0
// is defaulted to 1000 and the process is registered successfully.
func TestSchedulerRegisterDefaultLogBufferSize(t *testing.T) {
	t.Parallel()

	s := New()
	def := ProcessDef{Name: "app", Command: "/bin/sh", LogBufferSize: 0}
	if err := s.Register(def); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Logs should return nil (empty buffer, no writes yet)
	lines, err := s.Logs("app")
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if lines != nil {
		t.Errorf("Logs() = %v, want nil for empty buffer", lines)
	}

	// Verify LogBufferSize was defaulted inside the stored def
	mp, _ := s.Get("app")
	if mp.Def.LogBufferSize != 1000 {
		t.Errorf("LogBufferSize = %d, want 1000", mp.Def.LogBufferSize)
	}
}

// TestSchedulerRemove covers the Remove method: happy path, not-found,
// and the not-stopped guard.
func TestSchedulerRemove(t *testing.T) {
	t.Parallel()

	t.Run("remove idle process succeeds", func(t *testing.T) {
		t.Parallel()
		s := New()
		if err := s.Register(ProcessDef{Name: "idle-proc", Command: "/bin/sh"}); err != nil {
			t.Fatal(err)
		}
		if err := s.Remove("idle-proc"); err != nil {
			t.Fatalf("Remove: unexpected error: %v", err)
		}
		_, err := s.Get("idle-proc")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Get after Remove: got %v, want ErrNotFound", err)
		}
	})

	t.Run("remove nonexistent returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		s := New()
		err := s.Remove("no-such-proc")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Remove nonexistent: got %v, want ErrNotFound", err)
		}
	})

	t.Run("remove running process returns ErrNotStopped", func(t *testing.T) {
		t.Parallel()
		s := New()
		if err := s.Register(ProcessDef{Name: "runner", Command: "/bin/sh"}); err != nil {
			t.Fatal(err)
		}
		// Manually advance state to Running (tests are in same package)
		mp, _ := s.Get("runner")
		if err := transition(mp, StateStarting); err != nil {
			t.Fatalf("transition Idle->Starting: %v", err)
		}
		if err := transition(mp, StateRunning); err != nil {
			t.Fatalf("transition Starting->Running: %v", err)
		}

		err := s.Remove("runner")
		if !errors.Is(err, ErrNotStopped) {
			t.Errorf("Remove running: got %v, want ErrNotStopped", err)
		}
	})

	t.Run("remove stopped process succeeds", func(t *testing.T) {
		t.Parallel()
		s := New()
		if err := s.Register(ProcessDef{Name: "done", Command: "/bin/sh"}); err != nil {
			t.Fatal(err)
		}
		mp, _ := s.Get("done")
		_ = transition(mp, StateStarting)
		_ = transition(mp, StateRunning)
		_ = transition(mp, StateStopping)
		_ = transition(mp, StateStopped)

		if err := s.Remove("done"); err != nil {
			t.Fatalf("Remove stopped: %v", err)
		}
	})

	t.Run("remove failed process succeeds", func(t *testing.T) {
		t.Parallel()
		s := New()
		if err := s.Register(ProcessDef{Name: "crashed", Command: "/bin/sh"}); err != nil {
			t.Fatal(err)
		}
		mp, _ := s.Get("crashed")
		_ = transition(mp, StateStarting)
		_ = transition(mp, StateFailed)

		if err := s.Remove("crashed"); err != nil {
			t.Fatalf("Remove failed: %v", err)
		}
	})
}

// TestSchedulerList verifies the List method returns all registered processes.
func TestSchedulerList(t *testing.T) {
	t.Parallel()

	t.Run("empty scheduler returns empty slice", func(t *testing.T) {
		t.Parallel()
		s := New()
		list := s.List()
		if len(list) != 0 {
			t.Errorf("List() = %d entries, want 0", len(list))
		}
	})

	t.Run("three registrations returns three entries", func(t *testing.T) {
		t.Parallel()
		s := New()
		for _, name := range []string{"alpha", "beta", "gamma"} {
			if err := s.Register(ProcessDef{Name: name, Command: "/bin/sh"}); err != nil {
				t.Fatalf("Register(%q): %v", name, err)
			}
		}
		list := s.List()
		if len(list) != 3 {
			t.Errorf("List() = %d entries, want 3", len(list))
		}
	})
}

// TestSchedulerLogs covers the Logs method: empty buffer and not-found.
func TestSchedulerLogs(t *testing.T) {
	t.Parallel()

	t.Run("logs on registered process with no writes returns nil", func(t *testing.T) {
		t.Parallel()
		s := New()
		if err := s.Register(ProcessDef{Name: "quiet", Command: "/bin/sh"}); err != nil {
			t.Fatal(err)
		}
		lines, err := s.Logs("quiet")
		if err != nil {
			t.Fatalf("Logs: unexpected error: %v", err)
		}
		if lines != nil {
			t.Errorf("Logs() = %v, want nil for empty buffer", lines)
		}
	})

	t.Run("logs on nonexistent returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		s := New()
		_, err := s.Logs("no-such")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Logs nonexistent: got %v, want ErrNotFound", err)
		}
	})
}

// TestStateTransitions verifies the FSM via canTransition and transition().
func TestStateTransitions(t *testing.T) {
	t.Parallel()

	validCases := []struct {
		from State
		to   State
	}{
		{StateIdle, StateStarting},
		{StateStarting, StateRunning},
		{StateStarting, StateFailed},
		{StateRunning, StateStopping},
		{StateRunning, StateStopped},     // natural clean exit (exit code 0)
		{StateRunning, StateFailed},      // crash
		{StateRunning, StateRestarting},  // restart policy triggers backoff
		{StateRestarting, StateStarting}, // backoff elapsed — spawn next attempt
		{StateRestarting, StateStopping}, // Stop() interrupts pending restart
		{StateRestarting, StateFailed},   // MaxRetries exhausted
		{StateStopping, StateStopped},
		{StateStopping, StateFailed},
		{StateStopped, StateStarting},
		{StateFailed, StateStarting},
	}

	invalidCases := []struct {
		from State
		to   State
	}{
		{StateIdle, StateRunning},
		{StateIdle, StateStopped},
		{StateIdle, StateFailed},
		{StateRunning, StateIdle},
		{StateRunning, StateStarting},
		{StateStopped, StateIdle},
		{StateStopped, StateFailed},
		{StateRestarting, StateIdle},    // no jumping back to idle
		{StateRestarting, StateRunning}, // must go through Starting first
		{StateRestarting, StateStopped}, // cannot transition directly to Stopped
	}

	t.Run("valid transitions succeed", func(t *testing.T) {
		t.Parallel()
		for _, tc := range validCases {
			tc := tc
			t.Run(tc.from.String()+"->"+tc.to.String(), func(t *testing.T) {
				t.Parallel()
				mp := &ManagedProcess{State: tc.from}
				if err := transition(mp, tc.to); err != nil {
					t.Errorf("transition(%s -> %s): unexpected error: %v", tc.from, tc.to, err)
				}
				if mp.State != tc.to {
					t.Errorf("after transition, State = %v, want %v", mp.State, tc.to)
				}
			})
		}
	})

	t.Run("invalid transitions return ErrInvalidTransition", func(t *testing.T) {
		t.Parallel()
		for _, tc := range invalidCases {
			tc := tc
			t.Run(tc.from.String()+"->"+tc.to.String(), func(t *testing.T) {
				t.Parallel()
				mp := &ManagedProcess{State: tc.from}
				err := transition(mp, tc.to)
				if err == nil {
					t.Errorf("transition(%s -> %s): expected error, got nil", tc.from, tc.to)
					return
				}
				if !errors.Is(err, ErrInvalidTransition) {
					t.Errorf("transition(%s -> %s): got %v, want wrapping ErrInvalidTransition", tc.from, tc.to, err)
				}
				// State must not have changed on invalid transition
				if mp.State != tc.from {
					t.Errorf("State changed to %v on invalid transition, want %v", mp.State, tc.from)
				}
			})
		}
	})
}

// TestStateString verifies the String() method for all State values.
func TestStateString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state State
		want  string
	}{
		{StateIdle, "idle"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateStopping, "stopping"},
		{StateStopped, "stopped"},
		{StateFailed, "failed"},
		{StateRestarting, "restarting"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := tc.state.String()
			if got != tc.want {
				t.Errorf("State(%d).String() = %q, want %q", int(tc.state), got, tc.want)
			}
		})
	}
}

// TestSchedulerConcurrentAccess verifies that concurrent Register/Get/List
// calls produce no data races (run with go test -race).
func TestSchedulerConcurrentAccess(t *testing.T) {
	s := New()
	const goroutines = 20

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine registers one unique process then reads concurrently.
			name := procName(i)
			_ = s.Register(ProcessDef{Name: name, Command: "/bin/sh"})
			// Concurrent reads
			for j := 0; j < 10; j++ {
				s.List()
				s.Get(name)
			}
		}()
	}
	wg.Wait()
}

// procName builds a valid process name from an integer index, e.g. 0->"p0", 15->"p15".
func procName(i int) string {
	const digits = "0123456789"
	if i < 10 {
		return "p" + string(digits[i])
	}
	return "p" + string(digits[i/10]) + string(digits[i%10])
}
