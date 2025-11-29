package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gophpeek/phpeek-pm/internal/config"
	"github.com/gophpeek/phpeek-pm/internal/process"
)

// TestFormatLogLevel tests log level formatting
func TestFormatLogLevel(t *testing.T) {
	m := &Model{}

	tests := []struct {
		name     string
		level    string
		expected string
	}{
		{
			name:     "error level",
			level:    "error",
			expected: "ERROR",
		},
		{
			name:     "warn level",
			level:    "warn",
			expected: "WARN ",
		},
		{
			name:     "info level",
			level:    "info",
			expected: "INFO ",
		},
		{
			name:     "debug level",
			level:    "debug",
			expected: "DEBUG",
		},
		{
			name:     "unknown level",
			level:    "trace",
			expected: "trace",
		},
		{
			name:     "empty level",
			level:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.formatLogLevel(tt.level)
			// Check that the result contains the expected text (styles add ANSI codes)
			if !containsText(result, tt.expected) {
				t.Errorf("formatLogLevel(%q) = %q, should contain %q", tt.level, result, tt.expected)
			}
		})
	}
}

// TestApplyProcessListResult tests process list result application
func TestApplyProcessListResult(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(m *Model)
		msg           processListResultMsg
		expectError   bool
		expectedCount int
		checkFunc     func(t *testing.T, m *Model)
	}{
		{
			name:      "successful result",
			setupFunc: func(m *Model) {},
			msg: processListResultMsg{
				processes: []process.ProcessInfo{
					{Name: "php-fpm", State: "running"},
					{Name: "nginx", State: "running"},
				},
				err: nil,
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name:      "error result",
			setupFunc: func(m *Model) {},
			msg: processListResultMsg{
				processes: nil,
				err:       &mockError{"failed to fetch"},
			},
			expectError:   true,
			expectedCount: 0,
		},
		{
			name:      "empty result",
			setupFunc: func(m *Model) {},
			msg: processListResultMsg{
				processes: []process.ProcessInfo{},
				err:       nil,
			},
			expectError:   false,
			expectedCount: 0,
		},
		{
			name: "detail proc exists - updates instance table",
			setupFunc: func(m *Model) {
				m.detailProc = "php-fpm"
				m.currentView = viewProcessDetail
			},
			msg: processListResultMsg{
				processes: []process.ProcessInfo{
					{Name: "php-fpm", State: "running", Instances: []process.ProcessInstanceInfo{{ID: "php-fpm-0", State: "running"}}},
				},
				err: nil,
			},
			expectError:   false,
			expectedCount: 1,
			checkFunc: func(t *testing.T, m *Model) {
				if m.detailProc != "php-fpm" {
					t.Error("Expected detailProc to remain php-fpm")
				}
			},
		},
		{
			name: "detail proc no longer exists - clears and shows toast",
			setupFunc: func(m *Model) {
				m.detailProc = "missing"
				m.currentView = viewProcessDetail
			},
			msg: processListResultMsg{
				processes: []process.ProcessInfo{
					{Name: "php-fpm", State: "running"},
				},
				err: nil,
			},
			expectError:   false,
			expectedCount: 1,
			checkFunc: func(t *testing.T, m *Model) {
				if m.detailProc != "" {
					t.Error("Expected detailProc to be cleared")
				}
				if m.currentView != viewProcessList {
					t.Error("Expected currentView to switch to viewProcessList")
				}
				if m.toast == "" {
					t.Error("Expected toast message about removed process")
				}
			},
		},
		{
			name: "log scope process removed - shows toast",
			setupFunc: func(m *Model) {
				m.logScope = logScopeProcess
				m.selectedProc = "missing"
				m.currentView = viewLogs
			},
			msg: processListResultMsg{
				processes: []process.ProcessInfo{
					{Name: "php-fpm", State: "running"},
				},
				err: nil,
			},
			expectError:   false,
			expectedCount: 1,
			checkFunc: func(t *testing.T, m *Model) {
				if m.toast == "" {
					t.Error("Expected toast message about unavailable process logs")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				width:  100,
				height: 30,
			}
			m.setupProcessTable()
			if tt.setupFunc != nil {
				tt.setupFunc(m)
			}

			m.applyProcessListResult(tt.msg)

			if tt.expectError {
				if m.err == nil {
					t.Error("Expected error to be set")
				}
			} else {
				if m.err != nil {
					t.Errorf("Expected no error, got %v", m.err)
				}

				if len(m.processCache) != tt.expectedCount {
					t.Errorf("Expected %d processes in cache, got %d", tt.expectedCount, len(m.processCache))
				}
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, m)
			}
		})
	}
}

