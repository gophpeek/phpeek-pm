---
title: "Environment Variables"
description: "Override configuration with environment variables for flexible deployment across environments"
weight: 17
---

# Environment Variables

Override any YAML configuration using environment variables for flexible, environment-specific deployments.

## Overview

Environment variables enable:
- ✅ **Configuration without file changes:** Adjust settings per environment
- ✅ **Secret management:** Keep sensitive values out of version control
- ✅ **Container orchestration:** Easy Kubernetes/Docker configuration
- ✅ **CI/CD integration:** Dynamic configuration in pipelines
- ✅ **12-Factor compliance:** Externalized configuration

## Priority Order

Configuration is loaded in this order (later overrides earlier):

1. **Default values** - Built-in defaults
2. **YAML configuration file** - `phpeek-pm.yaml`
3. **Environment variables** - Runtime overrides

```bash
# YAML has log_level: info
# ENV overrides to debug
PHPEEK_PM_GLOBAL_LOG_LEVEL=debug ./phpeek-pm
```

## Naming Convention

### Global Settings

**Pattern:** `PHPEEK_PM_GLOBAL_<SETTING_NAME>`

```bash
PHPEEK_PM_GLOBAL_SHUTDOWN_TIMEOUT=60
PHPEEK_PM_GLOBAL_LOG_LEVEL=debug
PHPEEK_PM_GLOBAL_LOG_FORMAT=json
PHPEEK_PM_GLOBAL_METRICS_ENABLED=true
PHPEEK_PM_GLOBAL_METRICS_PORT=9090
PHPEEK_PM_GLOBAL_API_ENABLED=true
PHPEEK_PM_GLOBAL_API_PORT=8080
```

### Process-Specific Settings

**Pattern:** `PHPEEK_PM_PROCESS_<PROCESS_NAME>_<SETTING_NAME>`

```bash
PHPEEK_PM_PROCESS_NGINX_ENABLED=true
PHPEEK_PM_PROCESS_NGINX_PRIORITY=20
PHPEEK_PM_PROCESS_QUEUE_DEFAULT_SCALE=5
PHPEEK_PM_PROCESS_HORIZON_RESTART=always
```

**Important:** Process names are converted to uppercase and hyphens to underscores.

| Process Name | Environment Prefix |
|--------------|-------------------|
| `nginx` | `PHPEEK_PM_PROCESS_NGINX_` |
| `queue-default` | `PHPEEK_PM_PROCESS_QUEUE_DEFAULT_` |
| `php-fpm` | `PHPEEK_PM_PROCESS_PHP_FPM_` |

### PHP-FPM Auto-Tuning

**Pattern:** `PHP_FPM_AUTOTUNE_PROFILE`

```bash
PHP_FPM_AUTOTUNE_PROFILE=medium
PHP_FPM_AUTOTUNE_PROFILE=heavy
```

See [PHP-FPM Auto-Tuning](../php-fpm-autotune) for complete guide.

## Global Settings Reference

### Shutdown Configuration

```bash
# Shutdown timeout (seconds)
PHPEEK_PM_GLOBAL_SHUTDOWN_TIMEOUT=60
```

### Logging Configuration

```bash
# Log format (json|text)
PHPEEK_PM_GLOBAL_LOG_FORMAT=json

# Log level (debug|info|warn|error)
PHPEEK_PM_GLOBAL_LOG_LEVEL=info

# Multiline logging
PHPEEK_PM_GLOBAL_LOG_MULTILINE_ENABLED=true
PHPEEK_PM_GLOBAL_LOG_MULTILINE_TIMEOUT=500

# Log redaction
PHPEEK_PM_GLOBAL_LOG_REDACTION_ENABLED=true
```

### Metrics Configuration

```bash
# Enable Prometheus metrics
PHPEEK_PM_GLOBAL_METRICS_ENABLED=true

# Metrics HTTP port
PHPEEK_PM_GLOBAL_METRICS_PORT=9090

# Metrics URL path
PHPEEK_PM_GLOBAL_METRICS_PATH=/metrics
```

### API Configuration

```bash
# Enable Management API
PHPEEK_PM_GLOBAL_API_ENABLED=true

# API HTTP port
PHPEEK_PM_GLOBAL_API_PORT=8080

# API authentication token
PHPEEK_PM_GLOBAL_API_AUTH=your-secure-token-here
```

## Process Settings Reference

### Basic Process Settings

