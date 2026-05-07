package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	tangle "github.com/dpopsuev/tangle"
)

type Consolidator interface {
	Consolidate(ctx context.Context, need string, history []tangle.Message) error
}

type PipeConsolidator struct {
	Store    ReflexStore
	Embedder Embedder
}

func (c *PipeConsolidator) Consolidate(ctx context.Context, need string, history []tangle.Message) error {
	if c.Store == nil || c.Embedder == nil {
		return nil
	}

	steps := extractPipeSteps(history)
	if len(steps) == 0 {
		return nil
	}

	embedding, err := c.Embedder.Embed(ctx, need)
	if err != nil {
		slog.WarnContext(ctx, "consolidator.embed_error", slog.Any("error", err))
		return nil
	}

	if c.Store.Merge(embedding, steps) {
		slog.InfoContext(ctx, "consolidator.merged",
			slog.String("need", truncateStr(need, 50)),
			slog.Int("steps", len(steps)))
		return nil
	}

	pipe := Pipe{
		Name:       fmt.Sprintf("pipe-%d", time.Now().UnixNano()),
		Embedding:  embedding,
		Steps:      steps,
		Replays:    0,
		Usage:      1,
		LastPlayed: time.Now(),
	}

	if err := c.Store.Add(pipe); err != nil {
		slog.WarnContext(ctx, "consolidator.add_error", slog.Any("error", err))
		return nil
	}

	slog.InfoContext(ctx, "consolidator.promoted",
		slog.String("pipe", pipe.Name),
		slog.String("need", truncateStr(need, 50)),
		slog.Int("steps", len(steps)))

	return nil
}

func extractPipeSteps(history []tangle.Message) []PipeStep {
	var steps []PipeStep

	for i, msg := range history {
		if msg.Role != RoleAssistant || len(msg.ToolCalls) == 0 {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if isPhaseToolCall(tc.Name) {
				continue
			}

			var result []byte
			for j := i + 1; j < len(history); j++ {
				if history[j].Role == RoleTool && history[j].ToolCallID == tc.ID {
					result = []byte(history[j].Content)
					break
				}
			}

			var args map[string]any
			if len(tc.Input) > 0 {
				json.Unmarshal(tc.Input, &args)
			}
			step := PipeStep{
				ID:         tc.ID,
				Call:       tc.Name,
				Args:       args,
				Confidence: 0.6,
			}
			if len(result) > 0 {
				step.Expected = HashResult(result)
			}
			steps = append(steps, step)
		}
	}

	return steps
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
