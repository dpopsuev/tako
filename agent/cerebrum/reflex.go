package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/organ"
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

type StepResult struct {
	Call   string
	Output string
}

type ReplayResult struct {
	StepsTotal    int
	StepsReflex   int
	EscalatedAt   int
	EscalatedConventionality Conventionality
	Response      string
	Steps         []StepResult
}

func ReplayPipe(ctx context.Context, pipe *Pipe, caps map[string]organ.Func) (ReplayResult, error) {
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
			slog.InfoContext(ctx, "reflex.step.unknown",
				slog.Int("step", result.StepsTotal),
				slog.String("call", step.Call))
			exec.SubmitAndUnlock(runID, step.ID, nil, "unknown capability: "+step.Call, pr.steps)
			result.EscalatedAt = result.StepsTotal - 1
			result.EscalatedConventionality = ConventionalityChaotic
			break
		}

		inputJSON := argsToJSON(step.Args)
		slog.InfoContext(ctx, "reflex.step.exec",
			slog.Int("step", result.StepsTotal),
			slog.String("call", step.Call),
			slog.String("input", string(inputJSON)))

		out, err := cap.Execute(ctx, inputJSON)
		if err != nil {
			slog.WarnContext(ctx, "reflex.step.error",
				slog.Int("step", result.StepsTotal),
				slog.String("call", step.Call),
				slog.Any("error", err))
			exec.SubmitAndUnlock(runID, step.ID, nil, err.Error(), pr.steps)
			result.EscalatedAt = result.StepsTotal - 1
			result.EscalatedConventionality = ConventionalityComplex
			break
		}

		outText := string(out.Text())
		slog.InfoContext(ctx, "reflex.step.result",
			slog.Int("step", result.StepsTotal),
			slog.String("call", step.Call),
			slog.String("result", truncateStr(outText, 100)))

		actual := HashResult(out.Text())
		emptyHash := [32]byte{}
		if step.Expected != emptyHash && actual != step.Expected && step.Confidence < 0.5 {
			slog.InfoContext(ctx, "reflex.step.drift",
				slog.Int("step", result.StepsTotal),
				slog.String("call", step.Call),
				slog.Float64("confidence", step.Confidence))
			result.EscalatedAt = result.StepsTotal - 1
			result.EscalatedConventionality = ConventionalityComplex
			break
		}

		exec.SubmitAndUnlock(runID, step.ID, outText, "", pr.steps)
		result.StepsReflex++
		result.Response = outText
		result.Steps = append(result.Steps, StepResult{Call: step.Call, Output: outText})
	}

	if result.EscalatedAt == -1 {
		pipe.Replays++
	}
	return result, nil
}

func selectConventionality(overlap float64) Conventionality {
	switch {
	case overlap >= 0.95:
		return ConventionalityClear
	case overlap >= 0.7:
		return ConventionalityComplicated
	case overlap >= 0.3:
		return ConventionalityComplex
	default:
		return ConventionalityChaotic
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

func fireReflex(ctx context.Context, caps []organ.Func, overlap float64) {
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
