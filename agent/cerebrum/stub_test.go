package cerebrum

import "context"

type stubCompleter struct {
	response string
	err      error
}

func (s *stubCompleter) Complete(_ context.Context, _ string) (string, error) {
	return s.response, s.err
}
