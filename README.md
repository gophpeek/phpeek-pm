# PHPeek Process Manager

🚀 Production-grade process manager for Docker containers with Laravel-first design.

## Features

- ✅ **PID 1 Process Manager**: Proper signal handling and zombie process reaping
- ✅ **Multi-Process Orchestration**: Manage PHP-FPM, Nginx, Horizon, Reverb, and workers
- ✅ **PHP-FPM Auto-Tuning**: Intelligent worker calculation based on container limits
- ✅ **Dependency Management**: DAG-based process startup ordering with topological sort
- ✅ **Structured Logging**: JSON output with per-process segmentation
- ✅ **Lifecycle Hooks**: Pre/post start/stop customization for Laravel optimization
- ✅ **Health Monitoring**: TCP, HTTP, and exec-based health checks with success thresholds
- ✅ **Restart Policies**: Always, on-failure, never with exponential backoff
- ✅ **Prometheus Metrics**: Comprehensive process health, restarts, and hook metrics
- ✅ **Management API**: REST API for process control and monitoring
- ✅ **Graceful Shutdown**: Configurable timeouts and proper cleanup
- ✅ **Framework Detection**: Auto-detect Laravel, Symfony, WordPress, or generic PHP
- ✅ **Container Ready**: Zero-bash initialization, handles all setup in Go

## Quick Start

### Build

```bash
make build
```

### Run

```bash
./build/phpeek-pm
```

By default, looks for `phpeek-pm.yaml` in the current directory or `/etc/phpeek-pm/phpeek-pm.yaml`.

### Configuration

Create `phpeek-pm.yaml`:

```yaml
version: "1.0"

global:
  shutdown_timeout: 30
  log_format: json
  log_level: info

processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    priority: 10
    restart: always
```

See `configs/examples/` for more examples:
- `minimal.yaml` - Simple PHP-FPM setup
- `laravel-full.yaml` - Complete Laravel stack with Horizon, Reverb, workers
- `laravel-with-monitoring.yaml` - Full observability with metrics and API enabled
- `scheduled-tasks.yaml` - Cron-like scheduled task execution

### Environment Variables

Override any configuration with environment variables:

```bash
# Global settings
PHPEEK_PM_GLOBAL_LOG_LEVEL=debug
PHPEEK_PM_GLOBAL_SHUTDOWN_TIMEOUT=60

# Process-specific
PHPEEK_PM_PROCESS_NGINX_ENABLED=true
PHPEEK_PM_PROCESS_QUEUE_DEFAULT_SCALE=5
```

## PHP-FPM Auto-Tuning

PHPeek PM automatically calculates optimal PHP-FPM worker settings based on container resource limits (memory/CPU) detected via cgroups v1/v2.

### Quick Start

```bash
# Via CLI flag
./build/phpeek-pm --php-fpm-profile=medium

# Via environment variable (recommended for Docker)
docker run -e PHP_FPM_AUTOTUNE_PROFILE=medium myapp:latest
```

### Application Profiles

| Profile | Use Case | Memory/Worker* | Traffic Load |
|---------|----------|----------------|--------------|
| `dev` | Development | ~32MB + 64MB OPcache | N/A |
| `light` | Small apps | ~36MB + 96MB OPcache | 1-10 req/s |
| `medium` | Standard prod | ~42MB + 128MB OPcache | 10-50 req/s |
| `heavy` | High traffic | ~52MB + 256MB OPcache | 50-200 req/s |
| `bursty` | Traffic spikes | ~44MB + 128MB OPcache | Variable |

*OPcache is shared memory (not per-worker), reducing RAM usage significantly

### How It Works

1. Detects container limits from cgroup v1/v2 (memory + CPU quota)
2. Calculates optimal `pm.max_children` based on available memory
3. Sets `pm.start_servers`, `pm.min_spare_servers`, `pm.max_spare_servers` ratios
4. Exports environment variables for PHP-FPM pool configuration

### PHP-FPM Integration

In your `www.conf`:

```ini
[www]
pm = ${PHP_FPM_PM}
pm.max_children = ${PHP_FPM_MAX_CHILDREN}
pm.start_servers = ${PHP_FPM_START_SERVERS}
pm.min_spare_servers = ${PHP_FPM_MIN_SPARE}
pm.max_spare_servers = ${PHP_FPM_MAX_SPARE}
pm.max_requests = ${PHP_FPM_MAX_REQUESTS}
```

### Docker Compose Example

```yaml
services:
  app:
    image: myapp:latest
    environment:
      - PHP_FPM_AUTOTUNE_PROFILE=medium
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2'
```

See [docs/PHP-FPM-AUTOTUNE.md](docs/PHP-FPM-AUTOTUNE.md) for complete guide including safety features, calculation algorithm, and troubleshooting.

## Laravel Integration

PHPeek PM is designed for Laravel applications with built-in support for:

- **Pre-start hooks**: `config:cache`, `route:cache`, `view:cache`, `migrate`, `storage:link`
- **Horizon management**: Graceful termination with `horizon:terminate` hook
- **Reverb support**: WebSocket server management
- **Queue workers**: Scalable queue:work processes with graceful shutdown

