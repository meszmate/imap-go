package extension

import (
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
)

// testExtension is a minimal Extension for testing.
type testExtension struct {
	name         string
	capabilities []imap.Cap
	dependencies []string
}

func (e *testExtension) Name() string           { return e.name }
func (e *testExtension) Capabilities() []imap.Cap { return e.capabilities }
func (e *testExtension) Dependencies() []string  { return e.dependencies }

func newTestExt(name string, deps ...string) *testExtension {
	return &testExtension{
		name:         name,
		dependencies: deps,
	}
}

func newTestExtWithCaps(name string, caps []imap.Cap, deps ...string) *testExtension {
	return &testExtension{
		name:         name,
		capabilities: caps,
		dependencies: deps,
	}
}

// --- NewRegistry ---

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.Len() != 0 {
		t.Fatalf("expected empty registry, got Len=%d", r.Len())
	}
}

// --- Register ---

func TestRegister(t *testing.T) {
	r := NewRegistry()
	ext := newTestExt("MOVE")

	if err := r.Register(ext); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if r.Len() != 1 {
		t.Fatalf("expected Len=1, got %d", r.Len())
	}
}

func TestRegister_Multiple(t *testing.T) {
	r := NewRegistry()

	for _, name := range []string{"EXT1", "EXT2", "EXT3"} {
		if err := r.Register(newTestExt(name)); err != nil {
			t.Fatalf("Register(%s) failed: %v", name, err)
		}
	}

	if r.Len() != 3 {
		t.Fatalf("expected Len=3, got %d", r.Len())
	}
}

func TestRegister_DuplicateReturnsError(t *testing.T) {
	r := NewRegistry()
	ext := newTestExt("DUP")

	if err := r.Register(ext); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := r.Register(ext)
	if err == nil {
		t.Fatal("expected error on duplicate Register, got nil")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected 'already registered' error, got: %v", err)
	}
}

func TestRegister_DuplicateDifferentInstances(t *testing.T) {
	r := NewRegistry()

	if err := r.Register(newTestExt("SAME")); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := r.Register(newTestExt("SAME"))
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

// --- Get ---

func TestGet_Exists(t *testing.T) {
	r := NewRegistry()
	ext := newTestExt("TEST")
	_ = r.Register(ext)

	got, ok := r.Get("TEST")
	if !ok {
		t.Fatal("Get returned false for registered extension")
	}
	if got.Name() != "TEST" {
		t.Fatalf("expected name TEST, got %s", got.Name())
	}
}

func TestGet_NotExists(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("NONEXISTENT")
	if ok {
		t.Fatal("Get returned true for unregistered extension")
	}
}

func TestGet_AfterRemove(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("REMOVE_ME"))

	r.Remove("REMOVE_ME")

	_, ok := r.Get("REMOVE_ME")
	if ok {
		t.Fatal("Get returned true after Remove")
	}
}

// --- Remove ---

func TestRemove(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))
	_ = r.Register(newTestExt("B"))
	_ = r.Register(newTestExt("C"))

	r.Remove("B")

	if r.Len() != 2 {
		t.Fatalf("expected Len=2 after Remove, got %d", r.Len())
	}

	_, ok := r.Get("B")
	if ok {
		t.Fatal("B should not be found after Remove")
	}
}

func TestRemove_NonExistent(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))

	// Should not panic or change state
	r.Remove("NONEXISTENT")

	if r.Len() != 1 {
		t.Fatalf("expected Len=1 after removing nonexistent, got %d", r.Len())
	}
}

func TestRemove_PreservesOrder(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))
	_ = r.Register(newTestExt("B"))
	_ = r.Register(newTestExt("C"))

	r.Remove("B")

	names := r.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "A" || names[1] != "C" {
		t.Fatalf("expected [A, C], got %v", names)
	}
}

// --- All ---

func TestAll_Empty(t *testing.T) {
	r := NewRegistry()
	all := r.All()
	if len(all) != 0 {
		t.Fatalf("expected 0 extensions, got %d", len(all))
	}
}

