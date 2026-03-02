package scheduler

import (
	"errors"
	"sort"
	"testing"
	"time"
)

// makeMp creates a ManagedProcess with the given name and DependsOn slice.
// Used to build process maps directly for testing unexported topoCheck/topoOrder
// without going through the full Scheduler.
func makeMp(name string, deps ...string) *ManagedProcess {
	return &ManagedProcess{
		Def: ProcessDef{
			Name:      name,
			Command:   "echo",
			DependsOn: deps,
		},
		State: StateIdle,
	}
}

// sortLayers sorts the inner slices of a [][]string for stable comparison.
// Outer slice order is preserved; inner slice contents are sorted alphabetically.
func sortLayers(layers [][]string) [][]string {
	for _, layer := range layers {
		sort.Strings(layer)
	}
	return layers
}

// ---- topoCheck tests ----

// TestTopoCheck_SelfDependency verifies that a process depending on itself
// returns ErrDependencyCycle.
func TestTopoCheck_SelfDependency(t *testing.T) {
	t.Parallel()

	processes := map[string]*ManagedProcess{}
	def := ProcessDef{Name: "self", Command: "echo", DependsOn: []string{"self"}}
	err := topoCheck(processes, def)
	if err == nil {
		t.Fatal("expected ErrDependencyCycle for self-dependency, got nil")
	}
	if !errors.Is(err, ErrDependencyCycle) {
		t.Errorf("got %v, want ErrDependencyCycle", err)
	}
}

// TestTopoCheck_MissingDependency verifies that depending on an unregistered
// name returns ErrDependencyNotFound.
func TestTopoCheck_MissingDependency(t *testing.T) {
	t.Parallel()

	processes := map[string]*ManagedProcess{}
	def := ProcessDef{Name: "b", Command: "echo", DependsOn: []string{"a"}}
	err := topoCheck(processes, def)
	if err == nil {
		t.Fatal("expected ErrDependencyNotFound for missing dependency, got nil")
	}
	if !errors.Is(err, ErrDependencyNotFound) {
		t.Errorf("got %v, want ErrDependencyNotFound", err)
	}
}

// TestTopoCheck_ValidChain verifies that A -> B -> C is accepted without error.
func TestTopoCheck_ValidChain(t *testing.T) {
	t.Parallel()

	// Build processes map for A and B (already registered).
	processes := map[string]*ManagedProcess{
		"a": makeMp("a"),
		"b": makeMp("b", "a"),
	}
	// Adding C which depends on B — should succeed.
	def := ProcessDef{Name: "c", Command: "echo", DependsOn: []string{"b"}}
	err := topoCheck(processes, def)
	if err != nil {
		t.Errorf("expected nil for valid chain A->B->C, got %v", err)
	}
}

// TestTopoCheck_ValidDiamond verifies that a diamond dependency (A, B->A, C->A,
// D->B,C) is accepted without error.
func TestTopoCheck_ValidDiamond(t *testing.T) {
	t.Parallel()

	processes := map[string]*ManagedProcess{
		"a": makeMp("a"),
		"b": makeMp("b", "a"),
		"c": makeMp("c", "a"),
	}
	// Adding D which depends on B and C — should succeed.
	def := ProcessDef{Name: "d", Command: "echo", DependsOn: []string{"b", "c"}}
	err := topoCheck(processes, def)
	if err != nil {
		t.Errorf("expected nil for valid diamond, got %v", err)
	}
}

// TestTopoCheck_DuplicateDependency verifies that duplicate names in DependsOn
// are harmless and the registration succeeds.
func TestTopoCheck_DuplicateDependency(t *testing.T) {
	t.Parallel()

	processes := map[string]*ManagedProcess{
		"a": makeMp("a"),
	}
	// B depends on A twice — should succeed (duplicates are harmless in Kahn's).
	def := ProcessDef{Name: "b", Command: "echo", DependsOn: []string{"a", "a"}}
	err := topoCheck(processes, def)
	if err != nil {
		t.Errorf("expected nil for duplicate dependency, got %v", err)
	}
}

// ---- topoOrder tests ----

