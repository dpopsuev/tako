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
	if g.Root.SessionFactory != "Hooks()" {
		t.Errorf("root session_factory = %q, want Hooks()", g.Root.SessionFactory)
	}

	if len(g.Schematics) != 0 {
		t.Fatalf("sub-schematics = %d, want 0 (gnd is a separate service)", len(g.Schematics))
	}

	// Root should have options for source (writer/discoverer/store are optional)
	optNames := make([]string, 0, len(g.Root.Options))
	for _, o := range g.Root.Options {
		optNames = append(optNames, o.OptionFunc)
	}
	if !contains(optNames, "WithSourceReader") {
		t.Errorf("root options %v missing WithSourceReader", optNames)
	}

	// Source binding should be factory-mode (RP)
	for _, o := range g.Root.Options {
		if o.OptionFunc == "WithSourceReader" {
			if o.Wire != "factory" {
				t.Errorf("WithSourceReader wire = %q, want factory", o.Wire)
			}
			if !strings.Contains(o.Provider, "NewSourceReader") {
				t.Errorf("WithSourceReader provider = %q, want *NewSourceReader*", o.Provider)
			}
		}
	}

	// Imports should include rca + rp connector modules
	if len(g.Imports) < 2 {
		t.Errorf("imports = %d, want >= 2", len(g.Imports))
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

func TestTopoSort_MultipleRoots(t *testing.T) {
	m := &Manifest{
		Name: "test",
		Schematics: map[string]SchematicRef{
			"a": {Path: "github.com/dpopsuev/origami-rca"},
			"b": {Path: "github.com/dpopsuev/origami-gnd"},
		},
	}

	_, _, err := topoSort(m)
	if err == nil {
		t.Fatal("expected error for multiple roots")
	}
	if !strings.Contains(err.Error(), "multiple root") {
		t.Errorf("error = %q, want multiple root", err.Error())
	}
}

func TestImportAlias(t *testing.T) {
	tests := []struct {
		mod  string
		want string
	}{
		{"github.com/dpopsuev/origami-rca/connectors/rp", "rp"},
		{"github.com/dpopsuev/origami/connectors/github", "github"},
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
