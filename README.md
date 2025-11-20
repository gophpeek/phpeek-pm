# PHPeek Process Manager

🚀 Production-grade process manager for Docker containers with Laravel-first design.

## Features

- ✅ **PID 1 Process Manager**: Proper signal handling and zombie process reaping
- ✅ **Multi-Process Orchestration**: Manage PHP-FPM, Nginx, Horizon, Reverb, and workers
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
