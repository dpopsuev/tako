//go:build integration

package cerebrum

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/instrument"
	"github.com/dpopsuev/tangle/providers"
)

// VertexCompleter wraps the Vertex provider directly into Completer.
// Bypasses full Tangle broker machinery for testing.
// In production, TangleCompleter wraps Agent.Perform instead.
type VertexCompleter struct {
	provider *providers.VertexProvider
	model    string
}

func (vc *VertexCompleter) Complete(ctx context.Context, prompt []byte) ([]byte, error) {
	resp, err := vc.provider.Completion(ctx, providers.CompletionParams{
		Model: vc.model,
		Messages: []providers.Message{
			{Role: "user", Content: string(prompt)},
		},
		MaxTokens: 256,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return []byte(""), nil
	}
	return []byte(resp.Choices[0].Content), nil
}

func TestThink_RealLLM_Vertex(t *testing.T) {
	region := os.Getenv("CLOUD_ML_REGION")
	project := os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
	if region == "" || project == "" {
		t.Skip("CLOUD_ML_REGION and ANTHROPIC_VERTEX_PROJECT_ID required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	provider, err := providers.NewVertexProvider(ctx, region, project)
	if err != nil {
		t.Fatalf("NewVertexProvider: %v", err)
	}

	completer := &VertexCompleter{provider: provider, model: "claude-sonnet-4-20250514"}

	var _ instrument.Completer = completer

	circuit := reactivity.NewCircuit()
	cb := New(circuit, completer)
	cb.maxTurns = 10

	m, err := cb.Think(ctx, []byte("What is 2+2? Answer in one word."))
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	if !m.Sealed() {
		t.Error("Molecule should be sealed")
	}

	t.Logf("Real LLM test complete:")
	t.Logf("  Total atoms: %d", m.TotalMass())
	t.Logf("  Intent: %d", m.Mass(reactivity.IntentAtom))
	t.Logf("  Assessment: %d", m.Mass(reactivity.AssessmentAtom))
	t.Logf("  Plan: %d", m.Mass(reactivity.PlanAtom))
	t.Logf("  Execution: %d", m.Mass(reactivity.ExecutionAtom))
	t.Logf("  Retrospection: %d", m.Mass(reactivity.RetrospectionAtom))

	for _, a := range m.Atoms(reactivity.IntentAtom) {
		content := string(a.Content)
		if len(content) > 100 {
			content = content[:100]
		}
		t.Logf("  Intent atom: %s", content)
	}
}
