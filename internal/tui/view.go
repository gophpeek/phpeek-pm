package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the current view
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.currentView {
	case viewProcessList:
		return m.renderProcessList()
	case viewProcessDetail:
		return m.renderProcessDetail()
	case viewLogs:
		return m.renderLogs()
	case viewHelp:
		return m.renderHelp()
	case viewWizard:
		return m.renderWizard()
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
	b.WriteString(m.renderProcessTable())
	b.WriteString("\n")

	// Error display
	if m.err != nil {
		errMsg := errorStyle.Render("✗ Error: " + m.err.Error())
		b.WriteString(errMsg + "\n")
	}

	// Toast notification
	if m.toast != "" {
		toast := toastStyle.Render(m.toast)
		b.WriteString(toast + "\n")
	}

	// Footer with shortcuts
	footer := dimStyle.Render("<Enter> Process | <l> Logs | <L> Stack Logs | <e> Edit | <d> Delete | <a> Add | <r> Restart | <s> Stop | <x> Start | <+/-> Scale | <q> Quit")
	b.WriteString(footer)

	// Overlay dialogs
	view := b.String()

	if m.showConfirmation {
		view = m.withOverlay(view, m.renderConfirmationOverlay())
	}

	if m.showScaleDialog {
		view = m.withOverlay(view, m.renderScaleDialogOverlay())
	}

	return m.padViewHeight(view)
}

// renderProcessDetail renders the detail view for a single process
func (m Model) renderProcessDetail() string {
	if m.detailProc == "" {
		return m.padViewHeight("No process selected. Press ESC to return.\n")
	}

	info, ok := m.processCache[m.detailProc]
	if !ok {
		msg := fmt.Sprintf("Process %s not found. Press ESC to return.\n", m.detailProc)
		return m.padViewHeight(msg)
	}

	var b strings.Builder

	stateText, stateStyle := stateDisplay(info.State)
	header := titleStyle.Render(fmt.Sprintf("Process: %s", info.Name))
	status := stateStyle.Render(stateText)
	b.WriteString(header + " " + status + "\n")

	totalRestarts := 0
	oldestStart := int64(0)
	for _, inst := range info.Instances {
		totalRestarts += inst.RestartCount
		if inst.StartedAt > 0 && (oldestStart == 0 || inst.StartedAt < oldestStart) {
			oldestStart = inst.StartedAt
		}
	}

	summary := fmt.Sprintf("Type: %s  |  Scale: %d/%d  |  Restarts: %d  |  Uptime: %s",
		info.Type,
		info.Scale,
		info.DesiredScale,
		totalRestarts,
		formatUptime(oldestStart),
	)
	b.WriteString(dimStyle.Render(summary) + "\n")
	b.WriteString(strings.Repeat("─", m.width) + "\n")

	b.WriteString(highlightStyle.Render("Instances") + "\n")
	if len(info.Instances) == 0 {
		b.WriteString("No instances running.\n")
	} else {
		b.WriteString(m.instanceTable.View())
		b.WriteString("\n")
	}

	footer := dimStyle.Render("<l> Logs | <r> Restart | <s> Stop | <ESC> Back")
	b.WriteString(footer)

	return m.padViewHeight(b.String())
}

// renderLogs renders the log viewer
func (m Model) renderLogs() string {
	var b strings.Builder

	// Header
	scopeLabel := "Stack"
	if m.logScope == logScopeProcess && m.selectedProc != "" {
		if m.logInstance != "" {
			scopeLabel = fmt.Sprintf("%s (%s)", m.selectedProc, m.logInstance)
		} else {
			scopeLabel = m.selectedProc
		}
	}
	header := titleStyle.Render(fmt.Sprintf("Logs: %s", scopeLabel))
	status := ""
	if m.logsPaused {
		status = warnStyle.Render(" [PAUSED]")
	} else {
		status = successStyle.Render(" [LIVE]")
	}
	b.WriteString(header + status + "\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Scope: %s | Auto-scroll: ", m.logScopeDescription())) + status + dimStyle.Render(" | Press ESC to go back") + "\n")
	b.WriteString(strings.Repeat("─", m.width) + "\n")

	// Log viewport
	b.WriteString(m.logViewport.View())
	b.WriteString("\n")

	// Footer
	footer := dimStyle.Render("<Space> Pause | <j/k> Scroll | <g/G> Top/Bottom | <ESC> Back | <q> Quit")
	b.WriteString(footer)

	return m.padViewHeight(b.String())
}

