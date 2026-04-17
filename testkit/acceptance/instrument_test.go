package acceptance

// Feature: Instrument Dispatch
//   As a framework consumer
//   I want circuit nodes to dispatch through instrument manifests
//   So that I can use CLI tools as first-class circuit participants

import (
	"context"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

func TestInstrument_ExecDispatch_DummyEcho(t *testing.T) {
	// Scenario: Circuit with exec instrument completes and echoes input
	//   Given a circuit with a single node using instrument: dummy-echo, action: echo
	//   And the dummy-echo instrument manifest is registered
	//   When I walk the circuit with input
	//   Then the walk completes without error
	//   And the artifact contains the echoed input

	manifest, err := circuit.LoadInstrumentManifest(
		instrumentPath(t, "dummy-echo/instrument.yaml"),
	)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	def := loadFixture(t, "circuits/instrument-echo.yaml")

	reg := &engine.GraphRegistries{
		Manifests: engine.ManifestRegistry{
			"dummy-echo": manifest,
		},
		InstrumentDir: repoRoot(),
	}

	g, err := engine.BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := circuit.NewProcessWalker("test")
	if err := g.Walk(context.Background(), walker, "greet"); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	art, ok := walker.State().Outputs["greet"]
	if !ok {
		t.Fatal("greet artifact missing from outputs")
	}

	if art.Type() != "instrument:dummy-echo:echo" {
		t.Errorf("Type() = %q, want instrument:dummy-echo:echo", art.Type())
	}

	raw, ok := art.Raw().(map[string]any)
	if !ok {
		t.Fatalf("Raw() type = %T, want map[string]any", art.Raw())
	}

	echo, ok := raw["echo"].(map[string]any)
	if !ok {
		t.Fatalf("echo type = %T, want map[string]any", raw["echo"])
	}
	if echo == nil {
		t.Error("echo payload is nil")
	}
}

func TestInstrument_ExecDispatch_DummyFail(t *testing.T) {
	// Scenario: Circuit with failing exec instrument returns error
	//   Given a circuit with a node using the dummy-fail instrument
	//   When I walk the circuit
	//   Then the walk returns an error

	manifest, err := circuit.LoadInstrumentManifest(
		instrumentPath(t, "dummy-fail/instrument.yaml"),
	)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	def := &circuit.CircuitDef{
		Circuit: "fail-test",
		Start:   "boom",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "boom", Instrument: "dummy-fail", Action: "fail"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "boom-done", From: "boom", To: "_done"},
		},
	}

	reg := &engine.GraphRegistries{
		Manifests: engine.ManifestRegistry{
			"dummy-fail": manifest,
		},
		InstrumentDir: repoRoot(),
	}

	g, err := engine.BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := circuit.NewProcessWalker("test")
	err = g.Walk(context.Background(), walker, "boom")
	if err == nil {
		t.Fatal("Walk should fail for dummy-fail instrument")
	}
}

func TestInstrument_MultiInstrument_CorrectRouting(t *testing.T) {
	// Scenario: Circuit with two different instruments dispatches correctly
	//   Given a circuit with node "greet" using dummy-echo and node "transform" using dummy-upper
	//   When I walk the circuit
	//   Then greet produces an echo artifact and transform produces an upper artifact
	//   And each artifact type identifies the correct instrument

	echoManifest, err := circuit.LoadInstrumentManifest(
		instrumentPath(t, "dummy-echo/instrument.yaml"),
	)
	if err != nil {
		t.Fatalf("load echo manifest: %v", err)
	}

	upperManifest, err := circuit.LoadInstrumentManifest(
		instrumentPath(t, "dummy-upper/instrument.yaml"),
	)
	if err != nil {
		t.Fatalf("load upper manifest: %v", err)
	}

	def := &circuit.CircuitDef{
		Circuit: "multi-instrument",
		Start:   "greet",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "greet", Instrument: "dummy-echo", Action: "echo"},
			{Name: "transform", Instrument: "dummy-upper", Action: "upper"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "greet-transform", From: "greet", To: "transform"},
			{ID: "transform-done", From: "transform", To: "_done"},
		},
	}

	reg := &engine.GraphRegistries{
		Manifests: engine.ManifestRegistry{
			"dummy-echo":  echoManifest,
			"dummy-upper": upperManifest,
		},
		InstrumentDir: repoRoot(),
	}

	g, err := engine.BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := circuit.NewProcessWalker("test")
	if err := g.Walk(context.Background(), walker, "greet"); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Verify greet artifact comes from dummy-echo.
	greetArt, ok := walker.State().Outputs["greet"]
	if !ok {
		t.Fatal("greet artifact missing from outputs")
	}
	if greetArt.Type() != "instrument:dummy-echo:echo" {
		t.Errorf("greet Type() = %q, want instrument:dummy-echo:echo", greetArt.Type())
	}

	// Verify transform artifact comes from dummy-upper.
	transformArt, ok := walker.State().Outputs["transform"]
	if !ok {
		t.Fatal("transform artifact missing from outputs")
	}
	if transformArt.Type() != "instrument:dummy-upper:upper" {
		t.Errorf("transform Type() = %q, want instrument:dummy-upper:upper", transformArt.Type())
	}

	// Verify the upper artifact contains uppercased output.
	raw, ok := transformArt.Raw().(map[string]any)
	if !ok {
		t.Fatalf("transform Raw() type = %T, want map[string]any", transformArt.Raw())
	}
	upper, ok := raw["upper"].(string)
	if !ok {
		t.Fatalf("upper field type = %T, want string", raw["upper"])
	}
	if upper == "" {
		t.Error("upper output is empty")
	}
}

func TestInstrument_OutputSchemaViolation_RejectsWalk(t *testing.T) {
	// Scenario: Instrument returns JSON that violates declared output_schema
	//   Given a circuit with a node using dummy-badoutput which declares
	//     output_schema requiring "status" field but returns {"wrong": "field"}
	//   When I walk the circuit
	//   Then the walk fails with an error mentioning schema violation

	manifest, err := circuit.LoadInstrumentManifest(
		instrumentPath(t, "dummy-badoutput/instrument.yaml"),
	)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	def := &circuit.CircuitDef{
		Circuit: "schema-violation",
		Start:   "check",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "check", Instrument: "dummy-badoutput", Action: "run"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "check-done", From: "check", To: "_done"},
		},
	}

	reg := &engine.GraphRegistries{
		Manifests: engine.ManifestRegistry{
			"dummy-badoutput": manifest,
		},
		InstrumentDir: repoRoot(),
	}

	g, err := engine.BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := circuit.NewProcessWalker("test")
	err = g.Walk(context.Background(), walker, "check")
	if err == nil {
		t.Fatal("Walk should fail when instrument output violates schema")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "schema violation") && !strings.Contains(errMsg, "missing required") {
		t.Errorf("error should mention schema violation, got: %s", errMsg)
	}
}

// instrumentPath returns the absolute path to a testkit instrument fixture.
func instrumentPath(t *testing.T, rel string) string {
	t.Helper()
	return repoRoot() + "/testkit/instruments/" + rel
}
