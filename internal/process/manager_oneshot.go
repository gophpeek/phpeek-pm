package process

// GetOneshotHistory returns the oneshot execution history storage.
func (m *Manager) GetOneshotHistory() *OneshotHistory {
	return m.oneshotHistory
}

// GetOneshotExecutions returns oneshot execution history for a specific process.
func (m *Manager) GetOneshotExecutions(processName string, limit int) []OneshotExecution {
	if m.oneshotHistory == nil {
		return nil
	}
	if limit <= 0 {
		return m.oneshotHistory.GetAll(processName)
	}
	return m.oneshotHistory.GetRecent(processName, limit)
}

// GetAllOneshotExecutions returns oneshot execution history across all processes.
func (m *Manager) GetAllOneshotExecutions(limit int) []OneshotExecution {
	if m.oneshotHistory == nil {
		return nil
	}
	if limit <= 0 {
		return m.oneshotHistory.GetAllProcesses()
	}
	return m.oneshotHistory.GetRecentAll(limit)
}

// GetOneshotStats returns aggregate statistics for oneshot executions.
func (m *Manager) GetOneshotStats() OneshotHistoryStats {
	if m.oneshotHistory == nil {
		return OneshotHistoryStats{
			ByProcess: make(map[string]OneshotProcessStats),
		}
	}
	return m.oneshotHistory.Stats()
}
