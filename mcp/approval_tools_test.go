package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine/gate"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// extractText gets the text content from an MCP tool result.
func extractText(t *testing.T, res *sdkmcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("empty tool result content")
	}
	tc, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want TextContent", res.Content[0])
	}
	return tc.Text
}

// testStore is a minimal in-memory ApprovalStore for mcp-internal tests.
type testStore struct {
	items map[string]*gate.ApprovalItem
}

func newTestStore() *testStore { return &testStore{items: make(map[string]*gate.ApprovalItem)} }
func (s *testStore) Park(_ context.Context, item gate.ApprovalItem) error {
	cp := item
	s.items[item.ID] = &cp
	return nil
}
func (s *testStore) Get(_ context.Context, id string) (*gate.ApprovalItem, error) {
	item, ok := s.items[id]
	if !ok {
		return nil, gate.ErrApprovalNotFound
	}
	cp := *item
	return &cp, nil
}
func (s *testStore) List(_ context.Context, status gate.ApprovalStatus) ([]gate.ApprovalItem, error) {
	var result []gate.ApprovalItem
	for _, item := range s.items {
		if item.Status == status {
			result = append(result, *item)
		}
	}
	return result, nil
}
func (s *testStore) Resolve(_ context.Context, id string, d gate.Decision) error {
	item, ok := s.items[id]
	if !ok {
		return gate.ErrApprovalNotFound
	}
	item.Status = d.Status
	item.Decision = &d
	return nil
}

func TestApprovalHandler_ListEmpty(t *testing.T) {
	store := newTestStore()
	srv := &CircuitServer{Config: &CircuitConfig{ApprovalStore: store}}

	res, _, err := srv.handleApprovalDispatch(t.Context(), nil, approvalInput{Action: "list"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	var out approvalListOutput
	json.Unmarshal([]byte(extractText(t, res)), &out)
	if out.Count != 0 {
		t.Errorf("count = %d, want 0", out.Count)
	}
}

func TestApprovalHandler_ParkAndList(t *testing.T) {
	store := newTestStore()
	store.Park(t.Context(), gate.ApprovalItem{
		ID: "test-001", NodeName: "create-pr", Status: gate.ApprovalPending,
		Output: json.RawMessage(`{"diff":"..."}`), ParkedAt: time.Now(),
	})
	srv := &CircuitServer{Config: &CircuitConfig{ApprovalStore: store}}

	res, _, err := srv.handleApprovalDispatch(t.Context(), nil, approvalInput{Action: "list"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	var out approvalListOutput
	json.Unmarshal([]byte(extractText(t, res)), &out)
	if out.Count != 1 {
		t.Errorf("count = %d, want 1", out.Count)
	}
}

func TestApprovalHandler_Approve(t *testing.T) {
	store := newTestStore()
	store.Park(t.Context(), gate.ApprovalItem{
		ID: "test-001", NodeName: "create-pr", Status: gate.ApprovalPending,
	})
	srv := &CircuitServer{Config: &CircuitConfig{ApprovalStore: store}}

	_, _, err := srv.handleApprovalDispatch(t.Context(), nil, approvalInput{
		Action: "approve", ID: "test-001", Comment: "LGTM", Operator: "alice",
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}

	item, _ := store.Get(t.Context(), "test-001")
	if item.Status != gate.ApprovalApproved {
		t.Errorf("status = %q, want approved", item.Status)
	}
	if item.Decision.Operator != "alice" {
		t.Errorf("operator = %q, want alice", item.Decision.Operator)
	}
}

func TestApprovalHandler_Reject(t *testing.T) {
	store := newTestStore()
	store.Park(t.Context(), gate.ApprovalItem{
		ID: "test-001", NodeName: "deploy", Status: gate.ApprovalPending,
	})
	srv := &CircuitServer{Config: &CircuitConfig{ApprovalStore: store}}

	srv.handleApprovalDispatch(t.Context(), nil, approvalInput{
		Action: "reject", ID: "test-001", Comment: "rework", Operator: "bob",
	})

	item, _ := store.Get(t.Context(), "test-001")
	if item.Status != gate.ApprovalRejected {
		t.Errorf("status = %q, want rejected", item.Status)
	}
}

func TestApprovalHandler_GetById(t *testing.T) {
	store := newTestStore()
	store.Park(t.Context(), gate.ApprovalItem{
		ID: "test-001", NodeName: "create-pr", Status: gate.ApprovalPending,
		Output: json.RawMessage(`{"diff":"fix auth"}`),
	})
	srv := &CircuitServer{Config: &CircuitConfig{ApprovalStore: store}}

	res, _, err := srv.handleApprovalDispatch(t.Context(), nil, approvalInput{
		Action: "get", ID: "test-001",
	})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	var out struct {
		NodeName string `json:"node_name"`
	}
	json.Unmarshal([]byte(extractText(t, res)), &out)
	if out.NodeName != "create-pr" {
		t.Errorf("node_name = %q, want create-pr", out.NodeName)
	}
}

func TestApprovalHandler_MissingID(t *testing.T) {
	store := newTestStore()
	srv := &CircuitServer{Config: &CircuitConfig{ApprovalStore: store}}

	res, _, _ := srv.handleApprovalDispatch(t.Context(), nil, approvalInput{
		Action: "approve",
		// No ID — should return error.
	})
	if res == nil || !res.IsError {
		t.Error("approve without ID should return tool error")
	}
}
