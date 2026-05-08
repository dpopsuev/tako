package assemble

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/organs/code"
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

func (f *SubagentFactory) Capability() organ.Func {
	return organ.Func{
		Name:        "agent.spawn",
		Description: "Spawn a child agent to handle a sub-task. Types: explore (read-only, fast), plan (read-only, deep), general (full capabilities). Returns the child's result.",
		Schema:      json.RawMessage(`{"type":"object","properties":{"task":{"type":"string","description":"Task for the child agent"},"type":{"type":"string","enum":["explore","plan","general"],"description":"Agent type (default: general)"},"max_turns":{"type":"integer","description":"Max turns for child (default: 10)"}},"required":["task"]}`),
		Mode:        organ.WriteAction,
		Risk:        0.3,
		Source:      organ.BuiltIn,
		Writes:      []string{"filesystem", "git"},
		Execute:     f.execute,
	}
}

func (f *SubagentFactory) execute(ctx context.Context, input json.RawMessage) (organ.Result, error) {
	var in subagentInput
	if err := json.Unmarshal(input, &in); err != nil {
		return organ.Result{}, fmt.Errorf("spawn_agent: %w", err)
	}
	if in.Task == "" {
		return organ.ErrorResult("spawn_agent: task required"), nil
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
		return organ.ErrorResult(fmt.Sprintf("subagent failed: %s", err)), nil
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

	return organ.TextResult(result), nil
}

func (f *SubagentFactory) capsForType(agentType string) []organ.Func {
	all := code.Capabilities(f.Root)

	switch agentType {
	case "explore":
		var readOnly []organ.Func
		for _, c := range all {
			if c.Mode == organ.ReadAction {
				readOnly = append(readOnly, c)
			}
		}
		return readOnly
	case "plan":
		var readOnly []organ.Func
		for _, c := range all {
			if c.Mode == organ.ReadAction {
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
