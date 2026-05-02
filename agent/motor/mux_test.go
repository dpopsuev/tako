package motor

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

func TestMux_RoutesToCorrectAdapter(t *testing.T) {
	sensory := cerebrum.NewChannelBus(16)
	instrument := NewInstrumentAdapter(&stubShell{}, sensory)

	mux := NewMux()
	mux.Register("instrument", instrument)

	ctx := context.Background()
	mux.Send(ctx, cerebrum.Event{Kind: "instrument", Source: "grep", Payload: []byte(`"test"`)})

	event, ok := sensory.Receive(ctx)
	if !ok {
		t.Fatal("expected event on sensory bus")
	}
	if event.Kind != "instrument.result" {
		t.Errorf("expected instrument.result, got %s", event.Kind)
	}
}

func TestMux_UnknownKind_NoError(t *testing.T) {
	mux := NewMux()
	err := mux.Send(context.Background(), cerebrum.Event{Kind: "unknown"})
	if err != nil {
		t.Errorf("unknown kind should not error, got %v", err)
	}
}