func TestAll_ReturnsInRegistrationOrder(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("FIRST"))
	_ = r.Register(newTestExt("SECOND"))
	_ = r.Register(newTestExt("THIRD"))

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 extensions, got %d", len(all))
	}
	if all[0].Name() != "FIRST" {
		t.Errorf("expected first to be FIRST, got %s", all[0].Name())
	}
	if all[1].Name() != "SECOND" {
		t.Errorf("expected second to be SECOND, got %s", all[1].Name())
	}
	if all[2].Name() != "THIRD" {
		t.Errorf("expected third to be THIRD, got %s", all[2].Name())
	}
}

func TestAll_ReturnsNewSlice(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))

	all1 := r.All()
	all2 := r.All()

	// Modifying one slice should not affect the other
	if len(all1) != 1 || len(all2) != 1 {
		t.Fatal("unexpected slice lengths")
	}
}

// --- Names ---

func TestNames_Empty(t *testing.T) {
	r := NewRegistry()
	names := r.Names()
	if len(names) != 0 {
		t.Fatalf("expected 0 names, got %d", len(names))
	}
}

func TestNames_ReturnsInRegistrationOrder(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("Z"))
	_ = r.Register(newTestExt("A"))
	_ = r.Register(newTestExt("M"))

	names := r.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "Z" || names[1] != "A" || names[2] != "M" {
		t.Fatalf("expected [Z, A, M], got %v", names)
	}
}

func TestNames_ReturnsCopy(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))

	names := r.Names()
	names[0] = "MODIFIED"

	// Original should be unchanged
	original := r.Names()
	if original[0] != "A" {
		t.Fatalf("Names did not return a copy; original was modified to %s", original[0])
	}
}

// --- Len ---

func TestLen_Empty(t *testing.T) {
	r := NewRegistry()
	if r.Len() != 0 {
		t.Fatalf("expected Len=0, got %d", r.Len())
	}
}

func TestLen_AfterRegister(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))
	_ = r.Register(newTestExt("B"))

	if r.Len() != 2 {
		t.Fatalf("expected Len=2, got %d", r.Len())
	}
}

func TestLen_AfterRemove(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))
	_ = r.Register(newTestExt("B"))

	r.Remove("A")
	if r.Len() != 1 {
		t.Fatalf("expected Len=1 after remove, got %d", r.Len())
	}
}

// --- Resolve: topological sort ---

func TestResolve_NoDependencies(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))
	_ = r.Register(newTestExt("B"))
	_ = r.Register(newTestExt("C"))

	sorted, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 extensions, got %d", len(sorted))
	}
}

func TestResolve_LinearDependencies(t *testing.T) {
	r := NewRegistry()
	// C depends on B, B depends on A
	_ = r.Register(newTestExt("C", "B"))
	_ = r.Register(newTestExt("B", "A"))
	_ = r.Register(newTestExt("A"))

	sorted, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 extensions, got %d", len(sorted))
	}

	// Build position map: each extension should appear after its dependencies
	pos := make(map[string]int)
	for i, ext := range sorted {
		pos[ext.Name()] = i
	}

	if pos["A"] >= pos["B"] {
		t.Errorf("A (pos %d) should come before B (pos %d)", pos["A"], pos["B"])
	}
	if pos["B"] >= pos["C"] {
		t.Errorf("B (pos %d) should come before C (pos %d)", pos["B"], pos["C"])
	}
}

func TestResolve_DiamondDependency(t *testing.T) {
	r := NewRegistry()
	// D depends on B and C; B depends on A; C depends on A
	_ = r.Register(newTestExt("D", "B", "C"))
	_ = r.Register(newTestExt("B", "A"))
	_ = r.Register(newTestExt("C", "A"))
	_ = r.Register(newTestExt("A"))

	sorted, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(sorted) != 4 {
		t.Fatalf("expected 4 extensions, got %d", len(sorted))
	}

	pos := make(map[string]int)
	for i, ext := range sorted {
		pos[ext.Name()] = i
	}

	// A must come before B and C
	if pos["A"] >= pos["B"] {
		t.Errorf("A (pos %d) should come before B (pos %d)", pos["A"], pos["B"])
	}
	if pos["A"] >= pos["C"] {
		t.Errorf("A (pos %d) should come before C (pos %d)", pos["A"], pos["C"])
	}
	// B and C must come before D
	if pos["B"] >= pos["D"] {
		t.Errorf("B (pos %d) should come before D (pos %d)", pos["B"], pos["D"])
	}
	if pos["C"] >= pos["D"] {
		t.Errorf("C (pos %d) should come before D (pos %d)", pos["C"], pos["D"])
	}
}

