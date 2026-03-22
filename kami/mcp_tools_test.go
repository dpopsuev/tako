package kami

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupMCPTest() (*DebugController, *Server) {
	bridge := NewEventBridge(nil)
	dc := NewDebugController(bridge)
	srv := NewServer(Config{Bridge: bridge})
	return dc, srv
}

func TestMCPTools_SetBreakpointThenSnapshot(t *testing.T) {
	dc, _ := setupMCPTest()

	dc.SetBreakpoint("triage")

	dc.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter,
		Node: "recall",
	})
	dc.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeExit,
		Node: "recall",
	})

	snap := dc.Snapshot()
	if snap.State != "running" {
		t.Errorf("state = %q, want running", snap.State)
	}
	if len(snap.NodesVisited) != 1 || snap.NodesVisited[0] != "recall" {
		t.Errorf("visited = %v, want [recall]", snap.NodesVisited)
	}
	if len(snap.Breakpoints) != 1 || snap.Breakpoints[0] != "triage" {
		t.Errorf("breakpoints = %v, want [triage]", snap.Breakpoints)
	}
}

func TestMCPTools_PauseViaHandler(t *testing.T) {
	dc, _ := setupMCPTest()

	handler := handlePause(dc)
	res, _, err := handler(context.Background(), nil, emptyInput{})
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if len(res.Content) == 0 {
		t.Fatal("empty result content")
	}
	tc, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected *TextContent, got %T", res.Content[0])
	}
	if tc.Text != "paused" {
		t.Errorf("text = %q, want paused", tc.Text)
	}

	if dc.State() != StatePaused {
		t.Errorf("state = %v, want paused", dc.State())
	}
}

func TestMCPTools_GetSnapshotHandler(t *testing.T) {
	dc, _ := setupMCPTest()

	dc.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter,
		Node: "recall",
	})

	handler := handleGetSnapshot(dc)
	res, snap, err := handler(context.Background(), nil, emptyInput{})
	if err != nil {
		t.Fatalf("get_snapshot: %v", err)
	}
	if snap.CurrentNode != "recall" {
		t.Errorf("current_node = %q, want recall", snap.CurrentNode)
	}

	tc := res.Content[0].(*sdkmcp.TextContent)
	var parsed CircuitSnapshot
	if err := json.Unmarshal([]byte(tc.Text), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.CurrentNode != "recall" {
		t.Errorf("parsed current_node = %q, want recall", parsed.CurrentNode)
	}
}

func TestMCPTools_SetBreakpointHandler(t *testing.T) {
	dc, _ := setupMCPTest()

	handler := handleSetBreakpoint(dc)
	_, _, err := handler(context.Background(), nil, nodeInput{Node: "investigate"})
	if err != nil {
		t.Fatalf("set_breakpoint: %v", err)
	}

	bps := dc.ListBreakpoints()
	if len(bps) != 1 || bps[0] != "investigate" {
		t.Errorf("breakpoints = %v, want [investigate]", bps)
	}

	_, _, err = handler(context.Background(), nil, nodeInput{})
	if err == nil {
		t.Error("expected error for empty node")
	}
}

func TestMCPTools_SetSpeedHandler(t *testing.T) {
	_, srv := setupMCPTest()

	handler := handleSetSpeed(srv)

	_, _, err := handler(context.Background(), nil, speedInput{Speed: 0})
	if err == nil {
		t.Error("expected error for zero speed")
	}

	_, _, err = handler(context.Background(), nil, speedInput{Speed: -1})
	if err == nil {
		t.Error("expected error for negative speed")
	}
}

func TestMCPTools_GetSelectionHandler(t *testing.T) {
	_, srv := setupMCPTest()

	// No selection yet
	handler := handleGetSelection(srv)
	res, _, err := handler(context.Background(), nil, emptyInput{})
	if err != nil {
		t.Fatalf("get_selection: %v", err)
	}
	tc := res.Content[0].(*sdkmcp.TextContent)
	var empty map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &empty); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	elems, ok := empty["elements"]
	if !ok {
		t.Fatal("missing elements key")
	}
	if arr, ok := elems.([]any); !ok || len(arr) != 0 {
		t.Errorf("expected empty elements, got %v", elems)
	}

	// Set selection and retrieve
	srv.SetSelection(map[string]any{
		"elements": []any{
			map[string]any{"type": "node", "id": "recall"},
		},
		"timestamp": "2026-02-25T10:00:00Z",
	})

	res, _, err = handler(context.Background(), nil, emptyInput{})
	if err != nil {
		t.Fatalf("get_selection after set: %v", err)
	}
	tc = res.Content[0].(*sdkmcp.TextContent)
	var sel map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &sel); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	elems = sel["elements"]
	arr, ok := elems.([]any)
	if !ok || len(arr) != 1 {
		t.Errorf("expected 1 element, got %v", elems)
	}
}

