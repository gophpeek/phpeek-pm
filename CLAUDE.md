# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PHPeek Process Manager (phpeek-pm) is a production-grade PID 1 process manager for Docker containers, designed specifically for Laravel applications. Written in Go, it manages multiple processes (PHP-FPM, Nginx, Horizon, queue workers) with proper signal handling, zombie reaping, health checks, and graceful shutdown.

**Current Status**: Production-ready (Phase 7 complete). All core features implemented including multi-process orchestration, DAG-based dependencies, health checks, graceful shutdown, TUI, REST API, Prometheus metrics, distributed tracing, scaffolding tools, and dev mode.

## Build & Development Commands

### Building
```bash
make build              # Build for current platform → build/phpeek-pm
make build-all          # Build for all platforms (Linux/macOS, AMD64/ARM64)
make dev                # Build and run locally
make clean              # Remove build artifacts
```

### Testing
```bash
make test               # Run all tests with race detection and coverage
go test -v ./internal/process  # Test specific package
go test -run TestName   # Run specific test
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

### Dependencies
```bash
make deps               # Download and tidy dependencies
go mod download         # Download modules
go mod tidy             # Clean up go.mod/go.sum
```

### Running
```bash
# Show version and help
./build/phpeek-pm --version                    # Show version
./build/phpeek-pm -v                           # Show version (shorthand)
./build/phpeek-pm --help                       # Show help and all flags

# Run with config
./build/phpeek-pm                              # Auto-detects config (see priority order below)
./build/phpeek-pm --config configs/examples/minimal.yaml
./build/phpeek-pm -c configs/examples/minimal.yaml  # Shorthand
PHPEEK_PM_CONFIG=custom.yaml ./build/phpeek-pm

# Configuration Priority Order (highest to lowest):
# 1. --config flag (explicit, highest priority)
# 2. PHPEEK_PM_CONFIG environment variable
# 3. ~/.phpeek/pm/config.yaml (user-specific)
# 4. ~/.phpeek/pm/config.yml
# 5. /etc/phpeek/pm/config.yaml (system-wide)
# 6. /etc/phpeek/pm/config.yml
# 7. /etc/phpeek-pm/phpeek-pm.yaml (legacy)
# 8. phpeek-pm.yaml (current directory)

# PHP-FPM auto-tuning
./build/phpeek-pm --php-fpm-profile=medium                    # Via CLI flag
PHP_FPM_AUTOTUNE_PROFILE=medium ./build/phpeek-pm             # Via ENV var
docker run -e PHP_FPM_AUTOTUNE_PROFILE=medium myapp:latest    # Docker

# Validation modes
./build/phpeek-pm check-config                              # Full comprehensive validation
./build/phpeek-pm check-config --quiet                      # Quick summary only
./build/phpeek-pm check-config --strict                     # Fail on warnings (CI/CD)
./build/phpeek-pm check-config --json                       # JSON output for automation
./build/phpeek-pm check-config --config app.yaml            # Validate specific config
```

### CLI Flags
- `--version`, `-v` - Display version information and exit
- `--help`, `-h` - Display usage information and exit (auto-generated)
- `--config PATH`, `-c PATH` - Path to configuration file (overrides PHPEEK_PM_CONFIG env var)
- `--php-fpm-profile PROFILE` - Auto-tune PHP-FPM workers based on container limits (dev|light|medium|heavy|bursty)

### Config Validation & Linting

PHPeek PM includes comprehensive configuration validation with errors, warnings, and suggestions for best practices.

**Features:**
- ✅ **Errors** - Blocking issues that must be fixed before starting
- ⚠️ **Warnings** - Non-blocking issues that should be reviewed
- 💡 **Suggestions** - Best practice recommendations

**Validation Modes:**

```bash
# Full report mode (default)
./build/phpeek-pm check-config --config app.yaml
# Shows detailed report with all issues categorized

# Quiet mode (one-line summary)
./build/phpeek-pm check-config --quiet
# Output: ✅ Configuration is valid (with issues): ⚠️  3 warning(s), 💡 4 suggestion(s)

# Strict mode (CI/CD enforcement)
./build/phpeek-pm check-config --strict
# Exits with code 1 if warnings exist (perfect for CI pipelines)

# JSON mode (automation/scripting)
./build/phpeek-pm check-config --json
# Machine-readable JSON output with all validation results
```

**What's Validated:**

| Category | Examples |
|----------|----------|
| **Field Validation** | Data types, required fields, valid enum values |
| **Range Checks** | Timeouts, ports, scales, buffer sizes |
| **Security** | Missing authentication, hardcoded secrets, insecure defaults |
| **Best Practices** | Log formats, restart policies, health check intervals |
| **System Requirements** | Port privileges, OS compatibility, resource limits |
| **Dependencies** | Circular dependencies, missing dependencies |
| **Health Checks** | Valid URLs, reachable addresses, command existence |

**Example Output:**

```
═══════════════════════════════════════════════════════════════
  Configuration Validation Report
═══════════════════════════════════════════════════════════════

  Total Issues: 7  ⚠️  3 Warning(s)  💡 4 Suggestion(s)

⚠️  WARNINGS (should be reviewed):
───────────────────────────────────────────────────────────────
  1. [global.api_auth]
     API running without authentication or ACL
     → Recommendation: Consider enabling API token auth or IP ACL for security

💡 SUGGESTIONS (best practices):
───────────────────────────────────────────────────────────────
  1. [global.log_format]
     Text format is human-readable but not ideal for log aggregation
     → Consider: Use 'json' format for production with centralized logging

═══════════════════════════════════════════════════════════════
  ✅ Validation passed (with warnings)
═══════════════════════════════════════════════════════════════

📋 Configuration Summary:
   Path: configs/examples/app.yaml
   Version: 1.0
   Processes: 5
   Log Level: info
   Shutdown Timeout: 30s

✅ Configuration is valid but has warnings/suggestions
```

**CI/CD Integration:**

```yaml
# GitHub Actions example
- name: Validate Configuration
  run: |
    ./phpeek-pm check-config --strict --config production.yaml
    # Fails build if warnings exist

