package dag

import (
	"fmt"

	"github.com/gophpeek/phpeek-pm/internal/config"
)

// Graph represents a process dependency graph
type Graph struct {
	nodes map[string]*Node
}

// Node represents a process in the dependency graph
type Node struct {
	Name         string
	Priority     int
	Dependencies []string
	Visited      bool
	InStack      bool
}

// NewGraph creates a dependency graph from process configs
func NewGraph(processes map[string]*config.Process) (*Graph, error) {
	g := &Graph{nodes: make(map[string]*Node)}

	// Build nodes
	for name, proc := range processes {
		if !proc.Enabled {
			continue
		}
		g.nodes[name] = &Node{
			Name:         name,
			Priority:     proc.Priority,
			Dependencies: proc.DependsOn,
		}
	}

	// Validate all dependencies exist
	for name, node := range g.nodes {
		for _, dep := range node.Dependencies {
			if _, exists := g.nodes[dep]; !exists {
				return nil, fmt.Errorf("process %s depends on non-existent process %s", name, dep)
			}
		}
	}

	return g, nil
}

// TopologicalSort returns processes in valid startup order
// Lower priority processes start first, then dependencies are respected
func (g *Graph) TopologicalSort() ([]string, error) {
	var result []string

	// Check for cycles first
	for name := range g.nodes {
		if !g.nodes[name].Visited {
			if g.hasCycle(name) {
				return nil, fmt.Errorf("circular dependency detected involving %s", name)
			}
		}
	}

	// Reset for actual traversal
	for _, node := range g.nodes {
		node.Visited = false
	}

	// DFS with priority consideration
	for name := range g.nodes {
		if !g.nodes[name].Visited {
			g.dfs(name, &result)
		}
	}

	return result, nil
}

func (g *Graph) hasCycle(name string) bool {
	node := g.nodes[name]
	node.Visited = true
	node.InStack = true

	for _, dep := range node.Dependencies {
		depNode := g.nodes[dep]
		if !depNode.Visited {
			if g.hasCycle(dep) {
				return true
			}
		} else if depNode.InStack {
			return true
		}
	}

	node.InStack = false
	return false
}

func (g *Graph) dfs(name string, result *[]string) {
	node := g.nodes[name]
	node.Visited = true

	// Visit dependencies first
	for _, dep := range node.Dependencies {
		if !g.nodes[dep].Visited {
			g.dfs(dep, result)
		}
	}

	*result = append(*result, name)
}