func TestMCPTools_RegistersAllTools(t *testing.T) {
	bridge := NewEventBridge(nil)
	dc := NewDebugController(bridge)
	srv := NewServer(Config{Bridge: bridge})
	mcpSrv := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "kami-test", Version: "0.0.0"},
		nil,
	)
	RegisterMCPTools(mcpSrv, dc, srv)

	ctx := context.Background()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := mcpSrv.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := []string{
		"kami_get_circuit_state", "kami_get_snapshot", "kami_get_assertions",
		"kami_pause", "kami_resume", "kami_advance_node",
		"kami_set_breakpoint", "kami_clear_breakpoint",
		"kami_get_selection",
		"kami_highlight_nodes", "kami_highlight_zone", "kami_zoom_to_zone",
		"kami_place_marker", "kami_clear_all", "kami_set_speed",
		"sumi_get_view",
	}
	registered := make(map[string]bool)
	for _, tool := range tools.Tools {
		registered[tool.Name] = true
	}
	for _, name := range want {
		if !registered[name] {
			t.Errorf("tool %q not registered (dc=non-nil)", name)
		}
	}
	t.Logf("all %d tools registered with dc=non-nil", len(want))
}

func connectKami(t *testing.T, mcpSrv *sdkmcp.Server) *sdkmcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	t1, t2 := sdkmcp.NewInMemoryTransports()
	serverSess, err := mcpSrv.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { serverSess.Close() })
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func TestMCPProtocol_SumiGetView_NoFrame(t *testing.T) {
	_, srv := setupMCPTest()
	mcpSrv := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "kami-test", Version: "0.0.0"},
		nil,
	)
	RegisterMCPTools(mcpSrv, nil, srv)
	session := connectKami(t, mcpSrv)

	res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "sumi_get_view",
	})
	if err != nil {
		t.Fatalf("CallTool(sumi_get_view): %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error")
	}
	tc, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected *TextContent, got %T", res.Content[0])
	}
	if !strings.Contains(tc.Text, "not connected") {
		t.Fatalf("expected 'not connected' message, got %q", tc.Text)
	}
}

func TestMCPProtocol_SumiGetView_WithFrame(t *testing.T) {
	_, srv := setupMCPTest()
	srv.frameStore.Store(sampleFrame())

	mcpSrv := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "kami-test", Version: "0.0.0"},
		nil,
	)
	RegisterMCPTools(mcpSrv, nil, srv)
	session := connectKami(t, mcpSrv)

	res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "sumi_get_view",
	})
	if err != nil {
		t.Fatalf("CallTool(sumi_get_view): %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error")
	}
	tc := res.Content[0].(*sdkmcp.TextContent)
	if !strings.Contains(tc.Text, "Selected node: triage") {
		t.Fatalf("missing selected node: %s", tc.Text)
	}
	if !strings.Contains(tc.Text, "[triage]") {
		t.Fatalf("missing view text: %s", tc.Text)
	}
}

func TestMCPProtocol_VisualizationTools_NoError(t *testing.T) {
	_, srv := setupMCPTest()
	mcpSrv := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "kami-test", Version: "0.0.0"},
		nil,
	)
	RegisterMCPTools(mcpSrv, nil, srv)
	session := connectKami(t, mcpSrv)

	tools := []struct {
		name string
		args map[string]any
	}{
		{"kami_get_selection", nil},
		{"kami_clear_all", nil},
		{"kami_set_speed", map[string]any{"speed": 2.0}},
	}

	for _, tc := range tools {
		res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name:      tc.name,
			Arguments: tc.args,
		})
		if err != nil {
			t.Errorf("CallTool(%s): %v", tc.name, err)
			continue
		}
		if res.IsError {
			for _, c := range res.Content {
				if txt, ok := c.(*sdkmcp.TextContent); ok {
					t.Errorf("CallTool(%s) error: %s", tc.name, txt.Text)
				}
			}
			continue
		}
		t.Logf("%s: OK", tc.name)
	}
}

func TestMCPTools_RegisterNilDC(t *testing.T) {
	srv := NewServer(Config{Bridge: NewEventBridge(nil)})
	mcpSrv := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "kami-test-nil-dc", Version: "0.0.0"},
		nil,
	)
	RegisterMCPTools(mcpSrv, nil, srv)

	ctx := context.Background()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := mcpSrv.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	registered := make(map[string]bool)
	for _, tool := range tools.Tools {
		registered[tool.Name] = true
	}

	mustPresent := []string{
		"kami_get_selection",
		"kami_highlight_nodes", "kami_highlight_zone", "kami_zoom_to_zone",
		"kami_place_marker", "kami_clear_all", "kami_set_speed",
		"sumi_get_view",
	}
	for _, name := range mustPresent {
		if !registered[name] {
			t.Errorf("tool %q should be registered even with dc=nil", name)
		}
	}

	mustAbsent := []string{
		"kami_get_circuit_state", "kami_get_snapshot", "kami_get_assertions",
		"kami_pause", "kami_resume", "kami_advance_node",
		"kami_set_breakpoint", "kami_clear_breakpoint",
	}
	for _, name := range mustAbsent {
		if registered[name] {
			t.Errorf("tool %q should NOT be registered when dc=nil", name)
		}
	}

	t.Logf("nil-dc: %d tools registered, %d debug tools correctly skipped",
		len(mustPresent), len(mustAbsent))
}