// TestTopoOrder_SingleProcess verifies a single no-dep process returns [["a"]].
func TestTopoOrder_SingleProcess(t *testing.T) {
	t.Parallel()

	processes := map[string]*ManagedProcess{
		"a": makeMp("a"),
	}
	layers, err := topoOrder(processes)
	if err != nil {
		t.Fatalf("topoOrder: unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d: %v", len(layers), layers)
	}
	if len(layers[0]) != 1 || layers[0][0] != "a" {
		t.Errorf("layer[0] = %v, want [\"a\"]", layers[0])
	}
}

// TestTopoOrder_Chain verifies A->B->C produces [["a"], ["b"], ["c"]].
func TestTopoOrder_Chain(t *testing.T) {
	t.Parallel()

	processes := map[string]*ManagedProcess{
		"a": makeMp("a"),
		"b": makeMp("b", "a"),
		"c": makeMp("c", "b"),
	}
	layers, err := topoOrder(processes)
	if err != nil {
		t.Fatalf("topoOrder: unexpected error: %v", err)
	}
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(layers), layers)
	}

	want := [][]string{{"a"}, {"b"}, {"c"}}
	for i, wantLayer := range want {
		if len(layers[i]) != len(wantLayer) || layers[i][0] != wantLayer[0] {
			t.Errorf("layer[%d] = %v, want %v", i, layers[i], wantLayer)
		}
	}
}

// TestTopoOrder_Diamond verifies A, B->A, C->A, D->B,C produces 3 layers:
// [["a"], ["b","c"], ["d"]].
func TestTopoOrder_Diamond(t *testing.T) {
	t.Parallel()

	processes := map[string]*ManagedProcess{
		"a": makeMp("a"),
		"b": makeMp("b", "a"),
		"c": makeMp("c", "a"),
		"d": makeMp("d", "b", "c"),
	}
	layers, err := topoOrder(processes)
	if err != nil {
		t.Fatalf("topoOrder: unexpected error: %v", err)
	}
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(layers), layers)
	}

	// Layer 0 must be ["a"].
	if len(layers[0]) != 1 || layers[0][0] != "a" {
		t.Errorf("layer[0] = %v, want [\"a\"]", layers[0])
	}
	// Layer 1 must be ["b", "c"] (sorted alphabetically).
	sortLayers(layers)
	if len(layers[1]) != 2 || layers[1][0] != "b" || layers[1][1] != "c" {
		t.Errorf("layer[1] = %v, want [\"b\", \"c\"]", layers[1])
	}
	// Layer 2 must be ["d"].
	if len(layers[2]) != 1 || layers[2][0] != "d" {
		t.Errorf("layer[2] = %v, want [\"d\"]", layers[2])
	}
}

// TestTopoOrder_Independent verifies two no-dep processes produce a single
// layer [["a", "b"]].
func TestTopoOrder_Independent(t *testing.T) {
	t.Parallel()

	processes := map[string]*ManagedProcess{
		"a": makeMp("a"),
		"b": makeMp("b"),
	}
	layers, err := topoOrder(processes)
	if err != nil {
		t.Fatalf("topoOrder: unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d: %v", len(layers), layers)
	}
	sort.Strings(layers[0])
	if len(layers[0]) != 2 || layers[0][0] != "a" || layers[0][1] != "b" {
		t.Errorf("layer[0] = %v, want [\"a\", \"b\"]", layers[0])
	}
}

// TestTopoOrder_EmptyMap verifies that an empty process map returns nil, nil.
func TestTopoOrder_EmptyMap(t *testing.T) {
	t.Parallel()

	processes := map[string]*ManagedProcess{}
	layers, err := topoOrder(processes)
	if err != nil {
		t.Fatalf("topoOrder: unexpected error for empty map: %v", err)
	}
	if layers != nil {
		t.Errorf("topoOrder empty map: got %v, want nil", layers)
	}
}

// ---- Register integration tests ----

// TestRegister_RejectsSelfDependency verifies that registering a process with
// DependsOn containing its own name returns an error wrapping ErrDependencyCycle.
func TestRegister_RejectsSelfDependency(t *testing.T) {
	t.Parallel()

	s := New()
	err := s.Register(ProcessDef{Name: "loop", Command: "echo", DependsOn: []string{"loop"}})
	if err == nil {
		t.Fatal("expected error for self-dependency, got nil")
	}
	if !errors.Is(err, ErrDependencyCycle) {
		t.Errorf("got %v, want wrapping ErrDependencyCycle", err)
	}
}

