package motor

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestMux_RoutesToCorrectAdapter(t *testing.T) {
	sensory := make(chan reactivity.Atom, 16)
	instrument := NewInstrumentAdapter(&stubShell{}, sensory)

	mux := NewMux()
	mux.Register("instrument", instrument)

	ctx := context.Background()
	mux.Send(ctx, cerebrum.Command{Kind: "instrument", Target: "grep", Payload: []byte(`"test"`)})

	select {
	case atom := <-sensory:
		if atom.Type != reactivity.ExecutionAtom {
			t.Errorf("expected ExecutionAtom, got %s", atom.Type)
		}
	default:
		t.Fatal("expected atom on sensory channel")
	}
}

func TestMux_UnknownKind_NoError(t *testing.T) {
	mux := NewMux()
	err := mux.Send(context.Background(), cerebrum.Command{Kind: "unknown"})
	if err != nil {
		t.Errorf("unknown kind should not error, got %v", err)
	}
}
