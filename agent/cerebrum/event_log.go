package cerebrum

import (
	"sync"
	"time"
)

type ToolEvent struct {
	Name    string        `json:"name"`
	Input   []byte        `json:"input,omitempty"`
	Result  []byte        `json:"result,omitempty"`
	Elapsed time.Duration `json:"elapsed_ms,omitempty"`
}

type ContextSnapshot struct {
	Phase    string  `json:"phase"`
	Turn     int     `json:"turn"`
	Distance float64 `json:"distance"`
}

type EventLog struct {
	mu         sync.Mutex
	Contexts   []ContextSnapshot `json:"contexts"`
	ToolCalls  []ToolEvent       `json:"tool_calls"`
	Responses  []string          `json:"responses"`
	Errors     []string          `json:"errors,omitempty"`
	TokensIn   int               `json:"tokens_in"`
	TokensOut  int               `json:"tokens_out"`
	TotalTools int               `json:"total_tools"`
}

func NewEventLog() *EventLog {
	return &EventLog{}
}

func (l *EventLog) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Contexts = nil
	l.ToolCalls = nil
	l.Responses = nil
	l.Errors = nil
	l.TokensIn = 0
	l.TokensOut = 0
	l.TotalTools = 0
}

var _ ContextListener = (*EventLog)(nil)

func (l *EventLog) OnContext(phase string, turn int, distance float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Contexts = append(l.Contexts, ContextSnapshot{Phase: phase, Turn: turn, Distance: distance})
}

func (l *EventLog) OnToolCall(name string, input []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ToolCalls = append(l.ToolCalls, ToolEvent{Name: name, Input: append([]byte(nil), input...)})
}

func (l *EventLog) OnToolResult(name string, result []byte, elapsed time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.ToolCalls) - 1; i >= 0; i-- {
		if l.ToolCalls[i].Name == name && l.ToolCalls[i].Result == nil {
			l.ToolCalls[i].Result = append([]byte(nil), result...)
			l.ToolCalls[i].Elapsed = elapsed
			break
		}
	}
}

func (l *EventLog) OnResponse(text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Responses = append(l.Responses, text)
}

func (l *EventLog) OnTokenUpdate(tokensIn, tokensOut, toolCalls int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.TokensIn += tokensIn
	l.TokensOut += tokensOut
	l.TotalTools += toolCalls
}

func (l *EventLog) OnSealed(_ string, _ float64, _ int) {}

func (l *EventLog) OnError(_ int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Errors = append(l.Errors, err.Error())
}

func (l *EventLog) OnToken(_ string) {}
