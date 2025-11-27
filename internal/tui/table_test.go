package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
)

// TestDefaultTableHeight tests table height calculation
func TestDefaultTableHeight(t *testing.T) {
	tests := []struct {
		name           string
		modelHeight    int
		expectedHeight int
	}{
		{
			name:           "normal height",
			modelHeight:    30,
			expectedHeight: 25,
		},
		{
			name:           "small height",
			modelHeight:    10,
			expectedHeight: 5,
		},
		{
			name:           "very small height",
			modelHeight:    3,
			expectedHeight: 5,
		},
		{
			name:           "zero height",
			modelHeight:    0,
			expectedHeight: 5,
		},
		{
			name:           "negative height",
			modelHeight:    -5,
			expectedHeight: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{height: tt.modelHeight}
			result := m.defaultTableHeight()
			if result != tt.expectedHeight {
				t.Errorf("defaultTableHeight() = %d, expected %d", result, tt.expectedHeight)
			}
		})
	}
}

// TestDetailTableHeight tests detail table height calculation
func TestDetailTableHeight(t *testing.T) {
	tests := []struct {
		name           string
		modelHeight    int
		expectedHeight int
	}{
		{
			name:           "normal height",
			modelHeight:    30,
			expectedHeight: 23,
		},
		{
			name:           "small height",
			modelHeight:    10,
			expectedHeight: 6,
		},
		{
			name:           "very small height",
			modelHeight:    5,
			expectedHeight: 6,
		},
		{
			name:           "zero height",
			modelHeight:    0,
			expectedHeight: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{height: tt.modelHeight}
			result := m.detailTableHeight()
			if result != tt.expectedHeight {
				t.Errorf("detailTableHeight() = %d, expected %d", result, tt.expectedHeight)
			}
		})
	}
}

// TestDetailTableWidth tests detail table width calculation
func TestDetailTableWidth(t *testing.T) {
	tests := []struct {
		name          string
		modelWidth    int
		expectedWidth int
	}{
		{
			name:          "normal width",
			modelWidth:    100,
			expectedWidth: 100,
		},
		{
			name:          "zero width",
			modelWidth:    0,
			expectedWidth: 80,
		},
		{
			name:          "negative width",
			modelWidth:    -10,
			expectedWidth: 80,
		},
		{
			name:          "small width",
			modelWidth:    50,
			expectedWidth: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{width: tt.modelWidth}
			result := m.detailTableWidth()
			if result != tt.expectedWidth {
				t.Errorf("detailTableWidth() = %d, expected %d", result, tt.expectedWidth)
			}
		})
	}
}

// TestDefaultColumns tests default column definitions
func TestDefaultColumns(t *testing.T) {
	m := &Model{}
	columns := m.defaultColumns()

	expectedTitles := []string{"NAME", "TYPE", "STATE", "HEALTH", "SCALE", "CPU", "RAM", "UPTIME", "RESTARTS"}
	expectedWidths := []int{18, 10, 14, 12, 10, 8, 10, 12, 10}

	if len(columns) != len(expectedTitles) {
		t.Fatalf("Expected %d columns, got %d", len(expectedTitles), len(columns))
	}

	for i, col := range columns {
		if col.Title != expectedTitles[i] {
			t.Errorf("Column %d: expected title %q, got %q", i, expectedTitles[i], col.Title)
		}

		if col.Width != expectedWidths[i] {
			t.Errorf("Column %d: expected width %d, got %d", i, expectedWidths[i], col.Width)
		}
	}
}

// TestComputeInstanceColumns tests instance column computation
func TestComputeInstanceColumns(t *testing.T) {
	tests := []struct {
		name           string
		modelWidth     int
		expectedTitles []string
		minFirstWidth  int
	}{
		{
			name:           "normal width",
			modelWidth:     120,
			expectedTitles: []string{"ID", "STATE", "PID", "CPU", "RAM", "UPTIME", "RESTARTS"},
			minFirstWidth:  18,
		},
		{
			name:           "wide width",
			modelWidth:     200,
			expectedTitles: []string{"ID", "STATE", "PID", "CPU", "RAM", "UPTIME", "RESTARTS"},
			minFirstWidth:  50,
		},
		{
			name:           "narrow width",
			modelWidth:     80,
			expectedTitles: []string{"ID", "STATE", "PID", "CPU", "RAM", "UPTIME", "RESTARTS"},
			minFirstWidth:  18,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{width: tt.modelWidth}
			columns := m.computeInstanceColumns()

			if len(columns) != len(tt.expectedTitles) {
				t.Fatalf("Expected %d columns, got %d", len(tt.expectedTitles), len(columns))
			}

			for i, col := range columns {
				if col.Title != tt.expectedTitles[i] {
					t.Errorf("Column %d: expected title %q, got %q", i, tt.expectedTitles[i], col.Title)
				}
			}

			// First column should get extra space
			if columns[0].Width < tt.minFirstWidth {
				t.Errorf("First column width %d should be at least %d", columns[0].Width, tt.minFirstWidth)
			}
		})
	}
}

