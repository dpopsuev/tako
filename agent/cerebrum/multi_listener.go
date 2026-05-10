package cerebrum

import "time"

type multiListener struct {
	listeners []ContextListener
}

func newMultiListener(listeners ...ContextListener) *multiListener {
	var filtered []ContextListener
	for _, l := range listeners {
		if l != nil {
			filtered = append(filtered, l)
		}
	}
	return &multiListener{listeners: filtered}
}

func (m *multiListener) OnContext(phase string, turn int, distance float64) {
	for _, l := range m.listeners {
		l.OnContext(phase, turn, distance)
	}
}

func (m *multiListener) OnToolCall(name string, input []byte) {
	for _, l := range m.listeners {
		l.OnToolCall(name, input)
	}
}

func (m *multiListener) OnToolResult(name string, result []byte, elapsed time.Duration) {
	for _, l := range m.listeners {
		l.OnToolResult(name, result, elapsed)
	}
}

func (m *multiListener) OnResponse(text string) {
	for _, l := range m.listeners {
		l.OnResponse(text)
	}
}

func (m *multiListener) OnTokenUpdate(tokensIn, tokensOut, toolCalls int) {
	for _, l := range m.listeners {
		l.OnTokenUpdate(tokensIn, tokensOut, toolCalls)
	}
}

func (m *multiListener) OnSealed(id string, distance float64, turns int) {
	for _, l := range m.listeners {
		l.OnSealed(id, distance, turns)
	}
}

func (m *multiListener) OnError(turn int, err error) {
	for _, l := range m.listeners {
		l.OnError(turn, err)
	}
}

func (m *multiListener) OnToken(token string) {
	for _, l := range m.listeners {
		l.OnToken(token)
	}
}

var _ ContextListener = (*multiListener)(nil)
