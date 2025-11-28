package schedule

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewProcessExecutor(t *testing.T) {
	logger := testLogger()

	e := NewProcessExecutor(logger)
	if e == nil {
		t.Fatal("expected non-nil executor")
	}
	if e.configs == nil {
		t.Error("configs map should be initialized")
	}
}

func TestProcessExecutor_RegisterProcess(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	cfg := ProcessConfig{
		Command:    []string{"echo", "hello"},
		WorkingDir: "/tmp",
		Env:        map[string]string{"FOO": "bar"},
		Timeout:    10 * time.Second,
	}

	e.RegisterProcess("test", cfg)

	// Verify registration
	stored, exists := e.configs["test"]
	if !exists {
		t.Fatal("process should be registered")
	}
	if len(stored.Command) != 2 {
		t.Errorf("Command length = %d, want 2", len(stored.Command))
	}
	if stored.WorkingDir != "/tmp" {
		t.Errorf("WorkingDir = %q, want '/tmp'", stored.WorkingDir)
	}
}

func TestProcessExecutor_UnregisterProcess(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	e.RegisterProcess("test", ProcessConfig{Command: []string{"echo"}})
	e.UnregisterProcess("test")

	_, exists := e.configs["test"]
	if exists {
		t.Error("process should be unregistered")
	}

	// Unregistering non-existent process should not panic
	e.UnregisterProcess("non-existent")
}

func TestProcessExecutor_Execute_Success(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	e.RegisterProcess("test", ProcessConfig{
		Command: []string{"echo", "hello"},
	})

	ctx := context.Background()
	exitCode, err := e.Execute(ctx, "test")

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
}

func TestProcessExecutor_Execute_NotRegistered(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	ctx := context.Background()
	_, err := e.Execute(ctx, "non-existent")

	if err == nil {
		t.Error("Execute() should error on non-registered process")
	}
}

func TestProcessExecutor_Execute_EmptyCommand(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	e.RegisterProcess("test", ProcessConfig{
		Command: []string{},
	})

	ctx := context.Background()
	_, err := e.Execute(ctx, "test")

	if err == nil {
		t.Error("Execute() should error on empty command")
	}
}

func TestProcessExecutor_Execute_NonZeroExit(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	e.RegisterProcess("test", ProcessConfig{
		Command: []string{"sh", "-c", "exit 42"},
	})

	ctx := context.Background()
	exitCode, err := e.Execute(ctx, "test")

	if err == nil {
		t.Error("Execute() should error on non-zero exit")
	}
	if exitCode != 42 {
		t.Errorf("exitCode = %d, want 42", exitCode)
	}
}

func TestProcessExecutor_Execute_WithWorkingDir(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	e.RegisterProcess("test", ProcessConfig{
		Command:    []string{"pwd"},
		WorkingDir: "/tmp",
	})

	ctx := context.Background()
	exitCode, err := e.Execute(ctx, "test")

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
}

func TestProcessExecutor_Execute_WithEnv(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	e.RegisterProcess("test", ProcessConfig{
		Command: []string{"sh", "-c", "test $TEST_VAR = hello"},
		Env:     map[string]string{"TEST_VAR": "hello"},
	})

	ctx := context.Background()
	exitCode, err := e.Execute(ctx, "test")

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0 (env var should be set)", exitCode)
	}
}

func TestProcessExecutor_Execute_WithTimeout(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	e.RegisterProcess("test", ProcessConfig{
		Command: []string{"sleep", "5"},
		Timeout: 100 * time.Millisecond,
	})

	ctx := context.Background()
	start := time.Now()
	_, err := e.Execute(ctx, "test")
	duration := time.Since(start)

	if err == nil {
		t.Error("Execute() should error on timeout")
	}
	if duration >= 1*time.Second {
		t.Error("Execute() should have timed out quickly")
	}
}

func TestProcessExecutor_Execute_ContextCanceled(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	e.RegisterProcess("test", ProcessConfig{
		Command: []string{"sleep", "5"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := e.Execute(ctx, "test")

	if err == nil {
		t.Error("Execute() should error on context cancel")
	}
}

func TestProcessExecutor_Execute_ProcessEnvVars(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	// Test that PHPEEK_PM_PROCESS and PHPEEK_PM_SCHEDULED are set
	e.RegisterProcess("test", ProcessConfig{
		Command: []string{"sh", "-c", "test $PHPEEK_PM_PROCESS = test && test $PHPEEK_PM_SCHEDULED = true"},
	})

	ctx := context.Background()
	exitCode, err := e.Execute(ctx, "test")

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0 (PHPEEK_PM_* env vars should be set)", exitCode)
	}
}

func TestProcessExecutor_Execute_CommandNotFound(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	e.RegisterProcess("test", ProcessConfig{
		Command: []string{"nonexistent-command-12345"},
	})

	ctx := context.Background()
	_, err := e.Execute(ctx, "test")

	if err == nil {
		t.Error("Execute() should error on command not found")
	}
}

func TestProcessExecutor_Execute_PreservesParentEnv(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	// Set a test env var
	os.Setenv("TEST_PARENT_VAR", "inherited")
	defer os.Unsetenv("TEST_PARENT_VAR")

	e.RegisterProcess("test", ProcessConfig{
		Command: []string{"sh", "-c", "test $TEST_PARENT_VAR = inherited"},
	})

	ctx := context.Background()
	exitCode, err := e.Execute(ctx, "test")

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0 (parent env should be inherited)", exitCode)
	}
}

func TestProcessExecutor_Execute_CustomEnvOverridesParent(t *testing.T) {
	logger := testLogger()
	e := NewProcessExecutor(logger)

	// Set a test env var
	os.Setenv("OVERRIDE_VAR", "original")
	defer os.Unsetenv("OVERRIDE_VAR")

	e.RegisterProcess("test", ProcessConfig{
		Command: []string{"sh", "-c", "test $OVERRIDE_VAR = overridden"},
		Env:     map[string]string{"OVERRIDE_VAR": "overridden"},
	})

	ctx := context.Background()
	exitCode, err := e.Execute(ctx, "test")

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0 (custom env should override parent)", exitCode)
	}
}
