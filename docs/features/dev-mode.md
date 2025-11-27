---
title: "Development Mode"
description: "Auto-reload configuration changes with file watching and graceful restart for faster development iteration"
weight: 28
---

# Development Mode

PHPeek PM includes a development mode with configuration file watching and automatic reload capabilities. When config changes are detected, PHPeek gracefully reloads, making it easier to iterate on configurations during development.

## Overview

**Features:**
- 📁 **File Watching** - Monitors configuration file using fsnotify
- 🔄 **Auto-Reload** - Automatically reloads when changes detected
- ⏱️ **Debouncing** - Prevents multiple rapid reloads (2-second window)
- ✅ **Validation** - Validates new config before triggering reload
- 🛡️ **Graceful Restart** - Cleanly shuts down existing processes

## Quick Start

```bash
# Enable dev mode with --dev flag
./phpeek-pm serve --dev

# Or with explicit config path
./phpeek-pm serve --config phpeek-pm.yaml --dev

# Dev mode watches the config file and auto-reloads on changes
```

## How It Works

### 1. Startup

PHPeek PM starts normally and initializes all processes with `--dev` flag:

```bash
$ ./phpeek-pm serve --config myconfig.yaml --dev

🚀 PHPeek Process Manager v1.0.0
time=2025-11-23T15:23:38.935+01:00 level=INFO msg="Development mode enabled" watch_config=/path/to/myconfig.yaml
time=2025-11-23T15:23:38.935+01:00 level=INFO msg="Config watcher started" path=/path/to/myconfig.yaml debounce=2s

# PHPeek PM starts all processes normally...
```

### 2. File Watching

Watcher monitors the configuration file for changes:

```bash
# Terminal 1: PHPeek PM running with --dev
```

```bash
# Terminal 2: Edit configuration
$ vim myconfig.yaml
# Make changes (e.g., change scale: 3 → scale: 5)
# Save file (:wq)
```

### 3. Change Detection

When config file is modified and saved, watcher detects the change:

```bash
# Terminal 1: Auto-reload triggered
time=2025-11-23T15:23:58.750+01:00 level=INFO msg="Config file changed, triggering reload" path=/path/to/myconfig.yaml event=WRITE
time=2025-11-23T15:23:58.751+01:00 level=INFO msg="Config reload triggered"
```

### 4. Validation

New configuration is loaded and validated before triggering reload:

```bash
time=2025-11-23T15:23:58.752+01:00 level=INFO msg="Performing config reload"
time=2025-11-23T15:23:58.752+01:00 level=INFO msg="Initiating graceful shutdown" reason="config reload" timeout=30s
```

**If validation fails:**
```bash
time=2025-11-23T15:24:10.123+01:00 level=INFO msg="Config file changed, triggering reload"
time=2025-11-23T15:24:10.124+01:00 level=ERROR msg="Config reload failed" error="invalid config: depends_on contains unknown process: 'nonexistent-process'"
# PHPeek PM continues running with old configuration
```

### 5. Graceful Reload

If validation passes, graceful shutdown is initiated:

```bash
time=2025-11-23T15:23:58.752+01:00 level=INFO msg="Initiating graceful shutdown" reason="config reload" timeout=30s
time=2025-11-23T15:23:59.100+01:00 level=INFO msg="Stopping process" name=horizon
time=2025-11-23T15:23:59.200+01:00 level=INFO msg="Stopping process" name=nginx
time=2025-11-23T15:23:59.300+01:00 level=INFO msg="Stopping process" name=php-fpm
time=2025-11-23T15:24:00.500+01:00 level=INFO msg="All processes stopped successfully"
```

### 6. Exit Message

PHPeek PM exits cleanly with message:

```bash
time=2025-11-23T15:24:00.752+01:00 level=INFO msg="Config reload complete - restart PHPeek PM to apply changes"
```

### 7. Manual Restart

User restarts PHPeek PM to apply new configuration:

```bash
$ ./phpeek-pm serve --config myconfig.yaml --dev
# PHPeek PM starts with new configuration
```

## Configuration

**No special configuration required** - dev mode is enabled via CLI flag only.

```bash
# Enable dev mode
./phpeek-pm serve --dev
```