// TestRegister_RejectsMissingDependency verifies that registering a process that
// depends on an unregistered name returns an error wrapping ErrDependencyNotFound.
func TestRegister_RejectsMissingDependency(t *testing.T) {
	t.Parallel()

	s := New()
	err := s.Register(ProcessDef{Name: "orphan", Command: "echo", DependsOn: []string{"ghost"}})
	if err == nil {
		t.Fatal("expected error for missing dependency, got nil")
	}
	if !errors.Is(err, ErrDependencyNotFound) {
		t.Errorf("got %v, want wrapping ErrDependencyNotFound", err)
	}
}

// TestRegister_AcceptsValidDependency verifies that registering "db" then
// "api" depending on "db" both succeed.
func TestRegister_AcceptsValidDependency(t *testing.T) {
	t.Parallel()

	s := New()
	if err := s.Register(ProcessDef{Name: "db", Command: "echo"}); err != nil {
		t.Fatalf("Register db: %v", err)
	}
	if err := s.Register(ProcessDef{Name: "api", Command: "echo", DependsOn: []string{"db"}}); err != nil {
		t.Fatalf("Register api: %v", err)
	}
}

// ---- StartAll integration tests (real processes) ----

// TestStartAll_IndependentProcesses verifies that StartAll starts all processes
// when there are no inter-process dependencies.
func TestStartAll_IndependentProcesses(t *testing.T) {
	t.Parallel()

	s := New()
	if err := s.Register(ProcessDef{Name: "a", Command: "sleep", Args: []string{"30"}}); err != nil {
		t.Fatalf("Register a: %v", err)
	}
	if err := s.Register(ProcessDef{Name: "b", Command: "sleep", Args: []string{"30"}}); err != nil {
		t.Fatalf("Register b: %v", err)
	}
	t.Cleanup(func() {
		killProcess(t, s, "a")
		killProcess(t, s, "b")
	})

	if err := s.StartAll(); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	for _, name := range []string{"a", "b"} {
		st, ok := getState(s, name)
		if !ok {
			t.Fatalf("getState %s: process not found", name)
		}
		if st != StateRunning {
			t.Errorf("process %q: got state %s, want Running", name, st)
		}
	}
}

// TestStartAll_Chain verifies that StartAll starts a chain a->b->c in order,
// with all processes reaching StateRunning.
func TestStartAll_Chain(t *testing.T) {
	t.Parallel()

	s := New()
	defs := []ProcessDef{
		{Name: "a", Command: "sleep", Args: []string{"30"}},
		{Name: "b", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"a"}},
		{Name: "c", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"b"}},
	}
	for _, def := range defs {
		if err := s.Register(def); err != nil {
			t.Fatalf("Register %s: %v", def.Name, err)
		}
	}
	t.Cleanup(func() {
		killProcess(t, s, "c")
		killProcess(t, s, "b")
		killProcess(t, s, "a")
	})

	if err := s.StartAll(); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	for _, name := range []string{"a", "b", "c"} {
		st, ok := getState(s, name)
		if !ok {
			t.Fatalf("getState %s: process not found", name)
		}
		if st != StateRunning {
			t.Errorf("process %q: got state %s, want Running", name, st)
		}
	}
}

// TestStartAll_DiamondDependency verifies that StartAll resolves a diamond
// dependency (a, b->a, c->a, d->b,c) without starting "a" twice.
func TestStartAll_DiamondDependency(t *testing.T) {
	t.Parallel()

	s := New()
	defs := []ProcessDef{
		{Name: "a", Command: "sleep", Args: []string{"30"}},
		{Name: "b", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"a"}},
		{Name: "c", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"a"}},
		{Name: "d", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"b", "c"}},
	}
	for _, def := range defs {
		if err := s.Register(def); err != nil {
			t.Fatalf("Register %s: %v", def.Name, err)
		}
	}
	t.Cleanup(func() {
		killProcess(t, s, "d")
		killProcess(t, s, "c")
		killProcess(t, s, "b")
		killProcess(t, s, "a")
	})

	if err := s.StartAll(); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	for _, name := range []string{"a", "b", "c", "d"} {
		st, ok := getState(s, name)
		if !ok {
			t.Fatalf("getState %s: process not found", name)
		}
		if st != StateRunning {
			t.Errorf("process %q: got state %s, want Running", name, st)
		}
	}
}

