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
	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/memory"
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

func sumTokens(pool *ergograph.StubLedger) (int, int) {
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

func instrumentTools(adv *Game) []tangle.Tool {
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

func runScenarioWithNavigator(t *testing.T, scenario Scenario, nav reactivity.Navigator) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	completer := newCompleter(t, ctx)

	reactor := reactivity.NewReactor(
		reactivity.WithNavigator(nav),
		reactivity.WithDirective(reactivity.ExecutionAtom,
			reactivity.Directive("Available instruments:\n"+instrumentList(scenario.Adventure)),
		),
	)

	sensory := cerebrum.NewChannelBus(64)
	signal := NewFixtureSignal()
	pool := &ergograph.StubLedger{}
	cord := &andon.StubSignal{}

	corp := corpus.New()
	for _, cap := range scenario.Adventure.Capabilities() {
		corp.Register(cap)
	}

	var cb *cerebrum.Cerebrum
	motorBus := corp.MotorBus(sensory, signal, func() reactivity.Triad {
		if cb == nil {
			return reactivity.ThinkTriad
		}
		m := cb.Result()
		if m == nil {
			return reactivity.ThinkTriad
		}
		return m.CurrentTriad()
	})

	cb = cerebrum.New(reactor, completer,
		cerebrum.WithSensory(sensory),
		cerebrum.WithMotor(motorBus),
		cerebrum.WithSignal(signal),
		cerebrum.WithPool(pool),
		cerebrum.WithAndon(cord),
		cerebrum.WithCompactor(cerebrum.SummaryCompactor{}),
		cerebrum.WithBudget(cerebrum.Budget{
			MaxTurns:    30,
			TurnTimeout: 30 * time.Second,
		}),
		cerebrum.WithTools(instrumentTools(scenario.Adventure)),
	)

	scenario.Adventure.WithSensory(sensory)

	need := scenario.Need + "\n\nCurrent environment: " + scenario.Adventure.Observe()
	catalyst := reactivity.Catalyst{Need: need, Desired: scenario.Desired}
	if err := cb.Think(ctx, catalyst); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := cb.Result()

	solved := scenario.IsSolved != nil && scenario.IsSolved(scenario.Adventure.State())
	tokensIn, tokensOut := sumTokens(pool)
	result := ScenarioResult{
		Solved:        solved,
		Turns:         m.TotalMass(),
		TotalMass:     m.TotalMass(),
		OptimalTurns:  scenario.OptimalTurns,
		TokensIn:      tokensIn,
		TokensOut:     tokensOut,
		OptimalTokens: scenario.OptimalTurns * 1000,
	}

	t.Logf("=== Scenario: %s (Navigator: %T) ===", scenario.Name, nav)
	t.Logf("Solved: %v | OAE: %.1f%% | Turns: %d (optimal: %d) | Mass: %d | Tokens: %d in + %d out = %d",
		result.Solved, result.OAE()*100, result.Turns, result.OptimalTurns, result.TotalMass,
		result.TokensIn, result.TokensOut, result.TokensIn+result.TokensOut)

	for _, at := range reactivity.AllAtomTypes() {
		if mass := m.Mass(at); mass > 0 {
			t.Logf("  %s: %d", at, mass)
		}
	}

	t.Logf("Signal events: %d", len(signal.Signals()))
	t.Logf("Ergograph: %d records | Andon: %s", pool.Len(), cord.Status())

	if !solved {
		t.Logf("NOT SOLVED")
	}
}

func runScenario(t *testing.T, scenario Scenario, extraOpts ...cerebrum.Option) {
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
	signal := NewFixtureSignal()
	pool := &ergograph.StubLedger{}
	cord := &andon.StubSignal{}

	corp := corpus.New()
	for _, cap := range scenario.Adventure.Capabilities() {
		corp.Register(cap)
	}

	var cb *cerebrum.Cerebrum
	motorBus := corp.MotorBus(sensory, signal, func() reactivity.Triad {
		if cb == nil {
			return reactivity.ThinkTriad
		}
		m := cb.Result()
		if m == nil {
			return reactivity.ThinkTriad
		}
		return m.CurrentTriad()
	})

	opts := []cerebrum.Option{
		cerebrum.WithSensory(sensory),
		cerebrum.WithMotor(motorBus),
		cerebrum.WithSignal(signal),
		cerebrum.WithPool(pool),
		cerebrum.WithAndon(cord),
		cerebrum.WithCompactor(cerebrum.SummaryCompactor{}),
		cerebrum.WithBudget(cerebrum.Budget{
			MaxTurns:    30,
			TurnTimeout: 30 * time.Second,
		}),
		cerebrum.WithTools(instrumentTools(scenario.Adventure)),
	}
	opts = append(opts, extraOpts...)

	cb = cerebrum.New(reactor, completer, opts...)

	scenario.Adventure.WithSensory(sensory)

	need := scenario.Need + "\n\nCurrent environment: " + scenario.Adventure.Observe()
	catalyst := reactivity.Catalyst{Need: need, Desired: scenario.Desired}
	if err := cb.Think(ctx, catalyst); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := cb.Result()

	solved := scenario.IsSolved != nil && scenario.IsSolved(scenario.Adventure.State())
	tokensIn, tokensOut := sumTokens(pool)
	result := ScenarioResult{
		Solved:        solved,
		Turns:         m.TotalMass(),
		TotalMass:     m.TotalMass(),
		OptimalTurns:  scenario.OptimalTurns,
		TokensIn:      tokensIn,
		TokensOut:     tokensOut,
		OptimalTokens: scenario.OptimalTurns * 1000,
	}

	t.Logf("=== Scenario: %s ===", scenario.Name)
	t.Logf("Solved: %v | OAE: %.1f%% | Turns: %d (optimal: %d) | Mass: %d | Tokens: %d in + %d out = %d",
		result.Solved, result.OAE()*100, result.Turns, result.OptimalTurns, result.TotalMass,
		result.TokensIn, result.TokensOut, result.TokensIn+result.TokensOut)

	for _, at := range reactivity.AllAtomTypes() {
		if mass := m.Mass(at); mass > 0 {
			t.Logf("  %s: %d", at, mass)
		}
	}

	t.Logf("Final state:")
	for k, v := range scenario.Adventure.State() {
		t.Logf("  %s: %v", k, v)
	}

	t.Logf("Signal events: %d", len(signal.Signals()))
	for _, sig := range signal.Signals() {
		t.Logf("  [%s] %s", sig.Kind, sig.Source)
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

func TestAblation_Fridge_LinearVsTree(t *testing.T) {
	scenario := NewFridge()

	t.Log("=== LINEAR NAVIGATOR ===")
	runScenario(t, scenario)

	t.Log("=== TREE NAVIGATOR ===")
	scenario2 := NewFridge()
	runScenarioWithNavigator(t, scenario2, reactivity.TreeNavigator)
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

	recollector := cerebrum.MeshRecollector{Mesh: mesh}
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
	pool1 := &ergograph.StubLedger{}

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
		cerebrum.WithPool(pool1),
		cerebrum.WithAndon(&andon.StubSignal{}),
		cerebrum.WithBudget(cerebrum.Budget{MaxTurns: 30, TurnTimeout: 30 * time.Second}),
		cerebrum.WithTools(instrumentTools(scenario.Adventure)),
	)
	if err := cb1.Think(ctx1, reactivity.Catalyst{Need: scenario.Need}); err != nil {
		t.Fatalf("Run 1 Think: %v", err)
	}
	m1 := cb1.Result()
	run1Turns := m1.TotalMass()
	t.Logf("Run 1: mass=%d momentum=%.3f distance=%.3f", run1Turns, m1.Momentum(), m1.Distance())

	// Persist the book
	if err := cerebrum.PersistMolecule(mesh, m1, []byte(scenario.Need)); err != nil {
		t.Fatalf("PersistMolecule: %v", err)
	}
	t.Logf("Book persisted: %d nodes in mesh", len(mesh.Nodes()))

	// Run 2: replay — book moves loaded, should skip Think+Compose
	t.Log("=== RUN 2: REPLAY ===")
	scenario2 := NewFridge()
	recollector := cerebrum.MeshRecollector{Mesh: mesh}
	runScenario(t, scenario2, cerebrum.WithRecollector(recollector))
}
