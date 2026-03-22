package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

type StoreWiring = circuit.StoreWiring
type StoreBinding = circuit.StoreBinding
type StoreLifecycle = circuit.StoreLifecycle

const (
	LifecycleSession    = circuit.LifecycleSession
	LifecyclePersistent = circuit.LifecyclePersistent
)

type StoreDeclaration = circuit.StoreDeclaration
type StoreEngine = circuit.StoreEngine
type StoreSchema = circuit.StoreSchema
type StoreTableDef = circuit.StoreTableDef
type StoreColumnDef = circuit.StoreColumnDef
type StoreIndexDef = circuit.StoreIndexDef
type SchemaProvider = circuit.SchemaProvider

func LoadStoreSchema(data []byte) (*StoreSchema, error) { return circuit.LoadStoreSchema(data) }

func MergeStoreSchemas(base, overlay *StoreSchema) (*StoreSchema, error) {
	return circuit.MergeStoreSchemas(base, overlay)
}
