package cerebrum

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/service/andon"
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
	reactor *reactivity.Core
	router  CompleterRouter
	budget  Budget

	sensory Bus
	motor   Bus
	signal  Bus
	synapse Synapse

	classifier    Classifier
	promptBuilder PromptBuilder
	parser        ResponseParser
	toolDefs      []tangle.Tool

	pool        ergograph.Ledger
	andon       andon.Signal
	assert      reactivity.Assert
	recollector Recollector
	compactor   Compactor

	molecule *reactivity.Molecule
}

func New(reactor *reactivity.Core, completer tangle.Completer, opts ...Option) *Cerebrum {
	cb := &Cerebrum{
		reactor:       reactor,
		router:        SingleRouter(completer),
		budget:        DefaultBudget,
		classifier:    DefaultClassifier,
		promptBuilder: DefaultPromptBuilder,
		parser:        DefaultParser,
		sensory:       NewChannelBus(64),
		synapse:       DefaultSynapse{},
		assert:        reactivity.DefaultAssert,
	}
	for _, opt := range opts {
		opt(cb)
	}
	return cb
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

func (cb *Cerebrum) Think(ctx context.Context, catalyst reactivity.Catalyst) error {
	need := []byte(catalyst.Need)
	var molecule *reactivity.Molecule
	if len(catalyst.Criteria) > 0 {
		molecule = reactivity.NewMoleculeWithCatalyst(
			fmt.Sprintf("mol-%d", time.Now().UnixNano()), catalyst)
		cb.reactor.Add(molecule, reactivity.Atom{
			ID:   fmt.Sprintf("need-%d", time.Now().UnixNano()),
			Type: reactivity.IntentAtom, Taxonomy: "intent.need",
			Content: need, CreatedAt: time.Now(),
		})
	} else {
		molecule = cb.reactor.Cognize(reactivity.Atom{
			ID:   fmt.Sprintf("need-%d", time.Now().UnixNano()),
			Type: reactivity.IntentAtom, Taxonomy: "intent.need",
			Content: need, CreatedAt: time.Now(),
		})
	}

	cb.molecule = molecule

	if cb.recollector != nil {
		recollected := cb.recollector.Recollect(need)
		for _, atom := range recollected {
			cb.reactor.Add(molecule, atom)
		}
		if len(recollected) > 0 {
			slog.InfoContext(ctx, "cerebrum.think.recollect",
				slog.String("molecule", molecule.ID),
				slog.Int("atoms", len(recollected)),
				slog.String("phase", molecule.Phase().String()))
			cb.emit("cerebrum.recollect", map[string]string{
				"molecule": molecule.ID,
				"atoms":    fmt.Sprintf("%d", len(recollected)),
			})
		}
	}

	slog.InfoContext(ctx, "cerebrum.think.start",
		slog.String("molecule", molecule.ID),
		slog.String("phase", molecule.Phase().String()),
		slog.Int("max_turns", cb.budget.MaxTurns),
		slog.Duration("turn_timeout", cb.budget.TurnTimeout))

	history, _ := molecule.Context().([]tangle.Message)

	prevTriad := molecule.CurrentTriad()
	for turn := 0; turn < cb.budget.MaxTurns && !molecule.Sealed(); turn++ {
		molecule.Tick()

		if cb.compactor != nil {
			currentTriad := molecule.CurrentTriad()
			if currentTriad != prevTriad {
				history = cb.compactor.Compact(history, prevTriad)
				slog.InfoContext(ctx, "cerebrum.think.compact",
					slog.String("from", prevTriad.String()),
					slog.String("to", currentTriad.String()),
					slog.Int("history_len", len(history)))
				prevTriad = currentTriad
			}
		}

		domain := cb.classifier.Classify(molecule)
		if domain == Clear && molecule.CurrentTriad() == reactivity.ThinkTriad {
			molecule.SetPhase(reactivity.ExecutionAtom)
			slog.InfoContext(ctx, "cerebrum.think.cynefin_skip",
				slog.Int("turn", turn),
				slog.String("domain", domain.String()))
			cb.emit("cerebrum.cynefin_skip", map[string]string{
				"molecule": molecule.ID,
				"turn":     fmt.Sprintf("%d", turn),
				"domain":   domain.String(),
			})
		}
		contracts := cb.reactor.Contracts()
		directives := cb.reactor.Directives(molecule.Phase())
		prompt := buildContractPrompt(molecule, need, domain, contracts)
		for _, d := range directives {
			prompt += "\n> " + string(d)
		}

		slog.InfoContext(ctx, "cerebrum.think.turn",
			slog.Int("turn", turn),
			slog.String("phase", molecule.Phase().String()),
			slog.Int("mass", molecule.TotalMass()),
			slog.Int("history_len", len(history)),
			slog.String("domain", domain.String()),
			slog.Float64("momentum", molecule.Momentum()),
			slog.Float64("distance", molecule.Distance()))
		slog.DebugContext(ctx, "cerebrum.think.prompt",
			slog.Int("turn", turn),
			slog.String("content", prompt))

		messages := make([]tangle.Message, 0, len(history)+1)
		messages = append(messages, history...)
		messages = append(messages, tangle.Message{Role: "user", Content: prompt})

		completer := cb.router.Route(molecule.Phase())

		turnCtx, turnCancel := context.WithTimeout(ctx, cb.budget.TurnTimeout)
		start := time.Now()
		completion, err := completer.Complete(turnCtx, tangle.CompletionParams{
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

		cb.emit("cerebrum.turn", map[string]string{
			"molecule":   molecule.ID,
			"turn":       fmt.Sprintf("%d", turn),
			"phase":      molecule.Phase().String(),
			"tool_calls": fmt.Sprintf("%d", len(completion.ToolCalls)),
			"tokens_in":  fmt.Sprintf("%d", completion.Tokens.Input),
			"tokens_out": fmt.Sprintf("%d", completion.Tokens.Output),
			"elapsed_ms": fmt.Sprintf("%d", elapsed.Milliseconds()),
			"momentum":   fmt.Sprintf("%.3f", molecule.Momentum()),
			"distance":   fmt.Sprintf("%.3f", molecule.Distance()),
		})
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
			cb.emit("cerebrum.tool_call", map[string]string{
				"molecule": molecule.ID,
				"turn":     fmt.Sprintf("%d", turn),
				"tool":     tc.Name,
			})
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
				cb.emit("cerebrum.tool_result", map[string]string{
					"molecule":   molecule.ID,
					"turn":       fmt.Sprintf("%d", turn),
					"tool":       tc.Name,
					"result_len": fmt.Sprintf("%d", len(result)),
				})
				if molecule.Catalyst() != nil {
					cb.checkCatalystCriteria(molecule, tc.Name, result)
				}
			}
			toolCancel()
			if molecule.Sealed() {
				slog.InfoContext(ctx, "cerebrum.think.catalyst_sealed",
					slog.Int("turn", turn),
					slog.Float64("distance", molecule.Distance()))
				break
			}
			molecule.SetContext(history)
			continue
		}

		atoms, _ := cb.parser.Parse(completion.Content, molecule.Phase(), turn)

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

		criticality := cb.assert.Evaluate(molecule)
		if criticality == reactivity.Subcritical {
			slog.WarnContext(ctx, "cerebrum.think.subcritical",
				slog.Int("turn", turn),
				slog.Float64("momentum", molecule.Momentum()),
				slog.Float64("distance", molecule.Distance()))
			cb.pull(molecule.ID)
			cb.emit("cerebrum.subcritical", map[string]string{
				"molecule": molecule.ID,
				"turn":     fmt.Sprintf("%d", turn),
				"momentum": fmt.Sprintf("%.3f", molecule.Momentum()),
				"distance": fmt.Sprintf("%.3f", molecule.Distance()),
			})
			cb.reactor.Seal(molecule, reactivity.Atom{
				ID:        fmt.Sprintf("wish-subcritical-%d", turn),
				Type:      reactivity.RetrospectionAtom,
				Taxonomy:  "retrospection.wish.subcritical",
				Content:   []byte(fmt.Sprintf("subcritical: momentum=%.3f distance=%.3f", molecule.Momentum(), molecule.Distance())),
				CreatedAt: time.Now(),
			})
			break
		}
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

	cb.emit("cerebrum.sealed", map[string]string{
		"molecule": molecule.ID,
		"mass":     fmt.Sprintf("%d", molecule.TotalMass()),
		"phase":    molecule.Phase().String(),
		"sealed":   fmt.Sprintf("%v", molecule.Sealed()),
	})

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

func (cb *Cerebrum) emit(action string, labels map[string]string) {
	if cb.pool == nil {
		return
	}
	cb.pool.Append(ergograph.Record{
		Action:    action,
		Timestamp: time.Now(),
		Labels:    labels,
	})
}

func (cb *Cerebrum) pull(agentID string) {
	if cb.andon == nil {
		return
	}
	cb.andon.Pull(agentID)
}

func (cb *Cerebrum) checkCatalystCriteria(m *reactivity.Molecule, toolName, result string) {
	cat := m.Catalyst()
	if cat == nil {
		return
	}
	for key, expected := range cat.Criteria {
		switch v := expected.(type) {
		case bool:
			if !v && strings.Contains(strings.ToLower(result), "not "+key) {
				m.ReportSensor(key, false)
			} else if !v && strings.Contains(strings.ToLower(result), "no longer "+key) {
				m.ReportSensor(key, false)
			}
		case string:
			if strings.Contains(result, v) {
				m.ReportSensor(key, v)
			}
		}
	}
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
