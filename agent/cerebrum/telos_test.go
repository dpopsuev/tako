package cerebrum

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func speakCap() organ.Func {
	return organ.Func{
		Name:        "dialog_speak",
		Description: "Respond to the operator",
		Schema:      json.RawMessage(`{"type":"object","properties":{"response":{"type":"string"}},"required":["response"]}`),
		Mode:        organ.ReadAction,
		Source:      organ.BuiltIn,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			var args struct{ Response string `json:"response"` }
			json.Unmarshal(input, &args)
			return organ.TextResult(args.Response), nil
		},
	}
}

func TestTelos_SimpleConversation_SealOnNoToolCalls(t *testing.T) {
	speak := speakCap()
	completer := &stubCompleter{
		toolCalls: []tangle.ToolCall{
			{ID: "s1", Name: "dialog_speak", Input: json.RawMessage(`{"response":"Cows are bovine mammals."}`)},
		},
		toolCallOnce: true,
		response:     "here is the answer",
	}

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog_speak": speak},
		sensory: NewChannelBus(64),
	}

	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithCapabilities([]organ.Func{speak}),
		WithMaxTurns(5),
		WithTurnTimeout(5*time.Second),
	)

	cb.Think(context.Background(), reactivity.Catalyst{Need: "Tell me about cows"})
	m := cb.Result()

	if !m.Sealed() {
		t.Fatal("molecule should seal after dialog_speak with no more tool calls")
	}
	if m.Response() == "" {
		t.Error("molecule should have a response")
	}
	t.Logf("turns=%d response=%q", m.Turns(), m.Response())
}

func TestTelos_TaskWithDesired_SealOnDistanceZero(t *testing.T) {
	speak := speakCap()
	completer := &stubCompleter{
		toolCalls: []tangle.ToolCall{
			{ID: "s1", Name: "dialog_speak", Input: json.RawMessage(`{"response":"Cows produce milk."}`)},
		},
		toolCallOnce: true,
		response:     "done",
	}

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog_speak": speak},
		sensory: NewChannelBus(64),
	}

	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithCapabilities([]organ.Func{speak}),
		WithMaxTurns(5),
		WithTurnTimeout(5*time.Second),
	)

	catalyst := reactivity.Catalyst{
		Need:    "Do cows produce milk?",
		Desired: map[string]any{"answered": true},
	}

	cb.Think(context.Background(), catalyst)
	m := cb.Result()

	if !m.Sealed() {
		t.Fatal("molecule should seal")
	}
	t.Logf("turns=%d distance=%.2f", m.Turns(), m.Distance())
}

func TestTelos_DialogDoesNotSeal_JustAddsAtom(t *testing.T) {
	speak := speakCap()
	callCount := 0
	completer := tangle.CompleteFunc(func(_ context.Context, params tangle.CompletionParams) (*tangle.Completion, error) {
		callCount++
		switch callCount {
		case 1:
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: "s1", Name: "dialog_speak", Input: json.RawMessage(`{"response":"Tell me more about which animal?"}`)},
				},
			}, nil
		default:
			return &tangle.Completion{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"waiting for operator"}]}`,
			}, nil
		}
	})

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog_speak": speak},
		sensory: NewChannelBus(64),
	}

	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithCapabilities([]organ.Func{speak}),
		WithMaxTurns(3),
		WithTurnTimeout(5*time.Second),
	)

	cb.Think(context.Background(), reactivity.Catalyst{Need: "Tell me about animals"})
	m := cb.Result()

	chain := m.Chain()
	speakEvents := 0
	for _, e := range chain.All() {
		if e.Organ == "dialog_speak" {
			speakEvents++
		}
	}

	if speakEvents == 0 {
		t.Error("dialog_speak should appear in EventChain as execution, not as seal trigger")
	}

	t.Logf("turns=%d chain_events=%d speak_events=%d sealed=%v",
		m.Turns(), chain.Len(), speakEvents, m.Sealed())
}

func TestTelos_EventChain_TracksConversationFlow(t *testing.T) {
	speak := speakCap()
	readAnimal := organ.Func{
		Name:        "read_animal",
		Description: "Look up animal facts",
		Schema:      json.RawMessage(`{"type":"object","properties":{"animal":{"type":"string"}},"required":["animal"]}`),
		Mode:        organ.ReadAction,
		Source:      organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			var args struct{ Animal string `json:"animal"` }
			json.Unmarshal(input, &args)
			facts := map[string]string{
				"cow":     "Cows have four stomachs and produce milk.",
				"chicken": "Chickens can run up to 9 mph.",
			}
			if f, ok := facts[args.Animal]; ok {
				return organ.TextResult(f), nil
			}
			return organ.TextResult("Unknown animal."), nil
		},
	}

	callCount := 0
	completer := tangle.CompleteFunc(func(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
		callCount++
		switch callCount {
		case 1:
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: "r1", Name: "read_animal", Input: json.RawMessage(`{"animal":"cow"}`)},
				},
			}, nil
		case 2:
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: "s1", Name: "dialog_speak", Input: json.RawMessage(`{"response":"Cows have four stomachs and produce milk."}`)},
				},
			}, nil
		default:
			return &tangle.Completion{Content: "done"}, nil
		}
	})

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog_speak": speak, "read_animal": readAnimal},
		sensory: NewChannelBus(64),
	}

	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithCapabilities([]organ.Func{speak, readAnimal}),
		WithMaxTurns(5),
		WithTurnTimeout(5*time.Second),
	)

	cb.Think(context.Background(), reactivity.Catalyst{Need: "Tell me about cows"})
	m := cb.Result()

	chain := m.Chain()
	events := chain.All()

	if len(events) < 2 {
		t.Fatalf("expected at least 2 chain events (read + speak), got %d", len(events))
	}

	if events[0].Organ != "read_animal" {
		t.Errorf("first event should be read_animal (Sense), got %s", events[0].Organ)
	}
	if events[0].Kind != reactivity.Sense {
		t.Errorf("read_animal should be Sense, got %v", events[0].Kind)
	}

	if events[1].Organ != "dialog_speak" {
		t.Errorf("second event should be dialog_speak (Sense — ReadAction), got %s", events[1].Organ)
	}

	t.Logf("conversation flow: %s(%s) → %s(%s)",
		events[0].Organ, events[0].Kind,
		events[1].Organ, events[1].Kind)
}
