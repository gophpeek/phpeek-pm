package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/gophpeek/phpeek-pm/internal/process"
)

// setupProcessTable initializes the process table
func (m *Model) setupProcessTable() {
	columns := []table.Column{
		{Title: "NAME", Width: 18},
		{Title: "TYPE", Width: 8},
		{Title: "STATE", Width: 14},
		{Title: "HEALTH", Width: 14},
		{Title: "SCALE", Width: 8},
		{Title: "UPTIME", Width: 12},
		{Title: "RESTARTS", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(m.height-5),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(primaryColor).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(primaryColor).
		Bold(false)

	t.SetStyles(s)
	m.processTable = t
}

// refreshProcessList fetches current process state and updates table
func (m *Model) refreshProcessList() {
	var processes []process.ProcessInfo

	// Fetch from appropriate source (embedded or remote)
	if m.isRemote {
		if m.client == nil {
			return
		}
		var err error
		processes, err = m.client.ListProcesses()
		if err != nil {
			// TODO: Show error in TUI
			return
		}
	} else {
		if m.manager == nil {
			return
		}
		processes = m.manager.ListProcesses()
	}

	rows := []table.Row{}
	for _, proc := range processes {
		// Calculate metrics from instances
		currentInstances := len(proc.Instances)
		totalRestarts := 0
		oldestStart := int64(0)
		allHealthy := true

		for _, inst := range proc.Instances {
			totalRestarts += inst.RestartCount
			if oldestStart == 0 || inst.StartedAt < oldestStart {
				oldestStart = inst.StartedAt
			}
			// Assume healthy if running (no health status in ProcessInfo yet)
			if inst.State != "running" {
				allHealthy = false
			}
		}

		// Format row data
		name := proc.Name
		procType := "longrun" // TODO: Add Type to ProcessInfo
		state := formatState(proc.State)
		health := formatHealth(allHealthy && currentInstances > 0)
		scale := fmt.Sprintf("%d/%d", currentInstances, proc.Scale)
		uptime := formatUptime(oldestStart)
		restarts := fmt.Sprintf("%d", totalRestarts)

		rows = append(rows, table.Row{
			name,
			procType,
			state,
			health,
			scale,
			uptime,
			restarts,
		})
	}

	m.processTable.SetRows(rows)
}

// formatUptime formats a timestamp as uptime duration
func formatUptime(startedAt int64) string {
	if startedAt == 0 {
		return "-"
	}

	duration := time.Since(time.Unix(startedAt, 0))

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%ds", int(duration.Seconds()))
}
