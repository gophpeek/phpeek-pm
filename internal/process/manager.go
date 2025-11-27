package process

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/audit"
	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/deps"
	"github.com/gophpeek/phpeek-pm/internal/hooks"
	"github.com/gophpeek/phpeek-pm/internal/logger"
	"github.com/gophpeek/phpeek-pm/internal/metrics"
	"github.com/gophpeek/phpeek-pm/internal/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// Default timeouts and limits for process management operations.
// These can be overridden via configuration where applicable.
const (
	// DefaultDependencyReadinessTimeout is the maximum time to wait for
	// a process's dependencies to become ready before starting it.
	DefaultDependencyReadinessTimeout = 5 * time.Minute

	// DefaultProcessStartTimeout is the timeout for starting a single process.
	DefaultProcessStartTimeout = 30 * time.Second

	// DefaultProcessStopTimeout is the timeout for stopping a single process.
	DefaultProcessStopTimeout = 60 * time.Second

	// MaxProcessScale is the maximum number of instances a process can scale to.
	MaxProcessScale = 100
)

// Manager manages multiple processes
type Manager struct {
	config            *config.Config
	configPath        string // Path to config file for saving
	logger            *slog.Logger
	auditLogger       *audit.Logger
	processes         map[string]*Supervisor
	resourceCollector *metrics.ResourceCollector // Shared resource metrics collector
	mu                sync.RWMutex
	shutdownCh        chan struct{}
	shutdownOnce      sync.Once // Ensures shutdownCh is closed only once
	allDeadCh         chan struct{}
	processDeathCh    chan string
	startTime         time.Time
}

// NewManager creates a new process manager
func NewManager(cfg *config.Config, logger *slog.Logger, auditLogger *audit.Logger) *Manager {
	startTime := time.Now()
	metrics.SetManagerStartTime(float64(startTime.Unix()))

	// Initialize resource collector if enabled
	var resourceCollector *metrics.ResourceCollector
	if cfg.Global.ResourceMetricsEnabledValue() {
		interval := time.Duration(cfg.Global.ResourceMetricsInterval) * time.Second
		maxSamples := cfg.Global.ResourceMetricsMaxSamples
		resourceCollector = metrics.NewResourceCollector(interval, maxSamples, logger)
		logger.Info("Resource metrics enabled",
			"interval", interval,
			"max_samples", maxSamples,
		)
	}

	return &Manager{
		config:            cfg,
		logger:            logger,
		auditLogger:       auditLogger,
		processes:         make(map[string]*Supervisor),
		resourceCollector: resourceCollector,
		shutdownCh:        make(chan struct{}),
		allDeadCh:         make(chan struct{}),
		processDeathCh:    make(chan string, 10),
		startTime:         startTime,
	}
}

