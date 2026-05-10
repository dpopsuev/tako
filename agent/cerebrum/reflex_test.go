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

func TestSelectConventionality(t *testing.T) {
	tests := []struct {
		overlap float64
		want    Conventionality
	}{
		{1.0, ConventionalityClear},
		{0.95, ConventionalityClear},
		{0.9, ConventionalityComplicated},
		{0.7, ConventionalityComplicated},
		{0.5, ConventionalityComplex},
		{0.3, ConventionalityComplex},
		{0.2, ConventionalityChaotic},
		{0, ConventionalityChaotic},
	}

	for _, tt := range tests {
		got := selectConventionality(tt.overlap)
		if got != tt.want {
			t.Errorf("selectConventionality(%v) = %v, want %v", tt.overlap, got, tt.want)
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
	if result.EscalatedConventionality != ConventionalityComplex {
		t.Fatalf("gear = %s, want familiar", result.EscalatedConventionality)
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

func TestPipeExecutor_OrderPreserved_NoDependencies(t *testing.T) {
	pipe := Pipe{
		Name: "test",
		Steps: []PipeStep{
			{ID: "first", Call: "step_first"},
			{ID: "second", Call: "step_second"},
			{ID: "third", Call: "step_third"},
			{ID: "fourth", Call: "step_fourth"},
			{ID: "fifth", Call: "step_fifth"},
		},
	}

	exec := NewPipeExecutor()
	runID, pr := exec.StartWithPipe(pipe)

	expected := []string{"first", "second", "third", "fourth", "fifth"}
	for i, want := range expected {
		step, _, err := exec.NextStepFromPipe(runID, pr.steps)
		if err != nil {
			t.Fatalf("step %d error: %v", i, err)
		}
		if step == nil {
			t.Fatalf("step %d: nil, want %s", i, want)
		}
		if step.ID != want {
			t.Fatalf("step %d: got %s, want %s — insertion order not preserved", i, step.ID, want)
		}
		exec.SubmitAndUnlock(runID, step.ID, "done", "", pr.steps)
	}

	step, _, _ := exec.NextStepFromPipe(runID, pr.steps)
	if step != nil {
		t.Fatalf("expected nil after all steps, got %s", step.ID)
	}
}

func TestReplayPipe_MultiStep_OrderAndResults(t *testing.T) {
	var order []string
	makeCap := func(name, response string) organ.Func {
		return organ.Func{
			Name: name,
			Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
				order = append(order, name)
				return organ.TextResult(response), nil
			},
		}
	}

	pipe := &Pipe{
		Name: "cook",
		Steps: []PipeStep{
			{ID: "s1", Call: "look"},
			{ID: "s2", Call: "take"},
			{ID: "s3", Call: "cook"},
			{ID: "s4", Call: "eat"},
		},
	}

	caps := map[string]organ.Func{
		"look": makeCap("look", "fridge has eggs"),
		"take": makeCap("take", "took eggs"),
		"cook": makeCap("cook", "cooked eggs"),
		"eat":  makeCap("eat", "ate eggs, no longer hungry"),
	}

	result, err := ReplayPipe(context.Background(), pipe, caps)
	if err != nil {
		t.Fatal(err)
	}

	if result.StepsReflex != 4 {
		t.Errorf("steps reflex = %d, want 4", result.StepsReflex)
	}

	wantOrder := []string{"look", "take", "cook", "eat"}
	if len(order) != len(wantOrder) {
		t.Fatalf("execution order length = %d, want %d", len(order), len(wantOrder))
	}
	for i, want := range wantOrder {
		if order[i] != want {
			t.Errorf("execution order[%d] = %s, want %s", i, order[i], want)
		}
	}

	if len(result.Steps) != 4 {
		t.Fatalf("result.Steps length = %d, want 4", len(result.Steps))
	}
	if result.Steps[0].Call != "look" || result.Steps[0].Output != "fridge has eggs" {
		t.Errorf("step 0: %+v", result.Steps[0])
	}
	if result.Steps[3].Call != "eat" || result.Steps[3].Output != "ate eggs, no longer hungry" {
		t.Errorf("step 3: %+v", result.Steps[3])
	}
	if result.Response != "ate eggs, no longer hungry" {
		t.Errorf("response = %q, want last step output", result.Response)
	}
}

func TestReplayPipe_UnknownCapability_Escalates(t *testing.T) {
	pipe := &Pipe{
		Name:  "test",
		Steps: []PipeStep{{ID: "s1", Call: "missing_tool"}},
	}

	result, err := ReplayPipe(context.Background(), pipe, map[string]organ.Func{})
	if err != nil {
		t.Fatal(err)
	}
	if result.EscalatedAt != 0 {
		t.Errorf("escalated at %d, want 0", result.EscalatedAt)
	}
	if result.EscalatedConventionality != ConventionalityChaotic {
		t.Errorf("conventionality = %s, want chaotic", result.EscalatedConventionality)
	}
}

func TestReplayPipe_ErrorCapability_Escalates(t *testing.T) {
	pipe := &Pipe{
		Name:  "test",
		Steps: []PipeStep{{ID: "s1", Call: "boom"}},
	}
	caps := map[string]organ.Func{
		"boom": {
			Name: "boom",
			Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
				return organ.Result{}, errors.New("exploded")
			},
		},
	}

	result, err := ReplayPipe(context.Background(), pipe, caps)
	if err != nil {
		t.Fatal(err)
	}
	if result.EscalatedAt != 0 {
		t.Errorf("escalated at %d, want 0", result.EscalatedAt)
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
