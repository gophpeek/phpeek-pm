package process

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/dag"
	"github.com/gophpeek/phpeek-pm/internal/hooks"
	"github.com/gophpeek/phpeek-pm/internal/metrics"
)

// Manager manages multiple processes
type Manager struct {
	config           *config.Config
	logger           *slog.Logger
	processes        map[string]*Supervisor
	mu               sync.RWMutex
	shutdownCh       chan struct{}
	allDeadCh        chan struct{}
	processDeathCh   chan string
	startTime        time.Time
}

// NewManager creates a new process manager
func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	startTime := time.Now()
	metrics.SetManagerStartTime(float64(startTime.Unix()))

	return &Manager{
		config:         cfg,
		logger:         logger,
		processes:      make(map[string]*Supervisor),
		shutdownCh:     make(chan struct{}),
		allDeadCh:      make(chan struct{}),
		processDeathCh: make(chan string, 10),
		startTime:      startTime,
	}
}

// Start starts all enabled processes
func (m *Manager) Start(ctx context.Context) error {
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
	for _, name := range startupOrder {
		procCfg, ok := m.config.Processes[name]
		if !ok || !procCfg.Enabled {
			continue
		}

		m.logger.Info("Starting process",
			"name", name,
			"command", procCfg.Command,
			"scale", procCfg.Scale,
		)

		// Record desired scale
		metrics.SetDesiredScale(name, procCfg.Scale)

		// Create supervisor for this process
		sup := NewSupervisor(name, procCfg, &m.config.Global, m.logger)
		sup.SetDeathNotifier(m.NotifyProcessDeath)
		m.processes[name] = sup

		// Start the process
		if err := sup.Start(ctx); err != nil {
			return fmt.Errorf("failed to start process %s: %w", name, err)
		}

		m.logger.Info("Process started successfully", "name", name)
	}

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

	return nil
}

// Shutdown gracefully shuts down all processes
func (m *Manager) Shutdown(ctx context.Context) error {
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

	close(m.shutdownCh)

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
	graph, err := dag.NewGraph(m.config.Processes)
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
	Name      string                 `json:"name"`
	State     string                 `json:"state"`
	Scale     int                    `json:"scale"`
	Instances []ProcessInstanceInfo  `json:"instances"`
}

// ProcessInstanceInfo represents instance status
type ProcessInstanceInfo struct {
	ID           string `json:"id"`
	State        string `json:"state"`
	PID          int    `json:"pid"`
	StartedAt    int64  `json:"started_at"`
	RestartCount int    `json:"restart_count"`
}

// ListProcesses returns status of all processes
func (m *Manager) ListProcesses() []ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	processes := make([]ProcessInfo, 0, len(m.processes))

	for name, sup := range m.processes {
		info := ProcessInfo{
			Name:      name,
			State:     string(sup.GetState()),
			Scale:     len(sup.GetInstances()),
			Instances: make([]ProcessInstanceInfo, 0),
		}

		for _, inst := range sup.GetInstances() {
			info.Instances = append(info.Instances, ProcessInstanceInfo{
				ID:           inst.ID,
				State:        string(inst.State),
				PID:          inst.PID,
				StartedAt:    inst.StartedAt,
				RestartCount: inst.RestartCount,
			})
		}

		processes = append(processes, info)
	}

	return processes
}

// AllDeadChannel returns a channel that closes when all processes are dead
func (m *Manager) AllDeadChannel() <-chan struct{} {
	return m.allDeadCh
}

// MonitorProcessHealth starts monitoring for all processes dying
func (m *Manager) MonitorProcessHealth(ctx context.Context) {
	go func() {
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
