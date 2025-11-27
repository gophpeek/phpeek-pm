package autotune

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

// TestDetectCgroupV2Resources_Integration tests actual cgroup v2 detection
// This test creates temporary cgroup-like files and tests parsing
func TestDetectCgroupV2Resources_Integration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	// Test with actual file system (even if not in a container)
	resources := &ContainerResources{
		CPULimit: runtime.NumCPU(),
	}

	// Try to detect cgroup v2
	err := detectCgroupV2Resources(resources)

	// We don't expect this to necessarily succeed on all systems,
	// but it shouldn't panic or corrupt data
	if err != nil {
		t.Logf("Cgroup v2 detection failed (expected on non-containerized systems): %v", err)
		// Verify resources weren't corrupted
		if resources.CPULimit < 1 {
			t.Error("CPU limit should remain valid after failed detection")
		}
	} else {
		t.Logf("Cgroup v2 detected: %s", resources.String())
		// Verify parsed values are reasonable
		if resources.MemoryLimitBytes < 0 {
			t.Error("Memory bytes should be non-negative")
		}
		if resources.MemoryLimitMB < 0 {
			t.Error("Memory MB should be non-negative")
		}
		if resources.CPULimit < 1 {
			t.Error("CPU limit should be at least 1")
		}
	}
}

// TestDetectCgroupV1Resources_Integration tests actual cgroup v1 detection
func TestDetectCgroupV1Resources_Integration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	resources := &ContainerResources{
		CPULimit: runtime.NumCPU(),
	}

	// Try to detect cgroup v1
	err := detectCgroupV1Resources(resources)

	if err != nil {
		t.Logf("Cgroup v1 detection failed (expected on non-containerized systems): %v", err)
		// Verify resources weren't corrupted
		if resources.CPULimit < 1 {
			t.Error("CPU limit should remain valid after failed detection")
		}
	} else {
		t.Logf("Cgroup v1 detected: %s", resources.String())
		// Verify parsed values are reasonable
		if resources.MemoryLimitBytes < 0 {
			t.Error("Memory bytes should be non-negative")
		}
		if resources.MemoryLimitMB < 0 {
			t.Error("Memory MB should be non-negative")
		}
		if resources.CPULimit < 1 {
			t.Error("CPU limit should be at least 1")
		}

		// Verify cgroup v1 unlimited memory detection
		// If value is very large, it should be treated as unlimited
		if resources.MemoryLimitBytes > (1 << 50) {
			t.Error("Very large memory limit should be filtered out as unlimited")
		}
	}
}

// TestGetHostMemory_Integration tests actual /proc/meminfo reading
func TestGetHostMemory_Integration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux /proc/meminfo")
	}

	memory, err := getHostMemory()

	if err != nil {
		// Check if it's a "file not found" error vs parsing error
		if os.IsNotExist(err) {
			t.Skip("/proc/meminfo not found (expected in some environments)")
		}
		t.Errorf("getHostMemory() failed: %v", err)
		return
	}

	// Validate returned memory is reasonable
	minMemory := int64(100 * 1024 * 1024)         // 100 MB minimum
	maxMemory := int64(1024 * 1024 * 1024 * 1024) // 1 TB maximum

	if memory < minMemory {
		t.Errorf("Memory %d bytes is too low (< 100MB), possible parsing error", memory)
	}

	if memory > maxMemory {
		t.Errorf("Memory %d bytes is too high (> 1TB), possible parsing error", memory)
	}

	// Log the detected memory for debugging
	memoryMB := memory / (1024 * 1024)
	memoryGB := float64(memory) / (1024 * 1024 * 1024)
	t.Logf("Detected host memory: %d MB (%.2f GB)", memoryMB, memoryGB)

	// Verify memory is aligned to reasonable boundaries
	// Most systems have memory in powers of 2 or nice round numbers
	if memory%1024 != 0 {
		t.Logf("Warning: Memory %d bytes is not aligned to 1KB boundary", memory)
	}
}

// TestDetectContainerResources_CgroupPriority tests that v2 is preferred over v1
func TestDetectContainerResources_CgroupPriority(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	// Call actual detection
	resources, err := DetectContainerResources()
	if err != nil {
		t.Fatalf("DetectContainerResources() failed: %v", err)
	}

	// If containerized, should prefer v2 over v1
	if resources.IsContainerized {
		t.Logf("Containerized environment detected: %s", resources.String())

		// If both v1 and v2 exist, v2 should win
		v2Exists := cgroupV2Exists()
		v1Exists := cgroupV1Exists()

		t.Logf("Cgroup v2 exists: %v", v2Exists)
		t.Logf("Cgroup v1 exists: %v", v1Exists)

		if v2Exists && resources.CgroupVersion != 2 {
			t.Error("Cgroup v2 exists but was not detected as primary")
		}
	} else {
		t.Logf("Host environment detected (not containerized): %s", resources.String())
	}
}

// TestCgroupFiles_Existence tests which cgroup files actually exist on this system
func TestCgroupFiles_Existence(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	files := map[string]string{
		"cgroup v2 controllers": "/sys/fs/cgroup/cgroup.controllers",
		"cgroup v2 memory.max":  "/sys/fs/cgroup/memory.max",
		"cgroup v2 cpu.max":     "/sys/fs/cgroup/cpu.max",
		"cgroup v1 memory":      "/sys/fs/cgroup/memory/memory.limit_in_bytes",
		"cgroup v1 cpu quota":   "/sys/fs/cgroup/cpu/cpu.cfs_quota_us",
		"cgroup v1 cpu period":  "/sys/fs/cgroup/cpu/cpu.cfs_period_us",
		"proc meminfo":          "/proc/meminfo",
	}

	for name, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				t.Logf("%s: does not exist (%s)", name, path)
			} else {
				t.Logf("%s: error checking (%s): %v", name, path, err)
			}
		} else {
			if info.IsDir() {
				t.Logf("%s: exists (directory) (%s)", name, path)
			} else {
				t.Logf("%s: exists (file, %d bytes) (%s)", name, info.Size(), path)
			}
		}
	}
}

