package observe

import (
	"context"

	"github.com/dpopsuev/tako/artifact"
	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/service/depo"
	"go.opentelemetry.io/otel/trace"
)

// Shelf wraps a depo.Shelf with OTel spans and Ergograph records.
type Shelf struct {
	inner  depo.Shelf
	pool   ergograph.Pool
	tracer trace.Tracer
	name   string
}

var _ depo.Shelf = (*Shelf)(nil)

func NewShelf(inner depo.Shelf, pool ergograph.Pool, tracer trace.Tracer, name string) *Shelf {
	return &Shelf{inner: inner, pool: pool, tracer: tracer, name: name}
}

func (s *Shelf) Push(envelope artifact.Envelope) error {
	ctx, span := s.tracer.Start(context.Background(), "shelf.push")
	defer span.End()
	err := s.inner.Push(envelope)
	spanError(span, err)
	record(ctx, s.pool, "shelf.push", map[string]string{"shelf": s.name})
	return err
}

func (s *Shelf) Pull(agentID string) (artifact.Envelope, error) {
	ctx, span := s.tracer.Start(context.Background(), "shelf.pull")
	defer span.End()
	env, err := s.inner.Pull(agentID)
	spanError(span, err)
	record(ctx, s.pool, "shelf.pull", map[string]string{"shelf": s.name, "agent": agentID})
	return env, err
}

func (s *Shelf) Peek() []artifact.Envelope {
	_, span := s.tracer.Start(context.Background(), "shelf.peek")
	defer span.End()
	return s.inner.Peek()
}

func (s *Shelf) Watch() <-chan artifact.Envelope {
	_, span := s.tracer.Start(context.Background(), "shelf.watch")
	defer span.End()
	return s.inner.Watch()
}
