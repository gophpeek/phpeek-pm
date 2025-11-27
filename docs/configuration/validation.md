---
title: "Configuration Validation"
description: "Lint and validate PHPeek PM configurations with comprehensive error checking and CI/CD integration"
weight: 17
---

# Configuration Validation

PHPeek PM includes a powerful configuration validation system that catches errors before runtime. The `check-config` command provides comprehensive linting with errors, warnings, and best practice suggestions.

## Quick Start

```bash
# Validate configuration file
./phpeek-pm check-config

# Validate specific config
./phpeek-pm check-config --config production.yaml

# Quiet mode (one-line summary)
./phpeek-pm check-config --quiet

# Strict mode (fail on warnings, perfect for CI/CD)
./phpeek-pm check-config --strict

# JSON output (for automation/scripting)
./phpeek-pm check-config --json
```

## Validation Modes

### Full Report Mode (Default)

Displays detailed report with all validation issues categorized and explained.

```bash
./phpeek-pm check-config --config app.yaml
```

**Output:**
```
═══════════════════════════════════════════════════════════════
  Configuration Validation Report
═══════════════════════════════════════════════════════════════

  Total Issues: 7  ⚠️  3 Warning(s)  💡 4 Suggestion(s)

⚠️  WARNINGS (should be reviewed):
───────────────────────────────────────────────────────────────
  1. [global.api_auth]
     API running without authentication or ACL
     → Recommendation: Consider enabling API token auth or IP ACL for security

  2. [processes.horizon.shutdown.pre_stop_hook.timeout]
     Hook timeout (120s) exceeds global shutdown timeout (30s)
     → Recommendation: Set hook timeout < shutdown timeout to allow cleanup

  3. [processes.queue-default.restart]
     Restart policy 'always' with no health check
     → Recommendation: Add health check to prevent restart loops on persistent failures

💡 SUGGESTIONS (best practices):
───────────────────────────────────────────────────────────────
  1. [global.log_format]
     Text format is human-readable but not ideal for log aggregation
     → Consider: Use 'json' format for production with centralized logging

  2. [processes.php-fpm.health_check.interval]
     Health check interval (30s) is high
     → Consider: Reduce to 10-15s for faster failure detection

  3. [processes.nginx.depends_on]
     No dependency on php-fpm defined
     → Consider: Add 'depends_on: [php-fpm]' to ensure correct startup order

  4. [global.metrics_enabled]
     Metrics disabled - missing observability
     → Consider: Enable metrics for production monitoring

═══════════════════════════════════════════════════════════════
  ✅ Validation passed (with warnings)
═══════════════════════════════════════════════════════════════

📋 Configuration Summary:
   Path: configs/examples/app.yaml
   Version: 1.0
   Processes: 5
   Log Level: info
   Shutdown Timeout: 30s

✅ Configuration is valid but has warnings/suggestions
```

### Quiet Mode

One-line summary, perfect for shell scripts and quick checks.

```bash
./phpeek-pm check-config --quiet
```

**Output (success with warnings):**
```
✅ Configuration is valid (with issues): ⚠️  3 warning(s), 💡 4 suggestion(s)
```

**Output (errors found):**
```
❌ Configuration is invalid: 🚨 2 error(s), ⚠️  3 warning(s), 💡 1 suggestion(s)
```

**Exit codes:**
- `0` - Valid (may have warnings/suggestions)
- `1` - Invalid (errors found)

### Strict Mode

Treats warnings as errors, perfect for CI/CD pipelines where warnings must be resolved.

```bash
./phpeek-pm check-config --strict
```