// Start starts all enabled processes
func (m *Manager) Start(ctx context.Context) error {
	ctx, span := tracing.StartProcessManagerSpan(ctx, "start",
		attribute.Int("process_count", len(m.config.Processes)))
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Execute pre-start hooks
	if len(m.config.Hooks.PreStart) > 0 {
		m.logger.Info("Executing pre-start hooks", "count", len(m.config.Hooks.PreStart))
		executor := hooks.NewExecutor(m.logger)
		for _, hook := range m.config.Hooks.PreStart {
			if err := executor.ExecuteWithType(ctx, &hook, "pre_start"); err != nil {
				return fmt.Errorf("pre-start hook %s failed: %w", hook.Name, err)
			}
		}
		m.logger.Info("Pre-start hooks completed successfully")
	}

	// Get startup order (topological sort by priority and dependencies)
	startupOrder, err := m.getStartupOrder()
	if err != nil {
		return fmt.Errorf("failed to determine startup order: %w", err)
	}

	m.logger.Info("Starting processes",
		"count", len(startupOrder),
		"order", startupOrder,
	)

	// Record manager process count
	metrics.SetManagerProcessCount(len(startupOrder))

	// Start processes in order
	for i, name := range startupOrder {
		m.logger.Debug("Processing startup queue",
			"index", i,
			"total", len(startupOrder),
			"process", name,
		)

		procCfg, ok := m.config.Processes[name]
		if !ok || !procCfg.Enabled {
			m.logger.Debug("Skipping disabled/missing process", "name", name)
			continue
		}

		// Wait for dependencies to be ready before starting this process
		if len(procCfg.DependsOn) > 0 {
			m.logger.Info("Waiting for dependencies",
				"process", name,
				"dependencies", procCfg.DependsOn,
			)

			for _, depName := range procCfg.DependsOn {
				m.logger.Debug("Checking dependency", "process", name, "dependency", depName)

				depSup, ok := m.processes[depName]
				if !ok {
					return fmt.Errorf("dependency %s not found for process %s", depName, name)
				}

				// Wait for dependency readiness
				m.logger.Debug("Waiting for dependency readiness",
					"process", name,
					"dependency", depName,
					"timeout", DefaultDependencyReadinessTimeout,
				)

				if err := depSup.WaitForReadiness(ctx, DefaultDependencyReadinessTimeout); err != nil {
					return fmt.Errorf("dependency %s not ready for process %s: %w", depName, name, err)
				}

				m.logger.Debug("Dependency ready", "process", name, "dependency", depName)
			}

			m.logger.Info("All dependencies ready", "process", name)
		}

		m.logger.Debug("About to start process", "name", name)

		m.logger.Info("Starting process",
			"name", name,
			"command", procCfg.Command,
			"scale", procCfg.Scale,
		)

		// Record desired scale
		metrics.SetDesiredScale(name, procCfg.Scale)

		// Create supervisor for this process
		sup := NewSupervisor(name, procCfg, &m.config.Global, m.logger, m.auditLogger, m.resourceCollector)
		sup.SetDeathNotifier(m.NotifyProcessDeath)
		m.processes[name] = sup

		// Start the process only if initial_state is "running"
		if procCfg.InitialState == "running" {
			// Create span for individual process start
			processCtx, processSpan := tracing.StartProcessManagerSpan(ctx, "start_process",
				attribute.String("process_name", name),
				attribute.Int("scale", procCfg.Scale))

			if err := sup.Start(processCtx); err != nil {
				tracing.RecordError(processSpan, err, "Failed to start process")
				processSpan.End()
				return fmt.Errorf("failed to start process %s: %w", name, err)
			}
			tracing.RecordSuccess(processSpan)
			processSpan.End()
			m.logger.Info("Process started successfully", "name", name)
		} else {
			// CRITICAL: Mark stopped processes as ready immediately
			// This prevents deadlock when other processes depend on them
			sup.MarkReadyImmediately()
			m.logger.Info("Process in initial stopped state (can be started via TUI/API)",
				"name", name,
				"initial_state", procCfg.InitialState,
			)
		}
	}

	m.logger.Debug("Finished starting all processes, checking for post-start hooks",
		"hook_count", len(m.config.Hooks.PostStart),
	)

	// Execute post-start hooks
	if len(m.config.Hooks.PostStart) > 0 {
		m.logger.Info("Executing post-start hooks", "count", len(m.config.Hooks.PostStart))
		executor := hooks.NewExecutor(m.logger)
		for _, hook := range m.config.Hooks.PostStart {
			if err := executor.ExecuteWithType(ctx, &hook, "post_start"); err != nil {
				// Post-start failures are warnings, not fatal
				m.logger.Warn("Post-start hook failed", "hook", hook.Name, "error", err)
			}
		}
		m.logger.Info("Post-start hooks completed successfully")
	}

	m.logger.Debug("Manager.Start() about to return nil - startup complete")

	return nil
}

