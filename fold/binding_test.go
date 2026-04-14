package fold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func origamiRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(wd)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Skipf("origami root not found at %s", root)
	}
	// Skip when sibling origami-rca repo isn't available (e.g. CI).
	if _, err := os.Stat(filepath.Join(root, "origami-rca")); err != nil {
		t.Skipf("origami-rca sibling repo not found — skipping (CI)")
	}
	return root
}

func TestResolve_AsteriskLike(t *testing.T) {
	root := origamiRoot(t)

	m := &Manifest{
		Name:    "asterisk",
		Version: "1.0",
		Schematics: map[string]SchematicRef{
			"rca": {
				Path: "github.com/dpopsuev/origami-rca",
				Bindings: map[string]string{
					"source": "reportportal",
				},
			},
		},
		Connectors: map[string]ConnectorRef{
			"reportportal": {Path: "github.com/dpopsuev/origami-rca/connectors/rp"},
		},
	}

	g, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatal(err)
	}

	if g.Root.Name != "rca" {
		t.Errorf("root = %q, want rca", g.Root.Name)
	}
	if g.Root.SessionFactory != "Factory()" {
		t.Errorf("root session_factory = %q, want Factory()", g.Root.SessionFactory)
	}

	if len(g.Schematics) != 0 {
		t.Fatalf("sub-schematics = %d, want 0 (gnd is a separate service)", len(g.Schematics))
	}

	// RCA sockets have no option: fields (removed — functions don't exist yet).
	// No resolved options expected from socket binding.
	if len(g.Root.Options) != 0 {
		t.Errorf("root options = %v, want none (sockets have no option: field)", g.Root.Options)
	}
}

func TestResolve_MissingBinding(t *testing.T) {
	// rh-rca's sockets are all optional: true, so empty bindings
	// are accepted. Verify Resolve succeeds.
	root := origamiRoot(t)

	m := &Manifest{
		Name: "test",
		Schematics: map[string]SchematicRef{
			"rca": {
				Path:     "github.com/dpopsuev/origami-rca",
				Bindings: map[string]string{},
			},
		},
		Connectors: map[string]ConnectorRef{},
	}

	_, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatalf("Resolve should succeed with all-optional sockets: %v", err)
	}
}

func TestResolve_CycleDetection(t *testing.T) {
	m := &Manifest{
		Name: "test",
		Schematics: map[string]SchematicRef{
			"a": {Path: "github.com/dpopsuev/origami-rca", Bindings: map[string]string{"source": "b"}},
			"b": {Path: "github.com/dpopsuev/origami-gnd", Bindings: map[string]string{"git": "a"}},
		},
		Connectors: map[string]ConnectorRef{},
	}

	err := detectCycles(m)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want mention of cycle", err.Error())
	}
}

func TestTopoSort_SingleRoot(t *testing.T) {
	m := &Manifest{
		Name: "test",
		Schematics: map[string]SchematicRef{
			"rca": {Path: "github.com/dpopsuev/origami-rca", Bindings: map[string]string{"gnd": "gnd"}},
			"gnd": {Path: "github.com/dpopsuev/origami-gnd"},
		},
	}

	root, order, err := topoSort(m)
	if err != nil {
		t.Fatal(err)
	}
	if root != "rca" {
		t.Errorf("root = %q, want rca", root)
	}
	if len(order) != 1 || order[0] != "gnd" {
		t.Errorf("order = %v, want [gnd]", order)
	}
}

func TestTopoSort_MultipleRoots_NoEntry(t *testing.T) {
	m := &Manifest{
		Name: "test",
		Uses: map[string]UsesRef{
			"a": {Kind: "Schematic", Module: "github.com/dpopsuev/origami-rca"},
			"b": {Kind: "Schematic", Module: "github.com/dpopsuev/origami-gnd"},
		},
		Schematics: map[string]SchematicRef{
			"a": {Path: "github.com/dpopsuev/origami-rca"},
			"b": {Path: "github.com/dpopsuev/origami-gnd"},
		},
	}

	_, _, err := topoSort(m)
	if err == nil {
		t.Fatal("expected error for multiple roots without entry: true")
	}
	if !strings.Contains(err.Error(), "entry: true") {
		t.Errorf("error = %q, want mention of entry: true", err.Error())
	}
}

func TestTopoSort_MultipleRoots_WithEntry(t *testing.T) {
	m := &Manifest{
		Name: "test",
		Uses: map[string]UsesRef{
			"rca": {Kind: "Schematic", Module: "github.com/dpopsuev/origami-rca", Entry: true},
			"gnd": {Kind: "Schematic", Module: "github.com/dpopsuev/origami-gnd"},
		},
		Schematics: map[string]SchematicRef{
			"rca": {Path: "github.com/dpopsuev/origami-rca"},
			"gnd": {Path: "github.com/dpopsuev/origami-gnd"},
		},
	}

	root, order, err := topoSort(m)
	if err != nil {
		t.Fatal(err)
	}
	if root != "rca" {
		t.Errorf("root = %q, want rca", root)
	}
	if len(order) != 1 || order[0] != "gnd" {
		t.Errorf("order = %v, want [gnd]", order)
	}
}

// Trap test (poka-yoke): both schematics have session_factory in their
// component.yaml, but neither has entry: true in the board. topoSort MUST
// reject. This makes it structurally impossible to reintroduce a heuristic
// that reads component.yaml internals to pick the root — both look identical.
func TestTopoSort_MultipleRoots_BothWithSessionFactory_RequiresEntryFlag(t *testing.T) {
	m := &Manifest{
		Name: "test",
		Uses: map[string]UsesRef{
			"rca": {Kind: "Schematic", Module: "github.com/dpopsuev/origami-rca"},
			"gnd": {Kind: "Schematic", Module: "github.com/dpopsuev/origami-gnd"},
		},
		Schematics: map[string]SchematicRef{
			"rca": {Path: "github.com/dpopsuev/origami-rca"},
			"gnd": {Path: "github.com/dpopsuev/origami-gnd"},
		},
	}
	// Both have session_factory — topoSort cannot use it to disambiguate.
	// The ONLY valid resolution is entry: true on the board.
	_, _, err := topoSort(m)
	if err == nil {
		t.Fatal("topoSort must reject multiple roots — board must declare entry: true")
	}
}

func TestImportAlias(t *testing.T) {
	tests := []struct {
		mod  string
		want string
	}{
		{"github.com/dpopsuev/origami-rca/connectors/rp", "rp"},
		{"github.com/dpopsuev/origami/instruments/github", "github"},
		{"github.com/dpopsuev/origami-gnd", "origamignd"},
		{"github.com/dpopsuev/origami-rca/mcpconfig", "mcpconfig"},
	}
	for _, tt := range tests {
		got := importAlias(tt.mod)
		if got != tt.want {
			t.Errorf("importAlias(%q) = %q, want %q", tt.mod, got, tt.want)
		}
	}
}

func TestParseManifest_WithBindings(t *testing.T) {
	m, err := ParseManifest(loadFixtureManifest(t, "with-bindings"))
	if err != nil {
		t.Fatal(err)
	}
	if !m.HasBindings() {
		t.Error("HasBindings() = false, want true")
	}
	if len(m.Uses) != 3 {
		t.Errorf("uses count = %d, want 3", len(m.Uses))
	}
	if m.Uses["rca"].Module != "github.com/dpopsuev/origami-rca" {
		t.Errorf("uses[rca].module = %q", m.Uses["rca"].Module)
	}
	if m.Bind["rca"]["source"] != "reportportal" {
		t.Errorf("bind[rca][source] = %q", m.Bind["rca"]["source"])
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
