// Package scribeops provides Scribe-backed transformers for SDLC circuit nodes.
// Each transformer uses Battery's tool.Registry to call scribe.artifact via MCP.
// The registry is injected by the serve command — no direct MCP connections.
package scribeops

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/tako/engine/handler"
	"github.com/dpopsuev/tako/simulate/sdlc/sdlctype"
	"github.com/dpopsuev/tako/tool"
)

const scribeToolName = "scribe.artifact"

// scribeCall is a helper that executes a Scribe tool call via the registry.
func scribeCall(ctx context.Context, registry *tool.Registry, params map[string]any) (json.RawMessage, error) {
	input, _ := json.Marshal(params)
	result, err := registry.Execute(ctx, scribeToolName, input)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result.Text()), nil
}

// PollScribe queries Scribe for the highest-priority mature task in scope.
type PollScribe struct {
	registry *tool.Registry
}

// NewPollScribe creates a poll-scribe transformer.
func NewPollScribe(registry *tool.Registry) *PollScribe {
	return &PollScribe{registry: registry}
}

// Name implements handler.Instrument.
func (p *PollScribe) Name() string { return "poll-scribe" }

// Transform implements handler.Instrument.
func (p *PollScribe) Transform(ctx context.Context, tc *handler.InstrumentContext) (any, error) {
	scope, _ := tc.WalkerState.Context["scope"].(string)

	params := map[string]any{
		"action": "list",
		"status": "mature",
		"sort":   "priority",
		"kind":   "task",
		"fields": []string{"id", "title", "status", "priority"},
	}
	if scope != "" {
		params["scope"] = scope
	}

	raw, err := scribeCall(ctx, p.registry, params)
	if err != nil {
		return &sdlctype.PollScribeResult{HasTask: false}, nil
	}

	var items []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if json.Unmarshal(raw, &items) != nil || len(items) == 0 {
		return &sdlctype.PollScribeResult{HasTask: false}, nil
	}

	// Allocate the first task.
	_, _ = scribeCall(ctx, p.registry, map[string]any{
		"action": "set",
		"id":     items[0].ID,
		"field":  "status",
		"value":  "allocated",
	})

	return &sdlctype.PollScribeResult{
		HasTask: true,
		TaskID:  items[0].ID,
		Title:   items[0].Title,
	}, nil
}

// MarkDone sets a Scribe task status to complete.
type MarkDone struct {
	registry *tool.Registry
}

// NewMarkDone creates a mark-done transformer.
func NewMarkDone(registry *tool.Registry) *MarkDone {
	return &MarkDone{registry: registry}
}

// Name implements handler.Instrument.
func (m *MarkDone) Name() string { return "mark-done" }

// Transform implements handler.Instrument.
func (m *MarkDone) Transform(ctx context.Context, tc *handler.InstrumentContext) (any, error) {
	taskID := findTaskIDFromState(tc)
	if taskID == "" {
		return &sdlctype.MarkDoneResult{Updated: false}, nil
	}

	_, err := scribeCall(ctx, m.registry, map[string]any{
		"action": "set",
		"id":     taskID,
		"field":  "status",
		"value":  "complete",
	})
	if err != nil {
		return &sdlctype.MarkDoneResult{Updated: false}, nil
	}

	return &sdlctype.MarkDoneResult{Updated: true}, nil
}

// FileBug creates a Scribe bug artifact when a deployment fails.
type FileBug struct {
	registry *tool.Registry
}

// NewFileBug creates a file-bug transformer.
func NewFileBug(registry *tool.Registry) *FileBug {
	return &FileBug{registry: registry}
}

// Name implements handler.Instrument.
func (f *FileBug) Name() string { return "file-bug" }

// Transform implements handler.Instrument.
func (f *FileBug) Transform(ctx context.Context, tc *handler.InstrumentContext) (any, error) {
	scope, _ := tc.WalkerState.Context["scope"].(string)

	raw, err := scribeCall(ctx, f.registry, map[string]any{
		"action": "create",
		"kind":   "bug",
		"scope":  scope,
		"title":  fmt.Sprintf("deployment failure (circuit run %s)", tc.WalkerState.ID),
	})
	if err != nil {
		return &sdlctype.FileBugResult{BugID: ""}, nil
	}

	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &created)

	return &sdlctype.FileBugResult{BugID: created.ID}, nil
}

// GatePassthrough is a thin transformer for gate nodes (plan-review, diff-review).
// The approval gate mechanic does the real work — this just formats content.
type GatePassthrough struct {
	name string
}

// NewGatePassthrough creates a gate passthrough transformer.
func NewGatePassthrough(name string) *GatePassthrough {
	return &GatePassthrough{name: name}
}

// Name implements handler.Instrument.
func (g *GatePassthrough) Name() string { return g.name }

// Transform implements handler.Instrument.
func (g *GatePassthrough) Transform(_ context.Context, _ *handler.InstrumentContext) (any, error) {
	return &sdlctype.GateResult{Approved: true}, nil
}

// findTaskIDFromState searches walker state for a task_id from a prior node.
func findTaskIDFromState(tc *handler.InstrumentContext) string {
	if tc.WalkerState == nil {
		return ""
	}
	// Check context (set by poll-scribe via case context or delegate).
	if id, ok := tc.WalkerState.Context["task_id"].(string); ok && id != "" {
		return id
	}
	// Walk prior outputs for task_id field.
	for _, art := range tc.WalkerState.Outputs {
		if m, ok := art.Raw().(map[string]any); ok {
			if id, ok := m["task_id"].(string); ok && id != "" {
				return id
			}
		}
		if pr, ok := art.Raw().(*sdlctype.PollScribeResult); ok && pr.TaskID != "" {
			return pr.TaskID
		}
	}
	return ""
}

// Compile-time interface checks.
var (
	_ handler.Instrument = (*PollScribe)(nil)
	_ handler.Instrument = (*MarkDone)(nil)
	_ handler.Instrument = (*FileBug)(nil)
	_ handler.Instrument = (*GatePassthrough)(nil)
)
