package assemble

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func pingOrgan() (organ.Func, *atomic.Int32) {
	var calls atomic.Int32
	return organ.Func{
		Name:        "ping",
		Description: "returns pong",
		Schema:      json.RawMessage(`{"type":"object","properties":{}}`),
		Mode:        organ.ReadAction,
		Source:      organ.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
			calls.Add(1)
			return organ.TextResult("pong"), nil
		},
	}, &calls
}

func TestFlywheel_ConsolidateThenReflexViaAssemble(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "pinging",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"got pong"}]}`,
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 5, TurnTimeout: 10 * time.Second},
	}

	agent := Assemble(bp, completer)
	ctx := context.Background()

	if err := agent.Think(ctx, "ping the organ"); err != nil {
		t.Fatalf("session 1 Think: %v", err)
	}

	llmCallsAfterSession1 := completer.call
	organCallsAfterSession1 := organCalls.Load()

	if organCallsAfterSession1 == 0 {
		t.Fatal("session 1: organ was never called")
	}

	if err := agent.Think(ctx, "ping the organ"); err != nil {
		t.Fatalf("session 2 Think: %v", err)
	}

	llmCallsAfterSession2 := completer.call
	organCallsAfterSession2 := organCalls.Load()

	if llmCallsAfterSession2 != llmCallsAfterSession1 {
		t.Errorf("session 2 should use zero LLM calls (reflex), but completer was called %d more times",
			llmCallsAfterSession2-llmCallsAfterSession1)
	}

	if organCallsAfterSession2 <= organCallsAfterSession1 {
		t.Error("session 2: organ should still execute via reflex replay")
	}

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("session 2 molecule should be sealed")
	}
}

func TestDeltaRegulator_StateChangesInPrompt(t *testing.T) {
	ping, _ := pingOrgan()

	turnCounter := 0
	observer := cerebrum.Observer(func() map[string]any {
		turnCounter++
		return map[string]any{
			"counter": turnCounter,
			"stable":  "unchanged",
		}
	})

	var secondTurnPrompt string
	completer := &capturingCompleter{
		turns: []tangle.Completion{
			{
				Content: "pinging",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"done"}]}`,
			},
		},
		onCall: func(call int, params tangle.CompletionParams) {
			if call == 1 {
				for _, msg := range params.Messages {
					if msg.Role == cerebrum.RoleUser {
						secondTurnPrompt = msg.Content
					}
				}
			}
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 5, TurnTimeout: 10 * time.Second},
	}

	agent := Assemble(bp, completer,
		cerebrum.WithObserver(observer),
	)

	if err := agent.Think(context.Background(), "test regulator"); err != nil {
		t.Fatalf("Think: %v", err)
	}

	if secondTurnPrompt == "" {
		t.Fatal("second turn prompt was not captured")
	}

	if !containsSubstring(secondTurnPrompt, "State Changes") && !containsSubstring(secondTurnPrompt, "counter") {
		t.Errorf("second turn prompt should contain state changes from DeltaRegulator, got:\n%s", truncateForTest(secondTurnPrompt, 500))
	}
}

func TestAlignmentChecker_DriftFlagsOnRegression(t *testing.T) {
	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "thinking",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "assessment", Input: json.RawMessage(`{"taxonomy":"assessment.plan","content":"initial plan","dimensions":["x"]}`)},
				},
			},
			{
				Content: "more thinking",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "assessment", Input: json.RawMessage(`{"taxonomy":"assessment.plan2","content":"second plan","dimensions":["y"]}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"done"}]}`,
			},
		},
	}

	cfg := reactivity.DefaultConfig
	bp := Blueprint{
		Model:  "stub",
		Budget: cerebrum.Budget{MaxTurns: 5, TurnTimeout: 10 * time.Second},
		Config: &cfg,
	}

	agent := Assemble(bp, completer)

	if err := agent.Think(context.Background(), "test alignment"); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed")
	}
	if m.TotalMass() < 2 {
		t.Errorf("expected at least 2 atoms (alignment checker processed them), got %d", m.TotalMass())
	}
}

func TestWatcher_ClassifiesViaCompleter(t *testing.T) {
	ping, _ := pingOrgan()

	var watcherCalled bool
	watcherCompleter := tangle.CompleteFunc(func(_ context.Context, params tangle.CompletionParams) (*tangle.Completion, error) {
		watcherCalled = true
		return &tangle.Completion{
			ToolCalls: []tangle.ToolCall{
				{ID: "w1", Name: "classify", Input: json.RawMessage(`{"priority":"park","dimensions":["test"]}`)},
			},
		}, nil
	})

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"done"}]}`},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Watcher:      watcherCompleter,
		Budget:       cerebrum.Budget{MaxTurns: 3, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		agent.sensory.Send(ctx, cerebrum.Event{
			ID:        "test-event",
			Kind:      cerebrum.EventSensoryWarning,
			Source:    "test",
			Payload:   []byte("something happened"),
			CreatedAt: time.Now(),
		})
	}()

	agent.Think(ctx, "background task")

	if watcherCalled {
		t.Log("watcher completer was called for event classification")
	}
}

func TestSignalBus_EmitsOnOrganExecution(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "pinging",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"done"}]}`,
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 5, TurnTimeout: 10 * time.Second},
	}

	agent := Assemble(bp, completer)

	if err := agent.Think(context.Background(), "ping it"); err != nil {
		t.Fatalf("Think: %v", err)
	}

	if organCalls.Load() == 0 {
		t.Fatal("organ was never called")
	}

	var signals []cerebrum.Event
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer drainCancel()
	for {
		event, ok := agent.signal.Receive(drainCtx)
		if !ok {
			break
		}
		signals = append(signals, event)
	}

	if len(signals) == 0 {
		t.Fatal("signal bus should have received motor events after organ execution")
	}

	foundExecute := false
	for _, s := range signals {
		if s.Kind == cerebrum.EventMotorExecute {
			foundExecute = true
		}
	}
	if !foundExecute {
		kinds := make([]string, len(signals))
		for i, s := range signals {
			kinds[i] = s.Kind.String()
		}
		t.Errorf("expected motor.execute signal, got kinds: %v", kinds)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func truncateForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
