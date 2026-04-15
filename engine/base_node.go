package engine

import "github.com/dpopsuev/troupe/identity"

// baseNode provides the shared Name/Approach implementation for all engine node types.
type baseNode struct {
	name    string
	element identity.Element
}

func (n *baseNode) Name() string               { return n.name }
func (n *baseNode) Approach() identity.Element { return n.element }
