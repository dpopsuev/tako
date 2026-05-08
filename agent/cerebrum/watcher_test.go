package cerebrum

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func TestWatcherClassifier_ReflexHit(t *testing.T) {
	store := NewPipeStore()
	store.Add(Pipe{
		Name:      "read",
		Embedding: []float64{1, 0, 0},
		Steps:     []PipeStep{{ID: "read", Call: "file.read"}},
	})

	embedder := StubEmbedder{Dims: 3}
	wc := &WatcherClassifier{Reflex: store, Embedder: embedder}
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"file.exists": "done"},
	})

	event := Event{Kind: "sensory.input", Source: "user"}
	priority := wc.Classify(event, m)

	if priority != PriorityPark {
		t.Fatalf("reflex hit should return park, got %s", priority)
	}
}

func TestWatcherClassifier_LLMClassify(t *testing.T) {
	classifyInput, _ := json.Marshal(classifyResult{
		Priority:   "interrupt",
		Dimensions: []string{"urgent"},
		Action:     "mol-urgent",
	})

	wc := &WatcherClassifier{
		Watcher: &stubCompleter{
			toolCalls: []tangle.ToolCall{{
				ID:    "wc-1",
				Name:  "classify",
				Input: classifyInput,
			}},
		},
	}

	m := reactivity.NewMolecule("test")
	event := Event{Kind: "sensory.alarm", Source: "timer"}
	priority := wc.Classify(event, m)

	if priority != PriorityInterrupt {
		t.Fatalf("LLM classify should return interrupt, got %s", priority)
	}
}

func TestWatcherClassifier_FallbackOnNoWatcher(t *testing.T) {
	wc := &WatcherClassifier{}
	m := reactivity.NewMolecule("test")

	event := Event{Kind: "sensory.alarm", Source: "timer"}
	priority := wc.Classify(event, m)

	if priority != PriorityEmergency {
		t.Fatalf("fallback should use default classifier, alarm = emergency, got %s", priority)
	}
}

func TestWatcherClassifier_LLMError(t *testing.T) {
	wc := &WatcherClassifier{
		Watcher: &stubCompleter{err: context.DeadlineExceeded},
	}

	m := reactivity.NewMolecule("test")
	event := Event{Kind: "sensory.input", Source: "user"}
	priority := wc.Classify(event, m)

	if priority != PriorityPark {
		t.Fatalf("on error should fall back to park, got %s", priority)
	}
}

func TestWatcherClassifier_Interface(t *testing.T) {
	var _ PriorityClassifier = (*WatcherClassifier)(nil)
}
