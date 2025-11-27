---
title: "Terminal User Interface (TUI)"
description: "Interactive k9s-style terminal UI for monitoring and managing processes with keyboard-driven controls"
weight: 26
---

# Terminal User Interface (TUI)

PHPeek PM includes a modern, k9s-style terminal user interface for interactive process management. The TUI provides real-time monitoring, process control, and configuration management without leaving your terminal.

## Overview

**Features:**
- 📊 Real-time process status monitoring
- ⌨️ Keyboard-driven interface (Vim-style navigation)
- 🔄 Start, stop, restart, and scale processes interactively
- ➕ Add new processes via interactive wizard
- 📝 View process logs with tail functionality
- 🔌 Dual connectivity (Unix socket + TCP)
- 🔒 Automatic connection fallback and security

## Quick Start

```bash
# Local connection (auto-detects Unix socket, falls back to TCP)
./phpeek-pm tui

# Explicit TCP connection
./phpeek-pm tui --url http://localhost:9180

# Remote connection with authentication
./phpeek-pm tui --url http://remote-host:9180 --auth your-secret-token

# TLS connection
./phpeek-pm tui --url https://remote-host:9180 --auth your-secret-token
```

## Connection Modes

The TUI supports two connection modes with intelligent auto-detection:

### Unix Socket (Local, Preferred)

**Advantages:**
- 🔒 **Secure**: File permissions control access (0600 = owner-only)
- ⚡ **Fast**: No network stack overhead
- 🎯 **Simple**: No authentication required (filesystem handles it)

**Auto-detected paths** (priority order):
1. `/var/run/phpeek-pm.sock`
2. `/tmp/phpeek-pm.sock`
3. `/run/phpeek-pm.sock`

**Configuration:**
```yaml
global:
  api_enabled: true
  api_socket: "/var/run/phpeek-pm.sock"
```

### TCP (Remote, Fallback)

**Advantages:**
- 🌐 **Remote access**: Connect from anywhere
- 🔐 **Optional TLS**: Encrypted connections
- 🛡️ **ACL support**: IP-based filtering
- 🔑 **Authentication**: Bearer token auth

**Configuration:**
```yaml
global:
  api_enabled: true
  api_port: 9180

  # Optional: Authentication
  api_auth: "your-secret-token"

  # Optional: TLS
  api_tls:
    enabled: true
    cert_file: "/etc/phpeek-pm/server.crt"
    key_file: "/etc/phpeek-pm/server.key"

  # Optional: IP ACL
  api_acl:
    allow: ["127.0.0.1", "10.0.0.0/8"]
    deny: []
```

### Auto-Detection Logic

The TUI automatically tries connection methods in this order:

1. ✅ **Unix socket** (each path in priority order)
   - If socket exists and accessible → Use Unix socket
2. ⏭️ **TCP fallback** (if no socket found)
   - Falls back to TCP connection

**Example:**
```bash
# TUI tries:
# 1. /var/run/phpeek-pm.sock → Found! Using Unix socket
# OR
# 1. /var/run/phpeek-pm.sock → Not found
# 2. /tmp/phpeek-pm.sock → Not found
# 3. /run/phpeek-pm.sock → Not found
# 4. http://localhost:9180 → Fallback to TCP
```

## Keyboard Shortcuts

### Process List View

| Key | Action | Description |
|-----|--------|-------------|
| `↑/↓` or `j/k` | Navigate | Move selection up/down in process list |
| `a` | Add Process | Open interactive process creation wizard |
| `r` | Restart | Restart selected process |
| `s` | Stop | Stop selected process |
| `x` | Start | Start selected process |
| `+` or `=` | Scale Up | Open scale dialog (increase instances) |
| `-` | Scale Down | Open scale dialog (decrease instances) |
| `Enter` | View Logs | Open log viewer for selected process |
| `?` | Help | Show keyboard shortcuts help screen |
| `q` or `Esc` | Quit | Exit TUI |

### Add Process Wizard