# Pre-commit hook example
#!/bin/bash
./phpeek-pm check-config --quiet --config phpeek-pm.yaml || exit 1
```

### TUI (Terminal User Interface)

PHPeek PM includes a modern k9s-style TUI for interactive process management with dual-mode connectivity.

**Connection Modes:**

The TUI client supports both Unix socket (local) and TCP (remote) connections with automatic detection:

- **Unix Socket (local, preferred)**:
  - Secure: File permissions (0600 = owner-only access)
  - Fast: No network stack overhead
  - Auto-detected paths: `/var/run/phpeek-pm.sock`, `/tmp/phpeek-pm.sock`, `/run/phpeek-pm.sock`
  - No authentication required (filesystem handles access control)

- **TCP (remote, fallback)**:
  - Remote access capability
  - Optional TLS encryption
  - Optional ACL-based IP filtering
  - Optional Bearer token authentication

**Auto-Detection Logic:**
1. Client tries each socket path in priority order
2. If socket exists and is accessible → Use Unix socket
3. If no socket found → Fall back to TCP connection

**Configuration:**

```yaml
global:
  # Enable API server
  api_enabled: true

  # TCP configuration (always available)
  api_port: 8080

  # Unix socket configuration (optional, recommended for local access)
  api_socket: "/var/run/phpeek-pm.sock"

  # Optional: Authentication (TCP only, socket uses file permissions)
  api_auth: "your-secret-token"

  # Optional: TLS for TCP connections
  api_tls:
    enabled: true
    cert_file: "/etc/phpeek-pm/server.crt"
    key_file: "/etc/phpeek-pm/server.key"

  # Optional: ACL for TCP connections
  api_acl:
    allow: ["127.0.0.1", "10.0.0.0/8"]
    deny: []
```

**TUI Keyboard Shortcuts:**

**Process List View:**
- `↑/↓` or `j/k` - Navigate process list
- `a` - Add new process (opens interactive wizard)
- `r` - Restart selected process
- `s` - Stop selected process
- `x` - Start selected process
- `+` or `=` - Scale up (opens dialog)
- `-` - Scale down (opens dialog)
- `Enter` - View process logs
- `?` - Show help
- `q` or `Esc` - Quit

**Add Process Wizard:**
- `Tab` or `Enter` - Next step
- `Shift+Tab` - Previous step
- `Esc` - Cancel wizard
- Step-specific: `Ctrl+W` (add command part), `Ctrl+D` (remove), `↑/↓` (select options)

**Usage Examples:**

```bash
# Local connection (uses socket if available, falls back to TCP)
./build/phpeek-pm tui

# Explicit TCP connection
./build/phpeek-pm tui --url http://localhost:8080

# Remote connection with authentication
./build/phpeek-pm tui --url http://remote-host:8080 --auth your-secret-token

# TLS connection
./build/phpeek-pm tui --url https://remote-host:8080 --auth your-secret-token
```

### Runtime Service Management API

PHPeek PM provides a comprehensive REST API for dynamic process management without daemon restarts. All operations persist to the configuration file.

**Base URL:** `http://localhost:8080/api/v1` (or Unix socket)

#### Process CRUD Operations

**List Processes**
```bash
GET /api/v1/processes

Response 200 OK:
{
  "processes": [
    {
      "name": "php-fpm",
      "status": "running",
      "instances": 2,
      "desired": 2,
      "restarts": 0,
      "pid": 12345,
      "uptime": "2h30m15s",
      "memory": "256MB",
      "cpu": "2.5%"
    }
  ]
}
```

**Add Process**
```bash
POST /api/v1/processes
Content-Type: application/json

{
  "name": "queue-worker",
  "process": {
    "enabled": true,
    "command": ["php", "artisan", "queue:work", "--tries=3"],
    "scale": 2,
    "restart": "always",
    "priority": 40
  }
}

Response 201 Created:
{
  "message": "Process 'queue-worker' created successfully",
  "process": { ... }
}
```

**Update Process**
```bash
PUT /api/v1/processes/{name}
Content-Type: application/json

{
  "process": {
    "enabled": true,
    "command": ["php", "artisan", "queue:work", "--tries=5"],
    "scale": 3,
    "restart": "on-failure",
    "priority": 25
  }
}

Response 200 OK:
{
  "message": "Process 'queue-worker' updated successfully"
}
```

**Remove Process**
```bash
DELETE /api/v1/processes/{name}

Response 200 OK:
{
  "message": "Process 'queue-worker' removed successfully"
}
```

#### Process Control Operations

**Start Process**
```bash
POST /api/v1/processes/{name}/start

Response 200 OK / 202 Accepted
```

**Stop Process**
```bash
POST /api/v1/processes/{name}/stop

Response 200 OK / 202 Accepted
```

**Restart Process**
```bash
POST /api/v1/processes/{name}/restart

Response 200 OK / 202 Accepted
```

**Scale Process**
```bash
POST /api/v1/processes/{name}/scale
Content-Type: application/json

{
  "desired": 5
}

Response 200 OK
```

#### Configuration Persistence

**Save Configuration**
```bash
POST /api/v1/config/save

Response 200 OK:
{
  "message": "Configuration saved to /etc/phpeek/pm/config.yaml"
}
```

**Reload Configuration**
```bash
POST /api/v1/config/reload

Response 200 OK:
{
  "message": "Configuration reloaded successfully"
}
```

**Validate Configuration**
```bash
POST /api/v1/config/validate

Response 200 OK:
{
  "valid": true,
  "message": "Configuration is valid"
}
```

#### Integration with TUI Wizard

The interactive wizard (`a` key in TUI) uses these API endpoints:

1. **Step 1-5**: User fills wizard steps (name, command, scale, restart, priority)
2. **Step 6 (Preview)**: User reviews configuration
3. **On confirmation**: TUI calls `POST /api/v1/processes` with wizard data
4. **Auto-save**: TUI automatically calls `POST /api/v1/config/save` after successful creation
5. **Process list refresh**: TUI updates display with new process

