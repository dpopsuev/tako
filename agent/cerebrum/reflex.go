package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/shell"
)

type ReflexStore interface {
	Match(residual map[string]float64) ([]shell.Capability, float64)
}

type ReflexEntry struct {
	Pattern    map[string]float64
	Actions    []string
}

type InMemoryReflexStore struct {
	entries      []ReflexEntry
	capabilities []shell.Capability
}

func NewReflexStore(caps []shell.Capability) *InMemoryReflexStore {
	return &InMemoryReflexStore{capabilities: caps}
}

func (s *InMemoryReflexStore) AddReflex(pattern map[string]float64, actions []string) {
	s.entries = append(s.entries, ReflexEntry{Pattern: pattern, Actions: actions})
}

func (s *InMemoryReflexStore) Match(residual map[string]float64) ([]shell.Capability, float64) {
	if residual == nil || len(s.entries) == 0 {
		return nil, 0
	}

	var bestCaps []shell.Capability
	var bestOverlap float64

	for _, entry := range s.entries {
		overlap := computeOverlap(residual, entry.Pattern)
		if overlap > bestOverlap {
			bestOverlap = overlap
			bestCaps = s.resolveActions(entry.Actions)
		}
	}

	return bestCaps, bestOverlap
}

func computeOverlap(residual, pattern map[string]float64) float64 {
	if len(pattern) == 0 {
		return 0
	}
	matched := 0
	for k, v := range pattern {
		if rv, ok := residual[k]; ok && rv == v {
			matched++
		}
	}
	return float64(matched) / float64(len(pattern))
}

func (s *InMemoryReflexStore) resolveActions(names []string) []shell.Capability {
	var caps []shell.Capability
	for _, name := range names {
		for _, cap := range s.capabilities {
			if cap.Name == name {
				caps = append(caps, cap)
				break
			}
		}
	}
	return caps
}

func fireReflex(ctx context.Context, caps []shell.Capability, overlap float64) {
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

func selectGear(overlap float64) Gear {
	switch {
	case overlap >= 1.0:
		return GearReflex
	case overlap >= 0.7:
		return GearIntuition
	case overlap >= 0.3:
		return GearFamiliar
	default:
		return GearNovel
	}
}

func suggestionAtom(caps []shell.Capability, overlap float64, turn int) reactivity.Atom {
	var names []string
	for _, c := range caps {
		names = append(names, c.Name)
	}
	content := fmt.Sprintf("intuition suggests: %v (overlap=%.0f%%)", names, overlap*100)
	return reactivity.Atom{
		ID:        fmt.Sprintf("suggestion-turn-%d", turn),
		Type:      reactivity.KnowledgeAtom,
		Source:    reactivity.Recollected,
		Taxonomy:  "knowledge.suggestion.intuition",
		Content:   []byte(content),
		CreatedAt: time.Now(),
	}
}