**Not a config file setting:**
```yaml
# ❌ This does NOT enable dev mode
global:
  dev_mode: true  # Not a valid config option

# ✅ Use --dev flag instead
./phpeek-pm serve --dev
```

## File Watcher Behavior

### Monitored Events

Watcher responds to these filesystem events:
- **WRITE** - File content modified
- **CREATE** - File created (e.g., after delete + save)

### Ignored Events

These events do NOT trigger reload:
- **CHMOD** - Permission changes
- **RENAME** - File renamed (unless part of save strategy)
- **REMOVE** - File deleted

### Debouncing

**2-second debounce period** prevents multiple rapid reloads:

```bash
# Multiple saves within 2 seconds
time=2025-11-23T15:24:20.100+01:00 level=INFO msg="Config file changed, triggering reload"
time=2025-11-23T15:24:20.500+01:00 level=DEBUG msg="Config change debounced" since_last_reload=0.4s
time=2025-11-23T15:24:21.000+01:00 level=DEBUG msg="Config change debounced" since_last_reload=0.9s
# Only one reload is triggered after 2 seconds of stability
```

**Why debouncing?**
- Some editors save files multiple times
- Prevents unnecessary reload storms
- Ensures file is fully written before reload

### Auto-detected Paths

Watcher monitors the configuration file specified via:

1. `--config` flag (explicit path)
2. `PHPEEK_PM_CONFIG` environment variable
3. Auto-detected paths (priority order):
   - `phpeek-pm.yaml` (current directory)
   - `/etc/phpeek-pm/phpeek-pm.yaml` (system-wide)
   - `~/.phpeek/pm/config.yaml` (user-specific)

**Example:**
```bash
# Watches explicit config
./phpeek-pm serve --config custom.yaml --dev
# Watches: custom.yaml

# Watches auto-detected config
./phpeek-pm serve --dev
# Watches: phpeek-pm.yaml (if exists)
```

## Validation Before Reload

**Config validation runs before triggering reload:**

### Valid Config Example

```yaml
# Original config
processes:
  nginx:
    scale: 3

# Edit and save: scale: 3 → scale: 5
processes:
  nginx:
    scale: 5  # Valid change

# Result: Reload triggered ✅
```

### Invalid Config Example

```yaml
# Edit with error
processes:
  nginx:
    scale: abc  # Invalid! Must be integer

# Result: Error logged, reload aborted ❌
time=2025-11-23T15:24:10.124+01:00 level=ERROR msg="Config reload failed" error="invalid config: scale must be integer"
# PHPeek continues with old config
```

### Circular Dependency Example

```yaml
# Edit with circular dependency
processes:
  nginx:
    depends_on: [php-fpm]
  php-fpm:
    depends_on: [nginx]  # Circular!

# Result: Validation fails, reload aborted ❌
time=2025-11-23T15:24:10.124+01:00 level=ERROR msg="Config reload failed" error="circular dependency detected: nginx → php-fpm → nginx"
```

## Development Workflow

### Typical Iteration Cycle

**1. Start PHPeek with dev mode:**
```bash
./phpeek-pm serve --config dev.yaml --dev
```

**2. Edit configuration:**
```bash
# Another terminal
vim dev.yaml
# Change: scale: 3 → scale: 5
# Save (:wq)
```

**3. Watch auto-reload:**
```
time=2025-11-23T15:24:00.750+01:00 level=INFO msg="Config file changed, triggering reload"
time=2025-11-23T15:24:00.752+01:00 level=INFO msg="Config reload triggered"
time=2025-11-23T15:24:00.753+01:00 level=INFO msg="Initiating graceful shutdown"
...
time=2025-11-23T15:24:01.500+01:00 level=INFO msg="Config reload complete - restart PHPeek PM to apply changes"
```

**4. Restart to apply:**
```bash
./phpeek-pm serve --config dev.yaml --dev
# New configuration active
```

**5. Verify changes:**
```bash
# Check process list
curl http://localhost:9180/api/v1/processes
# OR
./phpeek-pm tui
```

**6. Iterate:**
- Repeat steps 2-5 as needed
- Fast feedback loop
- No manual process management

### Use Cases

#### 1. Adjusting Worker Counts

