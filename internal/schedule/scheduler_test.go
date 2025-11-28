package schedule

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

func TestNewScheduler(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()

	s := NewScheduler(executor, 50, logger)

	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if s.historySize != 50 {
		t.Errorf("historySize = %d, want 50", s.historySize)
	}
	if len(s.jobs) != 0 {
		t.Errorf("initial jobs = %d, want 0", len(s.jobs))
	}
	if s.IsStarted() {
		t.Error("scheduler should not be started initially")
	}
}

func TestScheduler_AddJob(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	// Add valid job
	err := s.AddJob("test-job", "*/5 * * * *", "UTC")
	if err != nil {
		t.Fatalf("AddJob() error = %v", err)
	}

	// Verify job exists
	job, exists := s.GetJob("test-job")
	if !exists {
		t.Error("job should exist")
	}
	if job.Name != "test-job" {
		t.Errorf("job.Name = %q, want 'test-job'", job.Name)
	}

	// Add duplicate job
	err = s.AddJob("test-job", "0 * * * *", "")
	if err == nil {
		t.Error("AddJob() should error on duplicate")
	}

	// Add job with invalid schedule
	err = s.AddJob("invalid-job", "not valid", "")
	if err == nil {
		t.Error("AddJob() should error on invalid schedule")
	}
}

func TestScheduler_RemoveJob(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "")

	// Remove existing job
	err := s.RemoveJob("test-job")
	if err != nil {
		t.Errorf("RemoveJob() error = %v", err)
	}

	// Verify job no longer exists
	_, exists := s.GetJob("test-job")
	if exists {
		t.Error("job should not exist after removal")
	}

	// Remove non-existent job
	err = s.RemoveJob("non-existent")
	if err == nil {
		t.Error("RemoveJob() should error on non-existent job")
	}
}

func TestScheduler_GetJob(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "")

	// Get existing job
	job, exists := s.GetJob("test-job")
	if !exists {
		t.Error("job should exist")
	}
	if job == nil {
		t.Error("job should not be nil")
	}

	// Get non-existent job
	_, exists = s.GetJob("non-existent")
	if exists {
		t.Error("non-existent job should not exist")
	}
}

func TestScheduler_GetAllJobs(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("job1", "*/5 * * * *", "")
	s.AddJob("job2", "0 * * * *", "")
	s.AddJob("job3", "0 9 * * *", "")

	jobs := s.GetAllJobs()
	if len(jobs) != 3 {
		t.Errorf("GetAllJobs() returned %d jobs, want 3", len(jobs))
	}

	// Verify all jobs present
	for _, name := range []string{"job1", "job2", "job3"} {
		if _, exists := jobs[name]; !exists {
			t.Errorf("job %q not found", name)
		}
	}
}

func TestScheduler_StartStop(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "")

	// Start scheduler
	s.Start()
	if !s.IsStarted() {
		t.Error("scheduler should be started")
	}

	// Start again (should be idempotent)
	s.Start()
	if !s.IsStarted() {
		t.Error("scheduler should still be started")
	}

	// Stop scheduler
	ctx := s.Stop()
	if s.IsStarted() {
		t.Error("scheduler should be stopped")
	}

	// Wait for stop to complete
	<-ctx.Done()

	// Stop again (should be idempotent)
	ctx = s.Stop()
	<-ctx.Done()
}

func TestScheduler_PauseResumeJob(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "")

	// Pause job
	err := s.PauseJob("test-job")
	if err != nil {
		t.Errorf("PauseJob() error = %v", err)
	}

	job, _ := s.GetJob("test-job")
	if !job.IsPaused() {
		t.Error("job should be paused")
	}

	// Resume job
	err = s.ResumeJob("test-job")
	if err != nil {
		t.Errorf("ResumeJob() error = %v", err)
	}

	if job.IsPaused() {
		t.Error("job should not be paused")
	}

	// Pause non-existent job
	err = s.PauseJob("non-existent")
	if err == nil {
		t.Error("PauseJob() should error on non-existent job")
	}

	// Resume non-existent job
	err = s.ResumeJob("non-existent")
	if err == nil {
		t.Error("ResumeJob() should error on non-existent job")
	}
}

func TestScheduler_TriggerJob(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "")

	ctx := context.Background()

	// Trigger job
	err := s.TriggerJob(ctx, "test-job")
	if err != nil {
		t.Errorf("TriggerJob() error = %v", err)
	}

	// Wait for execution
	time.Sleep(50 * time.Millisecond)

	if executor.callCount() != 1 {
		t.Errorf("executor called %d times, want 1", executor.callCount())
	}

	// Trigger non-existent job
	err = s.TriggerJob(ctx, "non-existent")
	if err == nil {
		t.Error("TriggerJob() should error on non-existent job")
	}
}

func TestScheduler_TriggerJobSync(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "")

	ctx := context.Background()

	// Trigger and wait
	exitCode, err := s.TriggerJobSync(ctx, "test-job")
	if err != nil {
		t.Errorf("TriggerJobSync() error = %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	// Trigger non-existent job
	_, err = s.TriggerJobSync(ctx, "non-existent")
	if err == nil {
		t.Error("TriggerJobSync() should error on non-existent job")
	}
}

func TestScheduler_GetJobStatus(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "UTC")

	// Get status
	status, err := s.GetJobStatus("test-job")
	if err != nil {
		t.Errorf("GetJobStatus() error = %v", err)
	}
	if status.Name != "test-job" {
		t.Errorf("status.Name = %q, want 'test-job'", status.Name)
	}
	if status.Schedule != "*/5 * * * *" {
		t.Errorf("status.Schedule = %q, want '*/5 * * * *'", status.Schedule)
	}

	// Get status of non-existent job
	_, err = s.GetJobStatus("non-existent")
	if err == nil {
		t.Error("GetJobStatus() should error on non-existent job")
	}
}

