# PHPeek PM - Local Testing Guide

Complete guide for testing PHPeek PM features locally during development.

## Quick Start

### Build
```bash
make build
# → Creates build/phpeek-pm
```

### Basic Commands
```bash
# Show version
./build/phpeek-pm version
./build/phpeek-pm version --short

# Validate configuration
./build/phpeek-pm check-config -c configs/examples/minimal.yaml

# Show help
./build/phpeek-pm --help
./build/phpeek-pm serve --help
```

---

## Testing Initial State Control

### Create Test Config
```bash
cat > test-initial-state.yaml <<'EOF'
version: "1.0"

global:
  log_level: info
  log_format: text
  api_enabled: true
  api_port: 8080

processes:
  auto-start:
    enabled: true
    initial_state: running  # Starts automatically
    command: ["sleep", "300"]
    restart: always

  manual-start:
    enabled: true
    initial_state: stopped  # Doesn't start automatically
    command: ["sleep", "300"]
    restart: always
EOF
```

### Test
```bash
# Terminal 1: Start daemon
./build/phpeek-pm serve -c test-initial-state.yaml
```

**Expected output:**
```
🚀 PHPeek Process Manager v1.0.0

INFO Process started successfully name=auto-start
INFO Process in initial stopped state name=manual-start initial_state=stopped
INFO All processes started successfully
INFO API server started port=8080
```

### Verify
```bash
# Terminal 2: Check running processes
ps aux | grep sleep
# Should only show auto-start, NOT manual-start

# Start via API
curl -X POST http://localhost:8080/api/v1/processes/manual-start/start

# Check again
ps aux | grep sleep
# Now shows BOTH processes
```

---

## Testing Scale Locked

### Create Test Config
```bash
cat > test-scale-locked.yaml <<'EOF'
version: "1.0"

global:
  api_enabled: true
  api_port: 8080

processes:
  fixed-port:
    command: ["sleep", "300"]
    scale: 1
    scale_locked: true  # Cannot scale (simulates nginx on :80)

  scalable:
    command: ["sleep", "300"]
    scale: 2
    scale_locked: false  # Can scale freely
EOF
```

### Test
```bash
# Start daemon
./build/phpeek-pm serve -c test-scale-locked.yaml &

# Try to scale locked process (should fail)
curl -X POST http://localhost:8080/api/v1/processes/fixed-port/scale \
  -H "Content-Type: application/json" \
  -d '{"desired": 2}'

# Expected: {"error":"process fixed-port is scale-locked (likely binds to fixed port - cannot scale)"}

# Try to scale unlocked process (will fail with "not implemented" but validates lock check works)
curl -X POST http://localhost:8080/api/v1/processes/scalable/scale \
  -H "Content-Type: application/json" \
  -d '{"desired": 5}'

# Expected: {"error":"scaling not yet implemented..."}  (but NOT scale-locked error!)
```

---

## Testing TUI (Embedded Mode)

### Basic TUI Test
```bash
cat > tui-demo.yaml <<'EOF'
version: "1.0"

processes:
  service1:
    command: ["sleep", "300"]
    initial_state: running

  service2:
    command: ["sleep", "300"]
    initial_state: stopped

  service3:
    command: ["sleep", "300"]
    scale: 3
EOF

# Launch TUI
./build/phpeek-pm tui -c tui-demo.yaml
```

**TUI Controls:**
- `j/k` or `↑↓` - Navigate process list
- `g` - Jump to top
- `G` - Jump to bottom
- `?` - Show help
- `Enter` - View logs (placeholder currently)
- `ESC` - Back to process list
- `q` - Quit

**Expected:**
- See process table with 3 processes
- service1 shows "✓ Running"
- service2 shows "○ Stopped"
- service3 shows scale "3/3"

---

## Testing PHP-FPM Auto-Tuning

### Without Container Limits (Host Memory)
```bash
PHP_FPM_AUTOTUNE_PROFILE=medium ./build/phpeek-pm serve -c configs/examples/minimal.yaml --dry-run
```

**Expected:**
- Warning about using host resources
- Calculates workers based on host memory
- Shows warnings count

