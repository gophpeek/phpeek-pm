# PHPeek PM Implementation Guide

Complete implementation roadmap for Goland development with architectural decisions and code patterns.

## 🎯 Architecture Principles

### Core Design Philosophy
1. **Interfaces over Concrete Types**: All major components define interfaces for testability
2. **Dependency Injection**: Pass dependencies explicitly, avoid globals except logger
3. **Error Wrapping**: Use `fmt.Errorf("context: %w", err)` for error chains
4. **Graceful Degradation**: Non-critical failures log warnings, don't crash
5. **Context Propagation**: Pass `context.Context` for cancellation and timeouts
6. **Self-Contained**: PHPeek PM handles all initialization as PID 1 (no external bash setup)

### Container Integration Philosophy

PHPeek PM is designed to run as **PID 1** directly from Docker ENTRYPOINT with minimal bash wrapper:

```dockerfile
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
```

**docker-entrypoint.sh flow**:
```bash
if [ "$PHPEEK_PROCESS_MANAGER" = "phpeek-pm" ]; then
    exec /usr/local/bin/phpeek-pm --config /etc/phpeek-pm/phpeek-pm.yaml
fi
# ... else default mode
```

**PHPeek PM responsibilities** (all in Go):
- ✅ Framework detection (Laravel, Symfony, WordPress)
- ✅ Environment variable substitution in config
- ✅ Permission setup (storage/, bootstrap/cache/)
- ✅ Config validation (php-fpm -t, nginx -t)
- ✅ Pre-start hooks execution
- ✅ Process lifecycle management
- ✅ Signal handling as PID 1
- ✅ Zombie reaping

**Bash wrapper responsibilities** (minimal):
- Check `PHPEEK_PROCESS_MANAGER` variable
- Exec to phpeek-pm binary (becomes PID 1)
- Fallback to default mode if not phpeek-pm

### Code Style Standards
```go
// ✅ GOOD: Explicit, testable, follows Go idioms
type ProcessManager interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Scale(name string, replicas int) error
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

// ❌ BAD: No interface, hard to test, globals
var Procs map[string]*Process

func StartAll() {
    for _, p := range Procs {
        p.Start()  // No error handling, no context
    }
}
```

## 📋 Phase 1.5: Container Integration (Before Phase 2)

Before implementing Phase 2 features, PHPeek PM needs to handle container initialization that bash currently can't do as PID 1.

### Priority 0: Framework Detection

**File**: `internal/framework/detector.go`

```go
package framework

import (
    "os"
    "path/filepath"
)

type Framework string

const (
    Laravel   Framework = "laravel"
    Symfony   Framework = "symfony"
    WordPress Framework = "wordpress"
    Generic   Framework = "generic"
)

// Detect identifies the PHP framework in the given directory
func Detect(dir string) Framework {
    // Laravel: check for artisan file
    if fileExists(filepath.Join(dir, "artisan")) {
        return Laravel
    }

    // Symfony: check for bin/console and var/cache
    if fileExists(filepath.Join(dir, "bin", "console")) &&
       dirExists(filepath.Join(dir, "var", "cache")) {
        return Symfony
    }

    // WordPress: check for wp-config.php
    if fileExists(filepath.Join(dir, "wp-config.php")) {
        return WordPress
    }

    return Generic
}

func fileExists(path string) bool {
    info, err := os.Stat(path)
    return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
    info, err := os.Stat(path)
    return err == nil && info.IsDir()
}
```

### Priority 1: Permission Setup

**File**: `internal/setup/permissions.go`

