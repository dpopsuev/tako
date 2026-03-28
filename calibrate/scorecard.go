package calibrate

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ThresholdStrategy describes how a metric's threshold was derived.
type ThresholdStrategy string

const (
	StrategyFixed            ThresholdStrategy = "fixed"
	StrategyBaselineRelative ThresholdStrategy = "baseline_relative"
	StrategyPercentile       ThresholdStrategy = "percentile"
	StrategyROIBased         ThresholdStrategy = "roi_based"
)

// MetricDef is a declarative metric definition: ID, name, threshold,
// evaluation strategy, weight, cost tier, and direction.
type MetricDef struct {
	ID        string            `json:"id" yaml:"id"`
	Name      string            `json:"name" yaml:"name"`
	Tier      CostTier          `json:"tier" yaml:"tier"`
	Direction EvalDirection     `json:"direction" yaml:"direction"`
	Threshold float64           `json:"threshold" yaml:"threshold"`
	Weight    float64           `json:"weight" yaml:"weight"`
	Strategy  ThresholdStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Rationale string            `json:"rationale,omitempty" yaml:"rationale,omitempty"`
	DryCapped bool              `json:"dry_capped,omitempty" yaml:"dry_capped,omitempty"`
	Scorer    string            `json:"scorer,omitempty" yaml:"scorer,omitempty"`
	Params    map[string]any    `json:"params,omitempty" yaml:"params,omitempty"`
}

// Evaluate checks whether a metric value passes given this definition's
// direction and threshold.
func (d *MetricDef) Evaluate(value float64) bool {
	switch d.Direction {
	case LowerIsBetter:
		return value <= d.Threshold
	case RangeCheck:
		return value >= 0 && value <= d.Threshold
	default:
		return value >= d.Threshold
	}
}

// ToMetric converts a MetricDef and a computed value into a runtime Metric.
func (d *MetricDef) ToMetric(value float64, detail string) Metric {
	return Metric{
		ID:        d.ID,
		Name:      d.Name,
		Value:     value,
		Threshold: d.Threshold,
		Pass:      d.Evaluate(value),
		Detail:    detail,
		DryCapped: d.DryCapped,
		Tier:      d.Tier,
		Direction: d.Direction,
	}
}

// CostModel captures the economic context for ROI-based threshold decisions.
type CostModel struct {
	CasesPerBatch              int     `json:"cases_per_batch" yaml:"cases_per_batch"`
	CostPerBatchUSD            float64 `json:"cost_per_batch_usd" yaml:"cost_per_batch_usd"`
	LaborSavedPerBatchPersonDays float64 `json:"labor_saved_per_batch_person_days" yaml:"labor_saved_per_batch_person_days"`
	PersonDayCostUSD           float64 `json:"person_day_cost_usd" yaml:"person_day_cost_usd"`
}

// ROI returns (savings - cost) / cost. Returns 0 if cost is zero.
func (cm CostModel) ROI() float64 {
	savings := cm.LaborSavedPerBatchPersonDays * cm.PersonDayCostUSD
	cost := cm.CostPerBatchUSD
	if cost == 0 {
		return 0
	}
	return (savings - cost) / cost
}

// AggregateConfig defines how to compute a composite aggregate metric
// from individual metric scores.
type AggregateConfig struct {
	ID        string  `json:"id" yaml:"id"`
	Name      string  `json:"name" yaml:"name"`
	Formula   string  `json:"formula" yaml:"formula"`
	Threshold float64 `json:"threshold" yaml:"threshold"`
	Include   []string `json:"include" yaml:"include"`
}

// ScoreCard is a named collection of MetricDefs with an aggregate formula.
// It is the "test suite" for a domain's calibration quality.
type ScoreCard struct {
	Name        string          `json:"scorecard" yaml:"scorecard"`
	Description string          `json:"description" yaml:"description"`
	Version     int             `json:"version" yaml:"version"`
	CostModel   *CostModel      `json:"cost_model,omitempty" yaml:"cost_model,omitempty"`
	MetricDefs  []MetricDef     `json:"metrics" yaml:"metrics"`
	Aggregate   *AggregateConfig `json:"aggregate,omitempty" yaml:"aggregate,omitempty"`
}

// FindDef looks up a MetricDef by ID. Returns nil if not found.
func (sc *ScoreCard) FindDef(id string) *MetricDef {
	for i := range sc.MetricDefs {
		if sc.MetricDefs[i].ID == id {
			return &sc.MetricDefs[i]
		}
	}
	return nil
}

