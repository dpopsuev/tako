package rehearsal

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/assemble"
	"github.com/dpopsuev/tako/shells/code"
	"github.com/dpopsuev/tako/testkit/arcade"
	tangle "github.com/dpopsuev/tangle"
)

type scriptedCompleter struct {
	turns []tangle.Completion
	call  int
}

func (s *scriptedCompleter) Complete(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
	if s.call >= len(s.turns) {
		return &tangle.Completion{Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"done"}]}`}, nil
	}
	c := s.turns[s.call]
	s.call++
	return &c, nil
}

func TestE2E_FixTheTest_Rehearsal(t *testing.T) {
	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "Let me find the failing test.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "grep", Input: json.RawMessage(`{"pattern":"func TestHandler_EmptySecret","path":"."}`)},
				},
			},
			{
				Content: "Found it. Reading the handler.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "read_file", Input: json.RawMessage(`{"path":"auth/handler.go"}`)},
				},
			},
			{
				Content: "The bug is that empty secret matches empty token. Fixing.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c3", Name: "edit", Input: json.RawMessage(`{"path":"auth/handler.go","old_string":"func (h *Handler) Validate(token string) bool {\n\treturn token == h.secret\n}","new_string":"func (h *Handler) Validate(token string) bool {\n\tif h.secret == \"\" {\n\t\treturn false\n\t}\n\treturn token == h.secret\n}"}`)},
				},
			},
			{
				Content: "Running tests.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c4", Name: "go_test", Input: json.RawMessage(`{"package":"./..."}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"Fixed the empty secret validation bug. All tests pass."}]}`,
			},
		},
	}

	workspace := SetupWorkspace(t, WithGitRepo(), WithFailingTest())

	bp := assemble.Blueprint{
		Model:        "stub",
		Capabilities: code.Capabilities(workspace),
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 30 * time.Second},
	}
	agent := assemble.Assemble(bp, completer)
	actor := func(ctx context.Context, prompt string) (string, error) {
		err := agent.Think(ctx, prompt)
		if err != nil {
			return "", err
		}
		return extractResult(agent.Result()), nil
	}

	runner, err := NewRunBuilder().
		WithScenario(NewStubScenario("fix-test", "Fix the failing test in auth/handler_failing_test.go")).
		WithReferee(&GoTestReferee{}).
		WithWorkspace(workspace).
		WithActor(actor).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	metrics, err := runner.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !metrics.Pass {
		t.Errorf("should pass, score=%.2f", metrics.Score)
	}
	t.Logf("FixTheTest: pass=%v score=%.2f elapsed=%v", metrics.Pass, metrics.Score, metrics.TimeElapsed)
}

func TestE2E_Fridge_AsRehearsal(t *testing.T) {
	scenario := arcade.NewFridge()

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "I'll take food from the fridge.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "take", Input: json.RawMessage(`"eggs"`)},
				},
			},
			{
				Content: "Now turn on the stove.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "turn_on_stove", Input: json.RawMessage(`""`)},
				},
			},
			{
				Content: "Cook the food.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c3", Name: "cook", Input: json.RawMessage(`"eggs"`)},
				},
			},
			{
				Content: "Eat from the plate.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c4", Name: "eat", Input: json.RawMessage(`""`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"Fed the tako."}]}`,
			},
		},
	}

	sensory := cerebrum.NewChannelBus(64)
	scenario.Adventure.WithSensory(sensory)

	gameActor := func(_ string) (ActorFunc, error) {
		bp := assemble.Blueprint{
			Model:        "stub",
			Capabilities: scenario.Adventure.Capabilities(),
			Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 10 * time.Second},
		}
		agent := assemble.Assemble(bp, completer)
		return func(ctx context.Context, prompt string) (string, error) {
			err := agent.Think(ctx, prompt)
			if err != nil {
				return "", err
			}
			return extractResult(agent.Result()), nil
		}, nil
	}

	gameReferee := &GameReferee{
		IsSolved: scenario.IsSolved,
		State:    scenario.Adventure.State,
	}

	runner, err := NewRunBuilder().
		WithScenario(NewStubScenario("fridge", scenario.Need)).
		WithReferee(gameReferee).
		WithActorFactory(gameActor).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	metrics, err := runner.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	t.Logf("Fridge as Rehearsal: pass=%v score=%.2f elapsed=%v", metrics.Pass, metrics.Score, metrics.TimeElapsed)
}