// Shutdown gracefully shuts down all processes
func (m *Manager) Shutdown(ctx context.Context) error {
	ctx, span := tracing.StartProcessManagerSpan(ctx, "shutdown",
		attribute.Int("process_count", len(m.processes)))
	defer span.End()

	shutdownStart := time.Now()
	defer func() {
		// Record shutdown duration metric
		duration := time.Since(shutdownStart).Seconds()
		metrics.RecordShutdownDuration(duration)
		m.logger.Info("Shutdown completed",
			"duration_seconds", duration,
		)
	}()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Close shutdownCh exactly once to prevent double-close panic
	m.shutdownOnce.Do(func() {
		close(m.shutdownCh)
	})

	// Execute pre-stop hooks
	if len(m.config.Hooks.PreStop) > 0 {
		m.logger.Info("Executing pre-stop hooks", "count", len(m.config.Hooks.PreStop))
		executor := hooks.NewExecutor(m.logger)
		for _, hook := range m.config.Hooks.PreStop {
			if err := executor.ExecuteWithType(ctx, &hook, "pre_stop"); err != nil {
				m.logger.Warn("Pre-stop hook failed", "hook", hook.Name, "error", err)
			}
		}
	}

	m.logger.Info("Shutting down processes", "count", len(m.processes))

	// Get shutdown order (reverse of startup order)
	shutdownOrder := m.getShutdownOrder()

	var wg sync.WaitGroup
	errChan := make(chan error, len(shutdownOrder))

	// Shutdown processes in reverse order
	for _, name := range shutdownOrder {
		sup, ok := m.processes[name]
		if !ok {
			continue
		}

		wg.Add(1)
		go func(name string, sup *Supervisor) {
			defer wg.Done()

			m.logger.Info("Stopping process", "name", name)

			if err := sup.Stop(ctx); err != nil {
				m.logger.Error("Failed to stop process",
					"name", name,
					"error", err,
				)
				errChan <- fmt.Errorf("process %s: %w", name, err)
				return
			}

			m.logger.Info("Process stopped successfully", "name", name)
		}(name, sup)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	// Execute post-stop hooks
	if len(m.config.Hooks.PostStop) > 0 {
		m.logger.Info("Executing post-stop hooks", "count", len(m.config.Hooks.PostStop))
		executor := hooks.NewExecutor(m.logger)
		for _, hook := range m.config.Hooks.PostStop {
			if err := executor.ExecuteWithType(ctx, &hook, "post_stop"); err != nil {
				m.logger.Warn("Post-stop hook failed", "hook", hook.Name, "error", err)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown completed with %d errors: %v", len(errs), errs)
	}

	return nil
}

// getStartupOrder returns processes in startup order (topological sort)
func (m *Manager) getStartupOrder() ([]string, error) {
	// Use DAG-based topological sort with dependency resolution
	graph, err := deps.NewGraphFromConfig(m.config.Processes)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	return graph.TopologicalSort()
}

// getShutdownOrder returns processes in shutdown order (reverse of startup)
func (m *Manager) getShutdownOrder() []string {
	startupOrder, _ := m.getStartupOrder()

	// Reverse the order
	shutdownOrder := make([]string, len(startupOrder))
	for i, name := range startupOrder {
		shutdownOrder[len(startupOrder)-1-i] = name
	}

	return shutdownOrder
}

// ProcessInfo represents process status information
type ProcessInfo struct {
	Name           string                `json:"name"`
	Type           string                `json:"type"` // oneshot | longrun
	State          string                `json:"state"`
	Scale          int                   `json:"scale"`
	DesiredScale   int                   `json:"desired_scale"`
	ScaleLocked    bool                  `json:"scale_locked"`
	CPUPercent     float64               `json:"cpu_percent"`
	MemoryRSSBytes uint64                `json:"memory_rss_bytes"`
	MemoryPercent  float64               `json:"memory_percent"`
	Instances      []ProcessInstanceInfo `json:"instances"`
}

// ProcessInstanceInfo represents instance status
type ProcessInstanceInfo struct {
	ID             string  `json:"id"`
	State          string  `json:"state"`
	PID            int     `json:"pid"`
	StartedAt      int64   `json:"started_at"`
	RestartCount   int     `json:"restart_count"`
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryRSSBytes uint64  `json:"memory_rss_bytes"`
	MemoryPercent  float64 `json:"memory_percent"`
}

// ListProcesses returns status of all processes
func (m *Manager) ListProcesses() []ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	processes := make([]ProcessInfo, 0, len(m.processes))
	rc := m.resourceCollector

	for name, sup := range m.processes {
		cfg := m.config.Processes[name]

		procType := "longrun" // default
		if cfg != nil && cfg.Type != "" {
			procType = cfg.Type
		}

		var desiredScale int
		var scaleLocked bool
		if cfg != nil {
			desiredScale = cfg.Scale
			scaleLocked = cfg.ScaleLocked
		}

		info := ProcessInfo{
			Name:         name,
			Type:         procType,
			State:        string(sup.GetState()),
			Scale:        len(sup.GetInstances()),
			DesiredScale: desiredScale,
			ScaleLocked:  scaleLocked,
			Instances:    make([]ProcessInstanceInfo, 0),
		}

		var totalCPU float64
		var totalMem uint64
		var totalMemPct float64

		for _, inst := range sup.GetInstances() {
			instInfo := ProcessInstanceInfo{
				ID:           inst.ID,
				State:        string(inst.State),
				PID:          inst.PID,
				StartedAt:    inst.StartedAt,
				RestartCount: inst.RestartCount,
			}

			if rc != nil {
				if sample, ok := rc.GetLatest(name, inst.ID); ok {
					instInfo.CPUPercent = sample.CPUPercent
					instInfo.MemoryRSSBytes = sample.MemoryRSSBytes
					instInfo.MemoryPercent = float64(sample.MemoryPercent)

					totalCPU += sample.CPUPercent
					totalMem += sample.MemoryRSSBytes
					totalMemPct += float64(sample.MemoryPercent)
				}
			}

			info.Instances = append(info.Instances, instInfo)
		}

		info.CPUPercent = totalCPU
		info.MemoryRSSBytes = totalMem
		info.MemoryPercent = totalMemPct

		processes = append(processes, info)
	}

	return processes
}

// StartProcess starts a stopped process
func (m *Manager) StartProcess(ctx context.Context, name string) error {
	// Input validation
	if name == "" {
		return fmt.Errorf("process name cannot be empty")
	}

	m.mu.RLock()
	sup, ok := m.processes[name]
	procCfg, cfgOk := m.config.Processes[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("process %s not found in running processes", name)
	}
	if !cfgOk {
		return fmt.Errorf("process %s not found in configuration", name)
	}

	// Check if process is enabled
	if !procCfg.Enabled {
		return fmt.Errorf("process %s is disabled in configuration", name)
	}

	// Check current state
	currentState := sup.GetState()
	if currentState != StateStopped {
		return fmt.Errorf("process %s is not stopped (current state: %s)", name, currentState)
	}

	m.logger.Info("Starting process via control command",
		"name", name,
		"previous_state", currentState,
	)

	// Start the process with timeout
	startCtx, cancel := context.WithTimeout(ctx, DefaultProcessStartTimeout)
	defer cancel()

	if err := sup.Start(startCtx); err != nil {
		m.logger.Error("Failed to start process",
			"name", name,
			"error", err,
		)
		return fmt.Errorf("failed to start process %s: %w", name, err)
	}

	m.logger.Info("Process started successfully via control command", "name", name)
	return nil
}

// StopProcess stops a running process
func (m *Manager) StopProcess(ctx context.Context, name string) error {
	// Input validation
	if name == "" {
		return fmt.Errorf("process name cannot be empty")
	}

	m.mu.RLock()
	sup, ok := m.processes[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("process %s not found", name)
	}

	// Check current state
	currentState := sup.GetState()
	if currentState == StateStopped {
		m.logger.Info("Process already stopped",
			"name", name,
		)
		return nil // Idempotent - not an error
	}

	m.logger.Info("Stopping process via control command",
		"name", name,
		"current_state", currentState,
	)

	// Stop the process with timeout
	stopCtx, cancel := context.WithTimeout(ctx, DefaultProcessStopTimeout)
	defer cancel()

	if err := sup.Stop(stopCtx); err != nil {
		m.logger.Error("Failed to stop process",
			"name", name,
			"error", err,
		)
		return fmt.Errorf("failed to stop process %s: %w", name, err)
	}

	m.logger.Info("Process stopped successfully via control command", "name", name)
	return nil
}

// RestartProcess restarts a process
func (m *Manager) RestartProcess(ctx context.Context, name string) error {
	// Input validation
	if name == "" {
		return fmt.Errorf("process name cannot be empty")
	}

	m.mu.RLock()
	sup, ok := m.processes[name]
	_, cfgOk := m.config.Processes[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("process %s not found", name)
	}
	if !cfgOk {
		return fmt.Errorf("process %s not found in configuration", name)
	}

	currentState := sup.GetState()
	m.logger.Info("Restarting process via control command",
		"name", name,
		"current_state", currentState,
	)

	// If already stopped, just start it
	if currentState == StateStopped {
		return m.StartProcess(ctx, name)
	}

	// Stop with timeout
	stopCtx, stopCancel := context.WithTimeout(ctx, DefaultProcessStopTimeout)
	if err := sup.Stop(stopCtx); err != nil {
		stopCancel()
		m.logger.Error("Failed to stop process during restart",
			"name", name,
			"error", err,
		)
		return fmt.Errorf("failed to stop process %s: %w", name, err)
	}
	stopCancel()

	// Start process with fresh background context so it isn't tied to API request lifetime
	if err := sup.Start(context.Background()); err != nil {
		m.logger.Error("Failed to start process after restart",
			"name", name,
			"error", err,
		)
		return fmt.Errorf("failed to start process %s after restart: %w", name, err)
	}

	m.logger.Info("Process restarted successfully", "name", name)
	return nil
}

// ScaleProcess changes the number of instances for a process
func (m *Manager) ScaleProcess(ctx context.Context, name string, desiredScale int) error {
	// Input validation
	if name == "" {
		return fmt.Errorf("process name cannot be empty")
	}
	if desiredScale > MaxProcessScale {
		return fmt.Errorf("desired scale %d exceeds maximum (%d)", desiredScale, MaxProcessScale)
	}

	m.mu.RLock()
	procCfg, ok := m.config.Processes[name]
	sup, supOk := m.processes[name]
	m.mu.RUnlock()

	if !ok || !supOk {
		return fmt.Errorf("process %s not found", name)
	}

	// Check if scale is locked
	if procCfg.ScaleLocked {
		return fmt.Errorf("process %s is scale-locked (likely binds to fixed port - cannot scale)", name)
	}

	// Check if process type supports scaling
	if procCfg.Type == "oneshot" {
		return fmt.Errorf("oneshot processes cannot be scaled (type: oneshot)")
	}

	currentScale := len(sup.GetInstances())

	// Handle scale-to-zero as stop
	if desiredScale <= 0 {
		if currentScale == 0 {
			m.logger.Info("Process already stopped", "name", name)
			return nil
		}
		m.logger.Info("Scale request to zero treated as stop", "name", name)
		stopCtx, cancel := context.WithTimeout(ctx, DefaultProcessStopTimeout)
		defer cancel()
		if err := sup.Stop(stopCtx); err != nil {
			return fmt.Errorf("failed to stop process %s for scale 0: %w", name, err)
		}
		m.mu.Lock()
		if cfg := m.config.Processes[name]; cfg != nil {
			cfg.Scale = 0
		}
		m.mu.Unlock()
		metrics.SetDesiredScale(name, 0)
		return nil
	}

	// If currently stopped and desired >=1, treat as start
	if currentScale == 0 {
		if desiredScale == 0 {
			m.logger.Info("Process already stopped", "name", name)
			return nil
		}
		m.logger.Info("Scale up from zero treated as start",
			"name", name,
			"desired", desiredScale,
		)
		if cfg := m.config.Processes[name]; cfg != nil {
			cfg.Scale = desiredScale
		}
		if err := sup.Start(context.Background()); err != nil {
			return fmt.Errorf("failed to start process %s for scale %d: %w", name, desiredScale, err)
		}
		metrics.SetDesiredScale(name, desiredScale)
		if desiredScale == 1 {
			return nil
		}
		currentScale = len(sup.GetInstances())
	}

	// Idempotent check
	if currentScale == desiredScale {
		m.logger.Info("Process already at desired scale",
			"name", name,
			"scale", currentScale,
		)
		return nil
	}

	m.logger.Info("Scale operation requested",
		"name", name,
		"current", currentScale,
		"desired", desiredScale,
	)

	// SCALE UP: Start new instances
	if desiredScale > currentScale {
		if err := sup.ScaleUp(ctx, desiredScale); err != nil {
			return fmt.Errorf("scale up failed: %w", err)
		}

		m.logger.Info("Scale up completed",
			"name", name,
			"new_scale", desiredScale,
		)
		goto updateConfig
	}

	// SCALE DOWN: Stop excess instances
	if desiredScale < currentScale {
		if err := sup.ScaleDown(ctx, desiredScale); err != nil {
			return fmt.Errorf("scale down failed: %w", err)
		}

		m.logger.Info("Scale down completed",
			"name", name,
			"new_scale", desiredScale,
		)
		goto updateConfig
	}

	return nil

updateConfig:
	m.mu.Lock()
	if cfg := m.config.Processes[name]; cfg != nil {
		cfg.Scale = desiredScale
	}
	m.mu.Unlock()
	metrics.SetDesiredScale(name, desiredScale)
	return nil
}

// AdjustScale modifies the desired scale relative to the current configuration.
// Positive delta scales up, negative delta scales down. Scaling to zero is
// treated as a stop, and zero->positive is treated as a start.
func (m *Manager) AdjustScale(ctx context.Context, name string, delta int) error {
	if delta == 0 {
		return nil
	}

	m.mu.RLock()
	cfg, ok := m.config.Processes[name]
	sup, supOk := m.processes[name]
	m.mu.RUnlock()
	if !ok || !supOk {
		return fmt.Errorf("process %s not found", name)
	}

	currentScale := len(sup.GetInstances())

	target := currentScale + delta
	if target < 0 {
		target = 0
	}

	// For scale-locked processes allow only 1->0 and 0->1 transitions
	if cfg.ScaleLocked {
		if !((currentScale == 1 && target == 0) || (currentScale == 0 && target == 1)) {
			return fmt.Errorf("process %s is scale-locked (only 0/1 transitions allowed)", name)
		}
	}

	if target == currentScale {
		m.logger.Info("Process already at desired scale",
			"name", name,
			"scale", cfg.Scale,
		)
		return nil
	}

	return m.ScaleProcess(ctx, name, target)
}

// AllDeadChannel returns a channel that closes when all processes are dead
func (m *Manager) AllDeadChannel() <-chan struct{} {
	return m.allDeadCh
}

// MonitorProcessHealth starts monitoring for all processes dying
func (m *Manager) MonitorProcessHealth(ctx context.Context) {
	go func() {
		// CRITICAL: Panic recovery in monitoring goroutine
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("PANIC in MonitorProcessHealth recovered",
					"panic", r,
				)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case processName := <-m.processDeathCh:
				m.logger.Debug("Process death notification received", "process", processName)
				m.checkAllProcessesDead()
			}
		}
	}()
}