// ValidateScorers checks that every MetricDef with a non-empty Scorer field
// references a scorer that exists in the given registry. Returns an error
// listing ALL missing scorer names (not just the first).
func (sc *ScoreCard) ValidateScorers(reg ScorerRegistry) error {
	var missing []string
	for i := range sc.MetricDefs {
		if sc.MetricDefs[i].Scorer == "" {
			continue
		}
		if _, err := reg.Get(sc.MetricDefs[i].Scorer); err != nil {
			missing = append(missing, sc.MetricDefs[i].Scorer)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("scorecard %q references unknown scorers: %v", sc.Name, missing)
	}
	return nil
}

// Evaluate takes a map of metric ID → computed value (with optional detail)
// and produces a MetricSet with direction-aware pass/fail for each defined metric.
func (sc *ScoreCard) Evaluate(values map[string]float64, details map[string]string) MetricSet {
	metrics := make([]Metric, 0, len(sc.MetricDefs))
	for i := range sc.MetricDefs {
		val, ok := values[sc.MetricDefs[i].ID]
		if !ok {
			continue
		}
		detail := ""
		if details != nil {
			detail = details[sc.MetricDefs[i].ID]
		}
		metrics = append(metrics, sc.MetricDefs[i].ToMetric(val, detail))
	}
	return MetricSet{Metrics: metrics}
}

// ComputeAggregate calculates the composite aggregate metric from a MetricSet
// using the scorecard's AggregateConfig. Returns a Metric for the aggregate.
// Only metrics listed in AggregateConfig.Include are considered.
// Currently supports "weighted_average" formula.
func (sc *ScoreCard) ComputeAggregate(ms MetricSet) (Metric, error) {
	if sc.Aggregate == nil {
		return Metric{}, fmt.Errorf("no aggregate config defined")
	}
	ac := sc.Aggregate

	includeSet := make(map[string]bool, len(ac.Include))
	for _, id := range ac.Include {
		includeSet[id] = true
	}

	byID := ms.ByID()
	weightedSum := 0.0
	totalWeight := 0.0

	for i := range sc.MetricDefs {
		if !includeSet[sc.MetricDefs[i].ID] {
			continue
		}
		m, ok := byID[sc.MetricDefs[i].ID]
		if !ok {
			continue
		}
		weightedSum += m.Value * sc.MetricDefs[i].Weight
		totalWeight += sc.MetricDefs[i].Weight
	}

	var aggValue float64
	if totalWeight > 0 {
		aggValue = weightedSum / totalWeight
	}

	aggMetric := Metric{
		ID:        ac.ID,
		Name:      ac.Name,
		Value:     aggValue,
		Threshold: ac.Threshold,
		Pass:      aggValue >= ac.Threshold,
		Detail:    fmt.Sprintf("weighted_average of %d metrics (total_weight=%.2f)", len(ac.Include), totalWeight),
		Tier:      TierMeta,
		Direction: HigherIsBetter,
	}

	return aggMetric, nil
}

// Report evaluates metrics, computes the aggregate (if configured),
// and returns a CalibrationReport ready for formatting.
func (sc *ScoreCard) Report(scenario, transformer string, runs int, values map[string]float64, details map[string]string) (*CalibrationReport, error) {
	ms := sc.Evaluate(values, details)

	if sc.Aggregate != nil {
		agg, err := sc.ComputeAggregate(ms)
		if err != nil {
			return nil, fmt.Errorf("aggregate: %w", err)
		}
		ms.Metrics = append(ms.Metrics, agg)
	}

	return &CalibrationReport{
		Scenario:    scenario,
		Transformer: transformer,
		Runs:        runs,
		Metrics:     ms,
	}, nil
}

// ParseScoreCard unmarshals a YAML scorecard definition from raw bytes.
func ParseScoreCard(data []byte) (*ScoreCard, error) {
	var sc ScoreCard
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("parse scorecard: %w", err)
	}
	return &sc, nil
}

// LoadScoreCard reads a YAML scorecard definition from disk.
func LoadScoreCard(path string) (*ScoreCard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scorecard %s: %w", path, err)
	}
	ext := filepath.Ext(path)
	switch ext {
	case ".yaml", ".yml":
		return ParseScoreCard(data)
	default:
		return nil, fmt.Errorf("unsupported scorecard format: %s (use .yaml or .yml)", ext)
	}
}

// --- Default Metrics (universal, domain-agnostic) ---

