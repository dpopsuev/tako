package motor

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

func TestMux_RoutesToCorrectAdapter(t *testing.T) {
	completer := NewCompleterAdapter(&stubCompleter{response: "hello"})

	mux := NewMux()
	mux.Register("complete", completer, completer)

	ctx := context.Background()
	mux.Send(ctx, cerebrum.Command{Kind: "complete", Payload: []byte("prompt")})

	sig, ok := mux.Receive(ctx)
	if !ok {
		t.Fatal("expected signal from mux")
	}
	if sig.Kind != "response" {
		t.Errorf("expected response, got %s", sig.Kind)
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
