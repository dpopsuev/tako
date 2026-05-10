package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/organs/code"
)

type input struct {
	Task     string `json:"task"`
	Type     string `json:"type"`
	MaxTurns int    `json:"max_turns"`
}

type SpawnFunc func(ctx context.Context, caps []organ.Func, task string, maxTurns int) (string, error)

type Factory struct {
	Root  string
	Spawn SpawnFunc
}

func (f *Factory) Organ() organ.Func {
	return organ.Func{
		Name:        "agent_spawn",
		Description: "Spawn a child agent to handle a sub-task. Types: explore (read-only, fast), plan (read-only, deep), general (full capabilities). Returns the child's result.",
		Schema:      json.RawMessage(`{"type":"object","properties":{"task":{"type":"string","description":"Task for the child agent"},"type":{"type":"string","enum":["explore","plan","general"],"description":"Agent type (default: general)"},"max_turns":{"type":"integer","description":"Max turns for child (default: 10)"}},"required":["task"]}`),
		Mode:        organ.WriteAction,
		Risk:        0.3,
		Source:      organ.BuiltIn,
		Writes:      []string{"filesystem", "git"},
		Execute:     f.execute,
	}
}

func (f *Factory) execute(ctx context.Context, raw json.RawMessage) (organ.Result, error) {
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
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

	slog.InfoContext(ctx, "subagent.spawn",
		slog.String("type", agentType),
		slog.String("task", truncate(in.Task, 80)),
		slog.Int("max_turns", maxTurns),
		slog.Int("organs", len(caps)))

	start := time.Now()
	result, err := f.Spawn(ctx, caps, in.Task, maxTurns)
	elapsed := time.Since(start)

	if err != nil {
		slog.WarnContext(ctx, "subagent.error",
			slog.String("type", agentType),
			slog.Duration("elapsed", elapsed),
			slog.Any("error", err))
		return organ.ErrorResult(fmt.Sprintf("subagent failed: %s", err)), nil
	}

	slog.InfoContext(ctx, "subagent.done",
		slog.String("type", agentType),
		slog.Duration("elapsed", elapsed))

	if result == "" {
		result = fmt.Sprintf("subagent completed (type=%s)", agentType)
	}

	return organ.TextResult(result), nil
}

func (f *Factory) capsForType(agentType string) []organ.Func {
	all := code.Organs(f.Root)

	switch agentType {
	case "explore", "plan":
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
