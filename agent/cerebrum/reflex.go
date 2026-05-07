package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/capability"
	"github.com/dpopsuev/tako/agent/reactivity"
)

type ReflexStore interface {
	Match(embedding []float64) (*Pipe, float64)
	Add(pipe Pipe) error
	Merge(embedding []float64, steps []PipeStep) bool
	Prune(minScore float64) int
	Save() error
	Len() int
}

type ReplayResult struct {
	StepsTotal    int
	StepsReflex   int
	EscalatedAt   int
	EscalatedGear Gear
	Response      string
}

func ReplayPipe(ctx context.Context, pipe *Pipe, caps map[string]capability.Capability) (ReplayResult, error) {
	pipe.Usage++
	pipe.LastPlayed = time.Now()

	exec := NewPipeExecutor()
	runID, pr := exec.StartWithPipe(*pipe)

	result := ReplayResult{EscalatedAt: -1}
	for {
		step, _, err := exec.NextStepFromPipe(runID, pr.steps)
		if err != nil {
			return result, err
		}
		if step == nil {
			break
		}
		result.StepsTotal++

		cap, ok := caps[step.Call]
		if !ok || cap.Execute == nil {
			exec.SubmitAndUnlock(runID, step.ID, nil, "unknown capability: "+step.Call, pr.steps)
			result.EscalatedAt = result.StepsTotal - 1
			result.EscalatedGear = GearNovel
			break
		}

		out, err := cap.Execute(ctx, argsToJSON(step.Args))
		if err != nil {
			exec.SubmitAndUnlock(runID, step.ID, nil, err.Error(), pr.steps)
			result.EscalatedAt = result.StepsTotal - 1
			result.EscalatedGear = GearFamiliar
			break
		}

		actual := HashResult(out.Text())
		emptyHash := [32]byte{}
		if step.Expected != emptyHash && actual != step.Expected && step.Confidence < 0.5 {
			slog.InfoContext(ctx, "reflex.drift",
				slog.String("step", step.ID),
				slog.String("action", step.Call),
				slog.Float64("confidence", step.Confidence))
			result.EscalatedAt = result.StepsTotal - 1
			result.EscalatedGear = GearFamiliar
			break
		}

		exec.SubmitAndUnlock(runID, step.ID, string(out.Text()), "", pr.steps)
		result.StepsReflex++
		result.Response = string(out.Text())
	}

	if result.EscalatedAt == -1 {
		pipe.Replays++
	}
	return result, nil
}

func selectGear(overlap float64) Gear {
	switch {
	case overlap >= 0.95:
		return GearReflex
	case overlap >= 0.7:
		return GearIntuition
	case overlap >= 0.3:
		return GearFamiliar
	default:
		return GearNovel
	}
}

func suggestionAtom(pipe *Pipe, overlap float64, turn int) reactivity.Atom {
	content := fmt.Sprintf("intuition suggests replay of %s (overlap=%.0f%%)", pipe.Name, overlap*100)
	return reactivity.Atom{
		ID:        fmt.Sprintf("suggestion-turn-%d", turn),
		Type:      reactivity.KnowledgeAtom,
		Source:    reactivity.Recollected,
		Taxonomy:  "knowledge.suggestion.intuition",
		Content:   []byte(content),
		CreatedAt: time.Now(),
	}
}

func fireReflex(ctx context.Context, caps []capability.Capability, overlap float64) {
	for _, cap := range caps {
		if cap.Execute == nil {
			continue
		}
		result, err := cap.Execute(ctx, json.RawMessage(`""`))
		if err != nil {
			slog.WarnContext(ctx, "reflex.error",
				slog.String("capability", cap.Name),
				slog.Any("error", err))
			continue
		}
		slog.InfoContext(ctx, "reflex.fired",
			slog.String("capability", cap.Name),
			slog.Float64("overlap", overlap),
			slog.Int("result_len", len(result.Text())))
	}
}
