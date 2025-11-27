---
title: "Examples"
description: "Real-world PHPeek PM configurations for Laravel, Docker, and Kubernetes deployments"
weight: 30
---

# Examples

Practical, production-ready configuration examples for common use cases.

## Available Examples

- [Minimal Setup](minimal) - Simple PHP-FPM configuration
- [Laravel Complete](laravel-complete) - Full Laravel stack with Horizon and queues
- [Laravel with Monitoring](laravel-with-monitoring) - Laravel + Prometheus + API
- [Scheduled Tasks](scheduled-tasks) - Cron scheduler setup
- [Kubernetes Deployment](kubernetes) - K8s deployment patterns
- [Docker Compose](docker-compose) - Multi-container orchestration

## Quick Start

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

### Laravel with Queue Workers

```yaml
version: "1.0"

processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]

  nginx:
    enabled: true
    command: ["nginx", "-g", "daemon off;"]
    depends_on: [php-fpm]

  queue-default:
    enabled: true
    command: ["php", "artisan", "queue:work"]
    scale: 3
```

## See Also

- [Quickstart Guide](../getting-started/quickstart) - Step-by-step tutorial
- [Configuration Reference](../configuration/overview) - All options explained
