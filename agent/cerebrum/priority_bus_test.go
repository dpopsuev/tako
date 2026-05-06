package cerebrum

import (
	"context"
	"testing"
)

func TestPriorityMotorBusSend(t *testing.T) {
	inner := &stubBus{}
	bus := NewPriorityMotorBus(inner)

	ctx := context.Background()
	event := Event{ID: "e1", Kind: "test", Source: "cap"}

	if err := bus.Send(ctx, event); err != nil {
		t.Fatal(err)
	}

	events := inner.Events()
	if len(events) != 1 || events[0].ID != "e1" {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestPriorityMotorBusSendWithPriority(t *testing.T) {
	inner := &stubBus{}
	bus := NewPriorityMotorBus(inner)

	ctx := context.Background()
	event := Event{ID: "e1", Kind: "test"}

	if err := bus.SendWithPriority(ctx, event, PriorityReflex); err != nil {
		t.Fatal(err)
	}

	if bus.CurrentPriority() != 0 {
		t.Fatalf("priority should reset to 0 after send, got %d", bus.CurrentPriority())
	}
}

func TestPriorityMotorBusDefaultPriority(t *testing.T) {
	inner := &stubBus{}
	bus := NewPriorityMotorBus(inner)

	if bus.CurrentPriority() != 0 {
		t.Fatalf("initial priority should be 0, got %d", bus.CurrentPriority())
	}
}

func TestPriorityMotorBusReceive(t *testing.T) {
	inner := &stubBus{}
	bus := NewPriorityMotorBus(inner)

	_, ok := bus.Receive(context.Background())
	if ok {
		t.Fatal("empty bus should return false")
	}
}

func TestPriorityConstants(t *testing.T) {
	if PriorityReflex >= PriorityWatcher {
		t.Fatal("reflex must be higher priority (lower number) than watcher")
	}
	if PriorityWatcher >= PriorityThinker {
		t.Fatal("watcher must be higher priority than thinker")
	}
}
