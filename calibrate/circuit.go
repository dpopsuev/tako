package calibrate

import (
	"context"
	"embed"
	"fmt"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

//go:embed circuits/calibration-runner.yaml
var embeddedCircuits embed.FS

// CircuitOption configures a calibration circuit run.
type CircuitOption func(*circuitConfig)

type circuitConfig struct {
	observer circuit.WalkObserver
}

// WithObserver attaches a WalkObserver (e.g., Kami bridge) to the
// calibration circuit for live visualization and debugging.
func WithObserver(obs circuit.WalkObserver) CircuitOption {
	return func(c *circuitConfig) { c.observer = obs }
}

// RunCircuit executes the calibration-runner circuit with the given input.
// This is the primary API for running calibration as a DSL circuit instead
// of procedural code.
//
// Usage:
//
//	report, err := calibrate.RunCircuit(ctx, &calibrate.CalibrationInput{
//	    Scenario:    "ptp",
//	    Transformer: "cursor",
//	    Cases:       cases,
//	    GroundTruth: gt,
//	    ScoreCard:  sc,
//	    CaseRunner: myRunner,
//	    CaseScorer: myScorer,
//	}, calibrate.WithObserver(kamiBridge))
func RunCircuit(ctx context.Context, input *CalibrationInput, opts ...CircuitOption) (*CalibrationReport, error) {
	cfg := &circuitConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	def, err := CircuitDef()
	if err != nil {
		return nil, err
	}

	edgeIDs := make([]string, len(def.Edges))
	for i := range def.Edges {
		edgeIDs[i] = def.Edges[i].ID
	}

	reg := &engine.GraphRegistries{
		Nodes: CalibrationNodeRegistry(),
		Edges: forwardEdgeFactory(edgeIDs...),
	}

	graph, err := engine.BuildGraph(def, reg)
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}

	if cfg.observer != nil {
		if dg, ok := graph.(*engine.DefaultGraph); ok {
			dg.SetObserver(cfg.observer)
		}
	}

	walker := circuit.NewProcessWalker("calibration")
	walker.State().Context["input"] = input

	if err := graph.Walk(ctx, walker, def.Start); err != nil {
		return nil, fmt.Errorf("walk circuit: %w", err)
	}

	reportArt, ok := walker.State().Outputs["report"]
	if !ok {
		return nil, fmt.Errorf("circuit did not produce a report artifact")
	}

	report, ok := reportArt.Raw().(*CalibrationReport)
	if !ok {
		return nil, fmt.Errorf("report artifact type %T, want *CalibrationReport", reportArt.Raw())
	}

	return report, nil
}

// CircuitDef returns the parsed calibration circuit definition.
// Useful for Kami registration or custom graph building.
func CircuitDef() (*circuit.CircuitDef, error) {
	data, err := circuitYAML()
	if err != nil {
		return nil, err
	}
	return circuit.LoadCircuit(data)
}

// circuitYAML returns the raw YAML for the calibration-runner circuit.
// The YAML is loaded from disk (testdata-adjacent) so it can be edited
// without recompilation.
func circuitYAML() ([]byte, error) {
	return embeddedCircuits.ReadFile("circuits/calibration-runner.yaml")
}

func forwardEdgeFactory(ids ...string) engine.EdgeFactory {
	ef := make(engine.EdgeFactory, len(ids))
	for _, id := range ids {
		ef[id] = func(def circuit.EdgeDef) circuit.Edge {
			return &forwardEdge{def: def}
		}
	}
	return ef
}

type forwardEdge struct {
	def circuit.EdgeDef
}

func (e *forwardEdge) ID() string         { return e.def.ID }
func (e *forwardEdge) From() string       { return e.def.From }
func (e *forwardEdge) To() string         { return e.def.To }
func (e *forwardEdge) IsShortcut() bool   { return e.def.Shortcut }
func (e *forwardEdge) IsLoop() bool       { return e.def.Loop }

func (e *forwardEdge) Evaluate(_ circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
	return &circuit.Transition{NextNode: e.def.To}
}
