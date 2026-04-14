package operator

import (
	"context"

	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/troupe/signal"
)

// Gate signal event names.
const (
	EventGateParked   = "gate.parked"
	EventGateResolved = "gate.resolved"
)

// Gate signal meta keys.
const (
	MetaKeyNodeName   = "node_name"
	MetaKeyApprovalID = "approval_id"
	MetaKeyCircuitRun = "circuit_run"
)

// SignalNotifier is a gate.Notifier that emits signals on the bus when
// a gate parks an item. Optional chaining: if Next is set, it delegates
// to the next notifier after emitting (e.g., WebhookNotifier).
type SignalNotifier struct {
	Bus  signal.Bus
	Next gate.Notifier // optional chain
}

// Notify emits a gate.parked signal and optionally chains to the next notifier.
func (n *SignalNotifier) Notify(ctx context.Context, item gate.ApprovalItem) error {
	n.Bus.Emit(&signal.Signal{
		Event: EventGateParked,
		Agent: signal.AgentServer,
		Meta: map[string]string{
			MetaKeyNodeName:   item.NodeName,
			MetaKeyApprovalID: item.ID,
			MetaKeyCircuitRun: item.CircuitRun,
		},
	})

	if n.Next != nil {
		return n.Next.Notify(ctx, item)
	}
	return nil
}

var _ gate.Notifier = (*SignalNotifier)(nil)