```go
package setup

import (
    "fmt"
    "log/slog"
    "os"
    "path/filepath"

    "github.com/gophpeek/phpeek-pm/internal/framework"
)

type PermissionManager struct {
    logger *slog.Logger
    workdir string
    framework framework.Framework
}

func NewPermissionManager(workdir string, fw framework.Framework, log *slog.Logger) *PermissionManager {
    return &PermissionManager{
        logger: log,
        workdir: workdir,
        framework: fw,
    }
}

// Setup creates necessary directories and sets permissions
func (pm *PermissionManager) Setup() error {
    pm.logger.Info("Setting up permissions", "framework", pm.framework)

    switch pm.framework {
    case framework.Laravel:
        return pm.setupLaravel()
    case framework.Symfony:
        return pm.setupSymfony()
    case framework.WordPress:
        return pm.setupWordPress()
    default:
        pm.logger.Debug("Generic framework, skipping permission setup")
        return nil
    }
}

func (pm *PermissionManager) setupLaravel() error {
    dirs := []string{
        filepath.Join(pm.workdir, "storage", "framework", "sessions"),
        filepath.Join(pm.workdir, "storage", "framework", "views"),
        filepath.Join(pm.workdir, "storage", "framework", "cache"),
        filepath.Join(pm.workdir, "storage", "logs"),
        filepath.Join(pm.workdir, "bootstrap", "cache"),
    }

    for _, dir := range dirs {
        if err := pm.createDir(dir, 0775); err != nil {
            pm.logger.Warn("Failed to create directory", "dir", dir, "error", err)
        }
    }

    // Set ownership (www-data UID/GID typically 82 on Alpine, 33 on Debian)
    pm.chownRecursive(filepath.Join(pm.workdir, "storage"), 82, 82)
    pm.chownRecursive(filepath.Join(pm.workdir, "bootstrap", "cache"), 82, 82)

    return nil
}

func (pm *PermissionManager) setupSymfony() error {
    dirs := []string{
        filepath.Join(pm.workdir, "var", "cache"),
        filepath.Join(pm.workdir, "var", "log"),
    }

    for _, dir := range dirs {
        if err := pm.createDir(dir, 0775); err != nil {
            pm.logger.Warn("Failed to create directory", "dir", dir, "error", err)
        }
    }

    pm.chownRecursive(filepath.Join(pm.workdir, "var"), 82, 82)
    return nil
}

func (pm *PermissionManager) setupWordPress() error {
    dir := filepath.Join(pm.workdir, "wp-content", "uploads")
    if err := pm.createDir(dir, 0775); err != nil {
        pm.logger.Warn("Failed to create uploads directory", "error", err)
    }

    pm.chownRecursive(filepath.Join(pm.workdir, "wp-content"), 82, 82)
    return nil
}

func (pm *PermissionManager) createDir(path string, perm os.FileMode) error {
    return os.MkdirAll(path, perm)
}

func (pm *PermissionManager) chownRecursive(path string, uid, gid int) {
    // Note: This will fail silently if not running as root
    // That's acceptable - in dev environments permissions may not matter
    filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
        if err == nil {
            os.Chown(name, uid, gid)
        }
        return nil
    })
}
```

### Priority 2: Config Validation

**File**: `internal/setup/validator.go`

```go
package setup

import (
    "fmt"
    "log/slog"
    "os/exec"
    "strings"
)

type ConfigValidator struct {
    logger *slog.Logger
}

func NewConfigValidator(log *slog.Logger) *ConfigValidator {
    return &ConfigValidator{logger: log}
}

// ValidateAll validates PHP-FPM and Nginx configurations
func (cv *ConfigValidator) ValidateAll() error {
    cv.logger.Info("Validating configurations...")

    if err := cv.ValidatePHPFPM(); err != nil {
        return fmt.Errorf("PHP-FPM config invalid: %w", err)
    }
    cv.logger.Debug("PHP-FPM config valid")

    if err := cv.ValidateNginx(); err != nil {
        return fmt.Errorf("Nginx config invalid: %w", err)
    }
    cv.logger.Debug("Nginx config valid")

    cv.logger.Info("All configurations valid")
    return nil
}

func (cv *ConfigValidator) ValidatePHPFPM() error {
    cmd := exec.Command("php-fpm", "-t")
    output, err := cmd.CombinedOutput()

    if err != nil {
        return fmt.Errorf("%w: %s", err, string(output))
    }

    // Check for success message
    if !strings.Contains(string(output), "test is successful") {
        return fmt.Errorf("unexpected output: %s", string(output))
    }

    return nil
}

func (cv *ConfigValidator) ValidateNginx() error {
    cmd := exec.Command("nginx", "-t")
    output, err := cmd.CombinedOutput()

    if err != nil {
        return fmt.Errorf("%w: %s", err, string(output))
    }

    // Check for success messages
    outputStr := string(output)
    if !strings.Contains(outputStr, "syntax is ok") ||
       !strings.Contains(outputStr, "test is successful") {
        return fmt.Errorf("unexpected output: %s", outputStr)
    }

    return nil
}
```

