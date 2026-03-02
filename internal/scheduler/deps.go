package scheduler

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrDependencyCycle is returned by Register when the DependsOn edges would
// introduce a cycle into the dependency graph.
var ErrDependencyCycle = errors.New("dependency cycle detected")

// ErrDependencyNotFound is returned by Register when a DependsOn name references
// a process that is not registered.
var ErrDependencyNotFound = errors.New("dependency not found")

// topoCheck verifies that adding newDef to the existing set of processes does
// not introduce a cycle or reference a non-existent process name.
// Called from Register() while holding the scheduler write lock.
// processes contains all already-registered processes; newDef is NOT yet added.
func topoCheck(processes map[string]*ManagedProcess, newDef ProcessDef) error {
	// First-pass: validate all DependsOn names.
	for _, dep := range newDef.DependsOn {
		if dep == newDef.Name {
			return fmt.Errorf("%w: %s depends on itself", ErrDependencyCycle, newDef.Name)
		}
		if _, ok := processes[dep]; !ok {
			return fmt.Errorf("%w: %s depends on unregistered process %q", ErrDependencyNotFound, newDef.Name, dep)
		}
	}

	// Build in-degree map and adjacency list (dependents: parent -> []child).
	// Graph: edge A -> B means "B depends on A" (A must start before B).
	inDegree := make(map[string]int, len(processes)+1)
	dependents := make(map[string][]string, len(processes)+1)

	// Initialize all nodes with in-degree 0.
	for name := range processes {
		inDegree[name] = 0
	}
	inDegree[newDef.Name] = 0

	// Add edges from existing processes.
	for name, mp := range processes {
		for _, dep := range mp.Def.DependsOn {
			inDegree[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}
	// Add edges for the new definition.
	for _, dep := range newDef.DependsOn {
		inDegree[newDef.Name]++
		dependents[dep] = append(dependents[dep], newDef.Name)
	}

	// Kahn's BFS: start with zero in-degree nodes.
	queue := make([]string, 0, len(inDegree))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	processed := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		processed++
		for _, child := range dependents[node] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	total := len(processes) + 1 // +1 for newDef
	if processed < total {
		return fmt.Errorf("%w: registering %q would create a cycle", ErrDependencyCycle, newDef.Name)
	}
	return nil
}

// topoOrder returns processes grouped into layers where each layer's processes
// can start concurrently. Layer 0 has no dependencies; layer N depends on
// layers 0..N-1. Entries within each layer are sorted alphabetically for
// deterministic output. Returns nil, nil for an empty process map.
// Called by StartAll() while holding the scheduler read lock (or on a snapshot).
// Returns ErrDependencyCycle if a cycle is found (defensive; Register prevents this).
func topoOrder(processes map[string]*ManagedProcess) ([][]string, error) {
	if len(processes) == 0 {
		return nil, nil
	}

	inDegree := make(map[string]int, len(processes))
	dependents := make(map[string][]string, len(processes))

	for name := range processes {
		inDegree[name] = 0
	}
	for name, mp := range processes {
		for _, dep := range mp.Def.DependsOn {
			inDegree[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}

	var layers [][]string
	for {
		var layer []string
		for name, deg := range inDegree {
			if deg == 0 {
				layer = append(layer, name)
			}
		}
		if len(layer) == 0 {
			break
		}
		sort.Strings(layer) // deterministic order within a layer
		layers = append(layers, layer)
		for _, name := range layer {
			delete(inDegree, name)
			for _, child := range dependents[name] {
				inDegree[child]--
			}
		}
	}

	if len(inDegree) > 0 {
		// Defensive: Register() should have caught this.
		return nil, fmt.Errorf("%w: topoOrder found unexpected cycle", ErrDependencyCycle)
	}
	return layers, nil
}

// StartAll starts all registered processes in topological order.
// Processes within the same dependency layer are started in sequence;
// all processes in a layer must reach StateRunning before the next layer starts.
// StartAll blocks until all processes are Running or returns the first error.
func (s *Scheduler) StartAll() error {
	s.mu.RLock()
	// Snapshot processes for topoOrder — do not hold the lock during Start() calls.
	processesCopy := make(map[string]*ManagedProcess, len(s.processes))
	for k, v := range s.processes {
		processesCopy[k] = v
	}
	s.mu.RUnlock()

	layers, err := topoOrder(processesCopy)
	if err != nil {
		return err
	}

	for _, layer := range layers {
		// Start all processes in this layer.
		for _, name := range layer {
			if err := s.Start(name); err != nil {
				return fmt.Errorf("StartAll: starting %q: %w", name, err)
			}
		}
		// Wait for all processes in this layer to become Running.
		for _, name := range layer {
			if err := waitRunning(s, name, 10*time.Second); err != nil {
				return fmt.Errorf("StartAll: waiting for %q to reach Running: %w", name, err)
			}
		}
	}
	return nil
}

// waitRunning polls until the named process reaches StateRunning or timeout.
// Used internally by StartAll().
func waitRunning(s *Scheduler, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		mp, ok := s.processes[name]
		if ok && mp.State == StateRunning {
			s.mu.RUnlock()
			return nil
		}
		// Check for terminal states that will never reach Running.
		if ok && (mp.State == StateFailed || mp.State == StateStopped) {
			s.mu.RUnlock()
			return fmt.Errorf("process %q reached terminal state %s before Running", name, mp.State)
		}
		s.mu.RUnlock()
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("process %q did not reach Running within %v", name, timeout)
}
