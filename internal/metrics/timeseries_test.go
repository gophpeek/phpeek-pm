package metrics

import (
	"testing"
	"time"
)

// TestNewTimeSeriesBuffer tests buffer creation with various sizes
func TestNewTimeSeriesBuffer(t *testing.T) {
	tests := []struct {
		name           string
		maxSamples     int
		expectedSize   int
		expectedLength int
	}{
		{
			name:           "default size for zero",
			maxSamples:     0,
			expectedSize:   720,
			expectedLength: 720,
		},
		{
			name:           "default size for negative",
			maxSamples:     -1,
			expectedSize:   720,
			expectedLength: 720,
		},
		{
			name:           "custom size",
			maxSamples:     100,
			expectedSize:   100,
			expectedLength: 100,
		},
		{
			name:           "single sample",
			maxSamples:     1,
			expectedSize:   1,
			expectedLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsb := NewTimeSeriesBuffer(tt.maxSamples)

			if tsb == nil {
				t.Fatal("Expected non-nil buffer")
			}

			if tsb.maxSamples != tt.expectedSize {
				t.Errorf("Expected maxSamples %d, got %d", tt.expectedSize, tsb.maxSamples)
			}

			if len(tsb.samples) != tt.expectedLength {
				t.Errorf("Expected samples length %d, got %d", tt.expectedLength, len(tsb.samples))
			}

			if tsb.Size() != 0 {
				t.Errorf("Expected initial size 0, got %d", tsb.Size())
			}
		})
	}
}

// TestTimeSeriesBuffer_Add tests adding samples to the buffer
func TestTimeSeriesBuffer_Add(t *testing.T) {
	tsb := NewTimeSeriesBuffer(5)

	now := time.Now()
	for i := 0; i < 3; i++ {
		sample := ResourceSample{
			Timestamp:      now.Add(time.Duration(i) * time.Second),
			CPUPercent:     float64(i * 10),
			MemoryRSSBytes: uint64(i * 1024),
		}
		tsb.Add(sample)
	}

	if tsb.Size() != 3 {
		t.Errorf("Expected size 3, got %d", tsb.Size())
	}

	// Verify samples can be retrieved
	samples := tsb.GetLast(3)
	if len(samples) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(samples))
	}

	// Verify chronological order (oldest first)
	for i := 0; i < 3; i++ {
		if samples[i].CPUPercent != float64(i*10) {
			t.Errorf("Sample %d: expected CPU %f, got %f", i, float64(i*10), samples[i].CPUPercent)
		}
	}
}

// TestTimeSeriesBuffer_RingOverflow tests buffer wrapping behavior
func TestTimeSeriesBuffer_RingOverflow(t *testing.T) {
	tsb := NewTimeSeriesBuffer(3)

	now := time.Now()
	// Add 5 samples to a buffer with max 3
	for i := 0; i < 5; i++ {
		sample := ResourceSample{
			Timestamp:      now.Add(time.Duration(i) * time.Second),
			CPUPercent:     float64(i),
			MemoryRSSBytes: uint64(i * 1024),
		}
		tsb.Add(sample)
	}

	// Size should cap at maxSamples (3)
	if tsb.Size() != 3 {
		t.Errorf("Expected size 3, got %d", tsb.Size())
	}

	// Should have the last 3 samples (indexes 2, 3, 4)
	samples := tsb.GetLast(3)
	if len(samples) != 3 {
		t.Fatalf("Expected 3 samples, got %d", len(samples))
	}

	// Verify we have samples 2, 3, 4 (oldest to newest)
	expectedCPU := []float64{2, 3, 4}
	for i := 0; i < 3; i++ {
		if samples[i].CPUPercent != expectedCPU[i] {
			t.Errorf("Sample %d: expected CPU %f, got %f", i, expectedCPU[i], samples[i].CPUPercent)
		}
	}
}

// TestTimeSeriesBuffer_GetLast tests retrieving last N samples
func TestTimeSeriesBuffer_GetLast(t *testing.T) {
	tsb := NewTimeSeriesBuffer(10)

	now := time.Now()
	for i := 0; i < 5; i++ {
		sample := ResourceSample{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i),
		}
		tsb.Add(sample)
	}

	tests := []struct {
		name           string
		n              int
		expectedLength int
	}{
		{
			name:           "get last 3",
			n:              3,
			expectedLength: 3,
		},
		{
			name:           "get last 10 (more than available)",
			n:              10,
			expectedLength: 5,
		},
		{
			name:           "get last 0",
			n:              0,
			expectedLength: 5, // Returns all when n=0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samples := tsb.GetLast(tt.n)

			if len(samples) != tt.expectedLength {
				t.Errorf("Expected %d samples, got %d", tt.expectedLength, len(samples))
			}

			// Verify chronological order (oldest first)
			for i := 1; i < len(samples); i++ {
				if !samples[i].Timestamp.After(samples[i-1].Timestamp) {
					t.Error("Samples not in chronological order")
				}
			}
		})
	}
}

