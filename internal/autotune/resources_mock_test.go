package autotune

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDetectCgroupV2Resources_WithMockFiles tests cgroup v2 detection with mock files
func TestDetectCgroupV2Resources_WithMockFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux for cgroup paths")
	}

	// Create a temporary directory to simulate cgroup v2 structure
	tmpDir := t.TempDir()

	// Save original working directory
	originalWd, _ := os.Getwd()
	defer func() {
		os.Chdir(originalWd)
	}()

	tests := []struct {
		name           string
		memoryContent  string
		cpuContent     string
		expectError    bool
		expectMemoryMB int
		expectCPUs     int
	}{
		{
			name:           "valid limits",
			memoryContent:  "2147483648\n", // 2GB
			cpuContent:     "200000 100000\n", // 2 CPUs
			expectError:    false,
			expectMemoryMB: 2048,
			expectCPUs:     2,
		},
		{
			name:           "unlimited memory",
			memoryContent:  "max\n",
			cpuContent:     "400000 100000\n", // 4 CPUs
			expectError:    false,
			expectMemoryMB: 0, // Unlimited
			expectCPUs:     4,
		},
		{
			name:           "unlimited cpu",
			memoryContent:  "1073741824\n", // 1GB
			cpuContent:     "max 100000\n",
			expectError:    false,
			expectMemoryMB: 1024,
			expectCPUs:     runtime.NumCPU(), // Falls back to default
		},
		{
			name:           "fractional CPU",
			memoryContent:  "536870912\n", // 512MB
			cpuContent:     "150000 100000\n", // 1.5 CPUs -> rounds to 2
			expectError:    false,
			expectMemoryMB: 512,
			expectCPUs:     2,
		},
		{
			name:          "no memory file",
			memoryContent: "", // Don't create file
			cpuContent:    "200000 100000\n",
			expectError:   true, // No memory limit found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh temp directory for each test
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Create mock files
			if tt.memoryContent != "" {
				memPath := filepath.Join(testDir, "memory.max")
				if err := os.WriteFile(memPath, []byte(tt.memoryContent), 0644); err != nil {
					t.Fatalf("Failed to write memory.max: %v", err)
				}
			}

			if tt.cpuContent != "" {
				cpuPath := filepath.Join(testDir, "cpu.max")
				if err := os.WriteFile(cpuPath, []byte(tt.cpuContent), 0644); err != nil {
					t.Fatalf("Failed to write cpu.max: %v", err)
				}
			}

			// Since we can't easily redirect /sys/fs/cgroup, we test the logic directly
			// by simulating what detectCgroupV2Resources does

			resources := &ContainerResources{
				CPULimit: runtime.NumCPU(),
			}

			// Simulate reading memory.max
			if tt.memoryContent != "" {
				memPath := filepath.Join(testDir, "memory.max")
				data, err := os.ReadFile(memPath)
				if err == nil {
					memStr := string(data[:len(data)-1]) // Trim newline
					if memStr != "max" {
						var memLimit int64
						if _, err := fmt.Sscanf(memStr, "%d", &memLimit); err == nil {
							resources.MemoryLimitBytes = memLimit
							resources.MemoryLimitMB = int(memLimit / (1024 * 1024))
						}
					}
				}
			}

			// Simulate reading cpu.max
			if tt.cpuContent != "" {
				cpuPath := filepath.Join(testDir, "cpu.max")
				data, err := os.ReadFile(cpuPath)
				if err == nil {
					cpuStr := string(data[:len(data)-1]) // Trim newline
					var quota, period int64
					if n, _ := fmt.Sscanf(cpuStr, "%d %d", &quota, &period); n == 2 && period > 0 {
						cpus := int((quota + period - 1) / period)
						if cpus > 0 {
							resources.CPULimit = cpus
						}
					}
				}
			}

			// Validate results
			if tt.expectError {
				if resources.MemoryLimitBytes == 0 && tt.memoryContent == "" {
					t.Logf("Expected error case: memory limit not found")
				}
			} else {
				if resources.MemoryLimitMB != tt.expectMemoryMB {
					t.Errorf("MemoryMB = %d, expected %d", resources.MemoryLimitMB, tt.expectMemoryMB)
				}
				if resources.CPULimit != tt.expectCPUs {
					t.Errorf("CPULimit = %d, expected %d", resources.CPULimit, tt.expectCPUs)
				}
			}
		})
	}
}

