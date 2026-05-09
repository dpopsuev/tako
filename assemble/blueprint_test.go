package assemble

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/organs/code"
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

func TestWalkingSkeleton_ReadFile(t *testing.T) {
	caps := code.Capabilities(".")

	var readCap bool
	for _, c := range caps {
		if c.Name == "file_read" {
			readCap = true
			break
		}
	}
	if !readCap {
		t.Fatal("read_file capability not found in organs/code")
	}

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "I'll read the blueprint.go file to understand the code.",
				ToolCalls: []tangle.ToolCall{
					{
						ID:    "call_1",
						Name:  "file_read",
						Input: json.RawMessage(`{"path":"blueprint.go"}`),
					},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"I read the file successfully"}]}`,
			},
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: caps,
		Budget: cerebrum.Budget{
			MaxTurns:    5,
			TurnTimeout: 10 * time.Second,
		},
	}

	agent := Assemble(bp, completer)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := agent.Think(ctx, "read the blueprint.go file")
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	m := agent.Result()
	if !m.Sealed() {
		t.Error("molecule should be sealed")
	}

	if completer.call < 2 {
		t.Errorf("expected at least 2 completer calls, got %d", completer.call)
	}
}

func TestAssemble_Capabilities(t *testing.T) {
	caps := code.Capabilities(".")

	bp := Blueprint{
		Model:        "stub",
		Capabilities: caps,
	}

	completer := &scriptedCompleter{}
	agent := Assemble(bp, completer)

	corpCaps := agent.corpus.Capabilities()
	names := corpCaps.Names()

	if len(names) < 5 {
		t.Errorf("expected at least 5 capabilities, got %d: %v", len(names), names)
	}

	found := false
	for _, n := range names {
		if n == "file_read" {
			found = true
		}
	}
	if !found {
		t.Error("read_file not registered on Corpus")
	}
}

func TestAssemble_EmptyCapabilities_Warns(t *testing.T) {
	bp := Blueprint{
		Model:        "stub",
		Capabilities: nil,
	}

	completer := &scriptedCompleter{}
	agent := Assemble(bp, completer)

	if agent.cerebrum == nil {
		t.Error("Cerebrum should not be nil even with empty capabilities")
	}
}

func TestAssemble_CustomConfig(t *testing.T) {
	cfg := reactivity.DefaultConfig
	cfg.DistanceClose = 0.1

	bp := Blueprint{
		Model:        "stub",
		Capabilities: code.Capabilities("."),
		Config:       &cfg,
	}

	completer := &scriptedCompleter{}
	agent := Assemble(bp, completer)

	if agent.cerebrum == nil {
		t.Error("Cerebrum should not be nil")
	}
}

func TestWalkingSkeleton_ToolResultReachesLLM(t *testing.T) {
	caps := code.Capabilities(".")

	var secondCallMessages []tangle.Message

	completer := &capturingCompleter{
		turns: []tangle.Completion{
			{
				Content: "reading file",
				ToolCalls: []tangle.ToolCall{
					{ID: "call_1", Name: "file_read", Input: json.RawMessage(`{"path":"blueprint.go"}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"done"}]}`,
			},
		},
		onCall: func(call int, params tangle.CompletionParams) {
			if call == 1 {
				secondCallMessages = params.Messages
			}
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: caps,
		Budget:       cerebrum.Budget{MaxTurns: 5, TurnTimeout: 10 * time.Second},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	agent.Think(ctx, "read blueprint.go")

	foundToolResult := false
	for _, msg := range secondCallMessages {
		if msg.Role == "tool" {
			foundToolResult = true
			break
		}
	}

	if !foundToolResult {
		t.Error("tool result should reach the LLM on the second call")
		for i, msg := range secondCallMessages {
			t.Logf("  msg[%d] role=%s len=%d", i, msg.Role, len(msg.Content))
		}
	}
}

func TestClosedCircuit_OrganExecutesAndResultReturns(t *testing.T) {
	var organCalled bool
	pingOrgan := organ.Func{
		Name:        "ping",
		Description: "returns pong",
		Schema:      json.RawMessage(`{"type":"object","properties":{}}`),
		Mode:        organ.ReadAction,
		Source:      organ.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
			organCalled = true
			return organ.TextResult("pong"), nil
		},
	}

	var toolResultContent string
	completer := &capturingCompleter{
		turns: []tangle.Completion{
			{
				Content: "pinging",
				ToolCalls: []tangle.ToolCall{
					{ID: "call_ping", Name: "ping", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"got pong"}]}`,
			},
		},
		onCall: func(call int, params tangle.CompletionParams) {
			if call == 1 {
				for _, msg := range params.Messages {
					if msg.Role == cerebrum.RoleTool && msg.ToolCallID == "call_ping" {
						toolResultContent = msg.Content
					}
				}
			}
		},
	}

	bp := Blueprint{
		Model:        "stub",
		Capabilities: []organ.Func{pingOrgan},
		Budget:       cerebrum.Budget{MaxTurns: 5, TurnTimeout: 10 * time.Second},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := agent.Think(ctx, "ping the organ"); err != nil {
		t.Fatalf("Think: %v", err)
	}

	if !organCalled {
		t.Fatal("organ.Func.Execute was never called — motor bus did not route the event")
	}

	if toolResultContent != "pong" {
		t.Errorf("tool result = %q, want %q — sensory bus did not return the result", toolResultContent, "pong")
	}

	m := agent.Result()
	if !m.Sealed() {
		t.Fatal("molecule should be sealed")
	}

	if completer.call < 2 {
		t.Errorf("expected at least 2 LLM calls (tool call + seal), got %d", completer.call)
	}
}

type capturingCompleter struct {
	turns  []tangle.Completion
	call   int
	onCall func(int, tangle.CompletionParams)
}

func (c *capturingCompleter) Complete(_ context.Context, params tangle.CompletionParams) (*tangle.Completion, error) {
	if c.onCall != nil {
		c.onCall(c.call, params)
	}
	if c.call >= len(c.turns) {
		return &tangle.Completion{Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"done"}]}`}, nil
	}
	comp := c.turns[c.call]
	c.call++
	return &comp, nil
}
