package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/battery/tool"
)

// ScribeObserver implements Observer by polling Scribe for mature tasks.
// Replaces GitObserver for Scribe-driven development — drift = pending tasks exist.
type ScribeObserver struct {
	registry *tool.Registry
	toolName string // e.g. "scribe.artifact"
}

// NewScribeObserver creates an observer that polls Scribe for mature tasks.
// The registry must contain a "scribe.artifact" tool connected via MCPAdapter.
func NewScribeObserver(registry *tool.Registry) *ScribeObserver {
	return &ScribeObserver{
		registry: registry,
		toolName: "scribe.artifact",
	}
}

// scribeListResponse is the expected shape of artifact list output.
type scribeListResponse struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Kind     string `json:"kind"`
	Scope    string `json:"scope"`
}

// Observe queries Scribe for mature tasks and returns drift if any exist.
func (o *ScribeObserver) Observe() (*CurrentState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) //nolint:mnd // reasonable timeout
	defer cancel()

	input, _ := json.Marshal(map[string]any{
		"action": "list",
		"status": "mature",
		"sort":   "priority",
		"kind":   "task",
		"fields": []string{"id", "title", "status", "priority", "kind", "scope"},
	})

	result, err := o.registry.Execute(ctx, o.toolName, input)
	if err != nil {
		return nil, fmt.Errorf("scribe observe: %w", err)
	}

	// Parse the table output — Scribe returns either tabular text or JSON.
	// For structured consumption, parse as JSON array.
	var items []scribeListResponse
	_ = json.Unmarshal([]byte(result.Text()), &items)

	return &CurrentState{
		HeadSHA:      "", // not git-based
		ScanFindings: len(items),
		BuildPassing: true,
		TestPassing:  true,
		ObservedAt:   time.Now(),
	}, nil
}

var _ Observer = (*ScribeObserver)(nil)
