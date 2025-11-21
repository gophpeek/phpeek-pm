package config

// Config represents the complete phpeek-pm configuration
type Config struct {
	Version   string              `yaml:"version" json:"version"`
	Global    GlobalConfig        `yaml:"global" json:"global"`
	Hooks     HooksConfig         `yaml:"hooks" json:"hooks"`
	Processes map[string]*Process `yaml:"processes" json:"processes"`
}

// GlobalConfig contains global settings for the process manager
type GlobalConfig struct {
	ShutdownTimeout          int     `yaml:"shutdown_timeout" json:"shutdown_timeout"`                       // seconds
	HealthCheckInterval      int     `yaml:"health_check_interval" json:"health_check_interval"`             // seconds
	RestartPolicy            string  `yaml:"restart_policy" json:"restart_policy"`                           // always | on-failure | never
	MaxRestartAttempts       int     `yaml:"max_restart_attempts" json:"max_restart_attempts"`               //
	RestartBackoff           int     `yaml:"restart_backoff" json:"restart_backoff"`                         // seconds
	AutotuneMemoryThreshold  float64 `yaml:"autotune_memory_threshold" json:"autotune_memory_threshold"`     // 0.0-2.0, overrides profile MaxMemoryUsage
	LogFormat                string  `yaml:"log_format" json:"log_format"`                                   // json | text
	LogLevel                 string  `yaml:"log_level" json:"log_level"`                                     // debug | info | warn | error
	LogTimestamps            bool    `yaml:"log_timestamps" json:"log_timestamps"`                           //
	MetricsEnabled           bool    `yaml:"metrics_enabled" json:"metrics_enabled"`                         //
	MetricsPort              int     `yaml:"metrics_port" json:"metrics_port"`                               //
	MetricsPath              string  `yaml:"metrics_path" json:"metrics_path"`                               //
	APIEnabled               bool    `yaml:"api_enabled" json:"api_enabled"`                                 //
	APIPort                  int     `yaml:"api_port" json:"api_port"`                                       //
	APIAuth                  string  `yaml:"api_auth" json:"api_auth"`                                       // Bearer token
}

// HooksConfig contains lifecycle hooks
type HooksConfig struct {
	PreStart  []Hook `yaml:"pre-start" json:"pre_start"`
	PostStart []Hook `yaml:"post-start" json:"post_start"`
	PreStop   []Hook `yaml:"pre-stop" json:"pre_stop"`
	PostStop  []Hook `yaml:"post-stop" json:"post_stop"`
}

// Hook represents a lifecycle hook command
type Hook struct {
	Name            string            `yaml:"name" json:"name"`
	Command         []string          `yaml:"command" json:"command"`
	Timeout         int               `yaml:"timeout" json:"timeout"` // seconds
	Retry           int               `yaml:"retry" json:"retry"`
	RetryDelay      int               `yaml:"retry_delay" json:"retry_delay"` // seconds
	ContinueOnError bool              `yaml:"continue_on_error" json:"continue_on_error"`
	Env             map[string]string `yaml:"env" json:"env"`
	WorkingDir      string            `yaml:"working_dir" json:"working_dir"`
}

// Process represents a managed process definition
type Process struct {
	Enabled      bool              `yaml:"enabled" json:"enabled"`
	Type         string            `yaml:"type" json:"type"`               // oneshot | longrun (default: longrun)
	InitialState string            `yaml:"initial_state" json:"initial_state"` // running | stopped (default: running)
	Command      []string          `yaml:"command" json:"command"`
	Priority     int               `yaml:"priority" json:"priority"`     // Lower starts first
	Restart      string            `yaml:"restart" json:"restart"`       // always | on-failure | never
	Scale        int               `yaml:"scale" json:"scale"`           // Number of instances
	ScaleLocked  bool              `yaml:"scale_locked" json:"scale_locked"` // Prevent scaling (port conflicts)
	DependsOn    []string          `yaml:"depends_on" json:"depends_on"`     // Process dependencies
	Env          map[string]string `yaml:"env" json:"env"`
	HealthCheck  *HealthCheck      `yaml:"health_check" json:"health_check"`
	Shutdown     *ShutdownConfig   `yaml:"shutdown" json:"shutdown"`
	Logging      *LoggingConfig    `yaml:"logging" json:"logging"`
	Schedule     string            `yaml:"schedule" json:"schedule"`   // Cron expression for scheduled tasks
	Heartbeat    *HeartbeatConfig  `yaml:"heartbeat" json:"heartbeat"` // Heartbeat/dead man's switch
}