**Example workflow:**
```bash
# User presses 'a' in TUI
# Wizard collects: name="worker", command=["php","artisan","queue:work"], scale=2
# TUI sends: POST /api/v1/processes with complete config
# TUI sends: POST /api/v1/config/save to persist
# TUI refreshes list showing new "worker" process
```

#### API Authentication

**Unix Socket (local):** No authentication required - filesystem permissions control access (recommended: 0600)

**TCP (remote):** Optional Bearer token authentication
```bash
curl -H "Authorization: Bearer your-secret-token" \
     http://localhost:8080/api/v1/processes
```

**Configuration:**
```yaml
global:
  api_auth: "your-secret-token"  # Optional, TCP only
```

#### Error Responses

All API errors follow consistent format:
```json
{
  "error": "Process 'unknown' not found"
}
```

Common status codes:
- `200 OK` - Success
- `201 Created` - Resource created
- `202 Accepted` - Operation accepted (async)
- `400 Bad Request` - Invalid input
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource already exists
- `500 Internal Server Error` - Server error

### PHP-FPM Auto-Tuning

PHPeek PM can automatically calculate optimal PHP-FPM worker settings based on container resource limits (memory/CPU) detected via cgroups v1/v2.

**Application Profiles:**

| Profile | Use Case | Avg Memory/Worker | Traffic Load | PM Mode |
|---------|----------|-------------------|--------------|---------|
| `dev` | Development | 64MB | N/A | static (2 workers) |
| `light` | Small apps, low traffic | 128MB | 1-10 req/s | dynamic |
| `medium` | Standard production | 256MB | 10-50 req/s | dynamic |
| `heavy` | High traffic apps | 512MB | 50-200 req/s | dynamic |
| `bursty` | Traffic spike handling | 256MB | Variable spikes | dynamic (high spare) |

**Safety Features:**
- Never uses >80% of available memory (profile-dependent)
- Reserves memory for Nginx, system, and other services
- Enforces CPU limits (max 4 workers per core)
- Validates all calculations before applying
- Enforces profile minimums and safe PM relationships

**Usage:**
```bash
# CLI flag (explicit)
./build/phpeek-pm --php-fpm-profile=medium

# Environment variable (recommended for containers)
PHP_FPM_AUTOTUNE_PROFILE=medium ./build/phpeek-pm

# Docker / Docker Compose (set via environment)
docker run -e PHP_FPM_AUTOTUNE_PROFILE=medium myapp:latest

# CLI flag overrides ENV var
PHP_FPM_AUTOTUNE_PROFILE=light ./build/phpeek-pm --php-fpm-profile=heavy
# Uses: heavy (CLI takes priority)

# Test autotune without starting (dry-run)
./build/phpeek-pm --php-fpm-profile=medium --dry-run
```

**Priority:** CLI flag `--php-fpm-profile` > ENV var `PHP_FPM_AUTOTUNE_PROFILE`

**How It Works:**
1. Detects container limits from cgroup v1/v2 (memory + CPU quota)
2. Calculates optimal `pm.max_children` based on available memory
3. Sets `pm.start_servers`, `pm.min_spare_servers`, `pm.max_spare_servers` ratios
4. Configures `pm.max_requests` for memory leak protection
5. Exports environment variables: `PHP_FPM_PM`, `PHP_FPM_MAX_CHILDREN`, etc.

**Environment Variables Set:**
```bash
PHP_FPM_PM=dynamic
PHP_FPM_MAX_CHILDREN=10
PHP_FPM_START_SERVERS=3
PHP_FPM_MIN_SPARE=2
PHP_FPM_MAX_SPARE=5
PHP_FPM_MAX_REQUESTS=1000
```

**PHP-FPM Pool Configuration Integration:**

To use auto-tuned values in your PHP-FPM pool config (`www.conf`):

```ini
[www]
pm = ${PHP_FPM_PM}
pm.max_children = ${PHP_FPM_MAX_CHILDREN}
pm.start_servers = ${PHP_FPM_START_SERVERS}
pm.min_spare_servers = ${PHP_FPM_MIN_SPARE}
pm.max_spare_servers = ${PHP_FPM_MAX_SPARE}
pm.max_requests = ${PHP_FPM_MAX_REQUESTS}
```

**Example Output:**
```
🎯 PHP-FPM auto-tuned (medium profile):
   pm = dynamic
   pm.max_children = 10
   pm.start_servers = 3
   pm.min_spare_servers = 2
   pm.max_spare_servers = 5
   pm.max_requests = 1000
   Memory: 2560MB allocated / 4096MB total
```

### Resource Metrics & Monitoring

PHPeek PM tracks resource usage (CPU, memory, threads, file descriptors) for all managed process instances, with time series storage and both REST API and Prometheus exposition.

**Features:**
- Per-instance resource tracking with configurable collection interval
- Time series ring buffer for historical data (default: 720 samples = 1 hour at 5s interval)
- REST API for querying metrics history with flexible time ranges
- Prometheus metrics exposition (when `metrics_enabled: true`)
- Zero overhead when disabled

**Configuration:**

```yaml
global:
  resource_metrics_enabled: true           # Enable resource tracking (default: false)
  resource_metrics_interval: 5             # Collection interval in seconds (default: 5)
  resource_metrics_max_samples: 720        # Max samples per instance (default: 720)

  metrics_enabled: true                    # Enable Prometheus HTTP server (default: false)
  metrics_port: 9090                       # Prometheus metrics port (default: 9090)
  metrics_path: /metrics                   # Prometheus endpoint path (default: /metrics)
```

**Collected Metrics:**

| Metric | Description | Unit |
|--------|-------------|------|
| `cpu_percent` | CPU usage percentage | Percent (0-100 per core) |
| `memory_rss_bytes` | Resident Set Size (physical memory) | Bytes |
| `memory_vms_bytes` | Virtual Memory Size | Bytes |
| `memory_percent` | Memory usage as % of total system memory | Percent (0-100) |
| `threads` | Number of threads | Count |
| `file_descriptors` | Open file descriptors (Linux only) | Count |

**REST API Endpoint:**