| Key | Action | Description |
|-----|--------|-------------|
| `Tab` or `Enter` | Next Step | Move to next wizard step |
| `Shift+Tab` | Previous Step | Go back to previous step |
| `Esc` | Cancel | Cancel wizard and return to process list |
| `Ctrl+W` | Add Item | Add command part, environment variable, etc. |
| `Ctrl+D` | Remove Item | Remove selected item from list |
| `↑/↓` | Select Option | Navigate dropdown options |

### Log Viewer

| Key | Action | Description |
|-----|--------|-------------|
| `↑/↓` or `j/k` | Scroll | Scroll through log lines |
| `g` | Go to Top | Jump to beginning of logs |
| `G` | Go to Bottom | Jump to end of logs (tail mode) |
| `Esc` or `q` | Back | Return to process list |

## Process Management

### Start/Stop/Restart

**Interactive control:**
1. Navigate to process with `↑/↓` or `j/k`
2. Press:
   - `x` to start a stopped process
   - `s` to stop a running process
   - `r` to restart a running process
3. Confirmation dialog appears
4. Press `Enter` to confirm or `Esc` to cancel

**Visual feedback:**
- ✅ Success: Green toast notification
- ❌ Error: Red toast notification with error message
- ⏳ In Progress: Status indicator updates in real-time

### Scaling Processes

**Increase/decrease instances:**
1. Navigate to process
2. Press `+` (scale up) or `-` (scale down)
3. Scale dialog appears with:
   - Current desired instances
   - Current actual instances
   - Input field for new desired count
4. Enter new value and press `Enter`
5. Process supervisor adjusts instances automatically

**Example:**
```
Process: queue-worker
Current: 3 instances running
Action: Press '+' → Enter '5' → Press Enter
Result: 2 new instances spawned (total: 5)
```

## Add Process Wizard

The interactive wizard guides you through creating a new process configuration.

### Wizard Steps

**Step 1: Process Name**
```
┌─ Add New Process (1/6) ─────────────────────────┐
│ Process Name                                     │
│ ┌──────────────────────────────────────────────┐ │
│ │ queue-worker-notifications                   │ │
│ └──────────────────────────────────────────────┘ │
│                                                  │
│ Enter unique process identifier (lowercase)     │
└──────────────────────────────────────────────────┘
```

**Step 2: Command**
```
┌─ Add New Process (2/6) ─────────────────────────┐
│ Command                                          │
│ ┌──────────────────────────────────────────────┐ │
│ │ php                                          │ │
│ │ artisan                                      │ │
│ │ queue:work                                   │ │
│ │ --queue=notifications                        │ │
│ │ --tries=3                                    │ │
│ └──────────────────────────────────────────────┘ │
│                                                  │
│ Ctrl+W: Add part  Ctrl+D: Remove selected       │
└──────────────────────────────────────────────────┘
```

**Step 3: Scale (Instances)**
```
┌─ Add New Process (3/6) ─────────────────────────┐
│ Number of Instances                              │
│ ┌──────────────────────────────────────────────┐ │
│ │ 2                                            │ │
│ └──────────────────────────────────────────────┘ │
│                                                  │
│ How many instances to run (1-100)               │
└──────────────────────────────────────────────────┘
```

**Step 4: Restart Policy**
```
┌─ Add New Process (4/6) ─────────────────────────┐
│ Restart Policy                                   │
│ ┌──────────────────────────────────────────────┐ │
│ │ ▶ always                                     │ │
│ │   on-failure                                 │ │
│ │   never                                      │ │
│ └──────────────────────────────────────────────┘ │
│                                                  │
│ ↑/↓: Select  Enter: Confirm                     │
└──────────────────────────────────────────────────┘
```

**Step 5: Priority**
```
┌─ Add New Process (5/6) ─────────────────────────┐
│ Startup Priority                                 │
│ ┌──────────────────────────────────────────────┐ │
│ │ 40                                           │ │
│ └──────────────────────────────────────────────┘ │
│                                                  │
│ Lower numbers start first (1-100)               │
└──────────────────────────────────────────────────┘
```

