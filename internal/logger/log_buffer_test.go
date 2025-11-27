package logger

import (
	"testing"
	"time"
)

func TestNewLogBuffer_DefaultSize(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		wantSize int
	}{
		{
			name:     "negative size defaults to 1000",
			size:     -1,
			wantSize: 1000,
		},
		{
			name:     "zero size defaults to 1000",
			size:     0,
			wantSize: 1000,
		},
		{
			name:     "positive size",
			size:     500,
			wantSize: 500,
		},
		{
			name:     "large size",
			size:     10000,
			wantSize: 10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := NewLogBuffer(tt.size)
			if buffer.size != tt.wantSize {
				t.Errorf("NewLogBuffer(%d) size = %d, want %d", tt.size, buffer.size, tt.wantSize)
			}
			if buffer.index != 0 {
				t.Errorf("initial index should be 0, got %d", buffer.index)
			}
			if buffer.full {
				t.Error("initial full should be false")
			}
		})
	}
}

func TestLogBuffer_Add_SingleEntry(t *testing.T) {
	buffer := NewLogBuffer(10)

	entry := LogEntry{
		Timestamp:   time.Now(),
		ProcessName: "test-process",
		InstanceID:  "test-0",
		Stream:      "stdout",
		Message:     "test message",
		Level:       "info",
	}

	buffer.Add(entry)

	if buffer.index != 1 {
		t.Errorf("index should be 1, got %d", buffer.index)
	}
	if buffer.full {
		t.Error("buffer should not be full after 1 entry")
	}
	if buffer.Size() != 1 {
		t.Errorf("Size() should be 1, got %d", buffer.Size())
	}
}

func TestLogBuffer_Add_MultipleEntries(t *testing.T) {
	buffer := NewLogBuffer(5)

	for i := 0; i < 3; i++ {
		entry := LogEntry{
			Timestamp:   time.Now(),
			ProcessName: "test-process",
			InstanceID:  "test-0",
			Stream:      "stdout",
			Message:     "message",
			Level:       "info",
		}
		buffer.Add(entry)
	}

	if buffer.index != 3 {
		t.Errorf("index should be 3, got %d", buffer.index)
	}
	if buffer.full {
		t.Error("buffer should not be full")
	}
	if buffer.Size() != 3 {
		t.Errorf("Size() should be 3, got %d", buffer.Size())
	}
}

func TestLogBuffer_Add_WrapsAround(t *testing.T) {
	buffer := NewLogBuffer(3)

	// Add 5 entries (more than capacity)
	for i := 0; i < 5; i++ {
		entry := LogEntry{
			Timestamp:   time.Now(),
			ProcessName: "test-process",
			Message:     "message",
		}
		buffer.Add(entry)
	}

	// After 5 entries in buffer of size 3:
	// index should be 2 (wrapped around: 0,1,2,0,1)
	if buffer.index != 2 {
		t.Errorf("index should be 2, got %d", buffer.index)
	}
	if !buffer.full {
		t.Error("buffer should be full")
	}
	if buffer.Size() != 3 {
		t.Errorf("Size() should be 3, got %d", buffer.Size())
	}
}

func TestLogBuffer_GetAll_NotFull(t *testing.T) {
	buffer := NewLogBuffer(10)

	entries := []LogEntry{
		{Message: "message 1", Level: "info"},
		{Message: "message 2", Level: "warn"},
		{Message: "message 3", Level: "error"},
	}

	for _, entry := range entries {
		buffer.Add(entry)
	}

	result := buffer.GetAll()

	if len(result) != 3 {
		t.Errorf("GetAll() should return 3 entries, got %d", len(result))
	}

	for i, entry := range result {
		if entry.Message != entries[i].Message {
			t.Errorf("entry[%d] message = %v, want %v", i, entry.Message, entries[i].Message)
		}
	}
}

func TestLogBuffer_GetAll_Full_ChronologicalOrder(t *testing.T) {
	buffer := NewLogBuffer(3)

	// Add 5 entries (wraps around)
	messages := []string{"msg1", "msg2", "msg3", "msg4", "msg5"}
	for _, msg := range messages {
		buffer.Add(LogEntry{Message: msg})
	}

	result := buffer.GetAll()

	if len(result) != 3 {
		t.Errorf("GetAll() should return 3 entries, got %d", len(result))
	}

	// After adding 5 entries to buffer of size 3:
	// Oldest entries are msg3, msg4, msg5 (in chronological order)
	expected := []string{"msg3", "msg4", "msg5"}
	for i, entry := range result {
		if entry.Message != expected[i] {
			t.Errorf("entry[%d] message = %v, want %v", i, entry.Message, expected[i])
		}
	}
}

