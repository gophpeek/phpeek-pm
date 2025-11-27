package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// TestFormatUptime tests uptime formatting
func TestFormatUptime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		startedAt int64
		expected  string
	}{
		{
			name:      "zero timestamp",
			startedAt: 0,
			expected:  "-",
		},
		{
			name:      "seconds only",
			startedAt: now.Add(-30 * time.Second).Unix(),
			expected:  "30s",
		},
		{
			name:      "minutes only",
			startedAt: now.Add(-5 * time.Minute).Unix(),
			expected:  "5m",
		},
		{
			name:      "hours and minutes",
			startedAt: now.Add(-2*time.Hour - 30*time.Minute).Unix(),
			expected:  "2h 30m",
		},
		{
			name:      "less than 24 hours",
			startedAt: now.Add(-20 * time.Hour).Unix(),
			expected:  "20h 0m",
		},
		{
			name:      "days and hours",
			startedAt: now.Add(-50 * time.Hour).Unix(),
			expected:  "2d 2h",
		},
		{
			name:      "multiple days",
			startedAt: now.Add(-168 * time.Hour).Unix(),
			expected:  "7d 0h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUptime(tt.startedAt)
			if result != tt.expected {
				t.Errorf("formatUptime(%d) = %s, expected %s", tt.startedAt, result, tt.expected)
			}
		})
	}
}

// TestFormatCPUUsage tests CPU usage formatting
func TestFormatCPUUsage(t *testing.T) {
	tests := []struct {
		name     string
		cpu      float64
		expected string
	}{
		{
			name:     "zero",
			cpu:      0,
			expected: "-",
		},
		{
			name:     "negative",
			cpu:      -1.5,
			expected: "-",
		},
		{
			name:     "low single digit",
			cpu:      2.5,
			expected: "2.5%",
		},
		{
			name:     "high single digit",
			cpu:      9.8,
			expected: "9.8%",
		},
		{
			name:     "double digit",
			cpu:      15.7,
			expected: "16%",
		},
		{
			name:     "high CPU",
			cpu:      99.9,
			expected: "100%",
		},
		{
			name:     "over 100 percent",
			cpu:      150.0,
			expected: "150%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCPUUsage(tt.cpu)
			if result != tt.expected {
				t.Errorf("formatCPUUsage(%f) = %s, expected %s", tt.cpu, result, tt.expected)
			}
		})
	}
}

// TestFormatMemoryUsage tests memory usage formatting
func TestFormatMemoryUsage(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "-",
		},
		{
			name:     "kilobytes",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "kilobytes with decimals",
			bytes:    2560,
			expected: "2.5 KB",
		},
		{
			name:     "megabytes",
			bytes:    1024 * 1024,
			expected: "1.0 MB",
		},
		{
			name:     "megabytes with decimals",
			bytes:    150 * 1024 * 1024,
			expected: "150.0 MB",
		},
		{
			name:     "gigabytes",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GB",
		},
		{
			name:     "gigabytes with decimals",
			bytes:    2560 * 1024 * 1024,
			expected: "2.5 GB",
		},
		{
			name:     "large gigabytes",
			bytes:    10 * 1024 * 1024 * 1024,
			expected: "10.0 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMemoryUsage(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatMemoryUsage(%d) = %s, expected %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

// TestStateDisplay tests state display formatting
func TestStateDisplay(t *testing.T) {
	tests := []struct {
		name          string
		state         string
		expectedText  string
		expectedStyle lipgloss.Style
	}{
		{
			name:          "running",
			state:         "running",
			expectedText:  "✓ Running",
			expectedStyle: successStyle,
		},
		{
			name:          "starting",
			state:         "starting",
			expectedText:  "● Starting",
			expectedStyle: highlightStyle,
		},
		{
			name:          "stopped",
			state:         "stopped",
			expectedText:  "○ Stopped",
			expectedStyle: dimStyle,
		},
		{
			name:          "failed",
			state:         "failed",
			expectedText:  "✗ Failed",
			expectedStyle: errorStyle,
		},
		{
			name:          "completed",
			state:         "completed",
			expectedText:  "✓ Completed",
			expectedStyle: successStyle,
		},
		{
			name:          "unknown state",
			state:         "pending",
			expectedText:  "pending",
			expectedStyle: dimStyle,
		},
		{
			name:          "empty state",
			state:         "",
			expectedText:  "",
			expectedStyle: dimStyle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, style := stateDisplay(tt.state)
			if text != tt.expectedText {
				t.Errorf("stateDisplay(%s) text = %s, expected %s", tt.state, text, tt.expectedText)
			}
			// Note: We can't easily compare lipgloss.Style values directly,
			// but we can verify the function returns without panicking
			_ = style
		})
	}
}

