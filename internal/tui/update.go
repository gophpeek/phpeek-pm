package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all events (Elm architecture)
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateComponentSizes()
		return m, nil

	case tickMsg:
		// Refresh process state every second
		m.refreshProcessList()
		return m, tickCmd()

	case tea.QuitMsg:
		m.quitting = true
		return m, tea.Quit

	default:
		return m, nil
	}
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c", "q":
		if m.currentView == viewProcessList {
			return m, tea.Quit
		}
		// In other views, q returns to process list
		if msg.String() == "q" && m.currentView != viewProcessList {
			m.currentView = viewProcessList
			return m, nil
		}
		return m, tea.Quit

	case "?":
		m.currentView = viewHelp
		return m, nil

	case "esc":
		if m.currentView != viewProcessList {
			m.currentView = viewProcessList
		}
		return m, nil
	}

	// View-specific keys
	switch m.currentView {
	case viewProcessList:
		return m.handleProcessListKeys(msg)
	case viewLogs:
		return m.handleLogsKeys(msg)
	case viewHelp:
		return m.handleHelpKeys(msg)
	}

	return m, nil
}

// handleProcessListKeys handles keys in process list view
func (m Model) handleProcessListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.processTable.MoveUp(1)

	case "down", "j":
		m.processTable.MoveDown(1)

	case "g":
		// Go to top
		for i := 0; i < len(m.processTable.Rows()); i++ {
			m.processTable.MoveUp(1)
		}

	case "G":
		// Go to bottom
		for i := 0; i < len(m.processTable.Rows()); i++ {
			m.processTable.MoveDown(1)
		}

	case "enter":
		// View logs for selected process
		m.selectedProc = m.getSelectedProcess()
		m.currentView = viewLogs
		m.logsPaused = false
		m.logBuffer = []string{}
		return m, m.startLogTailing()

	// Future: Process control actions
	// case "r":
	//     return m, m.restartProcess()
	// case "s":
	//     return m, m.stopProcess()
	}

	return m, nil
}

// handleLogsKeys handles keys in log viewer
func (m Model) handleLogsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case " ":
		// Toggle pause
		m.logsPaused = !m.logsPaused

	case "up", "k":
		m.logViewport.LineUp(1)

	case "down", "j":
		m.logViewport.LineDown(1)

	case "g":
		m.logViewport.GotoTop()

	case "G":
		m.logViewport.GotoBottom()

	case "ctrl+u":
		m.logViewport.HalfViewUp()

	case "ctrl+d":
		m.logViewport.HalfViewDown()
	}

	return m, nil
}

// handleHelpKeys handles keys in help view
func (m Model) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Any key returns to previous view
	m.currentView = viewProcessList
	return m, nil
}

// updateComponentSizes updates component dimensions on terminal resize
func (m *Model) updateComponentSizes() {
	// Reserve space for header (3 lines) and footer (2 lines)
	availableHeight := m.height - 5

	// Recreate table with new height
	m.setupProcessTable()

	// Log viewport
	m.logViewport.Width = m.width - 2
	m.logViewport.Height = availableHeight
}

// getSelectedProcess returns the name of the currently selected process
func (m *Model) getSelectedProcess() string {
	if m.processTable.Cursor() < len(m.processTable.Rows()) {
		row := m.processTable.SelectedRow()
		if len(row) > 0 {
			return row[0] // First column is process name
		}
	}
	return ""
}

// startLogTailing begins tailing logs for the selected process
func (m *Model) startLogTailing() tea.Cmd {
	// This will be implemented to stream logs
	// For now, placeholder
	return nil
}
