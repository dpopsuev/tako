package assemble

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/organs/code"
	tangle "github.com/dpopsuev/tangle"
)

func TestSubagent_SpawnAndComplete(t *testing.T) {
	parentCompleter := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "I'll delegate this to a subagent.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "agent.spawn", Input: json.RawMessage(`{"task":"read the go.mod file","type":"explore","max_turns":3}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"subagent completed the task"}]}`,
			},
		},
	}

	caps := code.Capabilities(".")
	bp := Blueprint{
		Model:        "stub",
		Capabilities: caps,
		Budget:       cerebrum.Budget{MaxTurns: 5, TurnTimeout: 10 * time.Second},
	}

	agent := Assemble(bp, parentCompleter)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := agent.Think(ctx, "delegate reading go.mod to a subagent")
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	if parentCompleter.call < 2 {
		t.Errorf("expected at least 2 parent completer calls, got %d", parentCompleter.call)
	}
}

func TestSubagent_ExploreIsReadOnly(t *testing.T) {
	factory := &SubagentFactory{
		Root:      ".",
		Completer: &scriptedCompleter{},
	}

	caps := factory.capsForType("explore")
	for _, c := range caps {
		if c.Mode != 0 {
			t.Errorf("explore subagent should only have ReadAction capabilities, got %s with mode %d", c.Name, c.Mode)
		}
	}
}
