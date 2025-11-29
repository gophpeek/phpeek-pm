package deps

import (
	"strings"
	"testing"
)

func TestGraph_SimpleChain(t *testing.T) {
	// A → B → C
	g := NewGraph()
	g.AddNode("A", []string{})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{"B"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := []string{"A", "B", "C"}
	if !equalSlices(order, expected) {
		t.Errorf("Expected order %v, got %v", expected, order)
	}
}

func TestGraph_Diamond(t *testing.T) {
	// A → B,C → D (B and C should start in parallel, alphabetical order)
	g := NewGraph()
	g.AddNode("A", []string{})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{"A"})
	g.AddNode("D", []string{"B", "C"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// A must be first, D must be last
	if order[0] != "A" {
		t.Errorf("Expected A first, got %s", order[0])
	}
	if order[3] != "D" {
		t.Errorf("Expected D last, got %s", order[3])
	}

	// B and C should be in alphabetical order (B before C)
	bIndex := indexOf(order, "B")
	cIndex := indexOf(order, "C")
	if bIndex > cIndex {
		t.Errorf("Expected B before C (alphabetical), got order: %v", order)
	}
}

func TestGraph_ComplexDAG(t *testing.T) {
	// Complex DAG: A → B → D
	//              A → C → D
	//              A → E
	g := NewGraph()
	g.AddNode("A", []string{})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{"A"})
	g.AddNode("D", []string{"B", "C"})
	g.AddNode("E", []string{"A"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify A is first
	if order[0] != "A" {
		t.Errorf("Expected A first, got %s", order[0])
	}

	// Verify B and C come before D (since D depends on both)
	dIndex := indexOf(order, "D")
	bIndex := indexOf(order, "B")
	cIndex := indexOf(order, "C")

	if bIndex > dIndex {
		t.Errorf("Expected B before D")
	}
	if cIndex > dIndex {
		t.Errorf("Expected C before D")
	}

	// Verify E comes after A
	eIndex := indexOf(order, "E")
	if eIndex < 1 {
		t.Errorf("Expected E after A")
	}

	// With alphabetical sorting:
	// A first, then B,C,E (all depend on A, alphabetically ordered)
	// D becomes available after both B and C, so after C
	// Then D and E are in queue, alphabetically D < E, so D then E
	// Expected order: A, B, C, D, E
	expected := []string{"A", "B", "C", "D", "E"}
	if !equalSlices(order, expected) {
		t.Errorf("Expected order %v, got %v", expected, order)
	}
}

func TestGraph_SimpleCycle(t *testing.T) {
	// A → B → A (cycle)
	g := NewGraph()
	g.AddNode("A", []string{"B"})
	g.AddNode("B", []string{"A"})

	_, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("Expected cycle error, got nil")
	}

	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("Expected 'circular dependency' in error, got: %v", err)
	}
}

func TestGraph_ComplexCycle(t *testing.T) {
	// A → B → C → A (cycle)
	g := NewGraph()
	g.AddNode("A", []string{"C"})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{"B"})

	hasCycle, cycle := g.HasCycle()
	if !hasCycle {
		t.Fatal("Expected cycle to be detected")
	}

	if len(cycle) == 0 {
		t.Fatal("Expected non-empty cycle path")
	}

	// Verify cycle path contains all nodes
	if !contains(cycle, "A") || !contains(cycle, "B") || !contains(cycle, "C") {
		t.Errorf("Expected cycle to contain A, B, C, got: %v", cycle)
	}
}

func TestGraph_SelfDependency(t *testing.T) {
	// A → A (self-dependency)
	g := NewGraph()
	g.AddNode("A", []string{"A"})

	err := g.Validate()
	if err == nil {
		t.Fatal("Expected validation error for self-dependency")
	}

	if !strings.Contains(err.Error(), "self-dependency") {
		t.Errorf("Expected 'self-dependency' in error, got: %v", err)
	}
}

func TestGraph_MissingDependency(t *testing.T) {
	// A → NonExistent
	g := NewGraph()
	g.AddNode("A", []string{"NonExistent"})

	err := g.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing dependency")
	}

	if !strings.Contains(err.Error(), "non-existent") {
		t.Errorf("Expected 'non-existent' in error, got: %v", err)
	}
}

func TestGraph_NoDependencies(t *testing.T) {
	// All processes independent, should sort alphabetically
	g := NewGraph()
	g.AddNode("C", []string{})
	g.AddNode("A", []string{})
	g.AddNode("B", []string{})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should be sorted alphabetically: A, B, C
	expected := []string{"A", "B", "C"}
	if !equalSlices(order, expected) {
		t.Errorf("Expected order %v, got %v", expected, order)
	}
}

