package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/origami/tool"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/circuit/def"
)

func TestLocalHub_SetActiveNode_ToolsRotate(t *testing.T) {
	cdef := &circuit.CircuitDef{
		Circuit: "hub-test",
		Start:   "scan",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "scan", Instrument: "dummy-echo", Action: "echo"},
			{Name: "fix", Instrument: "dummy-fail", Action: "fail"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "scan-fix", From: "scan", To: "fix"},
			{ID: "fix-done", From: "fix", To: "_done"},
		},
	}

	instruments := ManifestRegistry{
		"dummy-echo": testHubManifest("dummy-echo", "echo"),
		"dummy-fail": testHubManifest("dummy-fail", "fail"),
	}

	hub, err := NewLocalHub(cdef, instruments, "")
	if err != nil {
		t.Fatalf("NewLocalHub: %v", err)
	}

	// No active node → no tools.
	if tools := hub.Tools(); len(tools) != 0 {
		t.Errorf("Tools() with no active node = %d, want 0", len(tools))
	}

	// Set scan → get scan tools.
	hub.SetActiveNode("scan")
	if hub.ActiveNode() != "scan" {
		t.Errorf("ActiveNode() = %q, want scan", hub.ActiveNode())
	}
	scanTools := hub.Tools()
	if len(scanTools) != 1 {
		t.Fatalf("Tools() for scan = %d, want 1", len(scanTools))
	}
	if scanTools[0].Name() != "dummy-echo_echo" {
		t.Errorf("scan tool name = %q, want dummy-echo_echo", scanTools[0].Name())
	}

	// Switch to fix → tools rotate.
	hub.SetActiveNode("fix")
	fixTools := hub.Tools()
	if len(fixTools) != 1 {
		t.Fatalf("Tools() for fix = %d, want 1", len(fixTools))
	}
	if fixTools[0].Name() != "dummy-fail_fail" {
		t.Errorf("fix tool name = %q, want dummy-fail_fail", fixTools[0].Name())
	}
}

func TestLocalHub_Call_UnknownTool(t *testing.T) {
	hub := localHubWithEcho(t)
	hub.SetActiveNode("scan")

	_, err := hub.Call(context.Background(), "nonexistent", json.RawMessage(`{}`))
	if err == nil {
		t.Error("Call() for unknown tool should return error")
	}
}

func TestLocalHub_Call_NoActiveNode(t *testing.T) {
	hub := localHubWithEcho(t)

	_, err := hub.Call(context.Background(), "dummy-echo_echo", json.RawMessage(`{}`))
	if err == nil {
		t.Error("Call() with no active node should return error")
	}
}

func TestLocalHub_InprocInstrument_Skipped(t *testing.T) {
	cdef := &circuit.CircuitDef{
		Circuit: "inproc-test",
		Start:   "scan",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "scan", Instrument: "transformer", Action: "llm"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "scan-done", From: "scan", To: "_done"},
		},
	}

	// No manifest for "transformer" — it's inproc.
	hub, err := NewLocalHub(cdef, ManifestRegistry{}, "")
	if err != nil {
		t.Fatalf("NewLocalHub: %v", err)
	}

	hub.SetActiveNode("scan")
	if tools := hub.Tools(); len(tools) != 0 {
		t.Errorf("inproc instrument should not produce hub tools, got %d", len(tools))
	}
}

func TestBuildHubRoutingTable_Basic(t *testing.T) {
	cdef := &circuit.CircuitDef{
		Circuit: "rt-test",
		Start:   "scan",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "scan", Instrument: "dummy-echo", Action: "echo"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "scan-done", From: "scan", To: "_done"},
		},
	}

	instruments := ManifestRegistry{
		"dummy-echo": testHubManifest("dummy-echo", "echo"),
	}

	table := BuildHubRoutingTable(cdef, instruments)
	if len(table["scan"]) != 1 {
		t.Errorf("routing table scan = %d tools, want 1", len(table["scan"]))
	}
}

func TestWalkWithHub_SetActiveNode(t *testing.T) {
	// Build a simple circuit and verify hub.SetActiveNode is called during walk.
	cdef := &circuit.CircuitDef{
		Circuit: "walk-hub-test",
		Start:   "greet",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "greet", Instrument: "transformer", Action: "passthrough"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "greet-done", From: "greet", To: "_done"},
		},
	}

	reg := &GraphRegistries{}
	g, err := BuildGraph(cdef, reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Attach a tracking hub via WithHub.
	tracker := &trackingHub{}
	dg := g.(*DefaultGraph)
	dg.hub = tracker

	walker := circuit.NewProcessWalker("test")
	if walkErr := g.Walk(context.Background(), walker, "greet"); walkErr != nil {
		t.Fatalf("Walk: %v", walkErr)
	}

	if len(tracker.nodes) == 0 {
		t.Error("hub.SetActiveNode was never called during walk")
	}
	if tracker.nodes[0] != "greet" {
		t.Errorf("first SetActiveNode = %q, want greet", tracker.nodes[0])
	}
}

// --- helpers ---

func testHubManifest(name, action string) *circuit.InstrumentManifest {
	return &circuit.InstrumentManifest{
		Kind:      circuit.KindInstrument,
		Name:      name,
		Namespace: "test",
		Dispatch:  circuit.DispatchCLI,
		Binary:    "echo",
		Tune:      "--version",
		Actions: map[string]def.ActionDef{
			action: {Command: "ok"},
		},
	}
}

func localHubWithEcho(t *testing.T) *LocalHub {
	t.Helper()
	cdef := &circuit.CircuitDef{
		Circuit: "echo-test",
		Start:   "scan",
		Done:    "_done",
		Nodes: []circuit.NodeDef{
			{Name: "scan", Instrument: "dummy-echo", Action: "echo"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "scan-done", From: "scan", To: "_done"},
		},
	}
	instruments := ManifestRegistry{
		"dummy-echo": testHubManifest("dummy-echo", "echo"),
	}
	hub, err := NewLocalHub(cdef, instruments, "")
	if err != nil {
		t.Fatalf("NewLocalHub: %v", err)
	}
	return hub
}

// trackingHub records SetActiveNode calls without doing anything else.
type trackingHub struct {
	nodes []string
}

func (h *trackingHub) SetActiveNode(name string) { h.nodes = append(h.nodes, name) }
func (h *trackingHub) ActiveNode() string {
	if len(h.nodes) == 0 {
		return ""
	}
	return h.nodes[len(h.nodes)-1]
}
func (h *trackingHub) Tools() []tool.Tool { return nil }
func (h *trackingHub) Call(_ context.Context, _ string, _ json.RawMessage) (string, error) {
	return "", nil
}
