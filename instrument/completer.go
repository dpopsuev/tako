package instrument

import "context"

// Completer is the seam for LLM calls. Prompt in, response out.
// The caller doesn't know the backend: stub, Tangle Caster, direct SDK, replay.
type Completer interface {
	Complete(ctx context.Context, prompt []byte) ([]byte, error)
}

// StubCompleter returns a canned response for testing.
type StubCompleter struct {
	Response []byte
	Err      error
}

var _ Completer = (*StubCompleter)(nil)

func (s *StubCompleter) Complete(_ context.Context, _ []byte) ([]byte, error) {
	return s.Response, s.Err
}