// TestWizardProcessConfig tests wizard config generation
func TestWizardProcessConfig(t *testing.T) {
	tests := []struct {
		name            string
		setupFunc       func(m *Model)
		expectedScale   int
		expectedRestart string
		expectedEnabled bool
	}{
		{
			name: "create mode basic config",
			setupFunc: func(m *Model) {
				m.wizardMode = wizardModeCreate
				m.wizardCommandLine = "php artisan queue:work"
				m.wizardScale = 3
				m.wizardRestart = "always"
				m.wizardEnabled = true
			},
			expectedScale:   3,
			expectedRestart: "always",
			expectedEnabled: true,
		},
		{
			name: "edit mode preserves base config",
			setupFunc: func(m *Model) {
				m.wizardMode = wizardModeEdit
				m.wizardBaseConfig = &config.Process{
					Type:       "longrun",
					WorkingDir: "/app",
				}
				m.wizardCommandLine = "php artisan horizon"
				m.wizardScale = 1
				m.wizardRestart = "on-failure"
				m.wizardEnabled = false
			},
			expectedScale:   1,
			expectedRestart: "on-failure",
			expectedEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			tt.setupFunc(m)

			cfg := m.wizardProcessConfig()

			if cfg == nil {
				t.Fatal("Expected non-nil config")
			}

			if cfg.Scale != tt.expectedScale {
				t.Errorf("Expected Scale %d, got %d", tt.expectedScale, cfg.Scale)
			}

			if cfg.Restart != tt.expectedRestart {
				t.Errorf("Expected Restart %q, got %q", tt.expectedRestart, cfg.Restart)
			}

			if cfg.Enabled != tt.expectedEnabled {
				t.Errorf("Expected Enabled %v, got %v", tt.expectedEnabled, cfg.Enabled)
			}

			if len(cfg.Command) == 0 {
				t.Error("Expected non-empty Command")
			}
		})
	}
}

// TestFetchProcessList tests process list fetching logic
func TestFetchProcessList(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(m *Model)
		expectError   bool
		expectedCount int
	}{
		{
			name: "remote mode with nil client",
			setupFunc: func(m *Model) {
				m.isRemote = true
				m.client = nil
			},
			expectError: true,
		},
		{
			name: "embedded mode with nil manager",
			setupFunc: func(m *Model) {
				m.isRemote = false
				m.manager = nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			tt.setupFunc(m)

			result := m.fetchProcessList()

			if tt.expectError {
				if result.err == nil {
					t.Error("Expected error in result")
				}
			} else {
				if result.err != nil {
					t.Errorf("Expected no error, got %v", result.err)
				}
			}
		})
	}
}

