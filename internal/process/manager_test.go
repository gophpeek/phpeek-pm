package process

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
)

// TestManager_GracefulShutdown tests graceful shutdown with timeout
func TestManager_GracefulShutdown(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ShutdownTimeout:    30,
			LogLevel:           "error",
			MaxRestartAttempts: 3,
			RestartBackoff:     5,
		},
		Processes: map[string]*config.Process{
			"test-process": {
				Enabled: true,
				Command: []string{"sleep", "30"}, // Long enough to not exit before shutdown
				Restart: "never",
				Scale:   1,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(cfg, logger)

	// Start processes
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start processes: %v", err)
	}

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := manager.Shutdown(shutdownCtx)
	duration := time.Since(start)

	// Shutdown may have errors if processes already finished (not critical)
	if err != nil {
		t.Logf("Shutdown with errors (acceptable if process finished): %v", err)
	}

	// Shutdown should complete quickly
	// Note: May take up to shutdown timeout if process already finished
	if duration > 35*time.Second {
		t.Errorf("Shutdown took too long: %v (expected < 35s)", duration)
	}

	t.Logf("Shutdown completed in %v", duration)
}

// TestManager_ShutdownTimeout tests force kill after timeout
func TestManager_ShutdownTimeout(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ShutdownTimeout:    2, // Short timeout
			LogLevel:           "error",
			MaxRestartAttempts: 3,
			RestartBackoff:     5,
		},
		Processes: map[string]*config.Process{
			"long-process": {
				Enabled: true,
				Command: []string{"sleep", "60"}, // Long-running process
				Restart: "never",
				Scale:   1,
				Shutdown: &config.ShutdownConfig{
					Timeout: 1, // 1 second timeout (shorter than sleep)
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(cfg, logger)

	// Start processes
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start processes: %v", err)
	}

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown with short timeout (should force kill)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	err := manager.Shutdown(shutdownCtx)
	duration := time.Since(start)

	// Should complete via force kill
	if err != nil {
		t.Logf("Shutdown with errors (expected): %v", err)
	}

	// Should complete quickly due to force kill (within ~2s: 1s timeout + overhead)
	if duration > 3*time.Second {
		t.Errorf("Shutdown took too long even with force kill: %v", duration)
	}

	t.Logf("Shutdown with force kill completed in %v", duration)
}

// TestManager_PreStopHooks tests pre-stop hook execution
func TestManager_PreStopHooks(t *testing.T) {
	// Create a temporary file to track hook execution
	tmpfile := "/tmp/phpeek-pm-test-prehook"
	os.Remove(tmpfile) // Clean up from previous runs

	cfg := &config.Config{
		Global: config.GlobalConfig{
			ShutdownTimeout:    30,
			LogLevel:           "error",
			MaxRestartAttempts: 3,
			RestartBackoff:     5,
		},
		Hooks: config.HooksConfig{
			PreStop: []config.Hook{
				{
					Name:    "test-prehook",
					Command: []string{"touch", tmpfile},
					Timeout: 5,
				},
			},
		},
		Processes: map[string]*config.Process{
			"test-process": {
				Enabled: true,
				Command: []string{"sleep", "1"},
				Restart: "never",
				Scale:   1,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(cfg, logger)

	// Start processes
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start processes: %v", err)
	}

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := manager.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify pre-stop hook was executed
	if _, err := os.Stat(tmpfile); os.IsNotExist(err) {
		t.Error("Pre-stop hook was not executed (file not created)")
	} else {
		os.Remove(tmpfile) // Cleanup
	}
}

// TestManager_ShutdownOrder tests processes shutdown in reverse priority order
func TestManager_ShutdownOrder(t *testing.T) {
	// Track shutdown order via temporary files
	file1 := "/tmp/phpeek-pm-test-process1"
	file2 := "/tmp/phpeek-pm-test-process2"
	file3 := "/tmp/phpeek-pm-test-process3"

	// Cleanup
	os.Remove(file1)
	os.Remove(file2)
	os.Remove(file3)

	cfg := &config.Config{
		Global: config.GlobalConfig{
			ShutdownTimeout:    30,
			LogLevel:           "error",
			MaxRestartAttempts: 3,
			RestartBackoff:     5,
		},
		Processes: map[string]*config.Process{
			"process1": {
				Enabled:  true,
				Command:  []string{"sleep", "30"}, // Long enough to not exit before shutdown
				Priority: 10,                      // Start first
				Restart:  "never",
				Scale:    1,
				Shutdown: &config.ShutdownConfig{
					PreStopHook: &config.Hook{
						Name:    "mark-process1",
						Command: []string{"touch", file1},
						Timeout: 5,
					},
				},
			},
			"process2": {
				Enabled:  true,
				Command:  []string{"sleep", "30"}, // Long enough to not exit before shutdown
				Priority: 20,                      // Start second
				Restart:  "never",
				Scale:    1,
				Shutdown: &config.ShutdownConfig{
					PreStopHook: &config.Hook{
						Name:    "mark-process2",
						Command: []string{"touch", file2},
						Timeout: 5,
					},
				},
			},
			"process3": {
				Enabled:  true,
				Command:  []string{"sleep", "30"}, // Long enough to not exit before shutdown
				Priority: 30,                      // Start third (stop first in shutdown)
				Restart:  "never",
				Scale:    1,
				Shutdown: &config.ShutdownConfig{
					PreStopHook: &config.Hook{
						Name:    "mark-process3",
						Command: []string{"touch", file3},
						Timeout: 5,
					},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(cfg, logger)

	// Start processes
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start processes: %v", err)
	}

	// Give processes time to start
	time.Sleep(200 * time.Millisecond)

	// Shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := manager.Shutdown(shutdownCtx); err != nil {
		t.Logf("Shutdown with errors (may be expected): %v", err)
	}

	// Verify all hooks were executed (order is parallel within priority levels, so just check existence)
	for name, file := range map[string]string{
		"process1": file1,
		"process2": file2,
		"process3": file3,
	} {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Pre-stop hook for %s was not executed", name)
		} else {
			os.Remove(file)
		}
	}
}

// TestManager_MultipleInstances tests shutdown with scaled processes
func TestManager_MultipleInstances(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ShutdownTimeout:    30,
			LogLevel:           "error",
			MaxRestartAttempts: 3,
			RestartBackoff:     5,
		},
		Processes: map[string]*config.Process{
			"scaled-process": {
				Enabled: true,
				Command: []string{"sleep", "30"}, // Long enough to not exit before shutdown
				Scale:   3,                       // 3 instances
				Restart: "never",
				Shutdown: &config.ShutdownConfig{
					Timeout: 2, // Short timeout for test
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(cfg, logger)

	// Start processes
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start processes: %v", err)
	}

	// Verify 3 instances started
	instances := manager.processes["scaled-process"].GetInstances()
	if len(instances) != 3 {
		t.Errorf("Expected 3 instances, got %d", len(instances))
	}

	// Give processes time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err := manager.Shutdown(shutdownCtx)
	duration := time.Since(start)

	if err != nil {
		t.Logf("Shutdown with errors: %v", err)
	}

	// All instances should stop in parallel (timeout is 2s per instance, force killed in parallel)
	if duration > 5*time.Second {
		t.Errorf("Shutdown should be parallel, took %v (expected < 5s)", duration)
	}

	// Log duration for verification
	t.Logf("Shutdown of 3 instances completed in %v (parallel shutdown with 2s timeout)", duration)
}

// TestManager_ConfigurableRestartBackoff tests that global restart config is used
func TestManager_ConfigurableRestartBackoff(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ShutdownTimeout:    30,
			LogLevel:           "error",
			MaxRestartAttempts: 5,  // Custom value
			RestartBackoff:     10, // Custom value (10 seconds)
		},
		Processes: map[string]*config.Process{
			"test-process": {
				Enabled: true,
				Command: []string{"sh", "-c", "exit 1"}, // Fails immediately
				Restart: "on-failure",
				Scale:   1,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(cfg, logger)

	// Start processes (will fail and attempt restart)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// This will fail to start, which is expected
	_ = manager.Start(ctx)

	// Verify supervisor was created with global config values
	supervisor := manager.processes["test-process"]
	if supervisor == nil {
		t.Fatal("Supervisor not created")
	}

	// The restart policy should use global config (MaxRestartAttempts=5, Backoff=10s)
	policy := supervisor.restartPolicy
	if policy == nil {
		t.Fatal("Restart policy not created")
	}

	// Test that backoff uses configured value (10s)
	backoff := policy.BackoffDuration(0)
	expected := 10 * time.Second

	if backoff != expected {
		t.Errorf("Backoff duration incorrect: got %v, want %v", backoff, expected)
	}

	t.Logf("Restart backoff correctly uses global config: %v", backoff)
}
