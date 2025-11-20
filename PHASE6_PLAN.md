# Phase 6: Production Hardening & Resource Management

**Goal**: Make PHPeek PM production-ready with resource limits, advanced monitoring, and reliability features.

## Architecture Overview

### 1. Resource Management (`internal/resources/`)

**Purpose**: Control CPU, memory, and I/O usage per process using Linux cgroups v2

**Components**:
- `internal/resources/manager.go` - Resource limit manager
- `internal/resources/cgroup.go` - cgroups v2 interface
- `internal/resources/monitor.go` - Resource usage monitoring

**Features**:
```yaml
processes:
  php-fpm:
    resources:
      limits:
        cpu: "2.0"          # 2 CPU cores
        memory: "2Gi"       # 2GB RAM
        memory_swap: "2Gi"  # Additional swap
        pids: 512           # Max PIDs (prevents fork bombs)
      reservations:
        cpu: "0.5"          # Guaranteed CPU
        memory: "512Mi"     # Guaranteed memory
```

**Implementation**:
1. Check if cgroups v2 is available (`/sys/fs/cgroup/cgroup.controllers`)
2. Create cgroup per process: `/sys/fs/cgroup/phpeek-pm/<process-name>`
3. Write limits to cgroup files
4. Add process PIDs to `cgroup.procs`
5. Monitor resource usage via `memory.current`, `cpu.stat`

### 2. Advanced Logging (`internal/logger/advanced.go`)

**Purpose**: Production-grade logging with rotation, compression, and multiple outputs

**Features**:
```yaml
global:
  logging:
    outputs:
      - type: stdout
        format: json
        level: info
      - type: file
        path: /var/log/phpeek-pm/phpeek.log
        format: json
        level: debug
        rotation:
          max_size: 100Mi
          max_age: 30d
          max_backups: 10
          compress: true
      - type: syslog
        address: "localhost:514"
        facility: daemon
        level: warning
```

**Implementation**:
- Use `lumberjack` for file rotation
- Support multiple simultaneous outputs
- Per-process log segmentation to separate files
- Structured fields for filtering and searching

### 3. Graceful Degradation

**Purpose**: Handle failures without cascading crashes

**Features**:
- Circuit breaker for health checks (prevent health check storms)
- Exponential backoff for failed operations
- Fallback behaviors for non-critical services
- Automatic recovery detection

**Implementation**:
```go
type CircuitBreaker struct {
    maxFailures   int
    resetTimeout  time.Duration
    state         State // Closed, Open, HalfOpen
    failures      int
    lastFailTime  time.Time
}
```

### 4. Rate Limiting (`internal/api/ratelimit.go`)

**Purpose**: Protect API from abuse

**Features**:
```yaml
global:
  api_rate_limit:
    requests_per_second: 100
    burst: 200
    per_client: true
```

**Implementation**:
- Token bucket algorithm using `golang.org/x/time/rate`
- Per-client rate limiting by IP or auth token
- Custom rate limits per endpoint
- Metrics for rate limit hits

### 5. Process Resource Monitoring

**Purpose**: Track actual resource usage and expose via metrics

**Features**:
- CPU usage (user + system time)
- Memory usage (RSS, VSZ, swap)
- I/O statistics (bytes read/written)
- Network statistics (if available)
- File descriptor count
- Thread count

**Metrics**:
```
phpeek_pm_process_cpu_seconds_total
phpeek_pm_process_memory_bytes
phpeek_pm_process_io_read_bytes_total
phpeek_pm_process_io_write_bytes_total
phpeek_pm_process_fds_open
phpeek_pm_process_threads
```

**Implementation**:
- Read from `/proc/<pid>/stat` for CPU/memory
- Read from `/proc/<pid>/io` for I/O stats
- Read from `/proc/<pid>/fd/` for file descriptors

## Configuration Schema Updates

Add to `internal/config/types.go`:

