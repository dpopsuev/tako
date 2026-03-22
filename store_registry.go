package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

type StoreRegistry = circuit.StoreRegistry
type StoreEngineFactory = circuit.StoreEngineFactory

func NewStoreRegistry(wiring *StoreWiring) *StoreRegistry {
	return circuit.NewStoreRegistry(wiring)
}
