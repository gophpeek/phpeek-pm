---
title: "Features"
description: "Comprehensive guide to PHPeek PM features including health checks, scaling, and monitoring"
weight: 20
---

# Features

PHPeek PM provides production-grade process management features designed specifically for Laravel applications in Docker containers.

## Core Features

- [Dependency Management](dependency-management) - DAG-based startup ordering
- [Health Checks](health-checks) - TCP, HTTP, and exec-based monitoring
- [Scheduled Tasks](scheduled-tasks) - Built-in cron scheduler
- [Process Scaling](process-scaling) - Multi-instance worker management
- [Restart Policies](restart-policies) - Always, on-failure, never strategies
- [Advanced Logging](advanced-logging) - Multiline, redaction, JSON parsing
- [Heartbeat Monitoring](heartbeat-monitoring) - External monitoring integration

## Quick Overview

### Health Monitoring

```yaml
processes:
  nginx:
    enabled: true
    command: ["nginx", "-g", "daemon off;"]
    health_check:
      type: tcp
      address: "127.0.0.1:80"
      interval: 10
      timeout: 5
      retries: 3
```

### Scheduled Tasks

```yaml
processes:
  backup-job:
    enabled: true
    command: ["php", "artisan", "backup:run"]
    schedule: "0 2 * * *"  # Daily at 2 AM
```

## See Also

- [Configuration Overview](../configuration/overview) - All configuration options
- [Examples](../examples/laravel-complete) - Real-world configurations
