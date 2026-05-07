package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func TestPipeConsolidator_ExtractsSteps(t *testing.T) {
	store := NewPipeStore()
	embedder := StubEmbedder{Dims: 8}
	cons := &PipeConsolidator{Store: store, Embedder: embedder}

	history := []tangle.Message{
		{Role: "user", Content: "fix the bug"},
		{Role: "assistant", ToolCalls: []tangle.ToolCall{
			{ID: "tc1", Name: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)},
		}},
		{Role: "tool", Content: "package main\nfunc main() {}", ToolCallID: "tc1"},
		{Role: "assistant", ToolCalls: []tangle.ToolCall{
			{ID: "tc2", Name: "edit", Input: json.RawMessage(`{"path":"main.go","old":"{}","new":"{fmt.Println()}"}`)},
		}},
		{Role: "tool", Content: "edited", ToolCallID: "tc2"},
	}

	cons.Consolidate(context.Background(), "fix the bug", history)

	if store.Len() != 1 {
		t.Fatalf("expected 1 pipe, got %d", store.Len())
	}

	embedding, _ := embedder.Embed(context.Background(), "fix the bug")
	pipe, sim := store.Match(embedding)
	if sim < 0.99 {
		t.Fatalf("should match, got sim=%v", sim)
	}
	if len(pipe.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(pipe.Steps))
	}
	if pipe.Steps[0].Call != "read_file" {
		t.Errorf("step 0 = %s, want read_file", pipe.Steps[0].Call)
	}
	if pipe.Steps[1].Call != "edit" {
		t.Errorf("step 1 = %s, want edit", pipe.Steps[1].Call)
	}
}

func TestPipeConsolidator_SkipsPhaseTools(t *testing.T) {
	store := NewPipeStore()
	embedder := StubEmbedder{Dims: 8}
	cons := &PipeConsolidator{Store: store, Embedder: embedder}

	history := []tangle.Message{
		{Role: "assistant", ToolCalls: []tangle.ToolCall{
			{ID: "tc1", Name: "intent", Input: json.RawMessage(`{"taxonomy":"intent.goal","content":"test","dimensions":["x"]}`)},
		}},
		{Role: "tool", Content: "atom recorded", ToolCallID: "tc1"},
		{Role: "assistant", ToolCalls: []tangle.ToolCall{
			{ID: "tc2", Name: "bash", Input: json.RawMessage(`{"command":"echo hi"}`)},
		}},
		{Role: "tool", Content: "hi", ToolCallID: "tc2"},
	}

	cons.Consolidate(context.Background(), "run echo", history)

	if store.Len() != 1 {
		t.Fatalf("expected 1 pipe, got %d", store.Len())
	}

	embedding, _ := embedder.Embed(context.Background(), "run echo")
	pipe, _ := store.Match(embedding)
	if len(pipe.Steps) != 1 {
		t.Fatalf("expected 1 step (bash only, intent skipped), got %d", len(pipe.Steps))
	}
	if pipe.Steps[0].Call != "bash" {
		t.Errorf("step = %s, want bash", pipe.Steps[0].Call)
	}
}

func TestPipeConsolidator_MergesOnSecondRun(t *testing.T) {
	store := NewPipeStore()
	embedder := StubEmbedder{Dims: 8}
	cons := &PipeConsolidator{Store: store, Embedder: embedder}

	history1 := []tangle.Message{
		{Role: "assistant", ToolCalls: []tangle.ToolCall{
			{ID: "tc1", Name: "read_file"},
		}},
		{Role: "tool", Content: "content", ToolCallID: "tc1"},
	}

	history2 := []tangle.Message{
		{Role: "assistant", ToolCalls: []tangle.ToolCall{
			{ID: "tc2", Name: "go_build"},
		}},
		{Role: "tool", Content: "ok", ToolCallID: "tc2"},
	}

	cons.Consolidate(context.Background(), "same task", history1)
	cons.Consolidate(context.Background(), "same task", history2)

	if store.Len() != 1 {
		t.Fatalf("should merge into same pipe, got %d pipes", store.Len())
	}
}

func TestFlywheel_ConsolidateThenReflex(t *testing.T) {
	embedder := StubEmbedder{Dims: 8}
	store := NewPipeStore()
	cons := &PipeConsolidator{Store: store, Embedder: embedder}

	speakCap := organ.Func{
		Name: "speak",
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			var args struct{ Response string `json:"response"` }
			json.Unmarshal(input, &args)
			return organ.TextResult(args.Response), nil
		},
	}

	// Session 1: Novel (LLM call)
	completer := &stubCompleter{
		toolCalls: []tangle.ToolCall{{
			ID:    "tc1",
			Name:  "speak",
			Input: json.RawMessage(`{"response":"Hi there!"}`),
		}},
	}

	sensory := NewChannelBus(64)
	motor := &autoExecMotor{caps: map[string]organ.Func{"speak": speakCap}, sensory: sensory}

	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(sensory),
		WithMotor(motor),
		WithEmbedder(embedder),
		WithReflexStore(store),
		WithConsolidator(cons),
		WithCapabilities([]organ.Func{speakCap}),
		WithMaxTurns(3),
	)

	cb.Think(context.Background(), reactivity.Catalyst{Need: "hello", Desired: map[string]any{"greeted": true}})

	if store.Len() != 1 {
		t.Fatalf("consolidator should have written 1 pipe, got %d", store.Len())
	}

	// Session 2: Should be Reflex (zero LLM)
	reactor2 := reactivity.NewReactor()
	shouldNotCall := &stubCompleter{err: fmt.Errorf("LLM should not be called")}
	cb2 := New(reactor2, shouldNotCall,
		WithEmbedder(embedder),
		WithReflexStore(store),
		WithCapabilities([]organ.Func{speakCap}),
	)

	err := cb2.Think(context.Background(), reactivity.Catalyst{Need: "hello"})
	if err != nil {
		t.Fatalf("session 2 Think: %v", err)
	}

	m := cb2.Result()
	if m.Response() != "Hi there!" {
		t.Fatalf("session 2 response = %q, want 'Hi there!'", m.Response())
	}
}
