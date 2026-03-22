package ouroboros

import (
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/element"
)

// ---------------------------------------------------------------------------
// Ouroboros — behavioral dimensions and probe circuit
// ---------------------------------------------------------------------------

// Dimension represents a behavioral axis measured by Ouroboros probes.
// Each maps 1:1 to an ElementTraits field, normalized to 0.0-1.0.
type Dimension string

const (
	DimSpeed                Dimension = "speed"
	DimPersistence          Dimension = "persistence"
	DimConvergenceThreshold Dimension = "convergence_threshold"
	DimShortcutAffinity     Dimension = "shortcut_affinity"
	DimEvidenceDepth        Dimension = "evidence_depth"
	DimFailureMode          Dimension = "failure_mode"
	DimGBWP                 Dimension = "gbwp"
)

// StepDimensionMap maps circuit step names to the behavioral dimensions
// that matter for that step. Consumers (e.g., RCA schematic) provide this
// mapping; Ouroboros itself is domain-agnostic.
type StepDimensionMap map[string][]Dimension

// AllDimensions returns the seven behavioral dimensions in canonical order.
func AllDimensions() []Dimension {
	return []Dimension{
		DimSpeed, DimPersistence, DimConvergenceThreshold,
		DimShortcutAffinity, DimEvidenceDepth, DimFailureMode,
		DimGBWP,
	}
}

// ModelProfile is the empirical output of one Ouroboros cycle for one model.
// Append-only: historical profiles are never overwritten.
type ModelProfile struct {
	Model             circuit.ModelIdentity        `json:"model"`
	BatteryVersion    string                         `json:"battery_version"`
	Timestamp         time.Time                      `json:"timestamp"`
	Dimensions        map[Dimension]float64          `json:"dimensions"`
	ElementMatch      element.Element              `json:"element_match"`
	ElementScores     map[element.Element]float64  `json:"element_scores"`
	SuggestedPersonas []string                       `json:"suggested_personas,omitempty"`
	OffsetPreamble    string                         `json:"offset_preamble,omitempty"`
	CostProfile       circuit.CostProfile          `json:"cost_profile"`
	RawResults        []ProbeResult                  `json:"raw_results"`
}

// DiscoveryConfig controls the recursive discovery loop.
// When ProbeIDs is set, the session iterates over each probe in sequence,
// running MaxIterations per probe. ProbeID is used as fallback when ProbeIDs
// is empty (single-probe backward compatibility).
type DiscoveryConfig struct {
	MaxIterations     int      `json:"max_iterations"`
	ProbeID           string   `json:"probe_id"`
	ProbeIDs          []string `json:"probe_ids,omitempty"`
	TerminateOnRepeat bool     `json:"terminate_on_repeat"`
}

// DefaultConfig returns a sensible starting configuration.
func DefaultConfig() DiscoveryConfig {
	return DiscoveryConfig{
		MaxIterations:     15,
		ProbeID:           "refactor-v1",
		TerminateOnRepeat: true,
	}
}

// ProbeScore holds the scored dimensions from a refactoring probe.
type ProbeScore struct {
	Renames           int     `json:"renames"`
	FunctionSplits    int     `json:"function_splits"`
	CommentsAdded     int     `json:"comments_added"`
	StructuralChanges int     `json:"structural_changes"`
	TotalScore        float64 `json:"total_score"`
}

// ProbeResult captures the raw output and scored result of a single probe.
// Legacy discovery probes populate Score; Ouroboros probes populate DimensionScores.
type ProbeResult struct {
	ProbeID         string               `json:"probe_id"`
	RawOutput       string               `json:"raw_output"`
	Score           ProbeScore           `json:"score"`
	DimensionScores map[Dimension]float64 `json:"dimension_scores,omitempty"`
	Elapsed         time.Duration        `json:"elapsed_ns"`
	TokensUsed      int                  `json:"tokens_used,omitempty"`
	Difficulty      string               `json:"difficulty,omitempty"`
	GoldSignalScore float64              `json:"gold_signal_score,omitempty"`
	TimedOut        bool                 `json:"timed_out,omitempty"`
	HintsUsed       int                  `json:"hints_used,omitempty"`
}

// DiscoveryResult records one iteration of the negation discovery loop.
type DiscoveryResult struct {
	Iteration       int                    `json:"iteration"`
	Model           circuit.ModelIdentity `json:"model"`
	ExclusionPrompt string                 `json:"exclusion_prompt"`
	Probe           ProbeResult            `json:"probe"`
	Timestamp       time.Time              `json:"timestamp"`
}

// RunReport is the complete output of a discovery run. Persisted as
// append-only JSON — each run gets its own file, never overwritten.
type RunReport struct {
	RunID        string                    `json:"run_id"`
	StartTime    time.Time                 `json:"start_time"`
	EndTime      time.Time                 `json:"end_time"`
	Config       DiscoveryConfig           `json:"config"`
	Results      []DiscoveryResult         `json:"results"`
	UniqueModels []circuit.ModelIdentity `json:"unique_models"`
	TermReason   string                    `json:"termination_reason"`
}

// ModelNames returns a sorted list of unique model names from the report.
func (r *RunReport) ModelNames() []string {
	names := make([]string, len(r.UniqueModels))
	for i, m := range r.UniqueModels {
		names[i] = m.String()
	}
	return names
}
