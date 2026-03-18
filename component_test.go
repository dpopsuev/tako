package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadComponentManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "component.yaml")

	manifest := `
component: test-component
namespace: test
version: "1.0.0"
description: A test component
provides:
  transformers: [my-transform]
  extractors: [my-extract]
  hooks: [my-hook]
requires:
  origami: ">=0.1.0"
`
	if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadComponentManifest(path)
	if err != nil {
		t.Fatal(err)
	}

	if m.Namespace != "test" {
		t.Errorf("namespace = %q, want %q", m.Namespace, "test")
	}
	if m.Component != "test-component" {
		t.Errorf("component = %q, want %q", m.Component, "test-component")
	}
	if m.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", m.Version, "1.0.0")
	}
	if len(m.Provides.Transformers) != 1 || m.Provides.Transformers[0] != "my-transform" {
		t.Errorf("provides.transformers = %v, want [my-transform]", m.Provides.Transformers)
	}
	if len(m.Provides.Extractors) != 1 || m.Provides.Extractors[0] != "my-extract" {
		t.Errorf("provides.extractors = %v, want [my-extract]", m.Provides.Extractors)
	}
	if len(m.Provides.Hooks) != 1 || m.Provides.Hooks[0] != "my-hook" {
		t.Errorf("provides.hooks = %v, want [my-hook]", m.Provides.Hooks)
	}
}

func TestLoadComponentManifest_SocketsAndSatisfies(t *testing.T) {
	dir := t.TempDir()
	schematicPath := filepath.Join(dir, "schematic.yaml")
	connectorPath := filepath.Join(dir, "connector.yaml")

	schematicYAML := `
component: my-schematic
namespace: rca
version: "1.0.0"
provides:
  transformers: [ctx-builder]
requires:
  origami: ">=0.3.0"
  sockets:
    - name: store
      type: store.Store
      description: Persistent storage backend
    - name: source
      type: SourceReader
      description: External tracker integration
`
	connectorYAML := `
component: my-connector
namespace: sqlite
version: "1.0.0"
provides:
  transformers: []
requires:
  origami: ">=0.3.0"
satisfies:
  - socket: store
    factory: NewStore
`
	if err := os.WriteFile(schematicPath, []byte(schematicYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(connectorPath, []byte(connectorYAML), 0644); err != nil {
		t.Fatal(err)
	}

	sm, err := LoadComponentManifest(schematicPath)
	if err != nil {
		t.Fatalf("load schematic manifest: %v", err)
	}
	if len(sm.Requires.Sockets) != 2 {
		t.Fatalf("sockets = %d, want 2", len(sm.Requires.Sockets))
	}
	if sm.Requires.Sockets[0].Name != "store" {
		t.Errorf("socket[0].name = %q, want %q", sm.Requires.Sockets[0].Name, "store")
	}
	if sm.Requires.Sockets[0].Type != "store.Store" {
		t.Errorf("socket[0].type = %q, want %q", sm.Requires.Sockets[0].Type, "store.Store")
	}
	if sm.Requires.Sockets[1].Name != "source" {
		t.Errorf("socket[1].name = %q, want %q", sm.Requires.Sockets[1].Name, "source")
	}

	cm, err := LoadComponentManifest(connectorPath)
	if err != nil {
		t.Fatalf("load connector manifest: %v", err)
	}
	if len(cm.Satisfies) != 1 {
		t.Fatalf("satisfies = %d, want 1", len(cm.Satisfies))
	}
	if cm.Satisfies[0].Socket != "store" {
		t.Errorf("satisfies[0].socket = %q, want %q", cm.Satisfies[0].Socket, "store")
	}
	if cm.Satisfies[0].Factory != "NewStore" {
		t.Errorf("satisfies[0].factory = %q, want %q", cm.Satisfies[0].Factory, "NewStore")
	}
}

func TestLoadComponentManifest_MissingNamespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "component.yaml")
	if err := os.WriteFile(path, []byte("component: bad\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for missing namespace")
	}
}

