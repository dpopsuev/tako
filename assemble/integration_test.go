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

	if _, err := agent.Think(ctx, "ping the organ"); err != nil {
		t.Fatalf("session 1 Think: %v", err)
	}

	llmCallsAfterSession1 := completer.call
	organCallsAfterSession1 := organCalls.Load()

	if organCallsAfterSession1 == 0 {
		t.Fatal("session 1: organ was never called")
	}

	if _, err := agent.Think(ctx, "ping the organ"); err != nil {
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

	if _, err := agent.Think(context.Background(), "test regulator"); err != nil {
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

	if _, err := agent.Think(context.Background(), "test alignment"); err != nil {
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

	_, _ = agent.Think(ctx, "background task")

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

	if _, err := agent.Think(context.Background(), "ping it"); err != nil {
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

	_, _ = agent.Think(ctx, "ping repeatedly")

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

	_, _ = agent.Think(ctx, "do a long task")

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed")
	}

	response := m.Response()
	if response == "" || response == "exceeded max turns" {
		t.Errorf("graceful degradation should produce a synthesis response, got %q", response)
	}
}

func TestEventChain_PopulatedAfterOrganCall(t *testing.T) {
	ping, _ := pingOrgan()

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
	_, _ = agent.Think(context.Background(), "ping it")

	m := agent.Result()
	chain := m.Chain()

	if chain.Len() == 0 {
		t.Fatal("EventChain should have events after organ call")
	}

	senses := chain.Senses()
	if len(senses) == 0 {
		t.Error("expected at least one Sense event (ping is ReadAction)")
	}

	if senses[0].Organ != "ping" {
		t.Errorf("first sense organ = %q, want %q", senses[0].Organ, "ping")
	}

	if string(senses[0].Output) != "pong" {
		t.Errorf("sense output = %q, want %q", string(senses[0].Output), "pong")
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

	if _, err := agent.Think(ctx, "How should we approach adding authentication to the API?"); err != nil {
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

	if _, err := agent.Think(ctx, "What auth strategy should we use?"); err != nil {
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

// --- SDLC Drills: Plan Phase ---

func TestDrill_Plan_TaskBreakdown(t *testing.T) {
	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "analyzing the spec",
				ToolCalls: []tangle.ToolCall{
					{ID: "a1", Name: "assessment", Input: json.RawMessage(`{
						"taxonomy": "assessment.decomposition",
						"content": "The feature decomposes into 4 tasks: 1) Add API endpoint, 2) Write handler, 3) Add validation, 4) Write tests",
						"dimensions": ["tasks"]
					}`)},
				},
			},
			{
				Content: "Based on the spec, here are the 4 subtasks with estimates.",
			},
		},
	}

	bp := Blueprint{
		Model:  "stub",
		Budget: cerebrum.Budget{MaxTurns: 5, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	if _, err := agent.Think(context.Background(), "Break down this feature spec into tasks: Add user profile editing with avatar upload"); err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed")
	}
	if m.TotalMass() < 2 {
		t.Errorf("expected assessment atom for decomposition, got %d atoms", m.TotalMass())
	}
}

func TestDrill_Plan_BlastRadius(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "searching for callers",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: "Found 12 callers across 5 packages. The blast radius is medium.",
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 5, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	_, _ = agent.Think(context.Background(), "Estimate the blast radius of renaming the Process function")

	if organCalls.Load() == 0 {
		t.Error("agent should call grep/search organ to find callers")
	}
	if !agent.Result().Sealed() {
		t.Fatal("molecule should be sealed")
	}
}

// --- SDLC Drills: Code Phase ---

func TestDrill_Code_RefactorRename(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "searching for references",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: "renaming across files",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "ping", Input: json.RawMessage(`{"file":"a.go"}`)},
					{ID: "c3", Name: "ping", Input: json.RawMessage(`{"file":"b.go"}`)},
				},
			},
			{
				Content: "Renamed Process to Transform in 3 files.",
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	_, _ = agent.Think(context.Background(), "Rename the function Process to Transform across all files")

	if organCalls.Load() < 2 {
		t.Errorf("rename should call organs multiple times (search + edit), got %d", organCalls.Load())
	}
	if !agent.Result().Sealed() {
		t.Fatal("molecule should be sealed")
	}
}

func TestDrill_Code_DeadCodeDetection(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "searching for unused functions",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: "Found 2 unused functions: oldHelper and deprecatedProcess. Removing them.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "ping", Input: json.RawMessage(`{"action":"delete"}`)},
				},
			},
			{Content: "Removed 2 dead functions. Build passes."},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	_, _ = agent.Think(context.Background(), "Find and remove dead code in the utils package")

	if organCalls.Load() < 2 {
		t.Errorf("dead code detection needs search + delete, got %d calls", organCalls.Load())
	}
	if !agent.Result().Sealed() {
		t.Fatal("molecule should be sealed")
	}
}

