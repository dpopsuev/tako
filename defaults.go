package framework
// Category: Processing & Support

// DefaultWalker returns a zero-config Walker suitable for consumers that
// don't need persona or element customization. Uses Earth element
// (stable, methodical) and Sentinel persona (observant, reliable).
//
// The identity is deterministic: calling DefaultWalker() twice produces
// identical walkers, making circuit runs reproducible.
func DefaultWalker() Walker {
	return defaultWalkerWith(ElementEarth)
}

// DefaultWalkerWithElement returns a default Walker with a custom element.
// The persona remains Sentinel; only the element changes.
func DefaultWalkerWithElement(element Element) Walker {
	return defaultWalkerWith(element)
}

func defaultWalkerWith(element Element) *ProcessWalker {
	var id AgentIdentity
	if DefaultPersonaResolver != nil {
		if p, ok := DefaultPersonaResolver("Sentinel"); ok {
			id = p.Identity
		}
	}
	id.Element = element
	return NewProcessWalkerWithIdentity(id, "default")
}
