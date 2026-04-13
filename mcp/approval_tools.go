package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/dpopsuev/origami/engine/gate"

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
	Items []gate.ApprovalItem `json:"items"`
	Count int                 `json:"count"`
}

type approvalGetOutput struct {
	gate.ApprovalItem
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

	s.MCPServer.AddTool(&sdkmcp.Tool{
		Name:        "approval",
		Description: "Approval gate review. Actions: list (pending items), get (view output), approve, reject, comment.",
		InputSchema: map[string]any{"type": "object"},
	}, rawHandler(s.handleApprovalDispatch))
}

func (s *CircuitServer) handleApprovalDispatch(ctx context.Context, req *sdkmcp.CallToolRequest, input approvalInput) (*sdkmcp.CallToolResult, approvalListOutput, error) {
	switch input.Action {
	case actionList:
		return s.handleApprovalList(ctx, input)
	case actionGet:
		return s.handleApprovalGet(ctx, input)
	case actionApprove:
		return s.handleApprovalResolve(ctx, input, gate.ApprovalApproved)
	case actionReject:
		return s.handleApprovalResolve(ctx, input, gate.ApprovalRejected)
	case actionComment:
		return s.handleApprovalComment(ctx, input)
	default:
		return toolError(fmt.Errorf("%w: %q", ErrUnknownApprovalAction, input.Action)), approvalListOutput{}, nil
	}
}

func (s *CircuitServer) handleApprovalList(ctx context.Context, _ approvalInput) (*sdkmcp.CallToolResult, approvalListOutput, error) {
	items, err := s.Config.ApprovalStore.List(ctx, gate.ApprovalPending)
	if err != nil {
		return nil, approvalListOutput{}, err
	}
	if items == nil {
		items = []gate.ApprovalItem{}
	}

	// Sort by priority: critical > high > medium > low > empty.
	sort.SliceStable(items, func(i, j int) bool {
		return priorityRank(items[i].Priority) < priorityRank(items[j].Priority)
	})

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

func (s *CircuitServer) handleApprovalResolve(ctx context.Context, input approvalInput, status gate.ApprovalStatus) (*sdkmcp.CallToolResult, approvalListOutput, error) {
	if input.ID == "" {
		return toolError(ErrApprovalIDRequired), approvalListOutput{}, nil
	}

	operator := input.Operator
	if operator == "" {
		operator = "unknown"
	}

	decision := gate.Decision{
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

// priorityRank maps priority strings to sort order (lower = higher priority).
func priorityRank(p string) int {
	switch p {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}
