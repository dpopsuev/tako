package stubs_test

import (
	"testing"

	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/origami/testkit/contracts"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func TestMemoryApprovalStore_Contract(t *testing.T) {
	contracts.RunApprovalStoreContract(t, func() gate.ApprovalStore {
		return stubs.NewMemoryApprovalStore()
	})
}

func TestStubNotifier_RecordsCalls(t *testing.T) {
	n := stubs.NewStubNotifier()

	item := gate.ApprovalItem{ID: "test-1", NodeName: "create-pr"}
	n.Notify(t.Context(), item)
	n.Notify(t.Context(), gate.ApprovalItem{ID: "test-2", NodeName: "deploy"})

	if n.CallCount() != 2 {
		t.Errorf("CallCount = %d, want 2", n.CallCount())
	}
	if n.Calls()[0].ID != "test-1" {
		t.Errorf("calls[0].ID = %q", n.Calls()[0].ID)
	}
}