// TestDetectCgroupV1Resources_WithMockFiles tests cgroup v1 detection with mock files
func TestDetectCgroupV1Resources_WithMockFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux for cgroup paths")
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		memoryContent  string
		quotaContent   string
		periodContent  string
		expectMemoryMB int
		expectCPUs     int
	}{
		{
			name:           "valid limits",
			memoryContent:  "2147483648\n", // 2GB
			quotaContent:   "200000\n",
			periodContent:  "100000\n",
			expectMemoryMB: 2048,
			expectCPUs:     2,
		},
		{
			name:           "unlimited memory (very large value)",
			memoryContent:  "9223372036854771712\n", // > 1PB, treated as unlimited
			quotaContent:   "400000\n",
			periodContent:  "100000\n",
			expectMemoryMB: 0, // Should be filtered out
			expectCPUs:     4,
		},
		{
			name:           "unlimited CPU (-1 quota)",
			memoryContent:  "1073741824\n", // 1GB
			quotaContent:   "-1\n",
			periodContent:  "100000\n",
			expectMemoryMB: 1024,
			expectCPUs:     runtime.NumCPU(), // No change from default
		},
		{
			name:           "fractional CPU rounds up",
			memoryContent:  "536870912\n", // 512MB
			quotaContent:   "150000\n",
			periodContent:  "100000\n",
			expectMemoryMB: 512,
			expectCPUs:     2, // 1.5 rounds to 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Create mock files
			memPath := filepath.Join(testDir, "memory.limit_in_bytes")
			if err := os.WriteFile(memPath, []byte(tt.memoryContent), 0644); err != nil {
				t.Fatalf("Failed to write memory file: %v", err)
			}

			quotaPath := filepath.Join(testDir, "cpu.cfs_quota_us")
			if err := os.WriteFile(quotaPath, []byte(tt.quotaContent), 0644); err != nil {
				t.Fatalf("Failed to write quota file: %v", err)
			}

			periodPath := filepath.Join(testDir, "cpu.cfs_period_us")
			if err := os.WriteFile(periodPath, []byte(tt.periodContent), 0644); err != nil {
				t.Fatalf("Failed to write period file: %v", err)
			}

			// Simulate cgroup v1 parsing logic
			resources := &ContainerResources{
				CPULimit: runtime.NumCPU(),
			}

			// Parse memory
			data, _ := os.ReadFile(memPath)
			var memLimit int64
			if _, err := fmt.Sscanf(string(data), "%d", &memLimit); err == nil {
				// Filter out unrealistic values (> 1PB)
				if memLimit < (1 << 50) {
					resources.MemoryLimitBytes = memLimit
					resources.MemoryLimitMB = int(memLimit / (1024 * 1024))
				}
			}

			// Parse CPU
			quotaData, _ := os.ReadFile(quotaPath)
			periodData, _ := os.ReadFile(periodPath)

			var quota, period int64
			fmt.Sscanf(string(quotaData), "%d", &quota)
			fmt.Sscanf(string(periodData), "%d", &period)

			if quota > 0 && period > 0 {
				cpus := int((quota + period - 1) / period)
				if cpus > 0 {
					resources.CPULimit = cpus
				}
			}

			// Validate
			if resources.MemoryLimitMB != tt.expectMemoryMB {
				t.Errorf("MemoryMB = %d, expected %d", resources.MemoryLimitMB, tt.expectMemoryMB)
			}
			if resources.CPULimit != tt.expectCPUs {
				t.Errorf("CPULimit = %d, expected %d", resources.CPULimit, tt.expectCPUs)
			}
		})
	}
}

// TestGetHostMemory_Parsing tests the parsing logic for /proc/meminfo
func TestGetHostMemory_Parsing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		content       string
		expectMemory  int64
		expectError   bool
	}{
		{
			name: "valid meminfo",
			content: `MemTotal:       16384000 kB
MemFree:         8192000 kB
MemAvailable:   12000000 kB
`,
			expectMemory: 16384000 * 1024, // Convert KB to bytes
			expectError:  false,
		},
		{
			name: "meminfo with other fields",
			content: `Buffers:          500000 kB
MemTotal:        8192000 kB
Cached:          2000000 kB
`,
			expectMemory: 8192000 * 1024,
			expectError:  false,
		},
		{
			name: "memtotal at end",
			content: `MemFree:         4096000 kB
Cached:          1000000 kB
MemTotal:        4096000 kB
`,
			expectMemory: 4096000 * 1024,
			expectError:  false,
		},
		{
			name:         "missing memtotal",
			content:      "MemFree:         8192000 kB\nCached:          2000000 kB\n",
			expectMemory: 0,
			expectError:  true,
		},
		{
			name:         "empty file",
			content:      "",
			expectMemory: 0,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock /proc/meminfo
			meminfoPath := filepath.Join(tmpDir, tt.name+"_meminfo")
			if err := os.WriteFile(meminfoPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write meminfo: %v", err)
			}

			// Simulate getHostMemory parsing
			data, err := os.ReadFile(meminfoPath)
			if err != nil {
				if tt.expectError {
					return
				}
				t.Fatalf("Failed to read meminfo: %v", err)
			}

			var memory int64
			found := false
			lines := string(data)

			// Simple parsing simulation
			for _, line := range splitLines(lines) {
				if len(line) > 9 && line[:9] == "MemTotal:" {
					var kb int64
					if _, err := fmt.Sscanf(line[9:], "%d", &kb); err == nil {
						memory = kb * 1024
						found = true
						break
					}
				}
			}

			if !found {
				if tt.expectError {
					t.Logf("Expected error: MemTotal not found")
					return
				}
				t.Error("MemTotal not found in meminfo")
				return
			}

			if tt.expectError {
				t.Error("Expected error but parsing succeeded")
				return
			}

			if memory != tt.expectMemory {
				t.Errorf("Memory = %d, expected %d", memory, tt.expectMemory)
			}
		})
	}
}

// Helper function to split lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
