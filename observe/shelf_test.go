package observe

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/artifact"
	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/service/depo"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setupTracer(t *testing.T) (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return exporter, tp
}

func TestShelfPush(t *testing.T) {
	exporter, tp := setupTracer(t)
	pool := &ergograph.StubPool{}
	dp := depo.NewStubDepo("test")
	inner := dp.Shelf("s1")

	shelf := NewShelf(inner, pool, tp.Tracer("test"), "s1")
	env := artifact.NewEnvelope("origin", []byte("data"))
	if err := shelf.Push(env); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	items := shelf.Peek()
	if len(items) != 1 {
		t.Errorf("expected 1 item on shelf, got %d", len(items))
	}

	spans := exporter.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "shelf.push" {
			found = true
		}
	}
	if !found {
		t.Error("expected span named 'shelf.push'")
	}

	if pool.Len() < 1 {
		t.Error("expected at least 1 ergograph record")
	}
}

func TestShelfPull(t *testing.T) {
	exporter, tp := setupTracer(t)
	pool := &ergograph.StubPool{}
	dp := depo.NewStubDepo("test")
	inner := dp.Shelf("s1")

	shelf := NewShelf(inner, pool, tp.Tracer("test"), "s1")
	env := artifact.NewEnvelope("origin", []byte("data"))
	_ = shelf.Push(env)

	pulled, err := shelf.Pull("agent-1")
	if err != nil {
		t.Fatalf("pull failed: %v", err)
	}
	if string(pulled.Payload) != "data" {
		t.Errorf("expected payload 'data', got %q", pulled.Payload)
	}

	spans := exporter.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "shelf.pull" {
			found = true
		}
	}
	if !found {
		t.Error("expected span named 'shelf.pull'")
	}
}

func TestShelfPeekNoRecord(t *testing.T) {
	_, tp := setupTracer(t)
	pool := &ergograph.StubPool{}
	dp := depo.NewStubDepo("test")
	inner := dp.Shelf("s1")

	shelf := NewShelf(inner, pool, tp.Tracer("test"), "s1")
	_ = shelf.Peek()

	if pool.Len() != 0 {
		t.Errorf("expected 0 records for read-only Peek, got %d", pool.Len())
	}
}

func TestShelfWatchNoRecord(t *testing.T) {
	_, tp := setupTracer(t)
	pool := &ergograph.StubPool{}
	dp := depo.NewStubDepo("test")
	inner := dp.Shelf("s1")

	shelf := NewShelf(inner, pool, tp.Tracer("test"), "s1")
	ch := shelf.Watch()
	if ch == nil {
		t.Error("expected non-nil watch channel")
	}

	if pool.Len() != 0 {
		t.Errorf("expected 0 records for read-only Watch, got %d", pool.Len())
	}
}
