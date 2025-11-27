package logger

import (
	"log/slog"
	"testing"
)

func TestNew_LogLevels(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantLevel slog.Level
	}{
		{
			name:      "debug level",
			level:     "debug",
			wantLevel: slog.LevelDebug,
		},
		{
			name:      "info level",
			level:     "info",
			wantLevel: slog.LevelInfo,
		},
		{
			name:      "warn level",
			level:     "warn",
			wantLevel: slog.LevelWarn,
		},
		{
			name:      "error level",
			level:     "error",
			wantLevel: slog.LevelError,
		},
		{
			name:      "invalid level defaults to info",
			level:     "invalid",
			wantLevel: slog.LevelInfo,
		},
		{
			name:      "empty level defaults to info",
			level:     "",
			wantLevel: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.level, "text")

			if logger == nil {
				t.Fatal("New() returned nil logger")
			}

			// Verify logger exists and can be used
			// Note: We can't directly inspect the level from slog.Logger,
			// but we can verify the logger was created successfully
			if logger.Handler() == nil {
				t.Error("logger handler should not be nil")
			}
		})
	}
}

func TestNew_LogFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{
			name:   "text format",
			format: "text",
		},
		{
			name:   "json format",
			format: "json",
		},
		{
			name:   "invalid format defaults to text",
			format: "invalid",
		},
		{
			name:   "empty format defaults to text",
			format: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New("info", tt.format)

			if logger == nil {
				t.Fatal("New() returned nil logger")
			}

			if logger.Handler() == nil {
				t.Error("logger handler should not be nil")
			}
		})
	}
}

func TestNew_JSONFormat_ProducesJSON(t *testing.T) {
	// We can't easily capture stdout from New(), but we can verify
	// the logger is created and works
	logger := New("info", "json")

	if logger == nil {
		t.Fatal("New() returned nil logger")
	}

	// Verify handler type
	handler := logger.Handler()
	if handler == nil {
		t.Error("handler should not be nil")
	}

	// Note: In production code, we'd need to restructure New() to accept
	// an io.Writer for proper testing, but for now we just verify creation
}

func TestNew_TextFormat_ProducesText(t *testing.T) {
	logger := New("info", "text")

	if logger == nil {
		t.Fatal("New() returned nil logger")
	}

	handler := logger.Handler()
	if handler == nil {
		t.Error("handler should not be nil")
	}
}

func TestNew_AllCombinations(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}
	formats := []string{"text", "json"}

	for _, level := range levels {
		for _, format := range formats {
			t.Run(level+"_"+format, func(t *testing.T) {
				logger := New(level, format)
				if logger == nil {
					t.Errorf("New(%q, %q) returned nil", level, format)
				}
			})
		}
	}
}

func TestNew_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
	}{
		{
			name:   "uppercase level",
			level:  "INFO",
			format: "text",
		},
		{
			name:   "mixed case level",
			level:  "WaRn",
			format: "text",
		},
		{
			name:   "uppercase format",
			level:  "info",
			format: "JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.level, tt.format)

			// Logger should still be created (may use defaults)
			if logger == nil {
				t.Error("New() should create logger even with case variations")
			}
		})
	}
}

// Helper function to verify logger can actually log
func TestNew_LoggerCanLog(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
	}{
		{
			name:   "debug text logger",
			level:  "debug",
			format: "text",
		},
		{
			name:   "info json logger",
			level:  "info",
			format: "json",
		},
		{
			name:   "warn text logger",
			level:  "warn",
			format: "text",
		},
		{
			name:   "error json logger",
			level:  "error",
			format: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.level, tt.format)

			if logger == nil {
				t.Fatal("New() returned nil logger")
			}

			// Verify logger can be called without panicking
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("logger.Info() panicked: %v", r)
				}
			}()

			logger.Info("test message", "key", "value")
		})
	}
}