// TestStartAll_EmptyScheduler verifies that StartAll on a scheduler with no
// registered processes returns nil (no-op).
func TestStartAll_EmptyScheduler(t *testing.T) {
	t.Parallel()

	s := New()
	if err := s.StartAll(); err != nil {
		t.Fatalf("StartAll on empty scheduler: got error %v, want nil", err)
	}
}

// TestStartAll_SingleProcess verifies that StartAll with a single no-dep
// process starts it and reaches StateRunning.
func TestStartAll_SingleProcess(t *testing.T) {
	t.Parallel()

	s := New()
	if err := s.Register(ProcessDef{Name: "solo", Command: "sleep", Args: []string{"30"}}); err != nil {
		t.Fatalf("Register solo: %v", err)
	}
	t.Cleanup(func() { killProcess(t, s, "solo") })

	if err := s.StartAll(); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	st, ok := getState(s, "solo")
	if !ok {
		t.Fatal("getState: process solo not found")
	}
	if st != StateRunning {
		t.Errorf("solo: got state %s, want Running", st)
	}
}

// ---- Start() dependency check tests ----

// TestStart_RejectsDependencyNotRunning verifies that Start("api") fails with
// ErrDependencyNotReady when its dependency "db" is not running.
func TestStart_RejectsDependencyNotRunning(t *testing.T) {
	t.Parallel()

	s := New()
	if err := s.Register(ProcessDef{Name: "db", Command: "sleep", Args: []string{"30"}}); err != nil {
		t.Fatalf("Register db: %v", err)
	}
	if err := s.Register(ProcessDef{Name: "api", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"db"}}); err != nil {
		t.Fatalf("Register api: %v", err)
	}

	// Do NOT start "db" — call Start("api") which should be rejected.
	err := s.Start("api")
	if err == nil {
		t.Fatal("Start(api): expected ErrDependencyNotReady, got nil")
	}
	if !errors.Is(err, ErrDependencyNotReady) {
		t.Errorf("Start(api): got %v, want wrapping ErrDependencyNotReady", err)
	}

	// "api" must remain in StateIdle since Start() was rejected.
	st, ok := getState(s, "api")
	if !ok {
		t.Fatal("getState: process api not found")
	}
	if st != StateIdle {
		t.Errorf("api state: got %s, want Idle", st)
	}
}

// TestStart_AcceptsDependencyRunning verifies that Start("api") succeeds when
// its dependency "db" is already in StateRunning.
func TestStart_AcceptsDependencyRunning(t *testing.T) {
	t.Parallel()

	s := New()
	if err := s.Register(ProcessDef{Name: "db", Command: "sleep", Args: []string{"30"}}); err != nil {
		t.Fatalf("Register db: %v", err)
	}
	if err := s.Register(ProcessDef{Name: "api", Command: "sleep", Args: []string{"30"}, DependsOn: []string{"db"}}); err != nil {
		t.Fatalf("Register api: %v", err)
	}
	t.Cleanup(func() {
		killProcess(t, s, "api")
		killProcess(t, s, "db")
	})

	// Start "db" and wait for it to be Running.
	if err := s.Start("db"); err != nil {
		t.Fatalf("Start(db): %v", err)
	}
	if err := waitRunning(s, "db", 10*time.Second); err != nil {
		t.Fatalf("waitRunning(db): %v", err)
	}

	// Now Start("api") should succeed.
	if err := s.Start("api"); err != nil {
		t.Fatalf("Start(api): %v", err)
	}

	// Both should be Running.
	for _, name := range []string{"db", "api"} {
		st, ok := getState(s, name)
		if !ok {
			t.Fatalf("getState %s: process not found", name)
		}
		if st != StateRunning {
			t.Errorf("%s: got state %s, want Running", name, st)
		}
	}
}
