package process

import "context"

// AllDeadChannel returns a channel that closes when all processes are dead.
func (m *Manager) AllDeadChannel() <-chan struct{} {
	return m.allDeadCh
}

// MonitorProcessHealth starts monitoring for all processes dying.
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

// NotifyProcessDeath is called by supervisors when a process dies and won't restart.
func (m *Manager) NotifyProcessDeath(processName string) {
	select {
	case m.processDeathCh <- processName:
	default:
		// Channel full, check immediately
		m.checkAllProcessesDead()
	}
}

// checkAllProcessesDead checks if all processes are dead and signals if so.
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
		// CRITICAL: Use sync.Once to prevent double-close panic from concurrent calls
		m.allDeadOnce.Do(func() {
			close(m.allDeadCh)
		})
	}
}
