package acceptance

// Feature: Instrument Dispatch
//   As a framework consumer
//   I want circuit nodes to dispatch through instrument manifests
//   So that I can use CLI tools as first-class circuit participants

import (
	"context"
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
		Instruments: engine.InstrumentRegistry{
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
		Instruments: engine.InstrumentRegistry{
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

// instrumentPath returns the absolute path to a testkit instrument fixture.
func instrumentPath(t *testing.T, rel string) string {
	t.Helper()
	return repoRoot() + "/testkit/instruments/" + rel
}
