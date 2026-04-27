package instrument

import "context"

// Tool is the interface every instrument implements.
type Tool interface {
	Name() string
	Signature() string
	Manual() string
}

// Executor runs an instrument by name.
type Executor interface {
	Exec(ctx context.Context, name string, input []byte) (Result, error)
}
