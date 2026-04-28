package observe

import (
	"context"

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

func (s *Shell) Exec(ctx context.Context, name string, input []byte) (instrument.Result, error) {
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

func (s *Shell) Signature(name string) (string, error) {
	_, span := s.tracer.Start(context.Background(), "shell.signature")
	defer span.End()
	sig, err := s.inner.Signature(name)
	spanError(span, err)
	return sig, err
}

func (s *Shell) Manual(name string) (string, error) {
	_, span := s.tracer.Start(context.Background(), "shell.manual")
	defer span.End()
	man, err := s.inner.Manual(name)
	spanError(span, err)
	return man, err
}
