//go:build integration

package arcade

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/corpus"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/memory"
	"github.com/dpopsuev/tako/service/andon"
	"github.com/dpopsuev/tako/testkit/rehearsal"
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

func sumTokens(pool *StubRecorder) (int, int) {
	var tokIn, tokOut int
	for _, rec := range pool.Records() {
		if rec.Action != "cerebrum.turn" {
			continue
		}
		if v, ok := rec.Labels["tokens_in"]; ok {
			n, _ := strconv.Atoi(v)
			tokIn += n
		}
		if v, ok := rec.Labels["tokens_out"]; ok {
			n, _ := strconv.Atoi(v)
			tokOut += n
		}
	}
	return tokIn, tokOut
}

func instrumentList(adv *Game) string {
	var parts []string
	for _, name := range adv.Names() {
		desc, _ := adv.Describe(name)
		parts = append(parts, fmt.Sprintf("- %s: %s", name, desc))
	}
	return strings.Join(parts, "\n")
}

func navigatorFromEnv() reactivity.Navigator {
	switch os.Getenv("TAKO_NAVIGATOR") {
	case "tree":
		return reactivity.TreeNavigator
	default:
		return reactivity.LinearNavigator
	}
}

func runScenario(t *testing.T, scenario Scenario, extraOpts ...cerebrum.Option) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	completer := newCompleter(t, ctx)
	agent := BuildArcadeAgent(scenario, completer)
	referee := ArcadeReferee(scenario)

	runner, err := rehearsal.NewRunBuilder().
		WithScenario(rehearsal.NewStubScenario(scenario.Name, scenario.Need)).
		WithReferee(referee).
		WithActor(agent).
		Build()
	if err != nil {
		t.Fatalf("RunBuilder: %v", err)
	}

	metrics, err := runner.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	t.Logf("=== Scenario: %s ===", scenario.Name)
	t.Logf("Pass: %v | Score: %.2f | Elapsed: %v", metrics.Pass, metrics.Score, metrics.TimeElapsed)

	t.Logf("Final state:")
	for k, v := range scenario.Adventure.State() {
		t.Logf("  %s: %v", k, v)
	}

	if !metrics.Pass {
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
	if err := cb.Think(ctx, reactivity.Catalyst{Need: "Say hello"}); err != nil {
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

func TestScenario_Impossible(t *testing.T) {
	runScenario(t, NewImpossible())
}

func TestScenario_Autoassembler(t *testing.T) {
	dir := t.TempDir()
	runScenario(t, NewAutoassembler(dir))
}

func TestScenario_Fridge_WithRecollection(t *testing.T) {
	mesh := memory.NewStubMesh()
	mesh.AddNode(memory.KnowledgeNode{
		ID:      "k1",
		Content: "The fridge contains eggs, milk, and cheese. The stove must be turned on before cooking.",
		Tier:    memory.Knowledge,
	})
	mesh.AddNode(memory.KnowledgeNode{
		ID:      "k2",
		Content: "To eat: take item from fridge, turn on stove, cook, then eat from plate.",
		Tier:    memory.Understanding,
	})

	recollector := MeshRecollector{Mesh: mesh}
	runScenario(t, NewFridge(), cerebrum.WithRecollector(recollector))
}

func TestScenario_Fridge_BookMoves(t *testing.T) {
	mesh := memory.NewStubMesh()
	scenario := NewFridge()

	// Run 1: learn — no book, full dialectic
	t.Log("=== RUN 1: LEARN ===")
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel1()
	completer := newCompleter(t, ctx1)
	reactor1 := reactivity.NewReactor(
		reactivity.WithDirective(reactivity.ExecutionAtom,
			reactivity.Directive("Available instruments:\n"+instrumentList(scenario.Adventure)),
		),
	)
	sensory1 := cerebrum.NewChannelBus(64)
	pool1 := &StubRecorder{}

	corp1 := corpus.New()
	for _, cap := range scenario.Adventure.Capabilities() {
		corp1.Register(cap)
	}

	var cb1 *cerebrum.Cerebrum
	motorBus1 := corp1.MotorBus(sensory1, NewFixtureSignal(), func() reactivity.Triad {
		if cb1 == nil {
			return reactivity.ThinkTriad
		}
		m := cb1.Result()
		if m == nil {
			return reactivity.ThinkTriad
		}
		return m.CurrentTriad()
	})

	cb1 = cerebrum.New(reactor1, completer,
		cerebrum.WithSensory(sensory1),
		cerebrum.WithMotor(motorBus1),
		cerebrum.WithRecorder(pool1),
		cerebrum.WithHalter(&andon.StubSignal{}),
		cerebrum.WithBudget(cerebrum.Budget{MaxTurns: 30, TurnTimeout: 30 * time.Second}),
		cerebrum.WithObserver(scenario.Adventure.State),
		cerebrum.WithCapabilities(scenario.Adventure.Capabilities()),
	)
	if err := cb1.Think(ctx1, reactivity.Catalyst{Need: scenario.Need}); err != nil {
		t.Fatalf("Run 1 Think: %v", err)
	}
	m1 := cb1.Result()
	run1Turns := m1.TotalMass()
	t.Logf("Run 1: mass=%d momentum=%.3f distance=%.3f", run1Turns, m1.Momentum(), m1.Distance())

	// Persist the book
	if err := PersistMolecule(mesh, m1, []byte(scenario.Need)); err != nil {
		t.Fatalf("PersistMolecule: %v", err)
	}
	t.Logf("Book persisted: %d nodes in mesh", len(mesh.Nodes()))

	// Run 2: replay — book moves loaded, should skip Think+Compose
	t.Log("=== RUN 2: REPLAY ===")
	scenario2 := NewFridge()
	recollector := MeshRecollector{Mesh: mesh}
	runScenario(t, scenario2, cerebrum.WithRecollector(recollector))
}
