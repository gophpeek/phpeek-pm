# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PHPeek Process Manager (phpeek-pm) is a production-grade PID 1 process manager for Docker containers, designed specifically for Laravel applications. Written in Go, it manages multiple processes (PHP-FPM, Nginx, Horizon, queue workers) with proper signal handling, zombie reaping, health checks, and graceful shutdown.

**Current Status**: Phase 1 complete (core foundation with single process support). Multi-process orchestration, dependencies, health checks, and scaling features are planned but not yet implemented.

## Build & Development Commands

### Building
```bash
make build              # Build for current platform → build/phpeek-pm
make build-all          # Build for all platforms (Linux/macOS, AMD64/ARM64)
make dev                # Build and run locally
make clean              # Remove build artifacts
```

### Testing
```bash
make test               # Run all tests with race detection and coverage
go test -v ./internal/process  # Test specific package
go test -run TestName   # Run specific test
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

### Dependencies
```bash
make deps               # Download and tidy dependencies
go mod download         # Download modules
go mod tidy             # Clean up go.mod/go.sum
```

### Running
```bash
./build/phpeek-pm                              # Uses phpeek-pm.yaml or /etc/phpeek-pm/phpeek-pm.yaml
./build/phpeek-pm --config configs/examples/minimal.yaml
PHPEEK_PM_CONFIG=custom.yaml ./build/phpeek-pm
```

## Architecture

### Core Design Principles

1. **Interfaces over Concrete Types**: All major components define interfaces for testability
   - `ProcessManager` interface in `internal/process/manager.go`
   - Components should accept interfaces, return concrete structs

2. **Dependency Injection**: Pass dependencies explicitly through constructors
   - Logger is passed to all components via constructor
   - Config passed during initialization
   - No global state except logger (set via `slog.SetDefault()`)

3. **Error Wrapping**: Use `fmt.Errorf("context: %w", err)` for error chains
   - Provides stack trace context
   - Allows error unwrapping with `errors.Is()` and `errors.As()`

4. **Context Propagation**: Pass `context.Context` as first parameter for cancellation/timeouts
   - All blocking operations accept context
   - Respect context cancellation in goroutines

5. **Graceful Degradation**: Non-critical failures log warnings, don't crash
   - Only fatal errors should cause shutdown
   - Use appropriate log levels (debug/info/warn/error)

### Directory Structure

```
cmd/phpeek-pm/          # Main entry point
internal/
  config/               # Configuration loading and validation
    config.go          # YAML + env var loading with precedence
    types.go           # Config structs (Config, Process, HealthCheck, etc.)
  logger/              # Structured logging (slog)
    logger.go          # Logger initialization
    process_writer.go  # Per-process log segmentation
  process/             # Process management core
    manager.go         # Multi-process orchestration
    supervisor.go      # Single process lifecycle management
  signals/             # Signal handling and zombie reaping
    handler.go         # PID 1 signal handling
configs/examples/      # Example configurations
  minimal.yaml         # Simple PHP-FPM setup
  laravel-full.yaml    # Complete Laravel stack
```

### Key Components

#### Configuration System (`internal/config/`)
- **Load Priority**: Environment variables → YAML file → Defaults
- **Environment Variables**: `PHPEEK_PM_GLOBAL_LOG_LEVEL`, `PHPEEK_PM_PROCESS_<NAME>_ENABLED`
- **Validation**: Checks for circular dependencies, required fields, valid values
- **Defaults**: Applied in `SetDefaults()` method

#### Process Management (`internal/process/`)
- **Manager**: Orchestrates multiple processes with startup/shutdown ordering
  - `getStartupOrder()`: Priority-based topological sort (Phase 1: simple priority, Phase 2+: DAG with dependencies)
  - `getShutdownOrder()`: Reverse of startup order
  - Parallel shutdown with error collection

- **Supervisor**: Manages lifecycle of single process with scaling
  - Creates multiple instances based on `Scale` config
  - Handles restart policies (always/on-failure/never)
  - Per-process logging with structured fields

#### Signal Handling (`internal/signals/`)
- **PID 1 Capability**: Proper zombie reaping with `signals.ReapZombies()`
- **Signal Handling**: SIGTERM, SIGINT, SIGQUIT trigger graceful shutdown
- Critical for Docker containers running as PID 1

#### Logging (`internal/logger/`)
- **Structured Logging**: Uses Go's `log/slog` for JSON/text output
- **Log Levels**: debug, info, warn, error
- **Process Segmentation**: Per-process log labels for filtering

## Code Patterns & Standards

### Good Pattern Examples

```go
// ✅ GOOD: Interface + DI + Error wrapping + Context
type ProcessManager interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type manager struct {
    config  *config.Config
    logger  *slog.Logger
    procs   map[string]Process
}