func TestDrill_Code_WriteNewModule(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "creating the module",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{"action":"write","file":"cache.go"}`)},
				},
			},
			{
				Content: "creating the test",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "ping", Input: json.RawMessage(`{"action":"write","file":"cache_test.go"}`)},
				},
			},
			{
				Content: "running tests",
				ToolCalls: []tangle.ToolCall{
					{ID: "c3", Name: "ping", Input: json.RawMessage(`{"action":"test"}`)},
				},
			},
			{Content: "Created cache.go with LRU cache implementation and cache_test.go. Tests pass."},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	_, _ = agent.Think(context.Background(), "Write a new LRU cache module with tests")

	if organCalls.Load() < 3 {
		t.Errorf("new module needs write + write_test + test, got %d calls", organCalls.Load())
	}
	if !agent.Result().Sealed() {
		t.Fatal("molecule should be sealed")
	}
}

// --- SDLC Drills: Build Phase ---

func TestDrill_Build_VetLintFix(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "running vet",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{"action":"vet"}`)},
				},
			},
			{
				Content: "fixing violation",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "ping", Input: json.RawMessage(`{"action":"edit"}`)},
				},
			},
			{
				Content: "re-running vet",
				ToolCalls: []tangle.ToolCall{
					{ID: "c3", Name: "ping", Input: json.RawMessage(`{"action":"vet"}`)},
				},
			},
			{Content: "All vet/lint violations fixed. Clean build."},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	_, _ = agent.Think(context.Background(), "Run go vet, fix any violations, verify clean")

	if organCalls.Load() < 3 {
		t.Errorf("vet-fix-verify loop needs at least 3 calls, got %d", organCalls.Load())
	}
	if !agent.Result().Sealed() {
		t.Fatal("molecule should be sealed")
	}
}

// --- SDLC Drills: Test Phase ---

func TestDrill_Test_AddCoverage(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "reading the untested function",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{"action":"read"}`)},
				},
			},
			{
				Content: "writing the test",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "ping", Input: json.RawMessage(`{"action":"write_test"}`)},
				},
			},
			{
				Content: "running tests",
				ToolCalls: []tangle.ToolCall{
					{ID: "c3", Name: "ping", Input: json.RawMessage(`{"action":"test"}`)},
				},
			},
			{Content: "Added TestValidateEmail with 4 table-driven cases. All pass."},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	_, _ = agent.Think(context.Background(), "Add test coverage for the ValidateEmail function")

	if organCalls.Load() < 3 {
		t.Errorf("add coverage needs read + write + test, got %d calls", organCalls.Load())
	}
	if !agent.Result().Sealed() {
		t.Fatal("molecule should be sealed")
	}
}

// --- SDLC Drills: Monitor Phase ---

func TestDrill_Monitor_ArchHealth(t *testing.T) {
	ping, organCalls := pingOrgan()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "analyzing dependencies",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "ping", Input: json.RawMessage(`{"action":"grep_imports"}`)},
				},
			},
			{
				Content: "checking for violations",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "ping", Input: json.RawMessage(`{"action":"read_structure"}`)},
				},
			},
			{
				Content: "Architecture audit: 2 layer violations found (service→domain, handler→db). 3 packages with fan-in=0. Recommend merging orphan packages.",
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{ping},
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	_, _ = agent.Think(context.Background(), "Audit the architecture health: check for layer violations and orphan packages")

	if organCalls.Load() < 2 {
		t.Errorf("arch audit needs grep + read, got %d calls", organCalls.Load())
	}
	if !agent.Result().Sealed() {
		t.Fatal("molecule should be sealed")
	}
}

func TestDrill_Dialog_ReadThenSpeak(t *testing.T) {
	readAnimal := organ.Func{
		Name:        "read_animal",
		Description: "look up animal facts",
		Schema:      json.RawMessage(`{"type":"object","properties":{"animal":{"type":"string"}},"required":["animal"]}`),
		Mode:        organ.ReadAction,
		Source:      organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			var args struct{ Animal string `json:"animal"` }
			json.Unmarshal(input, &args)
			facts := map[string]string{
				"cow":     "Cows have four stomachs and produce milk.",
				"chicken": "Chickens can run up to 9 mph.",
			}
			if f, ok := facts[args.Animal]; ok {
				return organ.TextResult(f), nil
			}
			return organ.TextResult("Unknown animal."), nil
		},
	}

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				ToolCalls: []tangle.ToolCall{
					{ID: "r1", Name: "read_animal", Input: json.RawMessage(`{"animal":"cow"}`)},
				},
			},
			{
				Content: "Cows have four stomachs and produce milk.",
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{readAnimal},
		Budget:       cerebrum.Budget{MaxTurns: 5, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	_, _ = agent.Think(context.Background(), "Tell me about cows")

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should seal after read + text response")
	}

	chain := m.Chain()
	if chain.Len() == 0 {
		t.Fatal("EventChain should capture the read_animal call")
	}

	senses := chain.Senses()
	if len(senses) == 0 {
		t.Fatal("expected read_animal as a Sense event")
	}
	if senses[0].Organ != "read_animal" {
		t.Errorf("first sense organ = %q, want read_animal", senses[0].Organ)
	}
	if !containsSubstring(string(senses[0].Output), "four stomachs") {
		t.Errorf("sense output should contain cow facts, got: %s", string(senses[0].Output))
	}

	t.Logf("dialog: turns=%d chain=%d sealed=%v", m.Turns(), chain.Len(), m.Sealed())
}

func TestDrill_Dialog_MultiOrganPipeline(t *testing.T) {
	var callOrder []string

	readAnimal := organ.Func{
		Name:   "read_animal",
		Schema: json.RawMessage(`{"type":"object","properties":{"animal":{"type":"string"}},"required":["animal"]}`),
		Mode:   organ.ReadAction,
		Source: organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			var args struct{ Animal string `json:"animal"` }
			json.Unmarshal(input, &args)
			callOrder = append(callOrder, "read:"+args.Animal)
			return organ.TextResult(args.Animal + " facts here"), nil
		},
	}

	writeNote := organ.Func{
		Name:   "write_note",
		Schema: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"]}`),
		Mode:   organ.WriteAction,
		Source: organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			var args struct{ Content string `json:"content"` }
			json.Unmarshal(input, &args)
			callOrder = append(callOrder, "write:note")
			return organ.TextResult("saved"), nil
		},
	}

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				ToolCalls: []tangle.ToolCall{
					{ID: "r1", Name: "read_animal", Input: json.RawMessage(`{"animal":"cow"}`)},
				},
			},
			{
				ToolCalls: []tangle.ToolCall{
					{ID: "r2", Name: "read_animal", Input: json.RawMessage(`{"animal":"chicken"}`)},
				},
			},
			{
				ToolCalls: []tangle.ToolCall{
					{ID: "w1", Name: "write_note", Input: json.RawMessage(`{"content":"cow and chicken compared"}`)},
				},
			},
			{
				Content: "Comparison complete.",
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{readAnimal, writeNote},
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer)
	_, _ = agent.Think(context.Background(), "Compare cows and chickens, save a note")

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should seal")
	}

	chain := m.Chain()
	events := chain.All()
	if len(events) < 3 {
		t.Fatalf("expected 3+ chain events (read cow, read chicken, write note), got %d", len(events))
	}

	if events[0].Organ != "read_animal" || !containsSubstring(string(events[0].Input), "cow") {
		t.Errorf("event[0] should be read_animal(cow), got %s(%s)", events[0].Organ, string(events[0].Input))
	}
	if events[1].Organ != "read_animal" || !containsSubstring(string(events[1].Input), "chicken") {
		t.Errorf("event[1] should be read_animal(chicken), got %s(%s)", events[1].Organ, string(events[1].Input))
	}
	if events[2].Organ != "write_note" {
		t.Errorf("event[2] should be write_note, got %s", events[2].Organ)
	}

	senses := chain.Senses()
	motors := chain.Motors()
	if len(senses) != 2 {
		t.Errorf("expected 2 senses (reads), got %d", len(senses))
	}
	if len(motors) != 1 {
		t.Errorf("expected 1 motor (write), got %d", len(motors))
	}

	if len(callOrder) != 3 || callOrder[0] != "read:cow" || callOrder[1] != "read:chicken" || callOrder[2] != "write:note" {
		t.Errorf("organ call order = %v, want [read:cow read:chicken write:note]", callOrder)
	}

	t.Logf("pipeline: turns=%d chain=%d senses=%d motors=%d order=%v",
		m.Turns(), chain.Len(), len(senses), len(motors), callOrder)
}

