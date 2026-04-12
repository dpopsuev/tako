// circuit_director.go — CircuitDirector implements troupe.Director.
//
// Bridges engine.Run() with Troupe's event streaming contract.
// WalkObserver events are converted to troupe.Event and sent on a channel.
package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/troupe"
)

// CircuitDetail carries circuit-specific data in a troupe.Event.
// Implements troupe.EventDetail (fmt.Stringer).
type CircuitDetail struct {
	Node       string  `json:"node,omitempty"`
	Edge       string  `json:"edge,omitempty"`
	Artifact   string  `json:"artifact,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

func (d CircuitDetail) String() string {
	if d.Artifact != "" {
		return fmt.Sprintf("node=%s artifact=%s conf=%.2f", d.Node, d.Artifact, d.Confidence)
	}
	return fmt.Sprintf("node=%s", d.Node)
}

// CircuitDirector implements troupe.Director by wrapping engine.Run().
// Each walk event is bridged to the troupe.Event stream.
type CircuitDirector struct {
	CircuitPath string
	Input       any
	Options     []RunOption
}

// Direct executes the circuit and returns a streaming event channel.
// The channel is closed when the circuit completes (or fails).
func (d *CircuitDirector) Direct(ctx context.Context, broker troupe.Broker) (<-chan troupe.Event, error) {
	ch := make(chan troupe.Event, 64) //nolint:mnd // buffer for burst events

	// Bridge WalkObserver → channel.
	observer := &channelObserver{ch: ch}

	opts := make([]RunOption, 0, len(d.Options)+1)
	opts = append(opts, d.Options...)
	opts = append(opts, WithRunObserver(observer))

	go func() {
		defer close(ch)
		start := time.Now()

		err := Run(ctx, d.CircuitPath, d.Input, opts...)
		if err != nil {
			ch <- troupe.Event{
				Kind:    troupe.Failed,
				Step:    "circuit",
				Error:   err,
				Elapsed: time.Since(start),
			}
		}
		ch <- troupe.Event{
			Kind:    troupe.Done,
			Elapsed: time.Since(start),
		}
	}()

	return ch, nil
}

// Event kind aliases from agentport (troupe).
var (
	eventStarted    = troupe.Started
	eventCompleted  = troupe.Completed
	eventFailed     = troupe.Failed
	eventTransition = troupe.Transition
)

// channelObserver bridges WalkObserver events to a troupe.Event channel.
type channelObserver struct {
	ch chan<- troupe.Event
}

func (o *channelObserver) OnEvent(e *circuit.WalkEvent) {
	var ev troupe.Event

	switch e.Type {
	case circuit.EventNodeEnter:
		ev = troupe.Event{
			Kind:  eventStarted,
			Step:  e.Node,
			Agent: e.Walker,
			Detail: CircuitDetail{
				Node: e.Node,
			},
		}
	case circuit.EventNodeExit:
		ev = troupe.Event{
			Kind:    eventCompleted,
			Step:    e.Node,
			Agent:   e.Walker,
			Elapsed: e.Elapsed,
			Detail: CircuitDetail{
				Node: e.Node,
			},
		}
		if e.Error != nil {
			ev.Kind = eventFailed
			ev.Error = e.Error
		}
	case circuit.EventTransition:
		ev = troupe.Event{
			Kind:  eventTransition,
			Step:  e.Node,
			Agent: e.Walker,
			Detail: CircuitDetail{
				Node: e.Node,
				Edge: e.Edge,
			},
		}
	default:
		return // skip non-essential events
	}

	select {
	case o.ch <- ev:
	default:
		// drop if channel full — non-blocking
	}
}

// Compile-time check: CircuitDirector satisfies the Director-like pattern.
// Note: troupe.Director accepts troupe.Broker; troupe.Broker is a type alias.
