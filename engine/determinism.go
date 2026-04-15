package engine

import "github.com/dpopsuev/origami/circuit"

// IsCircuitDeterministic returns true if every node in the circuit that
// references an instrument resolves to a deterministic instrument.
func IsCircuitDeterministic(def *circuit.CircuitDef, reg InstrumentRegistry) bool {
	if reg == nil {
		return false
	}
	for i := range def.Nodes {
		nd := &def.Nodes[i]
		if nd.Instrument != InstrumentTransformer {
			continue
		}
		action := nd.Action
		if action == "" {
			action = string(nd.Name)
		}
		t, err := reg.Get(action)
		if err != nil {
			return false
		}
		if !IsDeterministic(t) {
			return false
		}
	}
	return true
}
