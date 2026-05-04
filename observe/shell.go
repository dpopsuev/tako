package observe

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/agent/shell"
	"go.opentelemetry.io/otel/trace"
)

type Shell struct {
	inner  shell.Shell
	pool   ergograph.Ledger
	tracer trace.Tracer
	name   string
}

var _ shell.Shell = (*Shell)(nil)

func NewShell(inner shell.Shell, pool ergograph.Ledger, tracer trace.Tracer, name string) *Shell {
	return &Shell{inner: inner, pool: pool, tracer: tracer, name: name}
}

func (s *Shell) Exec(ctx context.Context, name string, input json.RawMessage) (shell.Result, error) {
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

func (s *Shell) Mode(name string) shell.ActionMode         { return s.inner.Mode(name) }
func (s *Shell) Approval(name string) shell.ActionApproval { return s.inner.Approval(name) }
func (s *Shell) Risk(name string) float64                  { return s.inner.Risk(name) }

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
