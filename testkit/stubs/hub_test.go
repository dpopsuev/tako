package stubs_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/tool"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/testkit/contracts"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func stubTool(name string) tool.Tool {
	return stubs.NewStubInstrumentTool(name, "test tool "+name)
}

func twoNodeHub() (hub engine.Hub, nodeA, nodeB string) {
	routes := engine.HubRoutingTable{
		"scan": {stubTool("oculus_scan")},
		"fix":  {stubTool("llm_fix")},
	}
	return stubs.NewStubHub(routes), "scan", "fix"
}

func oneNodeHub() (hub engine.Hub, node, toolName string) {
	routes := engine.HubRoutingTable{
		"scan": {stubTool("oculus_scan")},
	}
	return stubs.NewStubHub(routes), "scan", "oculus_scan"
}

func TestStubHub_RoutingContract(t *testing.T) {
	contracts.RunHubRoutingContract(t, twoNodeHub)
}

func TestStubHub_DispatchContract(t *testing.T) {
	contracts.RunHubDispatchContract(t, oneNodeHub)
}

func TestStubHub_ErrorInjection(t *testing.T) {
	hub := stubs.NewStubHub(engine.HubRoutingTable{
		"a": {stubTool("tool_a")},
	})
	hub.SetActiveNode("a")

	injected := errors.New("hub down")
	hub.SetError(injected)

	_, err := hub.Call(t.Context(), "tool_a", json.RawMessage(`{}`))
	if !errors.Is(err, injected) {
		t.Errorf("expected injected error, got %v", err)
	}
}

func TestStubHub_CallTracking(t *testing.T) {
	hub := stubs.NewStubHub(engine.HubRoutingTable{
		"a": {stubTool("tool_a")},
	})
	hub.SetActiveNode("a")
	hub.Call(t.Context(), "tool_a", json.RawMessage(`{"x":1}`))
	hub.Call(t.Context(), "tool_a", json.RawMessage(`{"x":2}`))

	if hub.CallCount() != 2 {
		t.Errorf("CallCount = %d, want 2", hub.CallCount())
	}
	calls := hub.Calls()
	if calls[0].ToolName != "tool_a" {
		t.Errorf("calls[0].ToolName = %q", calls[0].ToolName)
	}
}

func TestStubHub_Reset(t *testing.T) {
	hub := stubs.NewStubHub(engine.HubRoutingTable{
		"a": {stubTool("tool_a")},
	})
	hub.SetActiveNode("a")
	hub.SetError(errors.New("fail"))
	hub.Call(t.Context(), "tool_a", nil)
	hub.Reset()

	if hub.CallCount() != 0 {
		t.Errorf("CallCount after Reset = %d, want 0", hub.CallCount())
	}
	result, err := hub.Call(t.Context(), "tool_a", json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("unexpected error after Reset: %v", err)
	}
	if result == "" {
		t.Error("empty result after Reset")
	}
}
