package engine

// Category: Execution — default walker construction.

import "github.com/dpopsuev/origami/core"

// DefaultWalker returns a zero-config Walker suitable for consumers that
// don't need persona or element customization. Uses Earth element
// (stable, methodical) and Sentinel persona (observant, reliable).
//
// The identity is deterministic: calling DefaultWalker() twice produces
// identical walkers, making circuit runs reproducible.
func DefaultWalker() core.Walker {
	return defaultWalkerWith(core.ElementEarth)
}

// DefaultWalkerWithElement returns a default Walker with a custom element.
// The persona remains Sentinel; only the element changes.
func DefaultWalkerWithElement(element core.Element) core.Walker {
	return defaultWalkerWith(element)
}

func defaultWalkerWith(element core.Element) *core.ProcessWalker {
	var id core.AgentIdentity
	if core.DefaultPersonaResolver != nil {
		if p, ok := core.DefaultPersonaResolver("Sentinel"); ok {
			id = p.Identity
		}
	}
	id.Element = element
	return core.NewProcessWalkerWithIdentity(id, "default")
}