// TestTimeSeriesBuffer_GetSince tests retrieving samples since timestamp
func TestTimeSeriesBuffer_GetSince(t *testing.T) {
	tsb := NewTimeSeriesBuffer(10)

	now := time.Now()
	for i := 0; i < 5; i++ {
		sample := ResourceSample{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i),
		}
		tsb.Add(sample)
	}

	tests := []struct {
		name           string
		since          time.Time
		expectedCount  int
	}{
		{
			name:          "since 2 seconds ago",
			since:         now.Add(2 * time.Second),
			expectedCount: 3, // samples 2, 3, 4
		},
		{
			name:          "since beginning",
			since:         time.Time{},
			expectedCount: 5, // all samples
		},
		{
			name:          "since future",
			since:         now.Add(10 * time.Second),
			expectedCount: 0, // no samples
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samples := tsb.GetSince(tt.since)

			if len(samples) != tt.expectedCount {
				t.Errorf("Expected %d samples, got %d", tt.expectedCount, len(samples))
			}
		})
	}
}

// TestTimeSeriesBuffer_GetRange tests time range queries with limits
func TestTimeSeriesBuffer_GetRange(t *testing.T) {
	tsb := NewTimeSeriesBuffer(10)

	now := time.Now()
	for i := 0; i < 5; i++ {
		sample := ResourceSample{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i),
		}
		tsb.Add(sample)
	}

	tests := []struct {
		name           string
		since          time.Time
		limit          int
		expectedCount  int
	}{
		{
			name:          "range with limit",
			since:         now,
			limit:         3,
			expectedCount: 3,
		},
		{
			name:          "range no limit",
			since:         now,
			limit:         0,
			expectedCount: 5,
		},
		{
			name:          "range with high limit",
			since:         now.Add(2 * time.Second),
			limit:         10,
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samples := tsb.GetRange(tt.since, tt.limit)

			if len(samples) != tt.expectedCount {
				t.Errorf("Expected %d samples, got %d", tt.expectedCount, len(samples))
			}
		})
	}
}

// TestTimeSeriesBuffer_Clear tests clearing the buffer
func TestTimeSeriesBuffer_Clear(t *testing.T) {
	tsb := NewTimeSeriesBuffer(5)

	now := time.Now()
	for i := 0; i < 3; i++ {
		sample := ResourceSample{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i),
		}
		tsb.Add(sample)
	}

	if tsb.Size() != 3 {
		t.Fatalf("Expected size 3 before clear, got %d", tsb.Size())
	}

	tsb.Clear()

	if tsb.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", tsb.Size())
	}

	if tsb.head != 0 {
		t.Errorf("Expected head 0 after clear, got %d", tsb.head)
	}

	// Verify we can still add after clear
	sample := ResourceSample{
		Timestamp:  now,
		CPUPercent: 99.9,
	}
	tsb.Add(sample)

	if tsb.Size() != 1 {
		t.Errorf("Expected size 1 after adding post-clear, got %d", tsb.Size())
	}
}

// TestTimeSeriesBuffer_EmptyBuffer tests operations on empty buffer
func TestTimeSeriesBuffer_EmptyBuffer(t *testing.T) {
	tsb := NewTimeSeriesBuffer(5)

	samples := tsb.GetLast(10)
	if len(samples) != 0 {
		t.Errorf("Expected 0 samples from empty buffer, got %d", len(samples))
	}

	samples = tsb.GetSince(time.Now())
	if len(samples) != 0 {
		t.Errorf("Expected 0 samples from empty buffer, got %d", len(samples))
	}

	samples = tsb.GetRange(time.Now(), 5)
	if len(samples) != 0 {
		t.Errorf("Expected 0 samples from empty buffer, got %d", len(samples))
	}
}

// TestTimeSeriesBuffer_Latest tests getting the most recent sample
func TestTimeSeriesBuffer_Latest(t *testing.T) {
	tsb := NewTimeSeriesBuffer(10)

	// Test empty buffer
	_, exists := tsb.Latest()
	if exists {
		t.Error("Expected no latest sample in empty buffer")
	}

	// Add samples
	now := time.Now()
	for i := 0; i < 5; i++ {
		sample := ResourceSample{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i * 10),
		}
		tsb.Add(sample)
	}

	// Get latest
	latest, exists := tsb.Latest()
	if !exists {
		t.Fatal("Expected latest sample to exist")
	}

	// Should be the last added sample (i=4)
	if latest.CPUPercent != 40.0 {
		t.Errorf("Expected CPU 40.0, got %f", latest.CPUPercent)
	}

	expectedTime := now.Add(4 * time.Second)
	if !latest.Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, latest.Timestamp)
	}

	// Add more to test ring buffer wrapping
	tsb2 := NewTimeSeriesBuffer(3)
	for i := 0; i < 5; i++ {
		sample := ResourceSample{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i),
		}
		tsb2.Add(sample)
	}

	// Latest should be sample with i=4 (most recent)
	latest2, exists := tsb2.Latest()
	if !exists {
		t.Fatal("Expected latest sample in wrapped buffer")
	}

	if latest2.CPUPercent != 4.0 {
		t.Errorf("Expected CPU 4.0 after wrap, got %f", latest2.CPUPercent)
	}
}

// TestTimeSeriesBuffer_ConcurrentAccess tests thread-safety
func TestTimeSeriesBuffer_ConcurrentAccess(t *testing.T) {
	tsb := NewTimeSeriesBuffer(100)

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 50; i++ {
			sample := ResourceSample{
				Timestamp:  time.Now(),
				CPUPercent: float64(i),
			}
			tsb.Add(sample)
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 50; i++ {
			_ = tsb.GetLast(10)
			_ = tsb.Size()
			_, _ = tsb.Latest()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// No assertion needed - just checking no race conditions or panics
}
