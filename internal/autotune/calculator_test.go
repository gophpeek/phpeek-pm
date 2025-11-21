package autotune

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

// mockResources creates a mock ContainerResources for testing
func mockResources(memoryMB, cpus int) *ContainerResources {
	return &ContainerResources{
		MemoryLimitBytes: int64(memoryMB) * 1024 * 1024,
		MemoryLimitMB:    memoryMB,
		CPULimit:         cpus,
		IsContainerized:  true,
		CgroupVersion:    2,
	}
}

// mockCalculator creates a calculator with mocked resources
func mockCalculator(profile Profile, memoryMB, cpus int) *Calculator {
	profileConfig, _ := profile.GetConfig()
	return &Calculator{
		resources:       mockResources(memoryMB, cpus),
		profile:         profileConfig,
		memoryThreshold: 0, // Use profile default
		logger:          slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

func TestCalculator_DevProfile(t *testing.T) {
	calc := mockCalculator(ProfileDev, 512, 2)
	cfg, err := calc.Calculate()

	if err != nil {
		t.Fatalf("Calculate() failed: %v", err)
	}

	// Dev profile should use exactly 2 workers (static)
	if cfg.MaxChildren != 2 {
		t.Errorf("Expected 2 workers for dev profile, got %d", cfg.MaxChildren)
	}

	if cfg.ProcessManager != "static" {
		t.Errorf("Expected static PM for dev profile, got %s", cfg.ProcessManager)
	}

	// Memory check: 2 workers * 48MB = 96MB + 64MB OPcache + 64MB reserved = 224MB total
	totalExpected := 2*48 + 64 + 64 // workers + OPcache + reserved
	totalUsed := cfg.MemoryAllocated + cfg.MemoryOPcache + cfg.MemoryReserved
	if totalUsed > 512 {
		t.Errorf("Memory usage %dMB exceeds available 512MB", totalUsed)
	}

	if totalUsed > totalExpected*2 { // Allow some buffer
		t.Errorf("Memory usage too high: workers=%dMB, opcache=%dMB, reserved=%dMB, total=%dMB",
			cfg.MemoryAllocated, cfg.MemoryOPcache, cfg.MemoryReserved, totalUsed)
	}
}

func TestCalculator_MediumProfile(t *testing.T) {
	// Medium profile: 2GB RAM, 4 CPUs
	calc := mockCalculator(ProfileMedium, 2048, 4)
	cfg, err := calc.Calculate()

	if err != nil {
		t.Fatalf("Calculate() failed: %v", err)
	}

	// Available memory: 2048 * 0.75 = 1536MB
	// Reserved: 192MB (system) + 128MB (OPcache) = 320MB
	// Worker memory: 1536 - 320 = 1216MB
	// Workers: 1216 / 42MB = 28.9 → 28 workers
	// CPU limit: 4 CPUs * 4 = 16 max workers (CPU LIMITING!)
	expectedWorkers := 16
	if cfg.MaxChildren != expectedWorkers {
		t.Errorf("Expected %d workers, got %d", expectedWorkers, cfg.MaxChildren)
	}

	if cfg.ProcessManager != "dynamic" {
		t.Errorf("Expected dynamic PM for medium profile, got %s", cfg.ProcessManager)
	}

	// Validate dynamic PM relationships
	if cfg.MinSpare > cfg.MaxSpare {
		t.Errorf("min_spare (%d) > max_spare (%d)", cfg.MinSpare, cfg.MaxSpare)
	}

	if cfg.StartServers < cfg.MinSpare || cfg.StartServers > cfg.MaxSpare {
		t.Errorf("start_servers (%d) not between min_spare (%d) and max_spare (%d)",
			cfg.StartServers, cfg.MinSpare, cfg.MaxSpare)
	}

	// Total memory check (including OPcache)
	totalUsed := cfg.MemoryAllocated + cfg.MemoryOPcache + cfg.MemoryReserved
	if totalUsed > cfg.MemoryTotal {
		t.Errorf("Total memory usage %dMB exceeds available %dMB", totalUsed, cfg.MemoryTotal)
	}
}

func TestCalculator_HeavyProfile(t *testing.T) {
	// Heavy profile: 8GB RAM, 8 CPUs
	calc := mockCalculator(ProfileHeavy, 8192, 8)
	cfg, err := calc.Calculate()

	if err != nil {
		t.Fatalf("Calculate() failed: %v", err)
	}

	// Available memory: 8192 * 0.8 = 6553MB
	// Reserved: 384MB (system) + 256MB (OPcache) = 640MB
	// Worker memory: 6553 - 640 = 5913MB
	// Workers: 5913 / 128MB = 46.1 → 46 workers
	// CPU limit: 8 CPUs * 4 = 32 max workers (limiting!)
	expectedWorkers := 32
	if cfg.MaxChildren != expectedWorkers {
		t.Errorf("Expected %d workers, got %d", expectedWorkers, cfg.MaxChildren)
	}

	// Minimum check from profile
	if cfg.MaxChildren < 8 {
		t.Errorf("Heavy profile should have at least 8 workers, got %d", cfg.MaxChildren)
	}

	// Memory safety (including OPcache)
	totalUsed := cfg.MemoryAllocated + cfg.MemoryOPcache + cfg.MemoryReserved
	if totalUsed > cfg.MemoryTotal {
		t.Errorf("Memory overprovisioned: %dMB used, %dMB available", totalUsed, cfg.MemoryTotal)
	}
}

func TestCalculator_CPULimit(t *testing.T) {
	// Test CPU limiting: 4GB RAM but only 1 CPU
	calc := mockCalculator(ProfileMedium, 4096, 1)
	cfg, err := calc.Calculate()

	if err != nil {
		t.Fatalf("Calculate() failed: %v", err)
	}

	// CPU limit: 1 CPU * 4 = 4 workers max
	// Memory would allow: (4096*0.75 - 320) / 80 = 34 workers (medium profile)
	// Should be limited to 4 by CPU
	if cfg.MaxChildren > 4 {
		t.Errorf("Expected CPU limit to cap workers at 4, got %d", cfg.MaxChildren)
	}

	if len(cfg.Warnings) == 0 {
		t.Error("Expected warning about CPU limiting, got none")
	}
}

func TestCalculator_InsufficientMemory(t *testing.T) {
	// 64MB RAM - insufficient for any profile
	calc := mockCalculator(ProfileMedium, 64, 2)
	_, err := calc.Calculate()

	if err == nil {
		t.Error("Expected error for insufficient memory, got none")
	}
}

func TestCalculator_MinimalMemory(t *testing.T) {
	// Test with 768MB - minimal for light profile
	// Available: 768 * 0.7 = 537MB
	// Reserved: 128MB (system) + 96MB (OPcache) = 224MB
	// Worker memory: 537 - 224 = 313MB
	// Workers: 313 / 64MB = 4.8 → 4 workers
	calc := mockCalculator(ProfileLight, 768, 1)
	cfg, err := calc.Calculate()

	if err != nil {
		t.Fatalf("Calculate() failed with minimal memory: %v", err)
	}

	// Should get at least minimum workers
	if cfg.MaxChildren < 2 {
		t.Errorf("Expected at least 2 workers for light profile, got %d", cfg.MaxChildren)
	}

	// Must not exceed available memory (including OPcache)
	totalUsed := cfg.MemoryAllocated + cfg.MemoryOPcache + cfg.MemoryReserved
	if totalUsed > cfg.MemoryTotal {
		t.Errorf("Memory overprovisioned: %dMB used, %dMB available", totalUsed, cfg.MemoryTotal)
	}
}

func TestCalculator_BurstyProfile(t *testing.T) {
	// Use more memory to get more workers and show difference
	calc := mockCalculator(ProfileBursty, 4096, 4)
	cfg, err := calc.Calculate()

	if err != nil {
		t.Fatalf("Calculate() failed: %v", err)
	}

	// Bursty should have higher spare ratios
	spareRange := cfg.MaxSpare - cfg.MinSpare
	if spareRange < 2 {
		t.Errorf("Expected significant spare range for bursty profile, got min=%d max=%d",
			cfg.MinSpare, cfg.MaxSpare)
	}

	// Bursty should start with at least 50% of max workers
	expectedMinStart := cfg.MaxChildren / 2
	if cfg.StartServers < expectedMinStart {
		t.Errorf("Expected start_servers >= %d (50%% of max_children) for bursty profile, got %d",
			expectedMinStart, cfg.StartServers)
	}
}

func TestPHPFPMConfig_ToEnvVars(t *testing.T) {
	cfg := &PHPFPMConfig{
		ProcessManager: "dynamic",
		MaxChildren:    10,
		StartServers:   3,
		MinSpare:       2,
		MaxSpare:       5,
		MaxRequests:    1000,
	}

	env := cfg.ToEnvVars()

	tests := []struct {
		key      string
		expected string
	}{
		{"PHP_FPM_PM", "dynamic"},
		{"PHP_FPM_MAX_CHILDREN", "10"},
		{"PHP_FPM_START_SERVERS", "3"},
		{"PHP_FPM_MIN_SPARE", "2"},
		{"PHP_FPM_MAX_SPARE", "5"},
		{"PHP_FPM_MAX_REQUESTS", "1000"},
	}

	for _, tt := range tests {
		if got := env[tt.key]; got != tt.expected {
			t.Errorf("env[%s] = %s, expected %s", tt.key, got, tt.expected)
		}
	}
}

func TestPHPFPMConfig_ToEnvVars_Static(t *testing.T) {
	cfg := &PHPFPMConfig{
		ProcessManager: "static",
		MaxChildren:    2,
		MaxRequests:    100,
	}

	env := cfg.ToEnvVars()

	// Static mode should not have spare/start server vars
	if _, exists := env["PHP_FPM_START_SERVERS"]; exists {
		t.Error("Static mode should not have START_SERVERS env var")
	}

	if _, exists := env["PHP_FPM_MIN_SPARE"]; exists {
		t.Error("Static mode should not have MIN_SPARE env var")
	}
}

func TestCalculator_NoContainerLimits(t *testing.T) {
	// Mock host resources (not containerized)
	profileConfig, _ := ProfileMedium.GetConfig()
	resources := &ContainerResources{
		MemoryLimitBytes: 8 * 1024 * 1024 * 1024, // 8GB host memory
		MemoryLimitMB:    8 * 1024,
		CPULimit:         4,
		IsContainerized:  false, // ← Not in container!
		CgroupVersion:    0,
	}

	calc := &Calculator{
		resources:       resources,
		profile:         profileConfig,
		memoryThreshold: 0, // Use profile default
		logger:          slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	cfg, err := calc.Calculate()

	// Should succeed but with warnings
	if err != nil {
		t.Errorf("Should not error on host resources, got: %v", err)
	}

	// Should have warning about using host resources
	if len(cfg.Warnings) == 0 {
		t.Error("Expected warning about host resources, got none")
	}

	foundWarning := false
	for _, w := range cfg.Warnings {
		if strings.Contains(w, "WITHOUT container limits") {
			foundWarning = true
			t.Logf("Warning found: %s", w)
		}
	}

	if !foundWarning {
		t.Error("Expected warning about container limits, but not found")
	}

	// Should still calculate workers (using host memory)
	if cfg.MaxChildren == 0 {
		t.Error("Should calculate workers even without container limits")
	}

	t.Logf("Auto-tuning on host: %d workers with warnings", cfg.MaxChildren)
}

func TestCalculator_MemoryThresholdOverride(t *testing.T) {
	// Test with conservative threshold (60%)
	profileConfig, _ := ProfileMedium.GetConfig()
	calc := &Calculator{
		resources:       mockResources(2048, 4),
		profile:         profileConfig,
		memoryThreshold: 0.6, // 60% instead of profile default (75%)
		logger:          slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	cfg, err := calc.Calculate()
	if err != nil {
		t.Fatalf("Calculate() failed: %v", err)
	}

	// With 60% threshold: 2048 * 0.6 = 1228MB
	// Reserved: 320MB → Worker pool: 908MB
	// Workers: 908 / 42MB = 21 workers
	// CPU limit: 4 * 4 = 16 workers (LIMITED)
	expectedWorkers := 16

	if cfg.MaxChildren != expectedWorkers {
		t.Errorf("Expected %d workers with 60%% threshold, got %d", expectedWorkers, cfg.MaxChildren)
	}

	t.Logf("Conservative threshold (60%%): %d workers", cfg.MaxChildren)
}

func TestCalculator_MemoryThresholdOversubscription(t *testing.T) {
	// Test with oversubscription (130% - DANGEROUS!)
	profileConfig, _ := ProfileMedium.GetConfig()
	calc := &Calculator{
		resources:       mockResources(2048, 4),
		profile:         profileConfig,
		memoryThreshold: 1.3, // 130% - allows oversubscription!
		logger:          slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	cfg, err := calc.Calculate()
	if err != nil {
		t.Fatalf("Calculate() failed: %v", err)
	}

	// With 130% threshold: 2048 * 1.3 = 2662MB (MORE than container has!)
	// Reserved: 320MB → Worker pool: 2342MB
	// Workers: 2342 / 42MB = 55 workers
	// CPU limit: 4 * 4 = 16 workers (LIMITED)
	expectedWorkers := 16

	if cfg.MaxChildren != expectedWorkers {
		t.Errorf("Expected %d workers with oversubscription, got %d", expectedWorkers, cfg.MaxChildren)
	}

	// Should have warning about oversubscription
	foundWarning := false
	for _, w := range cfg.Warnings {
		if strings.Contains(w, "OVERSUBSCRIPTION") || strings.Contains(w, ">100%") {
			foundWarning = true
			t.Logf("Oversubscription warning: %s", w)
		}
	}

	if !foundWarning {
		t.Error("Expected warning about oversubscription (>100%), but not found")
	}

	t.Logf("Oversubscription threshold (130%%): %d workers with DANGER warning", cfg.MaxChildren)
}

func TestCalculator_MemoryThresholdLow(t *testing.T) {
	// Test with very low threshold (20% - wasteful)
	profileConfig, _ := ProfileMedium.GetConfig()
	calc := &Calculator{
		resources:       mockResources(2048, 4),
		profile:         profileConfig,
		memoryThreshold: 0.2, // 20% - very conservative
		logger:          slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	cfg, err := calc.Calculate()
	if err != nil {
		t.Fatalf("Calculate() failed: %v", err)
	}

	// Should have warning about being too conservative
	foundWarning := false
	for _, w := range cfg.Warnings {
		if strings.Contains(w, "very conservative") {
			foundWarning = true
			t.Logf("Conservative warning: %s", w)
		}
	}

	if !foundWarning {
		t.Error("Expected warning about very conservative threshold (<30%), but not found")
	}

	t.Logf("Low threshold (20%%): %d workers with conservation warning", cfg.MaxChildren)
}

func TestCalculator_SafetyValidations(t *testing.T) {
	tests := []struct {
		name      string
		profile   Profile
		memoryMB  int
		cpus      int
		wantError bool
	}{
		{"Sufficient resources", ProfileMedium, 2048, 4, false},
		{"Minimal resources", ProfileLight, 768, 1, false}, // 768*0.7=537, 537-224=313, 313/64=4 workers
		{"Dev tiny", ProfileDev, 384, 1, false},            // 384*0.5=192, 192-128=64, 64/48=1 worker (min 2 enforced)
		{"Insufficient memory", ProfileMedium, 100, 4, true},
		{"Too small", ProfileHeavy, 256, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := mockCalculator(tt.profile, tt.memoryMB, tt.cpus)
			cfg, err := calc.Calculate()

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for %s, got none", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Validate no memory overprovisioning (including OPcache)
			totalUsed := cfg.MemoryAllocated + cfg.MemoryOPcache + cfg.MemoryReserved
			if totalUsed > cfg.MemoryTotal {
				t.Errorf("SAFETY VIOLATION: Memory overprovisioned: %dMB used, %dMB available",
					totalUsed, cfg.MemoryTotal)
			}

			// Validate PM relationships
			if cfg.ProcessManager == "dynamic" {
				if cfg.MinSpare > cfg.MaxChildren {
					t.Errorf("SAFETY VIOLATION: min_spare > max_children")
				}
				if cfg.MaxSpare > cfg.MaxChildren {
					t.Errorf("SAFETY VIOLATION: max_spare > max_children")
				}
				if cfg.MinSpare > cfg.MaxSpare {
					t.Errorf("SAFETY VIOLATION: min_spare > max_spare")
				}
			}
		})
	}
}
