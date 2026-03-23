package engine

import "github.com/dpopsuev/origami/circuit"


// IsCircuitDeterministic returns true if every node in the circuit that
// references a transformer resolves to a deterministic transformer.
func IsCircuitDeterministic(def *circuit.CircuitDef, reg TransformerRegistry) bool {
	if reg == nil {
		return false
	}
	for _, nd := range def.Nodes {
		ht := nd.EffectiveHandlerType(def.HandlerType)
		name := nd.EffectiveHandler()
		if ht != HandlerTypeTransformer || name == "" {
			continue
		}
		t, err := reg.Get(name)
		if err != nil {
			return false
		}
		if !IsDeterministic(t) {
			return false
		}
	}
	return true
}