Example Laravel configuration:

```yaml
hooks:
  pre-start:
    - name: migrate
      command: ["php", "artisan", "migrate", "--force"]
      timeout: 300

processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    priority: 10

  nginx:
    enabled: true
    command: ["nginx", "-g", "daemon off;"]
    priority: 20
    depends_on: [php-fpm]

  horizon:
    enabled: true
    command: ["php", "artisan", "horizon"]
    priority: 30
    shutdown:
      pre_stop_hook:
        command: ["php", "artisan", "horizon:terminate"]
        timeout: 60

  queue-default:
    enabled: true
    command: ["php", "artisan", "queue:work", "--tries=3"]
    scale: 3
    priority: 40
```

## Scheduled Tasks

PHPeek PM includes a built-in cron-like scheduler for running periodic tasks without requiring a separate cron daemon. Perfect for Laravel scheduled commands, backups, cleanups, and maintenance tasks.

### Configuration

Use standard 5-field cron expressions (minute, hour, day, month, weekday):

```yaml
processes:
  backup-job:
    enabled: true
    command: ["php", "artisan", "backup:run"]
    schedule: "0 2 * * *"  # Daily at 2 AM
    env:
      BACKUP_DESTINATION: "s3"

  cache-warmup:
    enabled: true
    command: ["php", "artisan", "cache:warmup"]
    schedule: "*/15 * * * *"  # Every 15 minutes

  report-generator:
    enabled: true
    command: ["php", "artisan", "reports:generate"]
    schedule: "0 8 * * 1"  # Mondays at 8 AM
```

### Features

- **Standard Cron Format**: Familiar 5-field syntax (minute hour day month weekday)
- **Per-Task Statistics**: Track run count, success/failure rates, execution duration
- **Heartbeat Integration**: External monitoring support (healthchecks.io, etc.)
- **Structured Logging**: Task-specific logs with execution context
- **Graceful Shutdown**: Running tasks are cancelled cleanly on shutdown

### Environment Variables

Scheduled tasks receive additional environment variables:

```bash
PHPEEK_PM_PROCESS_NAME=backup-job
PHPEEK_PM_INSTANCE_ID=backup-job-run-42
PHPEEK_PM_SCHEDULED=true
PHPEEK_PM_SCHEDULE="0 2 * * *"
PHPEEK_PM_START_TIME=1732141200
```

### Metrics

Scheduled task metrics available via Prometheus:

- `phpeek_pm_scheduled_task_last_run_timestamp` - Last execution time
- `phpeek_pm_scheduled_task_next_run_timestamp` - Next scheduled time
- `phpeek_pm_scheduled_task_last_exit_code` - Most recent exit code
- `phpeek_pm_scheduled_task_duration_seconds` - Execution duration
- `phpeek_pm_scheduled_task_total` - Total runs by status (success/failure)

## External Monitoring (Heartbeat)