### Priority 3: Environment Variable Config Loading

**File**: `internal/config/envsubst.go`

```go
package config

import (
    "fmt"
    "os"
    "regexp"
    "strings"
)

// ExpandEnv expands environment variables in config content
// Supports ${VAR:-default} syntax
func ExpandEnv(content string) string {
    // Pattern: ${VAR:-default} or ${VAR}
    pattern := regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

    return pattern.ReplaceAllStringFunc(content, func(match string) string {
        parts := pattern.FindStringSubmatch(match)
        if len(parts) < 2 {
            return match
        }

        varName := parts[1]
        defaultValue := ""
        if len(parts) >= 3 {
            defaultValue = parts[2]
        }

        // Get from environment or use default
        if value := os.Getenv(varName); value != "" {
            return value
        }

        return defaultValue
    })
}

// LoadWithEnvExpansion loads config file and expands environment variables
func LoadWithEnvExpansion(path string) (*Config, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }

    // Expand environment variables
    expanded := ExpandEnv(string(content))

    // Parse YAML
    var cfg Config
    if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }

    return &cfg, nil
}
```

### Integration in main.go

Update `cmd/phpeek-pm/main.go` to include initialization:

```go
func main() {
    // ... existing banner and config load

    // Phase 1.5: Container initialization
    workdir := os.Getenv("WORKDIR")
    if workdir == "" {
        workdir = "/var/www/html"
    }

    // Detect framework
    fw := framework.Detect(workdir)
    slog.Info("Detected framework", "framework", fw, "workdir", workdir)

    // Setup permissions
    permMgr := setup.NewPermissionManager(workdir, fw, log)
    if err := permMgr.Setup(); err != nil {
        slog.Warn("Permission setup completed with warnings", "error", err)
    }

    // Validate configurations
    validator := setup.NewConfigValidator(log)
    if err := validator.ValidateAll(); err != nil {
        slog.Error("Configuration validation failed", "error", err)
        os.Exit(1)
    }

    // Load config with environment expansion
    cfg, err := config.LoadWithEnvExpansion("/etc/phpeek-pm/phpeek-pm.yaml")
    if err != nil {
        slog.Error("Failed to load configuration", "error", err)
        os.Exit(1)
    }

    // ... continue with existing process management
}
```

## 📋 Phase 2: Multi-Process Dependencies & Health Checks

### Priority 1: DAG Dependency Resolver

**File**: `internal/dag/resolver.go`

```go
package dag

import (
    "fmt"
    "github.com/gophpeek/phpeek-pm/internal/config"
)

// Graph represents a process dependency graph
type Graph struct {
    nodes map[string]*Node
}

// Node represents a process in the dependency graph
type Node struct {
    Name         string
    Priority     int
    Dependencies []string
    Visited      bool
    InStack      bool
}

// NewGraph creates a dependency graph from process configs
func NewGraph(processes map[string]*config.Process) (*Graph, error) {
    g := &Graph{nodes: make(map[string]*Node)}

    // Build nodes
    for name, proc := range processes {
        if !proc.Enabled {
            continue
        }
        g.nodes[name] = &Node{
            Name:         name,
            Priority:     proc.Priority,
            Dependencies: proc.DependsOn,
        }
    }

    // Validate all dependencies exist
    for name, node := range g.nodes {
        for _, dep := range node.Dependencies {
            if _, exists := g.nodes[dep]; !exists {
                return nil, fmt.Errorf("process %s depends on non-existent process %s", name, dep)
            }
        }
    }

    return g, nil
}

// TopologicalSort returns processes in valid startup order
// Lower priority processes start first, then dependencies are respected
func (g *Graph) TopologicalSort() ([]string, error) {
    var result []string

    // Check for cycles first
    for name := range g.nodes {
        if !g.nodes[name].Visited {
            if g.hasCycle(name) {
                return nil, fmt.Errorf("circular dependency detected involving %s", name)
            }
        }
    }

    // Reset for actual traversal
    for _, node := range g.nodes {
        node.Visited = false
    }

    // DFS with priority consideration
    for name := range g.nodes {
        if !g.nodes[name].Visited {
            g.dfs(name, &result)
        }
    }

    return result, nil
}

func (g *Graph) hasCycle(name string) bool {
    node := g.nodes[name]
    node.Visited = true
    node.InStack = true

    for _, dep := range node.Dependencies {
        depNode := g.nodes[dep]
        if !depNode.Visited {
            if g.hasCycle(dep) {
                return true
            }
        } else if depNode.InStack {
            return true
        }
    }

    node.InStack = false
    return false
}

func (g *Graph) dfs(name string, result *[]string) {
    node := g.nodes[name]
    node.Visited = true

    // Visit dependencies first
    for _, dep := range node.Dependencies {
        if !g.nodes[dep].Visited {
            g.dfs(dep, result)
        }
    }

    *result = append(*result, name)
}
```

