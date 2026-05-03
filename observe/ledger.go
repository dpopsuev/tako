package observe

import (
	"context"

	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/trace"
)

type Ledger struct {
	inner  ergograph.Ledger
	tracer trace.Tracer
	name   string
}

var _ ergograph.Ledger = (*Ledger)(nil)

func NewLedger(inner ergograph.Ledger, tracer trace.Tracer, name string) *Ledger {
	return &Ledger{inner: inner, tracer: tracer, name: name}
}

func (l *Ledger) Append(rec ergograph.Record) error {
	ctx, span := l.tracer.Start(context.Background(), "pool.append")
	defer span.End()
	err := l.inner.Append(rec)
	spanError(span, err)
	record(ctx, l.inner, "observe.pool.append", map[string]string{
		"pool":   l.name,
		"action": rec.Action,
	})
	return err
}

func (l *Ledger) Records() []ergograph.Record {
	_, span := l.tracer.Start(context.Background(), "pool.records")
	defer span.End()
	return l.inner.Records()
}

func (l *Ledger) VerifyChain() error {
	_, span := l.tracer.Start(context.Background(), "pool.verify_chain")
	defer span.End()
	err := l.inner.VerifyChain()
	spanError(span, err)
	return err
}

func (l *Ledger) Len() int {
	_, span := l.tracer.Start(context.Background(), "pool.len")
	defer span.End()
	return l.inner.Len()
}