// HealthCheck configuration
type HealthCheck struct {
	Type             string   `yaml:"type" json:"type"`                   // tcp | http | exec
	Address          string   `yaml:"address" json:"address"`             // For TCP
	URL              string   `yaml:"url" json:"url"`                     // For HTTP
	Command          []string `yaml:"command" json:"command"`             // For exec
	InitialDelay     int      `yaml:"initial_delay" json:"initial_delay"` // seconds
	Period           int      `yaml:"period" json:"period"`               // seconds
	Timeout          int      `yaml:"timeout" json:"timeout"`             // seconds
	FailureThreshold int      `yaml:"failure_threshold" json:"failure_threshold"`
	SuccessThreshold int      `yaml:"success_threshold" json:"success_threshold"`
	ExpectedStatus   int      `yaml:"expected_status" json:"expected_status"` // For HTTP
	Mode             string   `yaml:"mode" json:"mode"`                       // liveness | readiness | both (default: both)
}

// ShutdownConfig configures graceful shutdown behavior
type ShutdownConfig struct {
	Signal      string `yaml:"signal" json:"signal"`               // SIGTERM, SIGQUIT, etc.
	Timeout     int    `yaml:"timeout" json:"timeout"`             // seconds
	KillSignal  string `yaml:"kill_signal" json:"kill_signal"`     // SIGKILL
	Graceful    bool   `yaml:"graceful" json:"graceful"`           // Wait for connections
	PreStopHook *Hook  `yaml:"pre_stop_hook" json:"pre_stop_hook"` // Per-process pre-stop hook
}

// LoggingConfig configures per-process logging
type LoggingConfig struct {
	Stdout         bool                  `yaml:"stdout" json:"stdout"`
	Stderr         bool                  `yaml:"stderr" json:"stderr"`
	Labels         map[string]string     `yaml:"labels" json:"labels"`                   // Additional log labels
	MinLevel       string                `yaml:"min_level" json:"min_level"`             // Minimum log level to output (debug|info|warn|error)
	Redaction      *RedactionConfig      `yaml:"redaction" json:"redaction"`             // Sensitive data redaction
	Multiline      *MultilineConfig      `yaml:"multiline" json:"multiline"`             // Multiline log handling
	JSON           *JSONConfig           `yaml:"json" json:"json"`                       // JSON log parsing
	LevelDetection *LevelDetectionConfig `yaml:"level_detection" json:"level_detection"` // Log level detection from content
	Filters        *FilterConfig         `yaml:"filters" json:"filters"`                 // Include/exclude filtering
}

// RedactionConfig configures sensitive data redaction for compliance
type RedactionConfig struct {
	Enabled  bool               `yaml:"enabled" json:"enabled"`
	Patterns []RedactionPattern `yaml:"patterns" json:"patterns"`
}

// RedactionPattern defines a regex pattern for redacting sensitive data
type RedactionPattern struct {
	Name        string `yaml:"name" json:"name"`               // Pattern name for reference
	Pattern     string `yaml:"pattern" json:"pattern"`         // Regex pattern to match
	Replacement string `yaml:"replacement" json:"replacement"` // Replacement text (e.g., "***")
}

// MultilineConfig configures multiline log handling (e.g., stack traces)
type MultilineConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Pattern  string `yaml:"pattern" json:"pattern"`     // Regex pattern matching start of new log entry
	MaxLines int    `yaml:"max_lines" json:"max_lines"` // Max lines to buffer (default: 100)
	Timeout  int    `yaml:"timeout" json:"timeout"`     // Flush timeout in seconds (default: 1)
}

// JSONConfig configures JSON log parsing
type JSONConfig struct {
	Enabled        bool `yaml:"enabled" json:"enabled"`
	DetectAuto     bool `yaml:"detect_auto" json:"detect_auto"`         // Auto-detect JSON logs
	ExtractLevel   bool `yaml:"extract_level" json:"extract_level"`     // Extract 'level' field
	ExtractMessage bool `yaml:"extract_message" json:"extract_message"` // Extract 'message' field
	MergeFields    bool `yaml:"merge_fields" json:"merge_fields"`       // Merge other fields as attributes
}

// LevelDetectionConfig configures log level detection from log content
type LevelDetectionConfig struct {
	Enabled      bool              `yaml:"enabled" json:"enabled"`
	Patterns     map[string]string `yaml:"patterns" json:"patterns"`           // Map of level -> regex pattern
	DefaultLevel string            `yaml:"default_level" json:"default_level"` // Default level if no match (default: info)
}

// FilterConfig configures log filtering
type FilterConfig struct {
	Exclude []string `yaml:"exclude" json:"exclude"` // Exclude logs matching these patterns
	Include []string `yaml:"include" json:"include"` // Only include logs matching these patterns (if specified)
}

