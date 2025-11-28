package config

import "time"

// Config represents the complete phpeek-pm configuration
type Config struct {
	Version   string              `yaml:"version" json:"version"`
	Global    GlobalConfig        `yaml:"global" json:"global"`
	Hooks     HooksConfig         `yaml:"hooks" json:"hooks"`
	Processes map[string]*Process `yaml:"processes" json:"processes"`
}

// GlobalConfig contains global settings for the process manager
type GlobalConfig struct {
	ShutdownTimeout           int           `yaml:"shutdown_timeout" json:"shutdown_timeout"`                         // seconds
	HealthCheckInterval       int           `yaml:"health_check_interval" json:"health_check_interval"`               // seconds
	RestartPolicy             string        `yaml:"restart_policy" json:"restart_policy"`                             // always | on-failure | never
	MaxRestartAttempts        int           `yaml:"max_restart_attempts" json:"max_restart_attempts"`                 //
	RestartBackoff            int           `yaml:"restart_backoff" json:"restart_backoff"`                           // seconds (legacy, prefer restart_backoff_initial/max)
	RestartBackoffInitial     time.Duration `yaml:"restart_backoff_initial" json:"restart_backoff_initial"`           // initial duration (supports "5s" style)
	RestartBackoffMax         time.Duration `yaml:"restart_backoff_max" json:"restart_backoff_max"`                   // max duration
	AutotuneMemoryThreshold   float64       `yaml:"autotune_memory_threshold" json:"autotune_memory_threshold"`       // 0.0-2.0, overrides profile MaxMemoryUsage
	LogFormat                 string        `yaml:"log_format" json:"log_format"`                                     // json | text
	LogLevel                  string        `yaml:"log_level" json:"log_level"`                                       // debug | info | warn | error
	LogTimestamps             bool          `yaml:"log_timestamps" json:"log_timestamps"`                             //
	MetricsEnabled            *bool         `yaml:"metrics_enabled" json:"metrics_enabled"`                           //
	MetricsPort               int           `yaml:"metrics_port" json:"metrics_port"`                                 //
	MetricsPath               string        `yaml:"metrics_path" json:"metrics_path"`                                 //
	APIEnabled                *bool         `yaml:"api_enabled" json:"api_enabled"`                                   //
	APIPort                   int           `yaml:"api_port" json:"api_port"`                                         //
	APISocket                 string        `yaml:"api_socket" json:"api_socket"`                                     // Unix socket path (e.g. /var/run/phpeek-pm.sock)
	APIAuth                   string        `yaml:"api_auth" json:"api_auth"`                                         // Bearer token
	APITLS                    *TLSConfig    `yaml:"api_tls" json:"api_tls"`                                           // TLS configuration for API
	APIACL                    *ACLConfig    `yaml:"api_acl" json:"api_acl"`                                           // IP ACL for API
	MetricsTLS                *TLSConfig    `yaml:"metrics_tls" json:"metrics_tls"`                                   // TLS configuration for metrics
	MetricsACL                *ACLConfig    `yaml:"metrics_acl" json:"metrics_acl"`                                   // IP ACL for metrics
	ResourceMetricsEnabled    *bool         `yaml:"resource_metrics_enabled" json:"resource_metrics_enabled"`         // Enable CPU/RAM collection
	ResourceMetricsInterval   int           `yaml:"resource_metrics_interval" json:"resource_metrics_interval"`       // seconds (default: 5)
	ResourceMetricsMaxSamples int           `yaml:"resource_metrics_max_samples" json:"resource_metrics_max_samples"` // Per-instance buffer size (default: 720 = 1h at 5s)
	AuditEnabled              bool          `yaml:"audit_enabled" json:"audit_enabled"`                               // Enable audit logging
	TracingEnabled            bool          `yaml:"tracing_enabled" json:"tracing_enabled"`                           // Enable distributed tracing
	TracingExporter           string        `yaml:"tracing_exporter" json:"tracing_exporter"`                         // otlp-grpc | otlp-http | stdout | jaeger | zipkin
	TracingEndpoint           string        `yaml:"tracing_endpoint" json:"tracing_endpoint"`                         // Exporter endpoint (e.g., localhost:4317)
	TracingSampleRate         float64       `yaml:"tracing_sample_rate" json:"tracing_sample_rate"`                   // 0.0-1.0 (default: 1.0 = 100%)
	TracingServiceName        string        `yaml:"tracing_service_name" json:"tracing_service_name"`                 // Service name for traces (default: phpeek-pm)
	TracingUseTLS             bool          `yaml:"tracing_use_tls" json:"tracing_use_tls"`                           // Enable TLS for production (default: false)
	ScheduleHistorySize       int           `yaml:"schedule_history_size" json:"schedule_history_size"`               // Max execution history entries per job (default: 100)
	OneshotHistoryMaxEntries  int           `yaml:"oneshot_history_max_entries" json:"oneshot_history_max_entries"`   // Max oneshot history entries per process (default: 5000)
	OneshotHistoryMaxAge      time.Duration `yaml:"oneshot_history_max_age" json:"oneshot_history_max_age"`           // Max age of oneshot history entries (default: 24h)
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
	Type         string            `yaml:"type" json:"type"`                   // oneshot | longrun (default: longrun)
	InitialState string            `yaml:"initial_state" json:"initial_state"` // running | stopped (default: running)
	Command      []string          `yaml:"command" json:"command"`
	WorkingDir   string            `yaml:"working_dir" json:"working_dir"`   // Working directory override
	Stdout       *bool             `yaml:"stdout" json:"stdout"`             // Legacy shorthand for logging.stdout
	Stderr       *bool             `yaml:"stderr" json:"stderr"`             // Legacy shorthand for logging.stderr
	Restart      string            `yaml:"restart" json:"restart"`           // always | on-failure | never
	Scale        int               `yaml:"scale" json:"scale"`               // Number of instances
	MaxScale     int               `yaml:"max_scale" json:"max_scale"`       // Maximum instances (0 = no limit)
	DependsOn    []string          `yaml:"depends_on" json:"depends_on"`     // Process dependencies
	Env          map[string]string `yaml:"env" json:"env"`
	HealthCheck  *HealthCheck      `yaml:"health_check" json:"health_check"`
	Shutdown     *ShutdownConfig   `yaml:"shutdown" json:"shutdown"`
	Logging          *LoggingConfig `yaml:"logging" json:"logging"`
	Schedule              string           `yaml:"schedule" json:"schedule"`                               // Cron expression: "*/5 * * * *"
	ScheduleTimezone      string           `yaml:"schedule_timezone" json:"schedule_timezone"`             // Timezone: "UTC" (default) | "Local"
	ScheduleTimeout       string           `yaml:"schedule_timeout" json:"schedule_timeout"`               // Execution timeout: "30s", "5m", "1h" (default: no timeout)
	ScheduleMaxConcurrent int              `yaml:"schedule_max_concurrent" json:"schedule_max_concurrent"` // Max concurrent: 1=no overlap, 0=unlimited (default: 0)
	Heartbeat             *HeartbeatConfig `yaml:"heartbeat" json:"heartbeat"`                             // Heartbeat monitoring config
}

