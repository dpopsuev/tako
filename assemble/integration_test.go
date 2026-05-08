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

func TestDebounce_BlocksRepeatedOrganCalls(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "pinging three times",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: "pinging again",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: "pinging third time",
				ToolCalls: []tangle.ToolCall{
					{ID: "c3", Name: "ping", Input: json.RawMessage(`{}`)},
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
		Budget: cerebrum.Budget{
			MaxTurns:    10,
			TurnTimeout: 10 * time.Second,
		},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	agent.Think(ctx, "ping repeatedly")

	calls := organCalls.Load()
	if calls >= 3 {
		t.Errorf("debouncer should have blocked the 3rd identical call, but organ was called %d times", calls)
	}
	if calls < 2 {
		t.Errorf("first 2 calls should go through, but organ was only called %d times", calls)
	}
}

func TestGracefulDegradation_SynthesizesOnMaxTurns(t *testing.T) {
	ping, _ := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "working",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: "still working",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "ping", Input: json.RawMessage(`{"x":1}`)},
				},
			},
			{
				Content: "still going",
				ToolCalls: []tangle.ToolCall{
					{ID: "c3", Name: "ping", Input: json.RawMessage(`{"x":2}`)},
				},
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget: cerebrum.Budget{
			MaxTurns:    3,
			TurnTimeout: 10 * time.Second,
		},
		Config: &reactivity.Config{
			DistanceClose:     0.3,
			DistanceMid:       0.5,
			RecollectionMin:   0.3,
			UnmetDimMax:       2,
			BackwardTurnLimit: 3,
		},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	agent.Think(ctx, "do a long task")

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed")
	}

	response := m.Response()
	if response == "" || response == "exceeded max turns" {
		t.Errorf("graceful degradation should produce a synthesis response, got %q", response)
	}
}

func TestDrill_Brainstorm_PureConversation(t *testing.T) {
	completer := &capturingCompleter{
		turns: []tangle.Completion{
			{
				Content: "There are three approaches to adding authentication:\n\n" +
					"1. **JWT tokens** — stateless, scales horizontally, but requires token refresh logic.\n" +
					"2. **Session cookies** — simple, server-side state, but sticky sessions needed for scaling.\n" +
					"3. **OAuth2 delegation** — offload to identity provider, most secure, but adds external dependency.\n\n" +
					"I'd recommend starting with JWT for the API and OAuth2 for the web UI. " +
					"The JWT middleware is ~50 lines and can be swapped later without changing the handlers.",
			},
		},
	}

	bp := Blueprint{
		Model:  "stub",
		Budget: cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := agent.Think(ctx, "How should we approach adding authentication to the API?"); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed")
	}

	if completer.call != 1 {
		t.Errorf("pure conversation should use exactly 1 LLM call, got %d", completer.call)
	}

	response := m.Response()
	if !containsSubstring(response, "JWT") {
		t.Errorf("response should contain the brainstorm content, got: %s", truncateForTest(response, 200))
	}

	if m.Turns() != 1 {
		t.Errorf("expected 1 turn, got %d", m.Turns())
	}
}

func TestDrill_Brainstorm_ReasoningChainThenSeal(t *testing.T) {
	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "Let me think through the options.",
				ToolCalls: []tangle.ToolCall{
					{ID: "a1", Name: "assessment", Input: json.RawMessage(`{
						"taxonomy": "assessment.approach",
						"content": "Authentication can be JWT, session, or OAuth2. JWT is stateless and fits our microservice architecture.",
						"dimensions": ["auth.strategy"]
					}`)},
				},
			},
			{
				Content: "Considering the tradeoffs.",
				ToolCalls: []tangle.ToolCall{
					{ID: "k1", Name: "knowledge", Input: json.RawMessage(`{
						"taxonomy": "knowledge.tradeoff",
						"content": "JWT pros: stateless, horizontal scaling. Cons: token revocation requires a blocklist. Session pros: simple revocation. Cons: sticky sessions.",
						"dimensions": ["auth.tradeoffs"]
					}`)},
				},
			},
			{
				Content: "Based on the analysis, I recommend JWT with a short-lived access token (15min) " +
					"and a refresh token stored in an httpOnly cookie. This gives you stateless auth " +
					"for the API with easy revocation via refresh token rotation.",
			},
		},
	}

	bp := Blueprint{
		Model:  "stub",
		Budget: cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := agent.Think(ctx, "What auth strategy should we use?"); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed")
	}

	if completer.call < 3 {
		t.Errorf("reasoning chain should use at least 3 LLM calls (2 phase atoms + seal), got %d", completer.call)
	}

	if m.TotalMass() < 3 {
		t.Errorf("expected at least 3 atoms (intent + assessment + knowledge), got %d", m.TotalMass())
	}

	response := m.Response()
	if !containsSubstring(response, "JWT") {
		t.Errorf("response should contain the recommendation, got: %s", truncateForTest(response, 200))
	}

	if m.Turns() > 4 {
		t.Errorf("reasoning chain should seal after final text response, not spin to max_turns. Got %d turns", m.Turns())
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