// NotifyProcessDeath is called by supervisors when a process dies and won't restart
func (m *Manager) NotifyProcessDeath(processName string) {
	select {
	case m.processDeathCh <- processName:
	default:
		// Channel full, check immediately
		m.checkAllProcessesDead()
	}
}

// checkAllProcessesDead checks if all processes are dead and signals if so
func (m *Manager) checkAllProcessesDead() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allDead := true
	for name, sup := range m.processes {
		instances := sup.GetInstances()
		hasRunningInstance := false

		for _, inst := range instances {
			if inst.State == string(StateRunning) {
				hasRunningInstance = true
				break
			}
		}

		if hasRunningInstance {
			allDead = false
			m.logger.Debug("Process still has running instances",
				"process", name,
				"instances", len(instances))
			break
		}
	}

	if allDead && len(m.processes) > 0 {
		m.logger.Warn("All managed processes have died - initiating shutdown")
		select {
		case <-m.allDeadCh:
			// Already closed
		default:
			close(m.allDeadCh)
		}
	}
}

// SetConfigPath sets the config file path for saving
func (m *Manager) SetConfigPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configPath = path
}

// AddProcess adds a new process to the configuration and optionally starts it
func (m *Manager) AddProcess(ctx context.Context, name string, procCfg *config.Process) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate process name
	if name == "" {
		return fmt.Errorf("process name cannot be empty")
	}

	// Check if process already exists
	if _, exists := m.config.Processes[name]; exists {
		return fmt.Errorf("process %s already exists", name)
	}

	// Basic validation
	if len(procCfg.Command) == 0 {
		return fmt.Errorf("process command cannot be empty")
	}
	if procCfg.Scale < 1 {
		return fmt.Errorf("process scale must be at least 1")
	}

	// Add to config
	m.config.Processes[name] = procCfg

	// If enabled, start the process
	if procCfg.Enabled {
		m.logger.Info("Starting new process", "name", name, "command", procCfg.Command, "scale", procCfg.Scale)

		supervisor := NewSupervisor(name, procCfg, &m.config.Global, m.logger, m.auditLogger, m.resourceCollector)
		if err := supervisor.Start(ctx); err != nil {
			// Remove from config on failure
			delete(m.config.Processes, name)
			return fmt.Errorf("failed to start process: %w", err)
		}

		m.processes[name] = supervisor
		m.logger.Info("Process added and started successfully", "name", name)

		// Audit log
		m.auditLogger.LogProcessAdded(name, procCfg.Command, procCfg.Scale)
	} else {
		m.logger.Info("Process added (disabled)", "name", name)
	}

	return nil
}