```go
type ResourceLimits struct {
    CPU          string `yaml:"cpu"`           // "2.0" = 2 cores
    Memory       string `yaml:"memory"`        // "2Gi", "512Mi"
    MemorySwap   string `yaml:"memory_swap"`   // Total memory + swap
    PIDs         int    `yaml:"pids"`          // Max number of PIDs
}

type ResourceReservations struct {
    CPU    string `yaml:"cpu"`     // Guaranteed CPU
    Memory string `yaml:"memory"`  // Guaranteed memory
}

type Resources struct {
    Limits       *ResourceLimits       `yaml:"limits"`
    Reservations *ResourceReservations `yaml:"reservations"`
}

type LoggingOutput struct {
    Type     string         `yaml:"type"`      // stdout, file, syslog
    Format   string         `yaml:"format"`    // json, text
    Level    string         `yaml:"level"`     // debug, info, warn, error
    Path     string         `yaml:"path"`      // for file type
    Rotation *LogRotation   `yaml:"rotation"`  // for file type
    Address  string         `yaml:"address"`   // for syslog type
    Facility string         `yaml:"facility"`  // for syslog type
}

type LogRotation struct {
    MaxSize    string `yaml:"max_size"`     // 100Mi
    MaxAge     string `yaml:"max_age"`      // 30d
    MaxBackups int    `yaml:"max_backups"`  // 10
    Compress   bool   `yaml:"compress"`     // true
}

type Logging struct {
    Outputs []LoggingOutput `yaml:"outputs"`
}

type RateLimit struct {
    RequestsPerSecond int  `yaml:"requests_per_second"` // 100
    Burst             int  `yaml:"burst"`                // 200
    PerClient         bool `yaml:"per_client"`           // true
}

// Add to Process struct
type Process struct {
    // ... existing fields ...
    Resources *Resources `yaml:"resources"`
}

// Add to Global struct
type Global struct {
    // ... existing fields ...
    Logging      *Logging   `yaml:"logging"`
    APIRateLimit *RateLimit `yaml:"api_rate_limit"`
}
```

## Implementation Priority

### High Priority (Core Production Features)
1. **Resource Monitoring** - Essential for observability
2. **Advanced Logging** - File rotation prevents disk exhaustion
3. **Rate Limiting** - Protects API from abuse

### Medium Priority (Reliability Features)
4. **Resource Limits** - cgroups v2 for memory/CPU limits
5. **Circuit Breakers** - Prevent health check storms

### Low Priority (Nice to Have)
6. **Syslog Output** - Can be added later if needed
7. **Multi-output Logging** - Start with file + stdout

## Testing Strategy

### Unit Tests
- Resource limit parsing and validation
- Circuit breaker state transitions
- Rate limiter token bucket behavior
- Log rotation triggers

### Integration Tests
- cgroups creation and cleanup
- Resource usage collection from /proc
- File rotation with real files
- Rate limiting with concurrent requests

### Performance Tests
- Overhead of resource monitoring
- Log rotation performance
- Rate limiter throughput

## Graceful Fallbacks

All Phase 6 features must degrade gracefully:

1. **cgroups unavailable**: Log warning, continue without limits
2. **/proc not readable**: Log warning, skip resource metrics
3. **File rotation fails**: Fall back to stdout logging
4. **Rate limiter error**: Allow requests through (fail-open)

## Metrics to Add

```
# Resource limits
phpeek_pm_resource_limit_cpu_cores
phpeek_pm_resource_limit_memory_bytes

# Resource usage
phpeek_pm_process_cpu_seconds_total
phpeek_pm_process_memory_bytes{type="rss|vsz|swap"}
phpeek_pm_process_io_read_bytes_total
phpeek_pm_process_io_write_bytes_total
phpeek_pm_process_fds_open
phpeek_pm_process_threads

# Circuit breakers
phpeek_pm_circuit_breaker_state{name="healthcheck"}
phpeek_pm_circuit_breaker_failures_total

# Rate limiting
phpeek_pm_api_rate_limit_requests_total{allowed="true|false"}
```

## Documentation Updates

1. Add resource limits examples to `configs/examples/`
2. Update `docs/configuration/resources.md`
3. Add metrics documentation to `docs/observability/metrics.md`
4. Add production best practices guide

## Timeline Estimate

- Resource Monitoring: 3-4 hours
- Advanced Logging: 2-3 hours
- Rate Limiting: 1-2 hours
- Resource Limits (cgroups): 3-4 hours
- Circuit Breakers: 1-2 hours
- Testing: 2-3 hours
- Documentation: 1-2 hours

**Total**: 13-20 hours

## Success Criteria

✅ Resource usage monitoring with Prometheus metrics
✅ File rotation prevents disk exhaustion
✅ API rate limiting protects against abuse
✅ cgroups v2 limits prevent resource exhaustion
✅ Graceful degradation when features unavailable
✅ Comprehensive tests (>80% coverage)
✅ Production-ready example configurations
✅ Complete documentation

## Notes

- cgroups v2 requires Linux kernel 4.15+
- Some features are Linux-specific (will gracefully skip on macOS/Windows)
- All features must be optional and backward compatible
- Performance overhead must be minimal (<5%)
