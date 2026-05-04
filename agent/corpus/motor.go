package corpus

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	agentshell "github.com/dpopsuev/tako/agent/shell"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/artifact"
)

// MotorBus builds a cerebrum.Bus that routes motor events through the Corpus.
// Routes by event.Source (organ/instrument name), enforces RO/RW permissions,
// and emits to Signal organs as a side effect of every route.
func (c *Corpus) MotorBus(sensory cerebrum.Bus, signal cerebrum.Bus, phase func() reactivity.Triad) cerebrum.Bus {
	return &corpusMotor{
		corpus:  c,
		sensory: sensory,
		signal:  signal,
		phase:   phase,
	}
}

type corpusMotor struct {
	corpus  *Corpus
	sensory cerebrum.Bus
	signal  cerebrum.Bus
	phase   func() reactivity.Triad
}

func (m *corpusMotor) Send(ctx context.Context, event cerebrum.Event) error {
	if event.Kind != "instrument" {
		return nil
	}

	name := event.Source
	o, err := m.corpus.Handler(name)
	if err != nil {
		m.sendError(ctx, event.Source, fmt.Sprintf("unknown organ: %s", event.Source))
		return nil
	}

	if sh, ok := o.(agentshell.Shell); ok {
		mode := sh.Mode(event.Source)
		if mode == agentshell.WriteAction && m.phase() != reactivity.ImplementTriad {
			m.emitSignal(ctx, event, "denied.phase")
			m.sendError(ctx, event.Source, "permission denied: write actions available during implementation phase only")
			return nil
		}
		if sh.Approval(event.Source) == agentshell.HITL {
			m.emitSignal(ctx, event, "pending.hitl")
			approval, ok := m.sensory.Receive(ctx)
			if !ok || approval.Kind != "approval.hitl" {
				m.emitSignal(ctx, event, "denied.hitl")
				m.sendError(ctx, event.Source, "approval denied or timed out")
				return nil
			}
		}
	}

	m.emitSignal(ctx, event, "execute")

	wire := artifact.Wire{
		Kind:    event.Kind,
		Channel: event.Source,
		Payload: event.Payload,
	}
	if err := o.Receive(wire); err != nil {
		m.sendError(ctx, event.Source, err.Error())
		return nil
	}

	// For Shell-based organs, execute and return result via sensory bus.
	// Organs that implement agentshell.Shell get the full exec path.
	if sh, ok := o.(agentshell.Shell); ok {
		result, err := sh.Exec(ctx, event.Source, event.Payload)
		if err != nil {
			m.sendError(ctx, event.Source, err.Error())
			return nil
		}
		return m.sensory.Send(ctx, cerebrum.Event{
			ID:        fmt.Sprintf("motor-%s-%d", event.Source, time.Now().UnixNano()),
			Kind:      "instrument.result",
			Source:    event.Source,
			Payload:   result.Text(),
			CreatedAt: time.Now(),
		})
	}

	return nil
}

func (m *corpusMotor) Receive(_ context.Context) (cerebrum.Event, bool) {
	return cerebrum.Event{}, false
}

func (m *corpusMotor) sendError(_ context.Context, source, msg string) {
	m.sensory.Send(context.Background(), cerebrum.Event{
		ID:        fmt.Sprintf("motor-error-%s-%d", source, time.Now().UnixNano()),
		Kind:      "instrument.error",
		Source:    source,
		Payload:   []byte(msg),
		CreatedAt: time.Now(),
	})
}

func (m *corpusMotor) emitSignal(_ context.Context, event cerebrum.Event, action string) {
	if m.signal == nil {
		return
	}
	m.signal.Send(context.Background(), cerebrum.Event{
		ID:        fmt.Sprintf("signal-%s-%d", action, time.Now().UnixNano()),
		Kind:      fmt.Sprintf("motor.%s", action),
		Source:    event.Source,
		Payload:   event.Payload,
		CreatedAt: time.Now(),
	})
}