func TestDrill_Dialog_DesiredFulfillmentSeals(t *testing.T) {
	readAnimal := organ.Func{
		Name:   "read_animal",
		Schema: json.RawMessage(`{"type":"object","properties":{"animal":{"type":"string"}},"required":["animal"]}`),
		Mode:   organ.ReadAction,
		Source: organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			return organ.TextResult("Cows produce milk."), nil
		},
	}

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				ToolCalls: []tangle.ToolCall{
					{ID: "r1", Name: "read_animal", Input: json.RawMessage(`{"animal":"cow"}`)},
				},
			},
			{
				ToolCalls: []tangle.ToolCall{
					{ID: "r2", Name: "read_animal", Input: json.RawMessage(`{"animal":"cow"}`)},
				},
			},
			{
				Content: "Cows produce milk.",
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{readAnimal},
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 5 * time.Second},
	}

	agent := Assemble(bp, completer,
		cerebrum.WithSealStrategy(cerebrum.ImmediateSeal{}),
	)

	catalyst := reactivity.Catalyst{
		Need:    "Do cows produce milk?",
		Desired: map[string]any{"answered": true},
	}
	_, _ = agent.Think(context.Background(), catalyst.Need)

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should seal after text response with ImmediateSeal")
	}

	chain := m.Chain()
	if chain.Len() < 2 {
		t.Errorf("expected 2+ chain events from multi-turn research, got %d", chain.Len())
	}

	if m.Turns() < 2 {
		t.Errorf("expected 2+ turns (tool calls then text), got %d", m.Turns())
	}

	t.Logf("desired: turns=%d chain=%d sealed=%v distance=%.2f",
		m.Turns(), chain.Len(), m.Sealed(), m.Distance())
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
