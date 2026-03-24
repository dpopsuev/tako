package toolkit

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// TransformerForAllNodes registers a single Transformer under every node name
// in the given list. Useful for monolithic transformers that dispatch internally
// on the node name.
func TransformerForAllNodes(t engine.Transformer, nodeNames []string) engine.TransformerRegistry {
	reg := engine.TransformerRegistry{}
	for _, name := range nodeNames {
		reg[name] = t
	}
	return reg
}

// ExtractorForAllNodes registers an extractor factory's output under every
// node name. The factory receives the node name and returns an Extractor.
func ExtractorForAllNodes(factory func(nodeName string) engine.Extractor, nodeNames []string) engine.ExtractorRegistry {
	reg := engine.ExtractorRegistry{}
	for _, name := range nodeNames {
		reg[name] = factory(name)
	}
	return reg
}

// NodeNamesFromCircuit extracts the ordered list of node names from a
// CircuitDef. This replaces hardcoded node name lists in schematics.
func NodeNamesFromCircuit(cd *circuit.CircuitDef) []string {
	if cd == nil {
		return nil
	}
	names := make([]string, len(cd.Nodes))
	for i, n := range cd.Nodes {
		names[i] = n.Name
	}
	return names
}
