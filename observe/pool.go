package observe

import (
	"context"

	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/trace"
)

type Pool struct {
	inner  ergograph.Pool
	tracer trace.Tracer
	name   string
}

var _ ergograph.Pool = (*Pool)(nil)

func NewPool(inner ergograph.Pool, tracer trace.Tracer, name string) *Pool {
	return &Pool{inner: inner, tracer: tracer, name: name}
}

func (p *Pool) Append(rec ergograph.Record) error {
	ctx, span := p.tracer.Start(context.Background(), "pool.append")
	defer span.End()
	err := p.inner.Append(rec)
	spanError(span, err)
	record(ctx, p.inner, "observe.pool.append", map[string]string{
		"pool":   p.name,
		"action": rec.Action,
	})
	return err
}

func (p *Pool) Records() []ergograph.Record {
	_, span := p.tracer.Start(context.Background(), "pool.records")
	defer span.End()
	return p.inner.Records()
}

func (p *Pool) VerifyChain() error {
	_, span := p.tracer.Start(context.Background(), "pool.verify_chain")
	defer span.End()
	err := p.inner.VerifyChain()
	spanError(span, err)
	return err
}

func (p *Pool) Len() int {
	_, span := p.tracer.Start(context.Background(), "pool.len")
	defer span.End()
	return p.inner.Len()
}
