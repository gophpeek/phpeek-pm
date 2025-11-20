---
title: "Introduction"
description: "Production-grade PID 1 process manager for Docker containers with Laravel-first design"
weight: 1
---

# PHPeek Process Manager

PHPeek PM is a production-grade process manager designed specifically for running PHP applications in Docker containers. Built in Go, it serves as PID 1, providing proper signal handling, zombie reaping, and graceful shutdown for multi-process orchestration.

## What is PHPeek PM?

PHPeek PM manages multiple processes within a single Docker container, making it ideal for Laravel applications that need to run:

- PHP-FPM for web requests
- Nginx as a reverse proxy
- Laravel Horizon for queue management
- Laravel Reverb for WebSocket connections
- Multiple queue workers with different priorities
- Laravel Scheduler for cron jobs

## Why PHPeek PM?

**PID 1 Capability**
- Proper signal handling (SIGTERM, SIGINT, SIGQUIT)
- Zombie process reaping
- Clean process tree management

**Multi-Process Orchestration**
- DAG-based dependency management
- Priority-based startup ordering
- Graceful shutdown with configurable timeouts
- Process scaling (run multiple instances)

**Health Monitoring**
- TCP, HTTP, and exec health checks
- Success thresholds to prevent restart flapping
- Automatic restart policies (always, on-failure, never)

**Production Ready**
- Prometheus metrics for monitoring
- REST API for runtime control
- Structured JSON logging
- Framework detection (Laravel, Symfony, WordPress)

## Who Should Use PHPeek PM?

**Perfect For**
- Laravel applications needing multiple services in one container
- Production deployments requiring observability
- Teams wanting simplified Docker orchestration
- Applications with queue workers and scheduled tasks

**Not Ideal For**
- Simple single-process applications
- Kubernetes environments (use native pod patterns instead)
- Development environments (use Docker Compose for multiple containers)

## Architecture Overview

```
┌─────────────────────────────────────────┐
│         PHPeek PM (PID 1)              │
│                                         │
│  ┌──────────┐  ┌──────────┐           │
│  │ Metrics  │  │   API    │           │
│  │ :9090    │  │  :8080   │           │
│  └──────────┘  └──────────┘           │
│                                         │
│  ┌─────────────────────────────────┐  │
│  │     Process Manager             │  │
│  │  • Startup ordering             │  │
│  │  • Health monitoring            │  │
│  │  • Graceful shutdown            │  │
│  └─────────────────────────────────┘  │
│                                         │
│  ┌──────────┐ ┌──────────┐ ┌────────┐│
│  │ PHP-FPM  │ │  Nginx   │ │Horizon ││
│  │ (scale 2)│ │          │ │        ││
│  └──────────┘ └──────────┘ └────────┘│
│                                         │
│  ┌──────────┐ ┌──────────┐           │
│  │ Queue    │ │Scheduler │           │
│  │(scale 3) │ │          │           │
│  └──────────┘ └──────────┘           │
└─────────────────────────────────────────┘
```

## Key Features

**Process Management**
- Multi-process orchestration with dependency resolution
- Process scaling (run N instances of the same process)
- Restart policies with exponential backoff
- Lifecycle hooks (pre/post start/stop)

**Health Monitoring**
- TCP port checks
- HTTP endpoint validation
- Custom exec commands
- Configurable failure/success thresholds

**Observability**
- Prometheus metrics endpoint
- Process lifecycle metrics
- Health check duration tracking
- Hook execution metrics
- REST API for runtime inspection

**Framework Integration**
- Auto-detect Laravel, Symfony, WordPress
- Framework-specific permission setup
- Configuration validation
- Laravel Artisan command support

## Quick Example

```yaml
version: "1.0"

global:
  shutdown_timeout: 30
  log_level: info
  metrics_enabled: true
  api_enabled: true

processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    priority: 10
    restart: always
    health_check:
      type: tcp
      address: "127.0.0.1:9000"

  nginx:
    enabled: true
    command: ["nginx", "-g", "daemon off;"]
    priority: 20
    depends_on: [php-fpm]
    restart: always

  horizon:
    enabled: true
    command: ["php", "artisan", "horizon"]
    priority: 30
    depends_on: [php-fpm]
    shutdown:
      pre_stop_hook:
        command: ["php", "artisan", "horizon:terminate"]
        timeout: 60
```

## Next Steps

- [Installation](getting-started/installation) - Get PHPeek PM installed
- [Quick Start](getting-started/quickstart) - 5-minute getting started guide
- [Configuration](configuration/overview) - Complete configuration reference
- [Examples](examples/laravel-complete) - Real-world configuration examples

## Community

- [GitHub Repository](https://github.com/gophpeek/phpeek-pm)
- [Issue Tracker](https://github.com/gophpeek/phpeek-pm/issues)
- [Discussions](https://github.com/gophpeek/phpeek-pm/discussions)
