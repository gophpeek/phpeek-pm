package process

import (
	"time"
)

// RestartPolicy defines restart behavior
type RestartPolicy interface {
	ShouldRestart(exitCode int, restartCount int) bool
	BackoffDuration(restartCount int) time.Duration
}

// AlwaysRestartPolicy always restarts processes up to max attempts
type AlwaysRestartPolicy struct {
	maxAttempts    int
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

// NewAlwaysRestartPolicy creates a new always restart policy
func NewAlwaysRestartPolicy(maxAttempts int, initial, max time.Duration) *AlwaysRestartPolicy {
	return &AlwaysRestartPolicy{
		maxAttempts:    maxAttempts,
		initialBackoff: initial,
		maxBackoff:     max,
	}
}

func (p *AlwaysRestartPolicy) ShouldRestart(exitCode int, restartCount int) bool {
	if p.maxAttempts <= 0 {
		return true
	}
	return restartCount < p.maxAttempts
}

func (p *AlwaysRestartPolicy) BackoffDuration(restartCount int) time.Duration {
	return calculateBackoff(p.initialBackoff, p.maxBackoff, restartCount)
}

// OnFailureRestartPolicy restarts only on non-zero exit codes
type OnFailureRestartPolicy struct {
	maxAttempts    int
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

// NewOnFailureRestartPolicy creates a new on-failure restart policy
func NewOnFailureRestartPolicy(maxAttempts int, initial, max time.Duration) *OnFailureRestartPolicy {
	return &OnFailureRestartPolicy{
		maxAttempts:    maxAttempts,
		initialBackoff: initial,
		maxBackoff:     max,
	}
}

func (p *OnFailureRestartPolicy) ShouldRestart(exitCode int, restartCount int) bool {
	if exitCode == 0 {
		return false // Clean exit, don't restart
	}
	if p.maxAttempts <= 0 {
		return true
	}
	return restartCount < p.maxAttempts
}

func (p *OnFailureRestartPolicy) BackoffDuration(restartCount int) time.Duration {
	return calculateBackoff(p.initialBackoff, p.maxBackoff, restartCount)
}

// NeverRestartPolicy never restarts processes
type NeverRestartPolicy struct{}

// NewNeverRestartPolicy creates a new never restart policy
func NewNeverRestartPolicy() *NeverRestartPolicy {
	return &NeverRestartPolicy{}
}

func (p *NeverRestartPolicy) ShouldRestart(exitCode int, restartCount int) bool {
	return false
}

func (p *NeverRestartPolicy) BackoffDuration(restartCount int) time.Duration {
	return 0
}

// NewRestartPolicy creates a restart policy based on type
func NewRestartPolicy(policyType string, maxAttempts int, initial, max time.Duration) RestartPolicy {
	switch policyType {
	case "always":
		return NewAlwaysRestartPolicy(maxAttempts, initial, max)
	case "on-failure":
		return NewOnFailureRestartPolicy(maxAttempts, initial, max)
	default:
		return NewNeverRestartPolicy()
	}
}

func calculateBackoff(initial, max time.Duration, restartCount int) time.Duration {
	if initial <= 0 {
		initial = 1 * time.Second
	}
	// Cap restart count to prevent integer overflow (max 62 for safe bit shift)
	if restartCount < 0 {
		restartCount = 0
	}
	const maxShift = 62
	if restartCount > maxShift {
		restartCount = maxShift
	}
	delay := initial * time.Duration(1<<uint(restartCount)) // #nosec G115 -- bounds checked above
	if max > 0 && delay > max {
		return max
	}
	return delay
}
