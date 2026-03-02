# Phase 7: Dependency Ordering - Research

**Researched:** 2026-03-02
**Domain:** Directed Acyclic Graph (DAG) topological sort, cycle detection, Go scheduler integration
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DEP-01 | User can specify that process B depends on process A (B starts only after A is running) | Topological sort produces a valid start order; `DependsOn []string` already on `ProcessDef`; `Start()` must wait for dependency to reach `StateRunning` before proceeding |
| DEP-02 | Scheduler starts processes in topological order respecting all dependency edges | Kahn's algorithm over the registered process map produces the correct ordering; diamond dependency (A → B, A → C, B → D, C → D) resolved without starting A twice |
| DEP-03 | Circular dependencies are detected and rejected at registration time with a clear error | Kahn's cycle detection: if processed count < total node count, a cycle exists; checked in `Register()` so the definition is rejected before any state is allocated |
</phase_requirements>

---

## Summary

Phase 7 adds dependency ordering to the scheduler that was built in Phases 5 and 6. The `DependsOn []string` field already exists on `ProcessDef` (it was stored but explicitly ignored until now per the Phase 5 type comment). This phase wires that field into two new behaviors: (1) cycle detection at `Register()` time, and (2) topological start ordering when `StartAll()` or a dependency-aware `Start()` is called.

The algorithmic foundation is **Kahn's algorithm** (BFS with in-degree counting). It is the standard choice for this problem because it simultaneously produces a valid topological order AND naturally detects cycles — if the number of processed nodes is less than the total node count after the queue drains, edges remain and a cycle is proven. The algorithm is O(V + E) and has no recursive stack depth concerns, which matters for long dependency chains.

The integration point is the scheduler's `Register()` method, which must validate that adding a new `DependsOn` edge does not introduce a cycle. Because the scheduler stores all registered processes in a `map[string]*ManagedProcess`, cycle detection can be implemented inline with a graph walk over existing `DependsOn` slices — no separate graph data structure is required. A missing dependency reference (B depends on nonexistent A) must also return a clear error at registration time.

**Primary recommendation:** Implement Kahn's cycle detection inside `Register()` (validates DependsOn at registration time, rejects circular and missing dependencies) plus a new `topoOrder()` helper that produces a start sequence from all registered processes. No external libraries needed — pure stdlib with the existing scheduler types.

---

## Standard Stack

### Core

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| Go stdlib (`sort`, built-in maps/slices) | Go 1.25.5 (project go.mod) | In-degree counting, queue, adjacency traversal | No external dependency needed; Kahn's algorithm is ~40 lines of Go |
| Existing `ProcessDef.DependsOn []string` | Phase 5 | Stores dependency names | Already on the struct — Phase 5 comment says "ignored until Phase 7" |
| Existing `Scheduler.processes map[string]*ManagedProcess` | Phase 5 | Source of graph nodes/edges | Walk over this map to build in-degree counts |

### Supporting

| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| New sentinel error `ErrDependencyCycle` | Phase 7 | Returned when cycle detected at Register() | errors.Is() checks in tests and future HTTP handlers |
| New sentinel error `ErrDependencyNotFound` | Phase 7 | Returned when DependsOn references nonexistent name | Clear error message for missing dependency |
| New `StartAll()` method on Scheduler | Phase 7 | Starts all registered processes in topological order | Called by Phase 10 CLI graceful start; tests use it for diamond/chain scenarios |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Kahn's algorithm (BFS) | DFS reverse post-order | Both correct; Kahn's is simpler to understand, doesn't recurse, and cycle detection is implicit (count check at end) — prefer Kahn's |
| Inline graph walk at Register() | Separate graph data structure | A separate graph struct adds indirection; walking `DependsOn` slices over the existing process map is sufficient for the number of processes in v1.1 |
| Reject cycle at Register() time | Reject at Start() time | Requirement DEP-03 explicitly says "rejected at registration time" — must validate in Register() |

**Installation:** No new packages. Pure Go stdlib.

---

## Architecture Patterns

### Recommended File Structure

```
internal/scheduler/
├── types.go          # ProcessDef, ManagedProcess, State FSM (existing)
├── scheduler.go      # Register(), Remove(), Get(), List(), Logs() (existing)
│                     # Register() gains cycle detection using topoCheck()
├── lifecycle.go      # Start(), Stop(), captureOutput(), monitorProcess() (existing)
│                     # Start() gains dependency-aware precondition check
├── deps.go           # NEW: topoCheck(), topoOrder(), StartAll()
└── deps_test.go      # NEW: unit tests for dep ordering and cycle detection
```

