---
title: "Scheduled Tasks"
description: "Configure cron-like scheduled tasks with heartbeat monitoring and per-task statistics"
weight: 33
---

# Scheduled Tasks Example

Run periodic tasks like backups, reports, and maintenance jobs using PHPeek PM's built-in cron scheduler.

## Use Cases

- ✅ Replace cron in Docker containers
- ✅ Database backups and maintenance
- ✅ Report generation and data sync
- ✅ Cache warming and optimization
- ✅ Cleanup and housekeeping tasks
- ✅ External monitoring with heartbeats

## Cron Schedule Format

PHPeek PM uses standard 5-field cron expressions:

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, 0=Sunday)
│ │ │ │ │
* * * * *
```

**Special Characters:**
- `*` - Any value
- `,` - Value list separator (e.g., `1,3,5`)
- `-` - Range (e.g., `1-5`)
- `/` - Step (e.g., `*/15`)

## Common Schedule Patterns

```yaml
# Every minute
schedule: "* * * * *"

# Every 15 minutes
schedule: "*/15 * * * *"

# Every hour at :30
schedule: "30 * * * *"

# Daily at 2 AM
schedule: "0 2 * * *"

# Every Monday at 8 AM
schedule: "0 8 * * 1"

# First day of month at midnight
schedule: "0 0 1 * *"

# Business hours (9 AM - 5 PM, Mon-Fri)
schedule: "0 9-17 * * 1-5"

# Every 6 hours
schedule: "0 */6 * * *"

# Twice daily (6 AM and 6 PM)
schedule: "0 6,18 * * *"

# Weekend only (Sat-Sun at 3 AM)
schedule: "0 3 * * 6,0"
```

## Complete Configuration

```yaml
version: "1.0"

global:
  log_format: json
  log_level: info
  metrics_enabled: true
  metrics_port: 9090

processes:
  # Database backup - Daily at 2 AM
  database-backup:
    enabled: true
    command: ["php", "artisan", "backup:database"]
    schedule: "0 2 * * *"
    restart: never
    env:
      BACKUP_PATH: /backups
      RETENTION_DAYS: "30"
    heartbeat:
      success_url: https://hc-ping.com/backup-uuid
      failure_url: https://hc-ping.com/backup-uuid/fail
      timeout: 30

  # Cache warming - Every 15 minutes
  cache-warmer:
    enabled: true
    command: ["php", "artisan", "cache:warm"]
    schedule: "*/15 * * * *"
    restart: never

  # Reports - Hourly during business hours
  hourly-reports:
    enabled: true
    command: ["php", "artisan", "reports:generate"]
    schedule: "0 9-17 * * 1-5"  # 9 AM - 5 PM, Mon-Fri
    restart: never
    env:
      REPORT_TYPE: hourly
      OUTPUT_DIR: /var/www/storage/reports
    heartbeat:
      success_url: https://hc-ping.com/reports-uuid
      timeout: 60

  # Log rotation - Daily at midnight
  log-rotation:
    enabled: true
    command: ["/usr/local/bin/rotate-logs.sh"]
    schedule: "0 0 * * *"
    restart: never
    env:
      LOG_DIR: /var/log/app
      COMPRESS: "true"
      MAX_AGE_DAYS: "7"

  # Data sync - Every 30 minutes
  data-sync:
    enabled: true
    command: ["php", "artisan", "data:sync"]
    schedule: "*/30 * * * *"
    restart: never
    env:
      SYNC_SOURCE: remote-api
      SYNC_BATCH_SIZE: "100"
    heartbeat:
      success_url: https://hc-ping.com/sync-uuid
      retry_count: 5
      retry_delay: 10

  # Weekly maintenance - Sunday at 3 AM
  weekly-maintenance:
    enabled: true
    command: ["/usr/local/bin/weekly-maintenance.sh"]
    schedule: "0 3 * * 0"  # Sunday
    restart: never
    env:
      OPTIMIZE_DATABASE: "true"
      CLEAR_OLD_SESSIONS: "true"
    heartbeat:
      success_url: https://hc-ping.com/weekly-uuid
      timeout: 300