**Integration**: Update `process/manager.go`:
```go
func (m *Manager) getStartupOrder() ([]string, error) {
    graph, err := dag.NewGraph(m.config.Processes)
    if err != nil {
        return nil, fmt.Errorf("failed to build dependency graph: %w", err)
    }

    return graph.TopologicalSort()
}
```

### Priority 2: Health Check System

**File**: `internal/process/healthcheck.go`

```go
package process

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "os/exec"
    "time"

    "github.com/gophpeek/phpeek-pm/internal/config"
)

// HealthChecker defines the interface for health checks
type HealthChecker interface {
    Check(ctx context.Context) error
}

// NewHealthChecker creates appropriate health checker based on config
func NewHealthChecker(cfg *config.HealthCheck) (HealthChecker, error) {
    if cfg == nil {
        return &NoOpHealthChecker{}, nil
    }

    switch cfg.Type {
    case "tcp":
        return &TCPHealthChecker{address: cfg.Address}, nil
    case "http":
        return &HTTPHealthChecker{
            url:            cfg.URL,
            expectedStatus: cfg.ExpectedStatus,
        }, nil
    case "exec":
        return &ExecHealthChecker{command: cfg.Command}, nil
    default:
        return nil, fmt.Errorf("unknown health check type: %s", cfg.Type)
    }
}

// NoOpHealthChecker always succeeds (for processes without health checks)
type NoOpHealthChecker struct{}

func (n *NoOpHealthChecker) Check(ctx context.Context) error {
    return nil
}

// TCPHealthChecker checks if TCP port is accepting connections
type TCPHealthChecker struct {
    address string
}

func (t *TCPHealthChecker) Check(ctx context.Context) error {
    dialer := &net.Dialer{}
    conn, err := dialer.DialContext(ctx, "tcp", t.address)
    if err != nil {
        return fmt.Errorf("tcp connection failed: %w", err)
    }
    conn.Close()
    return nil
}

// HTTPHealthChecker performs HTTP GET and validates status code
type HTTPHealthChecker struct {
    url            string
    expectedStatus int
}

func (h *HTTPHealthChecker) Check(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, "GET", h.url, nil)
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("http request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != h.expectedStatus {
        return fmt.Errorf("unexpected status code: got %d, want %d", resp.StatusCode, h.expectedStatus)
    }

    return nil
}

// ExecHealthChecker runs a command and checks exit code
type ExecHealthChecker struct {
    command []string
}

func (e *ExecHealthChecker) Check(ctx context.Context) error {
    if len(e.command) == 0 {
        return fmt.Errorf("no command specified")
    }

    cmd := exec.CommandContext(ctx, e.command[0], e.command[1:]...)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("health check command failed: %w", err)
    }

    return nil
}

// HealthMonitor continuously monitors process health
type HealthMonitor struct {
    checker         HealthChecker
    config          *config.HealthCheck
    consecutiveFails int
    logger          *slog.Logger
}

func NewHealthMonitor(cfg *config.HealthCheck, log *slog.Logger) (*HealthMonitor, error) {
    checker, err := NewHealthChecker(cfg)
    if err != nil {
        return nil, err
    }

    return &HealthMonitor{
        checker: checker,
        config:  cfg,
        logger:  log,
    }, nil
}

// Start begins health check monitoring
func (hm *HealthMonitor) Start(ctx context.Context) <-chan HealthStatus {
    statusCh := make(chan HealthStatus, 1)

    go func() {
        defer close(statusCh)

        // Wait for initial delay
        if hm.config != nil && hm.config.InitialDelay > 0 {
            select {
            case <-time.After(time.Duration(hm.config.InitialDelay) * time.Second):
            case <-ctx.Done():
                return
            }
        }

        ticker := time.NewTicker(time.Duration(hm.config.Period) * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                status := hm.performCheck(ctx)
                statusCh <- status
            case <-ctx.Done():
                return
            }
        }
    }()

    return statusCh
}

func (hm *HealthMonitor) performCheck(ctx context.Context) HealthStatus {
    checkCtx, cancel := context.WithTimeout(ctx, time.Duration(hm.config.Timeout)*time.Second)
    defer cancel()

    err := hm.checker.Check(checkCtx)

    if err != nil {
        hm.consecutiveFails++
        hm.logger.Warn("Health check failed",
            "consecutive_fails", hm.consecutiveFails,
            "threshold", hm.config.FailureThreshold,
            "error", err,
        )

        if hm.consecutiveFails >= hm.config.FailureThreshold {
            return HealthStatus{Healthy: false, Error: err}
        }
        return HealthStatus{Healthy: true, Error: nil} // Not failed enough times yet
    }

    // Success - reset counter
    if hm.consecutiveFails > 0 {
        hm.logger.Info("Health check recovered",
            "previous_fails", hm.consecutiveFails,
        )
    }
    hm.consecutiveFails = 0
    return HealthStatus{Healthy: true, Error: nil}
}

type HealthStatus struct {
    Healthy bool
    Error   error
}
```

