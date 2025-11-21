package tui

import (
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