// DefaultMetrics returns 7 universal metrics every circuit needs.
// Three derived from TokiMeter (token_usage, token_cost_usd, latency_seconds),
// two from walk (path_efficiency, loop_ratio),
// two from multi-run (confidence_calibration, run_variance).
func DefaultMetrics() []MetricDef {
	return []MetricDef{
		{
			ID: "token_usage", Name: "Token Usage",
			Tier: TierEfficiency, Direction: LowerIsBetter,
			Threshold: 200000, Weight: 0,
			Strategy: StrategyFixed, Rationale: "Health check — total tokens per batch",
		},
		{
			ID: "token_cost_usd", Name: "Token Cost (USD)",
			Tier: TierEfficiency, Direction: LowerIsBetter,
			Threshold: 5.0, Weight: 0,
			Strategy: StrategyFixed, Rationale: "Health check — total cost per batch",
		},
		{
			ID: "latency_seconds", Name: "Wall-Clock Latency",
			Tier: TierEfficiency, Direction: LowerIsBetter,
			Threshold: 600, Weight: 0,
			Strategy: StrategyFixed, Rationale: "Health check — batch should complete in 10 min",
		},
		{
			ID: "path_efficiency", Name: "Path Efficiency",
			Tier: TierEfficiency, Direction: HigherIsBetter,
			Threshold: 0.60, Weight: 0,
			Strategy: StrategyFixed, Rationale: "Ratio of shortest-path steps to actual steps walked",
		},
		{
			ID: "loop_ratio", Name: "Loop Ratio",
			Tier: TierEfficiency, Direction: LowerIsBetter,
			Threshold: 0.20, Weight: 0,
			Strategy: StrategyFixed, Rationale: "Fraction of steps that revisit a previously visited node",
		},
		{
			ID: "confidence_calibration", Name: "Confidence Calibration",
			Tier: TierMeta, Direction: HigherIsBetter,
			Threshold: 0.50, Weight: 0.05,
			Strategy: StrategyFixed, Rationale: "Correlation between reported confidence and actual correctness",
		},
		{
			ID: "run_variance", Name: "Run Variance",
			Tier: TierMeta, Direction: LowerIsBetter,
			Threshold: 0.15, Weight: 0.05,
			Strategy: StrategyFixed, Rationale: "Stddev of primary metric across runs — lower is more deterministic",
		},
		{
			ID: "evidence_snr", Name: "Evidence SNR",
			Tier: TierEfficiency, Direction: HigherIsBetter,
			Threshold: 0.50, Weight: 0,
			Strategy: StrategyFixed, Rationale: "Health check — signal preservation ratio across nodes",
		},
		{
			ID: "walker_mismatch", Name: "Walker Mismatch",
			Tier: TierEfficiency, Direction: LowerIsBetter,
			Threshold: 0.30, Weight: 0,
			Strategy: StrategyFixed, Rationale: "Health check — walker-node impedance mismatch (lower = better fit)",
		},
	}
}

// DefaultScoreCard returns a ScoreCardBuilder pre-loaded with DefaultMetrics().
// Consumer pattern: DefaultScoreCard().WithMetrics(domain...).Build()
func DefaultScoreCard() *ScoreCardBuilder {
	return NewScoreCardBuilder("default").
		WithDescription("Universal circuit metrics").
		WithMetrics(DefaultMetrics()...)
}

// --- ScoreCard Builder (slog pattern) ---

// ScoreCardBuilder constructs a ScoreCard incrementally.
type ScoreCardBuilder struct {
	sc ScoreCard
}

// NewScoreCardBuilder creates a builder with a given name.
func NewScoreCardBuilder(name string) *ScoreCardBuilder {
	return &ScoreCardBuilder{sc: ScoreCard{Name: name, Version: 1}}
}

// WithDescription sets the description.
func (b *ScoreCardBuilder) WithDescription(desc string) *ScoreCardBuilder {
	b.sc.Description = desc
	return b
}

// WithCostModel sets the cost model.
func (b *ScoreCardBuilder) WithCostModel(cm CostModel) *ScoreCardBuilder {
	b.sc.CostModel = &cm
	return b
}

// WithMetrics appends metric definitions.
func (b *ScoreCardBuilder) WithMetrics(defs ...MetricDef) *ScoreCardBuilder {
	b.sc.MetricDefs = append(b.sc.MetricDefs, defs...)
	return b
}

// WithAggregate sets the aggregate config.
func (b *ScoreCardBuilder) WithAggregate(ac *AggregateConfig) *ScoreCardBuilder {
	b.sc.Aggregate = ac
	return b
}

// Build returns the constructed ScoreCard.
func (b *ScoreCardBuilder) Build() ScoreCard {
	return b.sc
}
