package assemble

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/capability"
	"github.com/dpopsuev/tako/shells/code"
	tangle "github.com/dpopsuev/tangle"
)

type subagentInput struct {
	Task       string `json:"task"`
	Type       string `json:"type"`
	MaxTurns   int    `json:"max_turns"`
}

type SubagentFactory struct {
	Root      string
	Completer tangle.Completer
}

func (f *SubagentFactory) Capability() capability.Capability {
	return capability.Capability{
		Name:        "spawn_agent",
		Description: "Spawn a child agent to handle a sub-task. Types: explore (read-only, fast), plan (read-only, deep), general (full capabilities). Returns the child's result.",
		Schema:      json.RawMessage(`{"type":"object","properties":{"task":{"type":"string","description":"Task for the child agent"},"type":{"type":"string","enum":["explore","plan","general"],"description":"Agent type (default: general)"},"max_turns":{"type":"integer","description":"Max turns for child (default: 10)"}},"required":["task"]}`),
		Mode:        capability.WriteAction,
		Risk:        0.3,
		Source:      capability.BuiltIn,
		Writes:      []string{"filesystem", "git"},
		Execute:     f.execute,
	}
}

func (f *SubagentFactory) execute(ctx context.Context, input json.RawMessage) (capability.Result, error) {
	var in subagentInput
	if err := json.Unmarshal(input, &in); err != nil {
		return capability.Result{}, fmt.Errorf("spawn_agent: %w", err)
	}
	if in.Task == "" {
		return capability.ErrorResult("spawn_agent: task required"), nil
	}

	agentType := in.Type
	if agentType == "" {
		agentType = "general"
	}

	maxTurns := in.MaxTurns
	if maxTurns == 0 {
		maxTurns = 10
	}

	caps := f.capsForType(agentType)

	bp := Blueprint{
		Model:        "child",
		Capabilities: caps,
		Budget: cerebrum.Budget{
			MaxTurns:    maxTurns,
			TurnTimeout: 30 * time.Second,
		},
	}

	child := Assemble(bp, f.Completer)

	slog.InfoContext(ctx, "subagent.spawn",
		slog.String("type", agentType),
		slog.String("task", truncateStr(in.Task, 80)),
		slog.Int("max_turns", maxTurns),
		slog.Int("capabilities", len(caps)))

	start := time.Now()
	err := child.Think(ctx, in.Task)
	elapsed := time.Since(start)

	if err != nil {
		slog.WarnContext(ctx, "subagent.error",
			slog.String("type", agentType),
			slog.Duration("elapsed", elapsed),
			slog.Any("error", err))
		return capability.ErrorResult(fmt.Sprintf("subagent failed: %s", err)), nil
	}

	m := child.Result()
	slog.InfoContext(ctx, "subagent.done",
		slog.String("type", agentType),
		slog.Duration("elapsed", elapsed),
		slog.Bool("sealed", m.Sealed()),
		slog.Float64("distance", m.Distance()),
		slog.Int("mass", m.TotalMass()))

	var result string
	retro := m.ByTaxonomy("retrospection.")
	if len(retro) > 0 {
		result = string(retro[len(retro)-1].Content)
	}
	if result == "" {
		result = fmt.Sprintf("subagent completed (type=%s, mass=%d, distance=%.2f)", agentType, m.TotalMass(), m.Distance())
	}

	return capability.TextResult(result), nil
}

func (f *SubagentFactory) capsForType(agentType string) []capability.Capability {
	all := code.Capabilities(f.Root)

	switch agentType {
	case "explore":
		var readOnly []capability.Capability
		for _, c := range all {
			if c.Mode == capability.ReadAction {
				readOnly = append(readOnly, c)
			}
		}
		return readOnly
	case "plan":
		var readOnly []capability.Capability
		for _, c := range all {
			if c.Mode == capability.ReadAction {
				readOnly = append(readOnly, c)
			}
		}
		return readOnly
	default:
		return all
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
