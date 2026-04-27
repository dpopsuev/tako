package instrument

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("instrument: not found")

// EchoTool is a stub instrument that returns its input as output.
type EchoTool struct{}

var _ Tool = EchoTool{}

func (EchoTool) Name() string      { return "echo" }
func (EchoTool) Signature() string { return "echo(input []byte) -> Result" }
func (EchoTool) Manual() string    { return "Returns input unchanged." }

// StubShell is an in-memory shell with a single echo instrument.
type StubShell struct {
	tools map[string]Tool
}

var _ Shell = (*StubShell)(nil)

// NewStubShell creates a shell pre-loaded with the echo instrument.
func NewStubShell() *StubShell {
	return &StubShell{
		tools: map[string]Tool{"echo": EchoTool{}},
	}
}

func (s *StubShell) Names() []string {
	out := make([]string, 0, len(s.tools))
	for name := range s.tools {
		out = append(out, name)
	}
	return out
}

func (s *StubShell) Signature(name string) (string, error) {
	t, ok := s.tools[name]
	if !ok {
		return "", ErrNotFound
	}
	return t.Signature(), nil
}

func (s *StubShell) Manual(name string) (string, error) {
	t, ok := s.tools[name]
	if !ok {
		return "", ErrNotFound
	}
	return t.Manual(), nil
}

func (s *StubShell) Exec(_ context.Context, name string, input []byte) (Result, error) {
	if _, ok := s.tools[name]; !ok {
		return Result{}, ErrNotFound
	}
	return Result{
		Content:   input,
		Structure: Blob,
		ExitCode:  0,
		Duration:  time.Millisecond,
	}, nil
}