```

## Scheduled Task Breakdown

### 1. Database Backup

```yaml
database-backup:
  command: ["php", "artisan", "backup:database"]
  schedule: "0 2 * * *"  # Daily at 2 AM
  restart: never
  heartbeat:
    success_url: https://hc-ping.com/backup-uuid
    failure_url: https://hc-ping.com/backup-uuid/fail
```

**Schedule:** Runs at 2 AM every day

**Heartbeat integration:**
- Pings success URL when backup completes
- Pings failure URL if backup fails
- External monitoring alerts if ping doesn't arrive

**Laravel Command:**
```php
// app/Console/Commands/BackupDatabase.php
class BackupDatabase extends Command
{
    protected $signature = 'backup:database';

    public function handle()
    {
        $filename = 'backup-' . date('Y-m-d') . '.sql';
        $path = env('BACKUP_PATH', '/backups');

        DB::beginTransaction();
        // ... backup logic
        DB::commit();

        $this->info("Backup created: {$path}/{$filename}");
        return 0;  // Success
    }
}
```

### 2. Cache Warming

```yaml
cache-warmer:
  command: ["php", "artisan", "cache:warm"]
  schedule: "*/15 * * * *"  # Every 15 minutes
```

**Schedule:** Runs every 15 minutes (00, 15, 30, 45)

**Why frequent caching:**
- Keep application responsive
- Prevent cache stampede
- Pre-populate expensive queries

### 3. Hourly Reports

```yaml
hourly-reports:
  schedule: "0 9-17 * * 1-5"  # 9 AM - 5 PM, Mon-Fri
```

**Schedule:** Every hour from 9 AM to 5 PM, Monday through Friday

**Breakdown:**
- `0` - At minute 0 (top of hour)
- `9-17` - Hours 9 through 17 (9 AM - 5 PM)
- `*` - Every day of month
- `*` - Every month
- `1-5` - Monday through Friday

### 4. Weekly Maintenance

```yaml
weekly-maintenance:
  schedule: "0 3 * * 0"  # Sunday at 3 AM
```

**Schedule:** Every Sunday at 3:00 AM

**Typical tasks:**
```bash
#!/bin/bash
# weekly-maintenance.sh
set -e

echo "Starting weekly maintenance..."

# Optimize database
php artisan db:optimize

# Clear old sessions
php artisan session:clear --old

# Vacuum analyze (if PostgreSQL)
psql -c "VACUUM ANALYZE;"

# Clear expired cache
php artisan cache:prune

echo "Weekly maintenance complete"
```

## Heartbeat Monitoring

### healthchecks.io Integration

```yaml
database-backup:
  schedule: "0 2 * * *"
  heartbeat:
    success_url: https://hc-ping.com/your-uuid-here
    failure_url: https://hc-ping.com/your-uuid-here/fail
    timeout: 30
```

**How it works:**
1. **Task starts:** Pings `/start` endpoint (optional)
2. **Task succeeds:** Pings main URL (exit code 0)
3. **Task fails:** Pings `/fail` endpoint with exit code

**Get notified if:**
- Task doesn't run on schedule
- Task fails (non-zero exit)
- Task takes too long (timeout)

### Setup healthchecks.io

1. Create check at https://healthchecks.io
2. Get ping URL: `https://hc-ping.com/your-uuid-here`
3. Add to heartbeat configuration
4. Configure alerting (email, Slack, PagerDuty)

### Other Monitoring Services

**Cronitor:**
```yaml
heartbeat:
  success_url: https://cronitor.link/p/your-key/job-name
  failure_url: https://cronitor.link/p/your-key/job-name/fail
```

**Better Uptime:**
```yaml
heartbeat:
  success_url: https://betteruptime.com/api/v1/heartbeat/your-uuid
```

**Custom Endpoint:**
```yaml
heartbeat:
  success_url: https://your-monitoring.com/ping/backup
  method: POST
  headers:
    Authorization: Bearer your-token
    X-Task-Name: database-backup
```

## Task Statistics

PHPeek PM tracks execution statistics for each scheduled task:

### Via Prometheus Metrics

```bash
# Check metrics
curl http://localhost:9090/metrics | grep scheduled_task

# Available metrics:
# - phpeek_pm_scheduled_task_last_run_timestamp
# - phpeek_pm_scheduled_task_next_run_timestamp
# - phpeek_pm_scheduled_task_last_exit_code
# - phpeek_pm_scheduled_task_duration_seconds
# - phpeek_pm_scheduled_task_total{status="success"}
# - phpeek_pm_scheduled_task_total{status="failure"}
```

