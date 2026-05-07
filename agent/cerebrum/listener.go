package cerebrum

import "time"

type ContextListener interface {
	OnContext(ctx Context, turn int)
	OnToolCall(name string, input []byte)
	OnToolResult(name string, result []byte, elapsed time.Duration)
	OnSealed(moleculeID string, distance float64, turns int, result string)
	OnError(turn int, err error)
}