```bash
# Query metrics history for a process instance
GET /api/v1/metrics/history?process=<name>&instance=<id>&since=<timestamp>&limit=<N>

# Parameters:
#   process  - Process name (required)
#   instance - Instance ID (required)
#   since    - Start time (RFC3339 or Unix timestamp, default: 1 hour ago)
#   limit    - Max samples to return (1-10000, default: 100)

# Example:
curl "http://localhost:8080/api/v1/metrics/history?process=php-fpm&instance=php-fpm-0&limit=20"
```

**Response Format:**

```json
{
  "process": "php-fpm",
  "instance": "php-fpm-0",
  "since": "2025-11-23T08:00:00Z",
  "limit": 20,
  "samples": 20,
  "data": [
    {
      "timestamp": "2025-11-23T09:27:02.263Z",
      "cpu_percent": 12.5,
      "memory_rss_bytes": 134217728,
      "memory_vms_bytes": 445747003392,
      "memory_percent": 1.95,
      "threads": 8,
      "file_descriptors": 42
    }
  ]
}
```

**Prometheus Metrics:**

When `metrics_enabled: true`, resource metrics are exposed on the Prometheus endpoint:

```bash
# Prometheus gauges (per process instance):
phpeek_pm_process_cpu_percent{process="php-fpm", instance="php-fpm-0"}
phpeek_pm_process_memory_bytes{process="php-fpm", instance="php-fpm-0", type="rss"}
phpeek_pm_process_memory_bytes{process="php-fpm", instance="php-fpm-0", type="vms"}
phpeek_pm_process_memory_percent{process="php-fpm", instance="php-fpm-0"}
phpeek_pm_process_threads{process="php-fpm", instance="php-fpm-0"}
phpeek_pm_process_file_descriptors{process="php-fpm", instance="php-fpm-0"}

# Collection metadata:
phpeek_pm_resource_collection_errors_total{process="...", instance="..."}
phpeek_pm_resource_collection_duration_seconds

# Query Prometheus:
curl http://localhost:9090/metrics | grep phpeek_pm_process
```

**Implementation Details:**

- **Shared Collector Pattern**: One `ResourceCollector` instance shared across all supervisors
- **Goroutine Per Supervisor**: Each supervisor runs a ticker-based collection loop
- **Lazy Buffer Initialization**: Time series buffers created on first sample (memory efficient)
- **Graceful Degradation**: Collection errors are logged but don't crash processes
- **Clean Shutdown**: Collection goroutines respect context cancellation

**Performance Considerations:**

- **Memory Usage**: Each sample ~96 bytes × max_samples × instance_count
  - Example: 720 samples × 10 instances = ~691 KB
- **CPU Overhead**: ~1ms per collection cycle (depends on instance count)
- **Recommended Intervals**:
  - Development: 2-5 seconds
  - Production: 10-30 seconds
  - High-scale: 60+ seconds

**Example Grafana Query:**

```promql
# Average CPU usage across all PHP-FPM instances
avg(phpeek_pm_process_cpu_percent{process="php-fpm"})

# Memory usage trend for specific instance
phpeek_pm_process_memory_bytes{process="nginx", instance="nginx-0", type="rss"}

# Total threads across all processes
sum(phpeek_pm_process_threads)
```

### Distributed Tracing (OpenTelemetry)

PHPeek PM supports distributed tracing using OpenTelemetry for deep observability into process lifecycle operations.

**Configuration:**

```yaml
global:
  # Enable distributed tracing
  tracing_enabled: true                        # Enable OpenTelemetry tracing (default: false)
  tracing_exporter: otlp-grpc                  # Exporter type: otlp-grpc | stdout
  tracing_endpoint: localhost:4317             # Exporter endpoint (default depends on exporter)
  tracing_sample_rate: 1.0                     # Sampling rate 0.0-1.0 (default: 1.0 = 100%)
  tracing_service_name: phpeek-pm              # Service name in traces (default: phpeek-pm)
```

**Supported Exporters:**

- **otlp-grpc**: OpenTelemetry Protocol over gRPC (production, works with Jaeger, Grafana Tempo, etc.)
- **stdout**: Pretty-printed JSON to stdout (development/debugging only)

**Instrumentation Coverage:**

PHPeek PM automatically creates spans for:

- **Process Manager Operations**:
  - `process_manager.start` - Overall startup with process count
  - `process_manager.start_process` - Individual process startup with name and scale
  - `process_manager.shutdown` - Graceful shutdown with process count

**Span Attributes:**

Each span includes contextual attributes:
- Process names, instance IDs, scale counts
- Error information when operations fail
- Service name and version in resource attributes

**Example with Jaeger:**

```yaml
global:
  tracing_enabled: true
  tracing_exporter: otlp-grpc
  tracing_endpoint: jaeger:4317
  tracing_sample_rate: 1.0
  tracing_service_name: phpeek-pm-production
```

**Example with Grafana Tempo:**

```yaml
global:
  tracing_enabled: true
  tracing_exporter: otlp-grpc
  tracing_endpoint: tempo:4317
  tracing_sample_rate: 0.1                     # 10% sampling for production
  tracing_service_name: phpeek-pm
```

**Development/Testing:**

```yaml
global:
  tracing_enabled: true
  tracing_exporter: stdout                     # Output traces to console
  tracing_sample_rate: 1.0
  tracing_service_name: phpeek-pm-dev
```

**Performance Considerations:**

- **Sampling**: Use lower sample rates (0.1 = 10%) in high-throughput production
- **Overhead**: Minimal (<1ms per span) with OTLP gRPC exporter
- **Memory**: Batched export keeps memory usage low
- **Network**: OTLP uses efficient protocol buffers over gRPC

**Trace Context Propagation:**

Spans are hierarchical with proper parent-child relationships:
- `process_manager.start` (root)
  - `process_manager.start_process` (child for each process)

**Integration:**

Works seamlessly with:
- Jaeger (via OTLP)
- Grafana Tempo (via OTLP)
- Honeycomb (via OTLP)
- Any OpenTelemetry-compatible backend

**Future Enhancements:**

- Additional exporters (Jaeger native, Zipkin)
- TLS support for OTLP gRPC
- HTTP health check span instrumentation
- Custom span events for process state changes

## Scaffolding Tools

