package engine

// Re-exports from engine/walker sub-package for backward compatibility.

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine/walker"
	"github.com/dpopsuev/troupe/identity"
)

// DefaultWalker is re-exported from engine/walker.
func DefaultWalker() circuit.Walker {
	return walker.DefaultWalker()
}

// DefaultWalkerWithElement is re-exported from engine/walker.
func DefaultWalkerWithElement(element identity.Element) circuit.Walker {
	return walker.DefaultWalkerWithElement(element)
}
