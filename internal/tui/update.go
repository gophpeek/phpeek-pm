package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/logger"
	"github.com/gophpeek/phpeek-pm/internal/process"
)

// processListResultMsg carries process listing data fetched asynchronously
type processListResultMsg struct {
	processes []process.ProcessInfo
	err       error
}

type processConfigResultMsg struct {
	name string
	cfg  *config.Process
	err  error
}

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
		// Clear expired toast
		m.clearToastIfExpired()
		// Refresh process state every second without blocking UI
		return m, tea.Batch(
			tickCmd(),
			m.refreshProcessListCmd(),
		)

	case tickLogRefreshMsg:
		// Refresh logs if in log view
		if m.currentView == viewLogs {
			m.refreshLogs()
			return m, tickLogRefreshCmd()
		}
		return m, nil

	case actionResultMsg:
		// Show toast notification for action result
		if msg.success {
			m.showToast(msg.message, 3*time.Second)
		} else {
			m.showToast(msg.message, 5*time.Second)
		}
		// Trigger async refresh to show updated state
		return m, m.refreshProcessListCmd()

	case processListResultMsg:
		m.applyProcessListResult(msg)
		return m, nil

	case processConfigResultMsg:
		if msg.err != nil {
			m.showToast(fmt.Sprintf("✗ Failed to load process: %v", msg.err), 5*time.Second)
			return m, nil
		}
		m.startEditWizard(msg.name, msg.cfg)
		return m, nil

	case tea.QuitMsg:
		m.quitting = true
		return m, tea.Quit

	default:
		return m, nil
	}
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle confirmation dialog
	if m.showConfirmation {
		return m.handleConfirmationKeys(msg)
	}

	// Handle scale dialog
	if m.showScaleDialog {
		return m.handleScaleDialogKeys(msg)
	}

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
		if m.currentView == viewLogs {
			m.currentView = m.logReturn
		} else if m.currentView != viewProcessList {
			m.currentView = viewProcessList
		}
		return m, nil
	}

	// View-specific keys
	switch m.currentView {
	case viewProcessList:
		return m.handleProcessListKeys(msg)
	case viewProcessDetail:
		return m.handleProcessDetailKeys(msg)
	case viewLogs:
		return m.handleLogsKeys(msg)
	case viewHelp:
		return m.handleHelpKeys(msg)
	case viewWizard:
		return m.handleWizardKeys(msg)
	}

	return m, nil
}

