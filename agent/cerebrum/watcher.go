package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

var classifyToolSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"priority":   {"type": "string", "enum": ["ignore", "park", "interrupt", "emergency"]},
		"dimensions": {"type": "array", "items": {"type": "string"}},
		"action":     {"type": "string", "description": "target molecule ID or reflex to fire"}
	},
	"required": ["priority", "dimensions"]
}`)

var classifyTool = tangle.Tool{
	Name:        "classify",
	Description: "Classify an incoming event: priority (ignore/park/interrupt/emergency), which state dimensions it affects, and what action to take.",
	InputSchema: classifyToolSchema,
}

type classifyResult struct {
	Priority   string   `json:"priority"`
	Dimensions []string `json:"dimensions"`
	Action     string   `json:"action"`
}

// EXPERIMENTAL: WatcherClassifier uses LLM for classification — vs Cynefin state-based (TSK-436)
type WatcherClassifier struct {
	Watcher  tangle.Completer
	Reflex   ReflexStore
	Embedder Embedder
}

func (w *WatcherClassifier) Classify(event Event, m *reactivity.Molecule) Priority {
	if w.Reflex != nil && w.Embedder != nil {
		embedding, err := w.Embedder.Embed(context.Background(), string(event.Payload))
		if err == nil {
			_, overlap := w.Reflex.Match(embedding)
			if overlap >= 0.95 {
				slog.Info("watcher.reflex_hit",
					slog.String("event", event.Kind.String()),
					slog.Float64("overlap", overlap))
				return PriorityPark
			}
		}
	}

	if w.Watcher == nil {
		return defaultClassifierImpl{}.Classify(event, m)
	}

	prompt := fmt.Sprintf(
		"Event: kind=%s source=%s\nMolecule: phase=%s distance=%.2f turns=%d\nClassify this event.",
		event.Kind, event.Source, m.Phase(), m.Distance(), m.Turns(),
	)

	completion, err := w.Watcher.Complete(context.Background(), tangle.CompletionParams{
		Messages: []tangle.Message{{Role: RoleUser, Content: prompt}},
		Tools:    []tangle.Tool{classifyTool},
	})
	if err != nil {
		slog.Warn("watcher.classify_error", slog.Any("error", err))
		return PriorityPark
	}

	for _, tc := range completion.ToolCalls {
		if tc.Name != "classify" {
			continue
		}
		var cr classifyResult
		if err := json.Unmarshal(tc.Input, &cr); err != nil {
			slog.Warn("watcher.classify_parse_error", slog.Any("error", err))
			return PriorityPark
		}

		slog.Info("watcher.classified",
			slog.String("event", event.Kind.String()),
			slog.String("priority", cr.Priority),
			slog.Any("dimensions", cr.Dimensions),
			slog.String("action", cr.Action))

		switch cr.Priority {
		case "ignore":
			return PriorityIgnore
		case "park":
			return PriorityPark
		case "interrupt":
			return PriorityInterrupt
		case "emergency":
			return PriorityEmergency
		default:
			return PriorityPark
		}
	}

	return PriorityPark
}
