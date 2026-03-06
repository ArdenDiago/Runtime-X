package scheduler

import (
	"math"
	"time"
)

// waitAndRestart performs an exponential-backoff sleep and then restarts the
// named process. It is launched as a goroutine by monitorProcess when the
// restart policy calls for another attempt.
//
// If mp.restartCancelCh is closed (by a concurrent Stop() call) before the
// timer fires, the goroutine exits without starting the process.
//
// The delay for restart attempt N is:
//
//	delay = Delay * (Factor ^ (RestartCount - 1))
//
// capped to MaxDelay when MaxDelay > 0. RestartCount has already been
// incremented by monitorProcess before this goroutine is launched.
func waitAndRestart(s *Scheduler, mp *ManagedProcess) {
	// Capture immutable policy values and volatile runtime fields under the lock.
	s.mu.RLock()
	policy := mp.Def.RestartPolicy
	restartCount := mp.RestartCount
	cancelCh := mp.restartCancelCh
	s.mu.RUnlock()

	// Compute the backoff delay for this attempt.
	delay := calcDelay(policy, restartCount)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Backoff elapsed — attempt the restart.
		// Start() handles all locking, state transitions, and error reporting
		// internally. Ignore the error: if Start() fails (e.g. command not found),
		// monitorProcess will be called again and will handle the failure state.
		_ = s.Start(mp.Def.Name)
	case <-cancelCh:
		// Stop() closed the channel to cancel the pending restart.
		// The state has already been transitioned to Stopping/Stopped by Stop().
		return
	}
}

// calcDelay computes the backoff delay for the Nth restart attempt.
// restartCount is the 1-based attempt number (already incremented by caller).
//
// formula: delay = Delay * (factor ^ (restartCount - 1))
// capped at MaxDelay when MaxDelay > 0.
// When Delay == 0, the result is always 0 (immediate retry).
func calcDelay(policy RestartPolicy, restartCount int) time.Duration {
	if policy.Delay <= 0 {
		return 0
	}

	factor := policy.BackoffFactor
	if factor <= 0 {
		factor = 2.0
	}

	exponent := restartCount - 1
	if exponent < 0 {
		exponent = 0
	}

	delay := float64(policy.Delay) * math.Pow(factor, float64(exponent))
	d := time.Duration(delay)

	if policy.MaxDelay > 0 && d > policy.MaxDelay {
		d = policy.MaxDelay
	}
	return d
}