```bash
# Initial config: 3 workers
processes:
  queue-worker:
    scale: 3

# Test with 5 workers
# Edit: scale: 3 → scale: 5
# Save → Auto-reload

# Test with 10 workers
# Edit: scale: 5 → scale: 10
# Save → Auto-reload

# Find optimal count through iteration
```

#### 2. Tuning Health Checks

```bash
# Initial config
health_check:
  interval: 30
  timeout: 5

# Too slow, adjust
# Edit: interval: 30 → interval: 10
# Save → Auto-reload

# Observe health check behavior

# Fine-tune based on observations
# Edit: timeout: 5 → timeout: 3
# Save → Auto-reload
```

#### 3. Testing Log Levels

```bash
# Start with info
global:
  log_level: info

# Hit issue, need debug logs
# Edit: log_level: info → log_level: debug
# Save → Auto-reload (now with debug logging)

# Investigate with detailed logs

# Restore normal level
# Edit: log_level: debug → log_level: info
# Save → Auto-reload
```

#### 4. Experimenting with Dependencies

```bash
# Initial: No dependencies
processes:
  nginx:
    priority: 20
  php-fpm:
    priority: 10

# Add dependency for correct order
# Edit: Add depends_on: [php-fpm] to nginx
# Save → Auto-reload

# Verify startup order
# Check logs for dependency resolution
```

## Limitations

### 1. Manual Restart Required

**PHPeek exits after reload** (does not auto-restart):

```bash
time=2025-11-23T15:24:00.752+01:00 level=INFO msg="Config reload complete - restart PHPeek PM to apply changes"
# Process exits with code 0

# User must restart manually:
./phpeek-pm serve --config config.yaml --dev
```

**Rationale:**
- Clean exit ensures proper cleanup
- User control over restart timing
- Prevents unexpected behavior

**Workaround for auto-restart:**
```bash
# Use systemd with Restart=always
[Unit]
Description=PHPeek PM

[Service]
ExecStart=/usr/local/bin/phpeek-pm serve --dev
Restart=always

# OR use simple loop (not recommended for production)
while true; do
  ./phpeek-pm serve --dev
  sleep 1
done
```

### 2. Full Restart (No Hot Reload)

**All processes are stopped and started** (not individual process reload):

- Cannot reload single process
- All instances restart together
- No zero-downtime reload

**For production:** Use rolling updates with orchestration (Kubernetes, etc.)

### 3. Debounce Delay

**Minimum 2 seconds between reloads:**

- Rapid changes are debounced
- Last change triggers reload after 2s stability
- Cannot force immediate reload

### 4. File System Events

**Relies on OS file system notifications:**

- May not work on some network filesystems (NFS, SMB)
- Rare edge cases with certain editors
- macOS/Linux only (fsnotify limitation)

### 5. Validation-Only Checking

**Validates config syntax, not runtime behavior:**

```yaml
# Syntax valid, but command doesn't exist
processes:
  worker:
    command: ["nonexistent-binary"]  # Won't catch until runtime

# Validation passes ✅
# Reload triggered
# Process fails to start at runtime ❌
```

## Troubleshooting

### Watcher Not Detecting Changes

**Issue:** Config changes not triggering reload

**Solutions:**

1. **Verify absolute path logged:**
   ```bash
   time=2025-11-23T15:23:38.935+01:00 level=INFO msg="Config watcher started" path=/absolute/path/to/config.yaml
   ```

2. **Use explicit config path:**
   ```bash
   ./phpeek-pm serve --config /absolute/path/to/config.yaml --dev
   ```

3. **Check filesystem support:**
   ```bash
   # Test fsnotify on your filesystem
   # Dev mode relies on filesystem event notifications
   ```

4. **Try different editor:**
   ```bash
   # Some editors use atomic saves that may not trigger events
   # Try vim, nano, or direct file write
   ```

### Multiple Reloads on Single Save

**Issue:** Reload triggers multiple times for one save

**Cause:** Editor saves file multiple times

**Solution:**
- Debouncing is automatic (2s window)
- Watch logs for "Config change debounced" messages
- Only one reload happens after stability

