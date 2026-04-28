package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/dpopsuev/tangle"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	errUnknownAgentAction = errors.New("unknown agent action")
	errPromptRequired     = errors.New("prompt is required for delegate action")
	errNoAgentsAvailable  = errors.New("no agents available")
)

// agentInput is the unified input for the "agent" tool.
type agentInput struct {
	Action string `json:"action" jsonschema:"required,enum=discover;delegate;status"`
	Role   string `json:"role,omitempty" jsonschema:"filter by role (discover, delegate)"`
	Prompt string `json:"prompt,omitempty" jsonschema:"prompt to delegate (delegate)"`
	Model  string `json:"model,omitempty" jsonschema:"model preference (delegate)"`
}

type agentCardInfo struct {
	Name   string   `json:"name"`
	Role   string   `json:"role"`
	Skills []string `json:"skills,omitempty"`
}

type agentDiscoverOutput struct {
	Agents []agentCardInfo `json:"agents"`
	Count  int             `json:"count"`
}

type agentDelegateOutput struct {
	Response string `json:"response"`
	AgentID  string `json:"agent_id"`
}

type agentStatusOutput struct {
	Agents []agentCardInfo `json:"agents"`
	Total  int             `json:"total"`
}

func cardsToInfo(cards []troupe.AgentCard) []agentCardInfo {
	out := make([]agentCardInfo, len(cards))
	for i, c := range cards {
		out[i] = agentCardInfo{Name: c.Name(), Role: c.Role(), Skills: c.Skills()}
	}
	return out
}

// registerAgentTool adds the "agent" MCP tool if a Broker is configured.
func (s *CircuitServer) registerAgentTool() {
	if s.Config.Broker == nil {
		return
	}

	s.MCPServer.AddTool(&sdkmcp.Tool{
		Name:        "agent",
		Description: "Agent coordination. Actions: discover (list agents), delegate (send prompt to agent), status (health summary).",
		InputSchema: map[string]any{"type": "object"},
	}, rawHandler(s.handleAgentDispatch))
}

func (s *CircuitServer) handleAgentDispatch(ctx context.Context, req *sdkmcp.CallToolRequest, input agentInput) (*sdkmcp.CallToolResult, agentDiscoverOutput, error) {
	switch input.Action {
	case "discover":
		return s.handleAgentDiscover(ctx, input)
	case "delegate":
		return s.handleAgentDelegate(ctx, input)
	case "status":
		return s.handleAgentStatus(ctx, input)
	default:
		return toolError(fmt.Errorf("%w: %q; valid: discover, delegate, status", errUnknownAgentAction, input.Action)), agentDiscoverOutput{}, nil
	}
}

func (s *CircuitServer) handleAgentDiscover(_ context.Context, input agentInput) (*sdkmcp.CallToolResult, agentDiscoverOutput, error) {
	cards := s.Config.Broker.Discover(input.Role)
	out := agentDiscoverOutput{
		Agents: cardsToInfo(cards),
		Count:  len(cards),
	}
	return nil, out, nil
}

func (s *CircuitServer) handleAgentDelegate(ctx context.Context, input agentInput) (*sdkmcp.CallToolResult, agentDiscoverOutput, error) {
	if input.Prompt == "" {
		return toolError(errPromptRequired), agentDiscoverOutput{}, nil
	}

	configs, err := s.Config.Broker.Pick(ctx, troupe.Preferences{
		Role:  input.Role,
		Model: input.Model,
		Count: 1,
	})
	if err != nil || len(configs) == 0 {
		return toolError(fmt.Errorf("%w for role %q", errNoAgentsAvailable, input.Role)), agentDiscoverOutput{}, nil
	}

	actor, err := s.Config.Broker.Spawn(ctx, configs[0])
	if err != nil {
		return toolError(fmt.Errorf("spawn failed: %w", err)), agentDiscoverOutput{}, nil
	}
	defer actor.Kill(ctx) //nolint:errcheck // best-effort cleanup

	response, err := actor.Perform(ctx, input.Prompt)
	if err != nil {
		return toolError(fmt.Errorf("perform failed: %w", err)), agentDiscoverOutput{}, nil
	}

	out := agentDelegateOutput{
		Response: response,
		AgentID:  configs[0].Role,
	}
	res, marshalErr := marshalToolResult(out)
	if marshalErr != nil {
		return toolError(marshalErr), agentDiscoverOutput{}, nil
	}
	return res, agentDiscoverOutput{}, nil
}

func (s *CircuitServer) handleAgentStatus(_ context.Context, input agentInput) (*sdkmcp.CallToolResult, agentDiscoverOutput, error) {
	cards := s.Config.Broker.Discover(input.Role)

	out := agentStatusOutput{
		Agents: cardsToInfo(cards),
		Total:  len(cards),
	}
	res, marshalErr := marshalToolResult(out)
	if marshalErr != nil {
		return toolError(marshalErr), agentDiscoverOutput{}, nil
	}
	return res, agentDiscoverOutput{}, nil
}