// TestHealthDisplay tests health display formatting
func TestHealthDisplay(t *testing.T) {
	tests := []struct {
		name         string
		healthy      bool
		expectedText string
	}{
		{
			name:         "healthy",
			healthy:      true,
			expectedText: "✓ Healthy",
		},
		{
			name:         "unhealthy",
			healthy:      false,
			expectedText: "⚠ Unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, style := healthDisplay(tt.healthy)
			if text != tt.expectedText {
				t.Errorf("healthDisplay(%v) text = %s, expected %s", tt.healthy, text, tt.expectedText)
			}
			_ = style // Style comparison not easily testable
		})
	}
}

// TestSplitCommandLine tests command line splitting
func TestSplitCommandLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			name:     "empty string",
			line:     "",
			expected: []string{},
		},
		{
			name:     "whitespace only",
			line:     "   \t\n  ",
			expected: []string{},
		},
		{
			name:     "single command",
			line:     "php",
			expected: []string{"php"},
		},
		{
			name:     "command with args",
			line:     "php artisan queue:work",
			expected: []string{"php", "artisan", "queue:work"},
		},
		{
			name:     "command with flags",
			line:     "php artisan queue:work --tries=3 --timeout=60",
			expected: []string{"php", "artisan", "queue:work", "--tries=3", "--timeout=60"},
		},
		{
			name:     "command with extra spaces",
			line:     "  php   artisan   queue:work  ",
			expected: []string{"php", "artisan", "queue:work"},
		},
		{
			name:     "complex command",
			line:     "/usr/bin/php -d memory_limit=512M artisan horizon",
			expected: []string{"/usr/bin/php", "-d", "memory_limit=512M", "artisan", "horizon"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitCommandLine(tt.line)
			if len(result) != len(tt.expected) {
				t.Errorf("splitCommandLine(%q) length = %d, expected %d", tt.line, len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitCommandLine(%q)[%d] = %s, expected %s", tt.line, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// TestFormatRow tests row formatting
func TestFormatRow(t *testing.T) {
	tests := []struct {
		name      string
		values    []string
		widths    []int
		alignLeft []bool
		expected  string
	}{
		{
			name:      "single column left aligned",
			values:    []string{"test"},
			widths:    []int{10},
			alignLeft: []bool{true},
			expected:  "test      ",
		},
		{
			name:      "single column right aligned",
			values:    []string{"test"},
			widths:    []int{10},
			alignLeft: []bool{false},
			expected:  "      test",
		},
		{
			name:      "multiple columns mixed alignment",
			values:    []string{"name", "value"},
			widths:    []int{10, 8},
			alignLeft: []bool{true, false},
			expected:  "name           value",
		},
		{
			name:      "exact width match",
			values:    []string{"exact"},
			widths:    []int{5},
			alignLeft: []bool{true},
			expected:  "exact",
		},
		{
			name:      "value longer than width",
			values:    []string{"toolongvalue"},
			widths:    []int{5},
			alignLeft: []bool{true},
			expected:  "toolongvalue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRow(tt.values, nil, tt.widths, tt.alignLeft)
			if result != tt.expected {
				t.Errorf("formatRow() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestBuildHeaderStyles tests header style building
func TestBuildHeaderStyles(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		usePlain bool
	}{
		{
			name:     "plain styles",
			count:    5,
			usePlain: true,
		},
		{
			name:     "styled headers",
			count:    5,
			usePlain: false,
		},
		{
			name:     "zero columns",
			count:    0,
			usePlain: false,
		},
		{
			name:     "single column",
			count:    1,
			usePlain: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			styles := buildHeaderStyles(tt.count, tt.usePlain)
			if len(styles) != tt.count {
				t.Errorf("buildHeaderStyles(%d, %v) length = %d, expected %d",
					tt.count, tt.usePlain, len(styles), tt.count)
			}
		})
	}
}
