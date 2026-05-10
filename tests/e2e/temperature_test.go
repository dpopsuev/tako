package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/assemble"
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
	reflexStore := cerebrum.NewPipeStore()
	consolidator := &cerebrum.PipeConsolidator{
		Store:    reflexStore,
		Embedder: embedder,
	}

	sessions := 5
	type sessionResult struct {
		Turns      int
		Sealed     bool
		Distance   float64
		Pressure   float64
		ReflexHits int
		Solved     bool
	}
	results := make([]sessionResult, 0, sessions)

	for i := 0; i < sessions; i++ {
		scenario := arcade.NewFridge()
		bp := assemble.Blueprint{
			Model:  model,
			Organs: scenario.Adventure.Organs(),
			Budget: cerebrum.Budget{
				MaxTurns:    20,
				TurnTimeout: 60 * time.Second,
			},
		}

		observe := func() map[string]any {
			return map[string]any{"world": scenario.Adventure.Observe()}
		}
		listener := &instrumentedListener{t: t, session: i + 1}
		agent := assemble.Assemble(bp, completer,
			cerebrum.WithEmbedder(embedder),
			cerebrum.WithReflexStore(reflexStore),
			cerebrum.WithConsolidator(consolidator),
			cerebrum.WithObserver(observe),
			cerebrum.WithContextListener(listener),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		catalyst := reactivity.Catalyst{
			Need:    scenario.Need,
			Desired: scenario.Desired,
		}
		if _, err := agent.ThinkWith(ctx, catalyst); err != nil {
			cancel()
			t.Logf("session %d error: %v", i+1, err)
			continue
		}
		cancel()

		m := agent.Result()
		solved := scenario.IsSolved(scenario.Adventure.State())

		sr := sessionResult{
			Turns:    m.Turns(),
			Sealed:   m.Sealed(),
			Distance: m.Distance(),
			Pressure: m.Pressure(),
			Solved:   solved,
		}
		results = append(results, sr)

		chain := m.Chain()
		t.Logf("session %d: turns=%d sealed=%v distance=%.2f pressure=%.2f solved=%v pipes=%d chain=%d",
			i+1, sr.Turns, sr.Sealed, sr.Distance, sr.Pressure, sr.Solved, reflexStore.Len(), chain.Len())
		for j, e := range chain.All() {
			out := string(e.Output)
			if len(out) > 80 { out = out[:80] + "..." }
			t.Logf("  chain[%d]: %s organ=%s response=%v out=%s", j, e.Kind, e.Organ, e.IsResponse, out)
		}
		state := scenario.Adventure.State()
		t.Logf("  game_state: hungry=%v hand=%v plate=%v stove=%v", state["hungry"], state["hand"], state["plate"], state["stove"])
	}

	t.Log("\n=== TEMPERATURE CURVE ===")
	t.Log(fmt.Sprintf("%-8s %-6s %-7s %-8s %-9s %-6s %-5s", "Session", "Turns", "Sealed", "Distance", "Pressure", "Solved", "Pipes"))
	for i, r := range results {
		t.Log(fmt.Sprintf("%-8d %-6d %-7v %-8.2f %-9.2f %-6v %-5d",
			i+1, r.Turns, r.Sealed, r.Distance, r.Pressure, r.Solved, reflexStore.Len()))
	}

	if len(results) >= 2 {
		first := results[0]
		last := results[len(results)-1]
		if last.Turns > first.Turns+5 {
			t.Errorf("later sessions should not be significantly slower: first=%d last=%d", first.Turns, last.Turns)
		}
	}

	solvedCount := 0
	for _, r := range results {
		if r.Solved {
			solvedCount++
		}
	}
	t.Logf("solved %d/%d sessions", solvedCount, len(results))
}

type instrumentedListener struct {
	t       *testing.T
	session int
}

func (l *instrumentedListener) OnContext(phase string, turn int, distance float64) {
	l.t.Logf("  [s%d] context: phase=%s turn=%d distance=%.2f", l.session, phase, turn, distance)
}

func (l *instrumentedListener) OnToolCall(name string, input []byte) {
	s := string(input)
	if len(s) > 100 { s = s[:100] + "..." }
	l.t.Logf("  [s%d] tool_call: %s input=%s", l.session, name, s)
}

func (l *instrumentedListener) OnToolResult(name string, result []byte, elapsed time.Duration) {
	s := string(result)
	if len(s) > 150 { s = s[:150] + "..." }
	l.t.Logf("  [s%d] tool_result: %s elapsed=%v result=%s", l.session, name, elapsed, s)
}

func (l *instrumentedListener) OnResponse(text string) {
	if len(text) > 150 { text = text[:150] + "..." }
	l.t.Logf("  [s%d] response: %s", l.session, text)
}

func (l *instrumentedListener) OnTokenUpdate(tokensIn, tokensOut, toolCalls int) {}
func (l *instrumentedListener) OnSealed(id string, distance float64, turns int) {
	l.t.Logf("  [s%d] sealed: molecule=%s distance=%.2f turns=%d", l.session, id, distance, turns)
}
func (l *instrumentedListener) OnError(turn int, err error) {
	l.t.Logf("  [s%d] error: turn=%d err=%v", l.session, turn, err)
}
func (l *instrumentedListener) OnToken(_ string) {}
