// Package walker provides default walker construction for circuit execution.
package walker

import (
	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tangle/visual"
)

// DefaultWalker returns a zero-config Walker suitable for consumers that
// don't need persona or element customization. Uses Earth element
// (stable, methodical).
func DefaultWalker() circuit.Walker {
	return defaultWalkerWith(visual.ElementEarth)
}

// DefaultWalkerWithElement returns a default Walker with a custom element.
func DefaultWalkerWithElement(element visual.Element) circuit.Walker {
	return defaultWalkerWith(element)
}

func defaultWalkerWith(element visual.Element) *circuit.ProcessWalker {
	id := circuit.AgentIdentity{
		Name:    "default",
		Element: element,
	}
	return circuit.NewProcessWalkerWithIdentity(&id, "default")
}
