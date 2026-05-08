package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

type sealResult struct {
	Turns    int
	Sealed   bool
	Response string
}

func runWithStrategy(t *testing.T, strategy SealStrategy, completer tangle.Completer, catalyst reactivity.Catalyst, caps []organ.Func) sealResult {
	t.Helper()
	cfg := reactivity.DefaultConfig
	reactor := reactivity.NewReactor(reactivity.WithNavigator(
		reactivity.NewTreeNavigator(&cfg),
	))

	sensory := NewChannelBus(64)
	motor := &autoExecMotor{
		caps:    make(map[string]organ.Func),
		sensory: sensory,
	}
	for _, c := range caps {
		motor.caps[c.Name] = c
	}

	cb := New(reactor, completer,
		WithSensory(sensory),
		WithMotor(motor),
		WithCapabilities(caps),
		WithConfig(&cfg),
		WithMaxTurns(10),
		WithTurnTimeout(5*time.Second),
		WithSealStrategy(strategy),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cb.Think(ctx, catalyst)
	m := cb.Result()

	return sealResult{
		Turns:    m.Turns(),
		Sealed:   m.Sealed(),
		Response: m.Response(),
	}
}

func pingCap() organ.Func {
	return organ.Func{
		Name:        "ping",
		Description: "returns pong",
		Schema:      json.RawMessage(`{"type":"object","properties":{}}`),
		Mode:        organ.ReadAction,
		Source:      organ.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
			return organ.TextResult("pong"), nil
		},
	}
}

func conversationCompleter() *stubCompleter {
	return &stubCompleter{response: "Hello! How can I help?"}
}

func singleToolCompleter() *stubCompleter {
	return &stubCompleter{
		response: "done",
		toolCalls: []tangle.ToolCall{
			{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
		},
		toolCallOnce: true,
	}
}

func multiStepCompleter() *stubCompleter {
	return &stubCompleter{
		response: "working on it",
		toolCalls: []tangle.ToolCall{
			{ID: "c1", Name: "ping", Input: json.RawMessage(`{}`)},
		},
	}
}

func stuckCompleter() *stubCompleter {
	return &stubCompleter{response: "I'm thinking..."}
}

type scenario struct {
	name      string
	completer tangle.Completer
	catalyst  reactivity.Catalyst
	caps      []organ.Func
	wantMax   int
}

func TestSealStrategy_Matrix(t *testing.T) {
	scenarios := []scenario{
		{
			name:      "conversation",
			completer: conversationCompleter(),
			catalyst:  reactivity.Catalyst{Need: "hello"},
			caps:      nil,
			wantMax:   2,
		},
		{
			name:      "single_tool",
			completer: singleToolCompleter(),
			catalyst:  reactivity.Catalyst{Need: "ping it"},
			caps:      []organ.Func{pingCap()},
			wantMax:   3,
		},
		{
			name:      "multi_step_with_desired",
			completer: multiStepCompleter(),
			catalyst: reactivity.Catalyst{
				Need:    "change greeting and verify",
				Desired: map[string]any{"greeting": "world", "tests": true},
			},
			caps:    []organ.Func{pingCap()},
			wantMax: 10,
		},
		{
			name:      "stuck_no_desired",
			completer: stuckCompleter(),
			catalyst:  reactivity.Catalyst{Need: "do something"},
			caps:      nil,
			wantMax:   3,
		},
	}

	strategies := []struct {
		name    string
		factory func() SealStrategy
	}{
		{"immediate", func() SealStrategy { return ImmediateSeal{} }},
		{"consecutive", func() SealStrategy { return &ConsecutiveSeal{} }},
		{"stagnant", func() SealStrategy { return &StagnantSeal{} }},
	}

	type cell struct {
		turns  int
		sealed bool
	}

	results := make(map[string]map[string]cell)

	for _, strat := range strategies {
		results[strat.name] = make(map[string]cell)
		for _, sc := range scenarios {
			t.Run(fmt.Sprintf("%s/%s", strat.name, sc.name), func(t *testing.T) {
				r := runWithStrategy(t, strat.factory(), sc.completer, sc.catalyst, sc.caps)
				results[strat.name][sc.name] = cell{turns: r.Turns, sealed: r.Sealed}

				if !r.Sealed {
					t.Error("molecule should be sealed")
				}
				if r.Turns > sc.wantMax {
					t.Errorf("turns=%d exceeds max=%d for this scenario", r.Turns, sc.wantMax)
				}
			})
		}
	}

	t.Run("summary", func(t *testing.T) {
		t.Logf("\n%-14s | %-14s | %-14s | %-14s | %-14s",
			"strategy", "conversation", "single_tool", "multi_step", "stuck")
		t.Logf("%-14s-+-%-14s-+-%-14s-+-%-14s-+-%-14s",
			"--------------", "--------------", "--------------", "--------------", "--------------")
		for _, strat := range strategies {
			conv := results[strat.name]["conversation"]
			tool := results[strat.name]["single_tool"]
			multi := results[strat.name]["multi_step_with_desired"]
			stuck := results[strat.name]["stuck_no_desired"]
			t.Logf("%-14s | %-14s | %-14s | %-14s | %-14s",
				strat.name,
				fmt.Sprintf("%d turns", conv.turns),
				fmt.Sprintf("%d turns", tool.turns),
				fmt.Sprintf("%d turns", multi.turns),
				fmt.Sprintf("%d turns", stuck.turns))
		}
	})
}