PHPeek PM includes scaffolding tools to quickly generate configuration files for common PHP frameworks and deployment scenarios. The scaffold command creates production-ready `phpeek-pm.yaml` configurations along with optional Docker Compose and Dockerfile templates.

### Available Presets

#### 1. Laravel (Full Stack)
Complete Laravel application setup with all services:
- **PHP-FPM**: TCP health check on port 9000
- **Nginx**: HTTP health check with PHP-FPM dependency
- **Horizon**: Redis queue manager with graceful shutdown hook
- **Queue Workers**: 3 instances with retry logic
- **Scheduler**: Laravel cron replacement
- **Features**: API + Metrics enabled by default

```bash
./build/phpeek-pm scaffold laravel --output /path/to/project
```

#### 2. Production (Laravel + Observability)
Production-ready Laravel with complete observability stack:
- All Laravel preset features
- **Distributed Tracing**: OpenTelemetry with OTLP gRPC
- **Log Level**: Warning (reduced noise in production)
- **Sample Rate**: 10% (performance optimized)
- **Docker Compose**: Includes Prometheus + Grafana

```bash
./build/phpeek-pm scaffold production --output /path/to/project --docker-compose
```

#### 3. Symfony
Symfony application with essential services:
- **PHP-FPM**: FastCGI process manager
- **Nginx**: Web server with health checks
- **Queue Workers**: Symfony Messenger workers
- **Features**: API + Metrics enabled

```bash
./build/phpeek-pm scaffold symfony --output /path/to/project
```

#### 4. Generic
Basic PHP application with web server:
- **Nginx**: Standalone web server
- **Features**: API + Metrics enabled
- Use case: Static sites, simple PHP apps without framework

```bash
./build/phpeek-pm scaffold generic --output /path/to/project
```

#### 5. Minimal
Bare minimum configuration template:
- No processes pre-configured
- Global settings only
- Use case: Starting from scratch, custom setups

```bash
./build/phpeek-pm scaffold minimal --output /path/to/project
```

### CLI Usage

#### Basic Usage
```bash
# Generate Laravel configuration
./build/phpeek-pm scaffold laravel

# Specify output directory
./build/phpeek-pm scaffold laravel --output ./docker

# Generate with Docker files
./build/phpeek-pm scaffold laravel --dockerfile --docker-compose

# Customize app name and workers
./build/phpeek-pm scaffold laravel --app-name my-app --queue-workers 5
```

#### Interactive Mode
```bash
./build/phpeek-pm scaffold --interactive

# Prompts for:
# 1. Preset selection (1-5)
# 2. Application name
# 3. Log level (debug/info/warn/error)
# 4. Queue worker count (Laravel/Symfony)
# 5. Queue connection (redis/database/sqs)
# 6. Feature toggles (metrics/API/tracing)
# 7. Docker file generation
```

#### CLI Flags
- `--interactive, -i`: Interactive mode with prompts
- `--output, -o PATH`: Output directory (default: current directory)
- `--dockerfile`: Generate Dockerfile
- `--docker-compose`: Generate docker-compose.yml
- `--app-name STRING`: Application name (default: "my-app")
- `--queue-workers INT`: Number of queue workers (default: 3)

### Generated Files

#### phpeek-pm.yaml
Main configuration file with process definitions, health checks, and global settings. Includes:
- Process orchestration with priorities
- Health checks (TCP/HTTP)
- Graceful shutdown hooks
- Resource scaling
- Logging configuration
- Optional observability (metrics, tracing, API)

#### docker-compose.yml (with --docker-compose)
Complete Docker Compose stack:
- Application container with PHPeek PM
- MySQL 8.0 database
- Redis cache/queue
- Prometheus metrics collection (production preset)
- Grafana dashboards (production preset)
- Network and volume configuration
- Port mappings (80, 443, 8080, 9090)

#### Dockerfile (with --dockerfile)
Multi-stage PHP 8.2 Docker image:
- Base: Official PHP-FPM with extensions
- Dependencies: Composer packages + system libraries
- PHPeek PM binary
- Optimized layers with caching
- Production-ready settings
- Health check integration

### Preset Comparison

| Feature | Minimal | Generic | Symfony | Laravel | Production |
|---------|---------|---------|---------|---------|------------|
| PHP-FPM | - | - | ✅ | ✅ | ✅ |
| Nginx | - | ✅ | ✅ | ✅ | ✅ |
| Horizon | - | - | - | ✅ | ✅ |
| Queue Workers | - | - | ✅ | ✅ | ✅ |
| Scheduler | - | - | - | ✅ | ✅ |
| Metrics | - | ✅ | ✅ | ✅ | ✅ |
| API | - | ✅ | ✅ | ✅ | ✅ |
| Tracing | - | - | - | - | ✅ |
| Health Checks | - | ✅ | ✅ | ✅ | ✅ |
| Docker Compose | - | - | - | - | ✅ (default) |

### Customization Workflow

1. **Generate Base Configuration**
   ```bash
   ./build/phpeek-pm scaffold laravel --output ./myapp
   ```

2. **Review Generated Files**
   ```bash
   cd ./myapp
   cat phpeek-pm.yaml  # Check process configuration
   ```

3. **Customize for Your Needs**
   - Adjust worker counts (`scale: 3` → `scale: 5`)
   - Modify health check endpoints
   - Change log levels
   - Add/remove processes
   - Configure environment-specific settings

4. **Test Configuration**
   ```bash
   phpeek-pm --config ./myapp/phpeek-pm.yaml --validate-config
   phpeek-pm --config ./myapp/phpeek-pm.yaml --dry-run
   ```

5. **Deploy**
   ```bash
   # Docker Compose
   cd ./myapp && docker-compose up -d

   # Or standalone
   phpeek-pm --config ./myapp/phpeek-pm.yaml
   ```

### Template Architecture

The scaffolding system uses Go's `text/template` package with conditional rendering:

**Config Struct** (`internal/scaffold/config.go`):
- Preset selection (laravel/symfony/generic/minimal/production)
- Feature flags (EnableNginx, EnableHorizon, EnableMetrics, etc.)
- Configurable values (QueueWorkers, LogLevel, Ports)