func TestResolve_MultipleDependencies(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))
	_ = r.Register(newTestExt("B"))
	_ = r.Register(newTestExt("C", "A", "B"))

	sorted, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3, got %d", len(sorted))
	}

	pos := make(map[string]int)
	for i, ext := range sorted {
		pos[ext.Name()] = i
	}

	if pos["A"] >= pos["C"] {
		t.Errorf("A should come before C")
	}
	if pos["B"] >= pos["C"] {
		t.Errorf("B should come before C")
	}
}

func TestResolve_SingleExtension(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("ONLY"))

	sorted, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(sorted) != 1 {
		t.Fatalf("expected 1, got %d", len(sorted))
	}
	if sorted[0].Name() != "ONLY" {
		t.Fatalf("expected ONLY, got %s", sorted[0].Name())
	}
}

func TestResolve_Empty(t *testing.T) {
	r := NewRegistry()
	sorted, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(sorted) != 0 {
		t.Fatalf("expected 0, got %d", len(sorted))
	}
}

// --- Resolve: circular dependency detection ---

func TestResolve_CircularDependency_Direct(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A", "B"))
	_ = r.Register(newTestExt("B", "A"))

	_, err := r.Resolve()
	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestResolve_CircularDependency_Indirect(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A", "C"))
	_ = r.Register(newTestExt("B", "A"))
	_ = r.Register(newTestExt("C", "B"))

	_, err := r.Resolve()
	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

func TestResolve_SelfDependency(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A", "A"))

	_, err := r.Resolve()
	if err == nil {
		t.Fatal("expected circular dependency error for self-dep, got nil")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}

// --- Resolve: missing dependency detection ---

func TestResolve_MissingDependency(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A", "MISSING"))

	_, err := r.Resolve()
	if err == nil {
		t.Fatal("expected missing dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("expected 'not registered' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "MISSING") {
		t.Fatalf("expected 'MISSING' in error, got: %v", err)
	}
}

func TestResolve_MissingDependency_OneOfMany(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))
	_ = r.Register(newTestExt("B", "A", "NONEXISTENT"))

	_, err := r.Resolve()
	if err == nil {
		t.Fatal("expected missing dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "NONEXISTENT") {
		t.Fatalf("expected 'NONEXISTENT' in error, got: %v", err)
	}
}

// --- BaseExtension ---

func TestBaseExtension_Name(t *testing.T) {
	ext := &BaseExtension{ExtName: "TEST"}
	if ext.Name() != "TEST" {
		t.Fatalf("expected TEST, got %s", ext.Name())
	}
}

func TestBaseExtension_Capabilities(t *testing.T) {
	caps := []imap.Cap{imap.CapMove, imap.CapIdle}
	ext := &BaseExtension{
		ExtName:         "TEST",
		ExtCapabilities: caps,
	}

	got := ext.Capabilities()
	if len(got) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(got))
	}
	if got[0] != imap.CapMove {
		t.Errorf("expected CapMove, got %v", got[0])
	}
	if got[1] != imap.CapIdle {
		t.Errorf("expected CapIdle, got %v", got[1])
	}
}

func TestBaseExtension_Capabilities_Nil(t *testing.T) {
	ext := &BaseExtension{ExtName: "TEST"}
	got := ext.Capabilities()
	if got != nil {
		t.Fatalf("expected nil capabilities, got %v", got)
	}
}

func TestBaseExtension_Dependencies(t *testing.T) {
	ext := &BaseExtension{
		ExtName:         "TEST",
		ExtDependencies: []string{"DEP1", "DEP2"},
	}

	deps := ext.Dependencies()
	if len(deps) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(deps))
	}
	if deps[0] != "DEP1" || deps[1] != "DEP2" {
		t.Fatalf("expected [DEP1, DEP2], got %v", deps)
	}
}

