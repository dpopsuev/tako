package cerebrum

import "context"

type Command struct {
	Kind    string
	Target  string
	Payload []byte
}

type MotorBus interface {
	Send(ctx context.Context, cmd Command) error
}
