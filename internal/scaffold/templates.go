package scaffold

import (
	"fmt"
	"strings"
	"text/template"
)

// Preset represents a configuration preset type
type Preset string

const (
	PresetLaravel   Preset = "laravel"
	PresetSymfony   Preset = "symfony"
	PresetGeneric   Preset = "generic"
	PresetMinimal   Preset = "minimal"
	PresetProduction Preset = "production"
)

// Config holds template configuration data
type Config struct {
	// Framework and app settings
	Preset      Preset
	AppName     string
	Framework   string

	// Feature flags
	EnableNginx     bool
	EnableHorizon   bool
	EnableQueue     bool
	EnableScheduler bool
	EnableMetrics   bool
	EnableAPI       bool
	EnableTracing   bool

	// Process configuration
	PHPFPMWorkers   int
	QueueWorkers    int
	QueueConnection string

	// Ports
	APIPort     int
	MetricsPort int

	// Paths
	WorkDir     string
	LogLevel    string
}

// DefaultConfig returns default configuration for a preset
func DefaultConfig(preset Preset) *Config {
	cfg := &Config{
		Preset:          preset,
		AppName:         "my-app",
		WorkDir:         "/var/www/html",
		LogLevel:        "info",
		EnableMetrics:   true,
		EnableAPI:       true,
		APIPort:         9180,
		MetricsPort:     9090,
		PHPFPMWorkers:   5,
		QueueWorkers:    3,
		QueueConnection: "redis",
	}

	switch preset {
	case PresetLaravel:
		cfg.Framework = "laravel"
		cfg.EnableNginx = true
		cfg.EnableHorizon = true
		cfg.EnableQueue = true
		cfg.EnableScheduler = true
	case PresetSymfony:
		cfg.Framework = "symfony"
		cfg.EnableNginx = true
		cfg.EnableQueue = true
	case PresetProduction:
		cfg.Framework = "laravel"
		cfg.EnableNginx = true
		cfg.EnableHorizon = true
		cfg.EnableQueue = true
		cfg.EnableScheduler = true
		cfg.EnableMetrics = true
		cfg.EnableAPI = true
		cfg.EnableTracing = true
		cfg.LogLevel = "warn"
	case PresetMinimal:
		cfg.Framework = "generic"
		cfg.EnableNginx = false
		cfg.EnableHorizon = false
		cfg.EnableQueue = false
		cfg.EnableScheduler = false
		cfg.EnableMetrics = false
		cfg.EnableAPI = false
	case PresetGeneric:
		cfg.Framework = "generic"
		cfg.EnableNginx = true
	}

	return cfg
}

// ConfigTemplate is the main configuration template
const ConfigTemplate = `version: "1.0"

global:
  shutdown_timeout: 30
  log_level: {{ .LogLevel }}
  log_format: json
  {{- if .EnableAPI }}

  # Management API
  api_enabled: true
  api_port: {{ .APIPort }}
  {{- end }}
  {{- if .EnableMetrics }}

  # Prometheus Metrics
  metrics_enabled: true
  metrics_port: {{ .MetricsPort }}
  metrics_path: /metrics
  {{- end }}
  {{- if .EnableTracing }}

  # Distributed Tracing
  tracing_enabled: true
  tracing_exporter: otlp-grpc
  tracing_endpoint: localhost:4317
  tracing_sample_rate: 0.1
  tracing_service_name: {{ .AppName }}
  {{- end }}

  # Restart configuration
  restart_backoff_initial: 1s
  restart_backoff_max: 60s
  max_restart_attempts: 5

processes:
  {{- if eq .Framework "laravel" }}
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    type: longrun
    restart: always
    scale: 1
    stdout: true
    stderr: true
    health_check:
      type: tcp
      address: "127.0.0.1:9000"
      period: 30
      timeout: 5
      failure_threshold: 3
  {{- else if eq .Framework "symfony" }}
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    type: longrun
    restart: always
    scale: 1
    stdout: true
    stderr: true
  {{- end }}
  {{- if .EnableNginx }}

  nginx:
    enabled: true
    command: ["nginx", "-g", "daemon off;"]
    type: longrun
    restart: always
    scale: 1
    {{- if eq .Framework "laravel" }}
    depends_on:
      - php-fpm
    {{- end }}
    stdout: true
    stderr: true
    health_check:
      type: http
      url: "http://127.0.0.1:80/health"
      period: 30
      timeout: 5
      failure_threshold: 3
      expected_status: 200
  {{- end }}
  {{- if .EnableHorizon }}

  horizon:
    enabled: true
    command: ["php", "artisan", "horizon"]
    type: longrun
    restart: always
    scale: 1
    working_dir: {{ .WorkDir }}
    stdout: true
    stderr: true
    shutdown:
      pre_stop_hook:
        command: ["php", "artisan", "horizon:terminate"]
        timeout: 60
  {{- end }}
  {{- if .EnableQueue }}

  queue-default:
    enabled: true
    command: ["php", "artisan", "queue:work", "{{ .QueueConnection }}", "--queue=default", "--tries=3"]
    type: longrun
    restart: always
    scale: {{ .QueueWorkers }}
    working_dir: {{ .WorkDir }}
    stdout: true
    stderr: true
  {{- end }}
  {{- if .EnableScheduler }}

  scheduler:
    enabled: true
    command: ["php", "artisan", "schedule:work"]
    type: longrun
    restart: always
    scale: 1
    working_dir: {{ .WorkDir }}
    stdout: true
    stderr: true
  {{- end }}
`