// TestDetectContainerResources_Consistency tests internal consistency
func TestDetectContainerResources_Consistency(t *testing.T) {
	resources, err := DetectContainerResources()
	if err != nil {
		t.Fatalf("DetectContainerResources() failed: %v", err)
	}

	// Verify bytes and MB are consistent
	expectedMB := int(resources.MemoryLimitBytes / (1024 * 1024))
	if resources.MemoryLimitMB != expectedMB {
		t.Errorf("Memory inconsistency: MemoryLimitMB=%d, expected %d from bytes=%d",
			resources.MemoryLimitMB, expectedMB, resources.MemoryLimitBytes)
	}

	// Verify CPU is reasonable
	if resources.CPULimit < 1 {
		t.Error("CPU limit should be at least 1")
	}
	if resources.CPULimit > 1024 {
		t.Errorf("CPU limit %d seems unreasonably high (> 1024 cores)", resources.CPULimit)
	}

	// Verify containerization flag consistency
	if resources.IsContainerized {
		if resources.CgroupVersion != 1 && resources.CgroupVersion != 2 {
			t.Errorf("Containerized resources should have cgroup version 1 or 2, got %d",
				resources.CgroupVersion)
		}
	} else {
		if resources.CgroupVersion != 0 {
			t.Errorf("Non-containerized resources should have cgroup version 0, got %d",
				resources.CgroupVersion)
		}
	}

	t.Logf("Detected resources: %s", resources.String())
}

// TestCgroupV2Files_ContentParsing tests actual cgroup v2 file content if available
func TestCgroupV2Files_ContentParsing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	if !cgroupV2Exists() {
		t.Skip("Cgroup v2 not available on this system")
	}

	// Test memory.max parsing
	memMaxPath := "/sys/fs/cgroup/memory.max"
	if data, err := os.ReadFile(memMaxPath); err == nil {
		content := string(data)
		t.Logf("memory.max content: %q", content)

		// Should be either "max" or a number
		if content != "max\n" {
			// Try to parse as number
			var memBytes int64
			if _, err := fmt.Sscanf(content, "%d", &memBytes); err != nil {
				t.Errorf("memory.max should be 'max' or a number, got: %q", content)
			} else {
				t.Logf("Parsed memory limit: %d bytes (%.2f GB)",
					memBytes, float64(memBytes)/(1024*1024*1024))
			}
		}
	}

	// Test cpu.max parsing
	cpuMaxPath := "/sys/fs/cgroup/cpu.max"
	if data, err := os.ReadFile(cpuMaxPath); err == nil {
		content := string(data)
		t.Logf("cpu.max content: %q", content)

		// Should be either "max PERIOD" or "QUOTA PERIOD"
		var quota, period int64
		if _, err := fmt.Sscanf(content, "%d %d", &quota, &period); err != nil {
			// Check if it's "max PERIOD"
			if _, err := fmt.Sscanf(content, "max %d", &period); err != nil {
				t.Errorf("cpu.max should be 'max PERIOD' or 'QUOTA PERIOD', got: %q", content)
			} else {
				t.Logf("CPU quota: unlimited (max), period: %d", period)
			}
		} else {
			cpus := float64(quota) / float64(period)
			t.Logf("CPU quota: %d, period: %d, effective CPUs: %.2f", quota, period, cpus)
		}
	}
}

// TestCgroupV1Files_ContentParsing tests actual cgroup v1 file content if available
func TestCgroupV1Files_ContentParsing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	if !cgroupV1Exists() {
		t.Skip("Cgroup v1 not available on this system")
	}

	// Test memory limit parsing
	memLimitPath := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	if data, err := os.ReadFile(memLimitPath); err == nil {
		content := string(data)
		t.Logf("memory.limit_in_bytes content: %q", content)

		var memBytes int64
		if _, err := fmt.Sscanf(content, "%d", &memBytes); err != nil {
			t.Errorf("Failed to parse memory limit: %v", err)
		} else {
			if memBytes > (1 << 50) {
				t.Logf("Memory limit appears unlimited (very large): %d bytes", memBytes)
			} else {
				t.Logf("Parsed memory limit: %d bytes (%.2f GB)",
					memBytes, float64(memBytes)/(1024*1024*1024))
			}
		}
	}

	// Test CPU quota parsing
	quotaPath := "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
	periodPath := "/sys/fs/cgroup/cpu/cpu.cfs_period_us"

	var quota, period int64

	if data, err := os.ReadFile(quotaPath); err == nil {
		if _, err := fmt.Sscanf(string(data), "%d", &quota); err != nil {
			t.Errorf("Failed to parse CPU quota: %v", err)
		}
		t.Logf("cpu.cfs_quota_us: %d", quota)
	}

	if data, err := os.ReadFile(periodPath); err == nil {
		if _, err := fmt.Sscanf(string(data), "%d", &period); err != nil {
			t.Errorf("Failed to parse CPU period: %v", err)
		}
		t.Logf("cpu.cfs_period_us: %d", period)
	}

	if quota > 0 && period > 0 {
		cpus := float64(quota) / float64(period)
		t.Logf("Effective CPUs: %.2f", cpus)
	} else if quota == -1 {
		t.Logf("CPU quota unlimited")
	}
}