func TestGraph_LaravelFullStack(t *testing.T) {
	// Real-world scenario: Laravel full stack
	// php-fpm → nginx, horizon, queue, scheduler
	g := NewGraph()
	g.AddNode("php-fpm", []string{})
	g.AddNode("nginx", []string{"php-fpm"})
	g.AddNode("horizon", []string{"php-fpm"})
	g.AddNode("queue-default", []string{"php-fpm"})
	g.AddNode("scheduler", []string{"php-fpm"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// php-fpm must be first
	if order[0] != "php-fpm" {
		t.Errorf("Expected php-fpm first, got %s", order[0])
	}

	// All others should come after php-fpm
	phpIndex := indexOf(order, "php-fpm")
	for _, proc := range []string{"nginx", "horizon", "queue-default", "scheduler"} {
		if indexOf(order, proc) <= phpIndex {
			t.Errorf("Expected %s after php-fpm", proc)
		}
	}

	// Verify alphabetical ordering among dependents (all depend on php-fpm)
	// horizon < nginx < queue-default < scheduler
	horizonIdx := indexOf(order, "horizon")
	nginxIdx := indexOf(order, "nginx")
	queueIdx := indexOf(order, "queue-default")
	schedIdx := indexOf(order, "scheduler")

	if horizonIdx > nginxIdx || nginxIdx > queueIdx || queueIdx > schedIdx {
		t.Errorf("Expected processes in alphabetical order, got: %v", order)
	}
}

func TestGraph_EmptyGraph(t *testing.T) {
	g := NewGraph()

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error for empty graph, got: %v", err)
	}

	if len(order) != 0 {
		t.Errorf("Expected empty result for empty graph, got: %v", order)
	}
}

func TestGraph_SingleNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", []string{})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := []string{"A"}
	if !equalSlices(order, expected) {
		t.Errorf("Expected order %v, got %v", expected, order)
	}
}

func TestGraph_TieBreakingAlphabetically(t *testing.T) {
	// Multiple nodes with same dependency level, should sort alphabetically
	// A → B, C, D
	g := NewGraph()
	g.AddNode("A", []string{})
	g.AddNode("D", []string{"A"})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{"A"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// A must be first
	if order[0] != "A" {
		t.Errorf("Expected A first, got %s", order[0])
	}

	// B < C < D (alphabetical)
	bIdx := indexOf(order, "B")
	cIdx := indexOf(order, "C")
	dIdx := indexOf(order, "D")

	if bIdx > cIdx || cIdx > dIdx {
		t.Errorf("Expected B < C < D alphabetically, got order: %v", order)
	}
}

func TestGraph_MultipleCycleDetection(t *testing.T) {
	// Two separate cycles: A → B → A and C → D → C
	g := NewGraph()
	g.AddNode("A", []string{"B"})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{"D"})
	g.AddNode("D", []string{"C"})

	hasCycle, cycle := g.HasCycle()
	if !hasCycle {
		t.Fatal("Expected cycle to be detected")
	}

	// Should detect at least one cycle
	if len(cycle) < 2 {
		t.Errorf("Expected cycle path with at least 2 nodes, got: %v", cycle)
	}
}

// Helper functions

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

// Additional comprehensive tests for edge cases

func TestGraph_DeepChain(t *testing.T) {
	// Test very deep dependency chain: A → B → C → D → E → F → G
	g := NewGraph()
	nodes := []string{"A", "B", "C", "D", "E", "F", "G"}

	g.AddNode("A", []string{})
	for i := 1; i < len(nodes); i++ {
		g.AddNode(nodes[i], []string{nodes[i-1]})
	}

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify order is exactly A, B, C, D, E, F, G
	if !equalSlices(order, nodes) {
		t.Errorf("Expected order %v, got %v", nodes, order)
	}
}

func TestGraph_MultipleMissingDependencies(t *testing.T) {
	// A depends on multiple non-existent nodes
	g := NewGraph()
	g.AddNode("A", []string{"Missing1", "Missing2", "Missing3"})

	err := g.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing dependencies")
	}

	// Should mention non-existent
	if !strings.Contains(err.Error(), "non-existent") {
		t.Errorf("Expected 'non-existent' in error, got: %v", err)
	}
}

func TestGraph_PartialMissingDependency(t *testing.T) {
	// A depends on B (exists) and C (missing)
	g := NewGraph()
	g.AddNode("A", []string{"B", "C"})
	g.AddNode("B", []string{})

	err := g.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing dependency C")
	}

	if !strings.Contains(err.Error(), "non-existent") {
		t.Errorf("Expected 'non-existent' in error, got: %v", err)
	}
}

