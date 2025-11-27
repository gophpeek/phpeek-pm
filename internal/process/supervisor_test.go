package process

import (
	"context"
	"log/slog"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/audit"
	"github.com/gophpeek/phpeek-pm/internal/config"
)

func TestSupervisor_WaitForReadiness(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auditLogger := audit.NewLogger(logger, false)

	tests := []struct {
		name        string
		healthCheck *config.HealthCheck
		markReady   bool
		timeout     time.Duration
		wantErr     bool
		description string
	}{
		{
			name:        "no health check - immediate ready",
			healthCheck: nil,
			markReady:   false,
			timeout:     1 * time.Second,
			wantErr:     false,
			description: "no health check configured returns immediately",
		},
		{
			name: "already ready with health check",
			healthCheck: &config.HealthCheck{
				Type:    "tcp",
				Address: "localhost:9000",
				Mode:    "readiness",
			},
			markReady:   true,
			timeout:     1 * time.Second,
			wantErr:     false,
			description: "process marked ready before wait",
		},
		{
			name: "becomes ready during wait",
			healthCheck: &config.HealthCheck{
				Type:    "tcp",
				Address: "localhost:9000",
				Mode:    "readiness",
			},
			markReady:   false,
			timeout:     2 * time.Second,
			wantErr:     false,
			description: "process becomes ready during wait period",
		},
		{
			name: "timeout waiting for readiness",
			healthCheck: &config.HealthCheck{
				Type:    "tcp",
				Address: "localhost:9000",
				Mode:    "readiness",
			},
			markReady:   false,
			timeout:     100 * time.Millisecond,
			wantErr:     true,
			description: "timeout exceeded before ready",
		},
		{
			name: "liveness mode skips readiness wait",
			healthCheck: &config.HealthCheck{
				Type:    "tcp",
				Address: "localhost:9000",
				Mode:    "liveness",
			},
			markReady:   false,
			timeout:     1 * time.Second,
			wantErr:     false,
			description: "liveness mode doesn't wait for readiness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Process{
				Enabled:     true,
				Command:     []string{"sleep", "60"},
				Restart:     "never",
				Scale:       1,
				HealthCheck: tt.healthCheck,
			}

			globalCfg := &config.GlobalConfig{
				LogLevel:           "error",
				MaxRestartAttempts: 3,
				RestartBackoff:     5,
			}

			sup := NewSupervisor("test-proc", cfg, globalCfg, logger, auditLogger, nil)

			if tt.markReady {
				sup.MarkReadyImmediately()
			} else if tt.name == "becomes ready during wait" {
				// Mark ready after a short delay
				go func() {
					time.Sleep(100 * time.Millisecond)
					sup.MarkReadyImmediately()
				}()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := sup.WaitForReadiness(ctx, tt.timeout)

			if (err != nil) != tt.wantErr {
				t.Errorf("WaitForReadiness() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSupervisor_StreamEnabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auditLogger := audit.NewLogger(logger, false)

	tests := []struct {
		name   string
		stream string
		want   bool
	}{
		{
			name:   "stdout stream",
			stream: "stdout",
			want:   true, // Default is true when not configured
		},
		{
			name:   "stderr stream",
			stream: "stderr",
			want:   true, // Default is true when not configured
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Process{
				Enabled: true,
				Command: []string{"echo", "test"},
			}

			globalCfg := &config.GlobalConfig{
				LogLevel: "error",
			}

			sup := NewSupervisor("test", cfg, globalCfg, logger, auditLogger, nil)
			got := sup.streamEnabled(tt.stream)

			if got != tt.want {
				t.Errorf("streamEnabled(%q) = %v, want %v", tt.stream, got, tt.want)
			}
		})
	}
}

func TestParseSignal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  syscall.Signal
	}{
		{
			name:  "SIGTERM",
			input: "SIGTERM",
			want:  syscall.SIGTERM,
		},
		{
			name:  "SIGKILL",
			input: "SIGKILL",
			want:  syscall.SIGKILL,
		},
		{
			name:  "SIGINT",
			input: "SIGINT",
			want:  syscall.SIGINT,
		},
		{
			name:  "SIGQUIT",
			input: "SIGQUIT",
			want:  syscall.SIGQUIT,
		},
		{
			name:  "unknown signal defaults to SIGTERM",
			input: "SIGUNKNOWN",
			want:  syscall.SIGTERM,
		},
		{
			name:  "empty string defaults to SIGTERM",
			input: "",
			want:  syscall.SIGTERM,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSignal(tt.input)

			if got != tt.want {
				t.Errorf("parseSignal(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSupervisor_GetState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auditLogger := audit.NewLogger(logger, false)

	tests := []struct {
		name          string
		initialState  string
		wantState     string
		startProcess  bool
		stopProcess   bool
		setupDelay    time.Duration
	}{
		{
			name:         "initial stopped state",
			initialState: "stopped",
			wantState:    "stopped",
			startProcess: false,
		},
		{
			name:         "running state",
			initialState: "running",
			wantState:    "running",
			startProcess: true,
			setupDelay:   200 * time.Millisecond,
		},
		{
			name:         "stopped after start and stop",
			initialState: "running",
			wantState:    "stopped",
			startProcess: true,
			stopProcess:  true,
			setupDelay:   200 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Process{
				Enabled:      true,
				InitialState: tt.initialState,
				Command:      []string{"sleep", "60"},
				Restart:      "never",
				Scale:        1,
			}

			globalCfg := &config.GlobalConfig{
				LogLevel:           "error",
				MaxRestartAttempts: 3,
				RestartBackoff:     5,
			}

			sup := NewSupervisor("test-state", cfg, globalCfg, logger, auditLogger, nil)

			ctx := context.Background()

			if tt.startProcess {
				if err := sup.Start(ctx); err != nil {
					t.Fatalf("Failed to start supervisor: %v", err)
				}
				time.Sleep(tt.setupDelay)

				if tt.stopProcess {
					stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					defer cancel()
					if err := sup.Stop(stopCtx); err != nil {
						t.Logf("Stop returned error (may be expected): %v", err)
					}
					time.Sleep(100 * time.Millisecond)
				}
			}

			state := sup.GetState()

			if string(state) != tt.wantState {
				t.Errorf("GetState() = %v, want %v", state, tt.wantState)
			}

			// Cleanup
			if tt.startProcess && !tt.stopProcess {
				stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				sup.Stop(stopCtx)
			}
		})
	}
}

func TestSupervisor_GetInstances(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auditLogger := audit.NewLogger(logger, false)

	tests := []struct {
		name              string
		scale             int
		wantInstanceCount int
		description       string
	}{
		{
			name:              "single instance",
			scale:             1,
			wantInstanceCount: 1,
			description:       "one process instance",
		},
		{
			name:              "multiple instances",
			scale:             3,
			wantInstanceCount: 3,
			description:       "three process instances",
		},
		{
			name:              "zero scale no instances",
			scale:             0,
			wantInstanceCount: 0,
			description:       "no instances when scale is 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Process{
				Enabled:      true,
				InitialState: "running",
				Command:      []string{"sleep", "60"},
				Restart:      "never",
				Scale:        tt.scale,
			}

			globalCfg := &config.GlobalConfig{
				LogLevel:           "error",
				MaxRestartAttempts: 3,
				RestartBackoff:     5,
			}

			sup := NewSupervisor("test-instances", cfg, globalCfg, logger, auditLogger, nil)

			if tt.scale > 0 {
				ctx := context.Background()
				if err := sup.Start(ctx); err != nil {
					t.Fatalf("Failed to start supervisor: %v", err)
				}
				defer func() {
					stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					sup.Stop(stopCtx)
				}()

				// Wait for instances to start
				time.Sleep(200 * time.Millisecond)
			}

			instances := sup.GetInstances()

			if len(instances) != tt.wantInstanceCount {
				t.Errorf("GetInstances() returned %d instances, want %d", len(instances), tt.wantInstanceCount)
			}

			// Verify instance info fields
			for i, inst := range instances {
				if inst.ID == "" {
					t.Errorf("Instance %d has empty ID", i)
				}
				if tt.scale > 0 && inst.PID <= 0 {
					t.Errorf("Instance %d has invalid PID: %d", i, inst.PID)
				}
			}
		})
	}
}

func TestSupervisor_CheckAllInstancesDead(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auditLogger := audit.NewLogger(logger, false)

	cfg := &config.Process{
		Enabled:      true,
		InitialState: "running",
		Command:      []string{"sh", "-c", "exit 0"}, // Exit immediately
		Restart:      "never",
		Scale:        2,
	}

	globalCfg := &config.GlobalConfig{
		LogLevel:           "error",
		MaxRestartAttempts: 3,
		RestartBackoff:     5,
	}

	sup := NewSupervisor("test-dead", cfg, globalCfg, logger, auditLogger, nil)

	// Track death notifications
	deathNotified := make(chan string, 1)
	sup.SetDeathNotifier(func(name string) {
		select {
		case deathNotified <- name:
		default:
		}
	})

	ctx := context.Background()
	if err := sup.Start(ctx); err != nil {
		t.Fatalf("Failed to start supervisor: %v", err)
	}

	// Wait for processes to exit and death check to complete
	select {
	case name := <-deathNotified:
		if name != "test-dead" {
			t.Errorf("Death notifier received wrong name: %s", name)
		}
	case <-time.After(5 * time.Second):
		// May not get notification if processes exit too fast
		t.Log("No death notification received (processes may have exited immediately)")
	}

	// Verify all instances are dead
	instances := sup.GetInstances()
	allDead := true
	for _, inst := range instances {
		if inst.State == "running" {
			allDead = false
			break
		}
	}

	if !allDead {
		t.Error("Expected all instances to be dead")
	}
}

func TestSupervisor_GetLogs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auditLogger := audit.NewLogger(logger, false)

	cfg := &config.Process{
		Enabled:      true,
		InitialState: "running",
		Command:      []string{"sh", "-c", "echo 'test log output' && sleep 1"},
		Restart:      "never",
		Scale:        1,
	}

	globalCfg := &config.GlobalConfig{
		LogLevel:           "info",
		MaxRestartAttempts: 3,
		RestartBackoff:     5,
	}

	sup := NewSupervisor("test-logs", cfg, globalCfg, logger, auditLogger, nil)

	ctx := context.Background()
	if err := sup.Start(ctx); err != nil {
		t.Fatalf("Failed to start supervisor: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		sup.Stop(stopCtx)
	}()

	// Wait for process to produce logs
	time.Sleep(500 * time.Millisecond)

	tests := []struct {
		name  string
		limit int
	}{
		{
			name:  "get all logs",
			limit: 0,
		},
		{
			name:  "get limited logs",
			limit: 5,
		},
		{
			name:  "get single log",
			limit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := sup.GetLogs(tt.limit)

			// We should have some logs (may be empty if process hasn't logged yet)
			if tt.limit > 0 && len(logs) > tt.limit {
				t.Errorf("GetLogs(%d) returned %d logs, expected <= %d", tt.limit, len(logs), tt.limit)
			}

			// Log content for debugging
			t.Logf("Retrieved %d logs with limit %d", len(logs), tt.limit)
		})
	}
}

func TestSupervisor_MonitorInstance_Lifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	auditLogger := audit.NewLogger(logger, false)

	tests := []struct {
		name         string
		command      []string
		restart      string
		expectRestart bool
		waitTime     time.Duration
	}{
		{
			name:          "process exits with restart never",
			command:       []string{"sh", "-c", "exit 0"},
			restart:       "never",
			expectRestart: false,
			waitTime:      2 * time.Second,
		},
		{
			name:          "process fails with restart on-failure",
			command:       []string{"sh", "-c", "exit 1"},
			restart:       "on-failure",
			expectRestart: true,
			waitTime:      3 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Process{
				Enabled:      true,
				InitialState: "running",
				Command:      tt.command,
				Restart:      tt.restart,
				Scale:        1,
			}

			globalCfg := &config.GlobalConfig{
				LogLevel:           "error",
				MaxRestartAttempts: 2,
				RestartBackoff:     1,
			}

			sup := NewSupervisor("test-monitor", cfg, globalCfg, logger, auditLogger, nil)

			ctx := context.Background()
			if err := sup.Start(ctx); err != nil {
				t.Fatalf("Failed to start supervisor: %v", err)
			}
			defer func() {
				stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				sup.Stop(stopCtx)
			}()

			// Wait for monitoring to kick in
			time.Sleep(tt.waitTime)

			instances := sup.GetInstances()
			if len(instances) == 0 {
				t.Fatal("No instances found")
			}

			// Check restart count
			sup.mu.RLock()
			restartCount := instances[0].RestartCount
			sup.mu.RUnlock()

			if tt.expectRestart && restartCount == 0 {
				t.Error("Expected process to restart, but restart count is 0")
			}

			if !tt.expectRestart && restartCount > 0 {
				t.Errorf("Expected no restarts, but got %d restarts", restartCount)
			}
		})
	}
}