**Example:**
```bash
time=2025-11-23T15:24:20.100+01:00 level=INFO msg="Config file changed, triggering reload"
time=2025-11-23T15:24:20.500+01:00 level=DEBUG msg="Config change debounced" since_last_reload=0.4s
# Only one reload after 2 seconds
```

### Reload Fails with Validation Error

**Issue:** Valid-looking config fails validation

**Solutions:**

1. **Check error message:**
   ```bash
   time=2025-11-23T15:24:10.124+01:00 level=ERROR msg="Config reload failed" error="..."
   ```

2. **Validate manually:**
   ```bash
   ./phpeek-pm check-config --config config.yaml --strict
   ```

3. **Common issues:**
   - Circular dependencies
   - Invalid data types
   - Out-of-range values
   - Missing required fields

### Dev Mode in Production

**Issue:** Accidentally using dev mode in production

**❌ Don't do this:**
```bash
# Production container
docker run myapp /usr/local/bin/phpeek-pm serve --dev
```

**✅ Use dev mode only for:**
- Local development
- Testing configurations
- Debugging issues

**Production best practices:**
- Never use `--dev` flag
- Use immutable container images
- Test config changes in staging first

## Best Practices

### 1. Use Dev Mode for Development Only

```bash
# ✅ Development
./phpeek-pm serve --dev

# ❌ Production
./phpeek-pm serve  # No --dev flag
```

### 2. Validate Before Saving

```bash
# Validate in another terminal before saving
./phpeek-pm check-config --config config.yaml --strict

# If valid, save file
# If invalid, fix errors first
```

### 3. Small, Incremental Changes

```bash
# ✅ Good: One change at a time
# Edit: scale: 3 → scale: 5
# Save → Test → Iterate

# ❌ Bad: Many changes at once
# Edit: Change scale, log level, dependencies, health checks
# Save → Hard to debug if issues arise
```

### 4. Watch the Logs

```bash
# Keep PHPeek PM logs visible
./phpeek-pm serve --dev | tee phpeek.log

# Watch for:
# - Config change detection
# - Validation errors
# - Graceful shutdown
# - Process start/stop
```

### 5. Test Validation First

```bash
# Before making risky changes
./phpeek-pm check-config --config config.yaml

# Intentionally make invalid change
# Observe validation prevents reload
# Fix error, save again
```

### 6. Use Version Control

```bash
# Commit working configs
git commit -m "Working config with 3 workers"

# Experiment with changes
# If broken, easy to revert
git checkout config.yaml
```

## Examples

### Example 1: Iterating on Worker Counts

```bash
# Terminal 1: Start PHPeek with dev mode
./phpeek-pm serve --config dev.yaml --dev
```

```bash
# Terminal 2: Edit config
vim dev.yaml

# Initial: scale: 3
# Change to: scale: 5
# Save

# PHPeek auto-reloads
# Restart PHPeek to apply

# Test with 5 workers

# If needed, change to: scale: 10
# Save, restart, test

# Find optimal count
```

### Example 2: Debugging with Log Levels

```bash
# Terminal 1: Running with info level
./phpeek-pm serve --config dev.yaml --dev
```

```bash
# Terminal 2: Hit issue, need debug logs
vim dev.yaml

# Change: log_level: info → log_level: debug
# Save

# Terminal 1: Auto-reload triggers
# Restart with debug logging

# Investigate issue with detailed logs

# Fix issue, restore info level
vim dev.yaml
# Change: log_level: debug → log_level: info
# Save, restart
```

### Example 3: Health Check Tuning

```bash
# Start with conservative settings
health_check:
  interval: 30
  timeout: 10
  retries: 5

# Edit: Reduce interval for faster detection
# Change interval: 30 → interval: 10
# Save, restart, observe

# Edit: Reduce timeout
# Change timeout: 10 → timeout: 5
# Save, restart, observe

# Edit: Reduce retries
# Change retries: 5 → retries: 3
# Save, restart, test

# Final tuned config based on observations
```

## Next Steps

- [Configuration Validation](../configuration/validation.md) - Validate changes before reload
- [Configuration Overview](../configuration/overview.md) - Complete configuration reference
- [Troubleshooting](../guides/troubleshooting.md) - Common issues and solutions
- [Quick Start](../getting-started/quickstart.md) - Get started with PHPeek PM
