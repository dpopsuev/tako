package cerebrum

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	troupe "github.com/dpopsuev/tangle"
)

type Cerebrum struct {
	reactor   *reactivity.Reactor
	completer troupe.Completer
	maxTurns  int

	sensory SensoryBus
	motor   MotorBus

	classifier    Classifier
	promptBuilder PromptBuilder
	parser        ResponseParser

	molecule *reactivity.Molecule
}

func New(reactor *reactivity.Reactor, completer troupe.Completer, opts ...Option) *Cerebrum {
	cb := &Cerebrum{
		reactor:       reactor,
		completer:     completer,
		maxTurns:      100,
		classifier:    DefaultClassifier,
		promptBuilder: DefaultPromptBuilder,
		parser:        DefaultParser,
	}
	for _, opt := range opts {
		opt(cb)
	}
	return cb
}

func (cb *Cerebrum) Think(ctx context.Context, need []byte) error {
	m, err := cb.think(ctx, need)
	if err != nil {
		return err
	}
	cb.molecule = m
	return nil
}

func (cb *Cerebrum) Result() *reactivity.Molecule {
	return cb.molecule
}

func (cb *Cerebrum) think(ctx context.Context, need []byte) (*reactivity.Molecule, error) {
	m := reactivity.NewMolecule(fmt.Sprintf("mol-%d", time.Now().UnixNano()))

	toolBudget := 10

	for turn := 0; turn < cb.maxTurns && !m.Sealed(); turn++ {
		domain := cb.classifier.Classify(m)
		prompt := cb.promptBuilder.Build(m, need, domain)

		response, err := cb.completer.Complete(ctx, prompt)
		if err != nil {
			cb.reactor.Seal(m, reactivity.Atom{
				ID:        fmt.Sprintf("wish-error-%d", turn),
				Type:      reactivity.RetrospectionAtom,
				Taxonomy:  "retrospection.wish.completer-error",
				Content:   []byte(err.Error()),
				CreatedAt: time.Now(),
			})
			return m, nil
		}

		atoms, toolCall, _ := cb.parser.Parse(response, m.Phase(), turn)

		for _, atom := range atoms {
			result, fortune := cb.reactor.Add(m, atom)
			if result == reactivity.Unresolvable {
				cb.reactor.Seal(m, reactivity.Atom{
					ID:        fmt.Sprintf("wish-unresolvable-%d", turn),
					Type:      reactivity.RetrospectionAtom,
					Taxonomy:  "retrospection.wish.unresolvable",
					Content:   []byte(fortune.Message),
					CreatedAt: time.Now(),
				})
				return m, nil
			}
		}

		if toolCall != nil && cb.motor != nil && m.Phase() == reactivity.ExecutionAtom && toolBudget > 0 {
			payload, _ := json.Marshal(toolCall)
			cb.motor.Send(ctx, Command{Kind: "instrument", Target: toolCall.Name, Payload: payload})

			if cb.sensory != nil {
				if sig, ok := cb.sensory.Receive(ctx); ok {
					instrumentAtom := reactivity.Atom{
						ID:        fmt.Sprintf("instrument-%s-%d", toolCall.Name, turn),
						Type:      reactivity.ExecutionAtom,
						Source:    reactivity.Instrument,
						Taxonomy:  fmt.Sprintf("execution.instrument.%s", toolCall.Name),
						Content:   sig.Content,
						CreatedAt: time.Now(),
					}
					cb.reactor.Add(m, instrumentAtom)
					toolBudget--
				}
			}
		}
	}

	if !m.Sealed() {
		cb.reactor.Seal(m, reactivity.Atom{
			ID:        "wish-max-turns",
			Type:      reactivity.RetrospectionAtom,
			Taxonomy:  "retrospection.wish.max-turns-exceeded",
			Content:   []byte("exceeded max turns"),
			CreatedAt: time.Now(),
		})
	}

	if cb.motor != nil {
		cb.motor.Send(ctx, Command{
			Kind:    "wish",
			Target:  "monolog",
			Payload: []byte(fmt.Sprintf("sealed %s: %d atoms, domain=%s", m.ID, m.TotalMass(), cb.classifier.Classify(m))),
		})
	}

	return m, nil
}
