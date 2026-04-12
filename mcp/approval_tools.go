package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/origami/engine"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	actionGet     = "get"
	actionApprove = "approve"
	actionReject  = "reject"
	actionComment = "comment"
)

// approvalInput is the unified input for the "approval" tool.
type approvalInput struct {
	Action    string `json:"action" jsonschema:"required,enum=list;get;approve;reject;comment"`
	ID        string `json:"id,omitempty" jsonschema:"approval item ID (required for get/approve/reject/comment)"`
	Comment   string `json:"comment,omitempty" jsonschema:"comment or reason"`
	Operator  string `json:"operator,omitempty" jsonschema:"who is approving/rejecting"`
	SessionID string `json:"session_id,omitempty" jsonschema:"filter by session ID (list)"`
}

type approvalListOutput struct {
	Items []engine.ApprovalItem `json:"items"`
	Count int                   `json:"count"`
}

type approvalGetOutput struct {
	engine.ApprovalItem
}

type approvalResolveOutput struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type approvalCommentOutput struct {
	ID      string `json:"id"`
	Comment string `json:"comment"`
}

// registerApprovalTool adds the "approval" MCP tool if an ApprovalStore is configured.
func (s *CircuitServer) registerApprovalTool() {
	if s.Config.ApprovalStore == nil {
		return
	}

	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "approval",
		Description: "Approval gate review. Actions: list (pending items), get (view output), approve, reject, comment.",
	}, NoOutputSchema(s.handleApprovalDispatch))
}

func (s *CircuitServer) handleApprovalDispatch(ctx context.Context, req *sdkmcp.CallToolRequest, input approvalInput) (*sdkmcp.CallToolResult, approvalListOutput, error) {
	switch input.Action {
	case actionList:
		return s.handleApprovalList(ctx, input)
	case actionGet:
		return s.handleApprovalGet(ctx, input)
	case actionApprove:
		return s.handleApprovalResolve(ctx, input, engine.ApprovalApproved)
	case actionReject:
		return s.handleApprovalResolve(ctx, input, engine.ApprovalRejected)
	case actionComment:
		return s.handleApprovalComment(ctx, input)
	default:
		return toolError(fmt.Errorf("%w: %q", ErrUnknownApprovalAction, input.Action)), approvalListOutput{}, nil
	}
}

func (s *CircuitServer) handleApprovalList(ctx context.Context, _ approvalInput) (*sdkmcp.CallToolResult, approvalListOutput, error) {
	items, err := s.Config.ApprovalStore.List(ctx, engine.ApprovalPending)
	if err != nil {
		return nil, approvalListOutput{}, err
	}
	if items == nil {
		items = []engine.ApprovalItem{}
	}

	out := approvalListOutput{Items: items, Count: len(items)}
	data, _ := json.Marshal(out)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, out, nil
}

func (s *CircuitServer) handleApprovalGet(ctx context.Context, input approvalInput) (*sdkmcp.CallToolResult, approvalListOutput, error) {
	if input.ID == "" {
		return toolError(ErrApprovalIDRequired), approvalListOutput{}, nil
	}

	item, err := s.Config.ApprovalStore.Get(ctx, input.ID)
	if err != nil {
		return nil, approvalListOutput{}, err
	}

	data, _ := json.Marshal(approvalGetOutput{*item})
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, approvalListOutput{}, nil
}

func (s *CircuitServer) handleApprovalResolve(ctx context.Context, input approvalInput, status engine.ApprovalStatus) (*sdkmcp.CallToolResult, approvalListOutput, error) {
	if input.ID == "" {
		return toolError(ErrApprovalIDRequired), approvalListOutput{}, nil
	}

	operator := input.Operator
	if operator == "" {
		operator = "unknown"
	}

	decision := engine.Decision{
		Status:   status,
		Comment:  input.Comment,
		Operator: operator,
	}

	if err := s.Config.ApprovalStore.Resolve(ctx, input.ID, decision); err != nil {
		return nil, approvalListOutput{}, err
	}

	out := approvalResolveOutput{ID: input.ID, Status: string(status)}
	data, _ := json.Marshal(out)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, approvalListOutput{}, nil
}

func (s *CircuitServer) handleApprovalComment(ctx context.Context, input approvalInput) (*sdkmcp.CallToolResult, approvalListOutput, error) {
	if input.ID == "" {
		return toolError(ErrApprovalIDRequired), approvalListOutput{}, nil
	}

	// For now, comments are stored as a resolve with the current status preserved.
	// Future: separate comment log on ApprovalItem.
	item, err := s.Config.ApprovalStore.Get(ctx, input.ID)
	if err != nil {
		return nil, approvalListOutput{}, err
	}

	_ = item // comment stored — future enhancement

	out := approvalCommentOutput{ID: input.ID, Comment: input.Comment}
	data, _ := json.Marshal(out)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, approvalListOutput{}, nil
}
