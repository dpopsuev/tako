package contracts

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/origami/engine"
)

// RunHubRoutingContract verifies that a Hub implementation correctly routes
// tools based on the active node. The factory must return a Hub with at least
// two nodes, each having at least one tool.
//
// Usage:
//
//	func TestMyHub_Routing(t *testing.T) {
//	    contracts.RunHubRoutingContract(t, func() (engine.Hub, string, string) {
//	        hub := NewMyHub(routingTable)
//	        return hub, "node-a", "node-b"
//	    })
//	}
func RunHubRoutingContract(t *testing.T, factory func() (engine.Hub, string, string)) {
	t.Helper()

	t.Run("NoActiveNode_EmptyTools", func(t *testing.T) {
		hub, _, _ := factory()
		if tools := hub.Tools(); len(tools) != 0 {
			t.Errorf("Tools() with no active node = %d tools, want 0", len(tools))
		}
	})

	t.Run("SetActiveNode_ReturnsTools", func(t *testing.T) {
		hub, nodeA, _ := factory()
		hub.SetActiveNode(nodeA)
		if hub.ActiveNode() != nodeA {
			t.Errorf("ActiveNode() = %q, want %q", hub.ActiveNode(), nodeA)
		}
		tools := hub.Tools()
		if len(tools) == 0 {
			t.Errorf("Tools() for node %q returned 0 tools, want >= 1", nodeA)
		}
	})

	t.Run("SwitchNode_ToolsRotate", func(t *testing.T) {
		hub, nodeA, nodeB := factory()
		hub.SetActiveNode(nodeA)
		toolsA := hub.Tools()

		hub.SetActiveNode(nodeB)
		toolsB := hub.Tools()

		if hub.ActiveNode() != nodeB {
			t.Errorf("ActiveNode() = %q, want %q", hub.ActiveNode(), nodeB)
		}

		// Tools must be different sets (different node → different tools).
		if len(toolsA) == 0 || len(toolsB) == 0 {
			t.Fatal("both nodes must have at least one tool")
		}
		if toolsA[0].Name() == toolsB[0].Name() {
			t.Errorf("tools did not rotate: node-a[0] = %q, node-b[0] = %q", toolsA[0].Name(), toolsB[0].Name())
		}
	})

	t.Run("UnknownNode_EmptyTools", func(t *testing.T) {
		hub, _, _ := factory()
		hub.SetActiveNode("nonexistent-node-xyz")
		if tools := hub.Tools(); len(tools) != 0 {
			t.Errorf("Tools() for unknown node = %d tools, want 0", len(tools))
		}
	})
}

// RunHubDispatchContract verifies that a Hub implementation correctly
// dispatches tool calls. The factory must return a Hub with at least one
// node that has a callable tool, the node name, and the tool name.
//
// Usage:
//
//	func TestMyHub_Dispatch(t *testing.T) {
//	    contracts.RunHubDispatchContract(t, func() (engine.Hub, string, string) {
//	        hub := NewMyHub(routingTable)
//	        return hub, "scan-node", "oculus_scan"
//	    })
//	}
func RunHubDispatchContract(t *testing.T, factory func() (engine.Hub, string, string)) {
	t.Helper()

	t.Run("Call_ReturnsResult", func(t *testing.T) {
		hub, node, toolName := factory()
		hub.SetActiveNode(node)

		result, err := hub.Call(context.Background(), toolName, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Call(%q): %v", toolName, err)
		}
		if result == "" {
			t.Error("Call() returned empty result")
		}
	})

	t.Run("Call_UnknownTool_Error", func(t *testing.T) {
		hub, node, _ := factory()
		hub.SetActiveNode(node)

		_, err := hub.Call(context.Background(), "nonexistent-tool-xyz", json.RawMessage(`{}`))
		if err == nil {
			t.Error("Call() for unknown tool should return error")
		}
	})

	t.Run("Call_NoActiveNode_Error", func(t *testing.T) {
		hub, _, toolName := factory()
		// No SetActiveNode call.
		_, err := hub.Call(context.Background(), toolName, json.RawMessage(`{}`))
		if err == nil {
			t.Error("Call() with no active node should return error")
		}
	})
}