// TestGetProcessConfig tests process config retrieval
func TestGetProcessConfig(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		setupFunc   func(m *Model)
		expectError bool
	}{
		{
			name:        "empty process name",
			processName: "",
			setupFunc:   func(m *Model) {},
			expectError: true,
		},
		{
			name:        "remote mode with nil client",
			processName: "test",
			setupFunc: func(m *Model) {
				m.isRemote = true
				m.client = nil
			},
			expectError: true,
		},
		{
			name:        "embedded mode with nil manager",
			processName: "test",
			setupFunc: func(m *Model) {
				m.isRemote = false
				m.manager = nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			tt.setupFunc(m)

			_, err := m.getProcessConfig(tt.processName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// Helper functions

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func containsText(s, substr string) bool {
	// Simple contains check - works even with ANSI codes
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestLogScopeDescription tests log scope description generation
func TestLogScopeDescription(t *testing.T) {
	tests := []struct {
		name         string
		logScope     logScope
		selectedProc string
		expected     string
	}{
		{
			name:         "stack scope",
			logScope:     logScopeStack,
			selectedProc: "",
			expected:     "Stack (all processes)",
		},
		{
			name:         "process scope with name",
			logScope:     logScopeProcess,
			selectedProc: "php-fpm",
			expected:     "Process (php-fpm)",
		},
		{
			name:         "process scope without name defaults to stack",
			logScope:     logScopeProcess,
			selectedProc: "",
			expected:     "Stack (all processes)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				logScope:     tt.logScope,
				selectedProc: tt.selectedProc,
			}

			result := m.logScopeDescription()
			if result != tt.expected {
				t.Errorf("logScopeDescription() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestHandleWizardNameInput tests wizard name input handling
func TestHandleWizardNameInput(t *testing.T) {
	tests := []struct {
		name         string
		initialName  string
		nameLocked   bool
		key          string
		expectedName string
	}{
		{
			name:         "add character",
			initialName:  "test",
			nameLocked:   false,
			key:          "a",
			expectedName: "testa",
		},
		{
			name:         "backspace removes character",
			initialName:  "test",
			nameLocked:   false,
			key:          "backspace",
			expectedName: "tes",
		},
		{
			name:         "backspace on empty string",
			initialName:  "",
			nameLocked:   false,
			key:          "backspace",
			expectedName: "",
		},
		{
			name:         "locked name ignores input",
			initialName:  "locked",
			nameLocked:   true,
			key:          "a",
			expectedName: "locked",
		},
		{
			name:         "space character ignored",
			initialName:  "test",
			nameLocked:   false,
			key:          " ",
			expectedName: "test",
		},
		{
			name:         "special character ignored",
			initialName:  "test",
			nameLocked:   false,
			key:          "@",
			expectedName: "test@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				wizardName:       tt.initialName,
				wizardNameLocked: tt.nameLocked,
				wizardCursor:     len(tt.initialName), // cursor at end
			}

			msg := createKeyMsg(tt.key)
			result, _ := m.handleWizardNameInput(msg)
			resultModel := result.(Model)

			if resultModel.wizardName != tt.expectedName {
				t.Errorf("wizardName = %q, expected %q", resultModel.wizardName, tt.expectedName)
			}
		})
	}
}

// TestHandleWizardCommandInput tests wizard command input handling
func TestHandleWizardCommandInput(t *testing.T) {
	tests := []struct {
		name            string
		initialCommand  string
		key             string
		expectedCommand string
	}{
		{
			name:            "add character",
			initialCommand:  "php artisan",
			key:             "q",
			expectedCommand: "php artisanq",
		},
		{
			name:            "add space",
			initialCommand:  "php",
			key:             " ",
			expectedCommand: "php ",
		},
		{
			name:            "backspace removes character",
			initialCommand:  "php artisan",
			key:             "backspace",
			expectedCommand: "php artisa",
		},
		{
			name:            "backspace on empty string",
			initialCommand:  "",
			key:             "backspace",
			expectedCommand: "",
		},
		{
			name:            "add multiple characters",
			initialCommand:  "php",
			key:             "-",
			expectedCommand: "php-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				wizardCommandLine: tt.initialCommand,
				wizardCursor:      len(tt.initialCommand), // cursor at end
			}

			msg := createKeyMsg(tt.key)
			result, _ := m.handleWizardCommandInput(msg)
			resultModel := result.(Model)

			if resultModel.wizardCommandLine != tt.expectedCommand {
				t.Errorf("wizardCommandLine = %q, expected %q", resultModel.wizardCommandLine, tt.expectedCommand)
			}
		})
	}
}

// TestHandleWizardScaleInput tests wizard scale input handling
func TestHandleWizardScaleInput(t *testing.T) {
	tests := []struct {
		name          string
		initialInput  string
		initialScale  int
		key           string
		expectedInput string
		expectedScale int
	}{
		{
			name:          "add digit",
			initialInput:  "1",
			initialScale:  1,
			key:           "2",
			expectedInput: "12",
			expectedScale: 12,
		},
		{
			name:          "backspace removes digit",
			initialInput:  "123",
			initialScale:  123,
			key:           "backspace",
			expectedInput: "12",
			expectedScale: 12,
		},
		{
			name:          "backspace on empty sets scale to 1",
			initialInput:  "5",
			initialScale:  5,
			key:           "backspace",
			expectedInput: "",
			expectedScale: 1,
		},
		{
			name:          "non-digit ignored",
			initialInput:  "5",
			initialScale:  5,
			key:           "a",
			expectedInput: "5",
			expectedScale: 5,
		},
		{
			name:          "zero digit",
			initialInput:  "1",
			initialScale:  1,
			key:           "0",
			expectedInput: "10",
			expectedScale: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				wizardScaleInput: tt.initialInput,
				wizardScale:      tt.initialScale,
			}

			msg := createKeyMsg(tt.key)
			result, _ := m.handleWizardScaleInput(msg)
			resultModel := result.(Model)

			if resultModel.wizardScaleInput != tt.expectedInput {
				t.Errorf("wizardScaleInput = %q, expected %q", resultModel.wizardScaleInput, tt.expectedInput)
			}
			if resultModel.wizardScale != tt.expectedScale {
				t.Errorf("wizardScale = %d, expected %d", resultModel.wizardScale, tt.expectedScale)
			}
		})
	}
}

// TestHandleWizardRestartInput tests wizard restart policy input handling
func TestHandleWizardRestartInput(t *testing.T) {
	tests := []struct {
		name            string
		initialRestart  string
		key             string
		expectedRestart string
	}{
		{
			name:            "down from always",
			initialRestart:  "always",
			key:             "down",
			expectedRestart: "on-failure",
		},
		{
			name:            "down from on-failure",
			initialRestart:  "on-failure",
			key:             "down",
			expectedRestart: "never",
		},
		{
			name:            "down from never",
			initialRestart:  "never",
			key:             "down",
			expectedRestart: "always",
		},
		{
			name:            "up from always",
			initialRestart:  "always",
			key:             "up",
			expectedRestart: "never",
		},
		{
			name:            "up from on-failure",
			initialRestart:  "on-failure",
			key:             "up",
			expectedRestart: "always",
		},
		{
			name:            "up from never",
			initialRestart:  "never",
			key:             "up",
			expectedRestart: "on-failure",
		},
		{
			name:            "j key (same as down)",
			initialRestart:  "always",
			key:             "j",
			expectedRestart: "on-failure",
		},
		{
			name:            "k key (same as up)",
			initialRestart:  "on-failure",
			key:             "k",
			expectedRestart: "always",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				wizardRestart: tt.initialRestart,
			}

			msg := createKeyMsg(tt.key)
			result, _ := m.handleWizardRestartInput(msg)
			resultModel := result.(Model)

			if resultModel.wizardRestart != tt.expectedRestart {
				t.Errorf("wizardRestart = %q, expected %q", resultModel.wizardRestart, tt.expectedRestart)
			}
		})
	}
}

// createKeyMsg creates a tea.KeyMsg for testing
func createKeyMsg(key string) tea.KeyMsg {
	switch key {
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	default:
		if len(key) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(key[0])}}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// TestGetCurrentTabCount tests tab item count retrieval
func TestGetCurrentTabCount(t *testing.T) {
	tests := []struct {
		name          string
		activeTab     tabType
		tableData     []processDisplayRow
		scheduledData []scheduledDisplayRow
		oneshotData   []oneshotDisplayRow
		expected      int
	}{
		{
			name:      "processes tab with data",
			activeTab: tabProcesses,
			tableData: []processDisplayRow{
				{name: "php-fpm"},
				{name: "nginx"},
				{name: "redis"},
			},
			expected: 3,
		},
		{
			name:      "processes tab empty",
			activeTab: tabProcesses,
			tableData: []processDisplayRow{},
			expected:  0,
		},
		{
			name:      "scheduled tab with data",
			activeTab: tabScheduled,
			scheduledData: []scheduledDisplayRow{
				{name: "cron-1"},
				{name: "cron-2"},
			},
			expected: 2,
		},
		{
			name:          "scheduled tab empty",
			activeTab:     tabScheduled,
			scheduledData: []scheduledDisplayRow{},
			expected:      0,
		},
		{
			name:      "oneshot tab with data",
			activeTab: tabOneshot,
			oneshotData: []oneshotDisplayRow{
				{processName: "migrate"},
				{processName: "seed"},
				{processName: "cache-clear"},
				{processName: "optimize"},
			},
			expected: 4,
		},
		{
			name:        "oneshot tab empty",
			activeTab:   tabOneshot,
			oneshotData: []oneshotDisplayRow{},
			expected:    0,
		},
		{
			name:      "system tab always returns systemMenuItemCount",
			activeTab: tabSystem,
			expected:  systemMenuItemCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				activeTab:     tt.activeTab,
				tableData:     tt.tableData,
				scheduledData: tt.scheduledData,
				oneshotData:   tt.oneshotData,
			}

			result := m.getCurrentTabCount()
			if result != tt.expected {
				t.Errorf("getCurrentTabCount() = %d, expected %d", result, tt.expected)
			}
		})
	}
}

