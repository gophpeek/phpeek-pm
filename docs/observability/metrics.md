---
title: "Prometheus Metrics"
description: "Monitor PHPeek PM processes with comprehensive Prometheus metrics and alerting"
weight: 10
---

# Prometheus Metrics

Comprehensive Prometheus metrics for monitoring PHPeek PM and managed processes.

## Configuration

Enable metrics in your `phpeek-pm.yaml`:

```yaml
global:
  metrics_enabled: true
  metrics_port: 9090
  metrics_path: /metrics
```

## Available Metrics

### Process Lifecycle Metrics

#### `phpeek_pm_process_up`
**Type:** Gauge
**Labels:** `name`, `instance`
**Description:** Process status (1=running, 0=stopped)

```promql
# Query running instances of php-fpm
phpeek_pm_process_up{name="php-fpm"}
```

#### `phpeek_pm_process_restarts_total`
**Type:** Counter
**Labels:** `name`, `reason`
**Description:** Total number of process restarts by reason (crash, health_check, normal_exit)

```promql
# Total restarts for all processes
sum(phpeek_pm_process_restarts_total) by (name)

# Restarts due to health check failures
phpeek_pm_process_restarts_total{reason="health_check"}
```

#### `phpeek_pm_process_start_time_seconds`
**Type:** Gauge
**Labels:** `name`, `instance`
**Description:** Unix timestamp when process instance started

```promql
# Process uptime in seconds
time() - phpeek_pm_process_start_time_seconds
```

#### `phpeek_pm_process_last_exit_code`
**Type:** Gauge
**Labels:** `name`, `instance`
**Description:** Last exit code of process instance

```promql
# Non-zero exit codes (errors)
phpeek_pm_process_last_exit_code != 0
```

### Health Check Metrics

#### `phpeek_pm_health_check_status`
**Type:** Gauge
**Labels:** `name`, `type`
**Description:** Health check status (1=healthy, 0=unhealthy)

```promql
# Unhealthy processes
phpeek_pm_health_check_status == 0
```

#### `phpeek_pm_health_check_duration_seconds`
**Type:** Histogram
**Labels:** `name`, `type`
**Description:** Health check duration in seconds

```promql
# 95th percentile health check latency
histogram_quantile(0.95,
  sum(rate(phpeek_pm_health_check_duration_seconds_bucket[5m])) by (le, name)
)
```

#### `phpeek_pm_health_check_total`
**Type:** Counter
**Labels:** `name`, `type`, `status`
**Description:** Total number of health checks performed

```promql
# Health check failure rate
rate(phpeek_pm_health_check_total{status="failure"}[5m])
```

#### `phpeek_pm_health_check_consecutive_fails`
**Type:** Gauge
**Labels:** `name`
**Description:** Current consecutive health check failures

```promql
# Processes with multiple consecutive failures
phpeek_pm_health_check_consecutive_fails > 1
```

### Scaling Metrics

#### `phpeek_pm_process_desired_scale`
**Type:** Gauge
**Labels:** `name`
**Description:** Desired number of process instances

```promql
# Desired scale configuration
phpeek_pm_process_desired_scale
```

#### `phpeek_pm_process_current_scale`
**Type:** Gauge
**Labels:** `name`
**Description:** Current number of running instances

```promql
# Scale drift (actual vs desired)
phpeek_pm_process_current_scale - phpeek_pm_process_desired_scale
```

### Hook Execution Metrics

#### `phpeek_pm_hook_executions_total`
**Type:** Counter
**Labels:** `name`, `type`, `status`
**Description:** Total hook executions by type and status

```promql
# Failed pre-start hooks
phpeek_pm_hook_executions_total{type="pre_start", status="failure"}
```

#### `phpeek_pm_hook_duration_seconds`
**Type:** Histogram
**Labels:** `name`, `type`
**Description:** Hook execution duration in seconds

