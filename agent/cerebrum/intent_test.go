package cerebrum

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestIntentRouter_ReflexBypass(t *testing.T) {
	embedder := StubEmbedder{Dims: 8}
	store := NewPipeStore()

	embedding, _ := embedder.Embed(context.Background(), "hello")
	store.Add(Pipe{
		Name:      "greet",
		Embedding: embedding,
		Steps: []PipeStep{{
			ID:       "dialog_speak",
			Call:     "dialog_speak",
			Args:     map[string]any{"response": "Hello! How can I help?"},
			Expected: HashResult([]byte("Hello! How can I help?")),
		}},
	})

	speakCap := organ.Func{
		Name: "dialog_speak",
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			var args struct{ Response string `json:"response"` }
			json.Unmarshal(input, &args)
			return organ.TextResult(args.Response), nil
		},
	}

	completer := &stubCompleter{response: "should not be called"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithEmbedder(embedder),
		WithReflexStore(store),
		WithCapabilities([]organ.Func{speakCap}),
	)

	err := cb.Think(context.Background(), reactivity.Catalyst{Need: "hello"})
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := cb.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed by reflex")
	}
	if m.Response() != "Hello! How can I help?" {
		t.Fatalf("response = %q, want 'Hello! How can I help?'", m.Response())
	}
}

func TestIntentRouter_NoMatch_FallsThrough(t *testing.T) {
	embedder := StubEmbedder{Dims: 8}
	store := NewPipeStore()

	completer := &stubCompleter{response: "novel response"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithEmbedder(embedder),
		WithReflexStore(store),
		WithMaxTurns(3),
	)

	err := cb.Think(context.Background(), reactivity.Catalyst{Need: "something novel", Desired: map[string]any{"done": true}})
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := cb.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed")
	}
}

func TestIntentRouter_NoEmbedder_FallsThrough(t *testing.T) {
	completer := &stubCompleter{response: "ok"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer, WithMaxTurns(3))

	err := cb.Think(context.Background(), reactivity.Catalyst{Need: "test", Desired: map[string]any{"done": true}})
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	if !cb.Result().Sealed() {
		t.Fatal("should seal without embedder")
	}
}