func TestLogBuffer_GetRecent_NotFull(t *testing.T) {
	buffer := NewLogBuffer(10)

	messages := []string{"msg1", "msg2", "msg3", "msg4", "msg5"}
	for _, msg := range messages {
		buffer.Add(LogEntry{Message: msg})
	}

	tests := []struct {
		name     string
		n        int
		expected []string
	}{
		{
			name:     "get last 2",
			n:        2,
			expected: []string{"msg4", "msg5"},
		},
		{
			name:     "get last 3",
			n:        3,
			expected: []string{"msg3", "msg4", "msg5"},
		},
		{
			name:     "get more than available",
			n:        10,
			expected: []string{"msg1", "msg2", "msg3", "msg4", "msg5"},
		},
		{
			name:     "get all",
			n:        5,
			expected: []string{"msg1", "msg2", "msg3", "msg4", "msg5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buffer.GetRecent(tt.n)
			if len(result) != len(tt.expected) {
				t.Errorf("GetRecent(%d) returned %d entries, want %d", tt.n, len(result), len(tt.expected))
			}
			for i, entry := range result {
				if entry.Message != tt.expected[i] {
					t.Errorf("entry[%d] message = %v, want %v", i, entry.Message, tt.expected[i])
				}
			}
		})
	}
}

func TestLogBuffer_GetRecent_Full(t *testing.T) {
	buffer := NewLogBuffer(4)

	// Add 6 entries (wraps around twice)
	messages := []string{"msg1", "msg2", "msg3", "msg4", "msg5", "msg6"}
	for _, msg := range messages {
		buffer.Add(LogEntry{Message: msg})
	}

	// Buffer contains: msg3, msg4, msg5, msg6 (oldest to newest)
	// index is at 2 (after msg6, wraps to position 2)

	tests := []struct {
		name     string
		n        int
		expected []string
	}{
		{
			name:     "get last 1",
			n:        1,
			expected: []string{"msg6"},
		},
		{
			name:     "get last 2",
			n:        2,
			expected: []string{"msg5", "msg6"},
		},
		{
			name:     "get last 3",
			n:        3,
			expected: []string{"msg4", "msg5", "msg6"},
		},
		{
			name:     "get all",
			n:        4,
			expected: []string{"msg3", "msg4", "msg5", "msg6"},
		},
		{
			name:     "get more than size",
			n:        10,
			expected: []string{"msg3", "msg4", "msg5", "msg6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buffer.GetRecent(tt.n)
			if len(result) != len(tt.expected) {
				t.Errorf("GetRecent(%d) returned %d entries, want %d", tt.n, len(result), len(tt.expected))
			}
			for i, entry := range result {
				if entry.Message != tt.expected[i] {
					t.Errorf("entry[%d] message = %v, want %v", i, entry.Message, tt.expected[i])
				}
			}
		})
	}
}

func TestLogBuffer_GetRecent_Full_WrapAround(t *testing.T) {
	buffer := NewLogBuffer(5)

	// Add 8 entries
	messages := []string{"msg1", "msg2", "msg3", "msg4", "msg5", "msg6", "msg7", "msg8"}
	for _, msg := range messages {
		buffer.Add(LogEntry{Message: msg})
	}

	// Buffer contains: msg4, msg5, msg6, msg7, msg8
	// index is at 3 (after msg8)

	// Get last 4 (should wrap around)
	result := buffer.GetRecent(4)
	expected := []string{"msg5", "msg6", "msg7", "msg8"}

	if len(result) != 4 {
		t.Errorf("GetRecent(4) returned %d entries, want 4", len(result))
	}

	for i, entry := range result {
		if entry.Message != expected[i] {
			t.Errorf("entry[%d] message = %v, want %v", i, entry.Message, expected[i])
		}
	}
}

func TestLogBuffer_Clear(t *testing.T) {
	buffer := NewLogBuffer(10)

	// Add some entries
	for i := 0; i < 5; i++ {
		buffer.Add(LogEntry{Message: "message"})
	}

	if buffer.Size() != 5 {
		t.Errorf("Size() should be 5 before clear, got %d", buffer.Size())
	}

	buffer.Clear()

	if buffer.index != 0 {
		t.Errorf("index should be 0 after clear, got %d", buffer.index)
	}
	if buffer.full {
		t.Error("full should be false after clear")
	}
	if buffer.Size() != 0 {
		t.Errorf("Size() should be 0 after clear, got %d", buffer.Size())
	}

	result := buffer.GetAll()
	if len(result) != 0 {
		t.Errorf("GetAll() should return 0 entries after clear, got %d", len(result))
	}
}

func TestLogBuffer_Size_Empty(t *testing.T) {
	buffer := NewLogBuffer(10)

	if buffer.Size() != 0 {
		t.Errorf("Size() should be 0 for empty buffer, got %d", buffer.Size())
	}
}

func TestLogBuffer_Size_NotFull(t *testing.T) {
	buffer := NewLogBuffer(10)

	for i := 1; i <= 5; i++ {
		buffer.Add(LogEntry{Message: "message"})
		if buffer.Size() != i {
			t.Errorf("Size() should be %d, got %d", i, buffer.Size())
		}
	}
}

