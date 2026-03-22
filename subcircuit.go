package framework

// Category: DSL & Build — aliases to circuit/ package.

import (
	"io/fs"

	"github.com/dpopsuev/origami/circuit"
)

func LoadSubCircuitsFromFS(fsys fs.FS, resolvers map[string]AssetResolver) map[string]*CircuitDef {
	return circuit.LoadSubCircuitsFromFS(fsys, resolvers)
}
