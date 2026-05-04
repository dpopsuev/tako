package corpus

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	agentshell "github.com/dpopsuev/tako/agent/shell"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/artifact"
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

type shellHandler struct {
	name           string
	shell          agentshell.Shell
	actionMode     agentshell.ActionMode
	actionApproval agentshell.ActionApproval
}

func newShellHandler(name string, shell agentshell.Shell) *shellHandler {
	return &shellHandler{name: name, shell: shell}
}

func (o *shellHandler) Name() string                                                               { return o.name }
func (o *shellHandler) Receive(_ artifact.Wire) error                                              { return nil }
func (o *shellHandler) Names() []string                                                            { return o.shell.Names() }
func (o *shellHandler) Describe(n string) (string, error)                                          { return o.shell.Describe(n) }
func (o *shellHandler) Schema(n string) (json.RawMessage, error)                                   { return o.shell.Schema(n) }
func (o *shellHandler) Mode(n string) agentshell.ActionMode                                             { return o.actionMode }
func (o *shellHandler) Approval(n string) agentshell.ActionApproval                                     { return o.actionApproval }
func (o *shellHandler) Exec(ctx context.Context, name string, input json.RawMessage) (agentshell.Result, error) {
	return o.shell.Exec(ctx, name, input)
}

var _ agentshell.Shell = (*shellHandler)(nil)

type autoApproveHITL struct {
	sensory cerebrum.Bus
}

func newAutoApproveHITL(sensory cerebrum.Bus) *autoApproveHITL {
	return &autoApproveHITL{sensory: sensory}
}

func (h *autoApproveHITL) Name() string { return "hitl" }
func (h *autoApproveHITL) Receive(wire artifact.Wire) error {
	if wire.Kind != "motor.pending.hitl" {
		return nil
	}
	h.sensory.Send(context.Background(), cerebrum.Event{
		Kind:   "approval.hitl",
		Source: "human",
	})
	return nil
}

func TestCorpusMotorBus_RW_Denied_During_Think(t *testing.T) {
	c := New()
	shell := agentshell.NewStubShell()
	o := newShellHandler("echo", shell)
	o.actionMode = agentshell.WriteAction
	c.Attach(o)

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ThinkTriad }
	bus := c.MotorBus(sensory, nil, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID: "test-1", Kind: "instrument", Source: "echo",
		Payload: []byte("hello"), CreatedAt: time.Now(),
	})

	events := sensory.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 sensory event, got %d", len(events))
	}
	if events[0].Kind != "instrument.error" {
		t.Errorf("expected instrument.error, got %s", events[0].Kind)
	}
}

func TestCorpusMotorBus_RO_Allowed_During_Think(t *testing.T) {
	c := New()
	shell := agentshell.NewStubShell()
	o := newShellHandler("echo", shell)
	c.Attach(o)

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ThinkTriad }
	bus := c.MotorBus(sensory, nil, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID: "test-2", Kind: "instrument", Source: "echo",
		Payload: []byte("hello"), CreatedAt: time.Now(),
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
	shell := agentshell.NewStubShell()
	o := newShellHandler("echo", shell)
	c.Attach(o)

	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, nil, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID: "test-3", Kind: "instrument", Source: "echo",
		Payload: []byte("hello"), CreatedAt: time.Now(),
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
	shell := agentshell.NewStubShell()
	motor := newShellHandler("echo", shell)
	c.Attach(motor)

	sensory := &captureBus{}
	signalBus := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, signalBus, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID: "test-4", Kind: "instrument", Source: "echo",
		Payload: []byte("hello"), CreatedAt: time.Now(),
	})

	signals := signalBus.Events()
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal event, got %d", len(signals))
	}
	if signals[0].Kind != "motor.execute" {
		t.Errorf("expected motor.execute, got %s", signals[0].Kind)
	}
}

func TestCorpusMotorBus_HITL_Denied(t *testing.T) {
	c := New()
	shell := agentshell.NewStubShell()
	o := newShellHandler("deploy", shell)
	o.actionMode = agentshell.WriteAction
	o.actionApproval = agentshell.HITL
	c.Attach(o)

	sensory := cerebrum.NewChannelBus(8)
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, nil, phase)

	sendCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	bus.Send(sendCtx, cerebrum.Event{
		ID: "test-hitl-deny", Kind: "instrument", Source: "deploy",
	})

	readCtx, readCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer readCancel()
	event, ok := sensory.Receive(readCtx)
	if !ok {
		t.Fatal("expected error event on sensory bus")
	}
	if event.Kind != "instrument.error" {
		t.Errorf("expected instrument.error, got %s", event.Kind)
	}
}

func TestCorpusMotorBus_HITL_Approved(t *testing.T) {
	c := New()
	shell := agentshell.NewStubShell()
	o := newShellHandler("echo", shell)
	o.actionMode = agentshell.WriteAction
	o.actionApproval = agentshell.HITL
	c.Attach(o)

	sensory := cerebrum.NewChannelBus(8)
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, nil, phase)

	// Pre-load approval on sensory bus (human responds before agent blocks)
	sensory.Send(context.Background(), cerebrum.Event{
		Kind: "approval.hitl", Source: "human",
	})

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID: "test-hitl-approve", Kind: "instrument", Source: "echo",
	})

	event, ok := sensory.Receive(ctx)
	if !ok {
		t.Fatal("expected result event")
	}
	if event.Kind != "instrument.result" {
		t.Errorf("expected instrument.result after approval, got %s", event.Kind)
	}
}

func TestCorpusMotorBus_UnknownOrgan(t *testing.T) {
	c := New()
	sensory := &captureBus{}
	phase := func() reactivity.Triad { return reactivity.ImplementTriad }
	bus := c.MotorBus(sensory, nil, phase)

	ctx := context.Background()
	bus.Send(ctx, cerebrum.Event{
		ID: "test-5", Kind: "instrument", Source: "nonexistent",
		Payload: []byte("hello"), CreatedAt: time.Now(),
	})

	events := sensory.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 error event, got %d", len(events))
	}
	if events[0].Kind != "instrument.error" {
		t.Errorf("expected instrument.error, got %s", events[0].Kind)
	}
}