func TestMergeComponents_NoCollision(t *testing.T) {
	stubT := TransformerFunc("base-t", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "base", nil
	})
	base := GraphRegistries{
		Transformers: TransformerRegistry{"base-t": stubT},
		Extractors:   ExtractorRegistry{},
		Hooks:        HookRegistry{},
	}

	compT := TransformerFunc("ext-t", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "comp", nil
	})
	comp := &Component{
		Namespace:    "vendor",
		Transformers: TransformerRegistry{"ext-t": compT},
	}

	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatal(err)
	}

	// FQCN key present
	if _, err := merged.Transformers.Get("vendor.ext-t"); err != nil {
		t.Errorf("FQCN lookup failed: %v", err)
	}
	// Short name present (no collision with base)
	if _, err := merged.Transformers.Get("ext-t"); err != nil {
		t.Errorf("short name lookup failed: %v", err)
	}
	// Base still present
	if _, err := merged.Transformers.Get("base-t"); err != nil {
		t.Errorf("base lookup failed: %v", err)
	}
}

func TestMergeComponents_ShortNameCollision(t *testing.T) {
	stubT := TransformerFunc("llm", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "base-llm", nil
	})
	base := GraphRegistries{
		Transformers: TransformerRegistry{"llm": stubT},
		Extractors:   ExtractorRegistry{},
		Hooks:        HookRegistry{},
	}

	compT := TransformerFunc("llm", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "comp-llm", nil
	})
	comp := &Component{
		Namespace:    "custom",
		Transformers: TransformerRegistry{"llm": compT},
	}

	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatal("MergeComponents should succeed; short name collision is not fatal")
	}

	// FQCN resolves to component's version
	if _, err := merged.Transformers.Get("custom.llm"); err != nil {
		t.Errorf("FQCN lookup failed: %v", err)
	}
	// Short name still resolves to the base (first-registered wins)
	result, err := merged.Transformers.Get("llm")
	if err != nil {
		t.Fatal(err)
	}
	out, _ := result.Transform(context.Background(), &TransformerContext{})
	if out != "base-llm" {
		t.Errorf("short name should resolve to base; got %v", out)
	}
}

func TestMergeComponents_FQCNCollision(t *testing.T) {
	base := GraphRegistries{
		Transformers: TransformerRegistry{},
		Extractors:   ExtractorRegistry{},
		Hooks:        HookRegistry{},
	}

	t1 := TransformerFunc("llm", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "a1", nil
	})
	t2 := TransformerFunc("llm", func(_ context.Context, _ *TransformerContext) (any, error) {
		return "a2", nil
	})
	a1 := &Component{Namespace: "vendor", Transformers: TransformerRegistry{"llm": t1}}
	a2 := &Component{Namespace: "vendor", Transformers: TransformerRegistry{"llm": t2}}

	_, err := MergeComponents(base, a1, a2)
	if err == nil {
		t.Fatal("expected FQCN collision error")
	}
	if got := err.Error(); got != `transformer "vendor.llm" collision (component vendor)` {
		t.Errorf("unexpected error: %v", got)
	}
}

func TestMergeComponents_DoesNotMutateBase(t *testing.T) {
	base := GraphRegistries{
		Transformers: TransformerRegistry{
			"base-t": TransformerFunc("base-t", func(_ context.Context, _ *TransformerContext) (any, error) { return nil, nil }),
		},
		Extractors: ExtractorRegistry{},
		Hooks:      HookRegistry{},
	}

	comp := &Component{
		Namespace: "x",
		Transformers: TransformerRegistry{
			"new-t": TransformerFunc("new-t", func(_ context.Context, _ *TransformerContext) (any, error) { return nil, nil }),
		},
	}

	_, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatal(err)
	}

	// Base must be unchanged
	if len(base.Transformers) != 1 {
		t.Errorf("base.Transformers was mutated: %d entries, want 1", len(base.Transformers))
	}
}

