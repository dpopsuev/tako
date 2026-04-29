package observe

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/instrument"
	"go.opentelemetry.io/otel/trace"
)

type Shell struct {
	inner  instrument.Shell
	pool   ergograph.Pool
	tracer trace.Tracer
	name   string
}

var _ instrument.Shell = (*Shell)(nil)

func NewShell(inner instrument.Shell, pool ergograph.Pool, tracer trace.Tracer, name string) *Shell {
	return &Shell{inner: inner, pool: pool, tracer: tracer, name: name}
}

func (s *Shell) Exec(ctx context.Context, name string, input json.RawMessage) (instrument.Result, error) {
	ctx, span := s.tracer.Start(ctx, "shell.exec")
	defer span.End()
	result, err := s.inner.Exec(ctx, name, input)
	spanError(span, err)
	record(ctx, s.pool, "shell.exec", map[string]string{"shell": s.name, "instrument": name})
	return result, err
}

func (s *Shell) Names() []string {
	_, span := s.tracer.Start(context.Background(), "shell.names")
	defer span.End()
	return s.inner.Names()
}

func (s *Shell) Describe(name string) (string, error) {
	_, span := s.tracer.Start(context.Background(), "shell.describe")
	defer span.End()
	desc, err := s.inner.Describe(name)
	spanError(span, err)
	return desc, err
}

func (s *Shell) Schema(name string) (json.RawMessage, error) {
	_, span := s.tracer.Start(context.Background(), "shell.schema")
	defer span.End()
	schema, err := s.inner.Schema(name)
	spanError(span, err)
	return schema, err
}
