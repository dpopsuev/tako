package cerebrum

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func TestDefaultSynapse_Encode(t *testing.T) {
	syn := DefaultSynapse{}
	event := Event{
		ID:        "evt-1",
		Kind:      "sensor.temperature",
		Source:    "thermometer",
		Payload:   []byte("42"),
		CreatedAt: time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
	}

	atom, err := syn.Encode(event)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	if atom.ID != "evt-1" {
		t.Errorf("ID: got %q, want %q", atom.ID, "evt-1")
	}
	if atom.Type != reactivity.IntentAtom {
		t.Errorf("Type: got %v, want IntentAtom", atom.Type)
	}
	if atom.Source != reactivity.Received {
		t.Errorf("Source: got %v, want Received", atom.Source)
	}
	if atom.Taxonomy != "intent.sensor.temperature" {
		t.Errorf("Taxonomy: got %q, want %q", atom.Taxonomy, "intent.sensor.temperature")
	}
	if string(atom.Content) != "42" {
		t.Errorf("Content: got %q, want %q", atom.Content, "42")
	}
	if !atom.CreatedAt.Equal(event.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", atom.CreatedAt, event.CreatedAt)
	}
}

func TestDefaultSynapse_Decode(t *testing.T) {
	syn := DefaultSynapse{}
	emission := reactivity.Emission{
		Kind:    "instrument",
		Target:  "look_fridge",
		Payload: []byte("{}"),
	}

	event := syn.Decode(emission)

	if event.Kind != "instrument" {
		t.Errorf("Kind: got %q, want %q", event.Kind, "instrument")
	}
	if event.Source != "look_fridge" {
		t.Errorf("Source: got %q, want %q", event.Source, "look_fridge")
	}
	if string(event.Payload) != "{}" {
		t.Errorf("Payload: got %q, want %q", event.Payload, "{}")
	}
	if event.ID == "" {
		t.Error("ID should be generated")
	}
	if event.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestRun_EventFlowsThrough(t *testing.T) {
	reactor := reactivity.NewReactor(
		reactivity.WithTriad(reactivity.ThinkTriad, &emittingTriadReactor{}),
	)
	completer := &stubCompleter{response: "done"}
	motor := &stubBus{}
	sensory := NewChannelBus(8)
	cb := New(reactor, completer, WithMotor(motor))
	cb.sensory = sensory

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		cb.Run(ctx)
		close(done)
	}()

	sensory.Send(ctx, Event{
		ID: "test-event", Kind: "test", Source: "unit",
		Payload: []byte("hello"), CreatedAt: time.Now(),
	})

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			cancel()
			<-done
			if len(motor.Events()) == 0 {
				t.Fatal("motor never received emission")
			}
			return
		default:
			events := motor.Events()
			if len(events) > 0 {
				cancel()
				<-done
				if events[0].Kind != "instrument" {
					t.Errorf("expected instrument emission, got %q", events[0].Kind)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRun_ContextCancel_CleansUp(t *testing.T) {
	reactor := reactivity.NewReactor()
	completer := &stubCompleter{response: "done"}
	cb := New(reactor, completer)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		cb.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func TestCerebrum_WithCustomSynapse(t *testing.T) {
	syn := &stubSynapse{}
	reactor := reactivity.NewReactor(
		reactivity.WithTriad(reactivity.ThinkTriad, &emittingTriadReactor{}),
	)
	completer := &stubCompleter{toolCalls: []tangle.ToolCall{{
		ID:    "tc-phase",
		Name:  "intent",
		Input: json.RawMessage(`{"taxonomy":"intent.goal.test","content":"go","dimensions":["test"]}`),
	}}}
	motor := &stubBus{}
	cb := New(reactor, completer, WithMotor(motor), WithSynapse(syn))

	if err := cb.Think(context.Background(), reactivity.Catalyst{Need: "test", Desired: map[string]any{"done": true}}); err != nil {
		t.Fatalf("Think: %v", err)
	}

	if syn.decodeCalled == 0 {
		t.Error("custom Synapse.Decode was never called during dispatch")
	}
}