### Via Management API

```bash
# Get task status
curl http://localhost:8080/api/v1/processes | jq '.[] | select(.scheduled==true)'

# Example response:
{
  "name": "database-backup",
  "scheduled": true,
  "schedule": "0 2 * * *",
  "last_run": "2024-11-21T02:00:00Z",
  "next_run": "2024-11-22T02:00:00Z",
  "last_exit_code": 0,
  "run_count": 42,
  "success_count": 41,
  "failure_count": 1
}
```

## Environment Variables

Scheduled tasks receive additional environment variables:

```bash
PHPEEK_PM_PROCESS_NAME=database-backup
PHPEEK_PM_INSTANCE_ID=database-backup-run-123
PHPEEK_PM_SCHEDULED=true
PHPEEK_PM_SCHEDULE="0 2 * * *"
PHPEEK_PM_START_TIME=1732162800
```

**Use in scripts:**
```bash
#!/bin/bash
echo "Task: $PHPEEK_PM_PROCESS_NAME"
echo "Run ID: $PHPEEK_PM_INSTANCE_ID"
echo "Started at: $(date -d @$PHPEEK_PM_START_TIME)"
```

## Docker Compose Integration

```yaml
version: '3.8'

services:
  scheduler:
    build: .
    environment:
      # Enable specific tasks
      PHPEEK_PM_PROCESS_DATABASE_BACKUP_ENABLED: "true"
      PHPEEK_PM_PROCESS_WEEKLY_MAINTENANCE_ENABLED: "false"

      # Configure heartbeats
      BACKUP_HEARTBEAT_URL: "https://hc-ping.com/${BACKUP_UUID}"
      REPORTS_HEARTBEAT_URL: "https://hc-ping.com/${REPORTS_UUID}"

    volumes:
      - ./backups:/backups
      - ./reports:/var/www/storage/reports
```

## Kubernetes CronJob Alternative

**Traditional Kubernetes CronJob:**
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: myapp:latest
            command: ["php", "artisan", "backup:database"]
          restartPolicy: Never
```

**PHPeek PM Alternative:**
```yaml
# Single long-running pod with multiple scheduled tasks
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scheduled-tasks
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: scheduler
        image: myapp:latest
        env:
          - name: PHPEEK_PM_CONFIG
            value: /etc/phpeek/scheduled-tasks.yaml
```

**Benefits:**
- Single pod for all scheduled tasks
- Shared resources (database connections, cache)
- Centralized logging and monitoring
- Easier secret management

## Best Practices

### ✅ Do

**Idempotent Tasks:**
```bash
# Safe to run multiple times
php artisan cache:clear  # ✅ Idempotent
php artisan backup:create  # ✅ Creates new backup each time
```

**Error Handling:**
```bash
#!/bin/bash
set -e  # Exit on error

# Validate environment
if [ -z "$BACKUP_PATH" ]; then
    echo "ERROR: BACKUP_PATH not set"
    exit 1
fi

# Execute with error handling
php artisan backup:run || {
    echo "Backup failed, sending alert..."
    curl -X POST https://alerts.example.com/backup-failed
    exit 1
}

echo "Backup completed successfully"
exit 0
```

**Timeout Safety:**
```yaml
database-backup:
  schedule: "0 2 * * *"
  timeout: 600  # 10 minutes max
```

**Heartbeat Monitoring:**
```yaml
critical-backup:
  schedule: "0 3 * * *"
  heartbeat:
    success_url: https://hc-ping.com/uuid  # Get alerted if job doesn't run
```

### ❌ Don't

**Don't assume other processes are running:**
```bash
# ❌ Bad - assumes database is ready
php artisan backup:database

# ✅ Good - wait for database
until php artisan db:ping; do sleep 1; done
php artisan backup:database
```

**Don't use long-running tasks:**
```yaml
# ❌ Bad - daemon processes don't work with schedule
schedule: "* * * * *"
command: ["./background-daemon"]  # Won't work, runs every minute

# ✅ Good - one-time execution
command: ["./process-batch-then-exit"]
```

**Don't forget restart: never:**
```yaml
# ❌ Bad - will restart and run again immediately
scheduled-task:
  schedule: "0 2 * * *"
  restart: always  # Don't do this!

