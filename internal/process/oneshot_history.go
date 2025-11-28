package process

import (
	"sync"
	"time"
)

// OneshotExecution represents a single oneshot process execution
type OneshotExecution struct {
	ID           int64     `json:"id"`
	ProcessName  string    `json:"process_name"`
	InstanceID   string    `json:"instance_id"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	ExitCode     int       `json:"exit_code"`
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
	Duration     string    `json:"duration,omitempty"`
	DurationMs   int64     `json:"duration_ms,omitempty"`
	TriggerType  string    `json:"trigger_type"` // "manual" | "startup" | "api"
}

// OneshotHistory stores execution history for oneshot processes
// with size and age-based eviction (whichever is hit first)
type OneshotHistory struct {
	maxEntries int
	maxAge     time.Duration
	entries    []OneshotExecution
	nextID     int64
	mu         sync.RWMutex
}

// NewOneshotHistory creates a new oneshot history store
func NewOneshotHistory(maxEntries int, maxAge time.Duration) *OneshotHistory {
	if maxEntries <= 0 {
		maxEntries = 5000
	}
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	return &OneshotHistory{
		maxEntries: maxEntries,
		maxAge:     maxAge,
		entries:    make([]OneshotExecution, 0, 100), // Start with reasonable capacity
		nextID:     1,
	}
}

// Record adds a new execution entry and returns its ID
func (h *OneshotHistory) Record(processName, instanceID, triggerType string) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := h.nextID
	h.nextID++

	entry := OneshotExecution{
		ID:          id,
		ProcessName: processName,
		InstanceID:  instanceID,
		StartedAt:   time.Now(),
		TriggerType: triggerType,
	}

	h.entries = append(h.entries, entry)
	h.evict()

	return id
}

// Complete marks an execution as finished
func (h *OneshotHistory) Complete(id int64, exitCode int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := len(h.entries) - 1; i >= 0; i-- {
		if h.entries[i].ID == id {
			h.entries[i].FinishedAt = time.Now()
			h.entries[i].ExitCode = exitCode
			h.entries[i].Success = exitCode == 0 && err == nil
			if err != nil {
				h.entries[i].Error = err.Error()
			}
			duration := h.entries[i].FinishedAt.Sub(h.entries[i].StartedAt)
			h.entries[i].Duration = formatDuration(duration)
			h.entries[i].DurationMs = duration.Milliseconds()
			return
		}
	}
}

// GetAll returns all entries for a process (newest first)
func (h *OneshotHistory) GetAll(processName string) []OneshotExecution {
	h.mu.RLock()
	defer h.mu.RUnlock()

	h.evictReadLocked()

	var result []OneshotExecution
	for i := len(h.entries) - 1; i >= 0; i-- {
		if h.entries[i].ProcessName == processName {
			result = append(result, h.entries[i])
		}
	}
	return result
}

// GetRecent returns the most recent N entries for a process (newest first)
func (h *OneshotHistory) GetRecent(processName string, limit int) []OneshotExecution {
	h.mu.RLock()
	defer h.mu.RUnlock()

	h.evictReadLocked()

	var result []OneshotExecution
	for i := len(h.entries) - 1; i >= 0 && len(result) < limit; i-- {
		if h.entries[i].ProcessName == processName {
			result = append(result, h.entries[i])
		}
	}
	return result
}

// GetAllProcesses returns all entries across all processes (newest first)
func (h *OneshotHistory) GetAllProcesses() []OneshotExecution {
	h.mu.RLock()
	defer h.mu.RUnlock()

	h.evictReadLocked()

	result := make([]OneshotExecution, len(h.entries))
	for i, j := len(h.entries)-1, 0; i >= 0; i, j = i-1, j+1 {
		result[j] = h.entries[i]
	}
	return result
}

// GetRecentAll returns the most recent N entries across all processes (newest first)
func (h *OneshotHistory) GetRecentAll(limit int) []OneshotExecution {
	h.mu.RLock()
	defer h.mu.RUnlock()

	h.evictReadLocked()

	count := len(h.entries)
	if count > limit {
		count = limit
	}

	result := make([]OneshotExecution, count)
	for i, j := len(h.entries)-1, 0; j < count; i, j = i-1, j+1 {
		result[j] = h.entries[i]
	}
	return result
}

// Stats returns aggregate statistics
func (h *OneshotHistory) Stats() OneshotHistoryStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	h.evictReadLocked()

	stats := OneshotHistoryStats{
		TotalEntries: len(h.entries),
		ByProcess:    make(map[string]OneshotProcessStats),
	}

	for _, e := range h.entries {
		ps := stats.ByProcess[e.ProcessName]
		ps.Total++
		if e.Success {
			ps.Successful++
		} else if !e.FinishedAt.IsZero() {
			ps.Failed++
		} else {
			ps.Running++
		}
		stats.ByProcess[e.ProcessName] = ps
	}

	return stats
}

// OneshotHistoryStats contains aggregate statistics
type OneshotHistoryStats struct {
	TotalEntries int                            `json:"total_entries"`
	ByProcess    map[string]OneshotProcessStats `json:"by_process"`
}

// OneshotProcessStats contains per-process statistics
type OneshotProcessStats struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
	Running    int `json:"running"`
}

// evict removes entries that exceed size or age limits (must hold write lock)
func (h *OneshotHistory) evict() {
	now := time.Now()
	cutoff := now.Add(-h.maxAge)

	// Find first entry within age limit
	ageStart := 0
	for i, e := range h.entries {
		if e.StartedAt.After(cutoff) {
			ageStart = i
			break
		}
		ageStart = i + 1
	}

	// Calculate size-based start
	sizeStart := 0
	if len(h.entries) > h.maxEntries {
		sizeStart = len(h.entries) - h.maxEntries
	}

	// Use whichever removes more entries
	start := ageStart
	if sizeStart > start {
		start = sizeStart
	}

	if start > 0 {
		h.entries = h.entries[start:]
	}
}

// evictReadLocked performs eviction check during read (deferred cleanup)
// Note: This is called with read lock held, so it doesn't actually evict
// but marks that eviction is needed. The actual eviction happens on next write.
func (h *OneshotHistory) evictReadLocked() {
	// For read operations, we just return current data
	// Eviction will happen on next write operation
	// This avoids upgrading read lock to write lock
}

// formatDuration formats duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return "< 1ms"
	}
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	if d < time.Minute {
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}
