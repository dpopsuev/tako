package cerebrum

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/instrument"
	"github.com/dpopsuev/tako/memory"
	troupe "github.com/dpopsuev/tangle"
)

type Cerebrum struct {
	circuit   *reactivity.Reactor
	completer troupe.Completer
	maxTurns  int

	shell   instrument.Shell
	mesh    memory.Mesh
	monolog discourse.Monolog

	classifier    Classifier
	promptBuilder PromptBuilder
	parser        ResponseParser
	recollector   Recollector
	dispatcher    Dispatcher

	molecule *reactivity.Molecule
}

func New(circuit *reactivity.Reactor, completer troupe.Completer, opts ...Option) *Cerebrum {
	cb := &Cerebrum{
		circuit:       circuit,
		completer:     completer,
		maxTurns:      100,
		classifier:    DefaultClassifier,
		promptBuilder: DefaultPromptBuilder,
		parser:        DefaultParser,
		recollector:   DefaultRecollector,
		dispatcher:    DefaultDispatcher,
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

	var recollected []reactivity.Atom
	if cb.mesh != nil {
		recollected = cb.recollector.Recollect(cb.mesh, need)
		for _, atom := range recollected {
			cb.circuit.Add(m, atom)
		}
	}

	toolBudget := 10

	for turn := 0; turn < cb.maxTurns && !m.Sealed(); turn++ {
		domain := cb.classifier.Classify(m)
		prompt := cb.promptBuilder.Build(m, need, domain, cb.shell, recollected)

		response, err := cb.completer.Complete(ctx, prompt)
		if err != nil {
			cb.circuit.Seal(m, reactivity.Atom{
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
			result, fortune := cb.circuit.Add(m, atom)
			if result == reactivity.Unresolvable {
				cb.circuit.Seal(m, reactivity.Atom{
					ID:        fmt.Sprintf("wish-unresolvable-%d", turn),
					Type:      reactivity.RetrospectionAtom,
					Taxonomy:  "retrospection.wish.unresolvable",
					Content:   []byte(fortune.Message),
					CreatedAt: time.Now(),
				})
				return m, nil
			}
		}

		if toolCall != nil && cb.shell != nil && m.Phase() == reactivity.ExecutionAtom && toolBudget > 0 {
			instrumentAtom, err := cb.dispatcher.Dispatch(ctx, cb.shell, toolCall)
			if err == nil {
				cb.circuit.Add(m, instrumentAtom)
				toolBudget--
			}
		}
	}

	if !m.Sealed() {
		cb.circuit.Seal(m, reactivity.Atom{
			ID:        "wish-max-turns",
			Type:      reactivity.RetrospectionAtom,
			Taxonomy:  "retrospection.wish.max-turns-exceeded",
			Content:   []byte("exceeded max turns"),
			CreatedAt: time.Now(),
		})
	}

	if cb.monolog != nil {
		cb.monolog.Write(discourse.Letter{
			From:      "cerebrum",
			Subject:   "think-complete",
			Body:      fmt.Sprintf("sealed molecule %s: %d atoms, domain=%s", m.ID, m.TotalMass(), cb.classifier.Classify(m)),
			CreatedAt: time.Now(),
		})
	}

	return m, nil
}