// handleProcessListKeys handles keys in process list view
func (m Model) handleProcessListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "k", "up":
		m.moveSelection(-1)
		return m, nil

	case "j", "down":
		m.moveSelection(1)
		return m, nil

	case "g":
		m.setSelection(0)
		return m, nil

	case "G":
		if count := len(m.tableData); count > 0 {
			m.setSelection(count - 1)
		}
		return m, nil

	case "enter":
		procName := m.getSelectedProcess()
		if procName == "" {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		m.openProcessDetail(procName)
		return m, nil

	case "l":
		procName := m.getSelectedProcess()
		if procName == "" {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		return m, m.openLogView(logScopeProcess, procName, "")

	case "L":
		return m, m.openLogView(logScopeStack, "", "")

	case "e":
		procName := m.getSelectedProcess()
		if procName == "" {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		return m, m.fetchProcessConfigCmd(procName)

	case "d":
		procName := m.getSelectedProcess()
		if procName == "" {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		m.confirmAction(actionDelete, procName)
		return m, nil

	case "r":
		// Restart process
		procName := m.getSelectedProcess()
		if procName != "" {
			return m, m.triggerAction(actionRestart, procName)
		}

	case "s":
		info := m.getSelectedProcessInfo()
		if info == nil {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		if strings.Contains(strings.ToLower(info.rawState), "stop") {
			m.showToast("Process already stopped", 3*time.Second)
			return m, nil
		}
		return m, m.triggerAction(actionStop, info.name)

	case "x":
		info := m.getSelectedProcessInfo()
		if info == nil {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		if strings.Contains(strings.ToLower(info.rawState), "run") {
			m.showToast("Process already running", 3*time.Second)
			return m, nil
		}
		return m, m.triggerAction(actionStart, info.name)

	case "+", "=":
		return m.handleQuickScale(1)

	case "-", "_":
		return m.handleQuickScale(-1)

	case "c":
		info := m.getSelectedProcessInfo()
		if info == nil {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		if info.scaleLocked {
			m.showToast("✗ Process is scale-locked", 3*time.Second)
			return m, nil
		}
		m.openScaleDialog(info.name)

	case "a":
		// Add process wizard
		m.startWizard()
		return m, nil
	}

	return m, nil
}

func (m *Model) moveSelection(delta int) {
	if len(m.tableData) == 0 {
		m.selectedIndex = 0
		m.processTable.SetCursor(0)
		return
	}
	newIdx := m.selectedIndex + delta
	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(m.tableData) {
		newIdx = len(m.tableData) - 1
	}
	m.setSelection(newIdx)
}

func (m *Model) setSelection(index int) {
	if len(m.tableData) == 0 {
		m.selectedIndex = 0
		m.processTable.SetCursor(0)
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.tableData) {
		index = len(m.tableData) - 1
	}
	m.selectedIndex = index
	m.processTable.SetCursor(index)
	m.ensureCursorVisible()
}

// handleProcessDetailKeys handles keys when viewing a single process
func (m Model) handleProcessDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "l":
		if m.detailProc == "" {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		instance := m.getSelectedInstanceID()
		return m, m.openLogView(logScopeProcess, m.detailProc, instance)

	case "r":
		if m.detailProc == "" {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		return m, m.triggerAction(actionRestart, m.detailProc)

	case "s":
		if m.detailProc == "" {
			m.showToast("✗ No process selected", 3*time.Second)
			return m, nil
		}
		if info, ok := m.processCache[m.detailProc]; ok {
			if strings.Contains(strings.ToLower(info.State), "stop") {
				m.showToast("Process already stopped", 3*time.Second)
				return m, nil
			}
		}
		return m, m.triggerAction(actionStop, m.detailProc)
	}

	var cmd tea.Cmd
	if m.instanceTable.Columns() != nil {
		m.instanceTable, cmd = m.instanceTable.Update(msg)
		return m, cmd
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
		m.logViewport.ScrollUp(1)

	case "down", "j":
		m.logViewport.ScrollDown(1)

	case "g":
		m.logViewport.GotoTop()

	case "G":
		m.logViewport.GotoBottom()

	case "ctrl+u":
		m.logViewport.HalfPageUp()

	case "ctrl+d":
		m.logViewport.HalfPageDown()
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
	m.ensureCursorVisible()

	// Log viewport
	m.logViewport.Width = m.width - 2
	m.logViewport.Height = availableHeight

	if m.instanceTable.Columns() != nil {
		m.instanceTable.SetWidth(m.detailTableWidth())
		m.instanceTable.SetHeight(m.detailTableHeight())
	}
}

// getSelectedProcess returns the name of the currently selected process
func (m *Model) getSelectedProcess() string {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.tableData) {
		return m.tableData[m.selectedIndex].name
	}
	return ""
}

func (m *Model) getSelectedProcessInfo() *processDisplayRow {
	idx := m.selectedIndex
	if idx >= 0 && idx < len(m.tableData) {
		return &m.tableData[idx]
	}
	return nil
}

func (m *Model) getSelectedInstanceID() string {
	if m.instanceTable.Columns() == nil || len(m.instanceTable.Rows()) == 0 {
		return ""
	}
	row := m.instanceTable.SelectedRow()
	if len(row) > 0 {
		return row[0]
	}
	return ""
}

func (m Model) handleQuickScale(delta int) (tea.Model, tea.Cmd) {
	info := m.getSelectedProcessInfo()
	if info == nil {
		m.showToast("✗ No process selected", 3*time.Second)
		return m, nil
	}

	target := info.name
	return m, func() tea.Msg {
		if m.isRemote {
			if err := m.client.ScaleProcessDelta(target, delta); err != nil {
				return actionResultMsg{success: false, message: fmt.Sprintf("✗ Scale failed: %v", err)}
			}
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := m.manager.AdjustScale(ctx, target, delta); err != nil {
				return actionResultMsg{success: false, message: fmt.Sprintf("✗ Scale failed: %v", err)}
			}
		}

		return actionResultMsg{success: true, message: fmt.Sprintf("✓ Scaled %s by %+d", target, delta)}
	}
}

func (m *Model) openProcessDetail(name string) {
	m.detailProc = name
	if m.instanceTable.Columns() == nil {
		m.setupInstanceTable()
	}
	m.updateInstanceTableFromCache()
	m.currentView = viewProcessDetail
}

func (m *Model) updateInstanceTableFromCache() {
	if m.detailProc == "" {
		m.updateInstanceTable(nil)
		return
	}
	if info, ok := m.processCache[m.detailProc]; ok {
		procCopy := info
		m.updateInstanceTable(&procCopy)
	} else {
		m.updateInstanceTable(nil)
	}
}

// openLogView switches to the log view with the provided scope
func (m *Model) openLogView(scope logScope, processName string, instance string) tea.Cmd {
	m.logScope = scope
	m.logsPaused = false
	m.logBuffer = []string{}
	if scope == logScopeProcess {
		m.selectedProc = processName
		m.logInstance = instance
	} else {
		m.selectedProc = ""
		m.logInstance = ""
	}
	m.logReturn = m.currentView
	m.currentView = viewLogs
	return m.startLogTailing()
}

// startLogTailing begins tailing logs for the selected process
func (m *Model) startLogTailing() tea.Cmd {
	// Fetch initial logs
	m.refreshLogs()

	// Start periodic log refresh
	return tickLogRefreshCmd()
}

// refreshLogs fetches and displays logs for the selected process
func (m *Model) refreshLogs() {
	// Fetch logs (embedded or remote mode)
	var logs []logger.LogEntry
	var err error

	const logLimit = 100

	switch m.logScope {
	case logScopeStack:
		if m.isRemote {
			if m.client == nil {
				m.logBuffer = []string{"API client not initialized"}
				m.logViewport.SetContent("API client not initialized")
				return
			}
			logs, err = m.client.GetStackLogs(logLimit)
		} else {
			if m.manager == nil {
				m.logBuffer = []string{"Manager not initialized"}
				m.logViewport.SetContent("Manager not initialized")
				return
			}
			logs = m.manager.GetStackLogs(logLimit)
		}
	case logScopeProcess:
		if m.selectedProc == "" {
			m.logBuffer = []string{"No process selected"}
			m.logViewport.SetContent("No process selected")
			return
		}

		if m.isRemote {
			if m.client == nil {
				m.logBuffer = []string{"API client not initialized"}
				m.logViewport.SetContent("API client not initialized")
				return
			}
			logs, err = m.client.GetLogs(m.selectedProc, logLimit)
		} else {
			if m.manager == nil {
				m.logBuffer = []string{"Manager not initialized"}
				m.logViewport.SetContent("Manager not initialized")
				return
			}
			logs, err = m.manager.GetLogs(m.selectedProc, logLimit)
		}

		if m.logInstance != "" {
			filtered := make([]logger.LogEntry, 0, len(logs))
			for _, entry := range logs {
				if entry.InstanceID == m.logInstance {
					filtered = append(filtered, entry)
				}
			}
			logs = filtered
		}
	}

	if err != nil {
		m.logBuffer = []string{
			fmt.Sprintf("Error fetching logs: %v", err),
		}
		m.logViewport.SetContent(strings.Join(m.logBuffer, "\n"))
		return
	}

	// Format logs for display (oldest first)
	m.logBuffer = make([]string, 0, len(logs))
	for i := len(logs) - 1; i >= 0; i-- {
		entry := logs[i]
		// Format: [timestamp] [level] [stream] [instance] message
		timestamp := entry.Timestamp.Format("15:04:05.000")
		levelStr := m.formatLogLevel(entry.Level)
		stream := entry.Stream
		instance := entry.InstanceID
		if entry.ProcessName != "" {
			instance = fmt.Sprintf("%s/%s", entry.ProcessName, entry.InstanceID)
		}

		line := fmt.Sprintf("[%s] %s [%s] [%s] %s",
			timestamp,
			levelStr,
			stream,
			instance,
			entry.Message,
		)
		m.logBuffer = append(m.logBuffer, line)
	}

	if len(m.logBuffer) == 0 {
		m.logBuffer = []string{"No logs available yet. Logs will appear as the process runs."}
	}

	// Update viewport content
	m.logViewport.SetContent(strings.Join(m.logBuffer, "\n"))

	// Auto-scroll to bottom if not paused
	if !m.logsPaused {
		m.logViewport.GotoBottom()
	}
}

// formatLogLevel adds color styling to log levels
func (m *Model) formatLogLevel(level string) string {
	switch level {
	case "error":
		return errorStyle.Render("ERROR")
	case "warn":
		return warnStyle.Render("WARN ")
	case "info":
		return successStyle.Render("INFO ")
	case "debug":
		return dimStyle.Render("DEBUG")
	default:
		return level
	}
}

// handleConfirmationKeys handles keys in confirmation dialog
func (m Model) handleConfirmationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		// Confirm action
		return m, m.executeAction()

	case "n", "N", "esc":
		// Cancel action
		m.cancelConfirmation()
		return m, nil
	}

	return m, nil
}

// handleScaleDialogKeys handles keys in scale dialog
func (m Model) handleScaleDialogKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Execute scale with entered value
		return m, m.executeScale()

	case "esc":
		// Cancel
		m.closeScaleDialog()
		return m, nil

	case "backspace":
		// Remove last character
		if len(m.scaleInput) > 0 {
			m.scaleInput = m.scaleInput[:len(m.scaleInput)-1]
		}

	default:
		// Add digit
		if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
			m.scaleInput += msg.String()
		}
	}

	return m, nil
}