**Behavior:**
- ❌ Exits with code `1` if ANY warnings exist
- ✅ Exits with code `0` only if zero errors AND zero warnings
- 💡 Suggestions are informational only (don't fail build)

**Example output:**
```
═══════════════════════════════════════════════════════════════
  Configuration Validation Report (Strict Mode)
═══════════════════════════════════════════════════════════════

⚠️  WARNINGS (treated as errors in strict mode):
───────────────────────────────────────────────────────────────
  1. [global.api_auth]
     API running without authentication or ACL
     → Recommendation: Enable API token auth or IP ACL for security

❌ Validation failed in strict mode (warnings exist)
Exit code: 1
```

### JSON Mode

Machine-readable output for automation, scripting, and tooling integration.

```bash
./phpeek-pm check-config --json
```

**Output structure:**
```json
{
  "valid": true,
  "errors": [],
  "warnings": [
    {
      "field": "global.api_auth",
      "message": "API running without authentication or ACL",
      "recommendation": "Consider enabling API token auth or IP ACL for security",
      "severity": "warning"
    }
  ],
  "suggestions": [
    {
      "field": "global.log_format",
      "message": "Text format is human-readable but not ideal for log aggregation",
      "recommendation": "Use 'json' format for production with centralized logging",
      "severity": "suggestion"
    }
  ],
  "summary": {
    "config_path": "configs/examples/app.yaml",
    "version": "1.0",
    "process_count": 5,
    "log_level": "info",
    "shutdown_timeout": "30s"
  },
  "counts": {
    "errors": 0,
    "warnings": 1,
    "suggestions": 1
  }
}
```

**Exit codes:**
- `0` - Valid (errors count is 0)
- `1` - Invalid (errors count > 0)

## Validation Categories

### 🚨 Errors (Blocking)

**Critical issues that MUST be fixed before starting PHPeek PM.**

**Examples:**
- Missing required fields (`version`, `processes`)
- Invalid data types (`shutdown_timeout: "abc"`)
- Circular dependencies (`A depends on B, B depends on A`)
- Invalid enum values (`restart: "sometimes"`)
- Out-of-range values (`scale: 1000`, max is 100)
- Unknown process references in `depends_on`
- Invalid regex patterns
- Port conflicts
- Invalid URLs/addresses

### ⚠️ Warnings (Non-Blocking)

**Issues that should be reviewed but won't prevent startup.**

**Examples:**
- Security concerns (API without auth, no TLS)
- Timing issues (hook timeout > shutdown timeout)
- Missing health checks with always-restart
- Deprecated options
- Sub-optimal configurations
- Resource limits not set

### 💡 Suggestions (Best Practices)

**Recommendations for optimal configuration and production readiness.**

**Examples:**
- JSON logging for production
- Metrics/API disabled
- Health check intervals too high/low
- Missing dependencies
- Log level too verbose for production
- Missing environment variable patterns

## What Gets Validated

### Field Validation

**Data types:**
```yaml
# ❌ Wrong type
shutdown_timeout: "thirty"  # Error: must be integer

# ✅ Correct type
shutdown_timeout: 30
```

**Required fields:**
```yaml
# ❌ Missing version
processes:
  php-fpm: ...

# ✅ Version included
version: "1.0"
processes:
  php-fpm: ...
```

**Valid enums:**
```yaml
# ❌ Invalid restart policy
restart: "sometimes"  # Error: must be always/on-failure/never

# ✅ Valid policy
restart: "always"
```

### Range Checks

**Timeouts:**
```yaml
# ⚠️ Warning: Too short
shutdown_timeout: 5  # Less than recommended 30s

# ✅ Recommended range
shutdown_timeout: 30  # 30-60s is ideal
```

**Ports:**
```yaml
# 🚨 Error: Privileged port without root
api_port: 80  # Requires root access

# 💡 Suggestion: Use non-privileged port
api_port: 9180  # No special permissions needed
```

**Scale limits:**
```yaml
# 🚨 Error: Scale too high
scale: 1000  # Max is 100

# ✅ Within limits
scale: 10
```

### Security Validation

**API authentication:**
```yaml
# ⚠️ Warning: No auth
global:
  api_enabled: true
  # No api_auth or api_acl defined

# ✅ Auth enabled
global:
  api_enabled: true
  api_auth: "${PHPEEK_PM_API_TOKEN}"
```

**TLS configuration:**
```yaml
# ⚠️ Warning: No TLS for remote access
global:
  api_enabled: true
  api_port: 9180
  # No api_tls defined

# ✅ TLS enabled
global:
  api_enabled: true
  api_port: 9180
  api_tls:
    enabled: true
    cert_file: "/etc/tls/server.crt"
    key_file: "/etc/tls/server.key"
```

**Secrets in config:**
```yaml
# ⚠️ Warning: Hardcoded secret
api_auth: "secret-token-123"

# ✅ Environment variable reference
api_auth: "${PHPEEK_PM_API_TOKEN}"
```

### Best Practices

**Log formats:**
```yaml
# 💡 Suggestion: Text format
log_format: text  # Not ideal for log aggregation

# ✅ Production-ready
log_format: json  # Ideal for centralized logging
```

**Restart policies:**
```yaml
# ⚠️ Warning: Always restart without health check
processes:
  worker:
    restart: always
    # No health_check defined

# ✅ With health check
processes:
  worker:
    restart: always
    health_check:
      type: exec
      command: ["pgrep", "-f", "queue:work"]
```

**Metrics and observability:**
```yaml
# 💡 Suggestion: Metrics disabled
global:
  metrics_enabled: false

# ✅ Observability enabled
global:
  metrics_enabled: true
  metrics_port: 9090
```

### System Requirements

**Privileged ports:**
```yaml
# 🚨 Error: Port 80 requires root
global:
  api_port: 80

# ✅ Non-privileged port
global:
  api_port: 9180
```

**File paths:**
```yaml
# 🚨 Error: File doesn't exist
api_tls:
  cert_file: "/nonexistent/server.crt"

# ✅ Valid path (validated at runtime)
api_tls:
  cert_file: "/etc/phpeek-pm/tls/server.crt"
```

### Dependencies

**Circular dependencies:**
```yaml
# 🚨 Error: Circular dependency
processes:
  nginx:
    depends_on: [php-fpm]
  php-fpm:
    depends_on: [nginx]  # Circular!

# ✅ Acyclic dependency graph
processes:
  php-fpm:
    depends_on: []
  nginx:
    depends_on: [php-fpm]
```

**Unknown references:**
```yaml
# 🚨 Error: Process doesn't exist
processes:
  nginx:
    depends_on: [nonexistent-process]

# ✅ Valid reference
processes:
  php-fpm:
    enabled: true
  nginx:
    depends_on: [php-fpm]
```

### Health Checks

**Valid addresses:**
```yaml
# 🚨 Error: Invalid URL
health_check:
  type: http
  address: "not-a-url"

# ✅ Valid HTTP URL
health_check:
  type: http
  address: "http://127.0.0.1:80/health"
```

**Command existence:**
```yaml
# ⚠️ Warning: Command may not exist
health_check:
  type: exec
  command: ["nonexistent-binary"]

# ✅ Standard command
health_check:
  type: exec
  command: ["pgrep", "-f", "php-fpm"]
```

## CI/CD Integration

### GitHub Actions

**Validate on pull requests:**

```yaml
name: Validate Configuration

on:
  pull_request:
    paths:
      - '**.yaml'
      - '**.yml'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Download PHPeek PM
        run: |
          curl -L https://github.com/gophpeek/phpeek-pm/releases/latest/download/phpeek-pm-linux-amd64 \
            -o phpeek-pm
          chmod +x phpeek-pm

      - name: Validate Configuration (Strict)
        run: |
          ./phpeek-pm check-config --config production.yaml --strict
          # Fails build if warnings exist
```

### GitLab CI

```yaml
validate-config:
  stage: test
  image: alpine:latest
  before_script:
    - apk add --no-cache curl
    - curl -L https://github.com/gophpeek/phpeek-pm/releases/latest/download/phpeek-pm-linux-amd64 -o phpeek-pm
    - chmod +x phpeek-pm
  script:
    - ./phpeek-pm check-config --config $CI_PROJECT_DIR/phpeek-pm.yaml --strict
  only:
    changes:
      - "**/*.yaml"
      - "**/*.yml"
```

### Pre-Commit Hook

**Validate before committing:**

```bash
#!/bin/bash
# .git/hooks/pre-commit

CONFIG_FILES=$(git diff --cached --name-only | grep -E '\.ya?ml$')

if [ -n "$CONFIG_FILES" ]; then
  echo "Validating configuration files..."

  for file in $CONFIG_FILES; do
    if echo "$file" | grep -q "phpeek-pm"; then
      ./phpeek-pm check-config --config "$file" --quiet || exit 1
    fi
  done

  echo "✅ All configurations valid"
fi
```

### Docker Build Validation

**Validate during image build:**

```dockerfile
FROM alpine:latest AS validator

# Copy binary and config
COPY phpeek-pm /usr/local/bin/phpeek-pm
COPY phpeek-pm.yaml /etc/phpeek-pm/phpeek-pm.yaml

# Validate configuration (fails build if invalid)
RUN phpeek-pm check-config --config /etc/phpeek-pm/phpeek-pm.yaml --strict

FROM alpine:latest
# ... rest of Dockerfile
```

### Makefile Integration

```makefile
.PHONY: validate
validate:
	@echo "Validating configuration..."
	@./phpeek-pm check-config --config phpeek-pm.yaml --strict

.PHONY: validate-all
validate-all:
	@echo "Validating all configs..."
	@for file in configs/**/*.yaml; do \
		echo "Checking $$file..."; \
		./phpeek-pm check-config --config $$file --strict || exit 1; \
	done
	@echo "✅ All configurations valid"

.PHONY: ci-validate
ci-validate:
	@./phpeek-pm check-config --config production.yaml --strict --json > validation-report.json
	@cat validation-report.json | jq '.valid' | grep -q 'true' || exit 1
```

## Automation and Tooling

### Shell Script Integration

```bash
#!/bin/bash
# validate-and-deploy.sh

set -e

CONFIG_FILE="production.yaml"

echo "Validating configuration..."
if phpeek-pm check-config --config "$CONFIG_FILE" --strict --quiet; then
  echo "✅ Configuration valid - proceeding with deployment"

  # Deploy
  docker build -t myapp:latest .
  docker push myapp:latest
else
  echo "❌ Configuration validation failed - aborting deployment"
  exit 1
fi
```

### JSON Output Processing

**Extract specific issues with jq:**

```bash
# Get all warnings
phpeek-pm check-config --json | jq '.warnings[]'

# Count errors
phpeek-pm check-config --json | jq '.counts.errors'

# Check if valid
phpeek-pm check-config --json | jq -r '.valid'

# Generate report
phpeek-pm check-config --json | jq -r '
  "Config: \(.summary.config_path)",
  "Status: \(if .valid then "✅ Valid" else "❌ Invalid" end)",
  "Errors: \(.counts.errors)",
  "Warnings: \(.counts.warnings)",
  "Suggestions: \(.counts.suggestions)"
'
```

### Slack/Discord Notifications

```bash
#!/bin/bash
# notify-validation-failure.sh

VALIDATION_OUTPUT=$(phpeek-pm check-config --json 2>&1)
IS_VALID=$(echo "$VALIDATION_OUTPUT" | jq -r '.valid')

if [ "$IS_VALID" != "true" ]; then
  ERROR_COUNT=$(echo "$VALIDATION_OUTPUT" | jq -r '.counts.errors')
  WARNING_COUNT=$(echo "$VALIDATION_OUTPUT" | jq -r '.counts.warnings')

  MESSAGE="❌ Configuration validation failed!\n"
  MESSAGE+="Errors: $ERROR_COUNT, Warnings: $WARNING_COUNT\n"
  MESSAGE+="Please fix issues before deployment."

  # Send to Slack
  curl -X POST -H 'Content-type: application/json' \
    --data "{\"text\":\"$MESSAGE\"}" \
    "$SLACK_WEBHOOK_URL"
fi
```

## Troubleshooting

### False Positives

**Issue:** Validator flags valid configuration as error

**Solution:**
1. Check PHPeek PM version: `phpeek-pm --version`
2. Review error message for specific field
3. Consult documentation for correct format
4. Report bug if validator is wrong

### Missing Warnings

**Issue:** Configuration has issues but validator passes

**Solution:**
- Validator checks syntax, not runtime behavior
- Some issues only appear during execution
- Use `--strict` mode for stricter validation
- Enable debug logging: `log_level: debug`

### CI/CD Failures

**Issue:** Validation passes locally but fails in CI

**Solutions:**
1. **Version mismatch:**
   ```bash
   # Pin version in CI
   PHPEEK_VERSION=v1.0.0
   curl -L https://github.com/gophpeek/phpeek-pm/releases/download/$PHPEEK_VERSION/...
   ```

2. **Environment variables:**
   ```yaml
   # CI may not have env vars set
   api_auth: "${PHPEEK_PM_API_TOKEN:-default}"  # Use default in CI
   ```

3. **File paths:**
   ```yaml
   # Use CI-compatible paths
   api_socket: "${SOCKET_PATH:-/tmp/phpeek-pm.sock}"
   ```

## Best Practices

1. **Validate early and often:**
   - Run `check-config` before every deployment
   - Add to pre-commit hooks
   - Validate in CI/CD pipelines

2. **Use strict mode in CI:**
   - Enforces higher quality standards
   - Prevents warnings from accumulating
   - Catches issues early

3. **Fix warnings, not just errors:**
   - Warnings indicate sub-optimal configurations
   - May cause issues in production
   - Best addressed proactively

4. **Monitor suggestions:**
   - Track over time in JSON format
   - Use as improvement backlog
   - Prioritize based on environment (dev vs prod)

5. **Automate validation:**
   - Pre-commit hooks for developers
   - CI/CD for pull requests
   - Docker build for image validation

## Next Steps

- [Configuration Overview](overview.md) - Complete configuration reference
- [Environment Variables](environment-variables.md) - ENV var documentation
- [Troubleshooting](../guides/troubleshooting.md) - Common issues and solutions
- [CI/CD Integration](../guides/recipes.md#cicd-patterns) - Deployment patterns
