package agentport

import "github.com/dpopsuev/jericho/trait"

// Trait type aliases — behavioral trait primitives.
type (
	Trait       = trait.Trait
	TraitSet    = trait.Set
	InferConfig = trait.InferConfig
)

// Trait vocabulary constants.
const (
	TraitSpeed      = trait.Speed
	TraitReasoning  = trait.Reasoning
	TraitRigor      = trait.Rigor
	TraitCoding     = trait.Coding
	TraitDiscipline = trait.Discipline
	TraitToolUse    = trait.ToolUse
	TraitDiscourse  = trait.Discourse
	TraitVisual     = trait.Visual
)

// Functions.
var (
	InferFromIntent   = trait.InferFromIntent
	TraitFromVector   = trait.FromVector
	DefaultVocabulary = trait.DefaultVocabulary
)
