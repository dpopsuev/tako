package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/origami/engine"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// instrumentInput is the unified input for the "instrument" tool.
type instrumentInput struct {
	Action string          `json:"action" jsonschema:"required,enum=list;invoke"`
	Name   string          `json:"name,omitempty" jsonschema:"instrument name (invoke)"`
	Params json.RawMessage `json:"params,omitempty" jsonschema:"parameters for the instrument action"`
}

type instrumentListOutput struct {
	Instruments []instrumentInfo `json:"instruments"`
	Count       int              `json:"count"`
}

type instrumentInfo struct {
	Name     string   `json:"name"`
	Dispatch string   `json:"dispatch"`
	Actions  []string `json:"actions"`
}

type instrumentInvokeOutput struct {
	Name   string          `json:"name"`
	Result json.RawMessage `json:"result"`
}

// registerInstrumentTool adds the "instrument" MCP tool if instruments are configured.
func (s *CircuitServer) registerInstrumentTool() {
	if len(s.Config.Instruments) == 0 {
		return
	}

	s.MCPServer.AddTool(&sdkmcp.Tool{
		Name:        "instrument",
		Description: "Invoke instruments on-demand outside circuits. Actions: list (available instruments), invoke (run an instrument action).",
		InputSchema: map[string]any{"type": "object"},
	}, rawHandler(s.handleInstrumentDispatch))
}

func (s *CircuitServer) handleInstrumentDispatch(_ context.Context, _ *sdkmcp.CallToolRequest, input instrumentInput) (*sdkmcp.CallToolResult, any, error) {
	switch input.Action {
	case actionList:
		return s.handleInstrumentList()
	case "invoke":
		return s.handleInstrumentInvoke(input)
	default:
		return toolError(fmt.Errorf("%w: %q; valid actions: list, invoke", ErrUnknownInstrumentAction, input.Action)), nil, nil
	}
}

func (s *CircuitServer) handleInstrumentList() (*sdkmcp.CallToolResult, any, error) {
	infos := make([]instrumentInfo, 0, len(s.Config.Instruments))
	for name, manifest := range s.Config.Instruments {
		var actions []string
		for actionName := range manifest.Actions {
			actions = append(actions, actionName)
		}
		infos = append(infos, instrumentInfo{
			Name:     name,
			Dispatch: string(manifest.Dispatch),
			Actions:  actions,
		})
	}

	out := instrumentListOutput{Instruments: infos, Count: len(infos)}
	data, _ := json.Marshal(out)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, out, nil
}

func (s *CircuitServer) handleInstrumentInvoke(input instrumentInput) (*sdkmcp.CallToolResult, any, error) {
	manifest, ok := s.Config.Instruments[input.Name]
	if !ok {
		return toolError(fmt.Errorf("%w: %q", engine.ErrInstrument, input.Name)), nil, nil
	}

	// Use the first action if none specified in params.
	var actionName string
	for name := range manifest.Actions {
		actionName = name
		break
	}

	action, err := manifest.Action(actionName)
	if err != nil {
		return toolError(err), nil, nil
	}

	dispatcher, err := engine.CreateDispatcher(manifest, action, s.Config.InstrumentDir)
	if err != nil {
		return toolError(err), nil, nil
	}

	params := input.Params
	if len(params) == 0 {
		params = json.RawMessage(`{}`)
	}

	result, err := dispatcher.Dispatch(context.Background(), params)
	if err != nil {
		return toolError(err), nil, nil
	}

	out := instrumentInvokeOutput{Name: input.Name, Result: result}
	data, _ := json.Marshal(out)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, out, nil
}
