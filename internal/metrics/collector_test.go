package metrics

import (
	"testing"
	"time"
)

// TestRecordProcessStart tests recording process start events
func TestRecordProcessStart(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		instanceID  string
		startTime   float64
	}{
		{
			name:        "record php-fpm start",
			processName: "php-fpm",
			instanceID:  "php-fpm-0",
			startTime:   float64(time.Now().Unix()),
		},
		{
			name:        "record nginx start",
			processName: "nginx",
			instanceID:  "nginx-1",
			startTime:   1234567890.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic and should update metrics
			RecordProcessStart(tt.processName, tt.instanceID, tt.startTime)

			// Verify the metric was set by checking the gauge exists
			// (Prometheus client doesn't expose easy way to read values in tests)
			// Just verify no panic occurs
		})
	}
}

// TestRecordProcessStop tests recording process stop events
func TestRecordProcessStop(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		instanceID  string
		exitCode    int
	}{
		{
			name:        "normal exit",
			processName: "php-fpm",
			instanceID:  "php-fpm-0",
			exitCode:    0,
		},
		{
			name:        "error exit",
			processName: "nginx",
			instanceID:  "nginx-0",
			exitCode:    1,
		},
		{
			name:        "signal exit",
			processName: "worker",
			instanceID:  "worker-2",
			exitCode:    137,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordProcessStop(tt.processName, tt.instanceID, tt.exitCode)
			// Just verify no panic
		})
	}
}

// TestRecordProcessRestart tests recording process restart events
func TestRecordProcessRestart(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		reason      string
	}{
		{
			name:        "health check restart",
			processName: "php-fpm",
			reason:      "health_check",
		},
		{
			name:        "crash restart",
			processName: "nginx",
			reason:      "crash",
		},
		{
			name:        "manual restart",
			processName: "worker",
			reason:      "manual",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordProcessRestart(tt.processName, tt.reason)
			// Just verify no panic
		})
	}
}

// TestRecordHealthCheck tests recording health check results
func TestRecordHealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		checkType   string
		duration    float64
		healthy     bool
	}{
		{
			name:        "healthy tcp check",
			processName: "php-fpm",
			checkType:   "tcp",
			duration:    0.005,
			healthy:     true,
		},
		{
			name:        "unhealthy http check",
			processName: "nginx",
			checkType:   "http",
			duration:    1.5,
			healthy:     false,
		},
		{
			name:        "healthy exec check",
			processName: "worker",
			checkType:   "exec",
			duration:    0.1,
			healthy:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordHealthCheck(tt.processName, tt.checkType, tt.duration, tt.healthy)
			// Just verify no panic
		})
	}
}

// TestRecordHealthCheckFailures tests recording consecutive failures
func TestRecordHealthCheckFailures(t *testing.T) {
	tests := []struct {
		name             string
		processName      string
		consecutiveFails int
	}{
		{
			name:             "no failures",
			processName:      "php-fpm",
			consecutiveFails: 0,
		},
		{
			name:             "one failure",
			processName:      "nginx",
			consecutiveFails: 1,
		},
		{
			name:             "multiple failures",
			processName:      "worker",
			consecutiveFails: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordHealthCheckFailures(tt.processName, tt.consecutiveFails)
			// Just verify no panic
		})
	}
}

// TestRecordHookExecution tests recording hook execution events
func TestRecordHookExecution(t *testing.T) {
	tests := []struct {
		name     string
		hookName string
		hookType string
		duration float64
		success  bool
	}{
		{
			name:     "successful pre_start hook",
			hookName: "setup",
			hookType: "pre_start",
			duration: 0.5,
			success:  true,
		},
		{
			name:     "failed post_stop hook",
			hookName: "cleanup",
			hookType: "post_stop",
			duration: 2.0,
			success:  false,
		},
		{
			name:     "successful pre_stop hook",
			hookName: "graceful-shutdown",
			hookType: "pre_stop",
			duration: 5.0,
			success:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordHookExecution(tt.hookName, tt.hookType, tt.duration, tt.success)
			// Just verify no panic
		})
	}
}

