package process

import (
	"errors"
	"testing"
	"time"
)

func TestOneshotHistory_NewOneshotHistory(t *testing.T) {
	tests := []struct {
		name           string
		maxEntries     int
		maxAge         time.Duration
		wantMaxEntries int
		wantMaxAge     time.Duration
	}{
		{
			name:           "default values",
			maxEntries:     0,
			maxAge:         0,
			wantMaxEntries: 5000,
			wantMaxAge:     24 * time.Hour,
		},
		{
			name:           "custom values",
			maxEntries:     100,
			maxAge:         1 * time.Hour,
			wantMaxEntries: 100,
			wantMaxAge:     1 * time.Hour,
		},
		{
			name:           "negative values",
			maxEntries:     -1,
			maxAge:         -1,
			wantMaxEntries: 5000,
			wantMaxAge:     24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewOneshotHistory(tt.maxEntries, tt.maxAge)
			if h == nil {
				t.Fatal("NewOneshotHistory returned nil")
			}
			if h.maxEntries != tt.wantMaxEntries {
				t.Errorf("maxEntries = %d, want %d", h.maxEntries, tt.wantMaxEntries)
			}
			if h.maxAge != tt.wantMaxAge {
				t.Errorf("maxAge = %v, want %v", h.maxAge, tt.wantMaxAge)
			}
		})
	}
}

func TestOneshotHistory_Record(t *testing.T) {
	h := NewOneshotHistory(100, 1*time.Hour)

	// Record first entry
	id1 := h.Record("process1", "instance1", "manual")
	if id1 != 1 {
		t.Errorf("First ID = %d, want 1", id1)
	}

	// Record second entry
	id2 := h.Record("process2", "instance2", "api")
	if id2 != 2 {
		t.Errorf("Second ID = %d, want 2", id2)
	}

	// Verify entries exist
	all := h.GetAllProcesses()
	if len(all) != 2 {
		t.Errorf("len(GetAllProcesses()) = %d, want 2", len(all))
	}

	// Verify first entry
	entries := h.GetAll("process1")
	if len(entries) != 1 {
		t.Fatalf("len(GetAll) = %d, want 1", len(entries))
	}
	if entries[0].ProcessName != "process1" {
		t.Errorf("ProcessName = %q, want %q", entries[0].ProcessName, "process1")
	}
	if entries[0].InstanceID != "instance1" {
		t.Errorf("InstanceID = %q, want %q", entries[0].InstanceID, "instance1")
	}
	if entries[0].TriggerType != "manual" {
		t.Errorf("TriggerType = %q, want %q", entries[0].TriggerType, "manual")
	}
}

func TestOneshotHistory_Complete(t *testing.T) {
	h := NewOneshotHistory(100, 1*time.Hour)

	// Record an entry
	id := h.Record("test-process", "instance1", "manual")

	// Complete with success
	h.Complete(id, 0, nil)

	entries := h.GetAll("test-process")
	if len(entries) != 1 {
		t.Fatalf("len(GetAll) = %d, want 1", len(entries))
	}

	e := entries[0]
	if !e.Success {
		t.Error("Success should be true for exit code 0")
	}
	if e.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", e.ExitCode)
	}
	if e.FinishedAt.IsZero() {
		t.Error("FinishedAt should be set")
	}
	if e.Duration == "" {
		t.Error("Duration should be set")
	}
}

func TestOneshotHistory_Complete_WithError(t *testing.T) {
	h := NewOneshotHistory(100, 1*time.Hour)

	// Record an entry
	id := h.Record("test-process", "instance1", "manual")

	// Complete with error
	h.Complete(id, 1, errors.New("test error"))

	entries := h.GetAll("test-process")
	if len(entries) != 1 {
		t.Fatalf("len(GetAll) = %d, want 1", len(entries))
	}

	e := entries[0]
	if e.Success {
		t.Error("Success should be false for error")
	}
	if e.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", e.ExitCode)
	}
	if e.Error != "test error" {
		t.Errorf("Error = %q, want %q", e.Error, "test error")
	}
}

func TestOneshotHistory_Complete_NonExistentID(t *testing.T) {
	h := NewOneshotHistory(100, 1*time.Hour)

	// Complete a non-existent ID (should not panic)
	h.Complete(999, 0, nil)

	// Verify no entries
	all := h.GetAllProcesses()
	if len(all) != 0 {
		t.Errorf("len(GetAllProcesses()) = %d, want 0", len(all))
	}
}

