package shell

import (
	"context"
	"encoding/json"
)

type functionEntry struct {
	fn       Function
	mode     ActionMode
	risk     float64
	approval ActionApproval
}

// FunctionShell is a configurable Shell with per-function mode, risk, and approval.
type FunctionShell struct {
	entries map[string]*functionEntry
	order   []string
}

var _ Shell = (*FunctionShell)(nil)

func NewFunctionShell() *FunctionShell {
	return &FunctionShell{entries: make(map[string]*functionEntry)}
}

func (s *FunctionShell) Add(fn Function, mode ActionMode, risk float64) {
	name := fn.Name()
	s.entries[name] = &functionEntry{fn: fn, mode: mode, risk: risk}
	s.order = append(s.order, name)
}

func (s *FunctionShell) AddWithApproval(fn Function, mode ActionMode, risk float64, approval ActionApproval) {
	name := fn.Name()
	s.entries[name] = &functionEntry{fn: fn, mode: mode, risk: risk, approval: approval}
	s.order = append(s.order, name)
}

func (s *FunctionShell) Names() []string {
	return append([]string(nil), s.order...)
}

func (s *FunctionShell) Describe(name string) (string, error) {
	e, ok := s.entries[name]
	if !ok {
		return "", ErrNotFound
	}
	return e.fn.Description(), nil
}

func (s *FunctionShell) Schema(name string) (json.RawMessage, error) {
	e, ok := s.entries[name]
	if !ok {
		return nil, ErrNotFound
	}
	return e.fn.InputSchema(), nil
}

func (s *FunctionShell) Mode(name string) ActionMode {
	e, ok := s.entries[name]
	if !ok {
		return ReadAction
	}
	return e.mode
}

func (s *FunctionShell) Approval(name string) ActionApproval {
	e, ok := s.entries[name]
	if !ok {
		return Auto
	}
	return e.approval
}

func (s *FunctionShell) Risk(name string) float64 {
	e, ok := s.entries[name]
	if !ok {
		return 0
	}
	return e.risk
}

func (s *FunctionShell) Exec(ctx context.Context, name string, input json.RawMessage) (Result, error) {
	e, ok := s.entries[name]
	if !ok {
		return Result{}, ErrNotFound
	}
	return e.fn.Execute(ctx, input)
}
