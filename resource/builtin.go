package resource

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/prompt"
)

// DefaultRegistry returns a KindRegistry pre-loaded with all known
// Origami kinds. Framework kinds use typed parsers from circuit/ and fold/.
// Domain kinds use passthrough handlers; consumers can override with
// typed handlers via Register().
func DefaultRegistry() *KindRegistry {
	reg := NewKindRegistry()

	// Framework kinds — typed parsers in circuit/.
	reg.Register(NewHandler(circuit.KindSchematic, circuit.LoadCircuit, nil, nil))
	reg.Register(NewHandler(circuit.KindStoreSchema, circuit.LoadStoreSchema, nil, circuit.MergeStoreSchemas))
	reg.Register(NewHandler(circuit.KindScorecard, circuit.LoadScorecardDef, nil, circuit.MergeScorecardDefs))
	reg.Register(NewHandler(circuit.KindReportTemplate, circuit.LoadReportTemplate, nil, circuit.MergeReportTemplates))

	// Board — passthrough (fold.ParseManifest is build-time, not runtime).
	reg.Register(NewPassthroughHandler(circuit.KindBoard))

	// Component — LoadComponentManifest takes path not bytes; use passthrough.
	reg.Register(NewPassthroughHandler(circuit.KindComponent))

	// Prompt — typed parser (markdown with YAML front matter).
	reg.Register(NewHandler(circuit.KindPrompt, prompt.ParsePrompt, nil, nil))

	// Domain kinds — passthrough (consumers override with typed handlers).
	reg.Register(NewPassthroughHandler(circuit.KindScenario))
	reg.Register(NewPassthroughHandler(circuit.KindSourcePack))
	reg.Register(NewPassthroughHandler(circuit.KindVocabulary))
	reg.Register(NewPassthroughHandler(circuit.KindHeuristicRules))
	reg.Register(NewPassthroughHandler(circuit.KindTuning))
	reg.Register(NewPassthroughHandler(circuit.KindArtifactSchema))
	reg.Register(NewPassthroughHandler(circuit.KindDataset))

	return reg
}