func TestGraph_LongCycle(t *testing.T) {
	// A → B → C → D → E → A (long cycle)
	g := NewGraph()
	g.AddNode("A", []string{"E"})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{"B"})
	g.AddNode("D", []string{"C"})
	g.AddNode("E", []string{"D"})

	hasCycle, cycle := g.HasCycle()
	if !hasCycle {
		t.Fatal("Expected cycle to be detected")
	}

	// Cycle path should be non-empty
	if len(cycle) == 0 {
		t.Fatal("Expected non-empty cycle path")
	}

	// TopologicalSort should also return error
	_, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("Expected TopologicalSort to return cycle error")
	}

	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("Expected 'circular dependency' in error, got: %v", err)
	}
}

func TestGraph_DisconnectedComponents(t *testing.T) {
	// Two disconnected dependency graphs
	// Group 1: A → B
	// Group 2: C → D
	g := NewGraph()
	g.AddNode("A", []string{})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{})
	g.AddNode("D", []string{"C"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// A should come before B
	aIdx := indexOf(order, "A")
	bIdx := indexOf(order, "B")
	if aIdx > bIdx {
		t.Errorf("Expected A before B, got order: %v", order)
	}

	// C should come before D
	cIdx := indexOf(order, "C")
	dIdx := indexOf(order, "D")
	if cIdx > dIdx {
		t.Errorf("Expected C before D, got order: %v", order)
	}
}

func TestGraph_MixedCyclicAndAcyclic(t *testing.T) {
	// Some nodes form cycle (A → B → A), others are fine (C → D)
	g := NewGraph()
	g.AddNode("A", []string{"B"})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{})
	g.AddNode("D", []string{"C"})

	hasCycle, _ := g.HasCycle()
	if !hasCycle {
		t.Fatal("Expected cycle to be detected")
	}

	_, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("Expected TopologicalSort to fail with cycle")
	}
}

func TestGraph_ValidateThenSort(t *testing.T) {
	// Test that Validate() catches issues before TopologicalSort()
	g := NewGraph()
	g.AddNode("A", []string{"Missing"})

	// Validate should catch missing dependency
	err := g.Validate()
	if err == nil {
		t.Fatal("Expected validation error")
	}

	// TopologicalSort should handle this case
	_, _ = g.TopologicalSort()
	// The behavior depends on implementation - it might error or just process existing nodes
	// This test just ensures it doesn't panic
}

func TestGraph_PriorityVsDependency(t *testing.T) {
	// Test that dependencies always take precedence over alphabetical ordering
	// Z depends on nothing (would be last alphabetically)
	// A depends on Z (would be first alphabetically, but must wait for Z)
	g := NewGraph()
	g.AddNode("Z", []string{})
	g.AddNode("A", []string{"Z"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	zIdx := indexOf(order, "Z")
	aIdx := indexOf(order, "A")

	if zIdx > aIdx {
		t.Errorf("Expected Z before A (dependency), got order: %v", order)
	}
}

func TestGraph_DuplicateAddNode(t *testing.T) {
	// Adding the same node twice should update dependencies
	g := NewGraph()
	g.AddNode("A", []string{})
	g.AddNode("B", []string{"A"})
	// Re-add B with different dependency - behavior may vary
	g.AddNode("B", []string{}) // Now B has no dependencies

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Both A and B should be in the result
	if !contains(order, "A") || !contains(order, "B") {
		t.Errorf("Expected both A and B in result, got: %v", order)
	}
}

func TestGraph_ManyDependencies(t *testing.T) {
	// One node depends on many others
	g := NewGraph()
	deps := []string{}
	for i := 0; i < 10; i++ {
		nodeName := string(rune('A' + i))
		g.AddNode(nodeName, []string{})
		deps = append(deps, nodeName)
	}
	g.AddNode("Z", deps) // Z depends on A through J

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Z must be last (after all its dependencies)
	zIdx := indexOf(order, "Z")
	if zIdx != len(order)-1 {
		t.Errorf("Expected Z to be last, got index %d in order: %v", zIdx, order)
	}

	// All deps should come before Z
	for _, dep := range deps {
		depIdx := indexOf(order, dep)
		if depIdx > zIdx {
			t.Errorf("Expected %s before Z, got order: %v", dep, order)
		}
	}
}

func TestGraph_HasCycle_NoCycle(t *testing.T) {
	// Verify HasCycle returns false for acyclic graph
	g := NewGraph()
	g.AddNode("A", []string{})
	g.AddNode("B", []string{"A"})
	g.AddNode("C", []string{"B"})

	hasCycle, cycle := g.HasCycle()
	if hasCycle {
		t.Errorf("Expected no cycle, got cycle: %v", cycle)
	}
	if len(cycle) > 0 {
		t.Errorf("Expected empty cycle path for acyclic graph, got: %v", cycle)
	}
}