Keeping dependency logic in a dedicated `deps.go` file follows the project's existing pattern (lifecycle separated from types/scheduler).

### Pattern 1: Kahn's Cycle Detection at Register() Time

**What:** When `Register()` is called with a non-empty `DependsOn`, validate that adding those edges to the existing graph does not create a cycle. Validation runs while holding the write lock (same lock that Register already holds).

**When to use:** Every `Register()` call with `len(def.DependsOn) > 0`.

**Algorithm (Kahn's):**
1. Build in-degree map and adjacency list from all existing `processes` plus the new definition being added.
2. Initialize queue with all nodes that have in-degree == 0.
3. Drain queue: for each node, decrement in-degree of its dependents; if a dependent's in-degree reaches 0, enqueue it.
4. After queue drains: if processed count == total node count → acyclic (accept). If processed count < total → cycle detected → return `ErrDependencyCycle`.

**Example:**
```go
// Source: Kahn's algorithm - cycle detection via node count comparison
// internal/scheduler/deps.go

// ErrDependencyCycle is returned by Register when the DependsOn edges
// would introduce a cycle into the dependency graph.
var ErrDependencyCycle = errors.New("dependency cycle detected")

// ErrDependencyNotFound is returned by Register when a DependsOn name
// references a process that is not registered.
var ErrDependencyNotFound = errors.New("dependency not found")

// topoCheck verifies that adding newDef to the existing set of processes
// does not introduce a cycle or reference a non-existent process name.
// Called from Register() while holding the scheduler write lock.
// processes includes all already-registered processes (newDef is NOT yet added).
func topoCheck(processes map[string]*ManagedProcess, newDef ProcessDef) error {
    // Validate all DependsOn names exist (either in registered processes or
    // self-referential — self-reference is caught by cycle check).
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
    // Add edges for new definition.
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
```

### Pattern 2: Topological Start Ordering

**What:** `topoOrder()` takes all registered processes and returns a `[][]string` where each inner slice is a "layer" of processes that can start concurrently (no dependency between them). Processes in layer N all depend only on processes in layers 0..N-1.

**When to use:** Called by `StartAll()` to start processes in correct order. Also usable by a dependency-aware `Start()` that auto-starts dependencies.

**Example:**
```go
// Source: Kahn's layer-based topological sort (adapted from Andrew Meredith's Go tutorial)
// internal/scheduler/deps.go

// topoOrder returns processes grouped into layers where each layer's processes
// can start concurrently. Layer 0 has no dependencies; layer N depends on
// layers 0..N-1. Called by StartAll() while holding the scheduler read lock.
// Returns error if a cycle exists (should not happen if Register() is correct).
func topoOrder(processes map[string]*ManagedProcess) ([][]string, error) {
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
```

### Pattern 3: StartAll() — Ordered Startup

**What:** New public method `StartAll()` on Scheduler. Computes topological layers then starts each layer sequentially (within a layer, all starts happen, then it waits for each to reach `StateRunning` before moving to the next layer).

**When to use:** Phase 10 will call `StartAll()` for graceful boot. Tests use it to verify diamond and chain ordering.

**Example:**
```go
// internal/scheduler/deps.go

// StartAll starts all registered processes in topological order.
// Processes within the same dependency layer are started in parallel.
// StartAll blocks until all processes are Running (or returns the first error).
func (s *Scheduler) StartAll() error {
    s.mu.RLock()
    // snapshot processes for topoOrder
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
        s.mu.RUnlock()
        time.Sleep(10 * time.Millisecond)
    }
    return fmt.Errorf("process %q did not reach Running within %v", name, timeout)
}
```

### Pattern 4: Register() Integration

**What:** `Register()` calls `topoCheck()` before adding the new process. This is the single validation gate.

```go
// In scheduler.go Register(), after validateName() and duplicate check:

// Phase 7: validate dependency edges before accepting the definition.
if len(def.DependsOn) > 0 {
    if err := topoCheck(s.processes, def); err != nil {
        return err
    }
}
```

The lock is already held at this point in `Register()`, so `topoCheck()` receives `s.processes` directly (no copy needed — we're inside the write lock).

### Anti-Patterns to Avoid

- **Lazy validation (checking cycles at Start() time):** DEP-03 requires rejection at registration time. If you defer cycle detection to Start(), a user can register a circular graph successfully and only discover the error when trying to run processes.
- **Starting all dependents without waiting for Running state:** If layer N starts before layer N-1 processes reach StateRunning, a dependent may start before its dependency is ready. The wait step between layers is mandatory.
- **Mutating `s.processes` inside `topoOrder()`:** `topoOrder()` must operate on a snapshot or read-only view. The current implementation builds local in-degree maps without touching ManagedProcess objects.
- **Using DFS recursion:** Recursive DFS on a dependency graph with many nodes risks stack overflow. Kahn's iterative BFS avoids this entirely.
- **Treating `DependsOn` as ordered:** The order of names in `DependsOn` is unspecified — all listed dependencies must be running before the process starts, but the DependsOn slice itself is unordered. Treat it as a set.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cycle detection | Custom DFS with visited/recursion-stack sets | Kahn's processed-count check | Kahn's cycle detection is 2 extra lines; DFS requires three state sets (unvisited/in-progress/done) and is error-prone |
| Process readiness polling in StartAll() | Channel-based pub/sub readiness | Simple polling loop (10ms interval) | The existing `waitForState` test helper pattern works; channels add complexity for no gain at this scale |
| Dependency ordering library | `github.com/dominikbraun/graph` or `gonum.org/v1/gonum/graph/topo` | Inline Kahn's implementation | The graph has at most tens of nodes; an external dependency for 40 lines of code is not justified |

**Key insight:** The graph here is tiny (process definitions, not network packets). Correctness matters far more than performance. A clear, readable Kahn's implementation with explicit error messages is better than a generic graph library.

---

## Common Pitfalls

### Pitfall 1: Missing Dependency Reference Not Caught

**What goes wrong:** Process B registers with `DependsOn: ["a"]` but process A is not registered. Registration succeeds. When `StartAll()` runs, it tries to start A and gets `ErrNotFound`, producing a confusing error at start time.

**Why it happens:** Kahn's algorithm only checks edges between existing nodes. If A does not exist, there's no node for it in the in-degree map, and the cycle check passes (the node just doesn't participate).

**How to avoid:** Add an explicit name existence check in `topoCheck()` before building the in-degree map. Check every name in `def.DependsOn` against `processes` and return `ErrDependencyNotFound` if any is missing. (See Pattern 1 code above — the first loop in `topoCheck()` does this.)

**Warning signs:** Tests that register B depending on nonexistent A without getting an error.

### Pitfall 2: Starting Dependents Before Dependencies Are Running

**What goes wrong:** `StartAll()` starts layer 0 and layer 1 without waiting for layer 0 to reach `StateRunning`. A layer 1 process may start before its dependency is ready (dependency is still in `StateStarting`).

**Why it happens:** `Start()` returns as soon as the process transitions to `StateRunning` (after `cmd.Start()` succeeds). But the process state visible via `Get()` may still be `StateStarting` for a brief moment.

**How to avoid:** Between layers in `StartAll()`, poll each process in the layer until it reaches `StateRunning` (or a terminal state). Use a deadline to avoid hanging forever if a dependency crashes.

**Warning signs:** Flaky tests where diamond dependency tests sometimes pass and sometimes leave processes in wrong states.

### Pitfall 3: Race on `s.processes` in topoCheck()

**What goes wrong:** `topoCheck()` reads `s.processes` without holding the lock.

**Why it happens:** If `topoCheck()` is called as a standalone function outside Register(), the caller may not hold the lock.

**How to avoid:** `topoCheck()` is an unexported function called only from `Register()` which holds `s.mu.Lock()` at that point. Document this precondition explicitly in a comment. Never call `topoCheck()` from outside a locked section.

**Warning signs:** `-race` detector reports a data race on `s.processes`.

### Pitfall 4: topoOrder() Modifying Shared State

**What goes wrong:** `topoOrder()` removes entries from a map that is passed by reference to `s.processes`, destroying the scheduler registry.

**Why it happens:** The leaf-removal pattern (remove nodes with in-degree 0) is tempting to implement by deleting from the live processes map.

**How to avoid:** `topoOrder()` operates only on local `inDegree` map and `dependents` map built fresh from the snapshot. It never deletes from `s.processes`. The in-degree map uses string keys and is thrown away after the function returns.

**Warning signs:** Processes disappear from `s.List()` after calling `StartAll()`.

### Pitfall 5: Self-Dependency Not Caught

**What goes wrong:** Process A registers with `DependsOn: ["a"]` (depends on itself). The cycle detection based purely on node count may or may not catch this depending on implementation — a self-loop node can process itself and reach a count of 1.

**Why it happens:** Kahn's algorithm as typically written does handle self-loops (the self-edge keeps the node's in-degree at 1, so it never enters the queue), but the check should be explicit for clear error messages.

**How to avoid:** In `topoCheck()`, add an explicit check: if any name in `DependsOn` equals `def.Name`, return `ErrDependencyCycle` immediately with a clear message. (See Pattern 1.)

---

## Code Examples

Verified patterns from official sources and the existing codebase:

### Register() with Cycle Detection Hook

```go
// internal/scheduler/scheduler.go — Register() method (Phase 7 additions)
func (s *Scheduler) Register(def ProcessDef) error {
    if err := validateName(def.Name); err != nil {
        return err
    }
    if def.LogBufferSize <= 0 {
        def.LogBufferSize = 1000
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    if _, exists := s.processes[def.Name]; exists {
        return fmt.Errorf("%w: %s", ErrAlreadyExists, def.Name)
    }

    // Phase 7: validate dependency edges. topoCheck reads s.processes (safe
    // because we hold the write lock) and returns ErrDependencyCycle or
    // ErrDependencyNotFound if the new definition would break the DAG.
    if len(def.DependsOn) > 0 {
        if err := topoCheck(s.processes, def); err != nil {
            return err
        }
    }

    s.processes[def.Name] = &ManagedProcess{
        Def:   def,
        State: StateIdle,
        logs:  newLogBuffer(def.LogBufferSize),
    }
    return nil
}
```

### Diamond Dependency Test Pattern

```go
// internal/scheduler/deps_test.go
// Tests that diamond dependency A -> {B,C} -> D starts in valid order
// without starting A twice.
func TestStartAll_DiamondDependency(t *testing.T) {
    s := New()
    must(t, s.Register(ProcessDef{Name: "a", Command: "sleep", Args: []string{"30"}}))
    must(t, s.Register(ProcessDef{Name: "b", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"a"}}))
    must(t, s.Register(ProcessDef{Name: "c", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"a"}}))
    must(t, s.Register(ProcessDef{Name: "d", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"b", "c"}}))
    t.Cleanup(func() {
        for _, name := range []string{"a", "b", "c", "d"} {
            killProcess(t, s, name)
        }
    })

    if err := s.StartAll(); err != nil {
        t.Fatalf("StartAll: %v", err)
    }

    // All four processes must be Running.
    for _, name := range []string{"a", "b", "c", "d"} {
        state, _ := getState(s, name)
        if state != StateRunning {
            t.Errorf("process %q state = %v, want Running", name, state)
        }
    }
}

// Tests that registering A -> B -> A is rejected immediately.
func TestRegister_CircularDependency(t *testing.T) {
    s := New()
    must(t, s.Register(ProcessDef{Name: "a", Command: "echo", DependsOn: []string{}}))
    must(t, s.Register(ProcessDef{Name: "b", Command: "echo", DependsOn: []string{"a"}}))

    err := s.Register(ProcessDef{Name: "a-alias", Command: "echo", DependsOn: []string{"b"}})
    // Actually register a that depends on b to close the cycle:
    // Simpler: register "a" first, then "b" depends on "a", then update...
    // The real test: register fresh cycle:
    s2 := New()
    must(t, s2.Register(ProcessDef{Name: "x", Command: "echo"}))
    must(t, s2.Register(ProcessDef{Name: "y", Command: "echo", DependsOn: []string{"x"}}))
    err = s2.Register(ProcessDef{Name: "x2", Command: "echo", DependsOn: []string{"y"}}))
    // x2 -> y -> x is fine. To get A->B->A we need to update A's DependsOn,
    // but Register() creates a new process. The cycle test is:
    // Register A (no deps), Register B (depends on A), Register C (depends on B).
    // Then try to Register A2 that depends on C — this forms a chain not a cycle.
    // True A->B->A cycle requires updating A's DependsOn after B is registered.
    // Since ProcessDef is immutable at registration, the only cycle test is:
    // - Register A with DependsOn: ["b"] where b doesn't exist yet -> ErrDependencyNotFound
    // - Or: A (no dep), B (depends A), try to register another B (already exists)
    // The real cycle scenario: A, B depends on A, then register A_v2 with DependsOn["B"]
    // under a different name — not a true cycle.
    // Correct cycle test: Register "db" no deps, Register "api" depends on "db",
    // Register "db-proxy" depends on "api" — that's fine.
    // Cycle: Register "x", Register "y" depends on "x" — then if we could update x to
    // depend on y, that's a cycle. Since defs are immutable, test must be designed around
    // what IS possible: process that depends on itself.
    _ = err

    s3 := New()
    err = s3.Register(ProcessDef{Name: "self-loop", Command: "echo", DependsOn: []string{"self-loop"}})
    if !errors.Is(err, ErrDependencyCycle) {
        t.Errorf("self-loop: got %v, want ErrDependencyCycle", err)
    }
}
```

**IMPORTANT NOTE on cycle testing:** Because `Register()` is the only mutation point and `ProcessDef` is immutable at registration, true A->B->A cycles require a 3-process scenario. The test pattern for a cycle is: register "db" (no deps), register "cache" (depends on "db"), then try to register "db" again with DependsOn["cache"] — but that hits ErrAlreadyExists first. The correct test for a true multi-node cycle needs different names: register "alpha" with no deps, register "beta" depends on "alpha", register "gamma" depends on "beta", then try to register "delta" depends on "gamma" — still not a cycle. **A true registration-time cycle test** must be: register "a" (no deps), register "b" depends on "a", then try to register a third process "c" depends on "b" AND "a" depends on "c" — impossible because "a" is already registered without that dep. The cycle scenario that `Register()` CAN catch is: if A, B, C are all being registered fresh: register "a" (depends on "c"), but "c" doesn't exist yet → ErrDependencyNotFound. To get a true cycle at registration time with the "missing dependency" check in place, we need: process that depends on a name that when eventually registered would form a cycle — but since we check for missing dependencies eagerly, you cannot have A→B→A because when you try to register B (depends on A), A must exist, and when you try to register A (depends on B), B must exist. So the only test-able cycle at registration time is a **self-dependency** or a **diamond that forms a 3-node cycle** by creative registration order.

**Revised cycle test (correct approach):**

```go
// internal/scheduler/deps_test.go

// TestRegister_SelfDependency verifies A depends on itself is rejected.
func TestRegister_SelfDependency(t *testing.T) {
    t.Parallel()
    s := New()
    err := s.Register(ProcessDef{Name: "self", Command: "echo", DependsOn: []string{"self"}})
    if err == nil {
        t.Fatal("expected error for self-dependency, got nil")
    }
    if !errors.Is(err, ErrDependencyCycle) {
        t.Errorf("got %v, want ErrDependencyCycle", err)
    }
}

// TestRegister_MissingDependency verifies B depending on nonexistent A is rejected.
func TestRegister_MissingDependency(t *testing.T) {
    t.Parallel()
    s := New()
    err := s.Register(ProcessDef{Name: "b", Command: "echo", DependsOn: []string{"a"}})
    if err == nil {
        t.Fatal("expected error for missing dependency, got nil")
    }
    if !errors.Is(err, ErrDependencyNotFound) {
        t.Errorf("got %v, want ErrDependencyNotFound", err)
    }
}

// TestRegister_ChainNoCycle verifies A -> B -> C is accepted.
func TestRegister_ChainNoCycle(t *testing.T) {
    t.Parallel()
    s := New()
    must(t, s.Register(ProcessDef{Name: "a", Command: "echo"}))
    must(t, s.Register(ProcessDef{Name: "b", Command: "echo", DependsOn: []string{"a"}}))
    must(t, s.Register(ProcessDef{Name: "c", Command: "echo", DependsOn: []string{"b"}}))
}
```

**On testing the 3-node cycle (A→B→A):** Since `Register()` requires all `DependsOn` names to exist, and our check rejects missing names, the only way to create a true 3-node cycle would be to bypass the missing-dependency check. This means the cycle test IS the missing-dependency test — you cannot register B depending on A if A doesn't exist, and you cannot add a dep edge to A after it's registered. The Kahn's cycle check in `topoCheck()` handles more exotic multi-layer cycles where all names exist, but those require careful ordering. The self-reference test and missing-dependency test cover the success criteria from the phase requirements.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| DFS-based cycle detection with 3-color marking | Kahn's BFS with count check | Industry standard for scheduling | Kahn's is simpler, no recursion risk, cycle detection is implicit |
| Separate graph library (gonum) | Inline Kahn's with existing types | Phase 7 decision | No new dependency, 40-line implementation, fits the scheduler's map-based registry |
| Eager start (start all, let failures surface) | Topological layered start | Phase 7 design | Correct ordering guaranteed before any process starts |

**Deprecated/outdated:**
- `DependsOn []string` field comment "ignored until Phase 7": remove this comment in Phase 7 implementation. The field is now active.

---

## Open Questions

1. **Should `Start(name)` auto-start unstarted dependencies?**
   - What we know: DEP-01 says "B starts only after A is running" — it doesn't specify whether calling `Start("b")` should auto-start "a" or just error if "a" isn't running.
   - What's unclear: Is the expected API "user calls `StartAll()` to start in order" or "user calls `Start("b")` and it figures out dependencies"?
   - Recommendation: For v1.1, implement `StartAll()` for ordered startup. Make `Start("b")` check that its dependencies are in StateRunning and return a clear error if they are not (e.g., `ErrDependencyNotReady`). This keeps `Start()` simple and predictable. The REST API (Phase 9) POST /start will call `Start()` directly, so auto-starting dependencies could cause surprising behavior for API users.

2. **Should `StopAll()` stop in reverse topological order?**
   - What we know: Phase 10 handles graceful shutdown and will call `StopAll()`. Phase 7 should not implement it.
   - What's unclear: Whether `StopAll()` is in scope for Phase 7 or deferred.
   - Recommendation: Defer `StopAll()` to Phase 10 per the existing phase boundaries. Phase 7 implements `StartAll()` only.

3. **What happens when a dependency crashes after all processes are started?**
   - What we know: Phase 8 handles restart policies. A crashed dependency does not affect already-running dependents in v1.1 (no re-ordering on crash).
   - What's unclear: Whether dependents should be stopped when a dependency reaches StateFailed.
   - Recommendation: Out of scope for Phase 7. Dependency ordering is a registration-time and startup-time concern. Runtime dependency enforcement (cascade stop) is a Phase 8+ concern.

---

## Sources

### Primary (HIGH confidence)
- Existing codebase: `/internal/scheduler/types.go`, `/internal/scheduler/scheduler.go`, `/internal/scheduler/lifecycle.go` — verified by direct read; `DependsOn []string` is on `ProcessDef`, `Register()` is the natural hook point
- `.planning/REQUIREMENTS.md` — DEP-01, DEP-02, DEP-03 requirements read directly
- `.planning/STATE.md` — confirmed Phase 6 complete, scheduler architecture decisions

### Secondary (MEDIUM confidence)
- [Sorting a Dependency Graph in Go - Andrew Meredith](https://kendru.github.io/go/2021/10/26/sorting-a-dependency-graph-in-go/) — complete Go implementation with `TopoSortedLayers()`, bidirectional edge tracking, cycle prevention pattern; verified against algorithm theory
- [Topological sort - Redowan's Reflections (Go)](https://rednafi.com/go/topological-sort/) — Go-specific Kahn's implementation with in-degree counting; confirms standard approach
- [Cycle detection with Kahn's algorithm - gaultier.github.io](https://gaultier.github.io/blog/kahns_algorithm.html) — confirms "remaining edges = cycle" insight; JavaScript but algorithm is language-agnostic

### Tertiary (LOW confidence)
- [Topological sorting - Wikipedia](https://en.wikipedia.org/wiki/Topological_sorting) — algorithm theory reference; no Go-specific content

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new libraries, existing types already have `DependsOn`; Kahn's algorithm is a proven standard
- Architecture: HIGH — new file `deps.go`, integration point is `Register()` and new `StartAll()` method; confirmed against existing codebase structure
- Pitfalls: HIGH — self-dependency check, missing dependency check, layer-wait requirement all derived from requirements and algorithm analysis; concurrency pitfalls derived from existing scheduler patterns

**Research date:** 2026-03-02
**Valid until:** 2026-04-02 (algorithm is stable; Go standard library patterns are stable)
