package circuit

// Category: DSL & Build — scorecard overlay definitions.

import "fmt"

// ScorecardDef is the YAML structure for a scorecard definition.
// Schematics provide default metrics. Consumers overlay to tune
// thresholds, weights, and add custom metrics.
type ScorecardDef struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Import      string            `yaml:"import,omitempty"`
	Metrics     []ScorecardMetric `yaml:"metrics"`
	CostModel   *CostModelDef     `yaml:"cost_model,omitempty"`
}

// ScorecardMetric is a single metric in a scorecard.
type ScorecardMetric struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Tier        string         `yaml:"tier,omitempty"`       // outcome, investigation, detection, efficiency, meta
	Scorer      string         `yaml:"scorer,omitempty"`     // scorer function name
	Direction   string         `yaml:"direction,omitempty"`  // higher_is_better, lower_is_better, range
	Threshold   float64        `yaml:"threshold,omitempty"`
	Weight      float64        `yaml:"weight,omitempty"`
	Params      map[string]any `yaml:"params,omitempty"`
	DisplayName string         `yaml:"display_name,omitempty"`
}

// CostModelDef declares the cost model for efficiency metrics.
type CostModelDef struct {
	TokenCost float64 `yaml:"token_cost,omitempty"` // USD per 1M tokens
	TimeCost  float64 `yaml:"time_cost,omitempty"`  // USD per minute
}

// LoadScorecardDef parses YAML bytes into a ScorecardDef.
func LoadScorecardDef(data []byte) (*ScorecardDef, error) {
	var sd ScorecardDef
	if err := yamlUnmarshal(data, &sd); err != nil {
		return nil, fmt.Errorf("parse scorecard: %w", err)
	}
	return &sd, nil
}

// MergeScorecardDefs merges an overlay scorecard onto a base.
// - Metrics with matching ID: overlay values replace base values for non-zero fields
// - New metrics (no matching ID in base): appended
// - CostModel: overlay wins if present
func MergeScorecardDefs(base, overlay *ScorecardDef) (*ScorecardDef, error) {
	if overlay.Import == "" {
		return overlay, nil
	}
	if base == nil {
		return nil, fmt.Errorf("base scorecard required when overlay has import")
	}

	merged := *base
	merged.Import = ""

	if overlay.Name != "" {
		merged.Name = overlay.Name
	}
	if overlay.Description != "" {
		merged.Description = overlay.Description
	}
	if overlay.CostModel != nil {
		merged.CostModel = overlay.CostModel
	}

	// Build index of base metrics
	baseIdx := make(map[string]int, len(merged.Metrics))
	for i, m := range merged.Metrics {
		baseIdx[m.ID] = i
	}

	for _, om := range overlay.Metrics {
		idx, exists := baseIdx[om.ID]
		if !exists {
			merged.Metrics = append(merged.Metrics, om)
			continue
		}
		applyMetricOverlay(&merged.Metrics[idx], &om)
	}

	return &merged, nil
}

func applyMetricOverlay(base, om *ScorecardMetric) {
	if om.Threshold != 0 {
		base.Threshold = om.Threshold
	}
	if om.Weight != 0 {
		base.Weight = om.Weight
	}
	if om.Scorer != "" {
		base.Scorer = om.Scorer
	}
	if len(om.Params) > 0 {
		if base.Params == nil {
			base.Params = make(map[string]any)
		}
		for k, v := range om.Params {
			base.Params[k] = v
		}
	}
	if om.DisplayName != "" {
		base.DisplayName = om.DisplayName
	}
}

// RegisterScorecardVocabulary registers display names from scorecard metrics
// into a vocabulary.
func RegisterScorecardVocabulary(sd *ScorecardDef, v *RichMapVocabulary) {
	for _, m := range sd.Metrics {
		displayName := m.DisplayName
		if displayName == "" {
			displayName = m.Name
		}
		if displayName != "" {
			v.RegisterEntry(m.ID, VocabEntry{Long: displayName})
		}
	}
}