// renderHelp renders the help overlay
func (m Model) renderHelp() string {
	var b strings.Builder

	help := `
PHPeek PM - Keyboard Shortcuts

Process List View:
  ↑/k, ↓/j      Navigate up/down
  g, G          Go to top/bottom
  Enter         View process details
  l             View selected process logs
  L             View stack logs
  e             Edit process configuration
  d             Delete process (confirmation)
  r             Restart process (with confirmation)
  s             Stop process (with confirmation)
  x             Start process (with confirmation)
  +/=           Scale process up (opens dialog)
  -/_           Scale process down (opens dialog)
  ?             Show this help
  q             Quit

Process Detail View:
  l             View process logs
  r             Restart process
  s             Stop process
  ESC           Return to list

Log Viewer:
  Space         Pause/resume auto-scroll
  ↑/k, ↓/j      Scroll up/down
  Ctrl+U/D      Page up/down
  g, G          Jump to top/bottom
  ESC           Return to process list
  q             Quit

Confirmation Dialog:
  y/Enter       Confirm action
  n/ESC         Cancel action

Scale Dialog:
  0-9           Enter scale value
  Backspace     Delete digit
  Enter         Apply scale
  ESC           Cancel

Press any key to return...
`

	b.WriteString(titleStyle.Render("Help") + "\n")
	b.WriteString(help)

	return m.padViewHeight(b.String())
}

func (m Model) logScopeDescription() string {
	if m.logScope == logScopeProcess && m.selectedProc != "" {
		return fmt.Sprintf("Process (%s)", m.selectedProc)
	}
	return "Stack (all processes)"
}

// renderConfirmationOverlay renders confirmation dialog
func (m Model) renderConfirmationOverlay() string {
	var actionText string
	switch m.pendingAction {
	case actionRestart:
		actionText = "Restart"
	case actionStop:
		actionText = "Stop"
	case actionStart:
		actionText = "Start"
	default:
		actionText = "Execute"
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		fmt.Sprintf("Confirm %s", actionText),
		"",
		fmt.Sprintf("Process: %s", m.pendingTarget),
		"",
		"Are you sure? (y/n)",
	)

	return dialogBoxStyle.Render(body)
}

// renderScaleDialogOverlay renders scale input dialog
func (m Model) renderScaleDialogOverlay() string {
	input := m.scaleInput
	if input == "" {
		input = "_"
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		"Scale Process",
		"",
		fmt.Sprintf("Process: %s", m.pendingTarget),
		"",
		fmt.Sprintf("Desired scale: %s", input),
		"",
		"Enter number and press Enter",
	)

	return dialogBoxStyle.Render(body)
}

// withOverlay appends a centered overlay block below the existing view
func (m Model) withOverlay(base, overlay string) string {
	if m.width <= 0 {
		return base + "\n\n" + overlay
	}

	block := lipgloss.Place(m.width, lipgloss.Height(overlay)+2, lipgloss.Center, lipgloss.Center, overlay)
	return base + "\n\n" + block
}

// padViewHeight fills remaining screen rows with blank lines so previous frames
// don't leave duplicated footers or headers behind.
func (m Model) padViewHeight(view string) string {
	if m.height <= 0 || m.width <= 0 {
		return view
	}

	currentHeight := lipgloss.Height(view)
	if currentHeight >= m.height {
		return view
	}

	var b strings.Builder
	b.WriteString(view)

	if !strings.HasSuffix(view, "\n") {
		b.WriteString("\n")
	}

	paddingLine := lipgloss.NewStyle().Width(m.width).Render("")
	extraLines := m.height - currentHeight
	for i := 0; i < extraLines; i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(paddingLine)
	}

	return b.String()
}

// renderWizard renders the process creation wizard
func (m Model) renderWizard() string {
	var b strings.Builder

	// Header
	title := "Add New Process"
	if m.wizardMode == wizardModeEdit {
		title = fmt.Sprintf("Edit Process: %s", m.wizardOriginal)
	}
	header := titleStyle.Render(title)
	stepInfo := dimStyle.Render(fmt.Sprintf(" [Step %d/6]", m.wizardStep+1))
	b.WriteString(header + stepInfo + "\n")
	b.WriteString(strings.Repeat("─", m.width) + "\n\n")

	// Progress bar
	progressBar := m.renderProgressBar()
	b.WriteString(progressBar + "\n\n")

	// Step content
	switch m.wizardStep {
	case 0:
		b.WriteString(m.renderWizardStepName())
	case 1:
		b.WriteString(m.renderWizardStepCommand())
	case 2:
		b.WriteString(m.renderWizardStepScale())
	case 3:
		b.WriteString(m.renderWizardStepRestart())
	case 4:
		b.WriteString(m.renderWizardStepPreview())
	}

	// Error display
	if m.wizardError != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render("⚠ " + m.wizardError))
	}

	// Footer with navigation hints
	b.WriteString("\n\n")
	footer := dimStyle.Render("<Tab/Enter> Next | <Shift+Tab> Back | <ESC> Cancel")
	b.WriteString(footer)

	return m.padViewHeight(b.String())
}

