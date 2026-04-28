// Package enrichment provides context enrichment transformers that use
// Battery tools (Lex, Locus) to augment task context with coding rules
// and architecture information.
package enrichment

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/dpopsuev/tako/engine/handler"
	"github.com/dpopsuev/tako/simulate/sdlc/sdlctype"
	"github.com/dpopsuev/tako/tool"
)

const (
	lexToolName   = "lex.lexicon"
	locusToolName = "locus.analysis"
	logKeyError   = "error"
)

// ResolveContext enriches task context with Lex coding rules and Locus
// architecture analysis. Uses Battery tools — degrades gracefully if
// either service is unavailable.
type ResolveContext struct {
	registry *tool.Registry
}

// NewResolveContext creates a resolve-context transformer.
func NewResolveContext(registry *tool.Registry) *ResolveContext {
	return &ResolveContext{registry: registry}
}

// Name implements handler.Instrument.
func (r *ResolveContext) Name() string { return "resolve-context" }

// Transform implements handler.Instrument.
func (r *ResolveContext) Transform(ctx context.Context, tc *handler.InstrumentContext) (any, error) {
	result := &sdlctype.ResolveContextResult{
		Spec: make(map[string]any),
	}

	// Copy task info from prior poll-scribe output if available.
	if tc.WalkerState != nil {
		for _, art := range tc.WalkerState.Outputs {
			if pr, ok := art.Raw().(*sdlctype.PollScribeResult); ok {
				result.Spec["task_id"] = pr.TaskID
				result.Spec["title"] = pr.Title
			}
		}
	}

	// Resolve Lex rules.
	if rules := r.resolveRules(ctx); len(rules) > 0 {
		result.Rules = rules
	}

	// Resolve Locus architecture.
	if arch := r.resolveArchitecture(ctx); arch != nil {
		result.Architecture = arch
	}

	return result, nil
}

func (r *ResolveContext) resolveRules(ctx context.Context) []string {
	if _, err := r.registry.Get(lexToolName); err != nil {
		return nil
	}

	input, _ := json.Marshal(map[string]any{
		"action":   "resolve",
		"language": "go",
		"budget":   2000,
	})

	raw, err := r.registry.Execute(ctx, lexToolName, input)
	if err != nil {
		slog.WarnContext(ctx, "Lex resolve failed", slog.String(logKeyError, err.Error()))
		return nil
	}

	var resolved struct {
		Rules []struct {
			Name string `json:"name"`
		} `json:"rules"`
	}
	if json.Unmarshal([]byte(raw.Text()), &resolved) != nil {
		return nil
	}

	names := make([]string, len(resolved.Rules))
	for i, r := range resolved.Rules {
		names[i] = r.Name
	}
	return names
}

func (r *ResolveContext) resolveArchitecture(ctx context.Context) map[string]any {
	if _, err := r.registry.Get(locusToolName); err != nil {
		return nil
	}

	input, _ := json.Marshal(map[string]any{
		"action": "preset",
		"preset": "architecture_review",
		"format": "json",
	})

	raw, err := r.registry.Execute(ctx, locusToolName, input)
	if err != nil {
		slog.WarnContext(ctx, "Locus analysis failed", slog.String(logKeyError, err.Error()))
		return nil
	}

	var arch map[string]any
	if json.Unmarshal([]byte(raw.Text()), &arch) != nil {
		return nil
	}
	return arch
}

var _ handler.Instrument = (*ResolveContext)(nil)
