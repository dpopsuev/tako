package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

type ScorecardDef = circuit.ScorecardDef
type ScorecardMetric = circuit.ScorecardMetric
type CostModelDef = circuit.CostModelDef

func LoadScorecardDef(data []byte) (*ScorecardDef, error) { return circuit.LoadScorecardDef(data) }

func MergeScorecardDefs(base, overlay *ScorecardDef) (*ScorecardDef, error) {
	return circuit.MergeScorecardDefs(base, overlay)
}

func RegisterScorecardVocabulary(sd *ScorecardDef, v *RichMapVocabulary) {
	circuit.RegisterScorecardVocabulary(sd, v)
}
