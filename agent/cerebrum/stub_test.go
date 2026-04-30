package cerebrum

import "context"

type stubCompleter struct {
	response string
	err      error
}

func (s *stubCompleter) Complete(_ context.Context, _ string) (string, error) {
	return s.response, s.err
}

type stubMotorBus struct {
	commands []Command
}

func (s *stubMotorBus) Send(_ context.Context, cmd Command) error {
	s.commands = append(s.commands, cmd)
	return nil
}