// executeScale executes scale action with user input
func (m *Model) executeScale() tea.Cmd {
	if m.scaleInput == "" || m.pendingTarget == "" {
		m.closeScaleDialog()
		return nil
	}

	// Parse scale value
	var desired int
	if _, err := fmt.Sscanf(m.scaleInput, "%d", &desired); err != nil || desired < 1 {
		m.showToast("✗ Invalid scale value (must be >= 1)", 3*time.Second)
		m.closeScaleDialog()
		return nil
	}

	target := m.pendingTarget
	m.closeScaleDialog()

	return m.scaleProcess(target, desired)
}

// handleWizardKeys handles keys in the wizard view
func (m Model) handleWizardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.wizardStep == 0 && m.wizardNameLocked {
		m.wizardStep = 1
	}

	key := msg.String()

	// Navigation keys
	switch key {
	case "esc":
		// Cancel wizard
		m.currentView = viewProcessList
		m.resetWizard()
		return m, nil

	case "tab", "enter":
		// Advance to next step (with validation)
		if m.wizardStep == 5 {
			// Final step - create process
			return m, m.executeWizardSubmit()
		}
		m.advanceWizardStep()
		return m, nil

	case "shift+tab":
		// Go back to previous step
		m.previousWizardStep()
		return m, nil
	}

	// Step-specific input handling
	switch m.wizardStep {
	case 0: // Process name
		return m.handleWizardNameInput(msg)
	case 1: // Command
		return m.handleWizardCommandInput(msg)
	case 2: // Scale
		return m.handleWizardScaleInput(msg)
	case 3: // Restart policy
		return m.handleWizardRestartInput(msg)
	case 4: // Preview/confirm - handled by enter key above
		return m, nil
	}

	return m, nil
}