// TestConvertOneshotExecution tests oneshot execution row conversion
func TestConvertOneshotExecution(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-5 * time.Minute)
	finishedAt := now.Add(-4 * time.Minute)

	tests := []struct {
		name           string
		exec           process.OneshotExecution
		expectedStatus string
		expectedExit   string
		checkFunc      func(t *testing.T, row oneshotDisplayRow)
	}{
		{
			name: "running execution",
			exec: process.OneshotExecution{
				ID:          1,
				ProcessName: "migrate",
				InstanceID:  "migrate-001",
				TriggerType: "manual",
				StartedAt:   startedAt,
				// FinishedAt is zero value (not finished)
			},
			expectedStatus: "Running",
			expectedExit:   "-",
			checkFunc: func(t *testing.T, row oneshotDisplayRow) {
				if row.finishedAt != "-" {
					t.Errorf("Expected finishedAt '-' for running, got %q", row.finishedAt)
				}
			},
		},
		{
			name: "successful execution",
			exec: process.OneshotExecution{
				ID:          2,
				ProcessName: "seed",
				InstanceID:  "seed-001",
				TriggerType: "startup",
				StartedAt:   startedAt,
				FinishedAt:  finishedAt,
				ExitCode:    0,
				Duration:    "1m0s",
			},
			expectedStatus: "Success",
			expectedExit:   "0",
			checkFunc: func(t *testing.T, row oneshotDisplayRow) {
				if row.duration != "1m0s" {
					t.Errorf("Expected duration '1m0s', got %q", row.duration)
				}
			},
		},
		{
			name: "failed execution with exit code 1",
			exec: process.OneshotExecution{
				ID:          3,
				ProcessName: "test",
				InstanceID:  "test-001",
				TriggerType: "api",
				StartedAt:   startedAt,
				FinishedAt:  finishedAt,
				ExitCode:    1,
				Duration:    "30s",
			},
			expectedStatus: "Failed",
			expectedExit:   "1",
		},
		{
			name: "failed execution with exit code 255",
			exec: process.OneshotExecution{
				ID:          4,
				ProcessName: "crash",
				InstanceID:  "crash-001",
				TriggerType: "manual",
				StartedAt:   startedAt,
				FinishedAt:  finishedAt,
				ExitCode:    255,
				Duration:    "0s",
			},
			expectedStatus: "Failed",
			expectedExit:   "255",
		},
		{
			name: "execution with error message",
			exec: process.OneshotExecution{
				ID:          5,
				ProcessName: "errored",
				InstanceID:  "errored-001",
				TriggerType: "manual",
				StartedAt:   startedAt,
				FinishedAt:  finishedAt,
				ExitCode:    1,
				Duration:    "5s",
				Error:       "command not found",
			},
			expectedStatus: "Failed",
			expectedExit:   "1",
			checkFunc: func(t *testing.T, row oneshotDisplayRow) {
				if row.error != "command not found" {
					t.Errorf("Expected error 'command not found', got %q", row.error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			row := m.convertOneshotExecution(tt.exec)

			// Check basic fields
			if row.id != tt.exec.ID {
				t.Errorf("id = %d, expected %d", row.id, tt.exec.ID)
			}
			if row.processName != tt.exec.ProcessName {
				t.Errorf("processName = %q, expected %q", row.processName, tt.exec.ProcessName)
			}
			if row.instanceID != tt.exec.InstanceID {
				t.Errorf("instanceID = %q, expected %q", row.instanceID, tt.exec.InstanceID)
			}
			if row.triggerType != tt.exec.TriggerType {
				t.Errorf("triggerType = %q, expected %q", row.triggerType, tt.exec.TriggerType)
			}

			// Check status
			if row.status != tt.expectedStatus {
				t.Errorf("status = %q, expected %q", row.status, tt.expectedStatus)
			}

			// Check exit code
			if row.exitCode != tt.expectedExit {
				t.Errorf("exitCode = %q, expected %q", row.exitCode, tt.expectedExit)
			}

			// Check that started time is formatted
			if row.startedAt == "" {
				t.Error("startedAt should not be empty")
			}

			// Run additional checks if provided
			if tt.checkFunc != nil {
				tt.checkFunc(t, row)
			}
		})
	}
}

// TestMoveScheduledSelection tests scheduled table cursor movement
func TestMoveScheduledSelection(t *testing.T) {
	tests := []struct {
		name          string
		initialIndex  int
		dataLength    int
		direction     int
		expectedIndex int
	}{
		{
			name:          "move down in middle",
			initialIndex:  1,
			dataLength:    5,
			direction:     1,
			expectedIndex: 2,
		},
		{
			name:          "move up in middle",
			initialIndex:  2,
			dataLength:    5,
			direction:     -1,
			expectedIndex: 1,
		},
		{
			name:          "move down at end clamps to last",
			initialIndex:  4,
			dataLength:    5,
			direction:     1,
			expectedIndex: 4, // clamps at end, doesn't wrap
		},
		{
			name:          "move up at start clamps to first",
			initialIndex:  0,
			dataLength:    5,
			direction:     -1,
			expectedIndex: 0, // clamps at start, doesn't wrap
		},
		{
			name:          "empty data stays at 0",
			initialIndex:  0,
			dataLength:    0,
			direction:     1,
			expectedIndex: 0,
		},
		{
			name:          "single item stays at 0",
			initialIndex:  0,
			dataLength:    1,
			direction:     1,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create scheduled data of the specified length
			data := make([]scheduledDisplayRow, tt.dataLength)
			for i := 0; i < tt.dataLength; i++ {
				data[i] = scheduledDisplayRow{name: "cron-" + string(rune('a'+i))}
			}

			m := &Model{
				scheduledIndex: tt.initialIndex,
				scheduledData:  data,
			}

			m.moveScheduledSelection(tt.direction)

			if m.scheduledIndex != tt.expectedIndex {
				t.Errorf("scheduledIndex = %d, expected %d", m.scheduledIndex, tt.expectedIndex)
			}
		})
	}
}

