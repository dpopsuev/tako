package motor

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/instrument"

	"fmt"
	"time"
)

type InstrumentAdapter struct {
	shell   instrument.Shell
	sensory chan<- reactivity.Atom
	mu      sync.Mutex
}

var _ cerebrum.MotorBus = (*InstrumentAdapter)(nil)

func NewInstrumentAdapter(shell instrument.Shell, sensory chan<- reactivity.Atom) *InstrumentAdapter {
	return &InstrumentAdapter{shell: shell, sensory: sensory}
}

func (a *InstrumentAdapter) Send(ctx context.Context, cmd cerebrum.Command) error {
	if cmd.Kind != "instrument" {
		return nil
	}

	result, err := a.shell.Exec(ctx, cmd.Target, json.RawMessage(cmd.Payload))

	var atom reactivity.Atom
	if err != nil {
		atom = reactivity.Atom{
			ID:        fmt.Sprintf("instrument-error-%s-%d", cmd.Target, time.Now().UnixNano()),
			Type:      reactivity.ExecutionAtom,
			Source:    reactivity.Instrument,
			Taxonomy:  fmt.Sprintf("execution.instrument-error.%s", cmd.Target),
			Content:   []byte(err.Error()),
			CreatedAt: time.Now(),
		}
	} else {
		atom = reactivity.Atom{
			ID:        fmt.Sprintf("instrument-%s-%d", cmd.Target, time.Now().UnixNano()),
			Type:      reactivity.ExecutionAtom,
			Source:    reactivity.Instrument,
			Taxonomy:  fmt.Sprintf("execution.instrument.%s", cmd.Target),
			Content:   []byte(result.Text()),
			CreatedAt: time.Now(),
		}
	}

	select {
	case a.sensory <- atom:
	case <-ctx.Done():
	}
	return nil
}
