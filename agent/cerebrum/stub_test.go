package cerebrum

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

type stubCompleter struct {
	response  string
	toolCalls []tangle.ToolCall
	err       error
}

func (s *stubCompleter) Complete(_ context.Context, params tangle.CompletionParams) (*tangle.Completion, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.toolCalls) > 0 {
		return &tangle.Completion{Content: s.response, ToolCalls: s.toolCalls}, nil
	}
	if s.response != "" {
		tc := phaseToolCallForResponse(s.response, params)
		return &tangle.Completion{ToolCalls: []tangle.ToolCall{tc}}, nil
	}
	return &tangle.Completion{Content: s.response}, nil
}

func phaseToolCallForResponse(response string, _ tangle.CompletionParams) tangle.ToolCall {
	input, _ := json.Marshal(map[string]string{"response": response})
	return tangle.ToolCall{
		ID:    "stub-tc",
		Name:  "speak",
		Input: input,
	}
}

type stubBus struct {
	mu     sync.Mutex
	events []Event
}

var _ Bus = (*stubBus)(nil)

func (s *stubBus) Send(_ context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *stubBus) Receive(_ context.Context) (Event, bool) {
	return Event{}, false
}

func (s *stubBus) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event(nil), s.events...)
}

type stubSynapse struct {
	encodeCalled int
	decodeCalled int
}

func (s *stubSynapse) Encode(e Event) (reactivity.Atom, error) {
	s.encodeCalled++
	return DefaultSynapse{}.Encode(e)
}

func (s *stubSynapse) Decode(e reactivity.Emission) Event {
	s.decodeCalled++
	return DefaultSynapse{}.Decode(e)
}

type emittingTriadReactor struct{}

func (emittingTriadReactor) React(m *reactivity.Molecule, atom reactivity.Atom) (reactivity.YieldKind, reactivity.Yield) {
	m.Emit(reactivity.Emission{Kind: "instrument", Target: "emitted-tool", Payload: atom.Content})
	if m.Mass(reactivity.AssessmentAtom) > 0 {
		m.SealTriad(reactivity.ThinkTriad)
		m.SetPhase(reactivity.ExpansionAtom)
		return reactivity.Pass, reactivity.Yield{}
	}
	if m.Phase() == reactivity.IntentAtom && m.Mass(reactivity.IntentAtom) > 0 {
		m.SetPhase(reactivity.AssessmentAtom)
		return reactivity.Pass, reactivity.Yield{}
	}
	return reactivity.Insufficient, reactivity.Yield{Result: reactivity.Insufficient}
}
