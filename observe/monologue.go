package observe

import (
	"context"

	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/trace"
)

type Monologue struct {
	inner  discourse.Monologue
	pool   ergograph.Pool
	tracer trace.Tracer
	name   string
}

var _ discourse.Monologue = (*Monologue)(nil)

func NewMonologue(inner discourse.Monologue, pool ergograph.Pool, tracer trace.Tracer, name string) *Monologue {
	return &Monologue{inner: inner, pool: pool, tracer: tracer, name: name}
}

func (m *Monologue) Pin(topic string) {
	ctx, span := m.tracer.Start(context.Background(), "monologue.pin")
	defer span.End()
	m.inner.Pin(topic)
	record(ctx, m.pool, "monologue.pin", map[string]string{"monologue": m.name, "topic": topic})
}

func (m *Monologue) Focus(topic string) {
	ctx, span := m.tracer.Start(context.Background(), "monologue.focus")
	defer span.End()
	m.inner.Focus(topic)
	record(ctx, m.pool, "monologue.focus", map[string]string{"monologue": m.name, "topic": topic})
}

func (m *Monologue) Write(letter discourse.Letter) {
	ctx, span := m.tracer.Start(context.Background(), "monologue.write")
	defer span.End()
	m.inner.Write(letter)
	record(ctx, m.pool, "monologue.write", map[string]string{"monologue": m.name, "from": letter.From, "subject": letter.Subject})
}

func (m *Monologue) Letters() []discourse.Letter {
	_, span := m.tracer.Start(context.Background(), "monologue.letters")
	defer span.End()
	return m.inner.Letters()
}
