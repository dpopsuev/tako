package engine

import "github.com/dpopsuev/origami/circuit"

// TransformerForAllNodes registers a single Transformer under every node name
// in the given list. Useful for monolithic transformers that dispatch internally
// on the node name.
func TransformerForAllNodes(t Transformer, nodeNames []string) TransformerRegistry {
	reg := TransformerRegistry{}
	for _, name := range nodeNames {
		reg[name] = t
	}
	return reg
}

// ExtractorForAllNodes registers an extractor factory's output under every
// node name. The factory receives the node name and returns an Extractor.
func ExtractorForAllNodes(factory func(nodeName string) Extractor, nodeNames []string) ExtractorRegistry {
	reg := ExtractorRegistry{}
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
	for i := range cd.Nodes {
		names[i] = string(cd.Nodes[i].Name)
	}
	return names
}
