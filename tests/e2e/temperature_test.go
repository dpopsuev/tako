package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/testkit"
	"github.com/dpopsuev/tako/testkit/arcade"
	"github.com/dpopsuev/tangle/providers"
)

func TestExperiment_FridgeTemperatureCurve(t *testing.T) {
	if os.Getenv("TAKO_PROVIDER") == "" {
		t.Skip("set TAKO_PROVIDER for real LLM experiment")
	}
	model := os.Getenv("TAKO_TEST_MODEL")
	if model == "" {
		t.Fatal("TAKO_TEST_MODEL not set")
	}

	p, err := providers.NewProviderFromEnv("TAKO_PROVIDER")
	if err != nil {
		t.Fatal(err)
	}
	completer := providers.NewCompleter(p, model, nil)

	embedder := cerebrum.StubEmbedder{Dims: 64}
	reflexStore, consolidator := arcade.NewFlywheel(embedder)

	sessions := 5
	report := arcade.ExperimentReport{Scenario: "fridge"}

	for i := 0; i < sessions; i++ {
		scenario := arcade.NewFridge()
		listener := testkit.NewCapturingListener()

		agent := arcade.BuildArcadeAgent(scenario, completer,
			arcade.WithEmbedder(embedder),
			arcade.WithReflexStore(reflexStore),
			arcade.WithConsolidator(consolidator),
			arcade.WithListener(listener),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		_, err := arcade.ThinkScenario(ctx, agent, scenario)
		cancel()
		if err != nil {
			t.Logf("session %d error: %v", i+1, err)
			continue
		}

		result := arcade.CollectResult(i+1, scenario, agent, listener, reflexStore)
		report.Sessions = append(report.Sessions, result)
	}

	t.Log(report.Pretty())
	t.Log("\n=== JSON (for agent consumption) ===")
	t.Log(report.JSON())

	solvedCount := 0
	for _, s := range report.Sessions {
		if s.Solved {
			solvedCount++
		}
	}
	if solvedCount == 0 {
		t.Error("no sessions solved")
	}
}