// RemoveProcess removes a process from the configuration and stops it if running
func (m *Manager) RemoveProcess(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if process exists
	if _, exists := m.config.Processes[name]; !exists {
		return fmt.Errorf("process %s does not exist", name)
	}

	// Stop the process if running
	if supervisor, running := m.processes[name]; running {
		m.logger.Info("Stopping process before removal", "name", name)

		if err := supervisor.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop process: %w", err)
		}

		delete(m.processes, name)
	}

	// Remove from config
	delete(m.config.Processes, name)

	m.logger.Info("Process removed successfully", "name", name)

	// Audit log
	m.auditLogger.LogProcessRemoved(name)

	return nil
}

// UpdateProcess updates an existing process configuration
func (m *Manager) UpdateProcess(ctx context.Context, name string, procCfg *config.Process) error {
	m.mu.Lock()
	err := m.updateProcessLocked(ctx, name, procCfg)
	m.mu.Unlock()
	if err != nil {
		return err
	}

	// Restart all running processes to ensure consistent state
	if err := m.restartAllProcesses(ctx); err != nil {
		return err
	}

	return nil
}

func (m *Manager) updateProcessLocked(ctx context.Context, name string, procCfg *config.Process) error {
	// Check if process exists
	oldCfg, exists := m.config.Processes[name]
	if !exists {
		return fmt.Errorf("process %s does not exist", name)
	}

	// Basic validation
	if len(procCfg.Command) == 0 {
		return fmt.Errorf("process command cannot be empty")
	}
	if procCfg.Scale < 1 {
		return fmt.Errorf("process scale must be at least 1")
	}

	// Update config
	m.config.Processes[name] = procCfg

	// If process is running, need to restart with new config
	if supervisor, running := m.processes[name]; running {
		m.logger.Info("Restarting process with new configuration", "name", name)

		// Stop old supervisor
		if err := supervisor.Stop(ctx); err != nil {
			// Rollback config change on error
			m.config.Processes[name] = oldCfg
			return fmt.Errorf("failed to stop process: %w", err)
		}

		// If new config is enabled, start with new config
		if procCfg.Enabled {
			newSupervisor := NewSupervisor(name, procCfg, &m.config.Global, m.logger, m.auditLogger, m.resourceCollector)
			if err := newSupervisor.Start(ctx); err != nil {
				// Rollback config change on error
				m.config.Processes[name] = oldCfg
				return fmt.Errorf("failed to start process with new config: %w", err)
			}

			m.processes[name] = newSupervisor
			m.logger.Info("Process updated and restarted", "name", name)
		} else {
			// New config is disabled, just remove from running processes
			delete(m.processes, name)
			m.logger.Info("Process updated and disabled", "name", name)
		}
	} else if procCfg.Enabled {
		// Process wasn't running but new config enables it
		m.logger.Info("Starting previously disabled process", "name", name)

		supervisor := NewSupervisor(name, procCfg, &m.config.Global, m.logger, m.auditLogger, m.resourceCollector)
		if err := supervisor.Start(ctx); err != nil {
			// Rollback config change on error
			m.config.Processes[name] = oldCfg
			return fmt.Errorf("failed to start process: %w", err)
		}

		m.processes[name] = supervisor
		m.logger.Info("Process updated and started", "name", name)
	}

	// Audit log
	m.auditLogger.LogProcessUpdated(name, procCfg.Command, procCfg.Scale)

	return nil
}

