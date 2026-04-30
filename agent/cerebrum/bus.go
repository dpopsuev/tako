package cerebrum

import "context"

type Signal struct {
	Kind    string
	Topic   string
	Content []byte
}

type Command struct {
	Kind    string
	Target  string
	Payload []byte
}

type SensoryBus interface {
	Receive(ctx context.Context) (Signal, bool)
}

type MotorBus interface {
	Send(ctx context.Context, cmd Command) error
}