```bash
# Enable/disable process
PHPEEK_PM_PROCESS_<NAME>_ENABLED=true

# Command (JSON array)
PHPEEK_PM_PROCESS_<NAME>_COMMAND='["php-fpm","-F","-R"]'

# Priority (startup order)
PHPEEK_PM_PROCESS_<NAME>_PRIORITY=10

# Restart policy (always|on-failure|never)
PHPEEK_PM_PROCESS_<NAME>_RESTART=always

# Scale (number of instances)
PHPEEK_PM_PROCESS_<NAME>_SCALE=3

# Working directory
PHPEEK_PM_PROCESS_<NAME>_WORKING_DIR=/var/www/html
```

### Process Environment Variables

```bash
# Set environment variable for process
PHPEEK_PM_PROCESS_<NAME>_ENV_<VAR_NAME>=value

# Examples:
PHPEEK_PM_PROCESS_QUEUE_ENV_QUEUE_CONNECTION=redis
PHPEEK_PM_PROCESS_QUEUE_ENV_REDIS_HOST=localhost
PHPEEK_PM_PROCESS_APP_ENV_DEBUG=true
```

## Complete Examples

### Docker Compose

```yaml
version: '3.8'

services:
  app:
    image: myapp:latest
    environment:
      # Global settings
      PHPEEK_PM_GLOBAL_LOG_LEVEL: "info"
      PHPEEK_PM_GLOBAL_METRICS_ENABLED: "true"
      PHPEEK_PM_GLOBAL_API_ENABLED: "true"

      # PHP-FPM auto-tuning
      PHP_FPM_AUTOTUNE_PROFILE: "medium"

      # Process-specific
      PHPEEK_PM_PROCESS_QUEUE_DEFAULT_SCALE: "5"
      PHPEEK_PM_PROCESS_HORIZON_ENABLED: "true"

    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2'
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: laravel-app
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        image: myapp:v1.2.3
        env:
          # Global config
          - name: PHPEEK_PM_GLOBAL_LOG_FORMAT
            value: "json"

          - name: PHPEEK_PM_GLOBAL_METRICS_ENABLED
            value: "true"

          # PHP-FPM auto-tuning from ConfigMap
          - name: PHP_FPM_AUTOTUNE_PROFILE
            valueFrom:
              configMapKeyRef:
                name: phpeek-config
                key: php_fpm_profile

          # API token from Secret
          - name: PHPEEK_PM_GLOBAL_API_AUTH
            valueFrom:
              secretKeyRef:
                name: phpeek-secrets
                key: api-token

          # Process scaling
          - name: PHPEEK_PM_PROCESS_QUEUE_DEFAULT_SCALE
            value: "5"

        resources:
          limits:
            memory: "2Gi"
            cpu: "2"
```

### Dockerfile

```dockerfile
FROM php:8.3-fpm-alpine

# Copy application
COPY . /var/www/html

# Copy phpeek-pm
COPY --from=builder /app/phpeek-pm /usr/local/bin/phpeek-pm

# Default environment variables
ENV PHPEEK_PM_GLOBAL_LOG_FORMAT=json \
    PHPEEK_PM_GLOBAL_LOG_LEVEL=info \
    PHP_FPM_AUTOTUNE_PROFILE=medium

# Run as PID 1
ENTRYPOINT ["/usr/local/bin/phpeek-pm"]
```

### Shell Script

```bash
#!/bin/bash

# Production environment
export PHPEEK_PM_GLOBAL_LOG_LEVEL=info
export PHPEEK_PM_GLOBAL_METRICS_ENABLED=true
export PHPEEK_PM_GLOBAL_API_ENABLED=true
export PHPEEK_PM_GLOBAL_API_AUTH=$(cat /secrets/api-token)

# PHP-FPM configuration
export PHP_FPM_AUTOTUNE_PROFILE=heavy

# Process configuration
export PHPEEK_PM_PROCESS_QUEUE_DEFAULT_SCALE=10
export PHPEEK_PM_PROCESS_HORIZON_ENABLED=true

# Run phpeek-pm
exec /usr/local/bin/phpeek-pm
```

## Environment-Specific Patterns

### Development

```bash
# development.env
PHPEEK_PM_GLOBAL_LOG_LEVEL=debug
PHPEEK_PM_GLOBAL_LOG_FORMAT=text  # Human-readable
PHPEEK_PM_GLOBAL_METRICS_ENABLED=false
PHP_FPM_AUTOTUNE_PROFILE=dev
PHPEEK_PM_PROCESS_QUEUE_DEFAULT_SCALE=1
```

