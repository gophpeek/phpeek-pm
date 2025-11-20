package dag

import (
	"testing"

	"github.com/gophpeek/phpeek-pm/internal/config"
)

func TestNewGraph(t *testing.T) {
	tests := []struct {
		name      string
		processes map[string]*config.Process
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid graph with no dependencies",
			processes: map[string]*config.Process{
				"php-fpm": {
					Enabled:  true,
					Priority: 10,
				},
				"nginx": {
					Enabled:  true,
					Priority: 20,
				},
			},
			wantErr: false,
		},
		{
			name: "valid graph with dependencies",
			processes: map[string]*config.Process{
				"php-fpm": {
					Enabled:  true,
					Priority: 10,
				},
				"nginx": {
					Enabled:   true,
					Priority:  20,
					DependsOn: []string{"php-fpm"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid dependency - non-existent process",
			processes: map[string]*config.Process{
				"nginx": {
					Enabled:   true,
					DependsOn: []string{"php-fpm"},
				},
			},
			wantErr: true,
			errMsg:  "non-existent process",
		},
		{
			name: "disabled process not included in graph",
			processes: map[string]*config.Process{
				"php-fpm": {
					Enabled: false,
				},
				"nginx": {
					Enabled:  true,
					Priority: 20,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := NewGraph(tt.processes)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewGraph() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewGraph() unexpected error: %v", err)
				return
			}

			if graph == nil {
				t.Errorf("NewGraph() returned nil graph")
			}
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name      string
		processes map[string]*config.Process
		wantOrder []string
		wantErr   bool
	}{
		{
			name: "simple linear dependency",
			processes: map[string]*config.Process{
				"php-fpm": {
					Enabled:  true,
					Priority: 10,
				},
				"nginx": {
					Enabled:   true,
					Priority:  20,
					DependsOn: []string{"php-fpm"},
				},
			},
			wantOrder: []string{"php-fpm", "nginx"},
			wantErr:   false,
		},
		{
			name: "multiple dependencies",
			processes: map[string]*config.Process{
				"php-fpm": {
					Enabled:  true,
					Priority: 10,
				},
				"redis": {
					Enabled:  true,
					Priority: 15,
				},
				"nginx": {
					Enabled:   true,
					Priority:  20,
					DependsOn: []string{"php-fpm"},
				},
				"horizon": {
					Enabled:   true,
					Priority:  30,
					DependsOn: []string{"php-fpm", "redis"},
				},
			},
			wantOrder: []string{"php-fpm", "redis", "nginx", "horizon"},
			wantErr:   false,
		},
		{
			name: "circular dependency",
			processes: map[string]*config.Process{
				"a": {
					Enabled:   true,
					DependsOn: []string{"b"},
				},
				"b": {
					Enabled:   true,
					DependsOn: []string{"a"},
				},
			},
			wantErr: true,
		},
		{
			name: "complex circular dependency",
			processes: map[string]*config.Process{
				"a": {
					Enabled:   true,
					DependsOn: []string{"b"},
				},
				"b": {
					Enabled:   true,
					DependsOn: []string{"c"},
				},
				"c": {
					Enabled:   true,
					DependsOn: []string{"a"},
				},
			},
			wantErr: true,
		},
		{
			name: "no dependencies",
			processes: map[string]*config.Process{
				"php-fpm": {
					Enabled:  true,
					Priority: 10,
				},
				"nginx": {
					Enabled:  true,
					Priority: 20,
				},
			},
			wantOrder: []string{"php-fpm", "nginx"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := NewGraph(tt.processes)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("NewGraph() unexpected error: %v", err)
				}
				return
			}

			order, err := graph.TopologicalSort()

			if tt.wantErr {
				if err == nil {
					t.Errorf("TopologicalSort() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("TopologicalSort() unexpected error: %v", err)
				return
			}

			if len(order) != len(tt.wantOrder) {
				t.Errorf("TopologicalSort() order length = %d, want %d", len(order), len(tt.wantOrder))
				return
			}

			// Verify dependency constraints are satisfied
			pos := make(map[string]int)
			for i, name := range order {
				pos[name] = i
			}

			for name, proc := range tt.processes {
				if !proc.Enabled {
					continue
				}
				for _, dep := range proc.DependsOn {
					if pos[dep] >= pos[name] {
						t.Errorf("TopologicalSort() dependency constraint violated: %s should come before %s", dep, name)
					}
				}
			}
		})
	}
}

func TestHasCycle(t *testing.T) {
	tests := []struct {
		name      string
		processes map[string]*config.Process
		wantCycle bool
	}{
		{
			name: "no cycle",
			processes: map[string]*config.Process{
				"a": {
					Enabled: true,
				},
				"b": {
					Enabled:   true,
					DependsOn: []string{"a"},
				},
			},
			wantCycle: false,
		},
		{
			name: "simple cycle",
			processes: map[string]*config.Process{
				"a": {
					Enabled:   true,
					DependsOn: []string{"b"},
				},
				"b": {
					Enabled:   true,
					DependsOn: []string{"a"},
				},
			},
			wantCycle: true,
		},
		{
			name: "self-reference cycle",
			processes: map[string]*config.Process{
				"a": {
					Enabled:   true,
					DependsOn: []string{"a"},
				},
			},
			wantCycle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := NewGraph(tt.processes)
			if err != nil {
				t.Fatalf("NewGraph() unexpected error: %v", err)
			}

			_, err = graph.TopologicalSort()
			hasCycle := err != nil

			if hasCycle != tt.wantCycle {
				t.Errorf("hasCycle() = %v, want %v", hasCycle, tt.wantCycle)
			}
		})
	}
}