**Templates** (`internal/scaffold/templates.go`):
- ConfigTemplate: Conditional YAML generation based on feature flags
- DockerComposeTemplate: Full stack with MySQL, Redis, observability
- DockerfileTemplate: Multi-stage PHP 8.2 build with extensions

**Generator** (`internal/scaffold/generator.go`):
- Orchestrates file generation from templates
- Applies preset defaults
- Supports customization via setter methods
- Handles file writing with error checking

### Examples

#### Example 1: Quick Laravel Development Setup
```bash
# Generate Laravel config for local development
./build/phpeek-pm scaffold laravel \
  --output ./docker \
  --app-name my-laravel-app \
  --queue-workers 2

# Result: phpeek-pm.yaml with 2 queue workers, API + Metrics enabled
```

#### Example 2: Production Deployment with Observability
```bash
# Generate production config with full observability stack
./build/phpeek-pm scaffold production \
  --output ./production \
  --app-name my-app-prod \
  --docker-compose

# Result:
# - phpeek-pm.yaml (with tracing, metrics, API)
# - docker-compose.yml (with Prometheus + Grafana)
# - Complete observability stack ready for deployment
```

#### Example 3: Interactive Configuration
```bash
./build/phpeek-pm scaffold --interactive

# Sample session:
# Select a preset: 1 (Laravel)
# Application name [my-app]: demo-app
# Log level [info]: warn
# Number of queue workers [3]: 5
# Queue connection [redis]: redis
# Enable Prometheus metrics? [y]: y
# Enable Management API? [y]: y
# Enable distributed tracing? [n]: n
# Generate docker-compose.yml? [n]: y
# Generate Dockerfile? [n]: n
```

#### Example 4: Minimal Customization Starting Point
```bash
# Start with minimal template for full customization
./build/phpeek-pm scaffold minimal --output ./custom

# Edit phpeek-pm.yaml manually to add your processes
vim ./custom/phpeek-pm.yaml
```

### Best Practices

1. **Start with Preset**: Choose the preset closest to your framework/needs
2. **Validate Early**: Use `--validate-config` before deployment
3. **Test with Dry Run**: Use `--dry-run` to verify without starting processes
4. **Version Control**: Commit generated configs to Git for reproducibility
5. **Environment Variables**: Use env vars for secrets and environment-specific values
6. **Customize Health Checks**: Adjust periods/timeouts based on app characteristics
7. **Scale Appropriately**: Start conservative with workers, scale based on metrics
8. **Enable Observability**: Use production preset for staging/production environments

## Local Development Mode

PHPeek PM includes a development mode with configuration file watching and automatic reload capabilities, making it easier to iterate on configurations during development.

### Overview

Development mode enables:
- **File Watching**: Monitors configuration file for changes using fsnotify
- **Auto-Reload**: Automatically reloads configuration when changes are detected
- **Debouncing**: Prevents multiple rapid reloads (2-second debounce period)
- **Graceful Reload**: Cleanly shuts down existing processes before applying changes

### Enabling Dev Mode

```bash
# Start with dev mode enabled
./build/phpeek-pm serve --dev

# Or with explicit config path
./build/phpeek-pm serve --config phpeek-pm.yaml --dev
```

### How It Works

1. **Startup**: PHPeek PM starts normally and initializes all processes
2. **Watcher**: File watcher monitors the configuration file for Write/Create events
3. **Change Detection**: When config file is modified and saved, watcher detects the change
4. **Validation**: New configuration is loaded and validated before triggering reload
5. **Reload**: If validation passes, graceful shutdown of all processes is initiated
6. **Exit**: PHPeek PM exits cleanly with message "Config reload complete"
7. **Restart**: User restarts PHPeek PM to apply new configuration

### Workflow Example

```bash
# Terminal 1: Start PHPeek PM in dev mode
$ ./build/phpeek-pm serve --config myconfig.yaml --dev

🚀 PHPeek Process Manager v1.0.0
...
time=2025-11-23T15:23:38.935+01:00 level=INFO msg="Development mode enabled" watch_config=/path/to/myconfig.yaml
time=2025-11-23T15:23:38.935+01:00 level=INFO msg="Config watcher started" path=/path/to/myconfig.yaml debounce=2s
```

```bash
# Terminal 2: Edit configuration
$ vim myconfig.yaml  # Make changes
$ # Save file (:wq)
```

```bash
# Terminal 1: Automatic reload triggered
time=2025-11-23T15:23:58.750+01:00 level=INFO msg="Config file changed, triggering reload" path=/path/to/myconfig.yaml event=WRITE
time=2025-11-23T15:23:58.751+01:00 level=INFO msg="Config reload triggered"
time=2025-11-23T15:23:58.752+01:00 level=INFO msg="Performing config reload"
time=2025-11-23T15:23:58.752+01:00 level=INFO msg="Initiating graceful shutdown" reason="config reload" timeout=30
...
time=2025-11-23T15:23:58.752+01:00 level=INFO msg="Config reload complete - restart PHPeek PM to apply changes"

# Now restart to apply changes
$ ./build/phpeek-pm serve --config myconfig.yaml --dev
```

### Configuration Validation

Dev mode validates configuration before triggering reload:

```yaml
# If you make an invalid change:
processes:
  nginx:
    depends_on: [non-existent-process]  # Invalid!
```

```bash
# Watcher detects change but validation fails
time=2025-11-23T15:24:10.123+01:00 level=INFO msg="Config file changed, triggering reload"
time=2025-11-23T15:24:10.124+01:00 level=ERROR msg="Config reload failed" error="invalid config: depends_on contains unknown process: 'non-existent-process'"
# PHPeek PM continues running with old configuration
```

### Debouncing

The watcher includes a 2-second debounce period to prevent multiple rapid reloads when editors save files multiple times:

```bash
# Multiple save events within 2 seconds are collapsed
time=2025-11-23T15:24:20.100+01:00 level=INFO msg="Config file changed, triggering reload"
time=2025-11-23T15:24:20.500+01:00 level=DEBUG msg="Config change debounced" since_last_reload=0.4s
time=2025-11-23T15:24:21.000+01:00 level=DEBUG msg="Config change debounced" since_last_reload=0.9s
# Only one reload is triggered after 2 seconds of stability
```

