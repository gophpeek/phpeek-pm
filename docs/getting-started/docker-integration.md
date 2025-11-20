---
title: "Docker Integration"
description: "Use PHPeek PM as PID 1 in Docker containers for proper signal handling"
weight: 3
---

# Docker Integration

PHPeek PM is designed to run as PID 1 in Docker containers, providing proper init system capabilities.

## Why PHPeek PM as PID 1?

**PID 1 Responsibilities**
- Signal forwarding to child processes
- Zombie process reaping
- Clean shutdown coordination
- Process tree management

**What PHPeek PM Provides**
- Proper SIGTERM/SIGINT/SIGQUIT handling
- Automatic zombie reaping
- Graceful shutdown with timeouts
- Multi-process lifecycle management

## Basic Docker Integration

### Single-Stage Dockerfile

```dockerfile
FROM php:8.3-fpm-alpine

# Install system dependencies
RUN apk add --no-cache nginx

# Install PHPeek PM
COPY --from=gophpeek/phpeek-pm:latest \
    /usr/local/bin/phpeek-pm \
    /usr/local/bin/phpeek-pm

# Copy configuration
COPY phpeek-pm.yaml /etc/phpeek-pm/phpeek-pm.yaml

# Copy application
COPY . /var/www/html
WORKDIR /var/www/html

# PHPeek PM as PID 1
ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

### Multi-Stage Build

```dockerfile
# Build stage
FROM php:8.3-fpm-alpine AS builder

WORKDIR /app
COPY . .

# Install dependencies
RUN composer install --no-dev --optimize-autoloader

# Runtime stage
FROM php:8.3-fpm-alpine

# Install runtime dependencies
RUN apk add --no-cache nginx

# Copy PHPeek PM
COPY --from=gophpeek/phpeek-pm:latest \
    /usr/local/bin/phpeek-pm \
    /usr/local/bin/phpeek-pm

# Copy application from builder
COPY --from=builder /app /var/www/html
WORKDIR /var/www/html

# Copy configuration
COPY docker/phpeek-pm.yaml /etc/phpeek-pm/phpeek-pm.yaml
COPY docker/nginx.conf /etc/nginx/nginx.conf

# PHPeek PM as entrypoint
ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

## Laravel-Specific Dockerfile

```dockerfile
FROM php:8.3-fpm-alpine

# Install system packages
RUN apk add --no-cache \
    nginx \
    supervisor \
    mysql-client \
    postgresql-client

# Install PHP extensions
RUN docker-php-ext-install \
    pdo_mysql \
    pdo_pgsql \
    pcntl \
    bcmath

# Install Composer
COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

# Copy PHPeek PM
COPY --from=gophpeek/phpeek-pm:latest \
    /usr/local/bin/phpeek-pm \
    /usr/local/bin/phpeek-pm

# Application setup
COPY . /var/www/html
WORKDIR /var/www/html

# Install dependencies
RUN composer install --no-dev --optimize-autoloader

# Laravel optimization
RUN php artisan config:cache && \
    php artisan route:cache && \
    php artisan view:cache

# Copy configurations
COPY docker/phpeek-pm.yaml /etc/phpeek-pm/phpeek-pm.yaml
COPY docker/nginx.conf /etc/nginx/nginx.conf

# Set permissions
RUN chown -R www-data:www-data /var/www/html/storage /var/www/html/bootstrap/cache

ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

## Configuration File Location

PHPeek PM looks for configuration in this order:

1. `PHPEEK_PM_CONFIG` environment variable
2. `/etc/phpeek-pm/phpeek-pm.yaml`
3. `./phpeek-pm.yaml` (current directory)

### Environment Variable Approach

```dockerfile
FROM php:8.3-fpm-alpine

COPY --from=gophpeek/phpeek-pm:latest \
    /usr/local/bin/phpeek-pm \
    /usr/local/bin/phpeek-pm

# Copy config to custom location
COPY phpeek-pm.yaml /app/config/phpeek-pm.yaml

# Set config path via environment
ENV PHPEEK_PM_CONFIG=/app/config/phpeek-pm.yaml

ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

### Standard Location Approach

```dockerfile
FROM php:8.3-fpm-alpine

COPY --from=gophpeek/phpeek-pm:latest \
    /usr/local/bin/phpeek-pm \
    /usr/local/bin/phpeek-pm

# Copy to standard location
COPY phpeek-pm.yaml /etc/phpeek-pm/phpeek-pm.yaml

# No ENV needed - uses default location
ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

## Docker Compose Integration

### Basic Setup

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:80"
    environment:
      - APP_ENV=production
      - DB_HOST=db
    depends_on:
      - db
    restart: unless-stopped

  db:
    image: mysql:8
    environment:
      MYSQL_DATABASE: laravel
      MYSQL_ROOT_PASSWORD: secret
    volumes:
      - db-data:/var/lib/mysql

volumes:
  db-data:
```

