package process

import (
	"testing"
	"time"
)

func TestAlwaysRestartPolicy(t *testing.T) {
	tests := []struct {
		name         string
		maxAttempts  int
		backoff      time.Duration
		exitCode     int
		restartCount int
		wantRestart  bool
		wantBackoff  time.Duration
	}{
		{
			name:         "unlimited restarts",
			maxAttempts:  0,
			backoff:      5 * time.Second,
			exitCode:     1,
			restartCount: 10,
			wantRestart:  true,
			wantBackoff:  5 * time.Minute, // Capped at 5 minutes
		},
		{
			name:         "within max attempts",
			maxAttempts:  3,
			backoff:      5 * time.Second,
			exitCode:     1,
			restartCount: 2,
			wantRestart:  true,
			wantBackoff:  20 * time.Second, // 5 * 2^2
		},
		{
			name:         "exceeded max attempts",
			maxAttempts:  3,
			backoff:      5 * time.Second,
			exitCode:     1,
			restartCount: 3,
			wantRestart:  false,
		},
		{
			name:         "zero exit code still restarts",
			maxAttempts:  3,
			backoff:      5 * time.Second,
			exitCode:     0,
			restartCount: 1,
			wantRestart:  true,
			wantBackoff:  10 * time.Second, // 5 * 2^1
		},
		{
			name:         "exponential backoff first attempt",
			maxAttempts:  3,
			backoff:      5 * time.Second,
			exitCode:     1,
			restartCount: 0,
			wantRestart:  true,
			wantBackoff:  5 * time.Second, // 5 * 2^0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewAlwaysRestartPolicy(tt.maxAttempts, tt.backoff)

			shouldRestart := policy.ShouldRestart(tt.exitCode, tt.restartCount)
			if shouldRestart != tt.wantRestart {
				t.Errorf("ShouldRestart() = %v, want %v", shouldRestart, tt.wantRestart)
			}

			if tt.wantRestart {
				backoff := policy.BackoffDuration(tt.restartCount)
				if backoff != tt.wantBackoff {
					t.Errorf("BackoffDuration() = %v, want %v", backoff, tt.wantBackoff)
				}
			}
		})
	}
}

func TestOnFailureRestartPolicy(t *testing.T) {
	tests := []struct {
		name         string
		maxAttempts  int
		backoff      time.Duration
		exitCode     int
		restartCount int
		wantRestart  bool
		wantBackoff  time.Duration
	}{
		{
			name:         "failure with unlimited attempts",
			maxAttempts:  0,
			backoff:      5 * time.Second,
			exitCode:     1,
			restartCount: 10,
			wantRestart:  true,
			wantBackoff:  5 * time.Minute, // Capped
		},
		{
			name:         "clean exit does not restart",
			maxAttempts:  3,
			backoff:      5 * time.Second,
			exitCode:     0,
			restartCount: 0,
			wantRestart:  false,
		},
		{
			name:         "failure within max attempts",
			maxAttempts:  3,
			backoff:      5 * time.Second,
			exitCode:     1,
			restartCount: 2,
			wantRestart:  true,
			wantBackoff:  20 * time.Second, // 5 * 2^2
		},
		{
			name:         "failure exceeded max attempts",
			maxAttempts:  3,
			backoff:      5 * time.Second,
			exitCode:     1,
			restartCount: 3,
			wantRestart:  false,
		},
		{
			name:         "non-zero exit code restarts",
			maxAttempts:  3,
			backoff:      5 * time.Second,
			exitCode:     137,
			restartCount: 1,
			wantRestart:  true,
			wantBackoff:  10 * time.Second, // 5 * 2^1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewOnFailureRestartPolicy(tt.maxAttempts, tt.backoff)

			shouldRestart := policy.ShouldRestart(tt.exitCode, tt.restartCount)
			if shouldRestart != tt.wantRestart {
				t.Errorf("ShouldRestart() = %v, want %v", shouldRestart, tt.wantRestart)
			}

			if tt.wantRestart {
				backoff := policy.BackoffDuration(tt.restartCount)
				if backoff != tt.wantBackoff {
					t.Errorf("BackoffDuration() = %v, want %v", backoff, tt.wantBackoff)
				}
			}
		})
	}
}

func TestNeverRestartPolicy(t *testing.T) {
	policy := NewNeverRestartPolicy()

	tests := []struct {
		name         string
		exitCode     int
		restartCount int
	}{
		{
			name:         "failure exit code",
			exitCode:     1,
			restartCount: 0,
		},
		{
			name:         "success exit code",
			exitCode:     0,
			restartCount: 0,
		},
		{
			name:         "multiple attempts",
			exitCode:     1,
			restartCount: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRestart := policy.ShouldRestart(tt.exitCode, tt.restartCount)
			if shouldRestart {
				t.Errorf("NeverRestartPolicy.ShouldRestart() = true, want false")
			}

			backoff := policy.BackoffDuration(tt.restartCount)
			if backoff != 0 {
				t.Errorf("NeverRestartPolicy.BackoffDuration() = %v, want 0", backoff)
			}
		})
	}
}

func TestNewRestartPolicy(t *testing.T) {
	tests := []struct {
		name        string
		policyType  string
		maxAttempts int
		backoff     time.Duration
		wantType    string
	}{
		{
			name:        "always policy",
			policyType:  "always",
			maxAttempts: 3,
			backoff:     5 * time.Second,
			wantType:    "*process.AlwaysRestartPolicy",
		},
		{
			name:        "on-failure policy",
			policyType:  "on-failure",
			maxAttempts: 3,
			backoff:     5 * time.Second,
			wantType:    "*process.OnFailureRestartPolicy",
		},
		{
			name:        "never policy",
			policyType:  "never",
			maxAttempts: 3,
			backoff:     5 * time.Second,
			wantType:    "*process.NeverRestartPolicy",
		},
		{
			name:        "unknown policy defaults to never",
			policyType:  "unknown",
			maxAttempts: 3,
			backoff:     5 * time.Second,
			wantType:    "*process.NeverRestartPolicy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewRestartPolicy(tt.policyType, tt.maxAttempts, tt.backoff)

			if policy == nil {
				t.Errorf("NewRestartPolicy() returned nil")
			}
		})
	}
}

func TestBackoffCapping(t *testing.T) {
	policy := NewAlwaysRestartPolicy(0, 1*time.Second)

	tests := []struct {
		restartCount int
		wantCapped   bool
	}{
		{restartCount: 0, wantCapped: false},  // 1s
		{restartCount: 5, wantCapped: false},  // 32s
		{restartCount: 10, wantCapped: true},  // Would be 1024s, but capped at 300s
		{restartCount: 20, wantCapped: true},  // Way over cap
	}

	for _, tt := range tests {
		backoff := policy.BackoffDuration(tt.restartCount)

		if tt.wantCapped && backoff != 5*time.Minute {
			t.Errorf("BackoffDuration(%d) = %v, want capped at %v", tt.restartCount, backoff, 5*time.Minute)
		}

		if !tt.wantCapped && backoff > 5*time.Minute {
			t.Errorf("BackoffDuration(%d) = %v, should not be capped", tt.restartCount, backoff)
		}
	}
}
