//go:build integration

package scenarios

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tangle/arsenal"
	"github.com/dpopsuev/tangle/providers"
)

func newCompleter(t *testing.T, ctx context.Context) interface {
	Complete(ctx context.Context, prompt string) (string, error)
} {
	t.Helper()
	region := os.Getenv("CLOUD_ML_REGION")
	project := os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
	if region == "" || project == "" {
		t.Skip("CLOUD_ML_REGION and ANTHROPIC_VERTEX_PROJECT_ID required")
	}

	ars, err := arsenal.NewArsenal("")
	if err != nil {
		t.Fatalf("NewArsenal: %v", err)
	}
	resolved, err := ars.Pick("claude-sonnet-4-6", "vertex-ai")
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	t.Logf("Model: %s Provider: %s", resolved.Model, resolved.Provider)

	provider, err := providers.NewVertexProvider(ctx, region, project)
	if err != nil {
		t.Fatalf("NewVertexProvider: %v", err)
	}
	return providers.NewCompleter(provider, resolved.Model, nil)
}

func runScenario(t *testing.T, scenario Scenario) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	completer := newCompleter(t, ctx)
	reactor := reactivity.NewReactor()
	sensory := cerebrum.NewChannelBus(64)
	motor := NewFixtureMotor(scenario.Instruments, sensory)
	signal := NewFixtureSignal()

	cb := cerebrum.New(reactor, completer,
		cerebrum.WithMotor(motor),
		cerebrum.WithSignal(signal),
		cerebrum.WithMaxTurns(20),
	)

	if err := cb.Think(ctx, []byte(scenario.Need)); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := cb.Result()

	t.Logf("=== Scenario: %s ===", scenario.Name)
	t.Logf("Total atoms: %d", m.TotalMass())
	for _, at := range reactivity.AllAtomTypes() {
		if mass := m.Mass(at); mass > 0 {
			t.Logf("  %s: %d", at, mass)
		}
	}

	t.Logf("Motor calls: %d", len(motor.Calls()))
	for _, call := range motor.Calls() {
		t.Logf("  [%s] %s: %s", call.Kind, call.Source, string(call.Payload))
	}

	t.Logf("Signals: %d", len(signal.Signals()))
	for _, sig := range signal.Signals() {
		t.Logf("  [%s] %s", sig.Kind, sig.Source)
	}

	for _, at := range reactivity.AllAtomTypes() {
		for _, a := range m.Atoms(at) {
			content := string(a.Content)
			if len(content) > 150 {
				content = content[:150] + "..."
			}
			t.Logf("  [%s] %s: %s", at, a.Taxonomy, content)
		}
	}

	if scenario.Expect.Sealed && !m.Sealed() {
		t.Error("expected molecule to be sealed")
	}
	if m.TotalMass() < scenario.Expect.MinAtoms {
		t.Errorf("expected at least %d atoms, got %d", scenario.Expect.MinAtoms, m.TotalMass())
	}
}

func TestScenario_Fridge(t *testing.T) {
	runScenario(t, Fridge)
}

func TestScenario_DirtyRoom(t *testing.T) {
	runScenario(t, DirtyRoom)
}
