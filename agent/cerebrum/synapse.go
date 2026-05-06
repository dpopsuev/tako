package cerebrum

import (
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type Synapse interface {
	Encode(event Event) (reactivity.Atom, error)
	Decode(emission reactivity.Emission) Event
}

type DefaultSynapse struct{}

func (DefaultSynapse) Encode(e Event) (reactivity.Atom, error) {
	return reactivity.Atom{
		ID:        e.ID,
		Type:      reactivity.IntentAtom,
		Source:    reactivity.Received,
		Taxonomy:  "intent." + e.Kind,
		Content:   e.Payload,
		CreatedAt: e.CreatedAt,
	}, nil
}

func (DefaultSynapse) Decode(e reactivity.Emission) Event {
	return Event{
		ID:         fmt.Sprintf("emission-%d", time.Now().UnixNano()),
		Kind:       e.Kind,
		Source:     e.Target,
		Payload:    e.Payload,
		ToolCallID: e.ToolCallID,
		CreatedAt:  time.Now(),
	}
}
