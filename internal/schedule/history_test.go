package schedule

import (
	"sync"
	"testing"
	"time"
)

func TestNewExecutionHistory(t *testing.T) {
	tests := []struct {
		name        string
		maxSize     int
		wantMaxSize int
	}{
		{
			name:        "positive max size",
			maxSize:     50,
			wantMaxSize: 50,
		},
		{
			name:        "zero defaults to 100",
			maxSize:     0,
			wantMaxSize: 100,
		},
		{
			name:        "negative defaults to 100",
			maxSize:     -1,
			wantMaxSize: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewExecutionHistory(tt.maxSize)
			if h == nil {
				t.Fatal("expected non-nil history")
			}
			if h.maxSize != tt.wantMaxSize {
				t.Errorf("maxSize = %d, want %d", h.maxSize, tt.wantMaxSize)
			}
			if h.nextID != 1 {
				t.Errorf("nextID = %d, want 1", h.nextID)
			}
			if h.Len() != 0 {
				t.Errorf("Len() = %d, want 0", h.Len())
			}
		})
	}
}

func TestExecutionHistory_StartExecution(t *testing.T) {
	h := NewExecutionHistory(10)

	// Start first execution
	id1 := h.StartExecution("schedule")
	if id1 != 1 {
		t.Errorf("first id = %d, want 1", id1)
	}

	// Verify entry exists
	entry, ok := h.GetByID(id1)
	if !ok {
		t.Fatal("entry not found")
	}
	if entry.Triggered != "schedule" {
		t.Errorf("Triggered = %q, want 'schedule'", entry.Triggered)
	}
	if entry.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}
	if !entry.IsRunning() {
		t.Error("entry should be running")
	}

	// Start second execution
	id2 := h.StartExecution("manual")
	if id2 != 2 {
		t.Errorf("second id = %d, want 2", id2)
	}

	if h.Len() != 2 {
		t.Errorf("Len() = %d, want 2", h.Len())
	}
}

func TestExecutionHistory_EndExecution(t *testing.T) {
	h := NewExecutionHistory(10)

	id := h.StartExecution("schedule")

	// End the execution
	h.EndExecution(id, 0, true, "")

	entry, ok := h.GetByID(id)
	if !ok {
		t.Fatal("entry not found")
	}
	if entry.EndTime.IsZero() {
		t.Error("EndTime should not be zero")
	}
	if entry.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", entry.ExitCode)
	}
	if !entry.Success {
		t.Error("Success should be true")
	}
	if entry.Error != "" {
		t.Errorf("Error = %q, want empty", entry.Error)
	}
	if entry.IsRunning() {
		t.Error("entry should not be running")
	}
}

func TestExecutionHistory_EndExecution_WithError(t *testing.T) {
	h := NewExecutionHistory(10)

	id := h.StartExecution("schedule")
	h.EndExecution(id, 1, false, "command failed")

	entry, _ := h.GetByID(id)
	if entry.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", entry.ExitCode)
	}
	if entry.Success {
		t.Error("Success should be false")
	}
	if entry.Error != "command failed" {
		t.Errorf("Error = %q, want 'command failed'", entry.Error)
	}
}

func TestExecutionHistory_EndExecution_NotFound(t *testing.T) {
	h := NewExecutionHistory(10)

	// Should not panic when entry doesn't exist
	h.EndExecution(999, 0, true, "")
}

func TestExecutionHistory_RingBuffer(t *testing.T) {
	h := NewExecutionHistory(3)

	// Add 5 entries, should keep only last 3
	for i := 0; i < 5; i++ {
		h.StartExecution("schedule")
	}

	if h.Len() != 3 {
		t.Errorf("Len() = %d, want 3", h.Len())
	}

	// Entry IDs 1 and 2 should have been removed
	_, ok1 := h.GetByID(1)
	_, ok2 := h.GetByID(2)
	if ok1 {
		t.Error("entry 1 should have been removed")
	}
	if ok2 {
		t.Error("entry 2 should have been removed")
	}

	// Entry IDs 3, 4, 5 should exist
	_, ok3 := h.GetByID(3)
	_, ok4 := h.GetByID(4)
	_, ok5 := h.GetByID(5)
	if !ok3 || !ok4 || !ok5 {
		t.Error("entries 3, 4, 5 should exist")
	}
}

func TestExecutionHistory_GetAll(t *testing.T) {
	h := NewExecutionHistory(10)

	h.StartExecution("schedule")
	h.StartExecution("manual")
	h.StartExecution("api")

	entries := h.GetAll()
	if len(entries) != 3 {
		t.Fatalf("GetAll() returned %d entries, want 3", len(entries))
	}

	// Should be in reverse order (newest first)
	if entries[0].ID != 3 {
		t.Errorf("entries[0].ID = %d, want 3", entries[0].ID)
	}
	if entries[1].ID != 2 {
		t.Errorf("entries[1].ID = %d, want 2", entries[1].ID)
	}
	if entries[2].ID != 1 {
		t.Errorf("entries[2].ID = %d, want 1", entries[2].ID)
	}
}