func TestMergeComponents_Hooks(t *testing.T) {
	base := GraphRegistries{
		Transformers: TransformerRegistry{},
		Extractors:   ExtractorRegistry{},
		Hooks:        HookRegistry{},
	}

	hook := NewHookFunc("store", func(_ context.Context, _ string, _ Artifact) error { return nil })
	comp := &Component{
		Namespace: "rca",
		Hooks:     HookRegistry{"store": hook},
	}

	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := merged.Hooks.Get("rca.store"); err != nil {
		t.Errorf("FQCN hook lookup failed: %v", err)
	}
	if _, err := merged.Hooks.Get("store"); err != nil {
		t.Errorf("short name hook lookup failed: %v", err)
	}
}

func TestMergeComponents_Extractors(t *testing.T) {
	base := GraphRegistries{
		Transformers: TransformerRegistry{},
		Extractors:   ExtractorRegistry{},
		Hooks:        HookRegistry{},
	}

	ext := &componentStubExtractor{name: "govulncheck"}
	comp := &Component{
		Namespace:  "achilles",
		Extractors: ExtractorRegistry{"govulncheck": ext},
	}

	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := merged.Extractors.Get("achilles.govulncheck"); err != nil {
		t.Errorf("FQCN extractor lookup failed: %v", err)
	}
	if _, err := merged.Extractors.Get("govulncheck"); err != nil {
		t.Errorf("short name extractor lookup failed: %v", err)
	}
}

type componentStubExtractor struct {
	name string
}

func (e *componentStubExtractor) Name() string                                   { return e.name }
func (e *componentStubExtractor) Extract(_ context.Context, _ any) (any, error) { return "extracted", nil }

func TestTransformerRegistry_FQCNResolution(t *testing.T) {
	reg := TransformerRegistry{
		"core.llm": TransformerFunc("llm", func(_ context.Context, _ *TransformerContext) (any, error) { return "llm", nil }),
	}

	// Direct FQCN lookup
	if _, err := reg.Get("core.llm"); err != nil {
		t.Errorf("FQCN lookup failed: %v", err)
	}

	// Unqualified name resolved via suffix scan
	if _, err := reg.Get("llm"); err != nil {
		t.Errorf("suffix resolution failed: %v", err)
	}

	// Unknown name
	if _, err := reg.Get("missing"); err == nil {
		t.Error("expected error for unknown transformer")
	}
}

func TestExtractorRegistry_FQCNResolution(t *testing.T) {
	reg := ExtractorRegistry{
		"achilles.govulncheck": &componentStubExtractor{name: "govulncheck"},
	}

	if _, err := reg.Get("achilles.govulncheck"); err != nil {
		t.Errorf("FQCN lookup failed: %v", err)
	}
	if _, err := reg.Get("govulncheck"); err != nil {
		t.Errorf("suffix resolution failed: %v", err)
	}
}

func TestHookRegistry_FQCNResolution(t *testing.T) {
	reg := HookRegistry{
		"rca.store": NewHookFunc("store", func(_ context.Context, _ string, _ Artifact) error { return nil }),
	}

	if _, err := reg.Get("rca.store"); err != nil {
		t.Errorf("FQCN lookup failed: %v", err)
	}
	if _, err := reg.Get("store"); err != nil {
		t.Errorf("suffix resolution failed: %v", err)
	}
}

func TestBuildGraph_ImportsWiring(t *testing.T) {
	def := &CircuitDef{
		Circuit: "test",
		Imports:  []string{"vendor"},
		Nodes: []NodeDef{
			{Name: "start", Transformer: "my-t"},
		},
		Edges: []EdgeDef{
			{ID: "e1", From: "start", To: "done"},
		},
		Start: "start",
		Done:  "done",
	}

	vendorT := TransformerFunc("my-t", func(_ context.Context, _ *TransformerContext) (any, error) { return "ok", nil })
	loader := func(name string) (*Component, error) {
		if name == "vendor" {
			return &Component{
				Namespace:    "vendor",
				Transformers: TransformerRegistry{"my-t": vendorT},
			}, nil
		}
		return nil, fmt.Errorf("unknown component: %s", name)
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{},
		Components:   loader,
	}

	g, err := def.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph with imports: %v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
}

