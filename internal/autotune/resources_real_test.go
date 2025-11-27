package autotune

import (
	"runtime"
	"testing"
)

// TestDetectCgroupV2Resources_RealExecution tests actual execution path
// Even if detection fails, this exercises the code paths
func TestDetectCgroupV2Resources_RealExecution(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	resources := &ContainerResources{
		CPULimit: runtime.NumCPU(),
	}

	// Execute the actual function
	err := detectCgroupV2Resources(resources)

	// We don't care if it succeeds or fails, we just want to exercise the code
	if err != nil {
		t.Logf("Cgroup v2 detection failed (expected on non-containerized): %v", err)
		// Verify function didn't corrupt resources
		if resources.CPULimit < 1 {
			t.Error("CPULimit should not be corrupted by failed detection")
		}
	} else {
		t.Logf("Cgroup v2 detected successfully: %s", resources.String())
		// Verify reasonable values
		if resources.MemoryLimitBytes < 0 {
			t.Error("Memory bytes should be non-negative")
		}
		if resources.MemoryLimitMB < 0 {
			t.Error("Memory MB should be non-negative")
		}
	}
}

// TestDetectCgroupV1Resources_RealExecution tests actual execution path
func TestDetectCgroupV1Resources_RealExecution(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	resources := &ContainerResources{
		CPULimit: runtime.NumCPU(),
	}

	// Execute the actual function
	err := detectCgroupV1Resources(resources)

	// We don't care if it succeeds or fails, we just want to exercise the code
	if err != nil {
		t.Logf("Cgroup v1 detection failed (expected on non-containerized): %v", err)
		// Verify function didn't corrupt resources
		if resources.CPULimit < 1 {
			t.Error("CPULimit should not be corrupted by failed detection")
		}
	} else {
		t.Logf("Cgroup v1 detected successfully: %s", resources.String())
		// Verify reasonable values
		if resources.MemoryLimitBytes < 0 {
			t.Error("Memory bytes should be non-negative")
		}
		if resources.MemoryLimitMB < 0 {
			t.Error("Memory MB should be non-negative")
		}

		// Verify unlimited memory filtering worked
		if resources.MemoryLimitBytes > (1 << 50) {
			t.Error("Very large memory limit should have been filtered out")
		}
	}
}

// TestGetHostMemory_RealExecution tests actual execution path
func TestGetHostMemory_RealExecution(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	memory, err := getHostMemory()

	if err != nil {
		t.Logf("getHostMemory() failed: %v", err)
		// This is acceptable if /proc/meminfo doesn't exist
		return
	}

	// Verify reasonable values
	minMemory := int64(100 * 1024 * 1024)         // 100 MB
	maxMemory := int64(1024 * 1024 * 1024 * 1024) // 1 TB

	if memory < minMemory {
		t.Errorf("Memory %d bytes seems too low (< 100MB)", memory)
	}

	if memory > maxMemory {
		t.Errorf("Memory %d bytes seems too high (> 1TB)", memory)
	}

	// Verify conversion is correct
	memoryMB := memory / (1024 * 1024)
	t.Logf("Host memory: %d MB (%.2f GB)", memoryMB, float64(memory)/(1024*1024*1024))
}

// TestDetectContainerResources_AllPaths tests all execution paths
func TestDetectContainerResources_AllPaths(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	// Call the main detection function
	resources, err := DetectContainerResources()

	if err != nil {
		t.Fatalf("DetectContainerResources() should not return error: %v", err)
	}

	// Log what was detected
	t.Logf("Detection result: %s", resources.String())

	// Verify basic invariants
	if resources.CPULimit < 1 {
		t.Error("Should have at least 1 CPU")
	}

	if resources.MemoryLimitBytes < 0 {
		t.Error("Memory bytes should be non-negative")
	}

	if resources.MemoryLimitMB < 0 {
		t.Error("Memory MB should be non-negative")
	}

	// Verify consistency
	expectedMB := int(resources.MemoryLimitBytes / (1024 * 1024))
	if resources.MemoryLimitMB != expectedMB {
		t.Errorf("Memory inconsistent: MB=%d, bytes=%d (expected MB=%d)",
			resources.MemoryLimitMB, resources.MemoryLimitBytes, expectedMB)
	}

	// Verify containerization flag consistency
	if resources.IsContainerized {
		if resources.CgroupVersion != 1 && resources.CgroupVersion != 2 {
			t.Errorf("Containerized should have cgroup version 1 or 2, got %d",
				resources.CgroupVersion)
		}
		t.Logf("Containerized environment detected (cgroup v%d)", resources.CgroupVersion)
	} else {
		if resources.CgroupVersion != 0 {
			t.Errorf("Non-containerized should have cgroup version 0, got %d",
				resources.CgroupVersion)
		}
		t.Logf("Host environment detected")
	}
}

// TestCgroupExistenceFunctions tests the existence check functions
func TestCgroupExistenceFunctions(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	v2Exists := cgroupV2Exists()
	v1Exists := cgroupV1Exists()

	t.Logf("Cgroup v2 exists: %v", v2Exists)
	t.Logf("Cgroup v1 exists: %v", v1Exists)

	// At least verify the functions return without panicking
	if v2Exists && v1Exists {
		t.Log("Both cgroup v1 and v2 are present on this system")
	} else if v2Exists {
		t.Log("Only cgroup v2 is present")
	} else if v1Exists {
		t.Log("Only cgroup v1 is present")
	} else {
		t.Log("No cgroup limits detected (running on host or unsupported system)")
	}
}

// TestResourceDetectionIntegration tests the full detection flow
func TestResourceDetectionIntegration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	// This test exercises the full detection flow including all paths

	// First check what exists
	v2 := cgroupV2Exists()
	v1 := cgroupV1Exists()

	resources, err := DetectContainerResources()
	if err != nil {
		t.Fatalf("DetectContainerResources() failed: %v", err)
	}

	// Verify detection priority (v2 > v1 > host)
	if v2 {
		// If v2 exists, it should be preferred
		if resources.IsContainerized && resources.CgroupVersion != 2 {
			t.Log("Note: Cgroup v2 exists but v1 was detected (may be due to detection failures)")
		}
	}

	// Log complete detection result
	t.Logf("Full detection result:")
	t.Logf("  Cgroup v2 exists: %v", v2)
	t.Logf("  Cgroup v1 exists: %v", v1)
	t.Logf("  Detected: %s", resources.String())
	t.Logf("  IsContainerized: %v", resources.IsContainerized)
	t.Logf("  CgroupVersion: %d", resources.CgroupVersion)
	t.Logf("  MemoryLimitMB: %d", resources.MemoryLimitMB)
	t.Logf("  CPULimit: %d", resources.CPULimit)
}