// restartAllProcesses restarts every running process sequentially
func (m *Manager) restartAllProcesses(ctx context.Context) error {
	m.mu.RLock()
	names := make([]string, 0, len(m.processes))
	for name := range m.processes {
		names = append(names, name)
	}
	m.mu.RUnlock()

	var errs []string
	for _, name := range names {
		if err := m.RestartProcess(ctx, name); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to restart processes: %s", strings.Join(errs, "; "))
	}

	return nil
}

// SaveConfig saves the current configuration to the config file
func (m *Manager) SaveConfig() error {
	m.mu.RLock()
	configPath := m.configPath
	cfg := m.config
	m.mu.RUnlock()

	if configPath == "" {
		return fmt.Errorf("config file path not set")
	}

	m.logger.Info("Saving configuration", "path", configPath)

	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.logger.Info("Configuration saved successfully", "path", configPath)

	// Audit log
	m.auditLogger.LogConfigSaved(configPath)

	return nil
}

// ReloadConfig reloads the configuration from the config file
func (m *Manager) ReloadConfig(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.configPath == "" {
		return fmt.Errorf("config file path not set")
	}

	m.logger.Info("Reloading configuration", "path", m.configPath)

	// Load new config
	newCfg, err := config.LoadWithEnvExpansion(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine what changed
	toStop := []string{}
	toStart := []string{}
	toUpdate := []string{}

	// Check for removed processes
	for name := range m.config.Processes {
		if _, exists := newCfg.Processes[name]; !exists {
			toStop = append(toStop, name)
		}
	}

	// Check for new or updated processes
	for name, newProc := range newCfg.Processes {
		if oldProc, exists := m.config.Processes[name]; exists {
			// Process exists, check if changed
			if !oldProc.Equal(newProc) {
				toUpdate = append(toUpdate, name)
			}
		} else {
			// New process
			toStart = append(toStart, name)
		}
	}

	m.logger.Info("Configuration reload plan",
		"to_stop", toStop,
		"to_start", toStart,
		"to_update", toUpdate,
	)

	// Stop removed processes
	for _, name := range toStop {
		if supervisor, running := m.processes[name]; running {
			m.logger.Info("Stopping removed process", "name", name)
			if err := supervisor.Stop(ctx); err != nil {
				m.logger.Error("Failed to stop process during reload", "name", name, "error", err)
			}
			delete(m.processes, name)
		}
	}

	// Update config
	m.config = newCfg

	// Start new processes
	for _, name := range toStart {
		procCfg := newCfg.Processes[name]
		if procCfg.Enabled {
			m.logger.Info("Starting new process", "name", name)
			supervisor := NewSupervisor(name, procCfg, &newCfg.Global, m.logger, m.auditLogger, m.resourceCollector)
			if err := supervisor.Start(ctx); err != nil {
				m.logger.Error("Failed to start new process during reload", "name", name, "error", err)
				continue
			}
			m.processes[name] = supervisor
		}
	}

	// Update changed processes
	for _, name := range toUpdate {
		procCfg := newCfg.Processes[name]

		if supervisor, running := m.processes[name]; running {
			m.logger.Info("Restarting updated process", "name", name)

			if err := supervisor.Stop(ctx); err != nil {
				m.logger.Error("Failed to stop process during update", "name", name, "error", err)
				continue
			}

			if procCfg.Enabled {
				newSupervisor := NewSupervisor(name, procCfg, &newCfg.Global, m.logger, m.auditLogger, m.resourceCollector)
				if err := newSupervisor.Start(ctx); err != nil {
					m.logger.Error("Failed to start updated process", "name", name, "error", err)
					continue
				}
				m.processes[name] = newSupervisor
			} else {
				delete(m.processes, name)
			}
		} else if procCfg.Enabled {
			m.logger.Info("Starting previously disabled process", "name", name)
			supervisor := NewSupervisor(name, procCfg, &newCfg.Global, m.logger, m.auditLogger, m.resourceCollector)
			if err := supervisor.Start(ctx); err != nil {
				m.logger.Error("Failed to start process during reload", "name", name, "error", err)
				continue
			}
			m.processes[name] = supervisor
		}
	}

	m.logger.Info("Configuration reloaded successfully")

	// Audit log
	m.auditLogger.LogConfigReloaded(m.configPath)

	return nil
}

// GetConfig returns a copy of the current configuration
func (m *Manager) GetConfig() *config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modifications
	cfgCopy := *m.config
	cfgCopy.Processes = make(map[string]*config.Process, len(m.config.Processes))
	for k, v := range m.config.Processes {
		procCopy := *v
		cfgCopy.Processes[k] = &procCopy
	}

	return &cfgCopy
}

