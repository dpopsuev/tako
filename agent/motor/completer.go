package motor

import (
	"context"
	"sync"

	"github.com/dpopsuev/tako/agent/cerebrum"
	troupe "github.com/dpopsuev/tangle"
)

// CompleterAdapter bridges tangle.Completer to the Motor/Sensory bus protocol.
// Receives Command{Kind:"complete"}, calls Completer, sends Signal{Kind:"response"}.
type CompleterAdapter struct {
	completer troupe.Completer
	mu        sync.Mutex
	inbox     []cerebrum.Signal
}

var _ cerebrum.MotorBus = (*CompleterAdapter)(nil)
var _ cerebrum.SensoryBus = (*CompleterAdapter)(nil)

func NewCompleterAdapter(c troupe.Completer) *CompleterAdapter {
	return &CompleterAdapter{completer: c}
}

func (a *CompleterAdapter) Send(ctx context.Context, cmd cerebrum.Command) error {
	if cmd.Kind != "complete" {
		return nil
	}

	response, err := a.completer.Complete(ctx, string(cmd.Payload))

	a.mu.Lock()
	defer a.mu.Unlock()

	if err != nil {
		a.inbox = append(a.inbox, cerebrum.Signal{
			Kind:    "error",
			Topic:   "completer",
			Content: []byte(err.Error()),
		})
		return nil
	}

	a.inbox = append(a.inbox, cerebrum.Signal{
		Kind:    "response",
		Topic:   "completer",
		Content: []byte(response),
	})
	return nil
}

func (a *CompleterAdapter) Receive(_ context.Context) (cerebrum.Signal, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.inbox) == 0 {
		return cerebrum.Signal{}, false
	}
	sig := a.inbox[0]
	a.inbox = a.inbox[1:]
	return sig, true
}