### With Different Profiles
```bash
# Dev profile (50% memory, 2 workers)
PHP_FPM_AUTOTUNE_PROFILE=dev ./build/phpeek-pm version

# Medium profile (75% memory)
PHP_FPM_AUTOTUNE_PROFILE=medium ./build/phpeek-pm version

# Heavy profile (80% memory)
PHP_FPM_AUTOTUNE_PROFILE=heavy ./build/phpeek-pm version
```

### With Custom Threshold
```bash
# Conservative (60%)
./build/phpeek-pm serve --php-fpm-profile=medium --autotune-memory-threshold=0.6 --dry-run

# Oversubscription (130% - DANGER!)
./build/phpeek-pm serve --php-fpm-profile=heavy --autotune-memory-threshold=1.3 --dry-run
```

**Expected:** DANGER warning about oversubscription

---

## Testing Readiness Blocking

### Requires Actual Services
```bash
# Use Laravel full config
./build/phpeek-pm serve -c configs/examples/laravel-full.yaml --dry-run

# Or create test with netcat
cat > test-readiness.yaml <<'EOF'
version: "1.0"

processes:
  backend:
    command: ["sh", "-c", "sleep 5 && nc -l -p 9999"]
    priority: 10
    health_check:
      type: tcp
      address: "127.0.0.1:9999"
      mode: readiness

  frontend:
    command: ["sleep", "300"]
    priority: 20
    depends_on: [backend]
EOF

./build/phpeek-pm serve -c test-readiness.yaml
```

**Expected:**
- backend starts
- "Waiting for dependencies" for frontend
- "Waiting for service readiness" (5 seconds)
- "Service ready"
- frontend starts AFTER backend is ready

---

## Testing Oneshot Services

```bash
cat > test-oneshot.yaml <<'EOF'
version: "1.0"

processes:
  init-task:
    type: oneshot
    command: ["sh", "-c", "echo 'Initialization...' && sleep 2 && echo 'Done!'"]
    priority: 10

  main-service:
    type: longrun
    command: ["sleep", "300"]
    priority: 20
    depends_on: [init-task]
EOF

./build/phpeek-pm serve -c test-oneshot.yaml
```

**Expected:**
```
INFO Starting process name=init-task
INFO Oneshot process completed successfully instance_id=init-task-0
INFO Oneshot completed, signaling readiness to dependents
INFO Waiting for dependencies process=main-service
INFO All dependencies ready
INFO Starting process name=main-service
```

---

## Testing API Endpoints

### Start Daemon with API
```bash
./build/phpeek-pm serve -c configs/examples/development.yaml &
DAEMON_PID=$!
```

### Test Endpoints
```bash
# Health check
curl http://localhost:8080/api/v1/health

# List processes
curl http://localhost:8080/api/v1/processes | jq

# Start stopped process
curl -X POST http://localhost:8080/api/v1/processes/nginx/start

# Stop running process
curl -X POST http://localhost:8080/api/v1/processes/nginx/stop

# Restart process
curl -X POST http://localhost:8080/api/v1/processes/nginx/restart

# Try to scale locked process (should fail)
curl -X POST http://localhost:8080/api/v1/processes/nginx/scale \
  -H "Content-Type: application/json" \
  -d '{"desired": 2}'
```

### Cleanup
```bash
kill $DAEMON_PID
```

---

## Testing Graceful Shutdown

### Test Ctrl+C (SIGINT)
```bash
./build/phpeek-pm serve -c configs/examples/minimal.yaml

# Press Ctrl+C
```

**Expected:**
```
INFO Received shutdown signal signal=interrupt
INFO Initiating graceful shutdown timeout=30
INFO Stopping process name=php-fpm
INFO Process stopped gracefully instance_id=php-fpm-0
INFO Shutdown completed duration_seconds=1.234
INFO PHPeek PM shutdown complete
```

**Should NOT restart processes after Ctrl+C!**

### Test SIGTERM (Docker stop)
```bash
# Terminal 1
./build/phpeek-pm serve -c configs/examples/minimal.yaml

# Terminal 2
pkill -TERM phpeek-pm
# Or: kill -TERM $(pgrep phpeek-pm)
```

