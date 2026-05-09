package cerebrum

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical", []float64{1, 0, 0}, []float64{1, 0, 0}, 1.0},
		{"orthogonal", []float64{1, 0, 0}, []float64{0, 1, 0}, 0.0},
		{"empty", nil, nil, 0},
		{"length mismatch", []float64{1}, []float64{1, 2}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CosineSimilarity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectGear(t *testing.T) {
	tests := []struct {
		overlap float64
		want    Gear
	}{
		{1.0, GearReflex},
		{0.95, GearReflex},
		{0.9, GearIntuition},
		{0.7, GearIntuition},
		{0.5, GearFamiliar},
		{0.3, GearFamiliar},
		{0.2, GearNovel},
		{0, GearNovel},
	}

	for _, tt := range tests {
		got := selectGear(tt.overlap)
		if got != tt.want {
			t.Errorf("selectGear(%v) = %v, want %v", tt.overlap, got, tt.want)
		}
	}
}

func TestPipeStoreMatch(t *testing.T) {
	store := NewPipeStore()
	store.Add(Pipe{
		Name:      "greet",
		Embedding: []float64{1, 0, 0},
		Steps:     []PipeStep{{ID: "respond", Call: "greet"}},
	})
	store.Add(Pipe{
		Name:      "code",
		Embedding: []float64{0, 1, 0},
		Steps:     []PipeStep{{ID: "read", Call: "file_read"}},
	})

	t.Run("exact match", func(t *testing.T) {
		pipe, sim := store.Match([]float64{1, 0, 0})
		if pipe == nil || pipe.Name != "greet" {
			t.Fatalf("expected greet, got %v", pipe)
		}
		if sim != 1.0 {
			t.Fatalf("sim = %v, want 1.0", sim)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, sim := store.Match([]float64{0, 0, 1})
		if sim > 0.01 {
			t.Fatalf("expected ~0 sim, got %v", sim)
		}
	})

	t.Run("empty embedding", func(t *testing.T) {
		pipe, sim := store.Match(nil)
		if pipe != nil || sim != 0 {
			t.Fatalf("expected nil/0")
		}
	})
}

func TestPipeScoring(t *testing.T) {
	pipe := Pipe{Replays: 0, Usage: 0}
	if s := pipe.Score(); s != 0.5 {
		t.Fatalf("initial score = %v, want 0.5", s)
	}

	pipe.Replays = 9
	pipe.Usage = 10
	if s := pipe.Score(); s < 0.8 {
		t.Fatalf("high replay score = %v, want > 0.8", s)
	}
}

func TestPipeStorePrune(t *testing.T) {
	store := NewPipeStore()
	store.Add(Pipe{Name: "good", Replays: 10, Usage: 10})
	store.Add(Pipe{Name: "bad", Replays: 0, Usage: 100})

	pruned := store.Prune(0.05)
	if pruned != 1 {
		t.Fatalf("pruned = %d, want 1", pruned)
	}
	if store.Len() != 1 {
		t.Fatalf("len = %d, want 1", store.Len())
	}
}

func TestReplayPipe_FullSuccess(t *testing.T) {
	pipe := &Pipe{
		Name:  "test",
		Steps: []PipeStep{{ID: "greet", Call: "greet", Expected: HashResult([]byte("hello"))}},
	}

	caps := map[string]organ.Func{
		"greet": {
			Name: "greet",
			Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
				return organ.TextResult("hello"), nil
			},
		},
	}

	result, err := ReplayPipe(context.Background(), pipe, caps)
	if err != nil {
		t.Fatal(err)
	}
	if result.StepsReflex != 1 {
		t.Fatalf("steps reflex = %d, want 1", result.StepsReflex)
	}
	if result.EscalatedAt != -1 {
		t.Fatalf("escalated at %d, want -1", result.EscalatedAt)
	}
	if pipe.Replays != 1 {
		t.Fatalf("replays = %d, want 1", pipe.Replays)
	}
}

func TestReplayPipe_Escalation(t *testing.T) {
	pipe := &Pipe{
		Name: "test",
		Steps: []PipeStep{{
			ID:         "read",
			Call:       "file_read",
			Expected:   HashResult([]byte("expected content")),
			Confidence: 0.1,
		}},
	}

	caps := map[string]organ.Func{
		"file_read": {
			Name: "file_read",
			Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
				return organ.TextResult("different content"), nil
			},
		},
	}

	result, err := ReplayPipe(context.Background(), pipe, caps)
	if err != nil {
		t.Fatal(err)
	}
	if result.EscalatedAt != 0 {
		t.Fatalf("escalated at %d, want 0", result.EscalatedAt)
	}
	if result.EscalatedGear != GearFamiliar {
		t.Fatalf("gear = %s, want familiar", result.EscalatedGear)
	}
}

