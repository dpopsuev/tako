package cerebrum

import "time"

type ContextListener interface {
	OnContext(phase string, turn int, distance float64)
	OnToolCall(name string, input []byte)
	OnToolResult(name string, result []byte, elapsed time.Duration)
	OnTokenUpdate(tokensIn, tokensOut, toolCalls int)
	OnSealed(moleculeID string, distance float64, turns int, result string)
	OnError(turn int, err error)
	OnToken(token string)
}
