package framework

// Category: DSL & Build — aliases to engine/ and circuit/ packages.

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// Manifest type aliases — definitions live in circuit/ sub-package.
type SocketDef = circuit.SocketDef
type SatisfiesDef = circuit.SatisfiesDef
type ComponentManifest = circuit.ComponentManifest

func LoadComponentManifest(path string) (*ComponentManifest, error) {
	return circuit.LoadComponentManifest(path)
}

// Component and MergeComponents — aliases to engine/ package.
type Component = engine.Component

var MergeComponents = engine.MergeComponents