// HeartbeatConfig configures heartbeat monitoring for scheduled jobs
type HeartbeatConfig struct {
	Enabled  bool `yaml:"enabled" json:"enabled"`   // Enable heartbeat monitoring
	Interval int  `yaml:"interval" json:"interval"` // Expected interval in seconds
	Grace    int  `yaml:"grace" json:"grace"`       // Grace period before alert in seconds
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

// TLSConfig configures TLS/HTTPS for API and metrics endpoints
type TLSConfig struct {
	Enabled            bool     `yaml:"enabled" json:"enabled"`                           // Enable TLS
	CertFile           string   `yaml:"cert_file" json:"cert_file"`                       // Path to certificate file
	KeyFile            string   `yaml:"key_file" json:"key_file"`                         // Path to private key file
	CAFile             string   `yaml:"ca_file" json:"ca_file"`                           // Path to CA certificate (for mTLS)
	ClientAuth         string   `yaml:"client_auth" json:"client_auth"`                   // none | request | require | verify (default: none)
	MinVersion         string   `yaml:"min_version" json:"min_version"`                   // TLS 1.2 | TLS 1.3 (default: TLS 1.2)
	CipherSuites       []string `yaml:"cipher_suites" json:"cipher_suites"`               // Allowed cipher suites (empty = defaults)
	AutoReload         bool     `yaml:"auto_reload" json:"auto_reload"`                   // Auto-reload certs on file change (default: false)
	AutoReloadInterval int      `yaml:"auto_reload_interval" json:"auto_reload_interval"` // Check interval in seconds (default: 300)
}

// ACLConfig configures IP-based Access Control Lists
type ACLConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`         // Enable IP ACL
	Mode       string   `yaml:"mode" json:"mode"`               // allow | deny (default: allow)
	AllowList  []string `yaml:"allow_list" json:"allow_list"`   // Allowed IP addresses/CIDRs
	DenyList   []string `yaml:"deny_list" json:"deny_list"`     // Denied IP addresses/CIDRs
	TrustProxy bool     `yaml:"trust_proxy" json:"trust_proxy"` // Trust X-Forwarded-For header (default: false)
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
	legacyBackoff := time.Duration(c.Global.RestartBackoff) * time.Second
	if c.Global.RestartBackoffInitial == 0 {
		if legacyBackoff > 0 {
			c.Global.RestartBackoffInitial = legacyBackoff
		} else {
			c.Global.RestartBackoffInitial = 5 * time.Second
		}
	}
	if c.Global.RestartBackoffMax == 0 {
		if legacyBackoff > 0 {
			c.Global.RestartBackoffMax = legacyBackoff * 12
		}
		if c.Global.RestartBackoffMax == 0 {
			c.Global.RestartBackoffMax = 1 * time.Minute
		}
	}
	if c.Global.LogFormat == "" {
		c.Global.LogFormat = "json"
	}
	if c.Global.LogLevel == "" {
		c.Global.LogLevel = "info"
	}
	c.Global.LogTimestamps = true
	// Metrics enabled by default for observability
	if c.Global.MetricsEnabled == nil {
		c.Global.SetMetricsEnabled(true)
	}
	if c.Global.MetricsPort == 0 {
		c.Global.MetricsPort = 9090
	}
	if c.Global.MetricsPath == "" {
		c.Global.MetricsPath = "/metrics"
	}
	// API is enabled by default for TUI/remote control
	// Can be disabled by explicitly setting api_enabled: false
	if c.Global.APIEnabled == nil {
		c.Global.SetAPIEnabled(true)
	}
	if c.Global.APIPort == 0 {
		c.Global.APIPort = 9180
	}
	// TLS defaults for API
	if c.Global.APITLS != nil {
		if c.Global.APITLS.MinVersion == "" {
			c.Global.APITLS.MinVersion = "TLS 1.2"
		}
		if c.Global.APITLS.ClientAuth == "" {
			c.Global.APITLS.ClientAuth = "none"
		}
		if c.Global.APITLS.AutoReloadInterval == 0 {
			c.Global.APITLS.AutoReloadInterval = 300 // 5 minutes
		}
	}
	// TLS defaults for Metrics
	if c.Global.MetricsTLS != nil {
		if c.Global.MetricsTLS.MinVersion == "" {
			c.Global.MetricsTLS.MinVersion = "TLS 1.2"
		}
		if c.Global.MetricsTLS.ClientAuth == "" {
			c.Global.MetricsTLS.ClientAuth = "none"
		}
		if c.Global.MetricsTLS.AutoReloadInterval == 0 {
			c.Global.MetricsTLS.AutoReloadInterval = 300 // 5 minutes
		}
	}
	// ACL defaults for API
	if c.Global.APIACL != nil {
		if c.Global.APIACL.Mode == "" {
			c.Global.APIACL.Mode = "allow" // Whitelist approach (deny all except allowed)
		}
	}
	// ACL defaults for Metrics
	if c.Global.MetricsACL != nil {
		if c.Global.MetricsACL.Mode == "" {
			c.Global.MetricsACL.Mode = "allow" // Whitelist approach
		}
	}
	// Enable resource metrics collection by default
	if c.Global.ResourceMetricsEnabled == nil {
		c.Global.SetResourceMetricsEnabled(true)
	}
	if c.Global.ResourceMetricsInterval == 0 {
		c.Global.ResourceMetricsInterval = 5 // 5 seconds
	}
	if c.Global.ResourceMetricsMaxSamples == 0 {
		c.Global.ResourceMetricsMaxSamples = 720 // 1 hour at 5s = 720 samples
	}
	// Distributed tracing disabled by default (opt-in for observability)
	if c.Global.TracingExporter == "" {
		c.Global.TracingExporter = "stdout" // Default: stdout for development
	}
	if c.Global.TracingSampleRate == 0 {
		c.Global.TracingSampleRate = 1.0 // Default: 100% sampling
	}
	if c.Global.TracingServiceName == "" {
		c.Global.TracingServiceName = "phpeek-pm" // Default service name
	}
	if c.Global.TracingEndpoint == "" {
		// Set default endpoint based on exporter type
		switch c.Global.TracingExporter {
		case "otlp-grpc":
			c.Global.TracingEndpoint = "localhost:4317"
		case "otlp-http":
			c.Global.TracingEndpoint = "localhost:4318"
		case "jaeger":
			c.Global.TracingEndpoint = "localhost:14268"
		case "zipkin":
			c.Global.TracingEndpoint = "http://localhost:9411/api/v2/spans"
		}
	}
	// Schedule history defaults
	if c.Global.ScheduleHistorySize == 0 {
		c.Global.ScheduleHistorySize = 100 // Default: 100 entries per job
	}
	// Oneshot history defaults
	if c.Global.OneshotHistoryMaxEntries == 0 {
		c.Global.OneshotHistoryMaxEntries = 5000 // Default: 5000 entries per process
	}
	if c.Global.OneshotHistoryMaxAge == 0 {
		c.Global.OneshotHistoryMaxAge = 24 * time.Hour // Default: 24 hours
	}

	// Process defaults
	for name, proc := range c.Processes {
		// Default: enabled=true if not explicitly set
		// Note: YAML unmarshals missing bool as false, so we can't distinguish
		// between explicit false and missing. For now, processes default to enabled.

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

		// Schedule defaults
		if proc.Schedule != "" && proc.ScheduleTimezone == "" {
			proc.ScheduleTimezone = "UTC" // Default: UTC timezone
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
		stdoutEnabled := true
		if proc.Stdout != nil {
			stdoutEnabled = *proc.Stdout
		}
		stderrEnabled := true
		if proc.Stderr != nil {
			stderrEnabled = *proc.Stderr
		}

		if proc.Logging == nil {
			proc.Logging = &LoggingConfig{
				Stdout: stdoutEnabled,
				Stderr: stderrEnabled,
				Labels: map[string]string{
					"process": name,
				},
			}
		} else {
			if proc.Stdout != nil {
				proc.Logging.Stdout = *proc.Stdout
			}
			if proc.Stderr != nil {
				proc.Logging.Stderr = *proc.Stderr
			}
			if proc.Stdout == nil && !proc.Logging.Stdout {
				proc.Logging.Stdout = true
			}
			if proc.Stderr == nil && !proc.Logging.Stderr {
				proc.Logging.Stderr = true
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

// MetricsEnabledValue returns true if metrics are enabled (default true)
func (g *GlobalConfig) MetricsEnabledValue() bool {
	if g == nil || g.MetricsEnabled == nil {
		return true
	}
	return *g.MetricsEnabled
}

// SetMetricsEnabled sets the metrics_enabled flag
func (g *GlobalConfig) SetMetricsEnabled(v bool) {
	g.MetricsEnabled = boolPtr(v)
}

// ResourceMetricsEnabledValue returns true if resource metrics enabled (default true)
func (g *GlobalConfig) ResourceMetricsEnabledValue() bool {
	if g == nil || g.ResourceMetricsEnabled == nil {
		return true
	}
	return *g.ResourceMetricsEnabled
}

// SetResourceMetricsEnabled sets resource metrics flag
func (g *GlobalConfig) SetResourceMetricsEnabled(v bool) {
	g.ResourceMetricsEnabled = boolPtr(v)
}

// APIEnabledValue returns true if API enabled (default true)
func (g *GlobalConfig) APIEnabledValue() bool {
	if g == nil || g.APIEnabled == nil {
		return true
	}
	return *g.APIEnabled
}

// SetAPIEnabled sets the api_enabled flag
func (g *GlobalConfig) SetAPIEnabled(v bool) {
	g.APIEnabled = boolPtr(v)
}

func boolPtr(v bool) *bool {
	return &v
}

// Equal compares two Process configurations for equality
func (p *Process) Equal(other *Process) bool {
	if p == nil || other == nil {
		return p == other
	}

	// Compare basic fields
	if p.Enabled != other.Enabled ||
		p.Type != other.Type ||
		p.InitialState != other.InitialState ||
		p.Scale != other.Scale ||
		p.MaxScale != other.MaxScale ||
		p.Restart != other.Restart ||
		p.WorkingDir != other.WorkingDir ||
		p.Schedule != other.Schedule ||
		p.ScheduleTimezone != other.ScheduleTimezone {
		return false
	}

	// Compare command slices
	if !stringSliceEqual(p.Command, other.Command) {
		return false
	}

	// Compare depends_on slices
	if !stringSliceEqual(p.DependsOn, other.DependsOn) {
		return false
	}

	// Compare environment variables
	if !stringMapEqual(p.Env, other.Env) {
		return false
	}

	// Compare health check config
	if !healthCheckEqual(p.HealthCheck, other.HealthCheck) {
		return false
	}

	// Compare shutdown config
	if !shutdownConfigEqual(p.Shutdown, other.Shutdown) {
		return false
	}

	return true
}

// stringSliceEqual compares two string slices for equality
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stringMapEqual compares two string maps for equality
func stringMapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// healthCheckEqual compares two HealthCheck configs
func healthCheckEqual(a, b *HealthCheck) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Type == b.Type &&
		a.Address == b.Address &&
		a.URL == b.URL &&
		a.InitialDelay == b.InitialDelay &&
		a.Period == b.Period &&
		a.Timeout == b.Timeout &&
		a.FailureThreshold == b.FailureThreshold &&
		a.SuccessThreshold == b.SuccessThreshold &&
		a.ExpectedStatus == b.ExpectedStatus &&
		a.Mode == b.Mode &&
		stringSliceEqual(a.Command, b.Command)
}

// shutdownConfigEqual compares two ShutdownConfig configs
func shutdownConfigEqual(a, b *ShutdownConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Signal != b.Signal ||
		a.Timeout != b.Timeout ||
		a.KillSignal != b.KillSignal ||
		a.Graceful != b.Graceful {
		return false
	}
	// Compare pre-stop hook
	return hookEqual(a.PreStopHook, b.PreStopHook)
}

// hookEqual compares two Hook configs
func hookEqual(a, b *Hook) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Name == b.Name &&
		a.Timeout == b.Timeout &&
		a.Retry == b.Retry &&
		a.RetryDelay == b.RetryDelay &&
		a.ContinueOnError == b.ContinueOnError &&
		a.WorkingDir == b.WorkingDir &&
		stringSliceEqual(a.Command, b.Command) &&
		stringMapEqual(a.Env, b.Env)
}