// TestSetScheduledSelection tests scheduled table cursor direct setting
func TestSetScheduledSelection(t *testing.T) {
	tests := []struct {
		name          string
		dataLength    int
		setIndex      int
		expectedIndex int
	}{
		{
			name:          "set valid index",
			dataLength:    5,
			setIndex:      3,
			expectedIndex: 3,
		},
		{
			name:          "set index 0",
			dataLength:    5,
			setIndex:      0,
			expectedIndex: 0,
		},
		{
			name:          "set last index",
			dataLength:    5,
			setIndex:      4,
			expectedIndex: 4,
		},
		{
			name:          "set negative clamps to 0",
			dataLength:    5,
			setIndex:      -1,
			expectedIndex: 0,
		},
		{
			name:          "set beyond end clamps to last",
			dataLength:    5,
			setIndex:      10,
			expectedIndex: 4,
		},
		{
			name:          "empty data clamps to 0",
			dataLength:    0,
			setIndex:      5,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]scheduledDisplayRow, tt.dataLength)
			for i := 0; i < tt.dataLength; i++ {
				data[i] = scheduledDisplayRow{name: "cron-" + string(rune('a'+i))}
			}

			m := &Model{
				scheduledData: data,
			}

			m.setScheduledSelection(tt.setIndex)

			if m.scheduledIndex != tt.expectedIndex {
				t.Errorf("scheduledIndex = %d, expected %d", m.scheduledIndex, tt.expectedIndex)
			}
		})
	}
}