### Architecture

**Watcher Package** (`internal/watcher/watcher.go`):
- Uses [fsnotify](https://github.com/fsnotify/fsnotify) for cross-platform file system events
- Monitors specific config file path (not directory)
- Filters for WRITE and CREATE events only
- Thread-safe with mutex protection
- Configurable debounce period
- Custom reload handler callback

**Integration** (`cmd/phpeek-pm/serve.go`):
- Watcher created only when `--dev` flag is set
- Reload handler validates config before triggering
- Reload signal communicated via buffered channel
- Main wait loop listens for reload events alongside shutdown signals
- Graceful shutdown executed on reload
- Clean exit with informative message

### Use Cases

**1. Configuration Iteration**
```bash
# Quickly test different worker counts
# Edit config: scale: 3 → scale: 5
# Save → Auto reload
# Observe behavior
# Edit config: scale: 5 → scale: 10
# Save → Auto reload
# Rinse and repeat
```

**2. Health Check Tuning**
```bash
# Adjust health check periods
# Edit config: period: 30 → period: 10
# Save → Auto reload
# Monitor health check behavior
# Fine-tune based on observations
```

**3. Log Level Debugging**
```bash
# Start with info level
# Hit an issue → Edit config: log_level: info → log_level: debug
# Save → Auto reload with debug logging
# Investigate with detailed logs
# Restore → Edit config: log_level: debug → log_level: info
# Save → Auto reload back to normal
```

**4. Process Configuration Testing**
```bash
# Test different command arguments
# Edit config: command: ["php", "artisan", "queue:work"]
# Save → Auto reload
# Verify behavior
# Iterate on configuration
```

### Best Practices

1. **Use in Development Only**: Dev mode is designed for local development, not production
2. **Validate Before Save**: Check your config syntax before saving to avoid reload failures
3. **Watch the Logs**: Keep terminal visible to see reload status and any errors
4. **Small Iterations**: Make small, incremental changes for easier debugging
5. **Test Validation**: Intentionally make invalid changes to verify validation works
6. **Graceful Shutdown**: Wait for graceful shutdown to complete before making more changes

### Limitations

1. **Manual Restart Required**: PHPeek PM exits after reload (does not auto-restart)
2. **Full Restart**: All processes are stopped and started (no hot reload of individual processes)
3. **Debounce Delay**: 2-second minimum between reloads
4. **File System Events**: Relies on OS file system notifications (may not work on some network filesystems)
5. **Validation Only**: Only catches configuration syntax errors, not runtime issues

### Troubleshooting

**Watcher Not Detecting Changes**:
```bash
# Check if file path is absolute
time=2025-11-23T15:23:38.935+01:00 level=INFO msg="Config watcher started" path=/absolute/path/to/config.yaml

# Try explicit config path
./build/phpeek-pm serve --config /absolute/path/to/config.yaml --dev
```

**Reload Failing**:
```bash
# Check validation error message
time=2025-11-23T15:24:10.124+01:00 level=ERROR msg="Config reload failed" error="..."

# Validate manually
./build/phpeek-pm serve --config config.yaml --validate-config
```

**Multiple Reloads**:
```bash
# Increase debounce if needed (requires code modification)
# Default: 2 seconds in cmd/phpeek-pm/serve.go:217

# Some editors save multiple times - this is normal
# Watch for "Config change debounced" debug messages
```

### Future Enhancements

Potential improvements for dev mode:
- **Hot Reload**: Reload only changed processes without full restart
- **Auto-Restart**: Automatically restart PHPeek PM after config reload
- **Config Diff**: Show which configuration values changed
- **Validation Warnings**: Non-fatal warnings that don't prevent reload
- **Interactive Confirmation**: Prompt before applying changes in interactive mode
- **Rollback**: Ability to rollback to previous configuration on failure

## Architecture

### Core Design Principles

1. **Interfaces over Concrete Types**: All major components define interfaces for testability
   - `ProcessManager` interface in `internal/process/manager.go`
   - Components should accept interfaces, return concrete structs

2. **Dependency Injection**: Pass dependencies explicitly through constructors
   - Logger is passed to all components via constructor
   - Config passed during initialization
   - No global state except logger (set via `slog.SetDefault()`)

3. **Error Wrapping**: Use `fmt.Errorf("context: %w", err)` for error chains
   - Provides stack trace context
   - Allows error unwrapping with `errors.Is()` and `errors.As()`

4. **Context Propagation**: Pass `context.Context` as first parameter for cancellation/timeouts
   - All blocking operations accept context
   - Respect context cancellation in goroutines

5. **Graceful Degradation**: Non-critical failures log warnings, don't crash
   - Only fatal errors should cause shutdown
   - Use appropriate log levels (debug/info/warn/error)

### Directory Structure

```
cmd/phpeek-pm/          # Main entry point
internal/
  config/               # Configuration loading and validation
    config.go          # YAML + env var loading with precedence
    types.go           # Config structs (Config, Process, HealthCheck, etc.)
  logger/              # Structured logging (slog)
    logger.go          # Logger initialization
    process_writer.go  # Per-process log segmentation
  process/             # Process management core
    manager.go         # Multi-process orchestration
    supervisor.go      # Single process lifecycle management
  signals/             # Signal handling and zombie reaping
    handler.go         # PID 1 signal handling
configs/examples/      # Example configurations
  minimal.yaml         # Simple PHP-FPM setup
  laravel-full.yaml    # Complete Laravel stack
```

### Key Components

#### Configuration System (`internal/config/`)
- **Load Priority**: Environment variables → YAML file → Defaults
- **Environment Variables**: `PHPEEK_PM_GLOBAL_LOG_LEVEL`, `PHPEEK_PM_PROCESS_<NAME>_ENABLED`
- **Validation**: Checks for circular dependencies, required fields, valid values
- **Defaults**: Applied in `SetDefaults()` method

#### Process Management (`internal/process/`)
- **Manager**: Orchestrates multiple processes with startup/shutdown ordering
  - `getStartupOrder()`: Priority-based topological sort (Phase 1: simple priority, Phase 2+: DAG with dependencies)
  - `getShutdownOrder()`: Reverse of startup order
  - Parallel shutdown with error collection

- **Supervisor**: Manages lifecycle of single process with scaling
  - Creates multiple instances based on `Scale` config
  - Handles restart policies (always/on-failure/never)
  - Per-process logging with structured fields

#### Signal Handling (`internal/signals/`)
- **PID 1 Capability**: Proper zombie reaping with `signals.ReapZombies()`
- **Signal Handling**: SIGTERM, SIGINT, SIGQUIT trigger graceful shutdown
- Critical for Docker containers running as PID 1

#### Logging (`internal/logger/`)
- **Structured Logging**: Uses Go's `log/slog` for JSON/text output
- **Log Levels**: debug, info, warn, error
- **Process Segmentation**: Per-process log labels for filtering

## Code Patterns & Standards

### Good Pattern Examples

```go
// ✅ GOOD: Interface + DI + Error wrapping + Context
type ProcessManager interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type manager struct {
    config  *config.Config
    logger  *slog.Logger
    procs   map[string]Process
}

func NewManager(cfg *config.Config, log *slog.Logger) ProcessManager {
    return &manager{
        config: cfg,
        logger: log,
        procs:  make(map[string]Process),
    }
}

func (m *manager) Start(ctx context.Context) error {
    if err := m.validateConfig(); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }
    // ... implementation
    return nil
}
```

### Anti-Patterns to Avoid

```go
// ❌ BAD: No interface, globals, no error handling, no context
var Procs map[string]*Process

func StartAll() {
    for _, p := range Procs {
        p.Start()  // No error handling, no context
    }
}

// ❌ BAD: No error wrapping
if err != nil {
    return err  // Lost context about where error occurred
}

// ❌ BAD: Shared state without mutex
func (m *Manager) GetProcess(name string) *Process {
    return m.processes[name]  // Race condition without m.mu.RLock()
}
```

### Testing Standards

```go
// Unit test pattern
func TestSupervisor_Start(t *testing.T) {
    tests := []struct {
        name    string
        config  *config.Process
        wantErr bool
    }{
        {
            name: "successful start",
            config: &config.Process{
                Command: []string{"sleep", "1"},
                Scale:   1,
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            logger := slog.Default()
            sup := NewSupervisor("test", tt.config, logger)

            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()

            err := sup.Start(ctx)
            if (err != nil) != tt.wantErr {
                t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
            }

            // Cleanup
            if err == nil {
                sup.Stop(ctx)
            }
        })
    }
}
```

### Go Idioms to Follow

1. **Accept interfaces, return structs** - Functions take interfaces, return concrete types
2. **Make zero values useful** - `&Manager{}` should be safe (use `New*()` constructors)
3. **Check every error** - Never ignore errors, wrap with context
4. **Defer cleanup** - Use `defer` for locks, file closes, cancellations
5. **Channels for communication** - Prefer channels over shared memory
6. **Goroutine lifecycle** - Always have clear termination via context or close

## Implementation Status

All phases are **complete**. PHPeek PM is production-ready.

### Core Features (Phase 1-2)
- ✅ Multi-process orchestration with DAG-based dependency resolution
- ✅ Configuration via YAML + environment variables
- ✅ Structured logging (JSON/text)
- ✅ PID 1 signal handling and zombie reaping
- ✅ Graceful shutdown with timeouts
- ✅ Health checks (TCP, HTTP, exec)
- ✅ Restart policies with exponential backoff
- ✅ Lifecycle hooks (pre/post start/stop)

### Management & API (Phase 6)
- ✅ Management REST API with dual-mode connectivity (Unix socket + TCP)
- ✅ Modern k9s-style TUI with keyboard-driven interface
- ✅ Runtime process control (start/stop/restart/scale)
- ✅ IP ACL and token authentication
- ✅ TLS support for API endpoints

### Observability & DX (Phase 7)
- ✅ Prometheus metrics with resource monitoring
- ✅ Distributed tracing (OpenTelemetry)
- ✅ Config validation and linting (`check-config`)
- ✅ Scaffolding tools for Laravel/Symfony/Generic presets
- ✅ Dev mode with file watching and auto-reload

## Configuration Examples

### Minimal PHP-FPM
```yaml
version: "1.0"
global:
  shutdown_timeout: 30
  log_level: info

processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    restart: always
```

### Laravel Full Stack
```yaml
processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    depends_on: []

  nginx:
    enabled: true
    command: ["nginx", "-g", "daemon off;"]
    depends_on: [php-fpm]

  horizon:
    enabled: true
    command: ["php", "artisan", "horizon"]
    shutdown:
      pre_stop_hook:
        command: ["php", "artisan", "horizon:terminate"]
        timeout: 60

  queue-default:
    enabled: true
    command: ["php", "artisan", "queue:work", "--tries=3"]
    scale: 3
```

## Feature Summary

All features are fully implemented and production-ready:

- **Dependencies**: DAG-based topological sort with cycle detection
- **Health Checks**: TCP, HTTP, and exec health checks with success thresholds
- **API & TUI**: REST API (Unix socket + TCP) with k9s-style TUI
- **Scaling**: Runtime scaling via TUI and API
- **Metrics**: Prometheus metrics server with resource monitoring
- **Tracing**: OpenTelemetry distributed tracing (OTLP gRPC)
- **Hooks**: Pre/post start/stop lifecycle hooks
- **Validation**: Comprehensive config validation with `check-config` command
- **Scaffolding**: Preset generators for Laravel, Symfony, and generic apps
- **Dev Mode**: File watching with auto-reload for development

## When Adding Features

1. **Define interfaces first** - Create testable contracts
2. **Add config structs** - Update `internal/config/types.go`
3. **Add validation** - Update `config.Validate()`
4. **Add defaults** - Update `config.SetDefaults()`
5. **Write tests** - Aim for >80% coverage
6. **Update examples** - Add to `configs/examples/`
7. **Document in IMPLEMENT.md** - Follow the implementation patterns there