func TestPipeExecutor_DependencyResolution(t *testing.T) {
	pipe := Pipe{
		Name: "test",
		Steps: []PipeStep{
			{ID: "a", Call: "step_a"},
			{ID: "b", Call: "step_b", DependsOn: []string{"a"}},
			{ID: "c", Call: "step_c", DependsOn: []string{"a", "b"}},
		},
	}

	exec := NewPipeExecutor()
	runID, pr := exec.StartWithPipe(pipe)

	step, _, _ := exec.NextStepFromPipe(runID, pr.steps)
	if step == nil || step.ID != "a" {
		t.Fatalf("first step should be a, got %v", step)
	}

	exec.SubmitAndUnlock(runID, "a", "done", "", pr.steps)

	step, _, _ = exec.NextStepFromPipe(runID, pr.steps)
	if step == nil || step.ID != "b" {
		t.Fatalf("after a, b should be ready, got %v", step)
	}

	exec.SubmitAndUnlock(runID, "b", "done", "", pr.steps)

	step, _, _ = exec.NextStepFromPipe(runID, pr.steps)
	if step == nil || step.ID != "c" {
		t.Fatalf("after a+b, c should be ready, got %v", step)
	}
}

func TestPipeExecutor_FailureCascade(t *testing.T) {
	pipe := Pipe{
		Name: "test",
		Steps: []PipeStep{
			{ID: "a", Call: "step_a"},
			{ID: "b", Call: "step_b", DependsOn: []string{"a"}},
		},
	}

	exec := NewPipeExecutor()
	runID, pr := exec.StartWithPipe(pipe)

	exec.NextStepFromPipe(runID, pr.steps)
	state, _ := exec.SubmitAndUnlock(runID, "a", nil, "boom", pr.steps)

	if state.Steps["b"].Status != "skipped" {
		t.Fatalf("b should be skipped, got %s", state.Steps["b"].Status)
	}
	if state.Status != "failed" {
		t.Fatalf("run should be failed, got %s", state.Status)
	}
}

func TestFireReflex(t *testing.T) {
	var called atomic.Int32
	caps := []organ.Func{
		{
			Name: "test_cap",
			Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
				called.Add(1)
				return organ.TextResult("ok"), nil
			},
		},
		{Name: "nil_execute"},
		{
			Name: "error_cap",
			Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
				return organ.Result{}, errors.New("boom")
			},
		},
	}

	fireReflex(context.Background(), caps, 0.9)
	if called.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", called.Load())
	}
}

func TestSuggestionAtom(t *testing.T) {
	pipe := &Pipe{Name: "test-pipe"}
	atom := suggestionAtom(pipe, 0.85, 3)

	if atom.Type.String() != "knowledge" {
		t.Fatalf("type = %s, want knowledge", atom.Type.String())
	}
	if atom.Source != reactivity.Recollected {
		t.Fatal("source should be recollected")
	}
}

func TestStubEmbedder(t *testing.T) {
	e := StubEmbedder{Dims: 8}
	v1, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(v1) != 8 {
		t.Fatalf("dims = %d, want 8", len(v1))
	}

	v2, _ := e.Embed(context.Background(), "hello")
	if CosineSimilarity(v1, v2) < 0.999 {
		t.Fatal("same input should produce same embedding")
	}

	v3, _ := e.Embed(context.Background(), "goodbye")
	sim := CosineSimilarity(v1, v3)
	if sim > 0.999 {
		t.Fatal("different inputs should produce different embeddings")
	}
}

func TestReflexStoreInterface(t *testing.T) {
	var _ ReflexStore = (*PipeStore)(nil)
}
