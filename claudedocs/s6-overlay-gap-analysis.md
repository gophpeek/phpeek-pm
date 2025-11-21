# s6-overlay Gap Analysis - Actual Status

## What We ALREADY Have ✅

### 1. Per-Process Signal Configuration ✅ FULLY IMPLEMENTED
**Config:** internal/config/types.go:83
```go
type ShutdownConfig struct {
    Signal string  // SIGTERM, SIGQUIT, etc.
}
```

**Implementation:** internal/process/supervisor.go:345-351
```go
sig := syscall.SIGTERM
if s.config.Shutdown != nil && s.config.Shutdown.Signal != "" {
    sig = parseSignal(s.config.Shutdown.Signal)
}
instance.cmd.Process.Signal(sig)
```

**Usage in Production:**
```yaml
processes:
  nginx:
    shutdown:
      signal: SIGQUIT  # Works NOW!
```

**Status:** ✅ NO GAP - Already works perfectly!

---

### 2. Readiness vs Liveness Mode ⚠️ PARTIAL
**Config EXISTS:** internal/config/types.go:78
```go
type HealthCheck struct {
    Mode string  // liveness | readiness | both
}
```

**Used in Examples:** configs/examples/laravel-full.yaml
```yaml
health_check:
  mode: both  # Config exists and is documented!
```

**BUT - Blocking Logic NOT Implemented:**
Searching manager.go Start() method (lines 76-102):
- Processes start in topological order
- NO waiting for health check success
- NO blocking dependent startup based on readiness mode

**Status:** ⚠️ CONFIG exists, BLOCKING logic missing

---

## What We Actually Need to Implement

### 1. Readiness Blocking Logic ⭐⭐⭐⭐⭐ CRITICAL
**Gap:** Config exists (`mode: readiness|both`) but blocking not implemented

**Current Behavior:**
```
PHP-FPM starts → Manager immediately starts Nginx
                 (even if PHP-FPM not ready!)
```

**Needed Behavior:**
```
PHP-FPM starts → Health check succeeds → THEN Nginx starts
                 (wait for readiness!)
```

**Implementation Needed:**
- manager.go: After starting process with `mode: readiness|both`
- Wait for first successful health check before starting dependents
- Timeout if health check never succeeds
- Only applies to processes with `depends_on`

**Effort:** 1-2 days

---

### 2. Oneshot Service Type ⭐⭐⭐⭐ IMPORTANT
**Gap:** Not implemented at all

**Current Workaround:** Use `restart: never` + hooks

**Needed:**
```yaml
processes:
  migrations:
    type: oneshot  # NEW field
    command: ["php", "artisan", "migrate", "--force"]

  php-fpm:
    type: longrun  # Default
    depends_on: [migrations]  # Waits for oneshot completion
```

**Implementation Needed:**
- Add `Type string` to config.Process
- Create oneshot supervisor logic
- Track "completed" vs "stopped" state
- Validate oneshot can't have `restart: always`

**Effort:** 2-3 days

---

### 3. Read-Only Root Filesystem ⭐⭐⭐ SECURITY
**Gap:** Not implemented or tested

**Needed:**
- Detect read-only root
- Use /run or /tmp for runtime state
- Skip permission changes on read-only FS

**Effort:** 1 day

---

## Revideret Plan

### Phase 1: Readiness Blocking (Days 1-2) - CRITICAL
Implement the missing blocking logic for `mode: readiness|both`

### Phase 2: Oneshot Services (Days 3-4) - IMPORTANT
Add `type: oneshot` support

### Phase 3: Read-Only Root (Day 5) - SECURITY
Support --read-only containers

**Total: 5 days (not 6-8, since signals already work!)**
