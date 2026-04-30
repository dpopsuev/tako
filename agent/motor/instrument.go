package motor

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/instrument"
)

// InstrumentAdapter bridges instrument.Shell to the Motor/Sensory bus protocol.
// Receives Command{Kind:"instrument"}, calls Shell.Exec(), sends Signal back.
type InstrumentAdapter struct {
	shell instrument.Shell
	mu    sync.Mutex
	inbox []cerebrum.Signal
}

var _ cerebrum.MotorBus = (*InstrumentAdapter)(nil)
var _ cerebrum.SensoryBus = (*InstrumentAdapter)(nil)

func NewInstrumentAdapter(shell instrument.Shell) *InstrumentAdapter {
	return &InstrumentAdapter{shell: shell}
}

func (a *InstrumentAdapter) Send(ctx context.Context, cmd cerebrum.Command) error {
	if cmd.Kind != "instrument" {
		return nil
	}

	result, err := a.shell.Exec(ctx, cmd.Target, json.RawMessage(cmd.Payload))

	a.mu.Lock()
	defer a.mu.Unlock()

	if err != nil {
		a.inbox = append(a.inbox, cerebrum.Signal{
			Kind:    "error",
			Topic:   cmd.Target,
			Content: []byte(err.Error()),
		})
		return nil
	}

	a.inbox = append(a.inbox, cerebrum.Signal{
		Kind:    "result",
		Topic:   cmd.Target,
		Content: []byte(result.Text()),
	})
	return nil
}

func (a *InstrumentAdapter) Receive(_ context.Context) (cerebrum.Signal, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.inbox) == 0 {
		return cerebrum.Signal{}, false
	}
	sig := a.inbox[0]
	a.inbox = a.inbox[1:]
	return sig, true
}
