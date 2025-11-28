# PHPeek Process Manager

Production-grade PID 1 process manager for Docker containers with Laravel-first design.

## Features

- **PID 1 Process Manager** - Proper signal handling and zombie process reaping
- **Multi-Process Orchestration** - Manage PHP-FPM, Nginx, Horizon, Reverb, and workers
- **PHP-FPM Auto-Tuning** - Intelligent worker calculation based on container limits
- **Dependency Management** - DAG-based process startup ordering
- **Health Monitoring** - TCP, HTTP, and exec-based health checks
- **Scheduled Tasks** - Built-in cron-like scheduler
- **Prometheus Metrics** - Comprehensive process and health metrics
- **Management API & TUI** - REST API and terminal UI for process control
- **Distributed Tracing** - OpenTelemetry support
- **Graceful Shutdown** - Configurable timeouts and proper cleanup

## Quick Start

```bash
# Build
make build

# Run (looks for phpeek-pm.yaml in current directory)
./build/phpeek-pm
```

### Minimal Configuration

```yaml
version: "1.0"

global:
  shutdown_timeout: 30
  log_format: json

processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    restart: always
```

### Docker Usage

```dockerfile
FROM php:8.3-fpm-alpine

COPY --from=builder /app/phpeek-pm /usr/local/bin/phpeek-pm
COPY phpeek-pm.yaml /etc/phpeek-pm/phpeek-pm.yaml

ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

## Development

```bash
make dev        # Build and run locally
make test       # Run tests with coverage
make build-all  # Build for all platforms
```

## Documentation

Full documentation available in [`/docs`](docs/):

- [Introduction](docs/introduction.md) - Overview and architecture
- [Quick Start](docs/getting-started/quickstart.md) - 5-minute tutorial
- [Configuration](docs/configuration/overview.md) - Complete configuration reference
- [Features](docs/features/) - Health checks, scaling, hooks, scheduled tasks
- [Observability](docs/observability/) - Metrics, API, tracing, resource monitoring

Example configurations in [`configs/examples/`](configs/examples/).

## License

MIT License - see LICENSE file for details.
