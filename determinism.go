package framework

// Category: Processing & Support — aliases to engine/ package.
// isCircuitDeterministic is unexported and used only within root tests;
// delegate to engine via a thin wrapper.

import "github.com/dpopsuev/origami/engine"

// IsDeterministic is aliased from engine/ via transformer.go.
// isCircuitDeterministic is now in engine/ but unexported.
// Root tests that need it can call engine.isCircuitDeterministic indirectly
// or define a forwarding wrapper here if needed.

// isCircuitDeterministic remains accessible via engine for root-package tests.
func isCircuitDeterministic(def *CircuitDef, reg TransformerRegistry) bool {
	_ = engine.IsDeterministic // ensure engine is imported
	// The function is now in engine/ as unexported. Root tests that used it
	// can access it through BuildGraph determinism or test helpers.
	// For backward compatibility, re-implement inline:
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