# ✅ Good - runs once per schedule
scheduled-task:
  schedule: "0 2 * * *"
  restart: never
```

## Advanced Patterns

### Conditional Execution

```bash
#!/bin/bash
# Only run in production
if [ "$APP_ENV" != "production" ]; then
    echo "Skipping in non-production environment"
    exit 0
fi

# Only run on specific day
if [ "$(date +%u)" -eq 1 ]; then  # Monday
    echo "Running weekly report..."
    php artisan reports:weekly
fi
```

### Chain Multiple Tasks

```yaml
multi-step-task:
  command: ["/usr/local/bin/multi-step-maintenance.sh"]
  schedule: "0 3 * * 0"  # Sunday 3 AM
```

**multi-step-maintenance.sh:**
```bash
#!/bin/bash
set -e

echo "Step 1: Database backup"
php artisan backup:database

echo "Step 2: Optimize database"
php artisan db:optimize

echo "Step 3: Clear old logs"
php artisan log:clear --days=30

echo "Step 4: Generate weekly report"
php artisan reports:weekly

echo "All steps completed successfully"
```

### Retry Logic

```bash
#!/bin/bash
MAX_RETRIES=3
RETRY_DELAY=60

for i in $(seq 1 $MAX_RETRIES); do
    if php artisan sync:external-api; then
        echo "Sync successful on attempt $i"
        exit 0
    fi

    if [ $i -lt $MAX_RETRIES ]; then
        echo "Sync failed, retrying in ${RETRY_DELAY}s..."
        sleep $RETRY_DELAY
    fi
done

echo "Sync failed after $MAX_RETRIES attempts"
exit 1
```

```yaml
data-sync:
  command: ["/usr/local/bin/sync-with-retry.sh"]
  schedule: "*/30 * * * *"
  heartbeat:
    failure_url: https://hc-ping.com/sync-uuid/fail
```

### Parallel Task Execution

```bash
#!/bin/bash
# Run multiple tasks in parallel

# Start background tasks
php artisan cache:warmup &
PID1=$!

php artisan sitemap:generate &
PID2=$!

php artisan images:optimize &
PID3=$!

# Wait for all tasks
wait $PID1 $PID2 $PID3
echo "All parallel tasks completed"
```

## Logging and Monitoring

### Task-Specific Logs

```yaml
database-backup:
  logging:
    stdout: true
    stderr: true
    labels:
      service: backup
      type: database
      schedule: daily
```

**Filter logs:**
```bash
# View only backup logs
docker logs app | jq 'select(.labels.service=="backup")'
```

### Execution Metrics

```bash
# Last run time
curl -s http://localhost:9090/metrics | grep "phpeek_pm_scheduled_task_last_run_timestamp{task=\"database-backup\"}"

# Success rate
curl -s http://localhost:9090/metrics | grep "phpeek_pm_scheduled_task_total{task=\"database-backup\"}"

# Duration
curl -s http://localhost:9090/metrics | grep "phpeek_pm_scheduled_task_duration_seconds{task=\"database-backup\"}"
```

### Alerts

**Prometheus Alert:**
```yaml
groups:
  - name: scheduled_tasks
    rules:
      - alert: ScheduledTaskFailed
        expr: phpeek_pm_scheduled_task_last_exit_code != 0
        for: 5m
        annotations:
          summary: "Scheduled task {{ $labels.task }} failed"

      - alert: ScheduledTaskNotRunning
        expr: time() - phpeek_pm_scheduled_task_last_run_timestamp > 86400
        annotations:
          summary: "Task {{ $labels.task }} hasn't run in 24h"
```

## Troubleshooting

### Task Not Running

**Check schedule syntax:**
```bash
# Test cron expression
# Use crontab.guru or similar tool

# Verify PHPeek PM parsed it correctly
docker logs app | grep "Scheduled task"
```

**Verify task is enabled:**
```yaml
my-task:
  enabled: true  # Must be true
  schedule: "0 2 * * *"
```

### Task Runs Multiple Times

**Problem:** Task configured with `restart: always`

**Solution:**
```yaml
scheduled-task:
  schedule: "0 2 * * *"
  restart: never  # Required for scheduled tasks
