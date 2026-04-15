// Package walker provides default walker construction for circuit execution.
package walker

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/troupe/identity"
)

// DefaultWalker returns a zero-config Walker suitable for consumers that
// don't need persona or element customization. Uses Earth element
// (stable, methodical) and Sentinel persona (observant, reliable).
//
// The identity is deterministic: calling DefaultWalker() twice produces
// identical walkers, making circuit runs reproducible.
func DefaultWalker() circuit.Walker {
	return defaultWalkerWith(identity.ElementEarth)
}

// DefaultWalkerWithElement returns a default Walker with a custom element.
// The persona remains Sentinel; only the element changes.
func DefaultWalkerWithElement(element identity.Element) circuit.Walker {
	return defaultWalkerWith(element)
}

func defaultWalkerWith(element identity.Element) *circuit.ProcessWalker {
	var id identity.Archetype
	resolver := identity.DefaultArchetypeResolver
	if resolver != nil {
		if p, ok := resolver("Sentinel"); ok {
			id = p
		}
	}
	id.Element = element
	return circuit.NewProcessWalkerWithIdentity(&id, "default")
}