// GetProcessConfig returns a copy of a single process configuration
func (m *Manager) GetProcessConfig(name string) (*config.Process, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proc, exists := m.config.Processes[name]
	if !exists {
		return nil, fmt.Errorf("process %s not found", name)
	}

	procCopy := *proc
	if proc.Command != nil {
		procCopy.Command = append([]string{}, proc.Command...)
	}
	if proc.Env != nil {
		procCopy.Env = make(map[string]string, len(proc.Env))
		for k, v := range proc.Env {
			procCopy.Env[k] = v
		}
	}
	if proc.DependsOn != nil {
		procCopy.DependsOn = append([]string{}, proc.DependsOn...)
	}
	return &procCopy, nil
}

// GetResourceCollector returns the resource collector (can be nil if disabled)
func (m *Manager) GetResourceCollector() *metrics.ResourceCollector {
	return m.resourceCollector
}

// GetLogs returns log entries for a specific process
// If limit > 0, returns only the most recent 'limit' entries
// Returns error if process doesn't exist
func (m *Manager) GetLogs(processName string, limit int) ([]logger.LogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sup, exists := m.processes[processName]
	if !exists {
		return nil, fmt.Errorf("process not found: %s", processName)
	}

	return sup.GetLogs(limit), nil
}

// GetStackLogs aggregates logs from all processes in the manager
// Returns the most recent entries across the stack capped by limit (if > 0)
func (m *Manager) GetStackLogs(limit int) []logger.LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allLogs := make([]logger.LogEntry, 0, len(m.processes)*limit)

	for _, sup := range m.processes {
		// Reuse supervisor ordering logic and enforce a per-process cap
		allLogs = append(allLogs, sup.GetLogs(limit)...)
	}

	// Sort entire stack newest-first
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Timestamp.After(allLogs[j].Timestamp)
	})

	if limit > 0 && len(allLogs) > limit {
		allLogs = allLogs[:limit]
	}

	return allLogs
}