```bash
# Run with development settings
set -a; source development.env; set +a
./phpeek-pm
```

### Staging

```bash
# staging.env
PHPEEK_PM_GLOBAL_LOG_LEVEL=info
PHPEEK_PM_GLOBAL_LOG_FORMAT=json
PHPEEK_PM_GLOBAL_METRICS_ENABLED=true
PHP_FPM_AUTOTUNE_PROFILE=medium
PHPEEK_PM_PROCESS_QUEUE_DEFAULT_SCALE=3
```

### Production

```bash
# production.env
PHPEEK_PM_GLOBAL_LOG_LEVEL=warn
PHPEEK_PM_GLOBAL_LOG_FORMAT=json
PHPEEK_PM_GLOBAL_LOG_REDACTION_ENABLED=true
PHPEEK_PM_GLOBAL_METRICS_ENABLED=true
PHPEEK_PM_GLOBAL_API_ENABLED=true
PHPEEK_PM_GLOBAL_API_AUTH=$(vault read -field=token secret/phpeek-api)
PHP_FPM_AUTOTUNE_PROFILE=heavy
PHPEEK_PM_PROCESS_QUEUE_DEFAULT_SCALE=10
```

## Secret Management

### HashiCorp Vault

```bash
#!/bin/bash
# Load secrets from Vault

export PHPEEK_PM_GLOBAL_API_AUTH=$(vault kv get -field=api_token secret/phpeek)
export DATABASE_PASSWORD=$(vault kv get -field=password secret/database)

exec /usr/local/bin/phpeek-pm
```

### AWS Secrets Manager

```bash
#!/bin/bash
# Load secrets from AWS Secrets Manager

export PHPEEK_PM_GLOBAL_API_AUTH=$(aws secretsmanager get-secret-value \
  --secret-id phpeek-api-token \
  --query SecretString \
  --output text)

exec /usr/local/bin/phpeek-pm
```

### Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: phpeek-secrets
type: Opaque
data:
  api-token: <base64-encoded-token>
---
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        env:
          - name: PHPEEK_PM_GLOBAL_API_AUTH
            valueFrom:
              secretKeyRef:
                name: phpeek-secrets
                key: api-token
```

## Verification

### Check Active Configuration

```bash
# Start with verbose logging
PHPEEK_PM_GLOBAL_LOG_LEVEL=debug ./phpeek-pm

# Check which values are being used (in logs)
# Look for "Configuration loaded" messages
```

### Validate Environment Variables

```bash
#!/bin/bash
# validate-env.sh

required_vars=(
    "PHPEEK_PM_GLOBAL_LOG_LEVEL"
    "PHP_FPM_AUTOTUNE_PROFILE"
    "PHPEEK_PM_GLOBAL_API_AUTH"
)

for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo "ERROR: Required variable $var is not set"
        exit 1
    fi
done

echo "All required variables are set"
```

## Troubleshooting

### Environment Variable Not Working

**Check variable name format:**
```bash
# ❌ Wrong
PHPEEK_PM_process_nginx_enabled=true

# ✅ Correct
PHPEEK_PM_PROCESS_NGINX_ENABLED=true
```

**Verify it's exported:**
```bash
# Check if variable is exported
env | grep PHPEEK_PM

# Export if needed
export PHPEEK_PM_GLOBAL_LOG_LEVEL=debug
```

### Complex Values (JSON Arrays)

```bash
# Process command as JSON array
PHPEEK_PM_PROCESS_APP_COMMAND='["./my-app","--port=8080","--host=0.0.0.0"]'

# Escape quotes properly in shell
PHPEEK_PM_PROCESS_APP_COMMAND="[\"./my-app\",\"--port=8080\"]"
```

### Boolean Values

```bash
# All these are treated as true
PHPEEK_PM_GLOBAL_METRICS_ENABLED=true
PHPEEK_PM_GLOBAL_METRICS_ENABLED=1
PHPEEK_PM_GLOBAL_METRICS_ENABLED=yes

# All these are treated as false
PHPEEK_PM_GLOBAL_METRICS_ENABLED=false
PHPEEK_PM_GLOBAL_METRICS_ENABLED=0
PHPEEK_PM_GLOBAL_METRICS_ENABLED=no
```

## See Also

- [Global Settings](global-settings) - Global configuration reference
- [Process Configuration](processes) - Process settings
- [Docker Integration](../getting-started/docker-integration) - Container patterns
- [Examples](../examples/kubernetes) - Kubernetes deployment examples
