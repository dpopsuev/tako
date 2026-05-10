package corpus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/organ"
)

// MotorBus builds a cerebrum.Bus that routes motor events through the Corpus.
// Routes by event.Source (handler name), enforces RO/RW permissions,
// and emits to signal bus as a side effect of every route.
func (c *Corpus) MotorBus(sensory cerebrum.Bus, signal cerebrum.Bus, phase func() reactivity.Triad, trust ...func() float64) cerebrum.Bus {
	m := &corpusMotor{
		corpus:  c,
		sensory: sensory,
		signal:  signal,
		phase:   phase,
	}
	if len(trust) > 0 {
		m.trust = trust[0]
	}
	return m
}

type corpusMotor struct {
	corpus  *Corpus
	sensory cerebrum.Bus
	signal  cerebrum.Bus
	phase   func() reactivity.Triad
	trust   func() float64
}

func (m *corpusMotor) Send(ctx context.Context, event cerebrum.Event) error {
	if event.Kind != cerebrum.EventOrgan {
		return nil
	}

	cap, ok := m.corpus.Organ(event.Source)
	if !ok {
		m.sendError(ctx, event.Source, fmt.Sprintf("unknown capability: %s", event.Source))
		return nil
	}

	return m.executeCapability(ctx, event, cap)
}

func (m *corpusMotor) executeCapability(ctx context.Context, event cerebrum.Event, cap organ.Func) error {
	if cap.Mode == organ.WriteAction && m.phase != nil && m.phase() != reactivity.ImplementTriad {
		slog.WarnContext(ctx, "corpus.motor.denied_phase",
			slog.String("capability", cap.Name),
			slog.String("mode", "write"),
			slog.String("phase", m.phase().String()))
		m.emitSignal(ctx, event, cerebrum.EventMotorDeniedPhase)
		m.sendError(ctx, event.Source, "permission denied: write actions available during implementation phase only")
		return nil
	}

	needsHITL := cap.Approval == organ.HITL
	if !needsHITL && m.trust != nil {
		needsHITL = cap.Risk > m.trust()
	}
	if needsHITL {
		slog.InfoContext(ctx, "corpus.motor.hitl_gate",
			slog.String("capability", cap.Name),
			slog.Float64("risk", cap.Risk))
		m.emitSignal(ctx, event, cerebrum.EventMotorPendingHITL)
		approval, ok := m.sensory.Receive(ctx)
		if !ok || approval.Kind != cerebrum.EventApprovalHITL {
			slog.WarnContext(ctx, "corpus.motor.hitl_denied",
				slog.String("capability", cap.Name))
			m.emitSignal(ctx, event, cerebrum.EventMotorDeniedHITL)
			m.sendError(ctx, event.Source, "approval denied or timed out")
			return nil
		}
	}

	m.emitSignal(ctx, event, cerebrum.EventMotorExecute)

	if cap.Execute == nil {
		slog.WarnContext(ctx, "corpus.motor.no_execute",
			slog.String("capability", cap.Name))
		m.sendError(ctx, event.Source, "capability has no execute function")
		return nil
	}

	start := time.Now()
	result, err := cap.Execute(ctx, event.Payload)
	elapsed := time.Since(start)

	if err != nil {
		slog.WarnContext(ctx, "corpus.motor.execute_error",
			slog.String("capability", cap.Name),
			slog.Duration("elapsed", elapsed),
			slog.Any("error", err))
		m.sendError(ctx, event.Source, err.Error())
		return nil
	}

	slog.InfoContext(ctx, "corpus.motor.execute_ok",
		slog.String("capability", cap.Name),
		slog.String("source", string(cap.Source.String())),
		slog.Duration("elapsed", elapsed),
		slog.Int("result_len", len(result.Text())))

	return m.sensory.Send(ctx, cerebrum.Event{
		ID:         fmt.Sprintf("motor-%s-%d", event.Source, time.Now().UnixNano()),
		Kind:       cerebrum.EventOrganResult,
		Source:     event.Source,
		Payload:    result.Text(),
		ToolCallID: event.ToolCallID,
		CreatedAt:  time.Now(),
	})
}

func (m *corpusMotor) Receive(_ context.Context) (cerebrum.Event, bool) {
	return cerebrum.Event{}, false
}

func (m *corpusMotor) sendError(_ context.Context, source, msg string) {
	m.sensory.Send(context.Background(), cerebrum.Event{
		ID:        fmt.Sprintf("motor-error-%s-%d", source, time.Now().UnixNano()),
		Kind:      cerebrum.EventOrganError,
		Source:    source,
		Payload:   []byte(msg),
		CreatedAt: time.Now(),
	})
}

func (m *corpusMotor) emitSignal(_ context.Context, event cerebrum.Event, action cerebrum.EventKind) {
	if m.signal == nil {
		return
	}
	m.signal.Send(context.Background(), cerebrum.Event{
		ID:        fmt.Sprintf("signal-%s-%d", action, time.Now().UnixNano()),
		Kind:      action,
		Source:    event.Source,
		Payload:   event.Payload,
		CreatedAt: time.Now(),
	})
}
