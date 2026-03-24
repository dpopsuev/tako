// Package calibrate provides generic calibration primitives for measuring
// domain accuracy across scenario-vs-ground-truth runs. Consumers (Asterisk,
// Achilles, etc.) supply domain-specific scoring; this package provides the
// shared types, aggregation, and report formatting.
package calibrate

import "github.com/dpopsuev/origami/agentport"

// CostTier classifies a metric by what it measures. Tiers are ordered by
// importance: outcome metrics dominate; efficiency metrics are health checks.
type CostTier string

const (
	TierOutcome       CostTier = "outcome"
	TierInvestigation CostTier = "investigation"
	TierDetection     CostTier = "detection"
	TierEfficiency    CostTier = "efficiency"
	TierMeta          CostTier = "meta"
)

// EvalDirection tells how to evaluate a metric value against its threshold.
type EvalDirection string

const (
	HigherIsBetter EvalDirection = "higher_is_better"
	LowerIsBetter  EvalDirection = "lower_is_better"
	RangeCheck     EvalDirection = "range"
)

// Metric is a single calibration metric with value, threshold, and pass/fail.
type Metric struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Value     float64       `json:"value"`
	Threshold float64       `json:"threshold"`
	Pass      bool          `json:"pass"`
	Detail    string        `json:"detail"`
	DryCapped bool          `json:"dry_capped,omitempty"`
	Tier      CostTier      `json:"tier,omitempty"`
	Direction EvalDirection `json:"direction,omitempty"`
}

// MetricSet holds all computed metrics for a calibration run as a flat list.
// View methods (ByTier, ByID) provide grouping without baking structure into
// the container.
type MetricSet struct {
	Metrics []Metric `json:"metrics"`
}

// AllMetrics returns all metrics as a flat list.
func (ms *MetricSet) AllMetrics() []Metric {
	return ms.Metrics
}

// PassCount returns (passed, total), excluding dry-capped metrics from both counts.
func (ms *MetricSet) PassCount() (int, int) {
	passed, total := 0, 0
	for _, m := range ms.Metrics {
		if m.DryCapped {
			continue
		}
		total++
		if m.Pass {
			passed++
		}
	}
	return passed, total
}

// ByTier groups metrics by their CostTier.
func (ms *MetricSet) ByTier() map[CostTier][]Metric {
	groups := make(map[CostTier][]Metric)
	for _, m := range ms.Metrics {
		groups[m.Tier] = append(groups[m.Tier], m)
	}
	return groups
}

// ByID returns a lookup map from metric ID to Metric.
func (ms *MetricSet) ByID() map[string]Metric {
	lookup := make(map[string]Metric, len(ms.Metrics))
	for _, m := range ms.Metrics {
		lookup[m.ID] = m
	}
	return lookup
}

// CalibrationReport is the generic output of a calibration run.
// Consumers embed this struct and add domain-specific fields
// (e.g. CaseResults, DatasetHealth).
type CalibrationReport struct {
	Scenario    string                 `json:"scenario"`
	Transformer string                 `json:"transformer"`
	Resolution  string                 `json:"resolution,omitempty"`
	Plan        string                 `json:"plan,omitempty"`
	Runs        int                    `json:"runs"`
	Metrics     MetricSet              `json:"metrics"`
	RunMetrics  []MetricSet            `json:"run_metrics,omitempty"`
	Tokens      *agentport.TokenSummary `json:"tokens,omitempty"`
}
