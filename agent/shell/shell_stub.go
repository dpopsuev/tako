package shell

import (
	"context"
	"encoding/json"
	"errors"
)

var ErrNotFound = errors.New("instrument: not found")

// EchoFunction is a stub instrument that returns its input as text output.
type EchoFunction struct{}

var _ Function = EchoFunction{}

func (EchoFunction) Name() string              { return "echo" }
func (EchoFunction) Description() string       { return "Returns input unchanged." }
func (EchoFunction) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}}}`) }

func (EchoFunction) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	return TextResult(string(input)), nil
}

// StubShell is an in-memory shell backed by registered Functions.
type StubShell struct {
	functions map[string]Function
}

var _ Shell = (*StubShell)(nil)

func NewStubShell() *StubShell {
	return &StubShell{
		functions: map[string]Function{"echo": EchoFunction{}},
	}
}

func NewShellWith(fns ...Function) *StubShell {
	s := &StubShell{functions: make(map[string]Function, len(fns))}
	for _, fn := range fns {
		s.functions[fn.Name()] = fn
	}
	return s
}

func (s *StubShell) Names() []string {
	out := make([]string, 0, len(s.functions))
	for name := range s.functions {
		out = append(out, name)
	}
	return out
}

func (s *StubShell) Mode(_ string) ActionMode        { return ReadAction }
func (s *StubShell) Approval(_ string) ActionApproval { return Auto }
func (s *StubShell) Risk(_ string) float64            { return 0 }

func (s *StubShell) Describe(name string) (string, error) {
	fn, ok := s.functions[name]
	if !ok {
		return "", ErrNotFound
	}
	return fn.Description(), nil
}

func (s *StubShell) Schema(name string) (json.RawMessage, error) {
	fn, ok := s.functions[name]
	if !ok {
		return nil, ErrNotFound
	}
	return fn.InputSchema(), nil
}

func (s *StubShell) Exec(ctx context.Context, name string, input json.RawMessage) (Result, error) {
	fn, ok := s.functions[name]
	if !ok {
		return Result{}, ErrNotFound
	}
	return fn.Execute(ctx, input)
}