**Expected:** Same graceful shutdown as Ctrl+C

---

## Testing Read-Only Root Filesystem

### Test Detection
```bash
# Set environment variable to simulate
PHPEEK_PM_READ_ONLY_ROOT=true ./build/phpeek-pm serve -c configs/examples/minimal.yaml --dry-run
```

**Expected:**
```
INFO Read-only root filesystem detected, skipping permission setup
  info="Runtime state will use /run/phpeek-pm (tmpfs)"
```

---

## Docker Testing

### Build Test Image
```bash
cat > Dockerfile.test <<'EOF'
FROM php:8.3-fpm-alpine

# Install nginx
RUN apk add --no-cache nginx

# Copy binary
COPY build/phpeek-pm /usr/local/bin/phpeek-pm

# Copy config
COPY configs/examples/development.yaml /etc/phpeek-pm/phpeek-pm.yaml

EXPOSE 80 8080 9090

CMD ["/usr/local/bin/phpeek-pm", "serve"]
EOF

docker build -f Dockerfile.test -t phpeek-test .
```

### Test Auto-Tuning with Memory Limits
```bash
# Dev profile with 512MB
docker run --rm -m 512M -e PHP_FPM_AUTOTUNE_PROFILE=dev phpeek-test serve --dry-run

# Medium profile with 2GB
docker run --rm -m 2G -e PHP_FPM_AUTOTUNE_PROFILE=medium phpeek-test serve --dry-run

# Heavy profile with 8GB
docker run --rm -m 8G --cpus=8 -e PHP_FPM_AUTOTUNE_PROFILE=heavy phpeek-test serve --dry-run
```

### Test Read-Only Root
```bash
docker run --rm --read-only --tmpfs /run:rw --tmpfs /tmp:rw \
  -e PHP_FPM_AUTOTUNE_PROFILE=medium \
  phpeek-test serve --dry-run
```

**Expected:** No errors, detects read-only root, uses /run for runtime state

---

## Run Tests

```bash
# All tests
make test

# Specific packages
go test ./internal/autotune -v
go test ./internal/process -v
go test ./internal/config -v
go test ./internal/api -v

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Manual Integration Test Script

```bash
cat > integration-test.sh <<'EOF'
#!/bin/bash
set -e

echo "=== PHPeek PM Integration Test ==="
echo

# 1. Build
echo "1. Building..."
make build
echo "✅ Build successful"
echo

# 2. Validate configs
echo "2. Validating configurations..."
./build/phpeek-pm check-config -c configs/examples/minimal.yaml
./build/phpeek-pm check-config -c configs/examples/development.yaml
./build/phpeek-pm check-config -c configs/examples/laravel-full.yaml
echo "✅ All configs valid"
echo

# 3. Test version
echo "3. Testing version command..."
./build/phpeek-pm version
./build/phpeek-pm version --short
echo

# 4. Test dry-run
echo "4. Testing dry-run mode..."
./build/phpeek-pm serve -c configs/examples/minimal.yaml --dry-run
echo

# 5. Test auto-tuning
echo "5. Testing auto-tuning..."
PHP_FPM_AUTOTUNE_PROFILE=medium ./build/phpeek-pm serve --dry-run -c configs/examples/minimal.yaml
echo

# 6. Test initial state (quick)
echo "6. Testing initial state..."
cat > /tmp/test-initial.yaml <<YAML
version: "1.0"
global:
  shutdown_timeout: 5
processes:
  running:
    command: ["sleep", "2"]
    initial_state: running
  stopped:
    command: ["sleep", "2"]
    initial_state: stopped
YAML

timeout 3 ./build/phpeek-pm serve -c /tmp/test-initial.yaml || true
echo "✅ Initial state test completed"
echo

echo "=== All Integration Tests Passed! ==="
EOF

chmod +x integration-test.sh
./integration-test.sh
```

---

## Quick Test Workflow

### Two-Terminal Test

**Terminal 1: Start daemon**
```bash
./build/phpeek-pm serve -c configs/examples/development.yaml
```

**Terminal 2: Control via API**
```bash
# Wait for daemon to start
sleep 2

