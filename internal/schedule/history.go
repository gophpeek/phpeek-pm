package schedule

import (
	"sync"
	"time"
)

// ExecutionEntry represents a single execution of a scheduled job
type ExecutionEntry struct {
	ID        int64     `json:"id"`         // Unique execution ID
	StartTime time.Time `json:"start_time"` // When execution started
	EndTime   time.Time `json:"end_time"`   // When execution ended (zero if still running)
	ExitCode  int       `json:"exit_code"`  // Process exit code
	Success   bool      `json:"success"`    // Whether execution was successful
	Error     string    `json:"error"`      // Error message if any
	Triggered string    `json:"triggered"`  // How it was triggered: "schedule", "manual", "api"
}

// Duration returns the execution duration
func (e *ExecutionEntry) Duration() time.Duration {
	if e.EndTime.IsZero() {
		return time.Since(e.StartTime)
	}
	return e.EndTime.Sub(e.StartTime)
}

// IsRunning returns true if the execution is still in progress
func (e *ExecutionEntry) IsRunning() bool {
	return e.EndTime.IsZero()
}

// ExecutionHistory is a thread-safe ring buffer for execution history
type ExecutionHistory struct {
	entries []ExecutionEntry
	maxSize int
	nextID  int64
	mu      sync.RWMutex
}

// NewExecutionHistory creates a new ExecutionHistory with the given max size
func NewExecutionHistory(maxSize int) *ExecutionHistory {
	if maxSize <= 0 {
		maxSize = 100 // Default
	}
	return &ExecutionHistory{
		entries: make([]ExecutionEntry, 0, maxSize),
		maxSize: maxSize,
		nextID:  1,
	}
}

// StartExecution records the start of a new execution and returns its ID
func (h *ExecutionHistory) StartExecution(triggered string) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry := ExecutionEntry{
		ID:        h.nextID,
		StartTime: time.Now(),
		Triggered: triggered,
	}
	h.nextID++

	// Add to ring buffer
	if len(h.entries) >= h.maxSize {
		// Remove oldest entry
		h.entries = h.entries[1:]
	}
	h.entries = append(h.entries, entry)

	return entry.ID
}

// EndExecution records the end of an execution
func (h *ExecutionHistory) EndExecution(id int64, exitCode int, success bool, errMsg string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find the entry by ID (search from end since it's likely recent)
	for i := len(h.entries) - 1; i >= 0; i-- {
		if h.entries[i].ID == id {
			h.entries[i].EndTime = time.Now()
			h.entries[i].ExitCode = exitCode
			h.entries[i].Success = success
			h.entries[i].Error = errMsg
			return
		}
	}
}

// GetAll returns all entries, newest first
func (h *ExecutionHistory) GetAll() []ExecutionEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]ExecutionEntry, len(h.entries))
	// Reverse order (newest first)
	for i, entry := range h.entries {
		result[len(h.entries)-1-i] = entry
	}
	return result
}

// GetRecent returns the N most recent entries, newest first
func (h *ExecutionHistory) GetRecent(n int) []ExecutionEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n <= 0 || n > len(h.entries) {
		n = len(h.entries)
	}

	result := make([]ExecutionEntry, n)
	// Get the last n entries in reverse order
	for i := 0; i < n; i++ {
		result[i] = h.entries[len(h.entries)-1-i]
	}
	return result
}

// GetByID returns a specific execution entry by ID
func (h *ExecutionHistory) GetByID(id int64) (ExecutionEntry, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := len(h.entries) - 1; i >= 0; i-- {
		if h.entries[i].ID == id {
			return h.entries[i], true
		}
	}
	return ExecutionEntry{}, false
}

// GetLast returns the most recent entry
func (h *ExecutionHistory) GetLast() (ExecutionEntry, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.entries) == 0 {
		return ExecutionEntry{}, false
	}
	return h.entries[len(h.entries)-1], true
}

// Len returns the number of entries
func (h *ExecutionHistory) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.entries)
}

// SuccessRate returns the success rate as a percentage (0-100)
func (h *ExecutionHistory) SuccessRate() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.entries) == 0 {
		return 0
	}

	var successes int
	var completed int
	for _, entry := range h.entries {
		if !entry.EndTime.IsZero() {
			completed++
			if entry.Success {
				successes++
			}
		}
	}

	if completed == 0 {
		return 0
	}
	return float64(successes) / float64(completed) * 100
}

// Stats returns aggregate statistics about execution history
type HistoryStats struct {
	TotalExecutions   int           `json:"total_executions"`
	SuccessCount      int           `json:"success_count"`
	FailureCount      int           `json:"failure_count"`
	RunningCount      int           `json:"running_count"`
	SuccessRate       float64       `json:"success_rate"`
	AverageDuration   time.Duration `json:"average_duration"`
	LastExecutionTime time.Time     `json:"last_execution_time"`
	LastSuccessTime   time.Time     `json:"last_success_time"`
	LastFailureTime   time.Time     `json:"last_failure_time"`
}

// Stats returns aggregate statistics
func (h *ExecutionHistory) Stats() HistoryStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := HistoryStats{}
	var totalDuration time.Duration

	for _, entry := range h.entries {
		stats.TotalExecutions++

		if entry.EndTime.IsZero() {
			stats.RunningCount++
			continue
		}

		totalDuration += entry.Duration()

		if entry.Success {
			stats.SuccessCount++
			if entry.EndTime.After(stats.LastSuccessTime) {
				stats.LastSuccessTime = entry.EndTime
			}
		} else {
			stats.FailureCount++
			if entry.EndTime.After(stats.LastFailureTime) {
				stats.LastFailureTime = entry.EndTime
			}
		}

		if entry.StartTime.After(stats.LastExecutionTime) {
			stats.LastExecutionTime = entry.StartTime
		}
	}

	completed := stats.SuccessCount + stats.FailureCount
	if completed > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(completed) * 100
		stats.AverageDuration = totalDuration / time.Duration(completed)
	}

	return stats
}
