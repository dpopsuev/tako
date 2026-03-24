package kami

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterMCPTools registers Kami debug and visualization tools on an MCP
// server. When dc is nil, debug/control tools (pause, resume, breakpoints,
// circuit state) are skipped — only visualization and Sumi tools are registered.
func RegisterMCPTools(mcpSrv *sdkmcp.Server, dc *DebugController, srv *Server) {
	if dc != nil {
		sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
			Name:        "kami_get_circuit_state",
			Description: "Get the current circuit state: running/paused, current node, visited nodes.",
		}, noOut(handleGetCircuitState(dc)))

		sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
			Name:        "kami_get_snapshot",
			Description: "Get a full circuit snapshot: state, breakpoints, visited nodes, artifacts.",
		}, noOut(handleGetSnapshot(dc)))

		sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
			Name:        "kami_get_assertions",
			Description: "Run all registered assertions and return results.",
		}, noOut(handleGetAssertions(dc)))

		sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
			Name:        "kami_pause",
			Description: "Pause circuit execution at the next node boundary.",
		}, noOut(handlePause(dc)))

		sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
			Name:        "kami_resume",
			Description: "Resume circuit execution from a paused state.",
		}, noOut(handleResume(dc)))

		sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
			Name:        "kami_advance_node",
			Description: "Step to the next node and pause again.",
		}, noOut(handleAdvanceNode(dc)))

		sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
			Name:        "kami_set_breakpoint",
			Description: "Set a breakpoint on a node. Execution pauses when the walk enters this node.",
		}, noOut(handleSetBreakpoint(dc)))

		sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
			Name:        "kami_clear_breakpoint",
			Description: "Clear a breakpoint from a node.",
		}, noOut(handleClearBreakpoint(dc)))
	}

	// Selection tools
	sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
		Name:        "kami_get_selection",
		Description: "Get the current browser element selection. Returns the list of highlighted UI elements selected by the user via CTRL+click.",
	}, noOut(handleGetSelection(srv)))

	// Visualization tools
	sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
		Name:        "kami_highlight_nodes",
		Description: "Highlight one or more nodes in the visualization.",
	}, noOut(handleHighlightNodes(srv)))

	sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
		Name:        "kami_highlight_zone",
		Description: "Highlight an entire zone in the visualization.",
	}, noOut(handleHighlightZone(srv)))

	sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
		Name:        "kami_zoom_to_zone",
		Description: "Zoom the visualization to a specific zone.",
	}, noOut(handleZoomToZone(srv)))

	sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
		Name:        "kami_place_marker",
		Description: "Place a labeled marker on a node.",
	}, noOut(handlePlaceMarker(srv)))

	sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
		Name:        "kami_clear_all",
		Description: "Clear all highlights, markers, and overlays.",
	}, noOut(handleClearAll(srv)))

	sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
		Name:        "kami_set_speed",
		Description: "Set the visualization playback speed multiplier.",
	}, noOut(handleSetSpeed(srv)))

	// Store management
	sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
		Name:        "kami_reset_store",
		Description: "Reset the circuit store, clearing all node states, walkers, and completion status. SSE clients receive a reset event.",
	}, noOut(handleResetStore(srv)))

	// Sumi TUI frame tool
	sdkmcp.AddTool(mcpSrv, &sdkmcp.Tool{
		Name:        "sumi_get_view",
		Description: "Get the latest Sumi TUI rendered frame. Returns the terminal text the user currently sees, plus metadata (timestamp, dimensions, selected node, focused panel, worker/event counts).",
	}, noOut(handleGetSumiView(srv)))

}