**Step 6: Preview & Confirm**
```
┌─ Add New Process (6/6) ─────────────────────────┐
│ Preview Configuration                            │
│                                                  │
│ Name: queue-worker-notifications                │
│ Command: ["php", "artisan", "queue:work",      │
│           "--queue=notifications", "--tries=3"] │
│ Scale: 2                                         │
│ Restart: always                                  │
│ Priority: 40                                     │
│                                                  │
│ Press Enter to create, Esc to cancel            │
└──────────────────────────────────────────────────┘
```

### Post-Creation

**After confirmation:**
1. TUI calls API: `POST /api/v1/processes` with wizard data
2. Process is created in configuration
3. TUI automatically calls: `POST /api/v1/config/save` to persist
4. Process list refreshes showing new process
5. New process starts automatically (if enabled)

## Process Status Display

### Status Indicators

```
NAME              STATUS    INSTANCES  RESTARTS  UPTIME     MEMORY    CPU
php-fpm           ✓ running 2/2        0         2h30m15s   256MB     2.5%
nginx             ✓ running 1/1        0         2h30m10s   64MB      0.8%
horizon           ✓ running 1/1        1         15m30s     128MB     5.2%
queue-default     ✓ running 3/3        0         2h30m05s   384MB     12.1%
queue-emails      ✗ stopped 0/2        5         -          -         -
```

**Status symbols:**
- ✓ `running` - Process is active and healthy
- ✗ `stopped` - Process is not running
- ⚠ `starting` - Process is initializing
- ⏸ `stopping` - Graceful shutdown in progress
- ⚠ `unhealthy` - Health checks failing

**Instance format:** `actual/desired`
- `2/2` - All desired instances running
- `1/2` - Scale drift detected (1 running, 2 desired)
- `0/2` - Process stopped or failed to start

## Configuration Requirements

**Minimum requirements** to use TUI:

```yaml
global:
  # Enable API server
  api_enabled: true

  # Choose connection method:

  # Option 1: Unix socket (local, recommended)
  api_socket: "/var/run/phpeek-pm.sock"

  # Option 2: TCP (remote access)
  api_port: 9180

  # Optional: Authentication (TCP only)
  api_auth: "your-secret-token"
```

**Full example with security:**

```yaml
global:
  api_enabled: true
  api_port: 9180
  api_socket: "/var/run/phpeek-pm.sock"

  # Authentication (TCP only, socket uses file permissions)
  api_auth: "${PHPEEK_PM_API_TOKEN}"

  # TLS for remote access
  api_tls:
    enabled: true
    cert_file: "/etc/phpeek-pm/tls/server.crt"
    key_file: "/etc/phpeek-pm/tls/server.key"

  # IP ACL for additional security
  api_acl:
    allow: ["127.0.0.1", "10.0.0.0/8"]
    deny: []
```

## Security Considerations

### Unix Socket Security

**File permissions:**
```bash
# Recommended: Owner-only access (0600)
chmod 600 /var/run/phpeek-pm.sock
chown phpeek:phpeek /var/run/phpeek-pm.sock

# Verify permissions
ls -la /var/run/phpeek-pm.sock
# Output: srw------- 1 phpeek phpeek 0 Nov 24 10:00 phpeek-pm.sock
```

**Access control:**
- Unix socket access controlled by filesystem permissions
- No authentication required (filesystem handles it)
- Recommended for local TUI access on production servers

### TCP Security

**Best practices:**
1. **Always use TLS** for remote connections
2. **Enable authentication** with strong tokens
3. **Use IP ACL** to restrict source IPs
4. **Rotate tokens** regularly
5. **Monitor API access** via logs

**Production example:**
```yaml
global:
  api_enabled: true
  api_port: 9180
  api_auth: "${PHPEEK_PM_API_TOKEN}"  # Load from env

  api_tls:
    enabled: true
    cert_file: "/etc/phpeek-pm/tls/server.crt"
    key_file: "/etc/phpeek-pm/tls/server.key"

  api_acl:
    allow: ["10.0.0.0/8"]  # VPN network only
    deny: []
```

## Troubleshooting

### TUI Won't Connect

**Error:** `Failed to connect to PHPeek PM API`

**Solutions:**
1. **Check API is enabled:**
   ```yaml
   global:
     api_enabled: true
   ```

