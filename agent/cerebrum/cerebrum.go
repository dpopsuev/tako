package cerebrum

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/shell"
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

type Record struct {
	Action    string
	Timestamp time.Time
	Labels    map[string]string
}

type Recorder interface {
	Append(record Record) error
}

type Halter interface {
	Pull(agentID string)
}

type Priority int

const (
	PriorityIgnore    Priority = iota
	PriorityPark
	PriorityInterrupt
	PriorityEmergency
)

func (p Priority) String() string {
	return [...]string{"ignore", "park", "interrupt", "emergency"}[p]
}

type PriorityClassifier interface {
	Classify(event Event, molecule *reactivity.Molecule) Priority
}

type defaultClassifierImpl struct{}

func (defaultClassifierImpl) Classify(event Event, _ *reactivity.Molecule) Priority {
	switch event.Kind {
	case "sensory.alarm", "sensory.emergency":
		return PriorityEmergency
	case "sensory.timer", "sensory.warning":
		return PriorityInterrupt
	default:
		return PriorityPark
	}
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

	recorder    Recorder
	halter      Halter
	assert      reactivity.Assert
	recollector Recollector
	compactor   Compactor

	observer           Observer
	regulator          Regulator
	assembler          Assembler
	capabilities       []shell.Capability
	config             *reactivity.Config
	priorityClassifier PriorityClassifier
	watcher            tangle.Completer
	globalStore        chan Event
	monitorEvents      chan Event

	molecule *reactivity.Molecule
}