func NewManager(cfg *config.Config, log *slog.Logger) ProcessManager {
    return &manager{
        config: cfg,
        logger: log,
        procs:  make(map[string]Process),
    }
}

func (m *manager) Start(ctx context.Context) error {
    if err := m.validateConfig(); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }
    // ... implementation
    return nil
}
```

### Anti-Patterns to Avoid

```go
// ❌ BAD: No interface, globals, no error handling, no context
var Procs map[string]*Process

func StartAll() {
    for _, p := range Procs {
        p.Start()  // No error handling, no context
    }
}

// ❌ BAD: No error wrapping
if err != nil {
    return err  // Lost context about where error occurred
}

// ❌ BAD: Shared state without mutex
func (m *Manager) GetProcess(name string) *Process {
    return m.processes[name]  // Race condition without m.mu.RLock()
}
```

### Testing Standards

```go
// Unit test pattern
func TestSupervisor_Start(t *testing.T) {
    tests := []struct {
        name    string
        config  *config.Process
        wantErr bool
    }{
        {
            name: "successful start",
            config: &config.Process{
                Command: []string{"sleep", "1"},
                Scale:   1,
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            logger := slog.Default()
            sup := NewSupervisor("test", tt.config, logger)

            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()

            err := sup.Start(ctx)
            if (err != nil) != tt.wantErr {
                t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
            }

            // Cleanup
            if err == nil {
                sup.Stop(ctx)
            }
        })
    }
}
```

### Go Idioms to Follow

1. **Accept interfaces, return structs** - Functions take interfaces, return concrete types
2. **Make zero values useful** - `&Manager{}` should be safe (use `New*()` constructors)
3. **Check every error** - Never ignore errors, wrap with context
4. **Defer cleanup** - Use `defer` for locks, file closes, cancellations
5. **Channels for communication** - Prefer channels over shared memory
6. **Goroutine lifecycle** - Always have clear termination via context or close

## Implementation Roadmap

### Phase 1: ✅ Complete
- Core process manager with single process support
- Configuration via YAML + environment variables
- Structured logging (JSON/text)
- PID 1 signal handling and zombie reaping
- Graceful shutdown with timeouts
- Priority-based startup ordering

### Phase 2: Planned (See IMPLEMENT.md)
- DAG-based dependency resolution (`depends_on` support)
- Multi-process orchestration with topological sort
- Health checks (TCP, HTTP, exec)
- Restart policies with exponential backoff
- Lifecycle hooks (pre/post start/stop)

### Phase 3-6: Future
- Dynamic scaling API
- Prometheus metrics
- Management REST API
- Production hardening

## Configuration Examples

### Minimal PHP-FPM
```yaml
version: "1.0"
global:
  shutdown_timeout: 30
  log_level: info

processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    priority: 10
    restart: always
```

### Laravel Full Stack
```yaml
processes:
  php-fpm:
    enabled: true
    command: ["php-fpm", "-F", "-R"]
    priority: 10
    depends_on: []

  nginx:
    enabled: true
    command: ["nginx", "-g", "daemon off;"]
    priority: 20
    depends_on: [php-fpm]

  horizon:
    enabled: true
    command: ["php", "artisan", "horizon"]
    priority: 30
    shutdown:
      pre_stop_hook:
        command: ["php", "artisan", "horizon:terminate"]
        timeout: 60

  queue-default:
    enabled: true
    command: ["php", "artisan", "queue:work", "--tries=3"]
    scale: 3
    priority: 40
```

## Important Notes

- **Phase 1 Limitations**: `depends_on` is validated but not enforced in startup order yet (uses simple priority)
- **Health Checks**: Config structs exist but health check execution is not implemented
- **Scaling**: Multiple instances launch but no runtime scaling API yet
- **Metrics/API**: Config exists but servers not implemented
- **Hooks**: Config exists but hook execution not implemented (planned for Phase 3)

## When Adding Features

1. **Define interfaces first** - Create testable contracts
2. **Add config structs** - Update `internal/config/types.go`
3. **Add validation** - Update `config.Validate()`
4. **Add defaults** - Update `config.SetDefaults()`
5. **Write tests** - Aim for >80% coverage
6. **Update examples** - Add to `configs/examples/`
7. **Document in IMPLEMENT.md** - Follow the implementation patterns there