// DockerComposeTemplate generates a docker-compose.yml file
const DockerComposeTemplate = `version: '3.8'

services:
  app:
    image: {{ .AppName }}:latest
    container_name: {{ .AppName }}
    restart: unless-stopped
    ports:
      {{- if .EnableNginx }}
      - "80:80"
      - "443:443"
      {{- end }}
      {{- if .EnableAPI }}
      - "{{ .APIPort }}:{{ .APIPort }}"
      {{- end }}
      {{- if .EnableMetrics }}
      - "{{ .MetricsPort }}:{{ .MetricsPort }}"
      {{- end }}
    volumes:
      - ./{{ .WorkDir }}:{{ .WorkDir }}
      - ./phpeek-pm.yaml:/etc/phpeek-pm/config.yaml:ro
    environment:
      - APP_ENV=production
      - APP_DEBUG=false
      {{- if eq .Framework "laravel" }}
      - DB_CONNECTION=mysql
      - DB_HOST=db
      - DB_PORT=3306
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      {{- end }}
    depends_on:
      {{- if eq .Framework "laravel" }}
      - db
      - redis
      {{- end }}
    networks:
      - app-network
{{- if eq .Framework "laravel" }}

  db:
    image: mysql:8.0
    container_name: {{ .AppName }}-db
    restart: unless-stopped
    environment:
      - MYSQL_ROOT_PASSWORD=secret
      - MYSQL_DATABASE={{ .AppName }}
      - MYSQL_USER={{ .AppName }}
      - MYSQL_PASSWORD=secret
    volumes:
      - db-data:/var/lib/mysql
    networks:
      - app-network

  redis:
    image: redis:7-alpine
    container_name: {{ .AppName }}-redis
    restart: unless-stopped
    volumes:
      - redis-data:/data
    networks:
      - app-network
{{- end }}
{{- if .EnableMetrics }}

  prometheus:
    image: prom/prometheus:latest
    container_name: {{ .AppName }}-prometheus
    restart: unless-stopped
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    networks:
      - app-network

  grafana:
    image: grafana/grafana:latest
    container_name: {{ .AppName }}-grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana-data:/var/lib/grafana
    networks:
      - app-network
{{- end }}

networks:
  app-network:
    driver: bridge

volumes:
{{- if eq .Framework "laravel" }}
  db-data:
  redis-data:
{{- end }}
{{- if .EnableMetrics }}
  prometheus-data:
  grafana-data:
{{- end }}
`

// DockerfileTemplate generates a Dockerfile
const DockerfileTemplate = `FROM php:8.2-fpm-alpine

# Install system dependencies
RUN apk add --no-cache \
    nginx \
    supervisor \
    bash \
    curl \
    git \
    zip \
    unzip

# Install PHP extensions
RUN docker-php-ext-install \
    pdo_mysql \
    opcache \
    pcntl \
    bcmath

# Install Composer
COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

# Set working directory
WORKDIR {{ .WorkDir }}

# Copy application
COPY . {{ .WorkDir }}

{{- if eq .Framework "laravel" }}
# Install PHP dependencies
RUN composer install --no-dev --optimize-autoloader

# Set permissions
RUN chown -R www-data:www-data {{ .WorkDir }}/storage {{ .WorkDir }}/bootstrap/cache
{{- end }}

# Copy PHPeek PM binary
COPY phpeek-pm /usr/local/bin/phpeek-pm
RUN chmod +x /usr/local/bin/phpeek-pm

# Copy configuration
COPY phpeek-pm.yaml /etc/phpeek-pm/config.yaml

# Expose ports
{{- if .EnableNginx }}
EXPOSE 80 443
{{- end }}
{{- if .EnableAPI }}
EXPOSE {{ .APIPort }}
{{- end }}
{{- if .EnableMetrics }}
EXPOSE {{ .MetricsPort }}
{{- end }}

# Start PHPeek PM
CMD ["/usr/local/bin/phpeek-pm", "serve", "--config", "/etc/phpeek-pm/config.yaml"]
`

// GenerateConfig generates configuration file from template
func GenerateConfig(cfg *Config) (string, error) {
	tmpl, err := template.New("config").Parse(ConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse config template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("failed to execute config template: %w", err)
	}

	return buf.String(), nil
}

// GenerateDockerCompose generates docker-compose.yml from template
func GenerateDockerCompose(cfg *Config) (string, error) {
	tmpl, err := template.New("docker-compose").Parse(DockerComposeTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse docker-compose template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("failed to execute docker-compose template: %w", err)
	}

	return buf.String(), nil
}

// GenerateDockerfile generates Dockerfile from template
func GenerateDockerfile(cfg *Config) (string, error) {
	tmpl, err := template.New("dockerfile").Parse(DockerfileTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse dockerfile template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("failed to execute dockerfile template: %w", err)
	}

	return buf.String(), nil
}

// ValidPresets returns list of valid preset names
func ValidPresets() []string {
	return []string{
		string(PresetLaravel),
		string(PresetSymfony),
		string(PresetGeneric),
		string(PresetMinimal),
		string(PresetProduction),
	}
}
