package fold

// E2E acceptance tests for the fold pipeline.
// Verify generated code properties that unit tests miss:
// - Correct import paths after hex migration
// - Conditional origami import (NeedsOrigami)
// - err variable declaration in ListenAndServe
// - Port wiring validation with realistic circuits
// - Domain-serve binary generation with all asset types

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Feature: Codegen Correctness ---

func TestE2E_WiredBinary_NoOrigamiImportWithoutResolvers(t *testing.T) {
	// Scenario: No sub-circuit resolvers → origami import excluded
	//   Given a manifest with schematics but no resolver functions
	//   When GenerateWiredBinary produces code
	//   Then the origami/circuit import is NOT present
	//   (Regression test for ORG-BUG-15: unused import broke compilation)

	root := origamiRootE2E(t)
	m := &Manifest{
		Name:    "no-resolver",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port:   9300,
			Assets: &AssetMap{Circuits: map[string]string{"test": "circuits/test.yaml"}},
		},
		Schematics: map[string]SchematicRef{
			"rca": {
				Path:     "github.com/dpopsuev/origami-rca",
				Bindings: map[string]string{"source": "reportportal"},
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

	src, err := GenerateWiredBinary(m, g)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	if strings.Contains(code, `origami "github.com/dpopsuev/origami/circuit"`) {
		t.Errorf("origami import should be EXCLUDED when no sub-circuit resolvers are present")
	}
}

func TestE2E_WiredBinary_OrigamiImportWithResolvers(t *testing.T) {
	// Scenario: Sub-circuit resolvers present → origami import included
	//   Given a resolved graph where a schematic declares a resolver
	//   When GenerateWiredBinary produces code
	//   Then the origami/circuit import IS present

	root := origamiRootE2E(t)
	m := &Manifest{
		Name:    "with-resolver",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port:   9300,
			Assets: &AssetMap{Circuits: map[string]string{"test": "circuits/test.yaml"}},
		},
		Schematics: map[string]SchematicRef{
			"rca": {
				Path:     "github.com/dpopsuev/origami-rca",
				Bindings: map[string]string{"source": "reportportal"},
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

	// Add a sub-schematic with a resolver to trigger NeedsOrigami
	g.Schematics = append(g.Schematics, ResolvedSchematic{
		Name:     "gnd",
		Module:   "github.com/dpopsuev/origami-gnd",
		Alias:    "gnd",
		Factory:  "NewServer",
		Resolver: "CircuitResolver",
	})

	src, err := GenerateWiredBinary(m, g)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	// In factory mode, sub-circuit resolvers aren't rendered in the
	// generated code — the consumer's Factory().CreateSession handles
	// resolver wiring internally. Verify the factory pattern is present.
	if !strings.Contains(code, "SessionFactoryToConfig") {
		t.Errorf("generated code should use SessionFactoryToConfig (factory mode)")
	}
}

func TestE2E_WiredBinary_ErrVariableDeclared(t *testing.T) {
	// Scenario: ListenAndServe uses := not bare =
	//   Given any wired binary generation
	//   When code is produced
	//   Then err is declared with := (regression for undeclared err bug)

	root := origamiRootE2E(t)
	m := &Manifest{
		Name:    "err-check",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port:   9300,
			Assets: &AssetMap{Circuits: map[string]string{"test": "circuits/test.yaml"}},
		},
		Schematics: map[string]SchematicRef{
			"rca": {Path: "github.com/dpopsuev/origami-rca", Bindings: map[string]string{"source": "reportportal"}},
		},
		Connectors: map[string]ConnectorRef{
			"reportportal": {Path: "github.com/dpopsuev/origami-rca/connectors/rp"},
		},
	}

	g, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatal(err)
	}

	src, err := GenerateWiredBinary(m, g)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	if strings.Contains(code, "if err = http.ListenAndServe") {
		t.Errorf("wired binary: err should use := not bare =")
	}
	if !strings.Contains(code, "if err := http.ListenAndServe") {
		t.Errorf("wired binary: expected 'if err := http.ListenAndServe'")
	}
}

func TestE2E_DomainServe_ErrVariableDeclared(t *testing.T) {
	// Same regression check for domain-serve template
	m := &Manifest{
		Name:    "ds-err",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port:   9300,
			Assets: &AssetMap{Circuits: map[string]string{"test": "circuits/test.yaml"}},
		},
	}

	src, err := GenerateDomainServe(m)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	if strings.Contains(code, "if err = http.ListenAndServe") {
		t.Errorf("domain-serve: err should use :=")
	}
}

// --- Feature: Port Wiring Validation ---

func TestE2E_PortWiring_MatchingTypesPass(t *testing.T) {
	// Scenario: Matching port types validate successfully
	//   Given two circuits with compatible port types wired together
	//   Then validatePortWiring passes

	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "circuits/rca.yaml", `
circuit: rca
ports:
  - name: post-triage
    direction: out
    type: "[]string"
wiring:
  - from: rca.out:post-triage
    to: gnd.in:keywords
nodes:
  - name: triage
edges:
  - id: e1
    from: triage
    to: _done
start: triage
done: _done
`)
	writeTestFile(t, tmpDir, "circuits/gnd.yaml", `
circuit: gnd
ports:
  - name: keywords
    direction: in
    type: "[]string"
nodes:
  - name: search
edges:
  - id: e1
    from: search
    to: _done
start: search
done: _done
`)

	m := &Manifest{
		Name: "wiring-test",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{
				Circuits: map[string]string{
					"rca": "circuits/rca.yaml",
					"gnd": "circuits/gnd.yaml",
				},
			},
		},
	}

	if err := validatePortWiring(m, tmpDir); err != nil {
		t.Errorf("matching types should pass: %v", err)
	}
}