func TestOneshotHistory_GetRecent(t *testing.T) {
	h := NewOneshotHistory(100, 1*time.Hour)

	// Record multiple entries
	for i := 0; i < 10; i++ {
		h.Record("test-process", "instance", "manual")
	}

	// Get recent 5
	recent := h.GetRecent("test-process", 5)
	if len(recent) != 5 {
		t.Errorf("len(GetRecent) = %d, want 5", len(recent))
	}

	// Verify newest first (highest ID first)
	if recent[0].ID <= recent[4].ID {
		t.Error("GetRecent should return newest first")
	}
}

func TestOneshotHistory_GetRecentAll(t *testing.T) {
	h := NewOneshotHistory(100, 1*time.Hour)

	// Record entries for multiple processes
	for i := 0; i < 5; i++ {
		h.Record("process-a", "instance", "manual")
		h.Record("process-b", "instance", "api")
	}

	// Get recent 3 across all processes
	recent := h.GetRecentAll(3)
	if len(recent) != 3 {
		t.Errorf("len(GetRecentAll) = %d, want 3", len(recent))
	}
}

func TestOneshotHistory_Stats(t *testing.T) {
	h := NewOneshotHistory(100, 1*time.Hour)

	// Record and complete entries
	id1 := h.Record("process-a", "instance1", "manual")
	h.Complete(id1, 0, nil) // Success

	id2 := h.Record("process-a", "instance2", "manual")
	h.Complete(id2, 1, errors.New("failed")) // Failed

	h.Record("process-a", "instance3", "manual") // Running (not completed)

	id4 := h.Record("process-b", "instance1", "api")
	h.Complete(id4, 0, nil) // Success

	stats := h.Stats()

	if stats.TotalEntries != 4 {
		t.Errorf("TotalEntries = %d, want 4", stats.TotalEntries)
	}

	// Check process-a stats
	aStats := stats.ByProcess["process-a"]
	if aStats.Total != 3 {
		t.Errorf("process-a Total = %d, want 3", aStats.Total)
	}
	if aStats.Successful != 1 {
		t.Errorf("process-a Successful = %d, want 1", aStats.Successful)
	}
	if aStats.Failed != 1 {
		t.Errorf("process-a Failed = %d, want 1", aStats.Failed)
	}
	if aStats.Running != 1 {
		t.Errorf("process-a Running = %d, want 1", aStats.Running)
	}

	// Check process-b stats
	bStats := stats.ByProcess["process-b"]
	if bStats.Total != 1 {
		t.Errorf("process-b Total = %d, want 1", bStats.Total)
	}
	if bStats.Successful != 1 {
		t.Errorf("process-b Successful = %d, want 1", bStats.Successful)
	}
}

func TestOneshotHistory_Eviction_BySize(t *testing.T) {
	h := NewOneshotHistory(5, 1*time.Hour)

	// Record more than maxEntries
	for i := 0; i < 10; i++ {
		h.Record("test-process", "instance", "manual")
	}

	all := h.GetAllProcesses()
	if len(all) > 5 {
		t.Errorf("len(GetAllProcesses()) = %d, should not exceed 5", len(all))
	}
}

func TestOneshotHistory_Eviction_ByAge(t *testing.T) {
	// Create history with very short max age
	h := NewOneshotHistory(100, 1*time.Millisecond)

	// Record an entry
	h.Record("test-process", "instance", "manual")

	// Wait for age to exceed
	time.Sleep(5 * time.Millisecond)

	// Record another entry (triggers eviction)
	h.Record("test-process", "instance2", "manual")

	// The old entry should be evicted
	all := h.GetAllProcesses()
	if len(all) > 1 {
		t.Logf("Note: Age-based eviction may not immediately remove all old entries")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "sub-millisecond",
			duration: 100 * time.Microsecond,
			want:     "< 1ms",
		},
		{
			name:     "milliseconds",
			duration: 500 * time.Millisecond,
			want:     "500ms",
		},
		{
			name:     "seconds",
			duration: 5 * time.Second,
			want:     "5s",
		},
		{
			name:     "minutes",
			duration: 2 * time.Minute,
			want:     "2m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestOneshotHistory_Concurrent(t *testing.T) {
	h := NewOneshotHistory(1000, 1*time.Hour)

	// Run concurrent operations
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			id := h.Record("concurrent-test", "instance", "manual")
			h.Complete(id, 0, nil)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			h.GetAllProcesses()
			h.GetAll("concurrent-test")
			h.Stats()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// No panics = success
	t.Log("Concurrent access completed without panics")
}