### With Observability

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:80"
      - "9090:9090"  # Prometheus metrics
      - "8081:8080"  # Management API
    environment:
      - PHPEEK_PM_GLOBAL_METRICS_ENABLED=true
      - PHPEEK_PM_GLOBAL_API_ENABLED=true
      - PHPEEK_PM_GLOBAL_API_AUTH=${API_TOKEN}
    restart: unless-stopped

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana-data:/var/lib/grafana

volumes:
  prometheus-data:
  grafana-data:
```

## Environment Variable Configuration

Override any configuration with environment variables:

```bash
docker run -d \
  -e PHPEEK_PM_GLOBAL_LOG_LEVEL=debug \
  -e PHPEEK_PM_GLOBAL_SHUTDOWN_TIMEOUT=60 \
  -e PHPEEK_PM_PROCESS_NGINX_ENABLED=true \
  -e PHPEEK_PM_PROCESS_QUEUE_DEFAULT_SCALE=5 \
  my-app
```

### Environment Variable Format

```
PHPEEK_PM_{SECTION}_{SUBSECTION}_{KEY}

Examples:
PHPEEK_PM_GLOBAL_LOG_LEVEL          → global.log_level
PHPEEK_PM_GLOBAL_METRICS_ENABLED    → global.metrics_enabled
PHPEEK_PM_PROCESS_NGINX_ENABLED     → processes.nginx.enabled
PHPEEK_PM_PROCESS_QUEUE_SCALE       → processes.queue.scale
```

## Signal Handling

PHPeek PM handles signals appropriately as PID 1:

```bash
# Graceful shutdown
docker stop my-app
# Sends SIGTERM → PHPeek PM gracefully shuts down processes

# Force shutdown
docker kill my-app
# Sends SIGKILL → Immediate termination

# Custom signal
docker kill -s SIGQUIT my-app
# PHPeek PM initiates graceful shutdown
```

## Health Check Integration

Use Docker health checks with PHPeek PM's management API:

```dockerfile
FROM php:8.3-fpm-alpine

# ... setup ...

HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=3 \
    CMD wget -q -O- http://localhost:8080/api/v1/health || exit 1

ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

Or check processes directly:

```dockerfile
HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=3 \
    CMD pgrep -f "php-fpm: master" && pgrep -f "nginx: master" || exit 1
```

## Resource Limits

Configure resource limits in Docker:

```yaml
services:
  app:
    build: .
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

## Security Considerations

### Run as Non-Root User

```dockerfile
FROM php:8.3-fpm-alpine

# Create user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

# ... setup ...

# Switch to non-root user
USER app

ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

### Read-Only Root Filesystem

```yaml
services:
  app:
    build: .
    read_only: true
    tmpfs:
      - /tmp
      - /var/run
      - /var/log/nginx
```

## Debugging

### View PHPeek PM Logs

```bash
# Follow logs
docker logs -f my-app

# Filter for PHPeek PM messages
docker logs my-app 2>&1 | grep "PHPeek PM"

# Filter for specific process
docker logs my-app 2>&1 | grep "process=nginx"
```

### Inspect Running Processes

```bash
# List all processes
docker exec my-app ps aux

# Check process tree
docker exec my-app pstree -p 1

# View PHPeek PM status via API
curl http://localhost:8080/api/v1/processes
```

### Debug Mode

```yaml
global:
  log_level: debug  # Enable debug logging
  log_format: text  # Human-readable logs
```

## Production Best Practices

1. **Always use specific version tags**
   ```dockerfile
   COPY --from=gophpeek/phpeek-pm:1.0.0 \
       /usr/local/bin/phpeek-pm \
       /usr/local/bin/phpeek-pm
   ```

2. **Enable health checks**
   - Use Docker HEALTHCHECK or
   - Configure PHPeek PM health checks

3. **Configure graceful shutdown**
   ```yaml
   global:
     shutdown_timeout: 60  # Adjust based on workload
   ```

4. **Enable observability**
   ```yaml
   global:
     metrics_enabled: true
     api_enabled: true
   ```

5. **Use read-only filesystem when possible**

6. **Set resource limits**

7. **Run as non-root user**

## Next Steps

- [Configuration Overview](../configuration/overview) - Complete configuration reference
- [Health Checks](../features/health-checks) - Configure health monitoring
- [Prometheus Metrics](../observability/metrics) - Monitor with Prometheus
- [Examples](../examples/laravel-complete) - Real-world Dockerfiles