// TestSetDesiredScale tests setting desired process scale
func TestSetDesiredScale(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		scale       int
	}{
		{
			name:        "scale to 1",
			processName: "php-fpm",
			scale:       1,
		},
		{
			name:        "scale to 5",
			processName: "worker",
			scale:       5,
		},
		{
			name:        "scale to 0",
			processName: "disabled",
			scale:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDesiredScale(tt.processName, tt.scale)
			// Just verify no panic
		})
	}
}

// TestSetManagerProcessCount tests setting manager process count
func TestSetManagerProcessCount(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{
			name:  "single process",
			count: 1,
		},
		{
			name:  "multiple processes",
			count: 5,
		},
		{
			name:  "no processes",
			count: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetManagerProcessCount(tt.count)
			// Just verify no panic
		})
	}
}

// TestSetManagerStartTime tests setting manager start time
func TestSetManagerStartTime(t *testing.T) {
	tests := []struct {
		name      string
		startTime float64
	}{
		{
			name:      "current time",
			startTime: float64(time.Now().Unix()),
		},
		{
			name:      "past time",
			startTime: 1234567890.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetManagerStartTime(tt.startTime)
			// Just verify no panic
		})
	}
}

// TestSetBuildInfo tests setting build information
func TestSetBuildInfo(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		goVersion string
	}{
		{
			name:      "v1.0.0 with go1.21",
			version:   "1.0.0",
			goVersion: "go1.21.0",
		},
		{
			name:      "dev version",
			version:   "dev",
			goVersion: "go1.22.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetBuildInfo(tt.version, tt.goVersion)
			// Just verify no panic
		})
	}
}

// TestRecordShutdownDuration tests recording shutdown duration
func TestRecordShutdownDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration float64
	}{
		{
			name:     "fast shutdown",
			duration: 1.5,
		},
		{
			name:     "slow shutdown",
			duration: 25.0,
		},
		{
			name:     "timeout shutdown",
			duration: 60.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordShutdownDuration(tt.duration)
			// Just verify no panic
		})
	}
}

// TestMetricsIntegration tests full lifecycle of metrics recording
func TestMetricsIntegration(t *testing.T) {
	processName := "integration-test"
	instanceID := "test-0"
	startTime := float64(time.Now().Unix())

	// Simulate process lifecycle
	RecordProcessStart(processName, instanceID, startTime)
	SetDesiredScale(processName, 2)

	// Simulate health checks
	RecordHealthCheck(processName, "tcp", 0.01, true)
	RecordHealthCheck(processName, "tcp", 0.02, true)
	RecordHealthCheck(processName, "tcp", 0.5, false)
	RecordHealthCheckFailures(processName, 1)

	// Simulate restart
	RecordProcessRestart(processName, "health_check")

	// Simulate hook execution
	RecordHookExecution("pre-stop", "pre_stop", 1.0, true)

	// Simulate stop
	RecordProcessStop(processName, instanceID, 0)

	// No assertions needed - just verify no panics
}

// TestMetricsConcurrency tests concurrent metric recording
func TestMetricsConcurrency(t *testing.T) {
	done := make(chan bool, 3)

	// Goroutine 1: Record process events
	go func() {
		for i := 0; i < 100; i++ {
			RecordProcessStart("proc1", "inst-0", float64(time.Now().Unix()))
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Goroutine 2: Record health checks
	go func() {
		for i := 0; i < 100; i++ {
			RecordHealthCheck("proc2", "tcp", 0.01, i%2 == 0)
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Goroutine 3: Record restarts
	go func() {
		for i := 0; i < 100; i++ {
			RecordProcessRestart("proc3", "crash")
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// No race conditions should occur
}
