package cerebrum

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/artifact"
	tangle "github.com/dpopsuev/tangle"
)

type Budget struct {
	MaxTurns    int
	TurnTimeout time.Duration
	MaxTokens   int
	MinOAE      float64
}

var DefaultBudget = Budget{
	MaxTurns:    100,
	TurnTimeout: 30 * time.Second,
	MaxTokens:   0,
}

type Cerebrum struct {
	reactor   *reactivity.Core
	completer tangle.Completer
	budget    Budget

	sensory Bus
	motor   Bus
	signal  Bus
	synapse Synapse

	classifier    Classifier
	promptBuilder PromptBuilder
	parser        ResponseParser
	toolDefs      []tangle.Tool

	molecule *reactivity.Molecule
}

func New(reactor *reactivity.Core, completer tangle.Completer, opts ...Option) *Cerebrum {
	cb := &Cerebrum{
		reactor:       reactor,
		completer:     completer,
		budget:        DefaultBudget,
		classifier:    DefaultClassifier,
		promptBuilder: DefaultPromptBuilder,
		parser:        DefaultParser,
		sensory:       NewChannelBus(64),
		synapse:       DefaultSynapse{},
	}
	for _, opt := range opts {
		opt(cb)
	}
	return cb
}

var _ organ.Organ = (*Cerebrum)(nil)

func (cb *Cerebrum) Name() organ.OrganName { return organ.CerebrumOrgan }

func (cb *Cerebrum) Receive(wire artifact.Wire) error {
	return cb.sensory.Send(context.Background(), Event{
		ID:        fmt.Sprintf("wire-%d", time.Now().UnixNano()),
		Kind:      wire.Kind,
		Source:    "wire",
		Payload:   wire.Payload,
		CreatedAt: time.Now(),
	})
}

