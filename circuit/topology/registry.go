package topology

import (
	"fmt"
	"sync"
)

// Registry holds named topology definitions.
type Registry struct {
	mu    sync.RWMutex
	topos map[string]*TopologyDef
}

// NewRegistry creates an empty topology registry.
func NewRegistry() *Registry {
	return &Registry{topos: make(map[string]*TopologyDef)}
}

// Register adds a topology definition. Returns an error on duplicate name.
func (r *Registry) Register(def *TopologyDef) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.topos[def.Name]; exists {
		return fmt.Errorf("%w: %q", ErrTopologyAlreadyRegistered, def.Name)
	}
	r.topos[def.Name] = def
	return nil
}

// Lookup returns the topology definition for the given name.
func (r *Registry) Lookup(name string) (*TopologyDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, ok := r.topos[name]
	return def, ok
}

// List returns all registered topology names in no particular order.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.topos))
	for name := range r.topos {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry returns a registry pre-loaded with the five built-in
// topology primitives: cascade, fan-out, fan-in, feedback-loop, bridge.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Cascade: linear pipeline — each stage feeds the next.
	//
	//   [entry] ──▶ [stage] ──▶ [stage] ──▶ [exit]
	//   0 in        1 in        1 in        1 in
	//   1 out       1 out       1 out       0 out
	//
	// Use for deterministic sequential processing (recall → triage → report).
	// Analogous to Unix pipes or Chain of Responsibility.
	_ = r.Register(&TopologyDef{
		Name:        "cascade",
		Description: "N stages in series, each with exactly 1 input and 1 output",
		MinNodes:    2,
		MaxNodes:    -1,
		Rules: []PositionRule{
			{Position: PositionEntry, MinInputs: 0, MaxInputs: 0, MinOutputs: 1, MaxOutputs: 1},
			{Position: PositionIntermediate, MinInputs: 1, MaxInputs: 1, MinOutputs: 1, MaxOutputs: 1},
			{Position: PositionExit, MinInputs: 1, MaxInputs: 1, MinOutputs: 0, MaxOutputs: 0},
		},
	})

	// Fan-out: one source scatters work to N parallel targets.
	//
	//              ┌──▶ [B]
	//   [A] ──▶───┼──▶ [C]
	//              └──▶ [D]
	//   0 in       1 in each
	//   2+ out     0 out each
	//
	// Use for parallel analysis (investigate multiple subsystems at once).
	// Analogous to Scatter or the Observer pattern.
	_ = r.Register(&TopologyDef{
		Name:        "fan-out",
		Description: "1 source fans to N target nodes",
		MinNodes:    2,
		MaxNodes:    -1,
		Rules: []PositionRule{
			{Position: PositionEntry, MinInputs: 0, MaxInputs: 0, MinOutputs: 2, MaxOutputs: -1},
			{Position: PositionExit, MinInputs: 1, MaxInputs: 1, MinOutputs: 0, MaxOutputs: 0},
		},
	})

	// Fan-in: N sources converge to one target that aggregates results.
	//
	//   [A] ──▶──┐
	//   [B] ──▶──┼──▶ [merge]
	//   [C] ──▶──┘
	//   0 in each     2+ in
	//   1 out each    0 out
	//
	// Use for evidence aggregation (gather parallel findings into one report).
	// Analogous to Gather, Barrier, or Join.
	_ = r.Register(&TopologyDef{
		Name:        "fan-in",
		Description: "N source nodes merge to 1 target node",
		MinNodes:    2,
		MaxNodes:    -1,
		Rules: []PositionRule{
			{Position: PositionEntry, MinInputs: 0, MaxInputs: 0, MinOutputs: 1, MaxOutputs: 1},
			{Position: PositionExit, MinInputs: 2, MaxInputs: -1, MinOutputs: 0, MaxOutputs: 0},
		},
	})

	// Feedback-loop: cascade with one back-edge for iterative refinement.
	//
	//              ◀── back-edge ──┐
	//              │               │
	//   [entry] ──▶ [work] ──▶ [eval] ──▶ [exit]
	//   0-1 in     1-2 in      1 in       1 in
	//   1 out      1-2 out     0-1 out    0 out
	//
	// Use for retry / refine loops (hallucination check → re-prompt).
	// Analogous to Retry or Visitor with re-entry.
	_ = r.Register(&TopologyDef{
		Name:        "feedback-loop",
		Description: "Cascade with one back-edge from downstream to upstream",
		MinNodes:    2,
		MaxNodes:    -1,
		Rules: []PositionRule{
			{Position: PositionEntry, MinInputs: 0, MaxInputs: 1, MinOutputs: 1, MaxOutputs: 1},
			{Position: PositionIntermediate, MinInputs: 1, MaxInputs: 2, MinOutputs: 1, MaxOutputs: 2},
			{Position: PositionExit, MinInputs: 1, MaxInputs: 1, MinOutputs: 0, MaxOutputs: 1},
		},
	})

	// Bridge: two parallel paths with cross-connections for mutual enrichment.
	//
	//   [A1] ──▶ [B1] ──▶ [exit1]
	//              ⇅ cross
	//   [A2] ──▶ [B2] ──▶ [exit2]
	//
	// Intermediate nodes accept 1-2 inputs and produce 1-2 outputs
	// to allow cross-path data exchange without merging paths.
	//
	// Use for adversarial review (prosecutor/defender exchange evidence).
	// Analogous to the Bridge pattern or dual-pipeline with coupling.
	_ = r.Register(&TopologyDef{
		Name:        "bridge",
		Description: "Two parallel paths with a cross-connection edge",
		MinNodes:    4,
		MaxNodes:    -1,
		Rules: []PositionRule{
			{Position: PositionEntry, MinInputs: 0, MaxInputs: 0, MinOutputs: 1, MaxOutputs: 2},
			{Position: PositionIntermediate, MinInputs: 1, MaxInputs: 2, MinOutputs: 1, MaxOutputs: 2},
			{Position: PositionExit, MinInputs: 1, MaxInputs: 2, MinOutputs: 0, MaxOutputs: 0},
		},
	})

	// Delegate: a node that spawns and walks a sub-circuit internally.
	//
	//   [entry] ──▶ [delegate] ──▶ [exit]
	//                  │
	//                  └─ walks sub-circuit:
	//                     [s1] ──▶ [s2] ──▶ [s3]
	//
	// Externally, delegate looks like any 1-in/1-out node.
	// Internally, it generates a CircuitDef at runtime and walks it.
	//
	// Use for dynamic sub-workflows (generate N investigation branches).
	// Analogous to the Strategy or Template Method pattern.
	_ = r.Register(&TopologyDef{
		Name:        "delegate",
		Description: "Single delegate node: 1 input, 1 output, sub-walk replaces fan-out",
		MinNodes:    1,
		MaxNodes:    -1,
		Rules: []PositionRule{
			{Position: PositionEntry, MinInputs: 0, MaxInputs: 0, MinOutputs: 1, MaxOutputs: 1},
			{Position: PositionIntermediate, MinInputs: 1, MaxInputs: 1, MinOutputs: 1, MaxOutputs: 1},
			{Position: PositionExit, MinInputs: 1, MaxInputs: 1, MinOutputs: 0, MaxOutputs: 0},
		},
	})

	return r
}