func TestScheduler_GetAllJobStatuses(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("job1", "*/5 * * * *", "")
	s.AddJob("job2", "0 * * * *", "")

	statuses := s.GetAllJobStatuses()
	if len(statuses) != 2 {
		t.Errorf("GetAllJobStatuses() returned %d statuses, want 2", len(statuses))
	}

	if statuses["job1"].Name != "job1" {
		t.Errorf("statuses[job1].Name = %q, want 'job1'", statuses["job1"].Name)
	}
	if statuses["job2"].Name != "job2" {
		t.Errorf("statuses[job2].Name = %q, want 'job2'", statuses["job2"].Name)
	}
}

func TestScheduler_GetJobHistory(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "")

	// Execute a few times
	ctx := context.Background()
	s.TriggerJobSync(ctx, "test-job")
	s.TriggerJobSync(ctx, "test-job")
	s.TriggerJobSync(ctx, "test-job")

	// Get all history
	history, err := s.GetJobHistory("test-job", 0)
	if err != nil {
		t.Errorf("GetJobHistory() error = %v", err)
	}
	if len(history) != 3 {
		t.Errorf("GetJobHistory() returned %d entries, want 3", len(history))
	}

	// Get limited history
	history, err = s.GetJobHistory("test-job", 2)
	if err != nil {
		t.Errorf("GetJobHistory() error = %v", err)
	}
	if len(history) != 2 {
		t.Errorf("GetJobHistory(2) returned %d entries, want 2", len(history))
	}

	// Get history of non-existent job
	_, err = s.GetJobHistory("non-existent", 0)
	if err == nil {
		t.Error("GetJobHistory() should error on non-existent job")
	}
}

func TestScheduler_UpdateNextRunTimes(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "")
	s.Start()
	defer s.Stop()

	// Update next run times
	s.UpdateNextRunTimes()

	job, _ := s.GetJob("test-job")
	nextRun := job.GetNextRun()
	if nextRun.IsZero() {
		t.Error("NextRun should not be zero after UpdateNextRunTimes()")
	}
}

func TestScheduler_Stats(t *testing.T) {
	executor := &mockExecutor{delay: 100 * time.Millisecond}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("job1", "*/5 * * * *", "")
	s.AddJob("job2", "0 * * * *", "")
	s.AddJob("job3", "0 9 * * *", "")

	// Pause one job
	s.PauseJob("job2")

	// Start execution of one job
	go s.TriggerJob(context.Background(), "job1")
	time.Sleep(10 * time.Millisecond)

	stats := s.Stats()

	if stats.TotalJobs != 3 {
		t.Errorf("TotalJobs = %d, want 3", stats.TotalJobs)
	}
	if stats.IdleJobs != 1 {
		t.Errorf("IdleJobs = %d, want 1", stats.IdleJobs)
	}
	if stats.ExecutingJobs != 1 {
		t.Errorf("ExecutingJobs = %d, want 1", stats.ExecutingJobs)
	}
	if stats.PausedJobs != 1 {
		t.Errorf("PausedJobs = %d, want 1", stats.PausedJobs)
	}

	// Wait for execution to complete
	time.Sleep(150 * time.Millisecond)
}

func TestScheduler_NextRunUpdatedOnStart(t *testing.T) {
	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	s.AddJob("test-job", "*/5 * * * *", "")

	// Before start, next run may be set from AddJob
	s.Start()
	defer s.Stop()

	job, _ := s.GetJob("test-job")
	nextRun := job.GetNextRun()

	if nextRun.IsZero() {
		t.Error("NextRun should be set after Start()")
	}

	// Should be within the next 5 minutes
	maxNextRun := time.Now().Add(6 * time.Minute)
	if nextRun.After(maxNextRun) {
		t.Error("NextRun should be within next 5 minutes for */5 schedule")
	}
}

func TestScheduler_Concurrent(t *testing.T) {
	executor := &mockExecutor{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s := NewScheduler(executor, 50, logger)

	// Add jobs
	for i := 0; i < 5; i++ {
		s.AddJob("job"+string(rune('a'+i)), "*/5 * * * *", "")
	}

	s.Start()
	defer s.Stop()

	var wg sync.WaitGroup

	// Concurrent operations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ctx := context.Background()
			jobName := "job" + string(rune('a'+(n%5)))

			switch n % 4 {
			case 0:
				s.TriggerJob(ctx, jobName)
			case 1:
				s.GetJobStatus(jobName)
			case 2:
				s.GetAllJobStatuses()
			case 3:
				s.Stats()
			}
		}(i)
	}

	wg.Wait()
}

func TestScheduler_CronTriggers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cron trigger test in short mode")
	}

	executor := &mockExecutor{}
	logger := testLogger()
	s := NewScheduler(executor, 50, logger)

	// Add job that runs every minute
	// Note: This test would need to wait ~1 minute to see the trigger
	// For now, we just verify the job is added and can be triggered manually
	s.AddJob("minutely", "* * * * *", "")

	s.Start()
	defer s.Stop()

	// Verify job is in cron
	job, _ := s.GetJob("minutely")
	cronID := job.GetCronID()
	if cronID == 0 {
		t.Error("job should have cron ID assigned")
	}
}
