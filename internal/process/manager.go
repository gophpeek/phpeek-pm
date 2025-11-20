package process

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gophpeek/phpeek-pm/internal/config"
)

// Manager manages multiple processes
type Manager struct {
	config     *config.Config
	logger     *slog.Logger
	processes  map[string]*Supervisor
	mu         sync.RWMutex
	shutdownCh chan struct{}
}

// NewManager creates a new process manager
func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	return &Manager{
		config:     cfg,
		logger:     logger,
		processes:  make(map[string]*Supervisor),
		shutdownCh: make(chan struct{}),
	}
}

// Start starts all enabled processes
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get startup order (topological sort by priority and dependencies)
	startupOrder, err := m.getStartupOrder()
	if err != nil {
		return fmt.Errorf("failed to determine startup order: %w", err)
	}

	m.logger.Info("Starting processes",
		"count", len(startupOrder),
		"order", startupOrder,
	)

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

		// Create supervisor for this process
		sup := NewSupervisor(name, procCfg, m.logger)
		m.processes[name] = sup

		// Start the process
		if err := sup.Start(ctx); err != nil {
			return fmt.Errorf("failed to start process %s: %w", name, err)
		}

		m.logger.Info("Process started successfully", "name", name)
	}

	return nil
}

// Shutdown gracefully shuts down all processes
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	close(m.shutdownCh)

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

	if len(errs) > 0 {
		return fmt.Errorf("shutdown completed with %d errors: %v", len(errs), errs)
	}

	return nil
}

// getStartupOrder returns processes in startup order (topological sort)
func (m *Manager) getStartupOrder() ([]string, error) {
	// Simple priority-based ordering for Phase 1
	// TODO: Implement proper topological sort with dependencies in Phase 2
	type procPriority struct {
		name     string
		priority int
	}

	var procs []procPriority
	for name, cfg := range m.config.Processes {
		if cfg.Enabled {
			procs = append(procs, procPriority{
				name:     name,
				priority: cfg.Priority,
			})
		}
	}

	// Sort by priority (lower priority starts first)
	for i := 0; i < len(procs); i++ {
		for j := i + 1; j < len(procs); j++ {
			if procs[i].priority > procs[j].priority {
				procs[i], procs[j] = procs[j], procs[i]
			}
		}
	}

	result := make([]string, len(procs))
	for i, p := range procs {
		result[i] = p.name
	}

	return result, nil
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
