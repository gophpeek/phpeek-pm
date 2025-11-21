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

	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF"))

	tableSelectedStyle = lipgloss.NewStyle().
				Background(primaryColor).
				Foreground(lipgloss.Color("#FFFFFF"))
)

// State formatters
func formatState(state string) string {
	switch state {
	case "running":
		return successStyle.Render("✓ Running")
	case "starting":
		return highlightStyle.Render("● Starting")
	case "stopped":
		return dimStyle.Render("○ Stopped")
	case "failed":
		return errorStyle.Render("✗ Failed")
	case "completed":
		return successStyle.Render("✓ Completed")
	default:
		return state
	}
}

func formatHealth(healthy bool) string {
	if healthy {
		return successStyle.Render("✓ Healthy")
	}
	return warnStyle.Render("⚠ Unhealthy")
}

func formatLogLevel(level string) string {
	switch level {
	case "ERROR", "error":
		return errorStyle.Render(level)
	case "WARN", "warn", "WARNING":
		return warnStyle.Render(level)
	case "INFO", "info":
		return successStyle.Render(level)
	case "DEBUG", "debug":
		return dimStyle.Render(level)
	default:
		return level
	}
}