func TestExecutionHistory_GetRecent(t *testing.T) {
	h := NewExecutionHistory(10)

	for i := 0; i < 5; i++ {
		h.StartExecution("schedule")
	}

	// Get 2 most recent
	recent := h.GetRecent(2)
	if len(recent) != 2 {
		t.Fatalf("GetRecent(2) returned %d entries, want 2", len(recent))
	}
	if recent[0].ID != 5 {
		t.Errorf("recent[0].ID = %d, want 5", recent[0].ID)
	}
	if recent[1].ID != 4 {
		t.Errorf("recent[1].ID = %d, want 4", recent[1].ID)
	}

	// Request more than available
	all := h.GetRecent(10)
	if len(all) != 5 {
		t.Errorf("GetRecent(10) returned %d entries, want 5", len(all))
	}

	// Zero or negative returns all
	allZero := h.GetRecent(0)
	if len(allZero) != 5 {
		t.Errorf("GetRecent(0) returned %d entries, want 5", len(allZero))
	}
}

func TestExecutionHistory_GetLast(t *testing.T) {
	h := NewExecutionHistory(10)

	// Empty history
	_, ok := h.GetLast()
	if ok {
		t.Error("GetLast() should return false for empty history")
	}

	h.StartExecution("schedule")
	h.StartExecution("manual")

	last, ok := h.GetLast()
	if !ok {
		t.Error("GetLast() should return true")
	}
	if last.ID != 2 {
		t.Errorf("last.ID = %d, want 2", last.ID)
	}
}

func TestExecutionHistory_SuccessRate(t *testing.T) {
	h := NewExecutionHistory(10)

	// Empty history
	if rate := h.SuccessRate(); rate != 0 {
		t.Errorf("SuccessRate() = %f, want 0 for empty history", rate)
	}

	// Add entries
	id1 := h.StartExecution("schedule")
	h.EndExecution(id1, 0, true, "")

	id2 := h.StartExecution("schedule")
	h.EndExecution(id2, 1, false, "error")

	id3 := h.StartExecution("schedule")
	h.EndExecution(id3, 0, true, "")

	// Running entry (should not count)
	h.StartExecution("schedule")

	rate := h.SuccessRate()
	// Expected: (2.0 / 3.0) * 100 = 66.67%
	if rate < 66.0 || rate > 67.0 {
		t.Errorf("SuccessRate() = %f, want ~66.67%%", rate)
	}

	// All running - no completed
	h2 := NewExecutionHistory(10)
	h2.StartExecution("schedule")
	if rate := h2.SuccessRate(); rate != 0 {
		t.Errorf("SuccessRate() = %f, want 0 when no completed entries", rate)
	}
}

func TestExecutionHistory_Stats(t *testing.T) {
	h := NewExecutionHistory(10)

	// Add some entries
	id1 := h.StartExecution("schedule")
	time.Sleep(10 * time.Millisecond)
	h.EndExecution(id1, 0, true, "")

	id2 := h.StartExecution("manual")
	time.Sleep(10 * time.Millisecond)
	h.EndExecution(id2, 1, false, "error")

	// One still running
	h.StartExecution("api")

	stats := h.Stats()

	if stats.TotalExecutions != 3 {
		t.Errorf("TotalExecutions = %d, want 3", stats.TotalExecutions)
	}
	if stats.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", stats.SuccessCount)
	}
	if stats.FailureCount != 1 {
		t.Errorf("FailureCount = %d, want 1", stats.FailureCount)
	}
	if stats.RunningCount != 1 {
		t.Errorf("RunningCount = %d, want 1", stats.RunningCount)
	}
	if stats.SuccessRate != 50.0 {
		t.Errorf("SuccessRate = %f, want 50.0", stats.SuccessRate)
	}
	if stats.AverageDuration <= 0 {
		t.Error("AverageDuration should be positive")
	}
	if stats.LastExecutionTime.IsZero() {
		t.Error("LastExecutionTime should not be zero")
	}
	if stats.LastSuccessTime.IsZero() {
		t.Error("LastSuccessTime should not be zero")
	}
	if stats.LastFailureTime.IsZero() {
		t.Error("LastFailureTime should not be zero")
	}
}

func TestExecutionEntry_Duration(t *testing.T) {
	// Completed entry
	start := time.Now()
	end := start.Add(5 * time.Second)
	entry := ExecutionEntry{
		StartTime: start,
		EndTime:   end,
	}
	if dur := entry.Duration(); dur != 5*time.Second {
		t.Errorf("Duration() = %v, want 5s", dur)
	}

	// Running entry
	runningEntry := ExecutionEntry{
		StartTime: time.Now().Add(-1 * time.Second),
	}
	if dur := runningEntry.Duration(); dur < 1*time.Second {
		t.Errorf("Duration() for running entry = %v, want >= 1s", dur)
	}
}

func TestExecutionHistory_Concurrent(t *testing.T) {
	h := NewExecutionHistory(100)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := h.StartExecution("schedule")
			time.Sleep(time.Millisecond)
			h.EndExecution(id, 0, true, "")
		}()
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.GetAll()
			h.GetRecent(5)
			h.SuccessRate()
			h.Stats()
			h.Len()
		}()
	}

	wg.Wait()

	if h.Len() != 50 {
		t.Errorf("Len() = %d, want 50", h.Len())
	}
}
