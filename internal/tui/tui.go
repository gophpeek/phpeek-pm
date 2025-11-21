package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gophpeek/phpeek-pm/internal/process"
)

// Run starts the TUI in full-screen mode
func Run(mgr *process.Manager) error {
	model := NewModel(mgr)

	// Set initial dimensions (will be updated on first WindowSizeMsg)
	model.width = 100
	model.height = 30

	model.setupProcessTable()
	model.setupLogViewport()

	// CRITICAL: Populate initial data before starting TUI
	// This ensures processes show up immediately instead of after first tick
	model.refreshProcessList()

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

// RunRemote starts the TUI in remote mode (connects to API)
func RunRemote(apiURL, auth string) error {
	model := NewRemoteModel(apiURL, auth)

	// Set initial dimensions
	model.width = 100
	model.height = 30

	model.setupProcessTable()
	model.setupLogViewport()

	// Test connection before starting
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := model.client.HealthCheck(ctx); err != nil {
		return fmt.Errorf("failed to connect to API: %w", err)
	}

	// Populate initial data
	model.refreshProcessList()

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

// RunLogs starts a simple inline log viewer (not full-screen)
func RunLogs(mgr *process.Manager) error {
	// Simpler model for inline logs
	model := NewModel(mgr)
	model.currentView = viewLogs
	model.setupLogViewport()

	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

// setupLogViewport initializes the log viewport
func (m *Model) setupLogViewport() {
	vp := viewport.New(m.width-2, m.height-5)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor)

	m.logViewport = vp
}

// Helper to update table styles
func getTableStyle() table.Styles {
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
	return s
}
