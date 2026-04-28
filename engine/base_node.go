package engine

import "github.com/dpopsuev/tangle/visual"

// baseNode provides the shared Name/Approach implementation for all engine node types.
type baseNode struct {
	name    string
	element visual.Element
}

func (n *baseNode) Name() string               { return n.name }
func (n *baseNode) Approach() visual.Element { return n.element }
