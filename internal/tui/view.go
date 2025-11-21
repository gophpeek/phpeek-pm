package tui

import (
	"fmt"
	"strings"
)

// View renders the current view
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.currentView {
	case viewProcessList:
		return m.renderProcessList()
	case viewLogs:
		return m.renderLogs()
	case viewHelp:
		return m.renderHelp()
	default:
		return "Unknown view"
	}
}

// renderProcessList renders the main process dashboard
func (m Model) renderProcessList() string {
	var b strings.Builder

	// Header
	header := titleStyle.Render("PHPeek PM v1.0.0")
	status := fmt.Sprintf("Processes: %d | Press ? for help", len(m.processTable.Rows()))
	b.WriteString(header + " " + dimStyle.Render(status) + "\n")
	b.WriteString(strings.Repeat("─", m.width) + "\n")

	// Process table
	b.WriteString(m.processTable.View())
	b.WriteString("\n")

	// Footer with shortcuts
	footer := dimStyle.Render("<Enter> Logs | <r> Restart | <s> Stop | <+/-> Scale | <q> Quit")
	b.WriteString(footer)

	return b.String()
}

// renderLogs renders the log viewer
func (m Model) renderLogs() string {
	var b strings.Builder

	// Header
	header := titleStyle.Render(fmt.Sprintf("Logs: %s", m.selectedProc))
	status := ""
	if m.logsPaused {
		status = warnStyle.Render(" [PAUSED]")
	} else {
		status = successStyle.Render(" [LIVE]")
	}
	b.WriteString(header + status + "\n")
	b.WriteString(dimStyle.Render("Auto-scroll: ") + status + dimStyle.Render(" | Press ESC to go back") + "\n")
	b.WriteString(strings.Repeat("─", m.width) + "\n")

	// Log viewport
	b.WriteString(m.logViewport.View())
	b.WriteString("\n")

	// Footer
	footer := dimStyle.Render("<Space> Pause | <j/k> Scroll | <g/G> Top/Bottom | <ESC> Back | <q> Quit")
	b.WriteString(footer)

	return b.String()
}

// renderHelp renders the help overlay
func (m Model) renderHelp() string {
	var b strings.Builder

	help := `
PHPeek PM - Keyboard Shortcuts

Process List View:
  ↑/k, ↓/j      Navigate up/down
  g, G          Go to top/bottom
  Enter         View process logs
  r             Restart process (future)
  s             Stop process (future)
  +/-           Scale process (future)
  ?             Show this help
  q             Quit

Log Viewer:
  Space         Pause/resume auto-scroll
  ↑/k, ↓/j      Scroll up/down
  Ctrl+U/D      Page up/down
  g, G          Jump to top/bottom
  ESC           Return to process list
  q             Quit

Press any key to return...
`

	b.WriteString(titleStyle.Render("Help") + "\n")
	b.WriteString(help)

	return b.String()
}
