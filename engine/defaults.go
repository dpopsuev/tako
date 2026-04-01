package engine

// Category: Execution — default walker construction.

import (
	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
)

// DefaultWalker returns a zero-config Walker suitable for consumers that
// don't need persona or element customization. Uses Earth element
// (stable, methodical) and Sentinel persona (observant, reliable).
//
// The identity is deterministic: calling DefaultWalker() twice produces
// identical walkers, making circuit runs reproducible.
func DefaultWalker() circuit.Walker {
	return defaultWalkerWith(circuit.ElementEarth)
}

// DefaultWalkerWithElement returns a default Walker with a custom element.
// The persona remains Sentinel; only the element changes.
func DefaultWalkerWithElement(element circuit.Element) circuit.Walker {
	return defaultWalkerWith(element)
}

func defaultWalkerWith(element circuit.Element) *circuit.ProcessWalker {
	var id circuit.AgentIdentity
	resolver := agentport.GetDefaultPersonaResolver()
	if resolver != nil {
		if p, ok := resolver("Sentinel"); ok {
			id = p
		}
	}
	id.Element = element
	return circuit.NewProcessWalkerWithIdentity(&id, "default")
}
