package operator_test

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/origami/operator"
	"github.com/dpopsuev/origami/testkit/stubs"
	"github.com/dpopsuev/troupe/signal"
)

func TestCLIGatekeeper_Approve(t *testing.T) {
	t.Parallel()
	store := stubs.NewMemoryApprovalStore()
	bus := signal.NewMemBus()

	gk := &operator.CLIGatekeeper{
		Store:    store,
		Bus:      bus,
		Operator: "test-operator",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Resolve the gate from a goroutine after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		items, _ := store.List(ctx, gate.ApprovalPending)
		if len(items) == 0 {
			return
		}
		_ = store.Resolve(ctx, items[0].ID, gate.Decision{
			Status:   gate.ApprovalApproved,
			Comment:  "LGTM",
			Operator: "alice",
		})
	}()

	allowed, reason, err := gk.Pass(ctx, `{"diff": "some changes"}`)
	if err != nil {
		t.Fatalf("Pass: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed=true")
	}
	if reason != "LGTM" {
		t.Errorf("reason = %q, want LGTM", reason)
	}

	// Signal should have been emitted.
	if bus.Len() == 0 {
		t.Error("no gate.parked signal emitted")
	}
}

func TestCLIGatekeeper_Reject(t *testing.T) {
	t.Parallel()
	store := stubs.NewMemoryApprovalStore()

	gk := &operator.CLIGatekeeper{
		Store:    store,
		Operator: "test-operator",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		items, _ := store.List(ctx, gate.ApprovalPending)
		if len(items) == 0 {
			return
		}
		_ = store.Resolve(ctx, items[0].ID, gate.Decision{
			Status:   gate.ApprovalRejected,
			Comment:  "don't change public API",
			Operator: "bob",
		})
	}()

	allowed, reason, err := gk.Pass(ctx, `{"diff": "bad changes"}`)
	if err != nil {
		t.Fatalf("Pass: %v", err)
	}
	if allowed {
		t.Fatal("expected allowed=false")
	}
	if reason != "don't change public API" {
		t.Errorf("reason = %q, want 'don't change public API'", reason)
	}
}

func TestCLIGatekeeper_ContextCanceled(t *testing.T) {
	t.Parallel()
	store := stubs.NewMemoryApprovalStore()

	gk := &operator.CLIGatekeeper{
		Store:    store,
		Operator: "test-operator",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Never resolve — should timeout.
	allowed, _, err := gk.Pass(ctx, `{"diff": "waiting forever"}`)
	if err == nil {
		t.Fatal("expected context error")
	}
	if allowed {
		t.Fatal("expected allowed=false on timeout")
	}
}

func TestCLIGatekeeper_NilBus(t *testing.T) {
	t.Parallel()
	store := stubs.NewMemoryApprovalStore()

	// No bus — should not panic.
	gk := &operator.CLIGatekeeper{
		Store:    store,
		Operator: "test-operator",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		items, _ := store.List(ctx, gate.ApprovalPending)
		if len(items) == 0 {
			return
		}
		_ = store.Resolve(ctx, items[0].ID, gate.Decision{
			Status: gate.ApprovalApproved,
		})
	}()

	allowed, _, err := gk.Pass(ctx, `ok`)
	if err != nil {
		t.Fatalf("Pass: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed=true")
	}
}
