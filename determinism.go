package framework

// Category: Processing & Support — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

// isCircuitDeterministic delegates to engine package.
func isCircuitDeterministic(def *CircuitDef, reg TransformerRegistry) bool {
	return engine.IsCircuitDeterministic(def, reg)
}
