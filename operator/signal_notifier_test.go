package operator_test

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/origami/operator"
	"github.com/dpopsuev/troupe/signal"
)

func TestSignalNotifier_EmitsGateParked(t *testing.T) {
	t.Parallel()
	bus := signal.NewMemBus()

	notifier := &operator.SignalNotifier{Bus: bus}

	item := gate.ApprovalItem{
		ID:         "w1:deploy:1",
		CircuitRun: "w1",
		NodeName:   "deploy",
		ParkedAt:   time.Now(),
		Status:     gate.ApprovalPending,
	}

	err := notifier.Notify(context.Background(), item)
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	signals := bus.Since(0)
	if len(signals) != 1 {
		t.Fatalf("signals = %d, want 1", len(signals))
	}

	sig := signals[0]
	if sig.Event != operator.EventGateParked {
		t.Errorf("event = %q, want %q", sig.Event, operator.EventGateParked)
	}
	if sig.Meta[operator.MetaKeyNodeName] != "deploy" {
		t.Errorf("node_name = %q, want deploy", sig.Meta[operator.MetaKeyNodeName])
	}
	if sig.Meta[operator.MetaKeyApprovalID] != "w1:deploy:1" {
		t.Errorf("approval_id = %q, want w1:deploy:1", sig.Meta[operator.MetaKeyApprovalID])
	}
	if sig.Meta[operator.MetaKeyCircuitRun] != "w1" {
		t.Errorf("circuit_run = %q, want w1", sig.Meta[operator.MetaKeyCircuitRun])
	}
}

func TestSignalNotifier_ChainsToNext(t *testing.T) {
	t.Parallel()
	bus := signal.NewMemBus()

	var nextCalled bool
	next := &stubNotifier{fn: func(_ context.Context, _ gate.ApprovalItem) error {
		nextCalled = true
		return nil
	}}

	notifier := &operator.SignalNotifier{Bus: bus, Next: next}

	err := notifier.Notify(context.Background(), gate.ApprovalItem{
		ID:       "w1:review:1",
		NodeName: "review",
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if !nextCalled {
		t.Fatal("chained notifier was not called")
	}
	if bus.Len() != 1 {
		t.Errorf("signals = %d, want 1 (signal emitted before chain)", bus.Len())
	}
}

func TestSignalNotifier_NilBusNilNext(t *testing.T) {
	t.Parallel()
	bus := signal.NewMemBus()
	notifier := &operator.SignalNotifier{Bus: bus}

	// No Next — should not panic.
	err := notifier.Notify(context.Background(), gate.ApprovalItem{ID: "x"})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
}

// stubNotifier is a test helper implementing gate.Notifier.
type stubNotifier struct {
	fn func(context.Context, gate.ApprovalItem) error
}

func (s *stubNotifier) Notify(ctx context.Context, item gate.ApprovalItem) error {
	return s.fn(ctx, item)
}
