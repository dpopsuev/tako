package corpus

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/organ"
)

func TestCapabilityPath_Execute(t *testing.T) {
	c := New()
	c.Register(organ.Func{
		Name:        "greet",
		Description: "Say hello",
		Mode:        organ.ReadAction,
		Risk:        0,
		Source:      organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			return organ.TextResult("hello " + string(input)), nil
		},
	})

	sensory := cerebrum.NewChannelBus(8)
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, nil, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID: "cap-1", Kind: "instrument", Source: "greet",
		Payload: []byte(`"world"`), CreatedAt: time.Now(),
	})

	result, ok := sensory.Receive(ctx)
	if !ok {
		t.Fatal("expected result on sensory bus")
	}
	if result.Kind != "instrument.result" {
		t.Errorf("expected instrument.result, got %s", result.Kind)
	}
	if string(result.Payload) != `hello "world"` {
		t.Errorf("payload = %q, want %q", string(result.Payload), `hello "world"`)
	}
}

func TestCapabilityPath_PhaseGating(t *testing.T) {
	c := New()
	c.Register(organ.Func{
		Name:   "deploy",
		Mode:   organ.WriteAction,
		Risk:   0.5,
		Source: organ.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
			return organ.TextResult("deployed"), nil
		},
	})

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ThinkTriad }
	bus := c.MotorBus(sensory, nil, phase)

	bus.Send(context.Background(), cerebrum.Event{
		ID: "cap-2", Kind: "instrument", Source: "deploy",
		CreatedAt: time.Now(),
	})

	events := sensory.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Kind != "instrument.error" {
		t.Errorf("expected instrument.error (write blocked in Think), got %s", events[0].Kind)
	}
}

func TestCapabilityPath_TrustGating(t *testing.T) {
	c := New()
	c.Register(organ.Func{
		Name:   "delete",
		Mode:   organ.WriteAction,
		Risk:   0.9,
		Source: organ.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
			return organ.TextResult("deleted"), nil
		},
	})

	sensory := cerebrum.NewChannelBus(8)
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	trust := func() float64 { return 0.3 }
	bus := c.MotorBus(sensory, nil, phase, trust)

	sendCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	bus.Send(sendCtx, cerebrum.Event{
		ID: "cap-3", Kind: "instrument", Source: "delete",
		CreatedAt: time.Now(),
	})

	readCtx, readCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer readCancel()
	event, ok := sensory.Receive(readCtx)
	if !ok {
		t.Fatal("expected error event")
	}
	if event.Kind != "instrument.error" {
		t.Errorf("expected instrument.error (risk > trust), got %s", event.Kind)
	}
}

func TestCapabilityPath_SignalEmission(t *testing.T) {
	c := New()
	c.Register(organ.Func{
		Name:   "look",
		Mode:   organ.ReadAction,
		Source: organ.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
			return organ.TextResult("you see a room"), nil
		},
	})

	sensory := &captureBus{}
	signalBus := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, signalBus, phase)

	bus.Send(context.Background(), cerebrum.Event{
		ID: "cap-4", Kind: "instrument", Source: "look",
		CreatedAt: time.Now(),
	})

	signals := signalBus.Events()
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].Kind != "motor.execute" {
		t.Errorf("expected motor.execute signal, got %s", signals[0].Kind)
	}
}

func TestCapabilityPath_UnknownCapability(t *testing.T) {
	c := New()

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, nil, phase)

	bus.Send(context.Background(), cerebrum.Event{
		ID: "cap-5", Kind: "instrument", Source: "nonexistent",
		CreatedAt: time.Now(),
	})

	events := sensory.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 error, got %d", len(events))
	}
	if events[0].Kind != "instrument.error" {
		t.Errorf("expected instrument.error, got %s", events[0].Kind)
	}
}