// handleWizardNameInput handles input for process name step
func (m Model) handleWizardNameInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.wizardNameLocked {
		return m, nil
	}

	key := msg.String()

	switch key {
	case "backspace":
		if len(m.wizardName) > 0 {
			m.wizardName = m.wizardName[:len(m.wizardName)-1]
		}
	default:
		// Add character if printable
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 && key[0] != ' ' {
			m.wizardName += key
		}
	}

	return m, nil
}

// handleWizardCommandInput handles input for command step
func (m Model) handleWizardCommandInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "backspace":
		if len(m.wizardCommandLine) > 0 {
			m.wizardCommandLine = m.wizardCommandLine[:len(m.wizardCommandLine)-1]
		}

	default:
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			m.wizardCommandLine += key
		} else if key == " " {
			m.wizardCommandLine += " "
		}
	}

	return m, nil
}

// handleWizardScaleInput handles input for scale step
func (m Model) handleWizardScaleInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "backspace":
		if len(m.wizardScaleInput) > 0 {
			m.wizardScaleInput = m.wizardScaleInput[:len(m.wizardScaleInput)-1]
			if m.wizardScaleInput == "" {
				m.wizardScale = 1
			} else {
				fmt.Sscanf(m.wizardScaleInput, "%d", &m.wizardScale)
			}
		}

	default:
		// Add digit
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			m.wizardScaleInput += key
			fmt.Sscanf(m.wizardScaleInput, "%d", &m.wizardScale)
		}
	}

	return m, nil
}