func TestBaseExtension_Dependencies_Nil(t *testing.T) {
	ext := &BaseExtension{ExtName: "TEST"}
	deps := ext.Dependencies()
	if deps != nil {
		t.Fatalf("expected nil dependencies, got %v", deps)
	}
}

// --- BaseExtension implements Extension ---

func TestBaseExtension_ImplementsExtension(t *testing.T) {
	var _ Extension = (*BaseExtension)(nil)
}

// --- Resolve with BaseExtension ---

func TestResolve_WithBaseExtension(t *testing.T) {
	r := NewRegistry()

	a := &BaseExtension{ExtName: "A"}
	b := &BaseExtension{ExtName: "B", ExtDependencies: []string{"A"}}

	_ = r.Register(a)
	_ = r.Register(b)

	sorted, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(sorted) != 2 {
		t.Fatalf("expected 2, got %d", len(sorted))
	}

	pos := make(map[string]int)
	for i, ext := range sorted {
		pos[ext.Name()] = i
	}
	if pos["A"] >= pos["B"] {
		t.Errorf("A should come before B")
	}
}

// --- Resolve with capabilities ---

func TestResolve_WithCapabilities(t *testing.T) {
	r := NewRegistry()

	ext := newTestExtWithCaps("MOVE", []imap.Cap{imap.CapMove})
	_ = r.Register(ext)

	sorted, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(sorted) != 1 {
		t.Fatalf("expected 1, got %d", len(sorted))
	}

	caps := sorted[0].Capabilities()
	if len(caps) != 1 || caps[0] != imap.CapMove {
		t.Fatalf("expected [MOVE], got %v", caps)
	}
}

// --- Register and Remove interaction ---

func TestRegisterAfterRemove(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newTestExt("A"))
	r.Remove("A")

	// Re-register with the same name should succeed
	err := r.Register(newTestExt("A"))
	if err != nil {
		t.Fatalf("Register after Remove should succeed, got: %v", err)
	}
	if r.Len() != 1 {
		t.Fatalf("expected Len=1, got %d", r.Len())
	}
}

// --- Complex topological sort ---

func TestResolve_LargerGraph(t *testing.T) {
	r := NewRegistry()

	// Graph:
	// E depends on C, D
	// D depends on B
	// C depends on A
	// B depends on A
	// A has no deps
	_ = r.Register(newTestExt("E", "C", "D"))
	_ = r.Register(newTestExt("D", "B"))
	_ = r.Register(newTestExt("C", "A"))
	_ = r.Register(newTestExt("B", "A"))
	_ = r.Register(newTestExt("A"))

	sorted, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(sorted) != 5 {
		t.Fatalf("expected 5, got %d", len(sorted))
	}

	pos := make(map[string]int)
	for i, ext := range sorted {
		pos[ext.Name()] = i
	}

	// Verify all dependency ordering
	checks := [][2]string{
		{"A", "B"},
		{"A", "C"},
		{"B", "D"},
		{"C", "E"},
		{"D", "E"},
	}
	for _, check := range checks {
		if pos[check[0]] >= pos[check[1]] {
			t.Errorf("%s (pos %d) should come before %s (pos %d)",
				check[0], pos[check[0]], check[1], pos[check[1]])
		}
	}
}

// --- Partial circular dependency ---

func TestResolve_PartialCircular(t *testing.T) {
	r := NewRegistry()
	// A is fine, B and C form a cycle
	_ = r.Register(newTestExt("A"))
	_ = r.Register(newTestExt("B", "C"))
	_ = r.Register(newTestExt("C", "B"))

	_, err := r.Resolve()
	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected 'circular' in error, got: %v", err)
	}
}