**Integration**: Update `process/supervisor.go`:
```go
type Supervisor struct {
    // ... existing fields
    healthMonitor *HealthMonitor
    healthStatus  chan HealthStatus
}

func (s *Supervisor) Start(ctx context.Context) error {
    // ... existing start logic

    // Start health monitoring
    if s.config.HealthCheck != nil {
        monitor, err := NewHealthMonitor(s.config.HealthCheck, s.logger)
        if err != nil {
            return fmt.Errorf("failed to create health monitor: %w", err)
        }

        s.healthMonitor = monitor
        s.healthStatus = monitor.Start(ctx)

        // Monitor health status in background
        go s.handleHealthStatus(ctx)
    }

    return nil
}

func (s *Supervisor) handleHealthStatus(ctx context.Context) {
    for {
        select {
        case status, ok := <-s.healthStatus:
            if !ok {
                return
            }

            if !status.Healthy {
                s.logger.Error("Process unhealthy, restarting",
                    "error", status.Error,
                )
                // TODO: Trigger restart based on restart policy
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### Priority 3: Restart Policies

**File**: `internal/process/restart.go`

```go
package process

import (
    "context"
    "time"
)

// RestartPolicy defines restart behavior
type RestartPolicy interface {
    ShouldRestart(exitCode int, restartCount int) bool
    BackoffDuration(restartCount int) time.Duration
}

type AlwaysRestartPolicy struct {
    maxAttempts int
    backoff     time.Duration
}

func (p *AlwaysRestartPolicy) ShouldRestart(exitCode int, restartCount int) bool {
    if p.maxAttempts <= 0 {
        return true
    }
    return restartCount < p.maxAttempts
}

func (p *AlwaysRestartPolicy) BackoffDuration(restartCount int) time.Duration {
    // Exponential backoff: backoff * 2^restartCount (capped at 5 minutes)
    duration := p.backoff * time.Duration(1<<uint(restartCount))
    if duration > 5*time.Minute {
        return 5 * time.Minute
    }
    return duration
}

type OnFailureRestartPolicy struct {
    maxAttempts int
    backoff     time.Duration
}