Integrate with external monitoring services like [healthchecks.io](https://healthchecks.io), [Cronitor](https://cronitor.io), or [Better Uptime](https://betteruptime.com) for dead man's switch monitoring.

### Configuration

```yaml
processes:
  critical-backup:
    enabled: true
    command: ["php", "artisan", "backup:critical"]
    schedule: "0 3 * * *"
    heartbeat:
      url: "https://hc-ping.com/your-uuid-here"
      timeout: 10  # HTTP request timeout in seconds
```

### How It Works

1. **Task Start**: Pings `/start` endpoint when task begins
2. **Task Success**: Pings main URL when task completes with exit code 0
3. **Task Failure**: Pings `/fail` endpoint with exit code when task fails

### Supported Services

Works with any service that supports HTTP ping URLs:

- **healthchecks.io**: `https://hc-ping.com/uuid`
- **Cronitor**: `https://cronitor.link/p/key/job-name`
- **Better Uptime**: `https://betteruptime.com/api/v1/heartbeat/uuid`
- **Custom endpoints**: Any URL accepting GET/POST requests

## Advanced Logging

PHPeek PM provides enterprise-grade log processing with automatic level detection, multiline handling, JSON parsing, and sensitive data redaction.

### Features

#### 1. Automatic Log Level Detection

Automatically detects log levels from various formats:

```
[ERROR] Database connection failed      → ERROR
2024-11-20 ERROR: Query timeout         → ERROR
{"level":"warn","msg":"Slow query"}     → WARN
php artisan: INFO - Cache cleared       → INFO
```

Supports: ERROR, WARN/WARNING, INFO, DEBUG, TRACE, FATAL, CRITICAL

#### 2. Multiline Log Handling

Automatically reassembles stack traces and multi-line error messages:

```
[ERROR] Exception in Controller
    at App\Http\Controllers\UserController->store()
    at Illuminate\Routing\Controller->callAction()
    at Illuminate\Routing\ControllerDispatcher->dispatch()
```

**Configuration:**

```yaml
global:
  log_multiline_enabled: true
  log_multiline_pattern: '^\[|^\d{4}-|^{"'  # Regex for line starts
  log_multiline_timeout: 500  # milliseconds
  log_multiline_max_lines: 100
```

#### 3. JSON Log Parsing

Parses JSON logs and extracts structured fields:

```json
{"level":"error","msg":"Query failed","query":"SELECT *","duration":5000}
```

Becomes:
```
ERROR [query_failed] Query failed (duration: 5000ms, query: SELECT *)
```

#### 4. Sensitive Data Redaction

Automatically redacts sensitive information to prevent credential leaks:

**Redacted Patterns:**
- Passwords: `password`, `passwd`, `pwd`
- API tokens: `token`, `api_key`, `secret`, `auth`
- Connection strings: `mysql://`, `postgres://`, database URLs
- Credit cards: Card number patterns
- Email addresses: (optional, disabled by default)

**Configuration:**

```yaml
global:
  log_redaction_enabled: true
  log_redaction_patterns:
    - "password"
    - "api_key"
    - "secret"
    - "token"
  log_redaction_placeholder: "***REDACTED***"
```

**Example:**

```
Before: {"password":"secret123","api_key":"sk_live_abc"}
After:  {"password":"***REDACTED***","api_key":"***REDACTED***"}
```

#### 5. Log Filtering

Filter logs by level, pattern, or process:

```yaml
global:
  log_filter_enabled: true
  log_filter_level: "info"  # Only INFO and above
  log_filter_patterns:
    - "health_check"  # Exclude health check logs
    - "metrics_export"  # Exclude metrics logs
```

### Compliance Support

Advanced logging features support:
- **GDPR**: Automatic PII redaction
- **PCI DSS**: Credit card number masking
- **HIPAA**: PHI data protection patterns
- **SOC 2**: Audit trail with structured logs

## Docker Integration

### As PID 1

```dockerfile
FROM php:8.3-fpm-alpine

# Copy phpeek-pm binary
COPY --from=builder /app/phpeek-pm /usr/local/bin/phpeek-pm

# Copy configuration
COPY phpeek-pm.yaml /etc/phpeek-pm/phpeek-pm.yaml

# Run as PID 1
ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

## Development

### Build for Development

```bash
make dev
```

### Run Tests

```bash
make test
```

### Build for All Platforms

```bash
make build-all
```

Produces binaries for:
- Linux AMD64
- Linux ARM64
- macOS AMD64
- macOS ARM64

## Observability

### Prometheus Metrics

PHPeek PM exports comprehensive metrics on port 9090 (configurable):

```yaml
global:
  metrics_enabled: true
  metrics_port: 9090
  metrics_path: /metrics
```

**Available metrics:**
- Process lifecycle (up/down, restarts, exit codes)
- Health check status and duration
- Hook execution time and success rate
- Scale drift (actual vs desired instances)
- Manager uptime and process count

See [docs/observability/metrics.md](docs/observability/metrics.md) for complete metric documentation.

### Management API

REST API for runtime process control on port 8080 (configurable):

```yaml
global:
  api_enabled: true
  api_port: 8080
  api_auth: "your-secure-token"  # Optional Bearer token
```

**Endpoints:**
- `GET /api/v1/health` - API health check
- `GET /api/v1/processes` - List all processes with status
- `POST /api/v1/processes/{name}/restart` - Restart process
- `POST /api/v1/processes/{name}/stop` - Stop process
- `POST /api/v1/processes/{name}/start` - Start process
- `POST /api/v1/processes/{name}/scale` - Scale process instances

See [docs/observability/api.md](docs/observability/api.md) for complete API documentation.

## Documentation

**Getting Started**
- [Introduction](docs/introduction.md) - Overview and architecture
- [Installation](docs/getting-started/installation.md) - Download and install
- [Quick Start](docs/getting-started/quickstart.md) - 5-minute tutorial
- [Docker Integration](docs/getting-started/docker-integration.md) - Use as PID 1

**Configuration**
- [PHP-FPM Auto-Tuning](docs/PHP-FPM-AUTOTUNE.md) - Intelligent worker calculation guide

**Observability**
- [Prometheus Metrics](docs/observability/metrics.md) - Complete metrics reference
- [Management API](docs/observability/api.md) - REST API documentation

## Roadmap

- [x] **Phase 1**: Core foundation with single process support
- [x] **Phase 1.5**: Container integration with framework detection
- [x] **Phase 2**: Multi-process orchestration with DAG dependencies
- [x] **Phase 3**: Health checks with success thresholds and lifecycle hooks
- [x] **Phase 4**: Prometheus metrics with comprehensive observability
- [x] **Phase 5**: Management API for runtime control
- [ ] **Phase 6**: Advanced scaling, resource limits, production hardening

## Contributing

PHPeek PM is part of the PHPeek ecosystem. For bugs and feature requests, please open an issue.

## License

MIT License - see LICENSE file for details

## Credits

Built with ❤️ by the PHPeek team as a modern alternative to s6-overlay and supervisord for PHP applications in Docker.
