package assertions_test

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/testkit/assertions"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func parkItem(store *stubs.MemoryApprovalStore, node string) {
	item := engine.ApprovalItem{
		ID:       "apr-" + node,
		NodeName: node,
		Status:   engine.ApprovalPending,
	}
	store.Park(context.TODO(), item)
}

func TestAssertParked_Found(t *testing.T) {
	store := stubs.NewMemoryApprovalStore()
	parkItem(store, "deploy")

	item := assertions.AssertParked(t, store, "deploy")
	if item.ID != "apr-deploy" {
		t.Errorf("ID = %q", item.ID)
	}
}

func TestAssertApproved_AfterResolve(t *testing.T) {
	store := stubs.NewMemoryApprovalStore()
	parkItem(store, "deploy")

	store.Resolve(context.TODO(), "apr-deploy", engine.Decision{
		Status:   engine.ApprovalApproved,
		Operator: "test",
	})

	assertions.AssertApproved(t, store, "apr-deploy")
}

func TestAssertNoPending_Empty(t *testing.T) {
	store := stubs.NewMemoryApprovalStore()
	assertions.AssertNoPending(t, store)
}

func TestAssertNoPending_AfterResolve(t *testing.T) {
	store := stubs.NewMemoryApprovalStore()
	parkItem(store, "deploy")

	store.Resolve(context.TODO(), "apr-deploy", engine.Decision{
		Status:   engine.ApprovalApproved,
		Operator: "test",
	})

	assertions.AssertNoPending(t, store)
}
