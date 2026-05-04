package corpus

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
)

type captureBus struct {
	mu     sync.Mutex
	events []cerebrum.Event
}

func (b *captureBus) Send(_ context.Context, event cerebrum.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
	return nil
}

func (b *captureBus) Receive(_ context.Context) (cerebrum.Event, bool) {
	return cerebrum.Event{}, false
}

func (b *captureBus) Events() []cerebrum.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]cerebrum.Event(nil), b.events...)
}

type shellOrgan struct {
	organ.StubOrgan
	shell          organ.Shell
	actionMode     organ.ActionMode
	actionApproval organ.ActionApproval
}

func newShellOrgan(name organ.OrganName, kind organ.Kind, shell organ.Shell) *shellOrgan {
	return &shellOrgan{
		StubOrgan: *organ.NewStubOrganWithKind(name, kind),
		shell:     shell,
	}
}

func (o *shellOrgan) Names() []string                          { return o.shell.Names() }
func (o *shellOrgan) Describe(n string) (string, error)        { return o.shell.Describe(n) }
func (o *shellOrgan) Schema(n string) (json.RawMessage, error) { return o.shell.Schema(n) }
func (o *shellOrgan) Mode(n string) organ.ActionMode             { return o.actionMode }
func (o *shellOrgan) Approval(n string) organ.ActionApproval     { return o.actionApproval }
func (o *shellOrgan) Exec(ctx context.Context, name string, input json.RawMessage) (organ.Result, error) {
	return o.shell.Exec(ctx, name, input)
}

var _ organ.Shell = (*shellOrgan)(nil)

func TestCorpusMotorBus_RW_Denied_During_Think(t *testing.T) {
	c := New()
	shell := organ.NewStubShell()
	o := newShellOrgan("echo", organ.Motor, shell)
	o.actionMode = organ.WriteAction
	c.Attach(o)

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ThinkTriad }
	bus := c.MotorBus(sensory, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID:        "test-1",
		Kind:      "instrument",
		Source:    "echo",
		Payload:   []byte("hello"),
		CreatedAt: time.Now(),
	})

	events := sensory.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 sensory event, got %d", len(events))
	}
	if events[0].Kind != "instrument.error" {
		t.Errorf("expected instrument.error, got %s", events[0].Kind)
	}
	if string(events[0].Payload) != "permission denied: write actions available during implementation phase only" {
		t.Errorf("unexpected error message: %s", events[0].Payload)
	}
}

func TestCorpusMotorBus_RO_Allowed_During_Think(t *testing.T) {
	c := New()
	shell := organ.NewStubShell()
	o := newShellOrgan("echo", organ.Motor, shell)
	c.Attach(o)

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ThinkTriad }
	bus := c.MotorBus(sensory, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID:        "test-2",
		Kind:      "instrument",
		Source:    "echo",
		Payload:   []byte("hello"),
		CreatedAt: time.Now(),
	})

	events := sensory.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 sensory event, got %d", len(events))
	}
	if events[0].Kind != "instrument.result" {
		t.Errorf("expected instrument.result, got %s", events[0].Kind)
	}
}

func TestCorpusMotorBus_RW_Allowed_During_Implement(t *testing.T) {
	c := New()
	shell := organ.NewStubShell()
	o := newShellOrgan("echo", organ.Motor, shell)
	c.Attach(o)

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID:        "test-3",
		Kind:      "instrument",
		Source:    "echo",
		Payload:   []byte("hello"),
		CreatedAt: time.Now(),
	})

	events := sensory.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 sensory event, got %d", len(events))
	}
	if events[0].Kind != "instrument.result" {
		t.Errorf("expected instrument.result, got %s", events[0].Kind)
	}
}

func TestCorpusMotorBus_SignalEmission(t *testing.T) {
	c := New()
	shell := organ.NewStubShell()
	motor := newShellOrgan("echo", organ.Motor, shell)
	signal := organ.NewStubOrganWithKind("andon", organ.Signal)
	c.Attach(motor)
	c.Attach(signal)

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID:        "test-4",
		Kind:      "instrument",
		Source:    "echo",
		Payload:   []byte("hello"),
		CreatedAt: time.Now(),
	})

	received := signal.Received()
	if len(received) != 1 {
		t.Fatalf("expected 1 signal wire, got %d", len(received))
	}
	if received[0].Kind != "motor.execute" {
		t.Errorf("expected motor.execute, got %s", received[0].Kind)
	}
}

func TestCorpusMotorBus_HITL_Denied(t *testing.T) {
	c := New()
	shell := organ.NewStubShell()
	o := newShellOrgan("deploy", organ.Motor, shell)
	o.actionMode = organ.WriteAction
	o.actionApproval = organ.HITL
	c.Attach(o)

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID:        "test-hitl",
		Kind:      "instrument",
		Source:    "deploy",
		Payload:   []byte("production"),
		CreatedAt: time.Now(),
	})

	events := sensory.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 sensory event, got %d", len(events))
	}
	if events[0].Kind != "instrument.error" {
		t.Errorf("expected instrument.error, got %s", events[0].Kind)
	}
	if string(events[0].Payload) != "approval required: this action needs human sign-off" {
		t.Errorf("unexpected error message: %s", events[0].Payload)
	}
}

func TestCorpusMotorBus_UnknownOrgan(t *testing.T) {
	c := New()
	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID:        "test-5",
		Kind:      "instrument",
		Source:    "nonexistent",
		Payload:   []byte("hello"),
		CreatedAt: time.Now(),
	})

	events := sensory.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 error event, got %d", len(events))
	}
	if events[0].Kind != "instrument.error" {
		t.Errorf("expected instrument.error, got %s", events[0].Kind)
	}
}
