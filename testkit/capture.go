package testkit

import (
	"sync"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

type ToolEvent struct {
	Name    string
	Input   []byte
	Result  []byte
	Elapsed time.Duration
}

type CapturingListener struct {
	mu         sync.Mutex
	Contexts   []ContextEvent
	ToolCalls  []ToolEvent
	Responses  []string
	Sealed     bool
	SealedID   string
	SealDist   float64
	SealTurns  int
	Errors     []error
	TokensIn   int
	TokensOut  int
	TotalTools int
}

type ContextEvent struct {
	Phase    string
	Turn     int
	Distance float64
}

var _ cerebrum.ContextListener = (*CapturingListener)(nil)

func NewCapturingListener() *CapturingListener {
	return &CapturingListener{}
}

func (c *CapturingListener) OnContext(phase string, turn int, distance float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Contexts = append(c.Contexts, ContextEvent{Phase: phase, Turn: turn, Distance: distance})
}

func (c *CapturingListener) OnToolCall(name string, input []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ToolCalls = append(c.ToolCalls, ToolEvent{Name: name, Input: append([]byte(nil), input...)})
}

func (c *CapturingListener) OnToolResult(name string, result []byte, elapsed time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.ToolCalls) - 1; i >= 0; i-- {
		if c.ToolCalls[i].Name == name && c.ToolCalls[i].Result == nil {
			c.ToolCalls[i].Result = append([]byte(nil), result...)
			c.ToolCalls[i].Elapsed = elapsed
			break
		}
	}
}

func (c *CapturingListener) OnResponse(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Responses = append(c.Responses, text)
}

func (c *CapturingListener) OnTokenUpdate(tokensIn, tokensOut, toolCalls int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.TokensIn += tokensIn
	c.TokensOut += tokensOut
	c.TotalTools += toolCalls
}

func (c *CapturingListener) OnSealed(id string, distance float64, turns int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Sealed = true
	c.SealedID = id
	c.SealDist = distance
	c.SealTurns = turns
}

func (c *CapturingListener) OnError(turn int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Errors = append(c.Errors, err)
}

func (c *CapturingListener) OnToken(_ string) {}