2. **Verify phpeek-pm is running:**
   ```bash
   ps aux | grep phpeek-pm
   ```

3. **Check socket exists (local):**
   ```bash
   ls -la /var/run/phpeek-pm.sock
   # Should show: srw------- 1 user user 0 ... phpeek-pm.sock
   ```

4. **Test TCP connection (remote):**
   ```bash
   curl http://localhost:9180/api/v1/health
   # Should return: {"status":"healthy"}
   ```

5. **Check authentication:**
   ```bash
   # If api_auth is set, token required
   curl -H "Authorization: Bearer your-token" \
     http://localhost:9180/api/v1/processes
   ```

### Permission Denied on Unix Socket

**Error:** `Permission denied: /var/run/phpeek-pm.sock`

**Solutions:**
1. **Check socket permissions:**
   ```bash
   ls -la /var/run/phpeek-pm.sock
   ```

2. **Add user to socket owner group:**
   ```bash
   sudo usermod -a -G phpeek $USER
   # Re-login for group changes to take effect
   ```

3. **Run TUI as socket owner:**
   ```bash
   sudo -u phpeek phpeek-pm tui
   ```

### Scale/Restart Not Working

**Error:** `Operation failed` or no response

**Solutions:**
1. **Check process is enabled:**
   ```yaml
   processes:
     worker:
       enabled: true  # Must be true
   ```

2. **Verify no config errors:**
   ```bash
   phpeek-pm check-config --strict
   ```

3. **Check API logs:**
   ```bash
   # Look for API errors in PHPeek logs
   journalctl -u phpeek-pm -f
   ```

## Examples

### Local Development

```bash
# Start PHPeek PM with TUI-ready config
./phpeek-pm serve --config dev.yaml

# In another terminal, launch TUI
./phpeek-pm tui
# Auto-detects socket at /tmp/phpeek-pm.sock
```

### Remote Production Monitoring

```bash
# From local machine, connect to production server
phpeek-pm tui \
  --url https://prod-server.example.com:9180 \
  --auth "${PHPEEK_PM_PROD_TOKEN}"

# Monitor processes, view logs, scale workers as needed
```

### Docker Container Access

```bash
# Exec into running container
docker exec -it my-app /bin/sh

# Launch TUI inside container
phpeek-pm tui
# Uses Unix socket /var/run/phpeek-pm.sock
```

## Advanced Features

### Log Streaming

**Real-time log viewing:**
1. Navigate to process and press `Enter`
2. Logs stream in real-time (tail mode)
3. Scroll with `↑/↓` or `j/k`
4. Jump to top with `g`, bottom with `G`
5. Press `Esc` to return to process list

**Features:**
- Automatic scrolling (tail -f style)
- Color-coded log levels (ERROR=red, WARN=yellow, etc.)
- Multi-line log reassembly (stack traces)
- Search/filter (coming soon)

### Bulk Operations

**Scale multiple workers:**
1. Scale `queue-default` from 3→5 (`+`)
2. Return to list
3. Scale `queue-emails` from 2→4 (`+`)
4. All changes persist automatically

**Quick restart all:**
- Restart each process individually
- Changes apply immediately
- Health checks validate restart success

## Integration with API

The TUI uses the Management API under the hood:

| TUI Action | API Call | Endpoint |
|------------|----------|----------|
| List processes | GET | `/api/v1/processes` |
| Restart process | POST | `/api/v1/processes/{name}/restart` |
| Stop process | POST | `/api/v1/processes/{name}/stop` |
| Start process | POST | `/api/v1/processes/{name}/start` |
| Scale process | POST | `/api/v1/processes/{name}/scale` |
| Add process | POST | `/api/v1/processes` |
| Save config | POST | `/api/v1/config/save` |

See [Management API](../observability/api.md) for API documentation.

## Next Steps

- [Management API](../observability/api.md) - REST API documentation
- [Configuration](../configuration/overview.md) - API configuration reference
- [Process Scaling](process-scaling.md) - Scaling strategies and patterns
- [Troubleshooting](../guides/troubleshooting.md) - Common issues and solutions
