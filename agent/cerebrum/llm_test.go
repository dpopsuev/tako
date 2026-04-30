//go:build integration

package cerebrum

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tangle/arsenal"
	"github.com/dpopsuev/tangle/providers"
)

func TestThink_RealLLM_Vertex(t *testing.T) {
	region := os.Getenv("CLOUD_ML_REGION")
	project := os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
	if region == "" || project == "" {
		t.Skip("CLOUD_ML_REGION and ANTHROPIC_VERTEX_PROJECT_ID required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Arsenal picks the model — no hardcoded names
	ars, err := arsenal.NewArsenal("")
	if err != nil {
		t.Fatalf("NewArsenal: %v", err)
	}

	resolved, err := ars.Pick("claude-sonnet-4-6", "vertex-ai")
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	model := resolved.Model
	t.Logf("Arsenal picked: model=%s provider=%s", model, resolved.Provider)

	// Provider connects to Vertex
	provider, err := providers.NewVertexProvider(ctx, region, project)
	if err != nil {
		t.Fatalf("NewVertexProvider: %v", err)
	}

	// NewCompleter bridges provider + model into troupe.Completer
	completer := providers.NewCompleter(provider, model, nil)

	// Cerebrum thinks
	circuit := reactivity.NewReactor()
	cb := New(circuit, completer, WithMaxTurns(10))

	if err := cb.Think(ctx, []byte("What is 2+2? Answer in one word.")); err != nil {
		t.Fatalf("Think: %v", err)
	}
	m := cb.Result()

	if !m.Sealed() {
		t.Error("Molecule should be sealed")
	}

	t.Logf("Real LLM test:")
	t.Logf("  Model: %s", model)
	t.Logf("  Total atoms: %d", m.TotalMass())
	t.Logf("  Intent: %d", m.Mass(reactivity.IntentAtom))
	t.Logf("  Assessment: %d", m.Mass(reactivity.AssessmentAtom))
	t.Logf("  Plan: %d", m.Mass(reactivity.ExpansionAtom))
	t.Logf("  Execution: %d", m.Mass(reactivity.ExecutionAtom))
	t.Logf("  Retrospection: %d", m.Mass(reactivity.RetrospectionAtom))

	for _, a := range m.Atoms(reactivity.IntentAtom) {
		content := string(a.Content)
		if len(content) > 200 {
			content = content[:200]
		}
		t.Logf("  Intent: %s", content)
	}
}