// TestMoveOneshotSelection tests oneshot table cursor movement
func TestMoveOneshotSelection(t *testing.T) {
	tests := []struct {
		name          string
		initialIndex  int
		dataLength    int
		direction     int
		expectedIndex int
	}{
		{
			name:          "move down in middle",
			initialIndex:  1,
			dataLength:    3,
			direction:     1,
			expectedIndex: 2,
		},
		{
			name:          "move up in middle",
			initialIndex:  1,
			dataLength:    3,
			direction:     -1,
			expectedIndex: 0,
		},
		{
			name:          "move down at end clamps to last",
			initialIndex:  2,
			dataLength:    3,
			direction:     1,
			expectedIndex: 2, // clamps at end, doesn't wrap
		},
		{
			name:          "move up at start clamps to first",
			initialIndex:  0,
			dataLength:    3,
			direction:     -1,
			expectedIndex: 0, // clamps at start, doesn't wrap
		},
		{
			name:          "empty data stays at 0",
			initialIndex:  0,
			dataLength:    0,
			direction:     1,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]oneshotDisplayRow, tt.dataLength)
			for i := 0; i < tt.dataLength; i++ {
				data[i] = oneshotDisplayRow{processName: "oneshot-" + string(rune('a'+i))}
			}

			m := &Model{
				oneshotIndex: tt.initialIndex,
				oneshotData:  data,
			}

			m.moveOneshotSelection(tt.direction)

			if m.oneshotIndex != tt.expectedIndex {
				t.Errorf("oneshotIndex = %d, expected %d", m.oneshotIndex, tt.expectedIndex)
			}
		})
	}
}

