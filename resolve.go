package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

type ResolveOption = circuit.ResolveOption

func WithSearchDirs(dirs ...string) ResolveOption { return circuit.WithSearchDirs(dirs...) }

func RegisterEmbeddedCircuit(name string, content []byte) {
	circuit.RegisterEmbeddedCircuit(name, content)
}

func ResolveCircuitPath(name string, opts ...ResolveOption) ([]byte, error) {
	return circuit.ResolveCircuitPath(name, opts...)
}

// clearEmbeddedCircuits is for testing only.
func clearEmbeddedCircuits() {
	circuit.ClearEmbeddedCircuits()
}