func TestLogBuffer_Size_Full(t *testing.T) {
	buffer := NewLogBuffer(5)

	// Fill buffer
	for i := 0; i < 5; i++ {
		buffer.Add(LogEntry{Message: "message"})
	}

	if buffer.Size() != 5 {
		t.Errorf("Size() should be 5 when full, got %d", buffer.Size())
	}

	// Add more entries (should still report size as capacity)
	for i := 0; i < 3; i++ {
		buffer.Add(LogEntry{Message: "message"})
		if buffer.Size() != 5 {
			t.Errorf("Size() should remain 5 when full, got %d", buffer.Size())
		}
	}
}

func TestLogBuffer_ConcurrentAccess(t *testing.T) {
	buffer := NewLogBuffer(100)

	// Test concurrent writes and reads
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 50; i++ {
			buffer.Add(LogEntry{Message: "concurrent write"})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 50; i++ {
			_ = buffer.GetRecent(10)
			_ = buffer.Size()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify buffer state
	if buffer.Size() != 50 {
		t.Errorf("Size() should be 50, got %d", buffer.Size())
	}
}

func TestLogBuffer_AllFields(t *testing.T) {
	buffer := NewLogBuffer(10)

	now := time.Now()
	entry := LogEntry{
		Timestamp:   now,
		ProcessName: "nginx",
		InstanceID:  "nginx-0",
		Stream:      "stderr",
		Message:     "Error occurred",
		Level:       "error",
	}

	buffer.Add(entry)

	result := buffer.GetAll()
	if len(result) != 1 {
		t.Fatalf("GetAll() should return 1 entry, got %d", len(result))
	}

	retrieved := result[0]
	if !retrieved.Timestamp.Equal(now) {
		t.Errorf("Timestamp mismatch")
	}
	if retrieved.ProcessName != "nginx" {
		t.Errorf("ProcessName = %v, want nginx", retrieved.ProcessName)
	}
	if retrieved.InstanceID != "nginx-0" {
		t.Errorf("InstanceID = %v, want nginx-0", retrieved.InstanceID)
	}
	if retrieved.Stream != "stderr" {
		t.Errorf("Stream = %v, want stderr", retrieved.Stream)
	}
	if retrieved.Message != "Error occurred" {
		t.Errorf("Message = %v, want 'Error occurred'", retrieved.Message)
	}
	if retrieved.Level != "error" {
		t.Errorf("Level = %v, want error", retrieved.Level)
	}
}

func TestLogBuffer_GetAll_EmptyBuffer(t *testing.T) {
	buffer := NewLogBuffer(10)

	result := buffer.GetAll()

	if len(result) != 0 {
		t.Errorf("GetAll() should return empty slice for empty buffer, got %d entries", len(result))
	}
}

func TestLogBuffer_GetRecent_Zero(t *testing.T) {
	buffer := NewLogBuffer(10)
	buffer.Add(LogEntry{Message: "test"})

	result := buffer.GetRecent(0)

	if len(result) != 0 {
		t.Errorf("GetRecent(0) should return empty slice, got %d entries", len(result))
	}
}

func TestLogBuffer_GetRecent_Full_NoWrapNeeded(t *testing.T) {
	// Test the specific case where buffer is full but n <= index (no wrap needed)
	// This targets line 106-108 in log_buffer.go
	buffer := NewLogBuffer(10)

	// Fill buffer completely
	for i := 1; i <= 10; i++ {
		buffer.Add(LogEntry{Message: "msg" + string(rune('0'+i))})
	}

	// Add 5 more entries - this wraps around
	// After 15 total adds to size 10 buffer:
	// - entries[0-4] contain msg11-msg15
	// - entries[5-9] contain msg6-msg10
	// - index = 5
	// - full = true
	for i := 11; i <= 15; i++ {
		buffer.Add(LogEntry{Message: "msg" + string(rune('0'+i))})
	}

	// Get last 3 entries where n (3) <= index (5)
	// This should use the no-wrap path: entries[index-n:index] = entries[2:5]
	result := buffer.GetRecent(3)

	if len(result) != 3 {
		t.Errorf("GetRecent(3) should return 3 entries, got %d", len(result))
	}

	// Should be msg13, msg14, msg15 (entries at positions 2, 3, 4)
	expected := []string{"msg" + string(rune('0'+13)), "msg" + string(rune('0'+14)), "msg" + string(rune('0'+15))}
	for i, entry := range result {
		if entry.Message != expected[i] {
			t.Errorf("entry[%d] message = %v, want %v", i, entry.Message, expected[i])
		}
	}

	// Also test where n exactly equals index (edge case)
	result2 := buffer.GetRecent(5)
	if len(result2) != 5 {
		t.Errorf("GetRecent(5) should return 5 entries, got %d", len(result2))
	}
}
