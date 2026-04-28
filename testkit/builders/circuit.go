// Package builders provides fluent test builders for framework types.
// Every builder returns fully initialized structs ready for test use.
package builders

import (
	"fmt"

	"github.com/dpopsuev/tako/circuit"
)

// CircuitDefBuilder constructs a CircuitDef incrementally for tests.
type CircuitDefBuilder struct {
	def       circuit.CircuitDef
	edgeCount int
}

// NewCircuitDef creates a new CircuitDefBuilder with the given circuit name.
func NewCircuitDef(name string) *CircuitDefBuilder {
	return &CircuitDefBuilder{
		def: circuit.CircuitDef{
			Circuit: name,
		},
	}
}

// AddNode adds a node with the given name and action.
func (b *CircuitDefBuilder) AddNode(name, action string) *CircuitDefBuilder {
	b.def.Nodes = append(b.def.Nodes, circuit.NodeDef{
		Name:   circuit.NodeName(name),
		Action: action,
	})
	return b
}

// AddNodeWithInstrument adds a node with the given name, instrument, and action.
func (b *CircuitDefBuilder) AddNodeWithInstrument(name, instrument, action string) *CircuitDefBuilder {
	b.def.Nodes = append(b.def.Nodes, circuit.NodeDef{
		Name:       circuit.NodeName(name),
		Instrument: instrument,
		Action:     action,
	})
	return b
}

// AddNodeDef adds a fully specified NodeDef.
func (b *CircuitDefBuilder) AddNodeDef(nd *circuit.NodeDef) *CircuitDefBuilder {
	b.def.Nodes = append(b.def.Nodes, *nd)
	return b
}

// AddEdge adds an edge from->to with the given when expression.
// An auto-generated ID is assigned.
func (b *CircuitDefBuilder) AddEdge(from, to, when string) *CircuitDefBuilder {
	b.edgeCount++
	id := fmt.Sprintf("e%d-%s-%s", b.edgeCount, from, to)
	b.def.Edges = append(b.def.Edges, circuit.EdgeDef{
		ID:   id,
		From: circuit.NodeName(from),
		To:   circuit.NodeName(to),
		When: when,
	})
	return b
}

// AddEdgeDef adds a fully specified EdgeDef.
func (b *CircuitDefBuilder) AddEdgeDef(ed *circuit.EdgeDef) *CircuitDefBuilder {
	b.def.Edges = append(b.def.Edges, *ed)
	return b
}

// Start sets the start node.
func (b *CircuitDefBuilder) Start(node string) *CircuitDefBuilder {
	b.def.Start = circuit.NodeName(node)
	return b
}

// Done sets the done node.
func (b *CircuitDefBuilder) Done(node string) *CircuitDefBuilder {
	b.def.Done = circuit.NodeName(node)
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
func (b *CircuitDefBuilder) Build() *circuit.CircuitDef {
	def := b.def
	return &def
}
