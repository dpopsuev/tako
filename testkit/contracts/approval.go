package contracts

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine/gate"
)

// RunApprovalStoreContract verifies that an ApprovalStore implementation
// satisfies the full lifecycle: park → get → list → resolve → list resolved.
//
// Usage:
//
//	func TestMyStore_Contract(t *testing.T) {
//	    contracts.RunApprovalStoreContract(t, func() gate.ApprovalStore {
//	        return NewMyApprovalStore()
//	    })
//	}
func RunApprovalStoreContract(t *testing.T, factory func() gate.ApprovalStore) {
	t.Helper()

	item := gate.ApprovalItem{
		ID:         "test-001",
		CircuitRun: "run-abc",
		NodeName:   "create-pr",
		Output:     json.RawMessage(`{"diff": "...", "title": "Fix bug"}`),
		ParkedAt:   time.Now(),
		Status:     gate.ApprovalPending,
	}

	t.Run("Park_and_Get", func(t *testing.T) {
		store := factory()
		if err := store.Park(context.Background(), item); err != nil {
			t.Fatalf("Park: %v", err)
		}
		got, err := store.Get(context.Background(), item.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.ID != item.ID {
			t.Errorf("ID = %q, want %q", got.ID, item.ID)
		}
		if got.NodeName != item.NodeName {
			t.Errorf("NodeName = %q, want %q", got.NodeName, item.NodeName)
		}
		if got.Status != gate.ApprovalPending {
			t.Errorf("Status = %q, want pending", got.Status)
		}
	})

	t.Run("List_Pending", func(t *testing.T) {
		store := factory()
		store.Park(context.Background(), item)

		pending, err := store.List(context.Background(), gate.ApprovalPending)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(pending) != 1 {
			t.Fatalf("List pending = %d, want 1", len(pending))
		}
		if pending[0].ID != item.ID {
			t.Errorf("pending[0].ID = %q, want %q", pending[0].ID, item.ID)
		}
	})

	t.Run("Resolve_Approve", func(t *testing.T) {
		store := factory()
		store.Park(context.Background(), item)

		decision := gate.Decision{
			Status:   gate.ApprovalApproved,
			Comment:  "LGTM",
			Operator: "alice",
		}
		if err := store.Resolve(context.Background(), item.ID, decision); err != nil {
			t.Fatalf("Resolve: %v", err)
		}

		got, _ := store.Get(context.Background(), item.ID)
		if got.Status != gate.ApprovalApproved {
			t.Errorf("Status after approve = %q, want approved", got.Status)
		}
		if got.Decision == nil {
			t.Fatal("Decision is nil after resolve")
		}
		if got.Decision.Operator != "alice" {
			t.Errorf("Operator = %q, want alice", got.Decision.Operator)
		}
	})

	t.Run("Resolve_Reject", func(t *testing.T) {
		store := factory()
		store.Park(context.Background(), item)

		decision := gate.Decision{
			Status:   gate.ApprovalRejected,
			Comment:  "Needs rework",
			Operator: "bob",
		}
		store.Resolve(context.Background(), item.ID, decision)

		got, _ := store.Get(context.Background(), item.ID)
		if got.Status != gate.ApprovalRejected {
			t.Errorf("Status after reject = %q, want rejected", got.Status)
		}
	})

	t.Run("List_After_Resolve_Excludes_Resolved", func(t *testing.T) {
		store := factory()
		store.Park(context.Background(), item)
		store.Resolve(context.Background(), item.ID, gate.Decision{
			Status: gate.ApprovalApproved, Operator: "alice",
		})

		pending, _ := store.List(context.Background(), gate.ApprovalPending)
		if len(pending) != 0 {
			t.Errorf("List pending after resolve = %d, want 0", len(pending))
		}

		approved, _ := store.List(context.Background(), gate.ApprovalApproved)
		if len(approved) != 1 {
			t.Errorf("List approved = %d, want 1", len(approved))
		}
	})

	t.Run("Get_NotFound", func(t *testing.T) {
		store := factory()
		_, err := store.Get(context.Background(), "nonexistent")
		if err == nil {
			t.Error("Get nonexistent should return error")
		}
	})

	t.Run("Resolve_NotFound", func(t *testing.T) {
		store := factory()
		err := store.Resolve(context.Background(), "nonexistent", gate.Decision{
			Status: gate.ApprovalApproved, Operator: "alice",
		})
		if err == nil {
			t.Error("Resolve nonexistent should return error")
		}
	})
}
