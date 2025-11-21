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
	manager      *process.Manager // For embedded mode
	client       *APIClient       // For remote mode
	isRemote     bool              // true if using API client
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

// NewModel creates a new TUI model for embedded mode
func NewModel(mgr *process.Manager) Model {
	return Model{
		manager:     mgr,
		client:      nil,
		isRemote:    false,
		currentView: viewProcessList,
		logBuffer:   make([]string, 0, 1000),
		logsPaused:  false,
	}
}

// NewRemoteModel creates a new TUI model for remote mode
func NewRemoteModel(apiURL, auth string) Model {
	return Model{
		manager:     nil,
		client:      NewAPIClient(apiURL, auth),
		isRemote:    true,
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
