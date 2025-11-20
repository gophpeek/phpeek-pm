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
	ShutdownTimeout      int    `yaml:"shutdown_timeout" json:"shutdown_timeout"`           // seconds
	HealthCheckInterval  int    `yaml:"health_check_interval" json:"health_check_interval"` // seconds
	RestartPolicy        string `yaml:"restart_policy" json:"restart_policy"`               // always | on-failure | never
	MaxRestartAttempts   int    `yaml:"max_restart_attempts" json:"max_restart_attempts"`
	RestartBackoff       int    `yaml:"restart_backoff" json:"restart_backoff"` // seconds
	LogFormat            string `yaml:"log_format" json:"log_format"`           // json | text
	LogLevel             string `yaml:"log_level" json:"log_level"`             // debug | info | warn | error
	LogTimestamps        bool   `yaml:"log_timestamps" json:"log_timestamps"`
	MetricsEnabled       bool   `yaml:"metrics_enabled" json:"metrics_enabled"`
	MetricsPort          int    `yaml:"metrics_port" json:"metrics_port"`
	MetricsPath          string `yaml:"metrics_path" json:"metrics_path"`
	APIEnabled           bool   `yaml:"api_enabled" json:"api_enabled"`
	APIPort              int    `yaml:"api_port" json:"api_port"`
	APIAuth              string `yaml:"api_auth" json:"api_auth"` // Bearer token
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
	Name             string            `yaml:"name" json:"name"`
	Command          []string          `yaml:"command" json:"command"`
	Timeout          int               `yaml:"timeout" json:"timeout"` // seconds
	Retry            int               `yaml:"retry" json:"retry"`
	RetryDelay       int               `yaml:"retry_delay" json:"retry_delay"`       // seconds
	ContinueOnError  bool              `yaml:"continue_on_error" json:"continue_on_error"`
	Env              map[string]string `yaml:"env" json:"env"`
	WorkingDir       string            `yaml:"working_dir" json:"working_dir"`
}

// Process represents a managed process definition
type Process struct {
	Enabled     bool              `yaml:"enabled" json:"enabled"`
	Command     []string          `yaml:"command" json:"command"`
	Priority    int               `yaml:"priority" json:"priority"`       // Lower starts first
	Restart     string            `yaml:"restart" json:"restart"`         // always | on-failure | never
	Scale       int               `yaml:"scale" json:"scale"`             // Number of instances
	DependsOn   []string          `yaml:"depends_on" json:"depends_on"`   // Process dependencies
	Env         map[string]string `yaml:"env" json:"env"`
	HealthCheck *HealthCheck      `yaml:"health_check" json:"health_check"`
	Shutdown    *ShutdownConfig   `yaml:"shutdown" json:"shutdown"`
	Logging     *LoggingConfig    `yaml:"logging" json:"logging"`
}

// HealthCheck configuration
type HealthCheck struct {
	Type              string   `yaml:"type" json:"type"`                             // tcp | http | exec
	Address           string   `yaml:"address" json:"address"`                       // For TCP
	URL               string   `yaml:"url" json:"url"`                               // For HTTP
	Command           []string `yaml:"command" json:"command"`                       // For exec
	InitialDelay      int      `yaml:"initial_delay" json:"initial_delay"`           // seconds
	Period            int      `yaml:"period" json:"period"`                         // seconds
	Timeout           int      `yaml:"timeout" json:"timeout"`                       // seconds
	FailureThreshold  int      `yaml:"failure_threshold" json:"failure_threshold"`
	SuccessThreshold  int      `yaml:"success_threshold" json:"success_threshold"`
	ExpectedStatus    int      `yaml:"expected_status" json:"expected_status"`       // For HTTP
}

// ShutdownConfig configures graceful shutdown behavior
type ShutdownConfig struct {
	Signal       string `yaml:"signal" json:"signal"`               // SIGTERM, SIGQUIT, etc.
	Timeout      int    `yaml:"timeout" json:"timeout"`             // seconds
	KillSignal   string `yaml:"kill_signal" json:"kill_signal"`     // SIGKILL
	Graceful     bool   `yaml:"graceful" json:"graceful"`           // Wait for connections
	PreStopHook  *Hook  `yaml:"pre_stop_hook" json:"pre_stop_hook"` // Per-process pre-stop hook
}

// LoggingConfig configures per-process logging
type LoggingConfig struct {
	Stdout bool              `yaml:"stdout" json:"stdout"`
	Stderr bool              `yaml:"stderr" json:"stderr"`
	Labels map[string]string `yaml:"labels" json:"labels"` // Additional log labels
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
	}
}