func New(reactor *reactivity.Core, completer tangle.Completer, opts ...Option) *Cerebrum {
	cfg := &reactivity.DefaultConfig
	cb := &Cerebrum{
		reactor:            reactor,
		router:             SingleRouter(completer),
		budget:             DefaultBudget,
		promptBuilder:      DefaultPromptBuilder,
		parser:             DefaultParser,
		sensory:            NewChannelBus(64),
		synapse:            DefaultSynapse{},
		assert:             reactivity.DefaultAssert,
		config:             cfg,
		priorityClassifier: defaultClassifierImpl{},
		globalStore:        make(chan Event, 128),
		monitorEvents:      make(chan Event, 64),
	}
	cb.classifier = ClassifierFunc(func(m *reactivity.Molecule) Domain {
		return ClassifyWithConfig(m, cb.config)
	})
	for _, opt := range opts {
		opt(cb)
	}
	cb.config.Validate()
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
	if len(catalyst.Desired) > 0 {
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
		slog.Duration("turn_timeout", cb.budget.TurnTimeout),
		slog.Float64("cfg.distance_close", cb.config.DistanceClose),
		slog.Float64("cfg.distance_mid", cb.config.DistanceMid),
		slog.Float64("cfg.recollection_min", cb.config.RecollectionMin),
		slog.Int("cfg.unmet_dim_max", cb.config.UnmetDimMax),
		slog.Int("cfg.backward_turn_limit", cb.config.BackwardTurnLimit))

	history, _ := molecule.Context().([]tangle.Message)
	var turnRecords []TurnRecord

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
		prompt := cb.assemble(molecule, need, domain, turn)

		slog.InfoContext(ctx, "cerebrum.think.turn",
			slog.Int("turn", turn),
			slog.String("phase", molecule.Phase().String()),
			slog.Int("mass", molecule.TotalMass()),
			slog.Int("history_len", len(history)),
			slog.String("domain", domain.String()),
			slog.Float64("momentum", molecule.Momentum()),
			slog.Float64("distance", molecule.Distance()),
			slog.Float64("delta", molecule.DeltaDistance()),
			slog.Any("residual", molecule.Residual()))
		slog.DebugContext(ctx, "cerebrum.think.prompt",
			slog.Int("turn", turn),
			slog.String("content", prompt))

		messages := make([]tangle.Message, 0, len(history)+1)
		messages = append(messages, history...)
		messages = append(messages, tangle.Message{Role: "user", Content: prompt})

		completer := cb.router.Route(molecule)

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

		unmetCount := 0
		if residual := molecule.Residual(); residual != nil {
			for _, v := range residual {
				if v > 0 {
					unmetCount++
				}
			}
		}
		tr := TurnRecord{
			MoleculeID:   molecule.ID,
			Turn:         turn,
			Phase:        molecule.Phase().String(),
			Gear:         GearNovel,
			Domain:       domain.String(),
			TokensIn:     completion.Tokens.Input,
			TokensOut:    completion.Tokens.Output,
			ToolCalls:    len(completion.ToolCalls),
			Distance:     molecule.Distance(),
			DeltaDistance: molecule.DeltaDistance(),
			Momentum:     molecule.Momentum(),
			UnmetCount:   unmetCount,
			ElapsedMs:    elapsed.Milliseconds(),
		}
		cb.emit("cerebrum.turn", tr.Labels())
		turnRecords = append(turnRecords, tr)
		slog.DebugContext(ctx, "cerebrum.think.response_content",
			slog.Int("turn", turn),
			slog.String("content", completion.Content))

		history = append(history, tangle.Message{Role: "user", Content: prompt})
		history = append(history, tangle.Message{
			Role:      "assistant",
			Content:   completion.Content,
			ToolCalls: completion.ToolCalls,
		})

		if len(completion.ToolCalls) > 0 {
			var phaseAtoms []reactivity.Atom
			var capCalls []tangle.ToolCall

			for _, tc := range completion.ToolCalls {
				if isPhaseToolCall(tc.Name) {
					atom, err := phaseToolCallToAtom(tc, molecule.Phase(), turn)
					if err != nil {
						slog.WarnContext(ctx, "cerebrum.think.phase_tool_error",
							slog.String("tool", tc.Name), slog.Any("error", err))
						continue
					}
					phaseAtoms = append(phaseAtoms, atom)
					history = append(history, tangle.Message{
						Role:       "tool",
						Content:    fmt.Sprintf("atom %s recorded: %s", atom.Type, atom.Taxonomy),
						ToolCallID: tc.ID,
					})
				} else {
					capCalls = append(capCalls, tc)
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
						Kind:       "instrument",
						Target:     tc.Name,
						Payload:    tc.Input,
						ToolCallID: tc.ID,
					})
				}
			}

			if len(capCalls) > 0 {
				cb.dispatch(ctx, molecule)
				toolCtx, toolCancel := context.WithTimeout(ctx, cb.budget.TurnTimeout)
				for _, tc := range capCalls {
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
						cb.checkCatalystDesired(molecule, tc.Name, result)
					}
				}
				toolCancel()
			}

			if molecule.Sealed() {
				slog.InfoContext(ctx, "cerebrum.think.catalyst_sealed",
					slog.Int("turn", turn),
					slog.Float64("distance", molecule.Distance()))
				break
			}

			if len(phaseAtoms) > 0 {
				for _, atom := range phaseAtoms {
					result, fortune := cb.reactor.Add(molecule, atom)
					slog.InfoContext(ctx, "cerebrum.think.phase_atom",
						slog.Int("turn", turn),
						slog.String("atom_type", atom.Type.String()),
						slog.String("taxonomy", atom.Taxonomy),
						slog.String("result", result.String()),
						slog.Any("dimensions", atom.Dimensions))

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

	summary := computeSessionSummary(molecule.ID, turnRecords, molecule)
	cb.emit("cerebrum.session", summary.Labels())

	slog.InfoContext(ctx, "cerebrum.think.session_summary",
		slog.String("molecule", molecule.ID),
		slog.Int("turns", summary.TotalTurns),
		slog.Int("tokens_in", summary.TotalTokensIn),
		slog.Int("tokens_out", summary.TotalTokensOut),
		slog.Float64("oae", summary.OAE),
		slog.Float64("gear_novel_pct", summary.GearNovelPct),
		slog.Float64("gear_familiar_pct", summary.GearFamiliarPct),
		slog.Float64("gear_reflex_pct", summary.GearReflexPct),
		slog.Int("reflex_hits", summary.ReflexHits),
		slog.Int64("avg_turn_ms", summary.AvgTurnMs),
		slog.Float64("final_distance", summary.FinalDistance))

	cb.molecule = molecule
	cb.reactor.Monolog().Park()
	return nil
}

func (cb *Cerebrum) Result() *reactivity.Molecule {
	return cb.molecule
}

func (cb *Cerebrum) Monitor(ctx context.Context, bus Bus) {
	for {
		event, ok := bus.Receive(ctx)
		if !ok {
			return
		}

		m := cb.molecule
		if m == nil {
			slog.DebugContext(ctx, "monitor.no_molecule", slog.String("event", event.Kind))
			continue
		}

		priority := cb.priorityClassifier.Classify(event, m)

		slog.InfoContext(ctx, "monitor.classify",
			slog.String("event", event.Kind),
			slog.String("priority", priority.String()),
			slog.String("molecule", m.ID))

		switch priority {
		case PriorityIgnore:
			continue

		case PriorityPark:
			select {
			case cb.monitorEvents <- event:
			default:
				slog.WarnContext(ctx, "monitor.park_overflow", slog.String("event", event.Kind))
			}

		case PriorityInterrupt:
			atom, err := cb.synapse.Encode(event)
			if err != nil {
				slog.WarnContext(ctx, "monitor.encode_error", slog.Any("error", err))
				continue
			}
			m.InsertAtom(atom)
			slog.InfoContext(ctx, "monitor.injected",
				slog.String("atom", atom.ID),
				slog.String("molecule", m.ID))

		case PriorityEmergency:
			if cb.halter != nil {
				cb.halter.Pull(m.ID)
				slog.WarnContext(ctx, "monitor.emergency_halt",
					slog.String("event", event.Kind),
					slog.String("molecule", m.ID))
			}
		}
	}
}

func (cb *Cerebrum) DrainMonitorEvents() []Event {
	var events []Event
	for {
		select {
		case e := <-cb.monitorEvents:
			events = append(events, e)
		default:
			return events
		}
	}
}

func (cb *Cerebrum) Store() *reactivity.MoleculeStore {
	return cb.reactor.Monolog()
}

func (cb *Cerebrum) SensoryBus() Bus {
	return cb.sensory
}

func (cb *Cerebrum) tools(phase reactivity.AtomType) []tangle.Tool {
	if len(cb.capabilities) == 0 {
		return cb.toolDefs
	}

	tools := []tangle.Tool{phaseToolFor(phase)}

	for _, cap := range cb.capabilities {
		if cap.Mode == shell.WriteAction && phase.Triad != reactivity.ImplementTriad {
			continue
		}
		tools = append(tools, tangle.Tool{
			Name:        cap.Name,
			Description: cap.Description,
			InputSchema: cap.Schema,
		})
	}
	return tools
}

func (cb *Cerebrum) assemble(m *reactivity.Molecule, need []byte, domain Domain, turn int) string {
	raw := RawContext{
		Need:         need,
		Observer:     cb.observer,
		Molecule:     m,
		Capabilities: cb.capabilities,
		Domain:       domain,
		Contracts:    cb.reactor.Contracts(),
		Directives:   cb.reactor.Directives(m.Phase()),
		Config:       cb.config,
		Turn:         turn,
	}
	ctx := cb.regulate(raw)
	return cb.render(ctx)
}

func (cb *Cerebrum) regulate(raw RawContext) Context {
	if cb.regulator != nil {
		return cb.regulator.Regulate(raw)
	}
	return defaultRegulate(raw)
}

func (cb *Cerebrum) render(ctx Context) string {
	if cb.assembler != nil {
		return cb.assembler.Assemble(ctx)
	}
	return defaultRender(ctx)
}

func (cb *Cerebrum) emit(action string, labels map[string]string) {
	if cb.recorder == nil {
		return
	}
	cb.recorder.Append(Record{
		Action:    action,
		Timestamp: time.Now(),
		Labels:    labels,
	})
}

func (cb *Cerebrum) pull(agentID string) {
	if cb.halter == nil {
		return
	}
	cb.halter.Pull(agentID)
}

func (cb *Cerebrum) checkCatalystDesired(m *reactivity.Molecule, toolName, result string) {
	cat := m.Catalyst()
	if cat == nil {
		return
	}
	for key, expected := range cat.Desired {
		switch v := expected.(type) {
		case bool:
			lower := strings.ToLower(result)
			hasKey := strings.Contains(lower, strings.ToLower(key))
			negated := strings.Contains(lower, "not "+strings.ToLower(key)) ||
				strings.Contains(lower, "no longer "+strings.ToLower(key))
			if v && hasKey && !negated {
				m.ReportSensor(key, true)
			} else if !v && negated {
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
	for {
		event, ok := cb.sensory.Receive(ctx)
		if !ok {
			return "tool call timed out"
		}
		if event.ToolCallID == tc.ID || (event.ToolCallID == "" && event.Source == tc.Name) {
			return string(event.Payload)
		}
		select {
		case cb.monitorEvents <- event:
		default:
			slog.Warn("cerebrum.waitToolResult.overflow",
				slog.String("event", event.Kind),
				slog.String("expected_tool", tc.Name))
		}
	}
}

func (cb *Cerebrum) dispatch(ctx context.Context, m *reactivity.Molecule) {
	for _, e := range m.DrainEmissions() {
		event := cb.synapse.Decode(e)
		if cb.motor != nil {
			cb.motor.Send(ctx, event)
		}
	}
}
