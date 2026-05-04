package corpus

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/artifact"
)

// MotorBus builds a cerebrum.Bus that routes motor events through the Corpus.
// Routes by event.Source (organ/instrument name), enforces RO/RW permissions,
// and emits to Signal organs as a side effect of every route.
func (c *Corpus) MotorBus(sensory cerebrum.Bus, phase func() reactivity.Triad) cerebrum.Bus {
	return &corpusMotor{
		corpus:  c,
		sensory: sensory,
		phase:   phase,
	}
}

type corpusMotor struct {
	corpus  *Corpus
	sensory cerebrum.Bus
	phase   func() reactivity.Triad
}

func (m *corpusMotor) Send(ctx context.Context, event cerebrum.Event) error {
	if event.Kind != "instrument" {
		return nil
	}

	name := organ.OrganName(event.Source)
	o, err := m.corpus.Organ(name)
	if err != nil {
		m.sendError(ctx, event.Source, fmt.Sprintf("unknown organ: %s", event.Source))
		return nil
	}

	if shell, ok := o.(organ.Shell); ok {
		mode := shell.Mode(event.Source)
		if mode == organ.WriteAction && m.phase() != reactivity.ImplementTriad {
			m.signal(ctx, event, "denied.phase")
			m.sendError(ctx, event.Source, "permission denied: write actions available during implementation phase only")
			return nil
		}
		if shell.Approval(event.Source) == organ.HITL {
			m.signal(ctx, event, "denied.hitl")
			m.sendError(ctx, event.Source, "approval required: this action needs human sign-off")
			return nil
		}
	}

	m.signal(ctx, event, "execute")

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
	// Organs that implement organ.Shell get the full exec path.
	if shell, ok := o.(organ.Shell); ok {
		result, err := shell.Exec(ctx, event.Source, event.Payload)
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

func (m *corpusMotor) sendError(ctx context.Context, source, msg string) {
	m.sensory.Send(ctx, cerebrum.Event{
		ID:        fmt.Sprintf("motor-error-%s-%d", source, time.Now().UnixNano()),
		Kind:      "instrument.error",
		Source:    source,
		Payload:   []byte(msg),
		CreatedAt: time.Now(),
	})
}

func (m *corpusMotor) signal(ctx context.Context, event cerebrum.Event, action string) {
	m.corpus.mu.RLock()
	defer m.corpus.mu.RUnlock()
	wire := artifact.Wire{
		Kind:    fmt.Sprintf("motor.%s", action),
		Channel: event.Source,
		Payload: event.Payload,
	}
	for _, o := range m.corpus.organs {
		if o.Kind() == organ.Signal {
			o.Receive(wire)
		}
	}
}