// renderProgressBar renders a visual progress bar for the wizard
func (m Model) renderProgressBar() string {
	steps := []string{"Name", "Command", "Scale", "Restart", "Preview"}
	var parts []string

	for i, step := range steps {
		if i < m.wizardStep {
			parts = append(parts, successStyle.Render("✓ "+step))
		} else if i == m.wizardStep {
			parts = append(parts, highlightStyle.Render("● "+step))
		} else {
			parts = append(parts, dimStyle.Render("○ "+step))
		}
	}

	return strings.Join(parts, "  ")
}

// renderWizardStepName renders the name input step
func (m Model) renderWizardStepName() string {
	var b strings.Builder

	b.WriteString(highlightStyle.Render("Process Name") + "\n\n")
	if m.wizardNameLocked {
		b.WriteString("Editing existing process. Name cannot be changed.\n\n")
	} else {
		b.WriteString("Enter a unique name for this process (no spaces):\n\n")
	}

	// Input field
	input := m.wizardName
	if input == "" {
		input = dimStyle.Render("_")
	}
	b.WriteString("  Name: " + highlightStyle.Render(input) + "\n\n")

	if !m.wizardNameLocked {
		b.WriteString(dimStyle.Render("Examples: php-fpm, nginx, horizon, queue-worker"))
	}

	return b.String()
}

// renderWizardStepCommand renders the command input step
func (m Model) renderWizardStepCommand() string {
	var b strings.Builder

	b.WriteString(highlightStyle.Render("Process Command") + "\n\n")
	b.WriteString("Enter the full command exactly as you would run it:\n\n")

	input := m.wizardCommandLine
	if input == "" {
		input = dimStyle.Render("_")
	}
	b.WriteString("  Command: " + highlightStyle.Render(input) + "\n\n")

	b.WriteString(dimStyle.Render("Tip: type the command normally (e.g. php artisan queue:work --tries=3)\n"))
	b.WriteString(dimStyle.Render("It will be split into parts automatically."))

	return b.String()
}

// renderWizardStepScale renders the scale input step
func (m Model) renderWizardStepScale() string {
	var b strings.Builder

	b.WriteString(highlightStyle.Render("Process Scale") + "\n\n")
	b.WriteString("How many instances should run simultaneously?\n\n")

	// Input field
	input := m.wizardScaleInput
	if input == "" {
		input = "1"
	}
	b.WriteString("  Scale: " + highlightStyle.Render(input) + " instance(s)\n\n")

	b.WriteString(dimStyle.Render("Recommended: 1 for single services, 2+ for workers"))

	return b.String()
}

// renderWizardStepRestart renders the restart policy selection step
func (m Model) renderWizardStepRestart() string {
	var b strings.Builder

	b.WriteString(highlightStyle.Render("Restart Policy") + "\n\n")
	b.WriteString("Select restart behavior (use ↑/↓ or j/k):\n\n")

	// Options
	options := []struct {
		value string
		desc  string
	}{
		{"always", "Always restart on exit"},
		{"on-failure", "Restart only on failure"},
		{"never", "Never restart automatically"},
	}

	for _, opt := range options {
		if m.wizardRestart == opt.value {
			b.WriteString("  " + highlightStyle.Render("● "+opt.value) + " - " + opt.desc + "\n")
		} else {
			b.WriteString("  " + dimStyle.Render("○ "+opt.value) + " - " + dimStyle.Render(opt.desc) + "\n")
		}
	}

	b.WriteString("\n" + dimStyle.Render("Recommended: 'always' for critical services"))

	return b.String()
}

// renderWizardStepPreview renders the preview/confirmation step
func (m Model) renderWizardStepPreview() string {
	var b strings.Builder

	b.WriteString(highlightStyle.Render("Preview Configuration") + "\n\n")
	b.WriteString("Review and confirm:\n\n")

	// Display all configuration
	b.WriteString("  " + highlightStyle.Render("Name:") + "     " + m.wizardName + "\n")
	commandPreview := m.wizardCommandLine
	if commandPreview == "" {
		commandPreview = "(not set)"
	}
	b.WriteString("  " + highlightStyle.Render("Command:") + "  " + commandPreview + "\n")
	b.WriteString("  " + highlightStyle.Render("Scale:") + "    " + fmt.Sprintf("%d", m.wizardScale) + "\n")
	b.WriteString("  " + highlightStyle.Render("Restart:") + "  " + m.wizardRestart + "\n")
	b.WriteString("  " + highlightStyle.Render("Enabled:") + "  " + fmt.Sprintf("%v", m.wizardEnabled) + "\n\n")

	b.WriteString(successStyle.Render("✓ Ready to create process\n"))
	b.WriteString(dimStyle.Render("Press Enter to create, Shift+Tab to go back"))

	return b.String()
}
