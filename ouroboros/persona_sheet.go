package ouroboros

import (
	"fmt"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/bugle/element"
	"github.com/dpopsuev/bugle/persona"
	"gopkg.in/yaml.v3"
)

// PersonaSheet is a per-model routing document combining ModelProfile with
// circuit step affinity. It is the output artifact that the AffinityScheduler
// and agent router consume for performance optimization.
type PersonaSheet struct {
	Model             string                         `yaml:"model"              json:"model"`
	ElementMatch      element.Element              `yaml:"element_match"      json:"element_match"`
	DimensionScores   map[Dimension]float64          `yaml:"dimension_scores"   json:"dimension_scores"`
	ElementScores     map[element.Element]float64  `yaml:"element_scores"     json:"element_scores"`
	SuggestedPersonas map[string]string              `yaml:"suggested_personas" json:"suggested_personas"`
	CostProfile       circuit.CostProfile          `yaml:"cost_profile"       json:"cost_profile"`
	GeneratedAt       time.Time                      `yaml:"generated_at"       json:"generated_at"`
}

// EmitPersonaSheet combines a ModelProfile with a circuit definition to produce
// a per-model routing document. stepDims maps step names to the behavioral
// dimensions that matter for that step. Steps present in stepDims get a
// dimension-specific element suggestion; steps absent from the map (or when
// stepDims is nil) receive a generalist assignment based on the profile's
// overall ElementMatch.
func EmitPersonaSheet(profile ModelProfile, circuit circuit.CircuitDef, stepDims StepDimensionMap) (*PersonaSheet, error) {
	if profile.Model.ModelName == "" {
		return nil, fmt.Errorf("model identity is empty")
	}

	stepAffinity := DeriveStepAffinity(profile, stepDims)

	suggestions := make(map[string]string)
	for _, node := range circuit.Nodes {
		if node.Name == "_done" || node.Name == "" {
			continue
		}
		affinity, ok := stepAffinity[node.Name]
		if ok && affinity > 0 {
			element := suggestElementForStep(node.Name, profile, stepDims)
			suggestions[node.Name] = personaNameForElement(element)
		} else {
			suggestions[node.Name] = personaNameForElement(profile.ElementMatch)
		}
	}

	return &PersonaSheet{
		Model:             profile.Model.String(),
		ElementMatch:      profile.ElementMatch,
		DimensionScores:   profile.Dimensions,
		ElementScores:     profile.ElementScores,
		SuggestedPersonas: suggestions,
		CostProfile:       profile.CostProfile,
		GeneratedAt:       time.Now(),
	}, nil
}

// suggestElementForStep returns the best element for a circuit step based on
// the step's dimensional requirements and the model's measured profile.
// Falls back to the profile's overall ElementMatch when stepDims is nil or
// has no entry for the step.
func suggestElementForStep(step string, profile ModelProfile, stepDims StepDimensionMap) element.Element {
	dims, ok := stepDims[step]
	if !ok || len(dims) == 0 {
		return profile.ElementMatch
	}

	stepProfile := ModelProfile{Dimensions: make(map[Dimension]float64)}
	for _, dim := range dims {
		stepProfile.Dimensions[dim] = profile.Dimensions[dim]
	}

	return ElementMatch(stepProfile)
}

// ProviderHints returns a map of circuit step names to preferred provider
// names, derived from the persona's element affinity and known provider-element
// mappings. Consumers (e.g., ProviderRouter) use this for empirical routing.
func (ps *PersonaSheet) ProviderHints(providerElements map[string]element.Element) map[string]string {
	elementProviders := make(map[element.Element]string)
	for provider, element := range providerElements {
		elementProviders[element] = provider
	}

	hints := make(map[string]string)
	for step, personaName := range ps.SuggestedPersonas {
		p, ok := persona.ByName(personaName)
		if !ok {
			continue
		}
		if provider, ok := elementProviders[p.Identity.Element]; ok {
			hints[step] = provider
		}
	}
	return hints
}

// MarshalYAML returns the PersonaSheet as human-readable YAML bytes.
func (ps *PersonaSheet) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(ps)
}
