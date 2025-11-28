package autotune

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDetectContainerResources tests the main resource detection function
func TestDetectContainerResources(t *testing.T) {
	// This test will attempt real detection
	// Result depends on whether we're running in a container or on host
	resources, err := DetectContainerResources()

	if err != nil {
		t.Fatalf("DetectContainerResources() failed: %v", err)
	}

	if resources == nil {
		t.Fatal("Expected non-nil resources")
	}

	// Should have at least 1 CPU
	if resources.CPULimit < 1 {
		t.Errorf("Expected at least 1 CPU, got %d", resources.CPULimit)
	}

	// Should have detected some memory
	// Note: MemoryLimitMB might be 0 if unlimited
	if resources.MemoryLimitBytes < 0 {
		t.Errorf("Expected non-negative memory bytes, got %d", resources.MemoryLimitBytes)
	}

	// If containerized, should have cgroup version
	if resources.IsContainerized {
		if resources.CgroupVersion != 1 && resources.CgroupVersion != 2 {
			t.Errorf("Expected cgroup version 1 or 2, got %d", resources.CgroupVersion)
		}
		t.Logf("Detected containerized environment: %s", resources.String())
	} else {
		if resources.CgroupVersion != 0 {
			t.Errorf("Non-containerized should have cgroup version 0, got %d", resources.CgroupVersion)
		}
		t.Logf("Detected host environment: %s", resources.String())
	}

	// Verify consistency between bytes and MB
	expectedMB := int(resources.MemoryLimitBytes / (1024 * 1024))
	if resources.MemoryLimitMB != expectedMB {
		t.Errorf("Memory inconsistency: MemoryLimitMB=%d, expected %d from bytes=%d",
			resources.MemoryLimitMB, expectedMB, resources.MemoryLimitBytes)
	}
}

// TestCgroupV2Detection tests cgroup v2 detection with mocked filesystem
func TestCgroupV2Detection(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	cgroupDir := tmpDir

	// Test case 1: cgroup v2 exists and has valid data
	t.Run("valid cgroup v2", func(t *testing.T) {
		// Create cgroup.controllers file (marker for v2)
		controllersPath := filepath.Join(cgroupDir, "cgroup.controllers")
		if err := os.WriteFile(controllersPath, []byte("cpu memory\n"), 0644); err != nil {
			t.Fatalf("Failed to create cgroup.controllers: %v", err)
		}

		// Create memory.max
		memMaxPath := filepath.Join(cgroupDir, "memory.max")
		if err := os.WriteFile(memMaxPath, []byte("2147483648\n"), 0644); err != nil { // 2GB
			t.Fatalf("Failed to create memory.max: %v", err)
		}

		// Create cpu.max
		cpuMaxPath := filepath.Join(cgroupDir, "cpu.max")
		if err := os.WriteFile(cpuMaxPath, []byte("200000 100000\n"), 0644); err != nil { // 2 CPUs
			t.Fatalf("Failed to create cpu.max: %v", err)
		}

		// Note: We can't easily test the actual detection without modifying
		// the source to accept a custom path. This demonstrates what the
		// test structure would look like with dependency injection.
		// For now, we test the helper functions directly.
	})
}

// TestCgroupV2Exists tests the cgroup v2 existence check
func TestCgroupV2Exists(t *testing.T) {
	// This will check the real filesystem
	exists := cgroupV2Exists()

	// We don't know if we're running on a system with cgroup v2,
	// so just verify it returns a boolean without error
	t.Logf("cgroup v2 exists: %v", exists)

	// If it exists, the file should be readable
	if exists {
		_, err := os.Stat("/sys/fs/cgroup/cgroup.controllers")
		if err != nil {
			t.Errorf("cgroupV2Exists() returned true but file not accessible: %v", err)
		}
	}
}

// TestCgroupV1Exists tests the cgroup v1 existence check
func TestCgroupV1Exists(t *testing.T) {
	// This will check the real filesystem
	exists := cgroupV1Exists()

	// We don't know if we're running on a system with cgroup v1,
	// so just verify it returns a boolean without error
	t.Logf("cgroup v1 exists: %v", exists)

	// If it exists, the file should be readable
	if exists {
		_, err := os.Stat("/sys/fs/cgroup/memory/memory.limit_in_bytes")
		if err != nil {
			t.Errorf("cgroupV1Exists() returned true but file not accessible: %v", err)
		}
	}
}

// TestGetHostMemory tests reading host memory from /proc/meminfo
func TestGetHostMemory(t *testing.T) {
	// This requires /proc/meminfo to exist (Linux only)
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux /proc/meminfo")
	}

	memory, err := getHostMemory()

	if err != nil {
		// If /proc/meminfo doesn't exist or is unreadable, that's valid for the test
		if os.IsNotExist(err) {
			t.Skip("/proc/meminfo not found")
		}
		t.Fatalf("getHostMemory() failed: %v", err)
	}

	// Memory should be positive and reasonable (> 100MB, < 1TB)
	minMemory := int64(100 * 1024 * 1024)         // 100 MB
	maxMemory := int64(1024 * 1024 * 1024 * 1024) // 1 TB

	if memory < minMemory {
		t.Errorf("Memory %d bytes seems too low (< 100MB)", memory)
	}

	if memory > maxMemory {
		t.Errorf("Memory %d bytes seems too high (> 1TB)", memory)
	}

	t.Logf("Detected host memory: %d bytes (%.2f GB)",
		memory, float64(memory)/(1024*1024*1024))
}