# Check status
curl http://localhost:8080/api/v1/processes | jq '.processes[] | {name, state}'

# Start nginx (initially stopped)
curl -X POST http://localhost:8080/api/v1/processes/nginx/start

# Check it started
curl http://localhost:8080/api/v1/processes | jq '.processes[] | select(.name=="nginx")'

# Stop it
curl -X POST http://localhost:8080/api/v1/processes/nginx/stop

# Try to scale php-fpm (should fail - scale_locked)
curl -X POST http://localhost:8080/api/v1/processes/php-fpm/scale \
  -H "Content-Type: application/json" \
  -d '{"desired": 2}'
# Expected error: scale-locked
```

**Terminal 1: Stop daemon**
```
Ctrl+C
# Watch graceful shutdown logs
```

---

## TUI Testing (Current MVP)

### Embedded Mode
```bash
./build/phpeek-pm tui -c configs/examples/development.yaml
```

**What works NOW:**
- ✅ Process list table displays
- ✅ Real-time state updates (every 1 second)
- ✅ Keyboard navigation (j/k/g/G)
- ✅ Help overlay (? key)
- ✅ Quit (q key)

**What's coming (Phase 2):**
- ⏳ Real log streaming (Enter on process)
- ⏳ Process actions (s/r/+/- keys)
- ⏳ Remote mode (--remote http://localhost:8080)

### Remote Mode (Placeholder)
```bash
./build/phpeek-pm tui --remote http://localhost:8080
```

**Expected:**
```
🔗 Connecting to remote API: http://localhost:8080
⚠️  Remote mode not yet implemented (coming in Phase 3)
💡 For now, use embedded mode (no --remote flag)
```

---

## Development Config Testing

```bash
# Start with development config
./build/phpeek-pm serve -c configs/examples/development.yaml

# Expected behavior:
# - PHP-FPM: ✓ Running (initial_state: running)
# - Nginx: ○ Stopped (initial_state: stopped)
# - Horizon: ○ Stopped (initial_state: stopped)
# - Queue: ○ Stopped (initial_state: stopped)
# - API enabled on :8080

# Control processes:
curl -X POST localhost:8080/api/v1/processes/nginx/start
curl -X POST localhost:8080/api/v1/processes/horizon/start
curl -X POST localhost:8080/api/v1/processes/queue-default/start

# All should now be running
curl localhost:8080/api/v1/processes | jq '.processes[] | {name, state}'
```

---

## Common Issues & Solutions

### "Address already in use"
```bash
# Check if another instance is running
pgrep phpeek-pm
kill $(pgrep phpeek-pm)

# Or change API port
./build/phpeek-pm serve --config myconfig.yaml
# Edit myconfig.yaml: api_port: 8081
```

### "Configuration file not found"
```bash
# Specify full path
./build/phpeek-pm serve -c /full/path/to/config.yaml

# Or use PHPEEK_PM_CONFIG env var
export PHPEEK_PM_CONFIG=/path/to/config.yaml
./build/phpeek-pm serve
```

### TUI Not Showing Processes
```bash
# Verify processes are actually starting
./build/phpeek-pm serve -c your-config.yaml

# In another terminal, check API
curl localhost:8080/api/v1/processes
```

---

## Next Steps

**After local testing passes:**

1. **Test in Docker:**
   ```bash
   docker build -f Dockerfile.test -t phpeek-test .
   docker run -m 2G phpeek-test
   ```

2. **Test TUI Remote Mode (Phase 3):**
   ```bash
   # Terminal 1: Daemon
   ./build/phpeek-pm serve

   # Terminal 2: TUI connects to daemon
   ./build/phpeek-pm tui --remote http://localhost:8080
   ```

3. **Test Log Streaming (Phase 3):**
   - Real-time logs in TUI
   - Multi-process log aggregation
   - Search and filtering

4. **Production Testing:**
   - Kubernetes deployment
   - Load testing
   - Failover scenarios
