package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/gophpeek/phpeek-pm/internal/process"
)

type processDisplayRow struct {
	name         string
	nameStyle    lipgloss.Style
	procType     string
	state        string
	rawState     string
	stateStyle   lipgloss.Style
	health       string
	healthStyle  lipgloss.Style
	scale        string
	currentScale int
	desiredScale int
	scaleLocked  bool
	cpuUsage     string
	memoryUsage  string
	uptime       string
	restarts     string
}

// setupProcessTable initializes the process table
func (m *Model) setupProcessTable() {
	var prevRows []table.Row
	if m.processTable.Columns() != nil {
		prevRows = m.processTable.Rows()
	}

	t := table.New(
		table.WithColumns(m.defaultColumns()),
		table.WithFocused(true),
		table.WithHeight(m.defaultTableHeight()),
		table.WithWidth(m.width),
	)

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

	t.SetStyles(s)

	if len(prevRows) > 0 {
		t.SetRows(prevRows)
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	t.SetCursor(m.selectedIndex)

	m.processTable = t
}

// setupInstanceTable initializes the instance table for process detail view
func (m *Model) setupInstanceTable() {
	m.instanceColumns = m.computeInstanceColumns()
	t := table.New(
		table.WithColumns(m.instanceColumns),
		table.WithFocused(true),
		table.WithHeight(m.detailTableHeight()),
		table.WithWidth(m.detailTableWidth()),
	)

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

	t.SetStyles(s)
	m.instanceTable = t
}

func (m *Model) defaultColumns() []table.Column {
	titles := []string{"NAME", "TYPE", "STATE", "HEALTH", "SCALE", "CPU", "RAM", "UPTIME", "RESTARTS"}
	widths := []int{18, 10, 14, 12, 10, 8, 10, 12, 10}
	columns := make([]table.Column, len(titles))
	for i, title := range titles {
		columns[i] = table.Column{Title: title, Width: widths[i]}
	}
	return columns
}

func (m *Model) defaultTableHeight() int {
	height := m.height - 5
	if height < 5 {
		return 5
	}
	return height
}

func (m *Model) detailTableHeight() int {
	height := m.height - 7
	if height < 6 {
		height = 6
	}
	return height
}

func (m *Model) detailTableWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

func (m *Model) computeInstanceColumns() []table.Column {
	titles := []string{"ID", "STATE", "PID", "CPU", "RAM", "UPTIME", "RESTARTS"}
	widths := []int{18, 12, 8, 8, 12, 12, 10}
	total := 0
	for _, w := range widths {
		total += w
	}
	available := m.detailTableWidth() - 4
	if extra := available - total; extra > 0 {
		widths[0] += extra
	}

	cols := make([]table.Column, len(titles))
	for i, title := range titles {
		cols[i] = table.Column{Title: title, Width: widths[i]}
	}
	return cols
}

// updateProcessTable updates the cached table data and column widths
func (m *Model) updateProcessTable(processes []process.ProcessInfo) {
	// Sort processes alphabetically by name for stable display order
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].Name < processes[j].Name
	})

	headers := []string{"NAME", "TYPE", "STATE", "HEALTH", "SCALE", "CPU", "RAM", "UPTIME", "RESTARTS"}

	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = lipgloss.Width(header)
	}

	displayRows := make([]processDisplayRow, 0, len(processes))
	cursorRows := make([]table.Row, 0, len(processes))

	for _, proc := range processes {
		currentInstances := len(proc.Instances)
		totalRestarts := 0
		oldestStart := int64(0)
		allHealthy := true
		hasFailed := false

		for _, inst := range proc.Instances {
			totalRestarts += inst.RestartCount
			if oldestStart == 0 || inst.StartedAt < oldestStart {
				oldestStart = inst.StartedAt
			}
			if inst.State != "running" {
				allHealthy = false
				if inst.State == "failed" {
					hasFailed = true
				}
			}
		}

		stateText, stateStyle := stateDisplay(proc.State)
		healthy := allHealthy && currentInstances > 0
		var healthText string
		var healthStyle lipgloss.Style
		switch {
		case proc.State != string(process.StateRunning) && proc.State != "running":
			healthText = "–"
			healthStyle = dimStyle
		case hasFailed:
			healthText, healthStyle = healthDisplay(false)
		case currentInstances != proc.DesiredScale:
			healthText = "⟳ Scaling"
			healthStyle = highlightStyle
		case !allHealthy:
			healthText = "⟳ Scaling"
			healthStyle = highlightStyle
		default:
			healthText, healthStyle = healthDisplay(healthy)
		}

		var nameStyle lipgloss.Style
		switch proc.State {
		case "running":
			nameStyle = successStyle
		case "failed":
			nameStyle = errorStyle
		default:
			nameStyle = warnStyle
		}

		scaleText := fmt.Sprintf("%d/%d", currentInstances, proc.DesiredScale)
		if proc.ScaleLocked {
			scaleText = "🔒 " + scaleText
		}

		row := processDisplayRow{
			name:         proc.Name,
			nameStyle:    nameStyle,
			procType:     proc.Type,
			state:        stateText,
			rawState:     proc.State,
			stateStyle:   stateStyle,
			health:       healthText,
			healthStyle:  healthStyle,
			scale:        scaleText,
			currentScale: currentInstances,
			desiredScale: proc.DesiredScale,
			scaleLocked:  proc.ScaleLocked,
			cpuUsage:     formatCPUUsage(proc.CPUPercent),
			memoryUsage:  formatMemoryUsage(proc.MemoryRSSBytes),
			uptime:       formatUptime(oldestStart),
			restarts:     fmt.Sprintf("%d", totalRestarts),
		}

		displayRows = append(displayRows, row)
		cursorRows = append(cursorRows, table.Row{proc.Name})

		values := []string{
			row.name,
			row.procType,
			row.state,
			row.health,
			row.scale,
			row.cpuUsage,
			row.memoryUsage,
			row.uptime,
			row.restarts,
		}

		for i, value := range values {
			if w := lipgloss.Width(value); w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	totalWidth := 0
	for _, w := range colWidths {
		totalWidth += w
	}

	separatorWidth := (len(headers) - 1) * len(columnSeparator)
	available := m.width - separatorWidth
	if available > totalWidth {
		colWidths[0] += available - totalWidth
		totalWidth = available
	}
	if available < totalWidth {
		available = totalWidth
	}

	m.tableData = displayRows
	m.tableColumnWidths = colWidths
	m.processTable.SetColumns(m.defaultColumns())
	m.processTable.SetWidth(available + separatorWidth)
	m.processTable.SetRows(cursorRows)
	if len(m.tableData) == 0 {
		m.selectedIndex = 0
	} else {
		if m.selectedIndex >= len(m.tableData) {
			m.selectedIndex = len(m.tableData) - 1
		}
		if m.selectedIndex < 0 {
			m.selectedIndex = 0
		}
	}
	m.processTable.SetCursor(m.selectedIndex)
	m.ensureCursorVisible()
}

// ensureCursorVisible keeps the cursor within the visible viewport
func (m *Model) ensureCursorVisible() {
	height := m.defaultTableHeight()
	if height <= 0 {
		height = 1
	}

	maxOffset := len(m.tableData) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.tableOffset > maxOffset {
		m.tableOffset = maxOffset
	}

	cursor := m.selectedIndex
	if cursor < m.tableOffset {
		m.tableOffset = cursor
	} else if cursor >= m.tableOffset+height {
		m.tableOffset = cursor - height + 1
		if m.tableOffset > maxOffset {
			m.tableOffset = maxOffset
		}
	}
	if m.tableOffset < 0 {
		m.tableOffset = 0
	}
}

// updateInstanceTable renders the instance table from process info
func (m *Model) updateInstanceTable(process *process.ProcessInfo) {
	if m.instanceTable.Columns() == nil {
		m.setupInstanceTable()
	}

	rows := []table.Row{}
	if process != nil {
		for _, inst := range process.Instances {
			stateText, _ := stateDisplay(inst.State)
			rows = append(rows, table.Row{
				inst.ID,
				stateText,
				fmt.Sprintf("%d", inst.PID),
				formatCPUUsage(inst.CPUPercent),
				formatMemoryUsage(inst.MemoryRSSBytes),
				formatUptime(inst.StartedAt),
				fmt.Sprintf("%d", inst.RestartCount),
			})
		}
	}

	m.instanceColumns = m.computeInstanceColumns()
	m.instanceTable.SetColumns(m.instanceColumns)
	m.instanceTable.SetRows(rows)
	m.instanceTable.SetWidth(m.detailTableWidth())
	m.instanceTable.SetHeight(m.detailTableHeight())
	if len(rows) == 0 {
		m.instanceTable.SetCursor(0)
	} else if m.instanceTable.Cursor() >= len(rows) {
		m.instanceTable.SetCursor(len(rows) - 1)
	}
}

// renderProcessTable renders the process list with custom alignment/colours
func (m Model) renderProcessTable() string {
	if len(m.tableData) == 0 {
		return "No processes found"
	}

	alignLeft := []bool{true, true, false, false, false, false, false, false, false}
	headers := []string{"NAME", "TYPE", "STATE", "HEALTH", "SCALE", "CPU", "RAM", "UPTIME", "RESTARTS"}

	usePlain := m.showScaleDialog || m.showConfirmation

	var b strings.Builder
	headerStyles := buildHeaderStyles(len(headers), usePlain)
	headerLine := formatRow(headers, headerStyles, m.tableColumnWidths, alignLeft)
	if usePlain {
		b.WriteString(headerLine)
	} else {
		b.WriteString(tableHeaderStyle.Render(headerLine))
	}
	b.WriteString("\n")

	height := m.defaultTableHeight()
	start := m.tableOffset
	end := start + height
	if end > len(m.tableData) {
		end = len(m.tableData)
	}

	for i := start; i < end; i++ {
		row := m.tableData[i]
		rowStyles := []lipgloss.Style{row.nameStyle, dimStyle, row.stateStyle, row.healthStyle, dimStyle, dimStyle, dimStyle, dimStyle, dimStyle}
		if usePlain {
			rowStyles = nil
		}
		line := formatRow(
			[]string{row.name, row.procType, row.state, row.health, row.scale, row.cpuUsage, row.memoryUsage, row.uptime, row.restarts},
			rowStyles,
			m.tableColumnWidths,
			alignLeft,
		)

		if i == m.selectedIndex {
			if usePlain {
				if len(line) >= 2 {
					line = "> " + line[2:]
				} else {
					line = "> " + line
				}
			} else {
				line = tableSelectedStyle.Render(line)
			}
		}

		b.WriteString(line)
		if i != end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func buildHeaderStyles(count int, usePlain bool) []lipgloss.Style {
	styles := make([]lipgloss.Style, count)
	for i := 0; i < count; i++ {
		if usePlain {
			styles[i] = lipgloss.NewStyle()
		} else {
			styles[i] = tableHeaderStyle
		}
	}
	return styles
}

const columnSeparator = "  "

func formatRow(values []string, styles []lipgloss.Style, widths []int, alignLeft []bool) string {
	parts := make([]string, len(values))
	for i, value := range values {
		width := len(value)
		if i < len(widths) {
			width = widths[i]
		}

		align := true
		if i < len(alignLeft) {
			align = alignLeft[i]
		}

		style := lipgloss.NewStyle()
		if styles != nil && i < len(styles) {
			style = styles[i]
		}

		colored := style.Render(value)

		padding := width - lipgloss.Width(value)
		if padding < 0 {
			padding = 0
		}
		spaces := strings.Repeat(" ", padding)
		if align {
			parts[i] = colored + spaces
		} else {
			parts[i] = spaces + colored
		}
	}

	return strings.Join(parts, columnSeparator)
}

// formatUptime formats a timestamp as uptime duration
func formatUptime(startedAt int64) string {
	if startedAt == 0 {
		return "-"
	}

	duration := time.Since(time.Unix(startedAt, 0))

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%ds", int(duration.Seconds()))
}

func formatCPUUsage(cpu float64) string {
	if cpu <= 0 {
		return "-"
	}
	if cpu < 10 {
		return fmt.Sprintf("%.1f%%", cpu)
	}
	return fmt.Sprintf("%.0f%%", cpu)
}

func formatMemoryUsage(bytes uint64) string {
	if bytes == 0 {
		return "-"
	}

	const (
		kilobyte = 1024
		megabyte = kilobyte * 1024
		gigabyte = megabyte * 1024
	)

	switch {
	case bytes >= gigabyte:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gigabyte))
	case bytes >= megabyte:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(megabyte))
	default:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kilobyte))
	}
}