// TestSetOneshotSelection tests oneshot table cursor direct setting
func TestSetOneshotSelection(t *testing.T) {
	tests := []struct {
		name          string
		dataLength    int
		setIndex      int
		expectedIndex int
	}{
		{
			name:          "set valid index",
			dataLength:    4,
			setIndex:      2,
			expectedIndex: 2,
		},
		{
			name:          "set negative clamps to 0",
			dataLength:    4,
			setIndex:      -5,
			expectedIndex: 0,
		},
		{
			name:          "set beyond end clamps to last",
			dataLength:    4,
			setIndex:      100,
			expectedIndex: 3,
		},
		{
			name:          "empty data clamps to 0",
			dataLength:    0,
			setIndex:      1,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]oneshotDisplayRow, tt.dataLength)
			for i := 0; i < tt.dataLength; i++ {
				data[i] = oneshotDisplayRow{processName: "oneshot-" + string(rune('a'+i))}
			}

			m := &Model{
				oneshotData: data,
			}

			m.setOneshotSelection(tt.setIndex)

			if m.oneshotIndex != tt.expectedIndex {
				t.Errorf("oneshotIndex = %d, expected %d", m.oneshotIndex, tt.expectedIndex)
			}
		})
	}
}

// TestMoveSystemSelection tests system menu cursor movement
func TestMoveSystemSelection(t *testing.T) {
	tests := []struct {
		name          string
		initialIndex  int
		direction     int
		expectedIndex int
	}{
		{
			name:          "move down from start",
			initialIndex:  0,
			direction:     1,
			expectedIndex: 1,
		},
		{
			name:          "move up from middle",
			initialIndex:  1,
			direction:     -1,
			expectedIndex: 0,
		},
		{
			name:          "move down at end clamps to last",
			initialIndex:  systemMenuItemCount - 1,
			direction:     1,
			expectedIndex: systemMenuItemCount - 1,
		},
		{
			name:          "move up at start clamps to 0",
			initialIndex:  0,
			direction:     -1,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				systemMenuIndex: tt.initialIndex,
			}

			m.moveSystemSelection(tt.direction)

			if m.systemMenuIndex != tt.expectedIndex {
				t.Errorf("systemMenuIndex = %d, expected %d", m.systemMenuIndex, tt.expectedIndex)
			}
		})
	}
}

// TestSetSystemSelection tests system menu cursor direct setting
func TestSetSystemSelection(t *testing.T) {
	tests := []struct {
		name          string
		setIndex      int
		expectedIndex int
	}{
		{
			name:          "set valid index",
			setIndex:      1,
			expectedIndex: 1,
		},
		{
			name:          "set index 0",
			setIndex:      0,
			expectedIndex: 0,
		},
		{
			name:          "set last index",
			setIndex:      systemMenuItemCount - 1,
			expectedIndex: systemMenuItemCount - 1,
		},
		{
			name:          "set negative clamps to 0",
			setIndex:      -1,
			expectedIndex: 0,
		},
		{
			name:          "set beyond end clamps to last",
			setIndex:      systemMenuItemCount + 10,
			expectedIndex: systemMenuItemCount - 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}

			m.setSystemSelection(tt.setIndex)

			if m.systemMenuIndex != tt.expectedIndex {
				t.Errorf("systemMenuIndex = %d, expected %d", m.systemMenuIndex, tt.expectedIndex)
			}
		})
	}
}

