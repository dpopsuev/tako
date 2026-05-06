package cerebrum

import (
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestRewardLoop_HighOAE_NarrowsThresholds(t *testing.T) {
	cfg := reactivity.DefaultConfig
	rl := NewRewardLoop(&cfg, 0.1)

	before := cfg.DistanceClose
	rl.Process(0.7)

	if cfg.DistanceClose >= before {
		t.Fatalf("high OAE should narrow DistanceClose: before=%v after=%v", before, cfg.DistanceClose)
	}
}

func TestRewardLoop_LowOAE_WidensThresholds(t *testing.T) {
	cfg := reactivity.DefaultConfig
	rl := NewRewardLoop(&cfg, 0.1)

	before := cfg.DistanceClose
	rl.Process(0.15)

	if cfg.DistanceClose <= before {
		t.Fatalf("low OAE should widen DistanceClose: before=%v after=%v", before, cfg.DistanceClose)
	}
}

func TestRewardLoop_ZeroLearningRate_NoChange(t *testing.T) {
	cfg := reactivity.DefaultConfig
	rl := NewRewardLoop(&cfg, 0)

	before := cfg.DistanceClose
	rl.Process(0.9)

	if cfg.DistanceClose != before {
		t.Fatalf("zero learning rate should not change thresholds: before=%v after=%v", before, cfg.DistanceClose)
	}
}

func TestRewardLoop_EMAConverges(t *testing.T) {
	cfg := reactivity.DefaultConfig
	rl := NewRewardLoop(&cfg, 0.1)

	for i := 0; i < 20; i++ {
		rl.Process(0.8)
	}

	if rl.ExpectedOAE() < 0.7 {
		t.Fatalf("EMA should converge toward 0.8, got %v", rl.ExpectedOAE())
	}
}

func TestRewardLoop_ClampsBounds(t *testing.T) {
	cfg := reactivity.DefaultConfig
	rl := NewRewardLoop(&cfg, 0.5)

	for i := 0; i < 50; i++ {
		rl.Process(1.0)
	}

	if cfg.DistanceClose < 0.1 {
		t.Fatalf("DistanceClose clamped below 0.1: %v", cfg.DistanceClose)
	}
	if cfg.DistanceMid < 0.2 {
		t.Fatalf("DistanceMid clamped below 0.2: %v", cfg.DistanceMid)
	}
}
