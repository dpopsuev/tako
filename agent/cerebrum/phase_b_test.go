package cerebrum

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func TestPhaseB_FullPipeline(t *testing.T) {
	cfg := reactivity.DefaultConfig

	readCap := organ.Func{
		Name:        "read_file",
		Description: "read a file",
		Schema:      json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		Mode:        organ.ReadAction,
		Source:      organ.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
			return organ.TextResult("file contents"), nil
		},
	}

	embedder := StubEmbedder{Dims: 8}
	pipeStore := NewPipeStore()

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
		WithCapabilities([]organ.Func{readCap}),
		WithConfig(&cfg),
		WithAssert(convergenceAssert),
		WithAlignmentChecker(alignment),
		WithMaxTurns(5),
		WithEmbedder(embedder),
		WithReflexStore(pipeStore),
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

	t.Run("pipe_store_embedding_match", func(t *testing.T) {
		embedding, _ := embedder.Embed(context.Background(), "read the config file")
		pipeStore.Add(Pipe{
			Name:      "test-read",
			Embedding: embedding,
			Steps:     []PipeStep{{ID: "read", Call: "read_file"}},
		})
		pipe, sim := pipeStore.Match(embedding)
		if sim < 0.999 || pipe == nil {
			t.Fatalf("exact embedding should match, got sim=%v", sim)
		}
		gear := selectGear(sim)
		if gear != GearReflex {
			t.Fatalf("high sim should be reflex, got %s", gear)
		}
	})

	t.Run("alignment_checks_atoms", func(t *testing.T) {
		atom := reactivity.Atom{
			Dimensions: []string{"file.exists"},
			CreatedAt:  time.Now(),
		}
		ar := alignment.Check(atom, m)
		if len(ar.DriftFlags) > 0 {
			t.Logf("drift flags: %v", ar.DriftFlags)
		}
	})

	t.Run("reward_loop_adjusts_thresholds", func(t *testing.T) {
		summary := computeSessionSummary(m.ID, nil, m)
		before := cfg.DistanceClose
		rewardLoop.Process(summary.OAE)
		after := cfg.DistanceClose
		t.Logf("OAE=%.3f, DistanceClose: %.6f → %.6f", summary.OAE, before, after)
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