// HeartbeatConfig configures heartbeat/dead man's switch notifications
type HeartbeatConfig struct {
	SuccessURL string            `yaml:"success_url" json:"success_url"` // URL to ping on success
	FailureURL string            `yaml:"failure_url" json:"failure_url"` // URL to ping on failure
	Timeout    int               `yaml:"timeout" json:"timeout"`         // HTTP timeout in seconds (default: 30)
	RetryCount int               `yaml:"retry_count" json:"retry_count"` // Number of retries on failure (default: 3)
	RetryDelay int               `yaml:"retry_delay" json:"retry_delay"` // Delay between retries in seconds (default: 5)
	Method     string            `yaml:"method" json:"method"`           // HTTP method (default: POST)
	Headers    map[string]string `yaml:"headers" json:"headers"`         // Custom HTTP headers (e.g., Authorization)
}

// SetDefaults sets sensible default values for the configuration
func (c *Config) SetDefaults() {
	if c.Version == "" {
		c.Version = "1.0"
	}

	// Global defaults
	if c.Global.ShutdownTimeout == 0 {
		c.Global.ShutdownTimeout = 30
	}
	if c.Global.HealthCheckInterval == 0 {
		c.Global.HealthCheckInterval = 10
	}
	if c.Global.RestartPolicy == "" {
		c.Global.RestartPolicy = "always"
	}
	if c.Global.MaxRestartAttempts == 0 {
		c.Global.MaxRestartAttempts = 3
	}
	if c.Global.RestartBackoff == 0 {
		c.Global.RestartBackoff = 5
	}
	if c.Global.LogFormat == "" {
		c.Global.LogFormat = "json"
	}
	if c.Global.LogLevel == "" {
		c.Global.LogLevel = "info"
	}
	c.Global.LogTimestamps = true
	c.Global.MetricsEnabled = true
	if c.Global.MetricsPort == 0 {
		c.Global.MetricsPort = 9090
	}
	if c.Global.MetricsPath == "" {
		c.Global.MetricsPath = "/metrics"
	}
	if c.Global.APIPort == 0 {
		c.Global.APIPort = 8080
	}

	// Process defaults
	for name, proc := range c.Processes {
		if proc.Type == "" {
			proc.Type = "longrun" // Default: long-running service
		}
		if proc.InitialState == "" {
			proc.InitialState = "running" // Default: start immediately
		}
		if proc.Restart == "" {
			proc.Restart = c.Global.RestartPolicy
		}
		if proc.Scale == 0 {
			proc.Scale = 1
		}

		// Health check defaults
		if proc.HealthCheck != nil {
			hc := proc.HealthCheck
			if hc.InitialDelay == 0 {
				hc.InitialDelay = 5
			}
			if hc.Period == 0 {
				hc.Period = 10
			}
			if hc.Timeout == 0 {
				hc.Timeout = 3
			}
			if hc.FailureThreshold == 0 {
				hc.FailureThreshold = 3
			}
			if hc.SuccessThreshold == 0 {
				hc.SuccessThreshold = 1
			}
			if hc.ExpectedStatus == 0 {
				hc.ExpectedStatus = 200
			}
			if hc.Mode == "" {
				hc.Mode = "both" // Default: use for both liveness and readiness
			}
		}

		// Shutdown defaults
		if proc.Shutdown != nil {
			sd := proc.Shutdown
			if sd.Signal == "" {
				sd.Signal = "SIGTERM"
			}
			if sd.Timeout == 0 {
				sd.Timeout = 30
			}
			if sd.KillSignal == "" {
				sd.KillSignal = "SIGKILL"
			}
		}

		// Logging defaults
		if proc.Logging == nil {
			proc.Logging = &LoggingConfig{
				Stdout: true,
				Stderr: true,
				Labels: map[string]string{
					"process": name,
				},
			}
		}

		// Set defaults for advanced logging features (all disabled by default, opt-in only)
		if proc.Logging != nil {
			// Multiline defaults (only set numeric values, keep enabled: false)
			if proc.Logging.Multiline != nil {
				if proc.Logging.Multiline.MaxLines == 0 {
					proc.Logging.Multiline.MaxLines = 100
				}
				if proc.Logging.Multiline.Timeout == 0 {
					proc.Logging.Multiline.Timeout = 1 // 1 second
				}
			}

			// Level detection defaults
			if proc.Logging.LevelDetection != nil && proc.Logging.LevelDetection.DefaultLevel == "" {
				proc.Logging.LevelDetection.DefaultLevel = "info"
			}

			// Min level default
			if proc.Logging.MinLevel == "" {
				proc.Logging.MinLevel = "info"
			}
		}
	}
}
