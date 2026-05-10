package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

// --- Stress 1: Rapid multi-turn — 10 inputs on same Molecule ---

func TestStress_RapidSequential_10Sessions(t *testing.T) {
	speak := dialogOrgan()
	var llmCalls atomic.Int32
	completer := tangle.CompleteFunc(func(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
		n := llmCalls.Add(1)
		return &tangle.Completion{
			ToolCalls: []tangle.ToolCall{
				{ID: fmt.Sprintf("s%d", n), Name: "dialog",
					Input: json.RawMessage(fmt.Sprintf(`{"response":"Answer %d"}`, n))},
			},
		}, nil
	})

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog": speak},
		sensory: NewChannelBus(64),
	}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithOrgans([]organ.Func{speak}),
		WithMaxTurns(3),
		WithTurnTimeout(5*time.Second),
	)

	for i := 0; i < 10; i++ {
		_, _ = cb.Think(context.Background(), reactivity.Catalyst{
			Need: fmt.Sprintf("Question %d about animals", i),
		})
	}

	m := cb.Result()
	if !m.Sealed() {
		t.Error("last molecule should be sealed (ImmediateSeal after text)")
	}
	if llmCalls.Load() < 10 {
		t.Errorf("expected 10+ LLM calls across sessions, got %d", llmCalls.Load())
	}
	t.Logf("10 sessions: llm_calls=%d last_molecule_mass=%d",
		llmCalls.Load(), m.TotalMass())
}

// --- Stress 2: Desired emerges mid-conversation ---

func TestStress_DesiredEmergesMidConversation(t *testing.T) {
	speak := dialogOrgan()
	readAnimal := organ.Func{
		Name:   "read_animal",
		Schema: json.RawMessage(`{"type":"object","properties":{"animal":{"type":"string"}}}`),
		Mode:   organ.ReadAction,
		Source: organ.Environment,
		Execute: func(_ context.Context, input json.RawMessage) (organ.Result, error) {
			return organ.TextResult("Animal facts here"), nil
		},
	}

	callCount := 0
	completer := tangle.CompleteFunc(func(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
		callCount++
		if callCount <= 2 {
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: fmt.Sprintf("s%d", callCount), Name: "dialog",
						Input: json.RawMessage(`{"response":"What would you like to know?"}`)},
				},
			}, nil
		}
		return &tangle.Completion{
			ToolCalls: []tangle.ToolCall{
				{ID: fmt.Sprintf("r%d", callCount), Name: "read_animal",
					Input: json.RawMessage(`{"animal":"cow"}`)},
			},
		}, nil
	})

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog": speak, "read_animal": readAnimal},
		sensory: NewChannelBus(64),
	}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithOrgans([]organ.Func{speak, readAnimal}),
		WithMaxTurns(5),
		WithTurnTimeout(5*time.Second),
	)

	// Session 1: open-ended, seals via ImmediateSeal
	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: "Hello"})
	m1 := cb.Result()
	if !m1.Sealed() {
		t.Fatal("should seal after Hello (ImmediateSeal: no Desired, text only)")
	}
	if m1.Catalyst() != nil && len(m1.Catalyst().Desired) > 0 {
		t.Fatal("should have no Desired after Hello")
	}

	// Session 2: Desired emerges on new Molecule
	_, _ = cb.Think(context.Background(), reactivity.Catalyst{
		Need:    "Tell me specifically about cow milk production",
		Desired: map[string]any{"milk_info": true},
	})
	m2 := cb.Result()

	if m2.Catalyst() == nil || len(m2.Catalyst().Desired) == 0 {
		t.Fatal("Desired should exist on second molecule")
	}
	if m2.ID == m1.ID {
		t.Fatal("second Think should create a new Molecule (first was sealed)")
	}

	t.Logf("desired emerged: mol1=%s mol2=%s has_desired=%v distance=%.2f",
		m1.ID, m2.ID, m2.Catalyst() != nil, m2.Distance())
}

// --- Stress 3: Organ failure mid-session ---

func TestStress_OrganFailureMidSession(t *testing.T) {
	speak := dialogOrgan()
	failCount := 0
	failOrgan := organ.Func{
		Name:   "flaky_read",
		Schema: json.RawMessage(`{"type":"object","properties":{}}`),
		Mode:   organ.ReadAction,
		Source: organ.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
			failCount++
			if failCount == 1 {
				return organ.ErrorResult("connection refused"), nil
			}
			return organ.TextResult("success on retry"), nil
		},
	}

	callCount := 0
	completer := tangle.CompleteFunc(func(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
		callCount++
		switch callCount {
		case 1:
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: "f1", Name: "flaky_read", Input: json.RawMessage(`{}`)},
				},
			}, nil
		case 2:
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: "f2", Name: "flaky_read", Input: json.RawMessage(`{}`)},
				},
			}, nil
		default:
			return &tangle.Completion{
				ToolCalls: []tangle.ToolCall{
					{ID: "s1", Name: "dialog",
						Input: json.RawMessage(`{"response":"Got it on second try"}`)},
				},
			}, nil
		}
	})

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog": speak, "flaky_read": failOrgan},
		sensory: NewChannelBus(64),
	}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithOrgans([]organ.Func{speak, failOrgan}),
		WithMaxTurns(5),
		WithTurnTimeout(5*time.Second),
	)

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{
		Need:    "Read the data",
		Desired: map[string]any{"data_read": true},
	})
	m := cb.Result()

	chain := m.Chain()
	errorEvents := 0
	successEvents := 0
	for _, e := range chain.All() {
		if e.Organ == "flaky_read" {
			if strings.Contains(string(e.Output), "refused") {
				errorEvents++
			} else {
				successEvents++
			}
		}
	}

	if errorEvents == 0 {
		t.Error("should have captured the failure in EventChain")
	}
	if successEvents == 0 {
		t.Error("should have captured the retry success in EventChain")
	}
	t.Logf("flaky organ: errors=%d successes=%d total_chain=%d",
		errorEvents, successEvents, chain.Len())
}

