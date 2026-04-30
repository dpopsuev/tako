package motor

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

func TestMux_RoutesToCorrectAdapter(t *testing.T) {
	instrument := NewInstrumentAdapter(&stubShell{})

	mux := NewMux()
	mux.Register("instrument", instrument, instrument)

	ctx := context.Background()
	mux.Send(ctx, cerebrum.Command{Kind: "instrument", Target: "grep", Payload: []byte(`"test"`)})

	sig, ok := mux.Receive(ctx)
	if !ok {
		t.Fatal("expected signal from mux")
	}
	if sig.Topic != "grep" {
		t.Errorf("expected topic grep, got %s", sig.Topic)
	}
}

func TestMux_UnknownKind_NoError(t *testing.T) {
	mux := NewMux()
	err := mux.Send(context.Background(), cerebrum.Command{Kind: "unknown"})
	if err != nil {
		t.Errorf("unknown kind should not error, got %v", err)
	}
}

func TestMux_EmptyReceive(t *testing.T) {
	mux := NewMux()
	_, ok := mux.Receive(context.Background())
	if ok {
		t.Error("empty mux should return false")
	}
}
