//go:build integration

package scenarios

import (
	"context"
	"fmt"
	"os"
	"strings"
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

func instrumentList(adv *TextAdventure) string {
	var parts []string
	for _, name := range adv.Names() {
		desc, _ := adv.Describe(name)
		parts = append(parts, fmt.Sprintf("- %s: %s", name, desc))
	}
	return strings.Join(parts, "\n")
}

func runScenario(t *testing.T, scenario Scenario) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	completer := newCompleter(t, ctx)

	reactor := reactivity.NewReactor(
		reactivity.WithDirective(reactivity.ExecutionAtom,
			reactivity.Directive("Available instruments:\n"+instrumentList(scenario.Adventure)),
		),
	)

	sensory := cerebrum.NewChannelBus(64)
	motor := NewFixtureMotor(scenario.Adventure.Names(), sensory)
	signal := NewFixtureSignal()

	// Wire motor to use the adventure's stateful instruments
	motor.instruments = make(map[string]string)
	motor.adventure = scenario.Adventure

	cb := cerebrum.New(reactor, completer,
		cerebrum.WithMotor(motor),
		cerebrum.WithSignal(signal),
		cerebrum.WithMaxTurns(30),
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
		payload := string(call.Payload)
		if len(payload) > 100 {
			payload = payload[:100] + "..."
		}
		t.Logf("  [%s] %s: %s", call.Kind, call.Source, payload)
	}

	t.Logf("Final state:")
	for k, v := range scenario.Adventure.State() {
		t.Logf("  %s: %v", k, v)
	}

	if m.Sealed() {
		t.Logf("Molecule: SEALED")
	} else {
		t.Logf("Molecule: OPEN (phase: %s)", m.Phase())
	}

	if scenario.IsSolved != nil {
		if scenario.IsSolved(scenario.Adventure.State()) {
			t.Logf("SOLVED")
		} else {
			t.Logf("NOT SOLVED")
		}
	}
}

func TestScenario_Fridge(t *testing.T) {
	runScenario(t, NewFridge())
}

func TestScenario_PastaBolognese(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	sensory := cerebrum.NewChannelBus(64)
	runScenario(t, NewPastaBolognese(ctx, sensory))
}

func TestScenario_DirtyRoom(t *testing.T) {
	runScenario(t, NewDirtyRoom())
}
