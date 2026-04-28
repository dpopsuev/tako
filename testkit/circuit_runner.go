// Package testkit provides test utilities for circuit development.
// RunCircuit executes a circuit in-process with stub transformers —
// no MCP server, no containers, no dispatch. Pure engine execution.
package testkit

import (
	"context"
	"fmt"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
	"github.com/dpopsuev/tako/engine/trace"
)

// RunResult captures the outcome of a testkit circuit run.
type RunResult struct {
	// Events contains all walk events (node enter/exit, transitions, errors).
	Events []circuit.WalkEvent

	// Error is non-nil if the walk failed.
	Error error

	// Path is the ordered list of visited node names.
	Path []string
}

// Enters returns node_enter events only.
func (r *RunResult) Enters() []circuit.WalkEvent {
	return r.eventsOfType(circuit.EventNodeEnter)
}

// Exits returns node_exit events only.
func (r *RunResult) Exits() []circuit.WalkEvent {
	return r.eventsOfType(circuit.EventNodeExit)
}

// Visited returns true if the named node was entered during the walk.
func (r *RunResult) Visited(node string) bool {
	for _, e := range r.Events {
		if e.Type == circuit.EventNodeEnter && e.Node == node {
			return true
		}
	}
	return false
}

func (r *RunResult) eventsOfType(t circuit.WalkEventType) []circuit.WalkEvent {
	var out []circuit.WalkEvent
	for _, e := range r.Events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

// RunCircuitOption configures a testkit circuit run.
type RunCircuitOption func(*runCircuitConfig)

type runCircuitConfig struct {
	input      any
	extractors engine.ExtractorRegistry
	hooks      engine.HookRegistry
	circuits   map[string]*circuit.CircuitDef
}

// WithInput sets the initial input for the walk.
func WithInput(input any) RunCircuitOption {
	return func(c *runCircuitConfig) { c.input = input }
}

// WithSubCircuits registers sub-circuit definitions for delegate nodes.
func WithSubCircuits(circuits map[string]*circuit.CircuitDef) RunCircuitOption {
	return func(c *runCircuitConfig) { c.circuits = circuits }
}

// WithExtractors registers extractors for the circuit run.
func WithExtractors(extractors engine.ExtractorRegistry) RunCircuitOption {
	return func(c *runCircuitConfig) { c.extractors = extractors }
}

// WithHooks registers hooks for the circuit run.
func WithHooks(hooks engine.HookRegistry) RunCircuitOption {
	return func(c *runCircuitConfig) { c.hooks = hooks }
}

// RunCircuit executes a circuit in-process with the given transformers.
// No MCP, no containers, no dispatch — pure engine walk. Returns a
// RunResult with events, path, and error for test assertions.
func RunCircuit(ctx context.Context, def *circuit.CircuitDef, transformers engine.InstrumentRegistry, opts ...RunCircuitOption) *RunResult {
	cfg := &runCircuitConfig{}
	for _, o := range opts {
		o(cfg)
	}

	reg := &engine.GraphRegistries{
		Instruments: transformers,
		Extractors:  cfg.extractors,
		Hooks:       cfg.hooks,
		Circuits:    cfg.circuits,
	}

	collector := &trace.TraceCollector{}

	runner, err := engine.NewRunnerWith(def, reg)
	if err != nil {
		return &RunResult{Error: fmt.Errorf("build graph: %w", err)}
	}

	if dg, ok := runner.Graph.(*engine.DefaultGraph); ok {
		dg.SetObserver(collector)
	}

	walker := engine.DefaultWalker()
	if cfg.input != nil {
		walker.State().Context["input"] = cfg.input
	}

	walkErr := runner.Walk(ctx, walker, string(def.Start))

	events := collector.Events()
	path := make([]string, 0)
	for _, e := range events {
		if e.Type == circuit.EventNodeEnter {
			path = append(path, e.Node)
		}
	}

	return &RunResult{
		Events: events,
		Error:  walkErr,
		Path:   path,
	}
}
