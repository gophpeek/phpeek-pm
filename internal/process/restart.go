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
	maxAttempts int
	backoff     time.Duration
}

// NewAlwaysRestartPolicy creates a new always restart policy
func NewAlwaysRestartPolicy(maxAttempts int, backoff time.Duration) *AlwaysRestartPolicy {
	return &AlwaysRestartPolicy{
		maxAttempts: maxAttempts,
		backoff:     backoff,
	}
}

func (p *AlwaysRestartPolicy) ShouldRestart(exitCode int, restartCount int) bool {
	if p.maxAttempts <= 0 {
		return true
	}
	return restartCount < p.maxAttempts
}

func (p *AlwaysRestartPolicy) BackoffDuration(restartCount int) time.Duration {
	// Exponential backoff: backoff * 2^restartCount (capped at 5 minutes)
	duration := p.backoff * time.Duration(1<<uint(restartCount))
	if duration > 5*time.Minute {
		return 5 * time.Minute
	}
	return duration
}

// OnFailureRestartPolicy restarts only on non-zero exit codes
type OnFailureRestartPolicy struct {
	maxAttempts int
	backoff     time.Duration
}

// NewOnFailureRestartPolicy creates a new on-failure restart policy
func NewOnFailureRestartPolicy(maxAttempts int, backoff time.Duration) *OnFailureRestartPolicy {
	return &OnFailureRestartPolicy{
		maxAttempts: maxAttempts,
		backoff:     backoff,
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
	duration := p.backoff * time.Duration(1<<uint(restartCount))
	if duration > 5*time.Minute {
		return 5 * time.Minute
	}
	return duration
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
func NewRestartPolicy(policyType string, maxAttempts int, backoff time.Duration) RestartPolicy {
	switch policyType {
	case "always":
		return NewAlwaysRestartPolicy(maxAttempts, backoff)
	case "on-failure":
		return NewOnFailureRestartPolicy(maxAttempts, backoff)
	default:
		return NewNeverRestartPolicy()
	}
}