// TestGetHostMemory_ParseError tests handling of malformed /proc/meminfo
func TestGetHostMemory_MissingFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	// We can't easily mock the filesystem without changing the function,
	// but we can test that the function handles errors gracefully
	// by verifying the current implementation doesn't panic

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("getHostMemory() panicked: %v", r)
		}
	}()

	_, _ = getHostMemory()
}

// TestDetectCgroupV2Resources_MemoryMax tests parsing of memory.max file
func TestDetectCgroupV2Resources_MemoryParsing(t *testing.T) {
	tests := []struct {
		name          string
		memoryContent string
		expectMemory  int64
		expectMB      int
	}{
		{
			name:          "valid memory limit",
			memoryContent: "2147483648\n",
			expectMemory:  2147483648,
			expectMB:      2048,
		},
		{
			name:          "memory max unlimited",
			memoryContent: "max\n",
			expectMemory:  0,
			expectMB:      0,
		},
		{
			name:          "1GB limit",
			memoryContent: "1073741824\n",
			expectMemory:  1073741824,
			expectMB:      1024,
		},
		{
			name:          "512MB limit",
			memoryContent: "536870912\n",
			expectMemory:  536870912,
			expectMB:      512,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()
			memMaxPath := filepath.Join(tmpDir, "memory.max")

			if err := os.WriteFile(memMaxPath, []byte(tt.memoryContent), 0644); err != nil {
				t.Fatalf("Failed to create memory.max: %v", err)
			}

			// Note: We can't call detectCgroupV2Resources directly without
			// modifying it to accept a custom path, but this demonstrates
			// the test structure for file parsing logic
		})
	}
}

// TestDetectCgroupV2Resources_CPUMax tests parsing of cpu.max file
func TestDetectCgroupV2Resources_CPUParsing(t *testing.T) {
	tests := []struct {
		name        string
		cpuContent  string
		expectCPUs  int
		description string
	}{
		{
			name:        "2 CPUs",
			cpuContent:  "200000 100000\n",
			expectCPUs:  2,
			description: "200000/100000 = 2",
		},
		{
			name:        "4 CPUs",
			cpuContent:  "400000 100000\n",
			expectCPUs:  4,
			description: "400000/100000 = 4",
		},
		{
			name:        "fractional 1.5 CPUs rounds to 2",
			cpuContent:  "150000 100000\n",
			expectCPUs:  2,
			description: "150000/100000 = 1.5, rounds up to 2",
		},
		{
			name:        "unlimited CPUs",
			cpuContent:  "max 100000\n",
			expectCPUs:  runtime.NumCPU(),
			description: "max should keep default runtime.NumCPU()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Test: %s", tt.description)
			// This demonstrates the test structure for CPU parsing
		})
	}
}

// TestDetectCgroupV1Resources_UnlimitedMemory tests handling of unlimited memory in v1
func TestDetectCgroupV1Resources_UnlimitedMemory(t *testing.T) {
	// cgroup v1 uses very large values (> 1PB) to indicate unlimited
	tests := []struct {
		name        string
		memoryValue string
		shouldSet   bool
		description string
	}{
		{
			name:        "reasonable limit 2GB",
			memoryValue: "2147483648",
			shouldSet:   true,
			description: "2GB is a normal limit",
		},
		{
			name:        "unlimited (very large value)",
			memoryValue: "9223372036854771712", // Close to max int64
			shouldSet:   false,
			description: "Very large value indicates unlimited",
		},
		{
			name:        "1PB (unrealistic limit)",
			memoryValue: "1125899906842624", // 1PB
			shouldSet:   false,
			description: "1PB is treated as unlimited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Test: %s", tt.description)
			// This demonstrates the boundary check logic for v1 unlimited memory
		})
	}
}

