package schedule

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockExecutor implements JobExecutor for testing
type mockExecutor struct {
	executeCalled int64
	delay         time.Duration
	returnCode    int
	returnErr     error
	mu            sync.Mutex
}

func (m *mockExecutor) Execute(ctx context.Context, processName string) (int, error) {
	atomic.AddInt64(&m.executeCalled, 1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return -1, ctx.Err()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.returnCode, m.returnErr
}

func (m *mockExecutor) callCount() int64 {
	return atomic.LoadInt64(&m.executeCalled)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestJobState_String(t *testing.T) {
	tests := []struct {
		state JobState
		want  string
	}{
		{JobStateIdle, "idle"},
		{JobStateExecuting, "executing"},
		{JobStatePaused, "paused"},
		{JobState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("JobState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestNewScheduledJob(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	tests := []struct {
		name         string
		scheduleExpr string
		wantErr      bool
	}{
		{
			name:         "valid cron expression",
			scheduleExpr: "*/5 * * * *",
			wantErr:      false,
		},
		{
			name:         "another valid expression",
			scheduleExpr: "0 9 * * 1-5",
			wantErr:      false,
		},
		{
			name:         "invalid cron expression",
			scheduleExpr: "not a cron",
			wantErr:      true,
		},
		{
			name:         "empty expression",
			scheduleExpr: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := NewScheduledJob("test-job", tt.scheduleExpr, "", 10, executor, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewScheduledJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if job == nil {
					t.Error("expected non-nil job")
					return
				}
				if job.Name != "test-job" {
					t.Errorf("Name = %q, want 'test-job'", job.Name)
				}
				if job.GetState() != JobStateIdle {
					t.Errorf("initial State = %v, want JobStateIdle", job.GetState())
				}
			}
		})
	}
}

func TestScheduledJob_PauseResume(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, err := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Initial state
	if job.IsPaused() {
		t.Error("job should not be paused initially")
	}

	// Pause
	if err := job.Pause(); err != nil {
		t.Errorf("Pause() error = %v", err)
	}
	if !job.IsPaused() {
		t.Error("job should be paused")
	}
	if job.GetState() != JobStatePaused {
		t.Errorf("State = %v, want JobStatePaused", job.GetState())
	}

	// Pause again (should be idempotent)
	if err := job.Pause(); err != nil {
		t.Errorf("Pause() second call error = %v", err)
	}

	// Resume
	if err := job.Resume(); err != nil {
		t.Errorf("Resume() error = %v", err)
	}
	if job.IsPaused() {
		t.Error("job should not be paused after resume")
	}
	if job.GetState() != JobStateIdle {
		t.Errorf("State = %v, want JobStateIdle", job.GetState())
	}

	// Resume when not paused
	if err := job.Resume(); err == nil {
		t.Error("Resume() should error when not paused")
	}
}

func TestScheduledJob_CannotPauseWhileExecuting(t *testing.T) {
	executor := &mockExecutor{delay: 100 * time.Millisecond}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	// Start execution in background
	go job.Run()

	// Wait a bit for execution to start
	time.Sleep(10 * time.Millisecond)

	// Should not be able to pause
	err := job.Pause()
	if err == nil {
		t.Error("Pause() should error when job is executing")
	}

	// Wait for execution to complete
	time.Sleep(150 * time.Millisecond)
}

func TestScheduledJob_Trigger(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	ctx := context.Background()

	// Trigger when idle
	if err := job.Trigger(ctx); err != nil {
		t.Errorf("Trigger() error = %v", err)
	}

	// Wait for execution
	time.Sleep(50 * time.Millisecond)

	if executor.callCount() != 1 {
		t.Errorf("executor called %d times, want 1", executor.callCount())
	}

	// Check history
	if job.History.Len() != 1 {
		t.Errorf("History.Len() = %d, want 1", job.History.Len())
	}
}

func TestScheduledJob_TriggerWhilePaused(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	job.Pause()

	ctx := context.Background()
	if err := job.Trigger(ctx); err == nil {
		t.Error("Trigger() should error when paused")
	}
}

func TestScheduledJob_TriggerWhileExecuting(t *testing.T) {
	executor := &mockExecutor{delay: 100 * time.Millisecond}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	ctx := context.Background()

	// Start first execution
	job.Trigger(ctx)
	time.Sleep(10 * time.Millisecond)

	// Try to trigger again
	if err := job.Trigger(ctx); err == nil {
		t.Error("Trigger() should error when already executing")
	}

	// Wait for completion
	time.Sleep(150 * time.Millisecond)
}

func TestScheduledJob_TriggerSync(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	ctx := context.Background()

	exitCode, err := job.TriggerSync(ctx)
	if err != nil {
		t.Errorf("TriggerSync() error = %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	// Verify history
	last, _ := job.History.GetLast()
	if !last.Success {
		t.Error("last execution should be successful")
	}
}

func TestScheduledJob_TriggerSync_WithError(t *testing.T) {
	executor := &mockExecutor{returnCode: 1, returnErr: errors.New("command failed")}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	ctx := context.Background()

	exitCode, err := job.TriggerSync(ctx)
	if err == nil {
		t.Error("TriggerSync() should return error")
	}
	if exitCode != 1 {
		t.Errorf("exitCode = %d, want 1", exitCode)
	}

	// Verify history
	last, _ := job.History.GetLast()
	if last.Success {
		t.Error("last execution should not be successful")
	}
	if last.Error == "" {
		t.Error("last execution should have error message")
	}
}

func TestScheduledJob_TriggerSync_Canceled(t *testing.T) {
	executor := &mockExecutor{delay: 1 * time.Second}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := job.TriggerSync(ctx)
	if err == nil {
		t.Error("TriggerSync() should error when context canceled")
	}
}

func TestScheduledJob_Run(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	// Run implements cron.Job interface
	job.Run()

	if executor.callCount() != 1 {
		t.Errorf("executor called %d times, want 1", executor.callCount())
	}
}

func TestScheduledJob_Run_SkipsWhenPaused(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)
	job.Pause()

	// Run should not execute
	job.Run()

	if executor.callCount() != 0 {
		t.Errorf("executor called %d times, want 0 when paused", executor.callCount())
	}
}

func TestScheduledJob_Status(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "UTC", 10, executor, logger)

	status := job.Status()

	if status.Name != "test-job" {
		t.Errorf("Name = %q, want 'test-job'", status.Name)
	}
	if status.Schedule != "*/5 * * * *" {
		t.Errorf("Schedule = %q, want '*/5 * * * *'", status.Schedule)
	}
	if status.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want 'UTC'", status.Timezone)
	}
	if status.State != "idle" {
		t.Errorf("State = %q, want 'idle'", status.State)
	}
}

func TestScheduledJob_CronID(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	// Initial ID should be 0
	if id := job.GetCronID(); id != 0 {
		t.Errorf("initial CronID = %d, want 0", id)
	}

	// Set and get
	job.SetCronID(42)
	if id := job.GetCronID(); id != 42 {
		t.Errorf("CronID = %d, want 42", id)
	}
}

func TestScheduledJob_NextRun(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	// Initial should be zero
	if !job.GetNextRun().IsZero() {
		t.Error("initial NextRun should be zero")
	}

	// Update and get
	nextRun := time.Now().Add(5 * time.Minute)
	job.UpdateNextRun(nextRun)

	got := job.GetNextRun()
	if !got.Equal(nextRun) {
		t.Errorf("NextRun = %v, want %v", got, nextRun)
	}
}

func TestScheduledJob_LastRun(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	// Initial should be zero
	if !job.GetLastRun().IsZero() {
		t.Error("initial LastRun should be zero")
	}

	// Execute and check LastRun is updated
	job.TriggerSync(context.Background())

	lastRun := job.GetLastRun()
	if lastRun.IsZero() {
		t.Error("LastRun should not be zero after execution")
	}
}

func TestScheduledJob_IsExecuting(t *testing.T) {
	executor := &mockExecutor{delay: 100 * time.Millisecond}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	// Initially not executing
	if job.IsExecuting() {
		t.Error("should not be executing initially")
	}

	// Start execution
	go job.Run()
	time.Sleep(10 * time.Millisecond)

	if !job.IsExecuting() {
		t.Error("should be executing")
	}

	// Wait for completion
	time.Sleep(150 * time.Millisecond)

	if job.IsExecuting() {
		t.Error("should not be executing after completion")
	}
}

func TestScheduledJob_CanExecute(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 10, executor, logger)

	// Can execute when idle
	if !job.CanExecute() {
		t.Error("should be able to execute when idle")
	}

	// Cannot execute when paused
	job.Pause()
	if job.CanExecute() {
		t.Error("should not be able to execute when paused")
	}
}

func TestScheduledJob_Concurrent(t *testing.T) {
	executor := &mockExecutor{delay: 10 * time.Millisecond}
	logger := testLogger()

	job, _ := NewScheduledJob("test-job", "*/5 * * * *", "", 100, executor, logger)

	var wg sync.WaitGroup

	// Multiple concurrent triggers (most should be rejected)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job.Trigger(context.Background())
		}()
	}

	// Concurrent state reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job.GetState()
			job.IsPaused()
			job.IsExecuting()
			job.CanExecute()
			job.Status()
		}()
	}

	wg.Wait()
}
