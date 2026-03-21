package builders

import "github.com/dpopsuev/origami/calibrate"

// ScoreCardBuilder constructs a calibrate.ScoreCard incrementally for tests.
// This is a test-oriented builder that provides a simpler API than the
// production ScoreCardBuilder in calibrate/.
type ScoreCardBuilder struct {
	name    string
	metrics []calibrate.MetricDef
	scorer  string
}

// NewScoreCard creates a new ScoreCardBuilder.
func NewScoreCard() *ScoreCardBuilder {
	return &ScoreCardBuilder{}
}

// AddMetric adds a metric with the given ID, name, and threshold.
// Direction defaults to HigherIsBetter, weight defaults to 1.0.
func (b *ScoreCardBuilder) AddMetric(id, name string, threshold float64) *ScoreCardBuilder {
	b.metrics = append(b.metrics, calibrate.MetricDef{
		ID:        id,
		Name:      name,
		Threshold: threshold,
		Direction: calibrate.HigherIsBetter,
		Weight:    1.0,
		Tier:      calibrate.TierOutcome,
		Scorer:    b.scorer,
	})
	return b
}

// WithScorer sets the default scorer name for subsequently added metrics.
func (b *ScoreCardBuilder) WithScorer(scorer string) *ScoreCardBuilder {
	b.scorer = scorer
	return b
}

// WithName sets the scorecard name.
func (b *ScoreCardBuilder) WithName(name string) *ScoreCardBuilder {
	b.name = name
	return b
}

// Build returns the constructed ScoreCard.
func (b *ScoreCardBuilder) Build() calibrate.ScoreCard {
	name := b.name
	if name == "" {
		name = "test-scorecard"
	}
	return calibrate.ScoreCard{
		Name:       name,
		Version:    1,
		MetricDefs: b.metrics,
	}
}
