package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

func TestTelos_SimpleConversation_ParksAfterSpeak(t *testing.T) {
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

	if m.Sealed() {
		t.Error("molecule should NOT seal after speak — it should park for more input")
	}

	chain := m.Chain()
	if chain.Len() == 0 {
		t.Error("EventChain should have the speak event")
	}

	t.Logf("turns=%d sealed=%v chain_events=%d", m.Turns(), m.Sealed(), chain.Len())
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

func TestTelos_MultiTurnDialog_SameMolecule(t *testing.T) {
	speak := speakCap()
	turnNum := 0
	completer := tangle.CompleteFunc(func(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
		turnNum++
		switch {
		case turnNum <= 2:
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: fmt.Sprintf("s%d", turnNum), Name: "dialog_speak",
						Input: json.RawMessage(fmt.Sprintf(`{"response":"Answer about turn %d"}`, turnNum))},
				},
			}, nil
		default:
			return &tangle.Completion{Content: "waiting"}, nil
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

	// Turn 1: operator asks about cows
	cb.Think(context.Background(), reactivity.Catalyst{Need: "Tell me about cows"})
	mol1 := cb.Result()
	mol1ID := mol1.ID

	// Turn 2: operator asks follow-up — SAME molecule should resume
	cb.Think(context.Background(), reactivity.Catalyst{Need: "What about their diet?"})
	mol2 := cb.Result()

	if mol2.ID != mol1ID {
		t.Errorf("second Think should resume same Molecule, got different ID: %q vs %q", mol1ID, mol2.ID)
	}

	chain := mol2.Chain()
	if chain.Len() < 2 {
		t.Errorf("EventChain should have events from BOTH turns, got %d", chain.Len())
	}

	intentAtoms := mol2.Mass(reactivity.IntentAtom)
	if intentAtoms < 2 {
		t.Errorf("should have 2+ intent atoms (one per operator input), got %d", intentAtoms)
	}

	t.Logf("multi-turn: molecule=%s turns=%d chain_events=%d intent_atoms=%d",
		mol2.ID, mol2.Turns(), chain.Len(), intentAtoms)
}

func TestTelos_MultiTurnDialog_CowsThenChickens_SameMolecule(t *testing.T) {
	speak := speakCap()
	readAnimal := organ.Func{
		Name:   "read_animal",
		Schema: json.RawMessage(`{"type":"object","properties":{"animal":{"type":"string"}},"required":["animal"]}`),
		Mode:   organ.ReadAction,
		Source: organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			var args struct{ Animal string `json:"animal"` }
			json.Unmarshal(input, &args)
			facts := map[string]string{
				"cow":     "Cows produce milk.",
				"chicken": "Chickens lay eggs.",
			}
			return organ.TextResult(facts[args.Animal]), nil
		},
	}

	callCount := 0
	completer := tangle.CompleteFunc(func(_ context.Context, params tangle.CompletionParams) (*tangle.Completion, error) {
		callCount++
		lastMsg := ""
		for _, m := range params.Messages {
			if m.Role == "user" {
				lastMsg = m.Content
			}
		}

		if strings.Contains(lastMsg, "cow") && callCount <= 2 {
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: fmt.Sprintf("r%d", callCount), Name: "read_animal", Input: json.RawMessage(`{"animal":"cow"}`)},
				},
			}, nil
		}
		if strings.Contains(lastMsg, "chicken") {
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: fmt.Sprintf("r%d", callCount), Name: "read_animal", Input: json.RawMessage(`{"animal":"chicken"}`)},
				},
			}, nil
		}
		return &tangle.Completion{
			ToolCalls: []tangle.ToolCall{
				{ID: fmt.Sprintf("s%d", callCount), Name: "dialog_speak",
					Input: json.RawMessage(`{"response":"Here's what I found."}`)},
			},
		}, nil
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
		WithMaxTurns(3),
		WithTurnTimeout(5*time.Second),
	)

	// Turn 1: cows
	cb.Think(context.Background(), reactivity.Catalyst{Need: "Tell me about cows"})

	// Turn 2: chickens — same molecule
	cb.Think(context.Background(), reactivity.Catalyst{Need: "Now tell me about chickens"})

	m := cb.Result()
	chain := m.Chain()

	cowEvents := 0
	chickenEvents := 0
	for _, e := range chain.All() {
		if e.Organ == "read_animal" {
			if strings.Contains(string(e.Input), "cow") {
				cowEvents++
			}
			if strings.Contains(string(e.Input), "chicken") {
				chickenEvents++
			}
		}
	}

	if cowEvents == 0 {
		t.Error("expected cow read_animal events in chain")
	}
	if chickenEvents == 0 {
		t.Error("expected chicken read_animal events in chain")
	}

	t.Logf("multi-topic: molecule=%s chain=%d cow_reads=%d chicken_reads=%d intent_atoms=%d",
		m.ID, chain.Len(), cowEvents, chickenEvents, m.Mass(reactivity.IntentAtom))
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
