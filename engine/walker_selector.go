package engine

import "github.com/dpopsuev/origami/circuit"

// WalkerSelector picks which walker handles a given node.
type WalkerSelector interface {
	SelectWalker(node circuit.Node, walkers []circuit.Walker, prior circuit.Walker) circuit.Walker
}

// WalkerSelectorFunc adapts a plain function to WalkerSelector.
type WalkerSelectorFunc func(node circuit.Node, walkers []circuit.Walker, prior circuit.Walker) circuit.Walker

func (f WalkerSelectorFunc) SelectWalker(node circuit.Node, walkers []circuit.Walker, prior circuit.Walker) circuit.Walker {
	return f(node, walkers, prior)
}
