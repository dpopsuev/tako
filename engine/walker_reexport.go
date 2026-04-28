package engine

// Re-exports from engine/walker sub-package for backward compatibility.

import (
	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine/walker"
	"github.com/dpopsuev/tangle/visual"
)

// DefaultWalker is re-exported from engine/walker.
func DefaultWalker() circuit.Walker {
	return walker.DefaultWalker()
}

// DefaultWalkerWithElement is re-exported from engine/walker.
func DefaultWalkerWithElement(element visual.Element) circuit.Walker {
	return walker.DefaultWalkerWithElement(element)
}
