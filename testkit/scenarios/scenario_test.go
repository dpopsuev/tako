//go:build integration

package scenarios

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/service/andon"
	tangle "github.com/dpopsuev/tangle"
	"github.com/dpopsuev/tangle/arsenal"
	"github.com/dpopsuev/tangle/providers"
)

func TestMain(m *testing.M) {
	level := slog.LevelInfo
	if os.Getenv("SLOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	os.Exit(m.Run())
}

func newCompleter(t *testing.T, ctx context.Context) tangle.Completer {
	t.Helper()
	region := os.Getenv("GOOGLE_CLOUD_LOCATION")
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if region == "" || project == "" {
		t.Skip("GOOGLE_CLOUD_LOCATION and GOOGLE_CLOUD_PROJECT required")
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

func instrumentTools(adv *TextAdventure) []tangle.Tool {
	var tools []tangle.Tool
	for _, name := range adv.Names() {
		desc, _ := adv.Describe(name)
		schema, _ := adv.Schema(name)
		tools = append(tools, tangle.Tool{
			Name:        name,
			Description: desc,
			InputSchema: schema,
		})
	}
	return tools
}

func runScenario(t *testing.T, scenario Scenario) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

	pool := &ergograph.StubLedger{}
	cord := &andon.StubSignal{}

	cb := cerebrum.New(reactor, completer,
		cerebrum.WithSensory(sensory),
		cerebrum.WithMotor(motor),
		cerebrum.WithSignal(signal),
		cerebrum.WithPool(pool),
		cerebrum.WithAndon(cord),
		cerebrum.WithBudget(cerebrum.Budget{
			MaxTurns:    30,
			TurnTimeout: 30 * time.Second,
			MinOAE:      0.3,
		}),
		cerebrum.WithTools(instrumentTools(scenario.Adventure)),
	)

	if err := cb.Think(ctx, []byte(scenario.Need)); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := cb.Result()

	solved := scenario.IsSolved != nil && scenario.IsSolved(scenario.Adventure.State())
	result := ScenarioResult{
		Solved:       solved,
		Turns:        m.TotalMass(),
		MotorCalls:   len(motor.Calls()),
		TotalMass:    m.TotalMass(),
		OptimalTurns: scenario.OptimalTurns,
	}

	t.Logf("=== Scenario: %s ===", scenario.Name)
	t.Logf("Solved: %v | OAE: %.1f%% | Turns: %d (optimal: %d) | Motor calls: %d | Mass: %d",
		result.Solved, result.OAE()*100, result.Turns, result.OptimalTurns, result.MotorCalls, result.TotalMass)

	for _, at := range reactivity.AllAtomTypes() {
		if mass := m.Mass(at); mass > 0 {
			t.Logf("  %s: %d", at, mass)
		}
	}

	for _, call := range motor.Calls() {
		payload := string(call.Payload)
		if len(payload) > 80 {
			payload = payload[:80] + "..."
		}
		t.Logf("  [%s] %s: %s", call.Kind, call.Source, payload)
	}

	t.Logf("Final state:")
	for k, v := range scenario.Adventure.State() {
		t.Logf("  %s: %v", k, v)
	}

	t.Logf("Ergograph: %d records | Andon: %s", pool.Len(), cord.Status())
	for _, rec := range pool.Records() {
		t.Logf("  [%s] %v", rec.Action, rec.Labels)
	}

	if !solved {
		t.Logf("NOT SOLVED")
	}
}

func TestSmoke_SingleCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	completer := newCompleter(t, ctx)

	reactor := reactivity.NewReactor()
	cb := cerebrum.New(reactor, completer,
		cerebrum.WithMaxTurns(3),
		cerebrum.WithPromptBuilder(cerebrum.BasicPromptBuilder),
		cerebrum.WithParser(cerebrum.PlainTextParser),
	)

	t.Log("Sending single Think with 3 max turns...")
	start := time.Now()
	if err := cb.Think(ctx, []byte("Say hello")); err != nil {
		t.Fatalf("Think: %v", err)
	}
	t.Logf("Think completed in %s", time.Since(start))

	m := cb.Result()
	t.Logf("Sealed: %v, Mass: %d, Phase: %s", m.Sealed(), m.TotalMass(), m.Phase())
	for _, at := range reactivity.AllAtomTypes() {
		if mass := m.Mass(at); mass > 0 {
			t.Logf("  %s: %d", at, mass)
		}
	}
}

func TestScenario_Fridge(t *testing.T) {
	runScenario(t, NewFridge())
}

func TestScenario_PastaBolognese(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	sensory := cerebrum.NewChannelBus(64)
	runScenario(t, NewPastaBolognese(ctx, sensory))
}

func TestScenario_Takoyaki(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	sensory := cerebrum.NewChannelBus(64)
	runScenario(t, NewTakoyaki(ctx, sensory))
}

func TestScenario_DirtyRoom(t *testing.T) {
	runScenario(t, NewDirtyRoom())
}

func TestScenario_Tork(t *testing.T) {
	runScenario(t, NewTork())
}

func TestScenario_TakoTrail(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	sensory := cerebrum.NewChannelBus(64)
	runScenario(t, NewTakoTrail(ctx, sensory))
}

func TestScenario_Takonomics(t *testing.T) {
	runScenario(t, NewTakonomics())
}

func TestScenario_HuntTheTako(t *testing.T) {
	runScenario(t, NewHuntTheTako())
}