// --- Stress 4: Debouncer fires in multi-turn ---

func TestStress_DebouncerAcrossMultiTurn(t *testing.T) {
	speak := dialogOrgan()
	var readCalls atomic.Int32
	repeatReader := organ.Func{
		Name:   "repeat_read",
		Schema: json.RawMessage(`{"type":"object","properties":{}}`),
		Mode:   organ.ReadAction,
		Source: organ.Environment,
		Execute: func(_ context.Context, _ json.RawMessage) (organ.Result, error) {
			readCalls.Add(1)
			return organ.TextResult("same data"), nil
		},
	}

	callCount := 0
	completer := tangle.CompleteFunc(func(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
		callCount++
		return &tangle.Completion{
			ToolCalls: []tangle.ToolCall{
				{ID: fmt.Sprintf("r%d", callCount), Name: "repeat_read",
					Input: json.RawMessage(`{}`)},
			},
		}, nil
	})

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog": speak, "repeat_read": repeatReader},
		sensory: NewChannelBus(64),
	}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithOrgans([]organ.Func{speak, repeatReader}),
		WithMaxTurns(5),
		WithTurnTimeout(5*time.Second),
	)

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{
		Need:    "Read the data",
		Desired: map[string]any{"data_read": true},
	})

	calls := readCalls.Load()
	if calls >= 5 {
		t.Errorf("debouncer should block repeated identical calls, but organ ran %d times", calls)
	}
	t.Logf("debouncer: organ_calls=%d (max_turns=5)", calls)
}

// --- Stress 5: Empty input ---

func TestStress_EmptyInput(t *testing.T) {
	speak := dialogOrgan()
	completer := &stubCompleter{
		toolCalls: []tangle.ToolCall{
			{ID: "s1", Name: "dialog",
				Input: json.RawMessage(`{"response":"I received empty input."}`)},
		},
		toolCallOnce: true,
	}

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog": speak},
		sensory: NewChannelBus(64),
	}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithOrgans([]organ.Func{speak}),
		WithMaxTurns(3),
		WithTurnTimeout(5*time.Second),
	)

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{Need: ""})
	m := cb.Result()

	if m == nil {
		t.Fatal("molecule should exist even for empty input")
	}
	t.Logf("empty input: sealed=%v turns=%d", m.Sealed(), m.Turns())
}

// --- Stress 6: Concurrent Think calls ---
// Known issue: Cerebrum is not thread-safe. cb.molecule races.
// This test documents the gap — skip under -race until fixed.

func TestStress_ConcurrentThinkPanics(t *testing.T) {
	if raceEnabled {
		t.Skip("known race: cb.molecule shared state — needs mutex or per-session isolation")
	}
	speak := dialogOrgan()
	completer := tangle.CompleteFunc(func(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
		time.Sleep(10 * time.Millisecond)
		return &tangle.Completion{
			ToolCalls: []tangle.ToolCall{
				{ID: "s1", Name: "dialog",
					Input: json.RawMessage(`{"response":"ok"}`)},
			},
		}, nil
	})

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog": speak},
		sensory: NewChannelBus(64),
	}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithOrgans([]organ.Func{speak}),
		WithMaxTurns(2),
		WithTurnTimeout(2*time.Second),
	)

	done := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		go func(n int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("concurrent Think panicked: %v", r)
				}
				done <- true
			}()
			_, _ = cb.Think(context.Background(), reactivity.Catalyst{
				Need: fmt.Sprintf("concurrent input %d", n),
			})
		}(i)
	}

	<-done
	<-done
	t.Log("concurrent Think completed without panic")
}

// --- Stress 7: Max turns with Desired never met ---

func TestStress_MaxTurns_DesiredNeverMet(t *testing.T) {
	speak := dialogOrgan()
	completer := tangle.CompleteFunc(func(_ context.Context, _ tangle.CompletionParams) (*tangle.Completion, error) {
		return &tangle.Completion{
			ToolCalls: []tangle.ToolCall{
				{ID: "s1", Name: "dialog",
					Input: json.RawMessage(`{"response":"still working on it"}`)},
			},
		}, nil
	})

	motor := &autoExecMotor{
		caps:    map[string]organ.Func{"dialog": speak},
		sensory: NewChannelBus(64),
	}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithSensory(motor.sensory),
		WithMotor(motor),
		WithOrgans([]organ.Func{speak}),
		WithMaxTurns(3),
		WithTurnTimeout(5*time.Second),
	)

	_, _ = cb.Think(context.Background(), reactivity.Catalyst{
		Need:    "Solve world hunger",
		Desired: map[string]any{"hunger_solved": true},
	})
	m := cb.Result()

	if !m.Sealed() {
		t.Error("should seal after max turns even if Desired not met")
	}
	if m.Distance() <= 0 {
		t.Error("distance should be > 0 since Desired was never met")
	}
	t.Logf("max turns: sealed=%v distance=%.2f turns=%d",
		m.Sealed(), m.Distance(), m.Turns())
}
