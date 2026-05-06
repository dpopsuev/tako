package cerebrum

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/capability"
	tangle "github.com/dpopsuev/tangle"
)

func TestPhaseB_FullPipeline(t *testing.T) {
	cfg := reactivity.DefaultConfig

	readCap := capability.Capability{
		Name:        "read_file",
		Description: "read a file",
		Schema:      json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		Mode:        capability.ReadAction,
		Source:      capability.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (capability.Result, error) {
			return capability.TextResult("file contents"), nil
		},
	}

	reflexStore := NewReflexStore([]capability.Capability{readCap})
	reflexStore.AddReflex(
		map[string]float64{"file.exists": 1},
		[]string{"read_file"},
	)

	alignment := TieredAlignment{}
	convergenceAssert := ConvergenceAssert{
		Inner: reactivity.DefaultAssert,
	}
	rewardLoop := NewRewardLoop(&cfg, 0.1)

	completer := &stubCompleter{
		toolCalls: []tangle.ToolCall{{
			ID:    "tc-1",
			Name:  "intent",
			Input: json.RawMessage(`{"taxonomy":"intent.goal.read","content":"read the file","dimensions":["file.exists"]}`),
		}},
	}
	reactor := reactivity.NewReactor(reactivity.WithNavigator(
		reactivity.NewTreeNavigator(&cfg),
	))
	motor := &stubBus{}

	cb := New(reactor, completer,
		WithMotor(motor),
		WithCapabilities([]capability.Capability{readCap}),
		WithConfig(&cfg),
		WithAssert(convergenceAssert),
		WithAlignmentChecker(alignment),
		WithMaxTurns(5),
	)

	catalyst := reactivity.Catalyst{
		Need:    "read the config file",
		Desired: map[string]any{"file.exists": true},
	}

	if err := cb.Think(context.Background(), catalyst); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := cb.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed")
	}

	t.Run("reflex_store_matches_residual", func(t *testing.T) {
		residual := map[string]float64{"file.exists": 1}
		caps, overlap := reflexStore.Match(residual)
		if overlap != 1.0 || len(caps) != 1 {
			t.Fatalf("reflex should match residual exactly, got overlap=%v caps=%d", overlap, len(caps))
		}
		gear := selectGear(overlap)
		if gear != GearReflex {
			t.Fatalf("100%% overlap should be reflex gear, got %s", gear)
		}
	})

	t.Run("alignment_checks_atoms", func(t *testing.T) {
		atom := reactivity.Atom{
			Dimensions: []string{"file.exists"},
			CreatedAt:  time.Now(),
		}
		ar := alignment.Check(atom, m)
		if len(ar.DriftFlags) > 0 {
			t.Logf("drift flags (expected for sealed molecule): %v", ar.DriftFlags)
		}
	})

	t.Run("convergence_on_sealed", func(t *testing.T) {
		diff := m.SynthesisDiff(reactivity.ThinkTriad)
		t.Logf("synthesis diff (think triad): %.3f", diff)
	})

	t.Run("reward_loop_adjusts_thresholds", func(t *testing.T) {
		summary := computeSessionSummary(m.ID, nil, m)
		before := cfg.DistanceClose
		rewardLoop.Process(summary.OAE)
		after := cfg.DistanceClose
		t.Logf("OAE=%.3f, DistanceClose: %.6f → %.6f", summary.OAE, before, after)
	})

	t.Run("gear_intuition_for_partial_match", func(t *testing.T) {
		partialStore := NewReflexStore([]capability.Capability{readCap})
		partialStore.AddReflex(
			map[string]float64{"file.exists": 1, "file.readable": 1, "file.writable": 1},
			[]string{"read_file"},
		)
		residual := map[string]float64{
			"file.exists":   1,
			"file.readable": 1,
			"file.writable": 0,
		}
		caps, overlap := partialStore.Match(residual)
		gear := selectGear(overlap)
		if gear != GearFamiliar && gear != GearIntuition {
			t.Fatalf("partial match should be familiar/intuition, got %s (overlap=%.2f, caps=%d)", gear, overlap, len(caps))
		}
		if overlap > 0 && overlap < 1.0 {
			suggestion := suggestionAtom(caps, overlap, 1)
			if suggestion.Source != reactivity.Recollected {
				t.Fatal("suggestion should be recollected")
			}
			t.Logf("suggestion generated: %s (overlap=%.0f%%)", suggestion.Content, overlap*100)
		}
	})
}

func TestPhaseB_InstrumentationGearIntuition(t *testing.T) {
	turns := []TurnRecord{
		{Gear: GearNovel},
		{Gear: GearFamiliar},
		{Gear: GearIntuition},
		{Gear: GearReflex},
		{Gear: GearIntuition},
	}

	m := reactivity.NewMolecule("test-gear-dist")
	summary := computeSessionSummary("test-gear-dist", turns, m)

	if summary.GearNovelPct != 20 {
		t.Errorf("novel pct = %.1f, want 20", summary.GearNovelPct)
	}
	if summary.GearFamiliarPct != 20 {
		t.Errorf("familiar pct = %.1f, want 20", summary.GearFamiliarPct)
	}
	if summary.GearIntuitionPct != 40 {
		t.Errorf("intuition pct = %.1f, want 40", summary.GearIntuitionPct)
	}
	if summary.GearReflexPct != 20 {
		t.Errorf("reflex pct = %.1f, want 20", summary.GearReflexPct)
	}
}
