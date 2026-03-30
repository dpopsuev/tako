package resource

import (
	"github.com/dpopsuev/origami/circuit"
)

// DefaultRegistry returns a KindRegistry pre-loaded with all framework
// kinds that have parsers in the circuit/ package. Domain kinds (scenario,
// source-pack, etc.) are registered by consumers.
func DefaultRegistry() *KindRegistry {
	reg := NewKindRegistry()

	// Framework kinds with parsers in circuit/.
	reg.Register(NewHandler(circuit.KindSchematic, circuit.LoadCircuit, nil, nil))
	reg.Register(NewHandler(circuit.KindStoreSchema, circuit.LoadStoreSchema, nil, circuit.MergeStoreSchemas))
	reg.Register(NewHandler(circuit.KindScorecard, circuit.LoadScorecardDef, nil, circuit.MergeScorecardDefs))
	reg.Register(NewHandler(circuit.KindReportTemplate, circuit.LoadReportTemplate, nil, circuit.MergeReportTemplates))

	return reg
}
