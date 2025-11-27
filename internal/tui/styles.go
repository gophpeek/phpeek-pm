package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors (k9s-inspired)
	primaryColor   = lipgloss.Color("#7D56F4") // Purple
	successColor   = lipgloss.Color("#00FF00") // Green
	errorColor     = lipgloss.Color("#FF0000") // Red
	warnColor      = lipgloss.Color("#FFA500") // Orange
	dimColor       = lipgloss.Color("#666666") // Gray
	highlightColor = lipgloss.Color("#00FFFF") // Cyan

	// Text styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	warnStyle = lipgloss.NewStyle().
			Foreground(warnColor)

	dimStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	highlightStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	// Toast notification style
	toastStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1).
			Bold(true)

	// Dialog box style
	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			Background(lipgloss.Color("#1a1a1a")).
			Foreground(lipgloss.Color("#FFFFFF"))

	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF"))

	tableSelectedStyle = lipgloss.NewStyle().
				Background(primaryColor).
				Foreground(lipgloss.Color("#FFFFFF"))
)

// State formatters
func stateDisplay(state string) (string, lipgloss.Style) {
	switch state {
	case "running":
		return "✓ Running", successStyle
	case "starting":
		return "● Starting", highlightStyle
	case "stopped":
		return "○ Stopped", dimStyle
	case "failed":
		return "✗ Failed", errorStyle
	case "completed":
		return "✓ Completed", successStyle
	default:
		return state, dimStyle
	}
}

func healthDisplay(healthy bool) (string, lipgloss.Style) {
	if healthy {
		return "✓ Healthy", successStyle
	}
	return "⚠ Unhealthy", warnStyle
}