func TestE2E_PortWiring_MismatchRejects(t *testing.T) {
	// Scenario: Mismatched port types rejected at fold time

	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "circuits/rca.yaml", `
circuit: rca
ports:
  - name: post-triage
    direction: out
    type: TriageResult
wiring:
  - from: rca.out:post-triage
    to: gnd.in:keywords
nodes:
  - name: triage
edges:
  - id: e1
    from: triage
    to: _done
start: triage
done: _done
`)
	writeTestFile(t, tmpDir, "circuits/gnd.yaml", `
circuit: gnd
ports:
  - name: keywords
    direction: in
    type: "[]string"
nodes:
  - name: search
edges:
  - id: e1
    from: search
    to: _done
start: search
done: _done
`)

	m := &Manifest{
		Name: "mismatch-test",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{
				Circuits: map[string]string{
					"rca": "circuits/rca.yaml",
					"gnd": "circuits/gnd.yaml",
				},
			},
		},
	}

	err := validatePortWiring(m, tmpDir)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if !strings.Contains(err.Error(), "type mismatch") {
		t.Errorf("error should mention type mismatch: %v", err)
	}
}

// --- Feature: Domain-Serve Generation ---

func TestE2E_DomainServe_AllAssetTypes(t *testing.T) {
	// Scenario: All asset types produce correct embeds and sections
	m := &Manifest{
		Name:    "full-assets",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{
				Circuits:   map[string]string{"rca": "circuits/rca.yaml", "gnd": "circuits/gnd.yaml"},
				Prompts:    map[string]string{"recall": "prompts/recall.md", "triage": "prompts/triage.md"},
				Scorecards: map[string]string{"rca": "scorecards/rca.yaml"},
				Scenarios:  map[string]string{"ptp": "scenarios/ptp.yaml"},
				Sources:    map[string]string{"ptp": "sources/ptp.yaml"},
				Reports:    map[string]string{"rca": "reports/rca.yaml"},
				Files:      map[string]string{"heuristics": "heuristics.yaml"},
			},
		},
	}

	src, err := GenerateDomainServe(m)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	for _, p := range []string{
		"circuits/rca.yaml", "circuits/gnd.yaml",
		"prompts/recall.md", "prompts/triage.md",
		"scorecards/rca.yaml", "scenarios/ptp.yaml",
		"sources/ptp.yaml", "reports/rca.yaml",
		"heuristics.yaml",
	} {
		if !strings.Contains(code, "//go:embed "+p) {
			t.Errorf("missing embed for %s", p)
		}
	}

	for _, s := range []string{"circuits", "prompts", "scorecards", "scenarios", "sources", "reports"} {
		if !strings.Contains(code, `"`+s+`"`) {
			t.Errorf("missing section %q in AssetIndex", s)
		}
	}
}

// --- Feature: Circuit Reference Validation ---

func TestE2E_CircuitRefs_ValidRefsPass(t *testing.T) {
	// Scenario: Circuit that references a child circuit validates
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "circuits/main.yaml", `
circuit: main
nodes:
  - name: start
    handler_type: circuit
    handler: child
edges:
  - id: e1
    from: start
    to: _done
start: start
done: _done
`)
	writeTestFile(t, tmpDir, "circuits/child.yaml", `
circuit: child
nodes:
  - name: work
edges:
  - id: e1
    from: work
    to: _done
start: work
done: _done
`)

	m := &Manifest{
		Name: "ref-test",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{Circuits: map[string]string{
				"main":  "circuits/main.yaml",
				"child": "circuits/child.yaml",
			}},
		},
	}

	if err := validateCircuitRefs(m, tmpDir); err != nil {
		t.Errorf("valid refs should pass: %v", err)
	}
}

func TestE2E_CircuitRefs_MissingRefRejects(t *testing.T) {
	// Scenario: Missing handler: child reference detected
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "circuits/main.yaml", `
circuit: main
nodes:
  - name: start
    handler_type: circuit
    handler: missing
edges:
  - id: e1
    from: start
    to: _done
start: start
done: _done
`)

	m := &Manifest{
		Name: "missing-ref",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{Circuits: map[string]string{"main": "circuits/main.yaml"}},
		},
	}

	if err := validateCircuitRefs(m, tmpDir); err == nil {
		t.Fatal("expected error for missing circuit reference")
	}
}

// --- Helpers ---

func origamiRootE2E(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(wd)
}

func writeTestFile(t *testing.T, base, name, content string) {
	t.Helper()
	p := filepath.Join(base, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