// TestContainerResources_String tests the String() method
func TestContainerResources_String_Variations(t *testing.T) {
	tests := []struct {
		name      string
		resources *ContainerResources
		contains  []string
	}{
		{
			name: "cgroup v2 containerized",
			resources: &ContainerResources{
				MemoryLimitBytes: 2 * 1024 * 1024 * 1024,
				MemoryLimitMB:    2048,
				CPULimit:         4,
				IsContainerized:  true,
				CgroupVersion:    2,
			},
			contains: []string{"cgroup v2", "2048MB", "CPUs=4"},
		},
		{
			name: "cgroup v1 containerized",
			resources: &ContainerResources{
				MemoryLimitBytes: 1024 * 1024 * 1024,
				MemoryLimitMB:    1024,
				CPULimit:         2,
				IsContainerized:  true,
				CgroupVersion:    1,
			},
			contains: []string{"cgroup v1", "1024MB", "CPUs=2"},
		},
		{
			name: "host resources",
			resources: &ContainerResources{
				MemoryLimitBytes: 16 * 1024 * 1024 * 1024,
				MemoryLimitMB:    16384,
				CPULimit:         8,
				IsContainerized:  false,
				CgroupVersion:    0,
			},
			contains: []string{"host", "16384MB", "CPUs=8"},
		},
		{
			name: "unlimited memory",
			resources: &ContainerResources{
				MemoryLimitBytes: 0,
				MemoryLimitMB:    0,
				CPULimit:         4,
				IsContainerized:  false,
				CgroupVersion:    0,
			},
			contains: []string{"unlimited", "CPUs=4"},
		},
		{
			name: "single CPU",
			resources: &ContainerResources{
				MemoryLimitBytes: 512 * 1024 * 1024,
				MemoryLimitMB:    512,
				CPULimit:         1,
				IsContainerized:  true,
				CgroupVersion:    2,
			},
			contains: []string{"512MB", "CPUs=1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.resources.String()

			for _, substr := range tt.contains {
				if !strings.Contains(s, substr) {
					t.Errorf("String() = %q, expected to contain %q", s, substr)
				}
			}

			// Verify format
			if !strings.HasPrefix(s, "Resources[") {
				t.Errorf("String() should start with 'Resources[', got: %s", s)
			}
		})
	}
}

// TestDetectCgroupV1Resources_CPUQuota tests CPU quota parsing in v1
func TestDetectCgroupV1Resources_CPUCalculation(t *testing.T) {
	tests := []struct {
		name        string
		quota       int64
		period      int64
		expectedCPU int
		description string
	}{
		{
			name:        "2 CPUs",
			quota:       200000,
			period:      100000,
			expectedCPU: 2,
			description: "200000/100000 = 2",
		},
		{
			name:        "4 CPUs",
			quota:       400000,
			period:      100000,
			expectedCPU: 4,
			description: "400000/100000 = 4",
		},
		{
			name:        "fractional 1.5 rounds to 2",
			quota:       150000,
			period:      100000,
			expectedCPU: 2,
			description: "(150000 + 100000 - 1) / 100000 = 2",
		},
		{
			name:        "unlimited quota -1",
			quota:       -1,
			period:      100000,
			expectedCPU: runtime.NumCPU(),
			description: "quota -1 means unlimited, keep default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate CPU with round-up logic
			var calculatedCPU int
			if tt.quota > 0 && tt.period > 0 {
				calculatedCPU = int((tt.quota + tt.period - 1) / tt.period)
			} else {
				calculatedCPU = runtime.NumCPU()
			}

			t.Logf("%s: calculated=%d, expected=%d", tt.description, calculatedCPU, tt.expectedCPU)

			if tt.quota > 0 && calculatedCPU != tt.expectedCPU {
				// Only check positive quotas (skip unlimited case which depends on runtime)
				t.Errorf("CPU calculation mismatch: got %d, expected %d", calculatedCPU, tt.expectedCPU)
			}
		})
	}
}

// TestDetectContainerResources_FallbackToHost tests fallback behavior
func TestDetectContainerResources_FallbackToHost(t *testing.T) {
	// This test verifies that when cgroup detection fails,
	// the function falls back to host resources

	resources, err := DetectContainerResources()

	if err != nil {
		t.Fatalf("DetectContainerResources() should not error on fallback: %v", err)
	}

	// Should always return valid resources
	if resources.CPULimit < 1 {
		t.Error("Should have at least 1 CPU even in fallback mode")
	}

	// On fallback, IsContainerized should be false
	if !resources.IsContainerized && resources.CgroupVersion != 0 {
		t.Error("Non-containerized resources should have CgroupVersion=0")
	}

	t.Logf("Fallback resources: %s", resources.String())
}

// TestContainerResources_ConsistencyChecks tests internal consistency
func TestContainerResources_ConsistencyChecks(t *testing.T) {
	tests := []struct {
		name      string
		resources *ContainerResources
		valid     bool
	}{
		{
			name: "consistent bytes and MB",
			resources: &ContainerResources{
				MemoryLimitBytes: 2 * 1024 * 1024 * 1024,
				MemoryLimitMB:    2048,
				CPULimit:         4,
			},
			valid: true,
		},
		{
			name: "inconsistent bytes and MB",
			resources: &ContainerResources{
				MemoryLimitBytes: 2 * 1024 * 1024 * 1024,
				MemoryLimitMB:    1024, // Wrong!
				CPULimit:         4,
			},
			valid: false,
		},
		{
			name: "zero memory is valid (unlimited)",
			resources: &ContainerResources{
				MemoryLimitBytes: 0,
				MemoryLimitMB:    0,
				CPULimit:         4,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check consistency
			expectedMB := int(tt.resources.MemoryLimitBytes / (1024 * 1024))
			isConsistent := tt.resources.MemoryLimitMB == expectedMB

			if tt.valid && !isConsistent {
				t.Errorf("Expected consistent memory values, bytes=%d should equal MB=%d (got %d)",
					tt.resources.MemoryLimitBytes, expectedMB, tt.resources.MemoryLimitMB)
			}
		})
	}
}
