# PHPeek Process Manager

🚀 Production-grade process manager for Docker containers with Laravel-first design.

## Features

- **PID 1 Process Manager**: Proper signal handling and zombie process reaping
- **Multi-Process Orchestration**: Manage PHP-FPM, Nginx, Horizon, Reverb, and workers
- **Structured Logging**: JSON output with per-process segmentation
- **Lifecycle Hooks**: Pre/post start/stop customization for Laravel optimization
- **Dynamic Scaling**: Runtime adjustment of worker counts (coming in Phase 5)
- **Health Monitoring**: TCP, HTTP, and exec-based health checks (coming in Phase 3)
- **Prometheus Metrics**: Process health and resource usage (coming in Phase 4)
- **Management API**: REST API for process control (coming in Phase 5)
- **Graceful Shutdown**: Configurable timeouts and proper cleanup

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

## Roadmap

- [x] **Phase 1** (Week 1): Core foundation with single process support
- [ ] **Phase 2** (Week 2): Multi-process orchestration with dependencies
- [ ] **Phase 3** (Week 3): Health checks and lifecycle hooks
- [ ] **Phase 4** (Week 4): Dynamic scaling and Prometheus metrics
- [ ] **Phase 5** (Week 5): Management API for runtime control
- [ ] **Phase 6** (Week 6): Testing, documentation, production readiness

## Contributing

PHPeek PM is part of the PHPeek ecosystem. For bugs and feature requests, please open an issue.

## License

MIT License - see LICENSE file for details

## Credits

Built with ❤️ by the PHPeek team as a modern alternative to s6-overlay and supervisord for PHP applications in Docker.
