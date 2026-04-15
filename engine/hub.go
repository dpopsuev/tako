package engine

// Category: Execution — instrument hub (node-bound tool routing).

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/battery/tool"

	"github.com/dpopsuev/origami/circuit"
)

// Hub is a node-bound tool router for instrument dispatch.
//
// In headful mode (CLI), an agent connects to the hub as a single MCP server.
// In headless mode (API), the engine reads Tools() and injects them into
// CompletionParams. Same instruments, two dispatch surfaces.
//
// The routing table maps circuit node names to instrument tools. When the
// walker enters a node, SetActiveNode updates which tools are visible.
type Hub interface {
	// SetActiveNode changes the active node. Tools() returns only the
	// tools bound to this node.
	SetActiveNode(name string)

	// ActiveNode returns the currently active node name.
	ActiveNode() string

	// Tools returns the tools available for the active node.
	// Returns nil if no node is active or no tools are bound.
	Tools() []tool.Tool

	// Call dispatches a tool call to the correct instrument.
	// Returns the tool result as a string, or an error.
	Call(ctx context.Context, name string, input json.RawMessage) (string, error)
}

// HubRoute binds a circuit node to a set of instrument tools.
type HubRoute struct {
	Node  string      // circuit node name
	Tools []tool.Tool // tools available when this node is active
}

// HubRoutingTable maps node names to their instrument tools.
type HubRoutingTable map[string][]tool.Tool

// BuildHubRoutingTable constructs a routing table from a circuit definition
// and instrument registry. Each node that references a manifest-based
// instrument gets tools for its action.
func BuildHubRoutingTable(def *circuit.CircuitDef, instruments ManifestRegistry) HubRoutingTable {
	table := make(HubRoutingTable)
	for i := range def.Nodes {
		nd := &def.Nodes[i]
		name := string(nd.Name)
		instrument := nd.Instrument
		if instrument == "" {
			continue
		}
		manifest, ok := instruments[instrument]
		if !ok {
			continue // inproc instrument — no hub tools
		}

		actionName := nd.Action
		if actionName == "" {
			actionName = name
		}
		action, err := manifest.Action(actionName)
		if err != nil {
			continue
		}

		toolName := instrument + "_" + actionName
		t := &instrumentTool{
			name:        toolName,
			description: manifest.Description,
			inputSchema: schemaOrDefault(action.InputSchema),
			dispatcher:  nil, // wired by Hub implementation
			manifest:    manifest,
			actionName:  actionName,
		}
		table[name] = append(table[name], t)
	}
	return table
}

// instrumentTool adapts an instrument action to the battery/tool.Tool interface.
type instrumentTool struct {
	name        string
	description string
	inputSchema json.RawMessage
	dispatcher  InstrumentDispatcher
	manifest    *circuit.InstrumentManifest
	actionName  string
}

func (t *instrumentTool) Name() string                 { return t.name }
func (t *instrumentTool) Description() string          { return t.description }
func (t *instrumentTool) InputSchema() json.RawMessage { return t.inputSchema }

func (t *instrumentTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	if t.dispatcher == nil {
		return tool.Result{}, ErrInstrumentDispatch
	}
	output, err := t.dispatcher.Dispatch(ctx, input)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.TextResult(string(output)), nil
}

// LocalHub is the in-process Hub implementation. It routes tool calls
// through real InstrumentDispatchers (ExecDispatcher for cli, etc.).
type LocalHub struct {
	routes     HubRoutingTable
	activeNode string
	toolIndex  map[string]tool.Tool // flat index: tool-name → tool (across all nodes)
}

// compile-time check.
var _ Hub = (*LocalHub)(nil)

// NewLocalHub creates a Hub from a routing table, wiring dispatchers for
// each instrument tool. The workDir is used for exec dispatch.
func NewLocalHub(def *circuit.CircuitDef, instruments ManifestRegistry, workDir string) (*LocalHub, error) {
	table := BuildHubRoutingTable(def, instruments)

	// Wire dispatchers into instrumentTools.
	for _, tools := range table {
		for _, t := range tools {
			it, ok := t.(*instrumentTool)
			if !ok || it.dispatcher != nil {
				continue
			}
			action, err := it.manifest.Action(it.actionName)
			if err != nil {
				continue
			}
			d, err := CreateDispatcher(it.manifest, action, workDir)
			if err != nil {
				return nil, err
			}
			it.dispatcher = d
		}
	}

	// Build flat tool index for Call().
	idx := make(map[string]tool.Tool)
	for _, tools := range table {
		for _, t := range tools {
			idx[t.Name()] = t
		}
	}

	return &LocalHub{
		routes:    table,
		toolIndex: idx,
	}, nil
}

func (h *LocalHub) SetActiveNode(name string) { h.activeNode = name }
func (h *LocalHub) ActiveNode() string        { return h.activeNode }

func (h *LocalHub) Tools() []tool.Tool {
	return h.routes[h.activeNode]
}

func (h *LocalHub) Call(ctx context.Context, name string, input json.RawMessage) (string, error) {
	if h.activeNode == "" {
		return "", fmt.Errorf("%w: no active node", ErrInstrument)
	}

	// Look up tool in active node's tools first.
	for _, t := range h.routes[h.activeNode] {
		if t.Name() == name {
			result, execErr := t.Execute(ctx, input)
			if execErr != nil {
				return "", execErr
			}
			return result.Text(), nil
		}
	}

	return "", fmt.Errorf("%w: tool %q not found for node %q", ErrInstrument, name, h.activeNode)
}

func schemaOrDefault(schema string) json.RawMessage {
	if schema == "" {
		return json.RawMessage(`{"type":"object"}`)
	}
	return json.RawMessage(schema)
}
