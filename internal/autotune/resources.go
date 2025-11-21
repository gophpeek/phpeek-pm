package autotune

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// ContainerResources represents detected container resource limits
type ContainerResources struct {
	MemoryLimitBytes int64 // Total memory limit in bytes (0 = unlimited)
	MemoryLimitMB    int   // Memory limit in MB for convenience
	CPULimit         int   // Number of CPU cores (including fractional via quota/period)
	IsContainerized  bool  // True if running in a container with limits
	CgroupVersion    int   // 1 or 2
}

// DetectContainerResources reads cgroup v1/v2 to determine container limits
// Returns host resources if not containerized or limits not set
func DetectContainerResources() (*ContainerResources, error) {
	resources := &ContainerResources{
		CPULimit: runtime.NumCPU(), // Default to host CPUs
	}

	// Try cgroup v2 first (newer, unified hierarchy)
	if cgroupV2Exists() {
		if err := detectCgroupV2Resources(resources); err == nil {
			resources.CgroupVersion = 2
			resources.IsContainerized = true
			return resources, nil
		}
	}

	// Fallback to cgroup v1
	if cgroupV1Exists() {
		if err := detectCgroupV1Resources(resources); err == nil {
			resources.CgroupVersion = 1
			resources.IsContainerized = true
			return resources, nil
		}
	}

	// Not containerized or limits not detected - use host resources
	resources.IsContainerized = false
	if memTotal, err := getHostMemory(); err == nil {
		resources.MemoryLimitBytes = memTotal
		resources.MemoryLimitMB = int(memTotal / (1024 * 1024))
	}

	return resources, nil
}

// cgroupV2Exists checks if cgroup v2 is available
func cgroupV2Exists() bool {
	_, err := os.Stat("/sys/fs/cgroup/cgroup.controllers")
	return err == nil
}

// cgroupV1Exists checks if cgroup v1 is available
func cgroupV1Exists() bool {
	_, err := os.Stat("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	return err == nil
}

// detectCgroupV2Resources reads cgroup v2 unified hierarchy
func detectCgroupV2Resources(r *ContainerResources) error {
	// Read memory limit from cgroup v2
	memMax, err := os.ReadFile("/sys/fs/cgroup/memory.max")
	if err == nil {
		memStr := strings.TrimSpace(string(memMax))
		if memStr != "max" {
			if memLimit, err := strconv.ParseInt(memStr, 10, 64); err == nil {
				r.MemoryLimitBytes = memLimit
				r.MemoryLimitMB = int(memLimit / (1024 * 1024))
			}
		}
	}

	// Read CPU quota and period from cgroup v2
	cpuMax, err := os.ReadFile("/sys/fs/cgroup/cpu.max")
	if err == nil {
		parts := strings.Fields(strings.TrimSpace(string(cpuMax)))
		if len(parts) == 2 && parts[0] != "max" {
			quota, err1 := strconv.ParseInt(parts[0], 10, 64)
			period, err2 := strconv.ParseInt(parts[1], 10, 64)
			if err1 == nil && err2 == nil && period > 0 {
				// CPU limit = quota / period (fractional CPUs supported)
				cpus := int((quota + period - 1) / period) // Round up
				if cpus > 0 {
					r.CPULimit = cpus
				}
			}
		}
	}

	// If memory limit found, consider it a success
	if r.MemoryLimitBytes > 0 {
		return nil
	}

	return fmt.Errorf("cgroup v2 limits not found")
}

// detectCgroupV1Resources reads cgroup v1 hierarchy
func detectCgroupV1Resources(r *ContainerResources) error {
	// Read memory limit from cgroup v1
	memLimit, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err == nil {
		memStr := strings.TrimSpace(string(memLimit))
		if limit, err := strconv.ParseInt(memStr, 10, 64); err == nil {
			// cgroup v1 sets very large values when unlimited
			if limit < (1 << 50) { // Less than ~1PB = realistic limit
				r.MemoryLimitBytes = limit
				r.MemoryLimitMB = int(limit / (1024 * 1024))
			}
		}
	}

	// Read CPU quota from cgroup v1
	quota, err := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	if err == nil {
		quotaStr := strings.TrimSpace(string(quota))
		if quotaVal, err := strconv.ParseInt(quotaStr, 10, 64); err == nil && quotaVal > 0 {
			// Read CPU period
			period, err := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
			if err == nil {
				periodStr := strings.TrimSpace(string(period))
				if periodVal, err := strconv.ParseInt(periodStr, 10, 64); err == nil && periodVal > 0 {
					// CPU limit = quota / period
					cpus := int((quotaVal + periodVal - 1) / periodVal) // Round up
					if cpus > 0 {
						r.CPULimit = cpus
					}
				}
			}
		}
	}

	// If memory limit found, consider it a success
	if r.MemoryLimitBytes > 0 {
		return nil
	}

	return fmt.Errorf("cgroup v1 limits not found")
}

// getHostMemory reads total system memory from /proc/meminfo
func getHostMemory() (int64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil {
					return kb * 1024, nil // Convert KB to bytes
				}
			}
		}
	}

	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

// String returns a human-readable representation of the resources
func (r *ContainerResources) String() string {
	source := "host"
	if r.IsContainerized {
		source = fmt.Sprintf("cgroup v%d", r.CgroupVersion)
	}

	memStr := "unlimited"
	if r.MemoryLimitMB > 0 {
		memStr = fmt.Sprintf("%dMB", r.MemoryLimitMB)
	}

	return fmt.Sprintf("Resources[%s]: Memory=%s, CPUs=%d", source, memStr, r.CPULimit)
}