// TestEnsureCursorVisible tests cursor visibility logic
func TestEnsureCursorVisible(t *testing.T) {
	tests := []struct {
		name              string
		tableDataLen      int
		initialOffset     int
		initialCursor     int
		modelHeight       int
		expectedOffset    int
	}{
		{
			name:           "cursor above viewport",
			tableDataLen:   20,
			initialOffset:  5,
			initialCursor:  3,
			modelHeight:    15,
			expectedOffset: 3,
		},
		{
			name:           "cursor below viewport",
			tableDataLen:   20,
			initialOffset:  0,
			initialCursor:  15,
			modelHeight:    15,
			expectedOffset: 6, // cursor(15) - height(10) + 1 = 6
		},
		{
			name:           "cursor in viewport",
			tableDataLen:   20,
			initialOffset:  5,
			initialCursor:  8,
			modelHeight:    15,
			expectedOffset: 5,
		},
		{
			name:           "offset exceeds max",
			tableDataLen:   10,
			initialOffset:  15,
			initialCursor:  5,
			modelHeight:    15,
			expectedOffset: 0,
		},
		{
			name:           "negative offset corrected",
			tableDataLen:   20,
			initialOffset:  -5,
			initialCursor:  5,
			modelHeight:    15,
			expectedOffset: 0,
		},
		{
			name:           "empty table",
			tableDataLen:   0,
			initialOffset:  5,
			initialCursor:  0,
			modelHeight:    15,
			expectedOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				tableData:     make([]processDisplayRow, tt.tableDataLen),
				tableOffset:   tt.initialOffset,
				selectedIndex: tt.initialCursor,
				height:        tt.modelHeight,
			}

			m.ensureCursorVisible()

			if m.tableOffset != tt.expectedOffset {
				t.Errorf("tableOffset = %d, expected %d", m.tableOffset, tt.expectedOffset)
			}

			if m.tableOffset < 0 {
				t.Errorf("tableOffset should not be negative, got %d", m.tableOffset)
			}
		})
	}
}

// TestSetupProcessTable tests process table initialization
func TestSetupProcessTable(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		height         int
		initialCursor  int
		expectFocused  bool
	}{
		{
			name:          "normal setup",
			width:         100,
			height:        30,
			initialCursor: -1,
			expectFocused: true,
		},
		{
			name:          "small dimensions",
			width:         50,
			height:        10,
			initialCursor: 0,
			expectFocused: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				width:         tt.width,
				height:        tt.height,
				selectedIndex: tt.initialCursor,
			}

			m.setupProcessTable()

			if m.processTable.Columns() == nil {
				t.Error("Expected columns to be set")
			}

			if m.selectedIndex < 0 {
				t.Errorf("selectedIndex should be >= 0, got %d", m.selectedIndex)
			}
		})
	}
}

// TestSetupInstanceTable tests instance table initialization
func TestSetupInstanceTable(t *testing.T) {
	m := &Model{
		width:  100,
		height: 30,
	}

	m.setupInstanceTable()

	if m.instanceTable.Columns() == nil {
		t.Error("Expected columns to be set")
	}

	if m.instanceColumns == nil {
		t.Error("Expected instance columns to be set")
	}

	if len(m.instanceColumns) == 0 {
		t.Error("Expected non-empty instance columns")
	}
}

// TestRenderProcessTable tests process table rendering
func TestRenderProcessTable(t *testing.T) {
	tests := []struct {
		name             string
		tableDataLen     int
		expectNoProcess  bool
	}{
		{
			name:            "empty table",
			tableDataLen:    0,
			expectNoProcess: true,
		},
		{
			name:            "table with data",
			tableDataLen:    5,
			expectNoProcess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				tableData:        make([]processDisplayRow, tt.tableDataLen),
				tableColumnWidths: []int{18, 10, 14, 12, 10, 8, 10, 12, 10},
				width:            100,
				height:           30,
			}

			// Initialize with sample data if not empty
			if tt.tableDataLen > 0 {
				for i := 0; i < tt.tableDataLen; i++ {
					m.tableData[i] = processDisplayRow{
						name:        "test-proc",
						procType:    "service",
						state:       "running",
						health:      "healthy",
						scale:       "1/1",
						cpuUsage:    "2.5%",
						memoryUsage: "128 MB",
						uptime:      "1h 30m",
						restarts:    "0",
					}
				}
			}

			result := m.renderProcessTable()

			if tt.expectNoProcess {
				if result != "No processes found" {
					t.Errorf("Expected 'No processes found', got %q", result)
				}
			} else {
				if len(result) == 0 {
					t.Error("Expected non-empty result")
				}
				// Should contain header
				if !containsAny(result, []string{"NAME", "TYPE", "STATE"}) {
					t.Error("Expected result to contain table headers")
				}
			}
		})
	}
}

// Helper function for tests
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// TestUpdateInstanceTable tests instance table updates
func TestUpdateInstanceTable(t *testing.T) {
	m := &Model{
		width:  100,
		height: 30,
	}

	// Test with nil process - should initialize table
	m.updateInstanceTable(nil)

	// Cursor should be at 0 or -1 for empty rows (bubbletea default behavior)
	cursor := m.instanceTable.Cursor()
	if cursor != 0 && cursor != -1 {
		t.Errorf("Expected cursor to be at 0 or -1 for nil process, got %d", cursor)
	}

	// Verify table was updated
	if m.instanceTable.Columns() == nil {
		t.Error("Expected columns to be set")
	}

	// Verify rows are empty for nil process
	rows := m.instanceTable.Rows()
	if len(rows) != 0 {
		t.Errorf("Expected 0 rows for nil process, got %d", len(rows))
	}
}

// TestProcessTableState tests process table state management
func TestProcessTableState(t *testing.T) {
	m := &Model{
		width:         100,
		height:        30,
		selectedIndex: -1,
	}

	// Initialize with previous rows
	m.setupProcessTable()
	prevRows := []table.Row{{"process1"}, {"process2"}}
	m.processTable.SetRows(prevRows)

	// Setup again - should preserve rows
	m.setupProcessTable()

	rows := m.processTable.Rows()
	if len(rows) != len(prevRows) {
		t.Errorf("Expected %d rows to be preserved, got %d", len(prevRows), len(rows))
	}
}
