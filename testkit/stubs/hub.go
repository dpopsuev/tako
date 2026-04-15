package stubs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dpopsuev/battery/tool"

	"github.com/dpopsuev/origami/engine"
)

// compile-time check.
var _ engine.Hub = (*StubHub)(nil)

// StubHub implements engine.Hub with a configurable routing table.
// Thread-safe, supports error injection, tracks calls.
type StubHub struct {
	mu         sync.Mutex
	routes     engine.HubRoutingTable
	activeNode string
	calls      []HubCall
	err        error
}

// HubCall records a tool call made through the hub.
type HubCall struct {
	Node     string
	ToolName string
	Input    json.RawMessage
}

// NewStubHub creates a hub with the given routing table.
func NewStubHub(routes engine.HubRoutingTable) *StubHub {
	if routes == nil {
		routes = make(engine.HubRoutingTable)
	}
	return &StubHub{routes: routes}
}

func (h *StubHub) SetActiveNode(name string) {
	h.mu.Lock()
	h.activeNode = name
	h.mu.Unlock()
}

func (h *StubHub) ActiveNode() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.activeNode
}

func (h *StubHub) Tools() []tool.Tool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.routes[h.activeNode]
}

func (h *StubHub) Call(ctx context.Context, name string, input json.RawMessage) (string, error) {
	h.mu.Lock()
	node := h.activeNode
	err := h.err
	h.calls = append(h.calls, HubCall{Node: node, ToolName: name, Input: input})
	h.mu.Unlock()

	if err != nil {
		return "", err
	}
	if node == "" {
		return "", fmt.Errorf("%w: no active node", engine.ErrInstrument)
	}

	tools := h.routes[node]
	for _, t := range tools {
		if t.Name() == name {
			result, execErr := t.Execute(ctx, input)
			if execErr != nil {
				return "", execErr
			}
			return result.Text(), nil
		}
	}
	return "", fmt.Errorf("%w: tool %q not found for node %q", engine.ErrInstrument, name, node)
}

// SetError injects an error for all subsequent Call invocations.
func (h *StubHub) SetError(err error) {
	h.mu.Lock()
	h.err = err
	h.mu.Unlock()
}

// Calls returns the ordered list of tool calls made through the hub.
func (h *StubHub) Calls() []HubCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]HubCall, len(h.calls))
	copy(out, h.calls)
	return out
}

// CallCount returns how many times Call was invoked.
func (h *StubHub) CallCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.calls)
}

// Reset clears call tracking and injected errors.
func (h *StubHub) Reset() {
	h.mu.Lock()
	h.calls = nil
	h.err = nil
	h.mu.Unlock()
}
