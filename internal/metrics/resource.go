package metrics

import (
	"log/slog"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// CollectProcessMetrics collects resource metrics for a single process
func CollectProcessMetrics(pid int, processName, instanceID string) (*ResourceSample, error) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}

	sample := &ResourceSample{
		Timestamp:       time.Now(),
		FileDescriptors: -1, // Default for non-Linux
	}

	// CPU Percent
	if cpu, err := proc.CPUPercent(); err == nil {
		sample.CPUPercent = cpu
	}

	// Memory Info
	if memInfo, err := proc.MemoryInfo(); err == nil {
		sample.MemoryRSSBytes = memInfo.RSS
		sample.MemoryVMSBytes = memInfo.VMS
	}

	// Memory Percent
	if memPct, err := proc.MemoryPercent(); err == nil {
		sample.MemoryPercent = memPct
	}

	// Thread Count
	if threads, err := proc.NumThreads(); err == nil {
		sample.Threads = threads
	}

	// File Descriptors (Linux only)
	if fds, err := proc.NumFDs(); err == nil {
		sample.FileDescriptors = int32(fds)
	}

	return sample, nil
}

// UpdatePrometheusMetrics updates Prometheus gauges with resource sample
func UpdatePrometheusMetrics(processName, instanceID string, sample *ResourceSample) {
	ProcessCPUPercent.WithLabelValues(processName, instanceID).Set(sample.CPUPercent)

	ProcessMemoryBytes.WithLabelValues(processName, instanceID, "rss").Set(float64(sample.MemoryRSSBytes))
	ProcessMemoryBytes.WithLabelValues(processName, instanceID, "vms").Set(float64(sample.MemoryVMSBytes))

	ProcessMemoryPercent.WithLabelValues(processName, instanceID).Set(float64(sample.MemoryPercent))

	ProcessThreads.WithLabelValues(processName, instanceID).Set(float64(sample.Threads))

	if sample.FileDescriptors >= 0 {
		ProcessFileDescriptors.WithLabelValues(processName, instanceID).Set(float64(sample.FileDescriptors))
	}
}

// ResourceCollector manages resource metric collection
type ResourceCollector struct {
	interval   time.Duration
	maxSamples int
	buffers    map[string]*TimeSeriesBuffer // key: "process-instance"
	mu         sync.RWMutex
	logger     *slog.Logger
}

// NewResourceCollector creates a new resource collector
func NewResourceCollector(interval time.Duration, maxSamples int, logger *slog.Logger) *ResourceCollector {
	return &ResourceCollector{
		interval:   interval,
		maxSamples: maxSamples,
		buffers:    make(map[string]*TimeSeriesBuffer),
		logger:     logger.With("component", "resource_collector"),
	}
}

// GetHistory returns time series for a process instance
func (rc *ResourceCollector) GetHistory(processName, instanceID string, since time.Time, limit int) []ResourceSample {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	key := processName + "-" + instanceID
	buffer, exists := rc.buffers[key]
	if !exists {
		return []ResourceSample{}
	}

	return buffer.GetRange(since, limit)
}

// AddSample adds a sample to the buffer for a process instance
func (rc *ResourceCollector) AddSample(processName, instanceID string, sample ResourceSample) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	key := processName + "-" + instanceID

	// Lazy initialization of buffer
	if _, exists := rc.buffers[key]; !exists {
		rc.buffers[key] = NewTimeSeriesBuffer(rc.maxSamples)
	}

	rc.buffers[key].Add(sample)
}

// RemoveBuffer removes buffer for stopped process
func (rc *ResourceCollector) RemoveBuffer(processName, instanceID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	key := processName + "-" + instanceID
	delete(rc.buffers, key)
}

// GetBufferSizes returns memory usage info
func (rc *ResourceCollector) GetBufferSizes() map[string]int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	sizes := make(map[string]int, len(rc.buffers))
	for key, buffer := range rc.buffers {
		sizes[key] = buffer.Size()
	}

	return sizes
}

// GetInterval returns the collection interval
func (rc *ResourceCollector) GetInterval() time.Duration {
	return rc.interval
}

// GetLatest returns the latest sample for a process instance if available
func (rc *ResourceCollector) GetLatest(processName, instanceID string) (ResourceSample, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	key := processName + "-" + instanceID
	buffer, exists := rc.buffers[key]
	if !exists {
		return ResourceSample{}, false
	}

	return buffer.Latest()
}