// noOut wraps a handler to suppress outputSchema and discard the structured
// content value. All Kami tools pack their response into CallToolResult text
// content; returning a non-nil Out (especially a plain string) causes MCP
// clients to fail with "expected record, received string" on structuredContent.
func noOut[In, Out any](h func(context.Context, *sdkmcp.CallToolRequest, In) (*sdkmcp.CallToolResult, Out, error)) sdkmcp.ToolHandlerFor[In, any] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input In) (*sdkmcp.CallToolResult, any, error) {
		res, _, err := h(ctx, req, input)
		return res, nil, err
	}
}

func jsonResult(v any) (*sdkmcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, nil
}

func textResult(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: msg}},
	}
}

// --- Read tool handlers ---

type emptyInput struct{}

func handleGetCircuitState(dc *DebugController) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, map[string]any, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, map[string]any, error) {
		snap := dc.Snapshot()
		result := map[string]any{
			"state":        snap.State,
			"current_node": snap.CurrentNode,
			"nodes_visited": snap.NodesVisited,
		}
		res, err := jsonResult(result)
		return res, result, err
	}
}

func handleGetSnapshot(dc *DebugController) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, CircuitSnapshot, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, CircuitSnapshot, error) {
		snap := dc.Snapshot()
		res, err := jsonResult(snap)
		return res, snap, err
	}
}

func handleGetAssertions(dc *DebugController) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, map[string]any, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, map[string]any, error) {
		errs := dc.RunAssertions()
		failures := make([]string, len(errs))
		for i, e := range errs {
			failures[i] = e.Error()
		}
		result := map[string]any{
			"total":    len(dc.Snapshot().Breakpoints),
			"failures": failures,
			"passed":   len(errs) == 0,
		}
		res, err := jsonResult(result)
		return res, result, err
	}
}

// --- Write tool handlers ---

func handlePause(dc *DebugController) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, string, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, string, error) {
		dc.Pause()
		return textResult("paused"), "paused", nil
	}
}

func handleResume(dc *DebugController) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, string, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, string, error) {
		dc.Resume()
		return textResult("resumed"), "resumed", nil
	}
}

func handleAdvanceNode(dc *DebugController) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, string, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, string, error) {
		dc.AdvanceNode()
		return textResult("advanced"), "advanced", nil
	}
}

type nodeInput struct {
	Node string `json:"node"`
}

func handleSetBreakpoint(dc *DebugController) func(context.Context, *sdkmcp.CallToolRequest, nodeInput) (*sdkmcp.CallToolResult, string, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input nodeInput) (*sdkmcp.CallToolResult, string, error) {
		if input.Node == "" {
			return nil, "", fmt.Errorf("node is required")
		}
		dc.SetBreakpoint(input.Node)
		return textResult(fmt.Sprintf("breakpoint set on %q", input.Node)), "ok", nil
	}
}

func handleClearBreakpoint(dc *DebugController) func(context.Context, *sdkmcp.CallToolRequest, nodeInput) (*sdkmcp.CallToolResult, string, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, input nodeInput) (*sdkmcp.CallToolResult, string, error) {
		if input.Node == "" {
			return nil, "", fmt.Errorf("node is required")
		}
		dc.ClearBreakpoint(input.Node)
		return textResult(fmt.Sprintf("breakpoint cleared on %q", input.Node)), "ok", nil
	}
}

// --- Selection tool handlers ---

func handleGetSelection(srv *Server) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, map[string]any, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, map[string]any, error) {
		sel := srv.GetSelection()
		if sel == nil {
			sel = map[string]any{"elements": []any{}}
		}
		res, err := jsonResult(sel)
		return res, sel, err
	}
}

// --- Visualization tool handlers ---

type highlightNodesInput struct {
	Nodes []string `json:"nodes"`
	Color string   `json:"color,omitempty"`
}

func handleHighlightNodes(srv *Server) func(context.Context, *sdkmcp.CallToolRequest, highlightNodesInput) (*sdkmcp.CallToolResult, string, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input highlightNodesInput) (*sdkmcp.CallToolResult, string, error) {
		msg := map[string]any{"action": "highlight_nodes", "nodes": input.Nodes, "color": input.Color}
		if err := srv.BroadcastWS(ctx, msg); err != nil {
			return nil, "", err
		}
		return textResult("highlighted"), "ok", nil
	}
}