func TestBuildGraph_ImportFailure(t *testing.T) {
	def := &CircuitDef{
		Circuit: "test",
		Imports:  []string{"missing-adapter"},
		Nodes:    []NodeDef{{Name: "start"}},
		Edges:    []EdgeDef{{ID: "e1", From: "start", To: "done"}},
		Start:    "start",
		Done:     "done",
	}

	loader := func(name string) (*Component, error) {
		return nil, fmt.Errorf("component %q not found", name)
	}

	reg := GraphRegistries{
		Nodes:      NodeRegistry{"": func(_ NodeDef) Node { return nil }},
		Components: loader,
	}

	_, err := def.BuildGraph(reg)
	if err == nil {
		t.Fatal("expected error for missing component import")
	}
	if !contains(err.Error(), "missing-adapter") {
		t.Errorf("error should reference the import name: %v", err)
	}
}

func TestMergeComponents_PreservesCircuits(t *testing.T) {
	child := &CircuitDef{Circuit: "child", Start: "s", Done: "d"}
	base := GraphRegistries{
		Circuits: map[string]*CircuitDef{"child": child},
	}

	comp := &Component{
		Namespace: "test",
		Transformers: map[string]Transformer{
			"echo": &passthroughTransformer{},
		},
	}

	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatal(err)
	}

	if merged.Circuits == nil {
		t.Fatal("MergeComponents lost Circuits")
	}
	if _, ok := merged.Circuits["child"]; !ok {
		t.Error("MergeComponents lost circuit 'child'")
	}
}

func TestMergeComponents_PreservesMediatorEndpoint(t *testing.T) {
	base := GraphRegistries{
		MediatorEndpoint: "http://mediator:9000/mcp",
	}

	comp := &Component{
		Namespace: "test",
		Transformers: map[string]Transformer{
			"echo": &passthroughTransformer{},
		},
	}

	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatal(err)
	}

	if merged.MediatorEndpoint != "http://mediator:9000/mcp" {
		t.Errorf("MediatorEndpoint = %q, want http://mediator:9000/mcp", merged.MediatorEndpoint)
	}
}

func TestMergeComponents_PreservesBothCircuitsAndMediator(t *testing.T) {
	child := &CircuitDef{Circuit: "dsr", Start: "s", Done: "d"}
	base := GraphRegistries{
		Circuits:         map[string]*CircuitDef{"dsr": child},
		MediatorEndpoint: "http://mediator:9000/mcp",
		Transformers:     TransformerRegistry{"base": &passthroughTransformer{}},
	}

	comp := &Component{
		Namespace: "rca",
		Transformers: map[string]Transformer{
			"analyze": &passthroughTransformer{},
		},
	}

	merged, err := MergeComponents(base, comp)
	if err != nil {
		t.Fatal(err)
	}

	if merged.Circuits == nil || merged.Circuits["dsr"] == nil {
		t.Error("MergeComponents lost circuit 'dsr'")
	}
	if merged.MediatorEndpoint != "http://mediator:9000/mcp" {
		t.Errorf("MediatorEndpoint = %q, want http://mediator:9000/mcp", merged.MediatorEndpoint)
	}
	if _, ok := merged.Transformers["rca.analyze"]; !ok {
		t.Error("MergeComponents lost component transformer 'rca.analyze'")
	}
	if _, ok := merged.Transformers["base"]; !ok {
		t.Error("MergeComponents lost base transformer 'base'")
	}
}

func TestCircuitDef_ImportsField(t *testing.T) {
	yaml := `
circuit: test
imports:
  - vendor.rca-tools
  - vendor.vuln-tools
nodes:
  - name: start
edges:
  - id: e1
    from: start
    to: done
start: start
done: done
`
	def, err := LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if len(def.Imports) != 2 {
		t.Fatalf("imports = %v, want 2 entries", def.Imports)
	}
	if def.Imports[0] != "vendor.rca-tools" {
		t.Errorf("imports[0] = %q, want %q", def.Imports[0], "vendor.rca-tools")
	}
	if def.Imports[1] != "vendor.vuln-tools" {
		t.Errorf("imports[1] = %q, want %q", def.Imports[1], "vendor.vuln-tools")
	}
}