func (cb *Cerebrum) Run(ctx context.Context) {
	atoms := make(chan reactivity.Atom, 64)
	emits := make(chan reactivity.Emission, 64)
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		defer close(atoms)
		for {
			event, ok := cb.sensory.Receive(ctx)
			if !ok {
				return
			}
			atom, err := cb.synapse.Encode(event)
			if err != nil {
				continue
			}
			select {
			case atoms <- atom:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer close(emits)
		for atom := range atoms {
			molecule := cb.reactor.Cognize(atom)
			for _, e := range molecule.DrainEmissions() {
				select {
				case emits <- e:
				case <-ctx.Done():
					return
				}
			}
			if molecule.Sealed() {
				cb.molecule = molecule
				cb.reactor.Monolog().Park()
			}
		}
	}()

	go func() {
		defer wg.Done()
		for e := range emits {
			event := cb.synapse.Decode(e)
			if cb.motor != nil {
				cb.motor.Send(ctx, event)
			}
		}
	}()

	wg.Wait()
}

func (cb *Cerebrum) Think(ctx context.Context, need []byte) error {
	molecule := cb.reactor.Cognize(reactivity.Atom{
		ID:        fmt.Sprintf("need-%d", time.Now().UnixNano()),
		Type:      reactivity.IntentAtom,
		Taxonomy:  "intent.need",
		Content:   need,
		CreatedAt: time.Now(),
	})

	slog.InfoContext(ctx, "cerebrum.think.start",
		slog.String("molecule", molecule.ID),
		slog.String("phase", molecule.Phase().String()),
		slog.Int("max_turns", cb.budget.MaxTurns),
		slog.Duration("turn_timeout", cb.budget.TurnTimeout))

	history, _ := molecule.Context().([]tangle.Message)

	for turn := 0; turn < cb.budget.MaxTurns && !molecule.Sealed(); turn++ {
		domain := cb.classifier.Classify(molecule)
		directives := cb.reactor.Directives(molecule.Phase())
		prompt := cb.promptBuilder.Build(molecule, need, domain)
		for _, d := range directives {
			prompt += "\n> " + string(d)
		}

		slog.InfoContext(ctx, "cerebrum.think.turn",
			slog.Int("turn", turn),
			slog.String("phase", molecule.Phase().String()),
			slog.Int("mass", molecule.TotalMass()),
			slog.Int("history_len", len(history)),
			slog.String("domain", domain.String()))
		slog.DebugContext(ctx, "cerebrum.think.prompt",
			slog.Int("turn", turn),
			slog.String("content", prompt))

		messages := make([]tangle.Message, 0, len(history)+1)
		messages = append(messages, history...)
		messages = append(messages, tangle.Message{Role: "user", Content: prompt})

		turnCtx, turnCancel := context.WithTimeout(ctx, cb.budget.TurnTimeout)
		start := time.Now()
		completion, err := cb.completer.Complete(turnCtx, tangle.CompletionParams{
			Messages:  messages,
			Tools:     cb.tools(molecule.Phase()),
			MaxTokens: cb.budget.MaxTokens,
		})
		elapsed := time.Since(start)
		turnCancel()

		if err != nil {
			slog.WarnContext(ctx, "cerebrum.think.completer_error",
				slog.Int("turn", turn),
				slog.Duration("elapsed", elapsed),
				slog.Any("error", err))
			cb.reactor.Seal(molecule, reactivity.Atom{
				ID:        fmt.Sprintf("wish-error-%d", turn),
				Type:      reactivity.RetrospectionAtom,
				Taxonomy:  "retrospection.wish.completer-error",
				Content:   []byte(err.Error()),
				CreatedAt: time.Now(),
			})
			break
		}

		slog.InfoContext(ctx, "cerebrum.think.response",
			slog.Int("turn", turn),
			slog.Duration("elapsed", elapsed),
			slog.Int("response_len", len(completion.Content)),
			slog.Int("tool_calls", len(completion.ToolCalls)),
			slog.Int("tokens_in", completion.Tokens.Input),
			slog.Int("tokens_out", completion.Tokens.Output))
		slog.DebugContext(ctx, "cerebrum.think.response_content",
			slog.Int("turn", turn),
			slog.String("content", completion.Content))

		history = append(history, tangle.Message{Role: "user", Content: prompt})
		history = append(history, tangle.Message{
			Role:      "assistant",
			Content:   completion.Content,
			ToolCalls: completion.ToolCalls,
		})

		for _, tc := range completion.ToolCalls {
			slog.InfoContext(ctx, "cerebrum.think.tool_call",
				slog.Int("turn", turn),
				slog.String("name", tc.Name),
				slog.Int("input_len", len(tc.Input)))
			molecule.Emit(reactivity.Emission{
				Kind:    "instrument",
				Target:  tc.Name,
				Payload: tc.Input,
			})
		}

		if len(completion.ToolCalls) > 0 {
			cb.dispatch(ctx, molecule)

			toolCtx, toolCancel := context.WithTimeout(ctx, cb.budget.TurnTimeout)
			for _, tc := range completion.ToolCalls {
				result := cb.waitToolResult(toolCtx, tc)
				history = append(history, tangle.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
				})
				slog.InfoContext(ctx, "cerebrum.think.tool_result",
					slog.Int("turn", turn),
					slog.String("name", tc.Name),
					slog.Int("result_len", len(result)))
			}
			toolCancel()
			molecule.SetContext(history)
			continue
		}

		atoms, _, _ := cb.parser.Parse(completion.Content, molecule.Phase(), turn)

		slog.InfoContext(ctx, "cerebrum.think.parsed",
			slog.Int("turn", turn),
			slog.Int("atoms", len(atoms)))

		for _, atom := range atoms {
			result, fortune := cb.reactor.Add(molecule, atom)

			slog.InfoContext(ctx, "cerebrum.think.react",
				slog.Int("turn", turn),
				slog.String("atom_type", atom.Type.String()),
				slog.String("taxonomy", atom.Taxonomy),
				slog.String("result", result.String()),
				slog.String("phase", molecule.Phase().String()))

			if result == reactivity.Unresolvable {
				slog.WarnContext(ctx, "cerebrum.think.unresolvable",
					slog.Int("turn", turn),
					slog.String("message", fortune.Message))
				cb.reactor.Seal(molecule, reactivity.Atom{
					ID:        fmt.Sprintf("wish-unresolvable-%d", turn),
					Type:      reactivity.RetrospectionAtom,
					Taxonomy:  "retrospection.wish.unresolvable",
					Content:   []byte(fortune.Message),
					CreatedAt: time.Now(),
				})
				break
			}
			cb.dispatch(ctx, molecule)
		}
		molecule.SetContext(history)
	}

	if !molecule.Sealed() {
		slog.WarnContext(ctx, "cerebrum.think.max_turns",
			slog.Int("max_turns", cb.budget.MaxTurns),
			slog.Int("mass", molecule.TotalMass()))
		cb.reactor.Seal(molecule, reactivity.Atom{
			ID:        "wish-max-turns",
			Type:      reactivity.RetrospectionAtom,
			Taxonomy:  "retrospection.wish.max-turns-exceeded",
			Content:   []byte("exceeded max turns"),
			CreatedAt: time.Now(),
		})
	}

	slog.InfoContext(ctx, "cerebrum.think.done",
		slog.String("molecule", molecule.ID),
		slog.Bool("sealed", molecule.Sealed()),
		slog.Int("mass", molecule.TotalMass()))

	cb.molecule = molecule
	cb.reactor.Monolog().Park()
	return nil
}

func (cb *Cerebrum) Result() *reactivity.Molecule {
	return cb.molecule
}

func (cb *Cerebrum) Store() *reactivity.MoleculeStore {
	return cb.reactor.Monolog()
}

func (cb *Cerebrum) SensoryBus() Bus {
	return cb.sensory
}

func (cb *Cerebrum) tools(phase reactivity.AtomType) []tangle.Tool {
	return cb.toolDefs
}

func (cb *Cerebrum) waitToolResult(ctx context.Context, tc tangle.ToolCall) string {
	event, ok := cb.sensory.Receive(ctx)
	if !ok {
		return "tool call timed out"
	}
	return string(event.Payload)
}

func (cb *Cerebrum) dispatch(ctx context.Context, m *reactivity.Molecule) {
	for _, e := range m.DrainEmissions() {
		event := cb.synapse.Decode(e)
		if cb.motor != nil {
			cb.motor.Send(ctx, event)
		}
	}
}
