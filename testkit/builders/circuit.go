// Package builders provides fluent test builders for framework types.
// Every builder returns fully initialized structs ready for test use.
package builders

import (
	"fmt"

	framework "github.com/dpopsuev/origami"
)

// CircuitDefBuilder constructs a CircuitDef incrementally for tests.
type CircuitDefBuilder struct {
	def       framework.CircuitDef
	edgeCount int
}

// NewCircuitDef creates a new CircuitDefBuilder with the given circuit name.
func NewCircuitDef(name string) *CircuitDefBuilder {
	return &CircuitDefBuilder{
		def: framework.CircuitDef{
			Circuit: name,
		},
	}
}

// HandlerType sets the circuit-level handler_type.
func (b *CircuitDefBuilder) HandlerType(ht string) *CircuitDefBuilder {
	b.def.HandlerType = ht
	return b
}

// AddNode adds a node with the given name and handler.
func (b *CircuitDefBuilder) AddNode(name, handler string) *CircuitDefBuilder {
	b.def.Nodes = append(b.def.Nodes, framework.NodeDef{
		Name:    name,
		Handler: handler,
	})
	return b
}

// AddNodeDef adds a fully specified NodeDef.
func (b *CircuitDefBuilder) AddNodeDef(nd framework.NodeDef) *CircuitDefBuilder {
	b.def.Nodes = append(b.def.Nodes, nd)
	return b
}

// AddEdge adds an edge from->to with the given when expression.
// An auto-generated ID is assigned.
func (b *CircuitDefBuilder) AddEdge(from, to, when string) *CircuitDefBuilder {
	b.edgeCount++
	id := fmt.Sprintf("e%d-%s-%s", b.edgeCount, from, to)
	b.def.Edges = append(b.def.Edges, framework.EdgeDef{
		ID:   id,
		From: from,
		To:   to,
		When: when,
	})
	return b
}

// AddEdgeDef adds a fully specified EdgeDef.
func (b *CircuitDefBuilder) AddEdgeDef(ed framework.EdgeDef) *CircuitDefBuilder {
	b.def.Edges = append(b.def.Edges, ed)
	return b
}

// Start sets the start node.
func (b *CircuitDefBuilder) Start(node string) *CircuitDefBuilder {
	b.def.Start = node
	return b
}

// Done sets the done node.
func (b *CircuitDefBuilder) Done(node string) *CircuitDefBuilder {
	b.def.Done = node
	return b
}

// WithVar sets a circuit variable.
func (b *CircuitDefBuilder) WithVar(key string, val any) *CircuitDefBuilder {
	if b.def.Vars == nil {
		b.def.Vars = make(map[string]any)
	}
	b.def.Vars[key] = val
	return b
}

// WithDescription sets the circuit description.
func (b *CircuitDefBuilder) WithDescription(desc string) *CircuitDefBuilder {
	b.def.Description = desc
	return b
}

// WithTopology sets the topology type.
func (b *CircuitDefBuilder) WithTopology(topo string) *CircuitDefBuilder {
	b.def.Topology = topo
	return b
}

// Build returns the constructed CircuitDef.
func (b *CircuitDefBuilder) Build() *framework.CircuitDef {
	def := b.def
	return &def
}