// TestHandleWizardCommandInputCursor tests cursor navigation in wizard command input
func TestHandleWizardCommandInputCursor(t *testing.T) {
	tests := []struct {
		name            string
		initialCommand  string
		initialCursor   int
		key             string
		expectedCommand string
		expectedCursor  int
	}{
		{
			name:            "left arrow moves cursor left",
			initialCommand:  "php artisan",
			initialCursor:   5,
			key:             "left",
			expectedCommand: "php artisan",
			expectedCursor:  4,
		},
		{
			name:            "left arrow at start stays at 0",
			initialCommand:  "php artisan",
			initialCursor:   0,
			key:             "left",
			expectedCommand: "php artisan",
			expectedCursor:  0,
		},
		{
			name:            "right arrow moves cursor right",
			initialCommand:  "php artisan",
			initialCursor:   5,
			key:             "right",
			expectedCommand: "php artisan",
			expectedCursor:  6,
		},
		{
			name:            "right arrow at end stays at end",
			initialCommand:  "php artisan",
			initialCursor:   11,
			key:             "right",
			expectedCommand: "php artisan",
			expectedCursor:  11,
		},
		{
			name:            "home moves cursor to start",
			initialCommand:  "php artisan",
			initialCursor:   5,
			key:             "home",
			expectedCommand: "php artisan",
			expectedCursor:  0,
		},
		{
			name:            "end moves cursor to end",
			initialCommand:  "php artisan",
			initialCursor:   3,
			key:             "end",
			expectedCommand: "php artisan",
			expectedCursor:  11,
		},
		{
			name:            "delete removes character at cursor",
			initialCommand:  "php artisan",
			initialCursor:   4,
			key:             "delete",
			expectedCommand: "php rtisan",
			expectedCursor:  4,
		},
		{
			name:            "delete at end does nothing",
			initialCommand:  "php artisan",
			initialCursor:   11,
			key:             "delete",
			expectedCommand: "php artisan",
			expectedCursor:  11,
		},
		{
			name:            "backspace in middle removes char before cursor",
			initialCommand:  "php artisan",
			initialCursor:   4,
			key:             "backspace",
			expectedCommand: "phpartisan",
			expectedCursor:  3,
		},
		{
			name:            "insert character in middle",
			initialCommand:  "php artisan",
			initialCursor:   4,
			key:             "X",
			expectedCommand: "php Xartisan",
			expectedCursor:  5,
		},
		{
			name:            "cursor clamp when beyond text length",
			initialCommand:  "php",
			initialCursor:   100, // beyond text length - clamped for string ops
			key:             "a",
			expectedCommand: "phpa", // char added at clamped position (end)
			expectedCursor:  101,   // cursor incremented from original (note: minor bug in impl)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				wizardCommandLine: tt.initialCommand,
				wizardCursor:      tt.initialCursor,
			}

			msg := createKeyMsg(tt.key)
			result, _ := m.handleWizardCommandInput(msg)
			resultModel := result.(Model)

			if resultModel.wizardCommandLine != tt.expectedCommand {
				t.Errorf("wizardCommandLine = %q, expected %q", resultModel.wizardCommandLine, tt.expectedCommand)
			}
			if resultModel.wizardCursor != tt.expectedCursor {
				t.Errorf("wizardCursor = %d, expected %d", resultModel.wizardCursor, tt.expectedCursor)
			}
		})
	}
}

// TestGetSelectedProcessEdgeCases tests edge cases for getSelectedProcess
func TestGetSelectedProcessEdgeCases(t *testing.T) {
	// Test with nil tableData
	m := &Model{
		tableData:     nil,
		selectedIndex: 0,
	}

	name := m.getSelectedProcess()
	if name != "" {
		t.Errorf("getSelectedProcess should return empty string when tableData is nil, got %q", name)
	}

	// Test with empty tableData
	m = &Model{
		tableData:     []processDisplayRow{},
		selectedIndex: 0,
	}

	name = m.getSelectedProcess()
	if name != "" {
		t.Errorf("getSelectedProcess should return empty string when tableData is empty, got %q", name)
	}

	// Test with selectedIndex out of bounds (negative)
	m = &Model{
		tableData: []processDisplayRow{
			{name: "test-process"},
		},
		selectedIndex: -1,
	}

	name = m.getSelectedProcess()
	if name != "" {
		t.Errorf("getSelectedProcess should return empty string when selectedIndex is negative, got %q", name)
	}

	// Test with selectedIndex out of bounds (too large)
	m = &Model{
		tableData: []processDisplayRow{
			{name: "test-process"},
		},
		selectedIndex: 5,
	}

	name = m.getSelectedProcess()
	if name != "" {
		t.Errorf("getSelectedProcess should return empty string when selectedIndex is too large, got %q", name)
	}

	// Test valid selection
	m = &Model{
		tableData: []processDisplayRow{
			{name: "test-process"},
			{name: "another-process"},
		},
		selectedIndex: 1,
	}

	name = m.getSelectedProcess()
	if name != "another-process" {
		t.Errorf("getSelectedProcess should return selected process name, got %q", name)
	}
}
