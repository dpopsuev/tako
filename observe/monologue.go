package observe

import (
	"context"

	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/trace"
)

type Monolog struct {
	inner  discourse.Monolog
	pool   ergograph.Ledger
	tracer trace.Tracer
	name   string
}

var _ discourse.Monolog = (*Monolog)(nil)

func NewMonolog(inner discourse.Monolog, pool ergograph.Ledger, tracer trace.Tracer, name string) *Monolog {
	return &Monolog{inner: inner, pool: pool, tracer: tracer, name: name}
}

func (m *Monolog) Pin(topic string) {
	ctx, span := m.tracer.Start(context.Background(), "monolog.pin")
	defer span.End()
	m.inner.Pin(topic)
	record(ctx, m.pool, "monolog.pin", map[string]string{"monolog": m.name, "topic": topic})
}

func (m *Monolog) Focus(topic string) {
	ctx, span := m.tracer.Start(context.Background(), "monolog.focus")
	defer span.End()
	m.inner.Focus(topic)
	record(ctx, m.pool, "monolog.focus", map[string]string{"monolog": m.name, "topic": topic})
}

func (m *Monolog) Write(letter discourse.Letter) {
	ctx, span := m.tracer.Start(context.Background(), "monolog.write")
	defer span.End()
	m.inner.Write(letter)
	record(ctx, m.pool, "monolog.write", map[string]string{"monolog": m.name, "from": letter.From, "subject": letter.Subject})
}

func (m *Monolog) Letters() []discourse.Letter {
	_, span := m.tracer.Start(context.Background(), "monolog.letters")
	defer span.End()
	return m.inner.Letters()
}
