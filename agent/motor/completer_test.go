package motor

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

type stubCompleter struct {
	response string
	err      error
}

func (s *stubCompleter) Complete(_ context.Context, _ string) (string, error) {
	return s.response, s.err
}

func TestCompleterAdapter_SendAndReceive(t *testing.T) {
	c := &stubCompleter{response: `{"atoms":[{"type":"intent","content":"test"}]}`}
	adapter := NewCompleterAdapter(c)

	ctx := context.Background()
	adapter.Send(ctx, cerebrum.Command{Kind: "complete", Payload: []byte("What is 2+2?")})

	sig, ok := adapter.Receive(ctx)
	if !ok {
		t.Fatal("expected signal")
	}
	if sig.Kind != "response" {
		t.Errorf("expected response, got %s", sig.Kind)
	}
	if sig.Topic != "completer" {
		t.Errorf("expected topic completer, got %s", sig.Topic)
	}
}

func TestCompleterAdapter_Error(t *testing.T) {
	c := &stubCompleter{err: context.DeadlineExceeded}
	adapter := NewCompleterAdapter(c)

	ctx := context.Background()
	adapter.Send(ctx, cerebrum.Command{Kind: "complete", Payload: []byte("timeout")})

	sig, ok := adapter.Receive(ctx)
	if !ok {
		t.Fatal("expected error signal")
	}
	if sig.Kind != "error" {
		t.Errorf("expected error kind, got %s", sig.Kind)
	}
}

func TestCompleterAdapter_IgnoresNonCompleteCommands(t *testing.T) {
	c := &stubCompleter{response: "ignored"}
	adapter := NewCompleterAdapter(c)

	adapter.Send(context.Background(), cerebrum.Command{Kind: "instrument", Payload: []byte("nope")})

	_, ok := adapter.Receive(context.Background())
	if ok {
		t.Error("should not produce signal for non-complete commands")
	}
}