func (p *OnFailureRestartPolicy) ShouldRestart(exitCode int, restartCount int) bool {
    if exitCode == 0 {
        return false // Clean exit, don't restart
    }
    if p.maxAttempts <= 0 {
        return true
    }
    return restartCount < p.maxAttempts
}

func (p *OnFailureRestartPolicy) BackoffDuration(restartCount int) time.Duration {
    duration := p.backoff * time.Duration(1<<uint(restartCount))
    if duration > 5*time.Minute {
        return 5 * time.Minute
    }
    return duration
}

type NeverRestartPolicy struct{}

func (p *NeverRestartPolicy) ShouldRestart(exitCode int, restartCount int) bool {
    return false
}

func (p *NeverRestartPolicy) BackoffDuration(restartCount int) time.Duration {
    return 0
}

func NewRestartPolicy(policyType string, maxAttempts int, backoff time.Duration) RestartPolicy {
    switch policyType {
    case "always":
        return &AlwaysRestartPolicy{maxAttempts: maxAttempts, backoff: backoff}
    case "on-failure":
        return &OnFailureRestartPolicy{maxAttempts: maxAttempts, backoff: backoff}
    default:
        return &NeverRestartPolicy{}
    }
}
```

## 🔧 Phase 3: Lifecycle Hooks

### Hook Executor

**File**: `internal/hooks/executor.go`

```go
package hooks

import (
    "context"
    "fmt"
    "log/slog"
    "os/exec"
    "time"

    "github.com/gophpeek/phpeek-pm/internal/config"
)

type Executor struct {
    logger *slog.Logger
}

func NewExecutor(log *slog.Logger) *Executor {
    return &Executor{logger: log}
}

// Execute runs a single hook with retry logic
func (e *Executor) Execute(ctx context.Context, hook *config.Hook) error {
    e.logger.Info("Executing hook",
        "name", hook.Name,
        "command", hook.Command,
    )

    var lastErr error
    attempts := hook.Retry + 1 // Retry + initial attempt

    for attempt := 0; attempt < attempts; attempt++ {
        if attempt > 0 {
            e.logger.Info("Retrying hook",
                "name", hook.Name,
                "attempt", attempt+1,
                "max_attempts", attempts,
            )

            // Wait before retry
            if hook.RetryDelay > 0 {
                select {
                case <-time.After(time.Duration(hook.RetryDelay) * time.Second):
                case <-ctx.Done():
                    return ctx.Err()
                }
            }
        }

        err := e.executeOnce(ctx, hook)
        if err == nil {
            e.logger.Info("Hook completed successfully", "name", hook.Name)
            return nil
        }

        lastErr = err
        e.logger.Warn("Hook failed",
            "name", hook.Name,
            "attempt", attempt+1,
            "error", err,
        )
    }

    if hook.ContinueOnError {
        e.logger.Warn("Hook failed but continuing due to continue_on_error",
            "name", hook.Name,
            "error", lastErr,
        )
        return nil
    }

    return fmt.Errorf("hook %s failed after %d attempts: %w", hook.Name, attempts, lastErr)
}

func (e *Executor) executeOnce(ctx context.Context, hook *config.Hook) error {
    if len(hook.Command) == 0 {
        return fmt.Errorf("empty command")
    }

    // Create command with timeout
    cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(hook.Timeout)*time.Second)
    defer cancel()

    cmd := exec.CommandContext(cmdCtx, hook.Command[0], hook.Command[1:]...)

    // Set working directory
    if hook.WorkingDir != "" {
        cmd.Dir = hook.WorkingDir
    }

    // Add environment variables
    cmd.Env = append(cmd.Env, e.buildEnv(hook)...)

    // Capture output
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("command failed: %w (output: %s)", err, string(output))
    }

    return nil
}

func (e *Executor) buildEnv(hook *config.Hook) []string {
    env := []string{
        fmt.Sprintf("PHPEEK_PM_HOOK_NAME=%s", hook.Name),
    }

    for key, value := range hook.Env {
        env = append(env, fmt.Sprintf("%s=%s", key, value))
    }

    return env
}

