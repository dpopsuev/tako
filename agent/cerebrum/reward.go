package cerebrum

import (
	"log/slog"
	"math"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type RewardLoop struct {
	expectedOAE  float64
	learningRate float64
	config       *reactivity.Config
}

func NewRewardLoop(cfg *reactivity.Config, learningRate float64) *RewardLoop {
	return &RewardLoop{
		expectedOAE:  0.5,
		learningRate: learningRate,
		config:       cfg,
	}
}

func (r *RewardLoop) Process(oae float64) {
	if r.learningRate == 0 {
		return
	}

	rpe := oae - r.expectedOAE

	r.expectedOAE = r.expectedOAE + r.learningRate*rpe

	before := *r.config
	r.adjust(rpe)

	if r.config.DistanceClose == 0.1 || r.config.DistanceClose == 0.5 {
		slog.Warn("reward.threshold_clamped",
			slog.String("param", "DistanceClose"),
			slog.Float64("value", r.config.DistanceClose))
	}
	if r.config.DistanceMid == 0.2 || r.config.DistanceMid == 0.7 {
		slog.Warn("reward.threshold_clamped",
			slog.String("param", "DistanceMid"),
			slog.Float64("value", r.config.DistanceMid))
	}
	if oae < 0.2 && r.expectedOAE > 0.4 {
		slog.Warn("reward.oae_collapse",
			slog.Float64("oae", oae),
			slog.Float64("expected", r.expectedOAE))
	}

	slog.Info("reward.process",
		slog.Float64("oae", oae),
		slog.Float64("rpe", rpe),
		slog.Float64("expected_oae", r.expectedOAE),
		slog.Float64("learning_rate", r.learningRate),
		slog.Float64("distance_close_before", before.DistanceClose),
		slog.Float64("distance_close_after", r.config.DistanceClose),
		slog.Float64("distance_mid_before", before.DistanceMid),
		slog.Float64("distance_mid_after", r.config.DistanceMid))
}

func (r *RewardLoop) adjust(rpe float64) {
	delta := r.learningRate * rpe
	r.config.DistanceClose = clamp(r.config.DistanceClose-delta*0.1, 0.1, 0.5)
	r.config.DistanceMid = clamp(r.config.DistanceMid-delta*0.1, 0.2, 0.7)
	r.config.RecollectionMin = clamp(r.config.RecollectionMin-delta*0.1, 0.1, 0.5)
}

func (r *RewardLoop) ExpectedOAE() float64   { return r.expectedOAE }
func (r *RewardLoop) Config() *reactivity.Config { return r.config }

func clamp(v, min, max float64) float64 {
	return math.Max(min, math.Min(max, v))
}
