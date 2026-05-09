package cerebrum

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func TestWaitToolResult_MatchByID(t *testing.T) {
	sensory := NewChannelBus(16)
	reactor := reactivity.NewReactor()
	cb := New(reactor, &stubCompleter{response: "x"}, WithSensory(sensory))

	go func() {
		sensory.Send(context.Background(), Event{
			ToolCallID: "tc-1",
			Source:     "file_read",
			Payload:    []byte("file contents"),
		})
	}()

	tc := tangle.ToolCall{ID: "tc-1", Name: "file_read"}
	cb.registerPending(tc.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := cb.waitToolResult(ctx, tc)

	if result.Content != "file contents" {
		t.Fatalf("result = %q, want 'file contents'", result.Content)
	}
}

func TestWaitToolResult_OutOfOrderBuffering(t *testing.T) {
	sensory := NewChannelBus(16)
	reactor := reactivity.NewReactor()
	cb := New(reactor, &stubCompleter{response: "x"}, WithSensory(sensory))

	cb.registerPending("tc-1")
	cb.registerPending("tc-2")
	cb.registerPending("tc-3")

	go func() {
		sensory.Send(context.Background(), Event{ToolCallID: "tc-3", Source: "go_test", Payload: []byte("result-3")})
		sensory.Send(context.Background(), Event{ToolCallID: "tc-1", Source: "file_read", Payload: []byte("result-1")})
		sensory.Send(context.Background(), Event{ToolCallID: "tc-2", Source: "edit", Payload: []byte("result-2")})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	r1 := cb.waitToolResult(ctx, tangle.ToolCall{ID: "tc-1", Name: "file_read"})
	if r1.Content != "result-1" {
		t.Fatalf("tc-1: got %q, want 'result-1'", r1.Content)
	}

	r2 := cb.waitToolResult(ctx, tangle.ToolCall{ID: "tc-2", Name: "edit"})
	if r2.Content != "result-2" {
		t.Fatalf("tc-2: got %q, want 'result-2'", r2.Content)
	}

	r3 := cb.waitToolResult(ctx, tangle.ToolCall{ID: "tc-3", Name: "go_test"})
	if r3.Content != "result-3" {
		t.Fatalf("tc-3: got %q, want 'result-3'", r3.Content)
	}
}

func TestWaitToolResult_NonToolEventRequeued(t *testing.T) {
	sensory := NewChannelBus(16)
	reactor := reactivity.NewReactor()
	cb := New(reactor, &stubCompleter{response: "x"}, WithSensory(sensory))

	cb.registerPending("tc-1")

	go func() {
		sensory.Send(context.Background(), Event{Kind: "sensory.timer", Source: "timer", Payload: []byte("tick")})
		sensory.Send(context.Background(), Event{ToolCallID: "tc-1", Source: "file_read", Payload: []byte("ok")})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := cb.waitToolResult(ctx, tangle.ToolCall{ID: "tc-1", Name: "file_read"})
	if result.Content != "ok" {
		t.Fatalf("result = %q, want 'ok'", result.Content)
	}

	events := cb.DrainMonitorEvents()
	if len(events) != 1 || events[0].Kind != "sensory.timer" {
		t.Fatalf("timer event should be requeued to monitorEvents, got %d events", len(events))
	}
}

func TestWaitToolResult_Timeout(t *testing.T) {
	sensory := NewChannelBus(16)
	reactor := reactivity.NewReactor()
	cb := New(reactor, &stubCompleter{response: "x"}, WithSensory(sensory))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := cb.waitToolResult(ctx, tangle.ToolCall{ID: "tc-never", Name: "slow"})
	if result.Content != "tool call timed out" {
		t.Fatalf("result = %q, want timeout message", result.Content)
	}
}

func TestWaitToolResult_BufferedResult(t *testing.T) {
	sensory := NewChannelBus(16)
	reactor := reactivity.NewReactor()
	cb := New(reactor, &stubCompleter{response: "x"}, WithSensory(sensory))

	cb.registerPending("tc-pre")
	cb.resultBuffer["tc-pre"] = Event{ToolCallID: "tc-pre", Payload: []byte("pre-buffered")}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := cb.waitToolResult(ctx, tangle.ToolCall{ID: "tc-pre", Name: "cached"})
	if result.Content != "pre-buffered" {
		t.Fatalf("result = %q, want 'pre-buffered'", result.Content)
	}

	if _, exists := cb.resultBuffer["tc-pre"]; exists {
		t.Fatal("buffer entry should be consumed")
	}
}