// ExecuteSequence runs multiple hooks in order
func (e *Executor) ExecuteSequence(ctx context.Context, hooks []config.Hook) error {
    for _, hook := range hooks {
        if err := e.Execute(ctx, &hook); err != nil {
            return fmt.Errorf("failed to execute hook %s: %w", hook.Name, err)
        }
    }
    return nil
}
```

**Integration**: Update `process/manager.go`:
```go
import "github.com/gophpeek/phpeek-pm/internal/hooks"

func (m *Manager) Start(ctx context.Context) error {
    // Execute pre-start hooks
    if len(m.config.Hooks.PreStart) > 0 {
        m.logger.Info("Executing pre-start hooks", "count", len(m.config.Hooks.PreStart))
        executor := hooks.NewExecutor(m.logger)
        if err := executor.ExecuteSequence(ctx, m.config.Hooks.PreStart); err != nil {
            return fmt.Errorf("pre-start hooks failed: %w", err)
        }
    }

    // ... existing start logic

    // Execute post-start hooks
    if len(m.config.Hooks.PostStart) > 0 {
        m.logger.Info("Executing post-start hooks", "count", len(m.config.Hooks.PostStart))
        executor := hooks.NewExecutor(m.logger)
        if err := executor.ExecuteSequence(ctx, m.config.Hooks.PostStart); err != nil {
            // Post-start failures are warnings, not fatal
            m.logger.Warn("Post-start hooks failed", "error", err)
        }
    }

    return nil
}
```

## 📊 Phase 4: Prometheus Metrics

### Metrics Collector

**File**: `internal/metrics/collector.go`

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Process metrics
    ProcessUp = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "phpeek_pm_process_up",
            Help: "Process status (1=running, 0=stopped)",
        },
        []string{"name", "instance"},
    )

    ProcessRestarts = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "phpeek_pm_process_restarts_total",
            Help: "Total number of process restarts",
        },
        []string{"name"},
    )

    ProcessCPU = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "phpeek_pm_process_cpu_seconds_total",
            Help: "Total CPU time consumed by process",
        },
        []string{"name", "instance"},
    )

    ProcessMemory = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "phpeek_pm_process_memory_bytes",
            Help: "Current memory usage in bytes",
        },
        []string{"name", "instance", "type"}, // type: rss, vms
    )

    // Health check metrics
    HealthCheckStatus = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "phpeek_pm_health_check_status",
            Help: "Health check status (1=healthy, 0=unhealthy)",
        },
        []string{"name", "type"},
    )

    HealthCheckDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "phpeek_pm_health_check_duration_seconds",
            Help: "Health check duration in seconds",
            Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
        },
        []string{"name", "type"},
    )

    // Scaling metrics
    ProcessDesiredScale = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "phpeek_pm_process_desired_scale",
            Help: "Desired number of process instances",
        },
        []string{"name"},
    )

    ProcessCurrentScale = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "phpeek_pm_process_current_scale",
            Help: "Current number of running instances",
        },
        []string{"name"},
    )
)
```

**File**: `internal/metrics/server.go`

```go
package metrics

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"

    "github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
    port   int
    path   string
    server *http.Server
    logger *slog.Logger
}

func NewServer(port int, path string, log *slog.Logger) *Server {
    return &Server{
        port:   port,
        path:   path,
        logger: log,
    }
}

func (s *Server) Start(ctx context.Context) error {
    mux := http.NewServeMux()
    mux.Handle(s.path, promhttp.Handler())

    s.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", s.port),
        Handler: mux,
    }

    s.logger.Info("Starting metrics server",
        "port", s.port,
        "path", s.path,
    )

    go func() {
        if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            s.logger.Error("Metrics server failed", "error", err)
        }
    }()

    return nil
}

func (s *Server) Stop(ctx context.Context) error {
    if s.server != nil {
        return s.server.Shutdown(ctx)
    }
    return nil
}
```

## 🌐 Phase 5: Management API

### API Server Structure

**File**: `internal/api/server.go`

