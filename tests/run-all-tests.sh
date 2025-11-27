#!/bin/bash
set -e

echo "==================================================================="
echo "PHPeek PM - Complete Test Suite"
echo "==================================================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
TESTS_PASSED=0
TESTS_FAILED=0

run_test() {
    local test_name="$1"
    local test_command="$2"

    echo -e "${YELLOW}Running: ${test_name}${NC}"

    if eval "$test_command"; then
        echo -e "${GREEN}✓ PASSED${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗ FAILED${NC}"
        ((TESTS_FAILED++))
    fi
    echo ""
}

# 1. Unit Tests
echo "==================================================================="
echo "1. Unit Tests"
echo "==================================================================="
echo ""

run_test "Go Unit Tests" "make test"
run_test "Race Detector" "go test -race -short ./..."

# 2. Build Tests
echo "==================================================================="
echo "2. Build Tests"
echo "==================================================================="
echo ""

run_test "Build Current Platform" "make build"
run_test "Build All Platforms" "make build-all"

# 3. Binary Tests
echo "==================================================================="
echo "3. Binary Tests"
echo "==================================================================="
echo ""

run_test "Binary Exists" "test -f build/phpeek-pm"
run_test "Binary Executable" "test -x build/phpeek-pm"
run_test "Binary Size Check" "test $(stat -f%z build/phpeek-pm 2>/dev/null || stat -c%s build/phpeek-pm) -lt 50000000"

# 4. Integration Tests (Docker required)
if command -v docker &> /dev/null; then
    echo "==================================================================="
    echo "4. Integration Tests"
    echo "==================================================================="
    echo ""

    for distro in alpine debian ubuntu; do
        run_test "Integration Test - $distro" "
            docker build -f tests/integration/Dockerfile.$distro -t phpeek-pm-test-$distro . > /dev/null 2>&1 && \
            docker run --rm phpeek-pm-test-$distro
        "
    done
else
    echo "==================================================================="
    echo "4. Integration Tests - SKIPPED (Docker not available)"
    echo "==================================================================="
    echo ""
fi

# 5. Functional Tests
echo "==================================================================="
echo "5. Functional Tests"
echo "==================================================================="
echo ""

# Create test config
cat > /tmp/phpeek-test-config.yaml <<EOF
version: "1.0"
global:
  shutdown_timeout: 10
  log_level: info
  metrics_enabled: true
  api_enabled: true
processes:
  sleeper:
    enabled: true
    command: ["sleep", "30"]
    priority: 10
    restart: never
    scale: 2
    health_check:
      type: exec
      command: ["echo", "healthy"]
      initial_delay: 1
      period: 5
      timeout: 2
      failure_threshold: 2
      success_threshold: 1
EOF

run_test "Start PHPeek PM" "
    PHPEEK_PM_CONFIG=/tmp/phpeek-test-config.yaml ./build/phpeek-pm > /tmp/phpeek-test.log 2>&1 &
    PHPEEK_PID=\$!
    sleep 3
    kill -0 \$PHPEEK_PID 2>/dev/null
    echo \$PHPEEK_PID > /tmp/phpeek-test.pid
"

if [ -f /tmp/phpeek-test.pid ]; then
    PHPEEK_PID=$(cat /tmp/phpeek-test.pid)

    run_test "Metrics Endpoint" "curl -sf http://localhost:9090/metrics > /dev/null"
    run_test "API Health Endpoint" "curl -sf http://localhost:9180/api/v1/health | grep -q healthy"
    run_test "API Processes Endpoint" "curl -sf http://localhost:9180/api/v1/processes | grep -q sleeper"

    run_test "Graceful Shutdown" "
        kill -TERM \$PHPEEK_PID
        sleep 3
        ! kill -0 \$PHPEEK_PID 2>/dev/null
    "

    # Cleanup
    kill -KILL $PHPEEK_PID 2>/dev/null || true
    rm -f /tmp/phpeek-test.pid /tmp/phpeek-test.log /tmp/phpeek-test-config.yaml
fi

# 6. Configuration Tests
echo "==================================================================="
echo "6. Configuration Tests"
echo "==================================================================="
echo ""

run_test "Valid YAML Config" "
    cat > /tmp/test-valid-config.yaml <<EOF
version: '1.0'
global:
  shutdown_timeout: 30
  log_level: info
processes:
  test:
    enabled: true
    command: ['sleep', '1']
    priority: 10
EOF
    PHPEEK_PM_CONFIG=/tmp/test-valid-config.yaml ./build/phpeek-pm > /dev/null 2>&1 &
    PID=\$!
    sleep 1
    kill -TERM \$PID 2>/dev/null
    wait \$PID 2>/dev/null
    rm -f /tmp/test-valid-config.yaml
"

# 7. Performance Tests
echo "==================================================================="
echo "7. Performance Tests"
echo "==================================================================="
echo ""

run_test "Startup Time < 2s" "
    START=\$(date +%s)
    cat > /tmp/perf-test-config.yaml <<EOF
version: '1.0'
global:
  shutdown_timeout: 5
  log_level: error
processes:
  test:
    enabled: true
    command: ['sleep', '1']
    priority: 10
    restart: never
EOF
    PHPEEK_PM_CONFIG=/tmp/perf-test-config.yaml ./build/phpeek-pm > /dev/null 2>&1 &
    PID=\$!
    sleep 1
    END=\$(date +%s)
    DURATION=\$((END - START))
    kill -TERM \$PID 2>/dev/null || true
    wait \$PID 2>/dev/null || true
    rm -f /tmp/perf-test-config.yaml
    [ \$DURATION -lt 2 ]
"

# Summary
echo "==================================================================="
echo "Test Summary"
echo "==================================================================="
echo ""
echo -e "Total Tests: $((TESTS_PASSED + TESTS_FAILED))"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}==================================================================="
    echo "✓ All Tests Passed!"
    echo -e "===================================================================${NC}"
    exit 0
else
    echo -e "${RED}==================================================================="
    echo "✗ Some Tests Failed"
    echo -e "===================================================================${NC}"
    exit 1
fi
