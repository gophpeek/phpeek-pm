package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gophpeek/phpeek-pm/internal/process"
)

// viewMode represents the current TUI view
type viewMode int

const (
	viewProcessList viewMode = iota
	viewLogs
	viewHelp
)

// Model is the main Bubbletea model for the TUI
type Model struct {
	manager      *process.Manager
	currentView  viewMode
	processTable table.Model
	logViewport  viewport.Model
	selectedProc string
	logsPaused   bool
	logBuffer    []string
	width        int
	height       int
	err          error
	quitting     bool
}

// NewModel creates a new TUI model
func NewModel(mgr *process.Manager) Model {
	return Model{
		manager:     mgr,
		currentView: viewProcessList,
		logBuffer:   make([]string, 0, 1000),
		logsPaused:  false,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.EnterAltScreen,
	)
}

// Messages for Bubbletea
type tickMsg time.Time
type processStateMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
