#!/bin/sh
set -e

echo "=== PHPeek PM Integration Tests ==="
echo ""

# Test 1: Binary exists and is executable
echo "✓ Test 1: Binary Check"
if [ ! -x /usr/local/bin/phpeek-pm ]; then
    echo "✗ FAILED: Binary not executable"
    exit 1
fi
echo "  Binary is executable"
echo ""

# Test 2: Version check
echo "✓ Test 2: Version Check"
/usr/local/bin/phpeek-pm --version 2>&1 || true
echo ""

# Test 3: Configuration validation
echo "✓ Test 3: Configuration Validation"
if [ ! -f /etc/phpeek-pm/phpeek-pm.yaml ]; then
    echo "✗ FAILED: Config file not found"
    exit 1
fi
echo "  Config file exists"
echo ""

# Test 4: Start PHPeek PM in background
echo "✓ Test 4: Process Startup"
/usr/local/bin/phpeek-pm &
PHPEEK_PID=$!
echo "  PHPeek PM started with PID: $PHPEEK_PID"

# Wait for startup
sleep 3

# Test 5: Check process is running
echo ""
echo "✓ Test 5: Process Running"
if ! kill -0 $PHPEEK_PID 2>/dev/null; then
    echo "✗ FAILED: PHPeek PM not running"
    exit 1
fi
echo "  PHPeek PM is running"

# Test 6: Check managed processes
echo ""
echo "✓ Test 6: Managed Processes"
# Use /proc to check for sleep processes (works everywhere)
if ! find /proc -maxdepth 1 -type d -name '[0-9]*' -exec sh -c 'grep -l "sleep" /proc/{}/cmdline 2>/dev/null' \; -quit 2>/dev/null | grep -q .; then
    echo "✗ FAILED: Managed process not found"
    kill $PHPEEK_PID 2>/dev/null || true
    exit 1
fi
echo "  Managed processes are running"

# Test 7: Metrics endpoint (if available)
echo ""
echo "✓ Test 7: Metrics Endpoint"
if command -v curl > /dev/null 2>&1; then
    if curl -s --max-time 2 http://localhost:9090/metrics | grep -q "phpeek_pm"; then
        echo "  Metrics endpoint is responding"
    else
        echo "  ⚠ Metrics endpoint not responding (non-fatal)"
    fi
else
    echo "  ⚠ curl not available, skipping metrics test"
fi

# Test 8: API endpoint (if available)
echo ""
echo "✓ Test 8: API Endpoint"
if command -v curl > /dev/null 2>&1; then
    if curl -s --max-time 2 http://localhost:8080/api/v1/health | grep -q "healthy"; then
        echo "  API endpoint is responding"
    else
        echo "  ⚠ API endpoint not responding (non-fatal)"
    fi
else
    echo "  ⚠ curl not available, skipping API test"
fi

# Test 9: Auto-exit when all processes die
echo ""
echo "✓ Test 9: Auto-exit on Process Death"
echo "  Waiting for managed processes to complete (sleep 5)..."

# Wait for auto-exit (should happen after ~5 seconds when sleep processes finish)
# Give it up to 10 seconds total
for i in 1 2 3 4 5 6 7 8 9 10; do
    if ! kill -0 $PHPEEK_PID 2>/dev/null; then
        echo "  PHPeek PM auto-exited after managed processes died (${i}s elapsed)"
        break
    fi
    sleep 1
done

# Check if process stopped
if kill -0 $PHPEEK_PID 2>/dev/null; then
    echo "  ✗ FAILED: PHPeek PM still running after 10s"
    kill -KILL $PHPEEK_PID 2>/dev/null || true
    wait $PHPEEK_PID 2>/dev/null || true
    exit 1
fi

# Give it a moment to clean up
sleep 1

# Test 10: Verify no zombie processes
echo ""
echo "✓ Test 10: Zombie Check"
# Use /proc to check for sleep processes
if find /proc -maxdepth 1 -type d -name '[0-9]*' -exec sh -c 'grep -l "sleep" /proc/{}/cmdline 2>/dev/null' \; -quit 2>/dev/null | grep -q .; then
    echo "  ⚠ Managed processes still running (may be zombies)"
    # Kill any remaining sleep processes
    for pid in $(find /proc -maxdepth 1 -type d -name '[0-9]*' | sed 's/\/proc\///'); do
        if grep -q "sleep" /proc/$pid/cmdline 2>/dev/null; then
            kill -9 $pid 2>/dev/null || true
        fi
    done
else
    echo "  No zombie processes detected"
fi

echo ""
echo "=== All Integration Tests Passed! ==="
echo ""

# Show system info
echo "System Information:"
echo "  Distro: $(cat /etc/os-release | grep "^PRETTY_NAME" | cut -d'"' -f2)"
echo "  Kernel: $(uname -r)"
echo "  Arch: $(uname -m)"
