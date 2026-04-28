package stubs

import (
	"context"
	"sync"

	"github.com/dpopsuev/tako/testkit"
)

// StubTransport implements testkit.Transport with call tracking.
// Thread-safe, supports error injection.
type StubTransport struct {
	mu    sync.Mutex
	calls []string
	err   error
}

// NewStubTransport creates a transport stub.
func NewStubTransport() *StubTransport {
	return &StubTransport{}
}

func (s *StubTransport) Serve(ctx context.Context, handler testkit.TransportHandler) error {
	s.mu.Lock()
	s.calls = append(s.calls, "Serve")
	err := s.err
	s.mu.Unlock()
	if err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

func (s *StubTransport) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "Shutdown")
	if s.err != nil {
		return s.err
	}
	return nil
}

// SetError injects an error returned by subsequent operations.
func (s *StubTransport) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// Calls returns a copy of the call log.
func (s *StubTransport) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

// Reset clears call tracking and errors.
func (s *StubTransport) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
	s.err = nil
}

// StubTrigger implements testkit.Trigger with call tracking.
type StubTrigger struct {
	mu     sync.Mutex
	calls  []string
	handle testkit.SessionHandle
	params testkit.TriggerParams
	err    error
}

// NewStubTrigger creates a trigger stub.
func NewStubTrigger() *StubTrigger {
	return &StubTrigger{}
}

func (s *StubTrigger) Start(ctx context.Context, params testkit.TriggerParams) (testkit.SessionHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "Start")
	s.params = params
	if s.err != nil {
		return nil, s.err
	}
	return s.handle, nil
}

// WithHandle sets the canned session handle returned by Start.
func (s *StubTrigger) WithHandle(h testkit.SessionHandle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handle = h
}

// Calls returns a copy of the call log.
func (s *StubTrigger) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

// LastParams returns the params from the most recent Start call.
func (s *StubTrigger) LastParams() testkit.TriggerParams {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.params
}

// SetError injects an error returned by Start.
func (s *StubTrigger) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// StubSessionHandle implements testkit.SessionHandle.
type StubSessionHandle struct {
	mu     sync.Mutex
	id     string
	done   chan struct{}
	result any
	err    error
}

// NewStubSessionHandle creates a session handle with the given ID.
func NewStubSessionHandle(id string) *StubSessionHandle {
	return &StubSessionHandle{
		id:   id,
		done: make(chan struct{}),
	}
}

func (h *StubSessionHandle) ID() string            { return h.id }
func (h *StubSessionHandle) Done() <-chan struct{} { return h.done }

func (h *StubSessionHandle) Result() any {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.result
}

func (h *StubSessionHandle) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func (h *StubSessionHandle) Cancel() {
	select {
	case <-h.done:
	default:
		close(h.done)
	}
}

// Close signals the session is done (same as Cancel, for test convenience).
func (h *StubSessionHandle) Close() {
	h.Cancel()
}

// SetResult sets the result returned by Result().
func (h *StubSessionHandle) SetResult(v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.result = v
}

// SetErr sets the error returned by Err().
func (h *StubSessionHandle) SetErr(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.err = err
}