// handleWizardRestartInput handles input for restart policy step
func (m Model) handleWizardRestartInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "up", "k":
		// Cycle through options: always -> on-failure -> never
		switch m.wizardRestart {
		case "always":
			m.wizardRestart = "never"
		case "on-failure":
			m.wizardRestart = "always"
		case "never":
			m.wizardRestart = "on-failure"
		}

	case "down", "j":
		// Cycle through options: always -> on-failure -> never
		switch m.wizardRestart {
		case "always":
			m.wizardRestart = "on-failure"
		case "on-failure":
			m.wizardRestart = "never"
		case "never":
			m.wizardRestart = "always"
		}
	}

	return m, nil
}

// executeWizardSubmit handles create or edit submissions
func (m *Model) executeWizardSubmit() tea.Cmd {
	// Validate final step
	if !m.validateWizardStep() {
		return nil
	}

	cfg := m.wizardProcessConfig()
	targetName := m.wizardName
	if m.wizardMode == wizardModeEdit && m.wizardOriginal != "" {
		targetName = m.wizardOriginal
	}

	commandParts := splitCommandLine(m.wizardCommandLine)

	// Return to process list
	m.currentView = viewProcessList
	m.resetWizard()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var err error
		var successMsg string

		switch m.wizardMode {
		case wizardModeEdit:
			successMsg = fmt.Sprintf("✓ Updated %s", targetName)
			if m.isRemote {
				err = m.client.UpdateProcess(targetName, cfg)
			} else {
				err = m.manager.UpdateProcess(ctx, targetName, cfg)
			}
		default:
			successMsg = fmt.Sprintf("✓ Process %s created successfully", targetName)
			if m.isRemote {
				err = m.client.AddProcess(ctx, m.wizardName, commandParts, m.wizardScale, m.wizardRestart, m.wizardEnabled)
			} else {
				// Embedded mode - create config and add process
				return actionResultMsg{success: false, message: "✗ Wizard not supported in embedded mode yet"}
			}
		}

		if err != nil {
			return actionResultMsg{success: false, message: fmt.Sprintf("✗ Failed to apply process: %v", err)}
		}

		return actionResultMsg{success: true, message: successMsg}
	}
}

