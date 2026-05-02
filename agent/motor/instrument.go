package motor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/instrument"
)

type InstrumentAdapter struct {
	shell   instrument.Shell
	sensory cerebrum.Bus
}

var _ cerebrum.Bus = (*InstrumentAdapter)(nil)

func NewInstrumentAdapter(shell instrument.Shell, sensory cerebrum.Bus) *InstrumentAdapter {
	return &InstrumentAdapter{shell: shell, sensory: sensory}
}

func (a *InstrumentAdapter) Send(ctx context.Context, event cerebrum.Event) error {
	if event.Kind != "instrument" {
		return nil
	}

	result, err := a.shell.Exec(ctx, event.Source, json.RawMessage(event.Payload))

	var response cerebrum.Event
	if err != nil {
		response = cerebrum.Event{
			ID:        fmt.Sprintf("instrument-error-%s-%d", event.Source, time.Now().UnixNano()),
			Kind:      "instrument.error",
			Source:    event.Source,
			Payload:   []byte(err.Error()),
			CreatedAt: time.Now(),
		}
	} else {
		response = cerebrum.Event{
			ID:        fmt.Sprintf("instrument-%s-%d", event.Source, time.Now().UnixNano()),
			Kind:      "instrument.result",
			Source:    event.Source,
			Payload:   []byte(result.Text()),
			CreatedAt: time.Now(),
		}
	}

	return a.sensory.Send(ctx, response)
}

func (a *InstrumentAdapter) Receive(_ context.Context) (cerebrum.Event, bool) {
	return cerebrum.Event{}, false
}