type zoneInput struct {
	Zone  string `json:"zone"`
	Color string `json:"color,omitempty"`
}

func handleHighlightZone(srv *Server) func(context.Context, *sdkmcp.CallToolRequest, zoneInput) (*sdkmcp.CallToolResult, string, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input zoneInput) (*sdkmcp.CallToolResult, string, error) {
		msg := map[string]any{"action": "highlight_zone", "zone": input.Zone, "color": input.Color}
		if err := srv.BroadcastWS(ctx, msg); err != nil {
			return nil, "", err
		}
		return textResult("highlighted"), "ok", nil
	}
}

func handleZoomToZone(srv *Server) func(context.Context, *sdkmcp.CallToolRequest, zoneInput) (*sdkmcp.CallToolResult, string, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input zoneInput) (*sdkmcp.CallToolResult, string, error) {
		msg := map[string]any{"action": "zoom_to_zone", "zone": input.Zone}
		if err := srv.BroadcastWS(ctx, msg); err != nil {
			return nil, "", err
		}
		return textResult("zoomed"), "ok", nil
	}
}

type markerInput struct {
	Node  string `json:"node"`
	Label string `json:"label"`
	Color string `json:"color,omitempty"`
}

func handlePlaceMarker(srv *Server) func(context.Context, *sdkmcp.CallToolRequest, markerInput) (*sdkmcp.CallToolResult, string, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input markerInput) (*sdkmcp.CallToolResult, string, error) {
		msg := map[string]any{"action": "place_marker", "node": input.Node, "label": input.Label, "color": input.Color}
		if err := srv.BroadcastWS(ctx, msg); err != nil {
			return nil, "", err
		}
		return textResult("marker placed"), "ok", nil
	}
}

func handleClearAll(srv *Server) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, string, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, string, error) {
		msg := map[string]any{"action": "clear_all"}
		if err := srv.BroadcastWS(ctx, msg); err != nil {
			return nil, "", err
		}
		return textResult("cleared"), "ok", nil
	}
}

type speedInput struct {
	Speed float64 `json:"speed"`
}

func handleSetSpeed(srv *Server) func(context.Context, *sdkmcp.CallToolRequest, speedInput) (*sdkmcp.CallToolResult, string, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, input speedInput) (*sdkmcp.CallToolResult, string, error) {
		if input.Speed <= 0 {
			return nil, "", fmt.Errorf("speed must be positive")
		}
		msg := map[string]any{"action": "set_speed", "speed": input.Speed}
		if err := srv.BroadcastWS(ctx, msg); err != nil {
			return nil, "", err
		}
		return textResult(fmt.Sprintf("speed set to %.1fx", input.Speed)), "ok", nil
	}
}

func handleResetStore(srv *Server) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, string, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, string, error) {
		srv.ResetStore()
		return textResult("store reset"), "ok", nil
	}
}

func handleGetSumiView(srv *Server) func(context.Context, *sdkmcp.CallToolRequest, emptyInput) (*sdkmcp.CallToolResult, string, error) {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, string, error) {
		f := srv.frameStore.Latest()
		if f == nil {
			return textResult("Sumi not connected or no frames recorded."), "", nil
		}
		header := fmt.Sprintf(
			"Timestamp: %s\nDimensions: %dx%d\nLayout: %s\nSelected node: %s\nFocused panel: %s\nWorkers: %d\nEvents: %d\n---\n",
			f.Timestamp.Format("2006-01-02 15:04:05"),
			f.Width, f.Height,
			f.LayoutTier,
			f.SelectedNode,
			f.FocusedPanel,
			f.WorkerCount,
			f.EventCount,
		)
		return textResult(header + f.ViewText), "ok", nil
	}
}