func (m *Model) wizardProcessConfig() *config.Process {
	var cfg config.Process
	if m.wizardMode == wizardModeEdit && m.wizardBaseConfig != nil {
		cfg = *m.wizardBaseConfig
	}

	cfg.Enabled = m.wizardEnabled
	cfg.Command = splitCommandLine(m.wizardCommandLine)
	cfg.Scale = m.wizardScale
	cfg.Restart = m.wizardRestart

	return &cfg
}

func splitCommandLine(line string) []string {
	if strings.TrimSpace(line) == "" {
		return []string{}
	}
	return strings.Fields(line)
}

// refreshProcessListCmd fetches process data asynchronously
func (m Model) refreshProcessListCmd() tea.Cmd {
	return func() tea.Msg {
		return m.fetchProcessList()
	}
}

// fetchProcessList retrieves process data for either embedded or remote mode
func (m Model) fetchProcessList() processListResultMsg {
	var (
		processes []process.ProcessInfo
		err       error
	)

	if m.isRemote {
		if m.client == nil {
			err = fmt.Errorf("API client not initialized")
		} else {
			processes, err = m.client.ListProcesses()
		}
	} else {
		if m.manager == nil {
			err = fmt.Errorf("manager not initialized")
		} else {
			processes = m.manager.ListProcesses()
		}
	}

	return processListResultMsg{
		processes: processes,
		err:       err,
	}
}

func (m Model) fetchProcessConfigCmd(name string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := m.getProcessConfig(name)
		return processConfigResultMsg{name: name, cfg: cfg, err: err}
	}
}

func (m *Model) getProcessConfig(name string) (*config.Process, error) {
	if name == "" {
		return nil, fmt.Errorf("process name is required")
	}

	if m.isRemote {
		if m.client == nil {
			return nil, fmt.Errorf("API client not initialized")
		}
		return m.client.GetProcessConfig(name)
	}

	if m.manager == nil {
		return nil, fmt.Errorf("manager not initialized")
	}

	return m.manager.GetProcessConfig(name)
}

// applyProcessListResult updates state after a process refresh completes
func (m *Model) applyProcessListResult(msg processListResultMsg) {
	if msg.err != nil {
		m.err = fmt.Errorf("failed to fetch processes: %w", msg.err)
		return
	}

	m.err = nil
	m.updateProcessTable(msg.processes)

	// Refresh process cache for detail/log views
	m.processCache = make(map[string]process.ProcessInfo, len(msg.processes))
	for _, proc := range msg.processes {
		m.processCache[proc.Name] = proc
	}

	if m.detailProc != "" {
		if info, exists := m.processCache[m.detailProc]; exists {
			procCopy := info
			m.updateInstanceTable(&procCopy)
		} else {
			m.detailProc = ""
			if m.currentView == viewProcessDetail {
				m.showToast("Selected process no longer exists", 3*time.Second)
				m.currentView = viewProcessList
			}
		}
	}

	if m.logScope == logScopeProcess && m.selectedProc != "" {
		if _, exists := m.processCache[m.selectedProc]; !exists && m.currentView == viewLogs {
			m.showToast("Process logs unavailable (process removed)", 3*time.Second)
		}
	}
}

func (m *Model) scaleProcess(target string, desired int) tea.Cmd {
	return func() tea.Msg {
		var err error

		if m.isRemote {
			err = m.client.ScaleProcess(target, desired)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err = m.manager.ScaleProcess(ctx, target, desired)
		}

		if err != nil {
			return actionResultMsg{success: false, message: fmt.Sprintf("✗ Scale failed: %v", err)}
		}

		return actionResultMsg{success: true, message: fmt.Sprintf("✓ Scaled %s to %d", target, desired)}
	}
}
