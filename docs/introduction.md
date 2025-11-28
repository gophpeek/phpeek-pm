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
┌─────────────────────────────────────────────────┐
│         PHPeek PM (PID 1)                      │
│                                                 │
│  ┌──────────┐  ┌──────────┐  ┌─────────────┐ │
│  │ Metrics  │  │   API    │  │  Heartbeat  │ │
│  │ :9090    │  │  :9180   │  │  Monitor    │ │
│  └──────────┘  └──────────┘  └──────┬──────┘ │
│                                       │         │
│                            External monitoring │
│                            (healthchecks.io)   │
│                                                 │
│  ┌──────────────────────────────────────────┐ │
│  │     Process Manager & Orchestration      │ │
│  │  • DAG-based dependency resolution       │ │
│  │  • Health monitoring with thresholds     │ │
│  │  • Lifecycle hooks (pre/post start/stop) │ │
│  │  • Graceful shutdown with timeouts       │ │
│  └──────────────────────────────────────────┘ │
│                                                 │
│  ┌──────────────────────────────────────────┐ │
│  │     Advanced Logging Pipeline            │ │
│  │  Log → Level Detect → Multiline →        │ │
│  │  JSON Parse → Redact → Filter → Output   │ │
│  └──────────────────────────────────────────┘ │
│                                                 │
│  ┌──────────┐ ┌──────────┐ ┌────────┐        │
│  │ PHP-FPM  │ │  Nginx   │ │Horizon │        │
│  │ (scale 2)│ │          │ │        │        │
│  └──────────┘ └──────────┘ └────────┘        │
│                                                 │
│  ┌──────────┐ ┌──────────────────────┐       │
│  │ Queue    │ │   Cron Scheduler     │       │
│  │(scale 3) │ │  • Standard 5-field  │       │
│  │          │ │  • Task statistics   │       │
│  │          │ │  • Heartbeat pings   │       │
│  └──────────┘ └──────────────────────┘       │
└─────────────────────────────────────────────────┘
```

## Key Features

**Process Management**
- Multi-process orchestration with DAG-based dependency resolution
- Process scaling (run N instances of the same process)
- Restart policies with exponential backoff
- Lifecycle hooks (pre/post start/stop)

**Scheduled Tasks**
- Built-in cron scheduler with standard 5-field format
- Per-task execution statistics and metrics
- External heartbeat monitoring integration (healthchecks.io, Cronitor, etc.)
- Graceful cancellation on shutdown

**Health Monitoring**
- TCP port checks
- HTTP endpoint validation with status code verification
- Custom exec commands
- Configurable failure/success thresholds to prevent flapping

**Advanced Logging**
- Automatic log level detection (ERROR, WARN, INFO, DEBUG)
- Multiline log reassembly (stack traces, exceptions)
- JSON log parsing and structured field extraction
- Sensitive data redaction (passwords, tokens, PII)
- GDPR, PCI DSS, HIPAA, SOC 2 compliance support

**Observability**
- Prometheus metrics endpoint
- Process lifecycle metrics (start, stop, restart, exit codes)
- Health check duration and status tracking
- Hook execution metrics
- Scheduled task metrics (last run, next run, duration, success/failure)
- REST API for runtime inspection and control

**Framework Integration**
- Auto-detect Laravel, Symfony, WordPress
- Framework-specific permission setup
- Configuration validation
- Laravel Artisan command support (Horizon, queue workers, scheduler)

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
    restart: always
    health_check:
      type: tcp
      address: "127.0.0.1:9000"

  nginx:
    enabled: true
    command: ["nginx", "-g", "daemon off;"]
    depends_on: [php-fpm]
    restart: always

  horizon:
    enabled: true
    command: ["php", "artisan", "horizon"]
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