```

### Task Timeouts

**Problem:** Task doesn't complete within timeout

**Solution:** Increase timeout or optimize task
```yaml
slow-task:
  command: ["./slow-process.sh"]
  timeout: 1800  # 30 minutes
```

### Heartbeat Not Pinging

**Check heartbeat URL:**
```bash
# Test manually
curl -X POST https://hc-ping.com/your-uuid

# Check response
# Should return: OK
```

**Verify network access:**
```yaml
heartbeat:
  success_url: https://hc-ping.com/uuid
  timeout: 30  # Increase if network is slow
  retry_count: 5  # Retry on failure
```

### Missing Environment Variables

**Problem:** Task can't access required variables

**Solution:**
```yaml
my-task:
  env:
    REQUIRED_VAR: value
    # Or reference from environment
    DATABASE_URL: ${DATABASE_URL}
```

## Real-World Examples

### Laravel Scheduled Tasks

```yaml
# Use Laravel's built-in scheduler
laravel-scheduler:
  command: ["php", "artisan", "schedule:run"]
  schedule: "* * * * *"  # Run every minute
  restart: never
```

**app/Console/Kernel.php:**
```php
protected function schedule(Schedule $schedule)
{
    // Backup database daily
    $schedule->command('backup:run')->daily();

    // Send emails every hour
    $schedule->command('emails:send')->hourly();

    // Clear cache every 6 hours
    $schedule->command('cache:clear')->cron('0 */6 * * *');

    // Generate reports on weekdays
    $schedule->command('reports:daily')
             ->weekdays()
             ->at('08:00');
}
```

### Database Maintenance

```yaml
db-optimize:
  command: ["php", "artisan", "db:optimize"]
  schedule: "0 4 * * 0"  # Sunday 4 AM
  restart: never
  heartbeat:
    success_url: https://hc-ping.com/db-optimize-uuid
  env:
    DB_OPTIMIZE_TABLES: "users,orders,products"
```

### Log Cleanup

```yaml
log-cleanup:
  command: ["/cleanup-logs.sh"]
  schedule: "0 1 * * *"  # Daily 1 AM
  restart: never
```

**cleanup-logs.sh:**
```bash
#!/bin/bash
LOG_DIR="/var/log/app"
MAX_AGE_DAYS=7

# Delete logs older than MAX_AGE_DAYS
find "$LOG_DIR" -name "*.log" -type f -mtime +$MAX_AGE_DAYS -delete

# Compress yesterday's logs
find "$LOG_DIR" -name "*.log" -type f -mtime 1 -exec gzip {} \;

echo "Log cleanup complete"
```

### API Data Sync

```yaml
api-sync:
  command: ["php", "artisan", "sync:external-api"]
  schedule: "*/30 * * * *"  # Every 30 minutes
  restart: never
  env:
    API_URL: https://api.example.com
    API_KEY: ${EXTERNAL_API_KEY}
  heartbeat:
    success_url: https://hc-ping.com/sync-uuid
    failure_url: https://hc-ping.com/sync-uuid/fail
    retry_count: 3
```

## Performance Tips

### Avoid Overlapping Runs

```yaml
long-running-task:
  command: ["/long-task.sh"]
  schedule: "0 * * * *"  # Every hour
  timeout: 3000  # 50 minutes (less than interval)
```

**Ensure:** `timeout < schedule_interval`

### Stagger Multiple Tasks

```yaml
# Don't run all tasks at same time
backup-database:
  schedule: "0 2 * * *"  # 2:00 AM

backup-files:
  schedule: "15 2 * * *"  # 2:15 AM

backup-logs:
  schedule: "30 2 * * *"  # 2:30 AM
```

### Resource-Intensive Tasks

```yaml
heavy-processing:
  schedule: "0 3 * * *"  # Run at low-traffic time
  env:
    # Limit resource usage
    PHP_MEMORY_LIMIT: 512M
    MAX_CONCURRENT_JOBS: "2"
```

## See Also

- [Process Configuration](../configuration/processes) - Process settings
- [Heartbeat Monitoring](../features/heartbeat-monitoring) - External monitoring
- [Prometheus Metrics](../observability/metrics) - Task statistics
- [Management API](../observability/api) - Runtime task inspection