```go
package api

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
    "strings"

    "github.com/gophpeek/phpeek-pm/internal/process"
)

type Server struct {
    port    int
    auth    string
    manager process.Manager
    server  *http.Server
    logger  *slog.Logger
}

func NewServer(port int, auth string, mgr process.Manager, log *slog.Logger) *Server {
    return &Server{
        port:    port,
        auth:    auth,
        manager: mgr,
        logger:  log,
    }
}

func (s *Server) Start(ctx context.Context) error {
    mux := http.NewServeMux()

    // Health endpoint
    mux.HandleFunc("/health", s.handleHealth)

    // Process endpoints
    mux.HandleFunc("/api/v1/processes", s.authMiddleware(s.handleProcesses))
    mux.HandleFunc("/api/v1/processes/", s.authMiddleware(s.handleProcessDetail))

    s.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", s.port),
        Handler: mux,
    }

    s.logger.Info("Starting API server", "port", s.port)

    go func() {
        if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            s.logger.Error("API server failed", "error", err)
        }
    }()

    return nil
}

func (s *Server) Stop(ctx context.Context) error {
    if s.server != nil {
        return s.server.Shutdown(ctx)
    }
    return nil
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if s.auth == "" {
            next(w, r)
            return
        }

        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != s.auth {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        next(w, r)
    }
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]string{
        "status":  "healthy",
        "version": "1.0.0",
    })
}
```

## 🧪 Testing Standards

### Unit Test Pattern
```go
// internal/process/supervisor_test.go
package process

import (
    "context"
    "testing"
    "time"

    "github.com/gophpeek/phpeek-pm/internal/config"
    "log/slog"
)

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
        {
            name: "invalid command",
            config: &config.Process{
                Command: []string{"nonexistent-command"},
                Scale:   1,
            },
            wantErr: true,
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

### Integration Test Pattern
```go
// internal/process/integration_test.go
// +build integration

package process

import (
    "context"
    "testing"
    "time"
)

func TestManager_FullLifecycle(t *testing.T) {
    // This test requires Docker or real processes
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Test complete lifecycle: start, health check, graceful shutdown
}
```

## ✅ Definition of Done Checklist

For each feature to be considered "complete":

- [ ] **Code**: Implementation follows patterns above
- [ ] **Interfaces**: All major components expose interfaces
- [ ] **Tests**: Unit tests with >80% coverage
- [ ] **Errors**: Proper error wrapping with context
- [ ] **Logging**: Structured logs at appropriate levels
- [ ] **Documentation**: GoDoc comments for public functions
- [ ] **Examples**: Config examples in `configs/examples/`
- [ ] **Integration**: Works with existing Phase 1 code
- [ ] **Performance**: No memory leaks, reasonable CPU usage
- [ ] **Metrics**: Prometheus metrics where applicable

## 🚀 Quick Development Commands

```bash
# Run tests
make test

# Build binary
make build

# Run with example config
./build/phpeek-pm --config configs/examples/minimal.yaml

# Check coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint
golangci-lint run

# Format code
gofmt -w .
```

## 📚 Key Go Idioms to Follow

1. **Accept interfaces, return structs**
2. **Make zero values useful** (e.g., `&Manager{}` should be safe)
3. **Error handling**: Check every error, wrap with context
4. **Defer cleanup**: Use `defer` for resource cleanup
5. **Context propagation**: Pass `context.Context` as first parameter
6. **Channels for communication**: Prefer channels over shared memory
7. **Goroutine lifecycle**: Always have clear termination conditions

## 🎯 Next Implementation Priority

### Phase 1.5 (Container Integration) - 4-6 hours
1. **Framework Detection** (1 hour) - `internal/framework/detector.go`
2. **Permission Setup** (2 hours) - `internal/setup/permissions.go`
3. **Config Validation** (1 hour) - `internal/setup/validator.go`
4. **Environment Expansion** (1-2 hours) - `internal/config/envsubst.go`

### Phase 2 (Multi-Process) - 10-12 hours
1. **DAG Resolver** (2-3 hours) - Critical for depends_on
2. **Health Checks** (3-4 hours) - Essential for production
3. **Restart Policies** (2 hours) - Core reliability feature
4. **Lifecycle Hooks** (2-3 hours) - Laravel integration

**Total estimate**: 14-18 hours for Phase 1.5 + Phase 2.

Happy coding in Goland! 🚀
