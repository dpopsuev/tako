package ergograph

import (
	"fmt"
	"math"
)

type CostTier string

const (
	TierOutcome       CostTier = "outcome"
	TierInvestigation CostTier = "investigation"
	TierDetection     CostTier = "detection"
	TierEfficiency    CostTier = "efficiency"
	TierMeta          CostTier = "meta"
)

type EvalDirection string

const (
	HigherIsBetter EvalDirection = "higher_is_better"
	LowerIsBetter  EvalDirection = "lower_is_better"
	RangeCheck     EvalDirection = "range"
)

type MetricDef struct {
	ID        string        `json:"id" yaml:"id"`
	Name      string        `json:"name" yaml:"name"`
	Tier      CostTier      `json:"tier" yaml:"tier"`
	Direction EvalDirection `json:"direction" yaml:"direction"`
	Threshold float64       `json:"threshold" yaml:"threshold"`
	Weight    float64       `json:"weight" yaml:"weight"`
	Scorer    string        `json:"scorer,omitempty" yaml:"scorer,omitempty"`
	Params    map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}

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

func (d *MetricDef) ToMetric(value float64, detail string) Metric {
	return Metric{
		ID:        d.ID,
		Name:      d.Name,
		Value:     value,
		Threshold: d.Threshold,
		Pass:      d.Evaluate(value),
		Detail:    detail,
		Tier:      d.Tier,
		Direction: d.Direction,
	}
}

type Metric struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Value     float64       `json:"value"`
	Threshold float64       `json:"threshold"`
	Pass      bool          `json:"pass"`
	Detail    string        `json:"detail"`
	Tier      CostTier      `json:"tier,omitempty"`
	Direction EvalDirection `json:"direction,omitempty"`
}

type MetricSet struct {
	Metrics []Metric `json:"metrics"`
}

func (ms *MetricSet) PassCount() (passed, total int) {
	for _, m := range ms.Metrics {
		total++
		if m.Pass {
			passed++
		}
	}
	return
}

func (ms *MetricSet) ByTier() map[CostTier][]Metric {
	groups := make(map[CostTier][]Metric)
	for _, m := range ms.Metrics {
		groups[m.Tier] = append(groups[m.Tier], m)
	}
	return groups
}

func (ms *MetricSet) ByID() map[string]Metric {
	lookup := make(map[string]Metric, len(ms.Metrics))
	for _, m := range ms.Metrics {
		lookup[m.ID] = m
	}
	return lookup
}

type ScorerFunc func(caseResult, groundTruth any, params map[string]any) (float64, string, error)

type ScorerRegistry map[string]ScorerFunc

func (r ScorerRegistry) Register(name string, fn ScorerFunc) {
	if _, exists := r[name]; exists {
		panic(fmt.Sprintf("duplicate scorer registration: %q", name))
	}
	r[name] = fn
}

func (r ScorerRegistry) Get(name string) (ScorerFunc, error) {
	fn, ok := r[name]
	if !ok {
		return nil, fmt.Errorf("scorer %q not registered", name)
	}
	return fn, nil
}

type PassEvaluator func(*Metric) bool

func DefaultPassEvaluator(m *Metric) bool {
	return m.Value >= m.Threshold
}

func AggregateRunMetrics(runs []MetricSet, eval PassEvaluator) MetricSet {
	if len(runs) == 0 {
		return MetricSet{}
	}
	if len(runs) == 1 {
		return runs[0]
	}
	if eval == nil {
		eval = DefaultPassEvaluator
	}

	allByID := make(map[string][]float64)
	for _, run := range runs {
		for _, m := range run.Metrics {
			allByID[m.ID] = append(allByID[m.ID], m.Value)
		}
	}

	agg := runs[0]
	for i := range agg.Metrics {
		vals := allByID[agg.Metrics[i].ID]
		agg.Metrics[i].Value = Mean(vals)
		sd := Stddev(vals)
		agg.Metrics[i].Detail = fmt.Sprintf("mean of %d runs (σ=%.3f)", len(runs), sd)
		agg.Metrics[i].Pass = eval(&agg.Metrics[i])
	}
	return agg
}

func Mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func Stddev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := Mean(vals)
	sum := 0.0
	for _, v := range vals {
		sum += (v - m) * (v - m)
	}
	return math.Sqrt(sum / float64(len(vals)-1))
}

func SafeDiv(num, denom int) float64 {
	if denom == 0 {
		return 1.0
	}
	return float64(num) / float64(denom)
}

func SafeDivFloat(num, denom float64) float64 {
	if denom == 0 {
		return 1.0
	}
	return num / denom
}