```promql
# 99th percentile hook duration
histogram_quantile(0.99,
  sum(rate(phpeek_pm_hook_duration_seconds_bucket[5m])) by (le, type)
)
```

### Manager Metrics

#### `phpeek_pm_manager_process_count`
**Type:** Gauge
**Description:** Total number of managed processes

```promql
# Total processes under management
phpeek_pm_manager_process_count
```

#### `phpeek_pm_manager_start_time_seconds`
**Type:** Gauge
**Description:** Unix timestamp when manager started

```promql
# Manager uptime in seconds
time() - phpeek_pm_manager_start_time_seconds
```

#### `phpeek_pm_build_info`
**Type:** Gauge
**Labels:** `version`, `go_version`
**Description:** PHPeek PM build information

```promql
# Version information
phpeek_pm_build_info
```

## Common Queries

### Process Health Overview

```promql
# Count of healthy processes
sum(phpeek_pm_process_up) by (name)

# Count of processes with health check failures
count(phpeek_pm_health_check_status{status="0"}) by (name)
```

### Restart Monitoring

```promql
# Restart rate per minute
rate(phpeek_pm_process_restarts_total[1m])

# Processes restarting frequently (>5/hour)
sum(increase(phpeek_pm_process_restarts_total[1h])) by (name) > 5
```

### Scale Monitoring

```promql
# Instances not matching desired scale
abs(phpeek_pm_process_current_scale - phpeek_pm_process_desired_scale) > 0
```

### Hook Performance

```promql
# Slow hooks (>30s)
max(phpeek_pm_hook_duration_seconds) by (name, type) > 30

# Hook failure rate
rate(phpeek_pm_hook_executions_total{status="failure"}[5m])
```

## Alerting Rules

### Recommended Prometheus Alerts

```yaml
groups:
  - name: phpeek_pm
    rules:
      # Process down
      - alert: ProcessDown
        expr: phpeek_pm_process_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Process {{ $labels.name }} instance {{ $labels.instance }} is down"

      # Frequent restarts
      - alert: FrequentRestarts
        expr: rate(phpeek_pm_process_restarts_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Process {{ $labels.name }} restarting frequently"

      # Health check failures
      - alert: HealthCheckFailing
        expr: phpeek_pm_health_check_status == 0
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Health check failing for {{ $labels.name }}"

      # Scale drift
      - alert: ScaleDrift
        expr: abs(phpeek_pm_process_current_scale - phpeek_pm_process_desired_scale) > 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "{{ $labels.name }} scale drift detected"

      # Hook failures
      - alert: HookFailures
        expr: rate(phpeek_pm_hook_executions_total{status="failure"}[5m]) > 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Hook {{ $labels.name }} failing"
```

## Grafana Dashboard

### Sample Dashboard JSON

```json
{
  "dashboard": {
    "title": "PHPeek PM Overview",
    "panels": [
      {
        "title": "Process Status",
        "targets": [
          {
            "expr": "phpeek_pm_process_up"
          }
        ]
      },
      {
        "title": "Restart Rate",
        "targets": [
          {
            "expr": "rate(phpeek_pm_process_restarts_total[5m])"
          }
        ]
      },
      {
        "title": "Health Check Status",
        "targets": [
          {
            "expr": "phpeek_pm_health_check_status"
          }
        ]
      },
      {
        "title": "Scale Status",
        "targets": [
          {
            "expr": "phpeek_pm_process_current_scale",
            "legendFormat": "Current"
          },
          {
            "expr": "phpeek_pm_process_desired_scale",
            "legendFormat": "Desired"
          }
        ]
      }
    ]
  }
}
```

## Scraping Configuration

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'phpeek-pm'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
```

### Docker Compose Integration

```yaml
services:
  phpeek-pm:
    image: gophpeek/phpeek-pm:latest
    environment:
      - PHPEEK_PM_GLOBAL_METRICS_ENABLED=true
      - PHPEEK_PM_GLOBAL_METRICS_PORT=9090
    ports:
      - "9090:9090"

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9091:9090"
```
