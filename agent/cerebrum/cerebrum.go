package cerebrum

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/artifact"
	troupe "github.com/dpopsuev/tangle"
)

type Cerebrum struct {
	reactor   *reactivity.Core
	completer troupe.Completer
	maxTurns  int

	sensory Bus
	motor   Bus
	signal  Bus

	classifier    Classifier
	promptBuilder PromptBuilder
	parser        ResponseParser

	molecule *reactivity.Molecule
}

func New(reactor *reactivity.Core, completer troupe.Completer, opts ...Option) *Cerebrum {
	cb := &Cerebrum{
		reactor:       reactor,
		completer:     completer,
		maxTurns:      100,
		classifier:    DefaultClassifier,
		promptBuilder: DefaultPromptBuilder,
		parser:        DefaultParser,
		sensory:       NewChannelBus(64),
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
	for {
		event, ok := cb.sensory.Receive(ctx)
		if !ok {
			return
		}
		atom := eventToAtom(event)
		molecule := cb.reactor.Cognize(atom)
		cb.dispatch(ctx, molecule)

		if molecule.Sealed() {
			cb.molecule = molecule
			cb.reactor.Monolog().Park()
		}
	}
}

func (cb *Cerebrum) Think(ctx context.Context, need []byte) error {
	molecule := cb.reactor.Cognize(reactivity.Atom{
		ID:        fmt.Sprintf("need-%d", time.Now().UnixNano()),
		Type:      reactivity.IntentAtom,
		Taxonomy:  "intent.need",
		Content:   need,
		CreatedAt: time.Now(),
	})

	for turn := 0; turn < cb.maxTurns && !molecule.Sealed(); turn++ {
		domain := cb.classifier.Classify(molecule)
		directives := cb.reactor.Directives(molecule.Phase())
		prompt := cb.promptBuilder.Build(molecule, need, domain)
		for _, d := range directives {
			prompt += "\n> " + string(d)
		}

		response, err := cb.completer.Complete(ctx, prompt)
		if err != nil {
			cb.reactor.Seal(molecule, reactivity.Atom{
				ID:        fmt.Sprintf("wish-error-%d", turn),
				Type:      reactivity.RetrospectionAtom,
				Taxonomy:  "retrospection.wish.completer-error",
				Content:   []byte(err.Error()),
				CreatedAt: time.Now(),
			})
			break
		}

		atoms, _, _ := cb.parser.Parse(response, molecule.Phase(), turn)
		for _, atom := range atoms {
			result, fortune := cb.reactor.Add(molecule, atom)
			if result == reactivity.Unresolvable {
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

	if !molecule.Sealed() {
		cb.reactor.Seal(molecule, reactivity.Atom{
			ID:        "wish-max-turns",
			Type:      reactivity.RetrospectionAtom,
			Taxonomy:  "retrospection.wish.max-turns-exceeded",
			Content:   []byte("exceeded max turns"),
			CreatedAt: time.Now(),
		})
	}

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

func eventToAtom(e Event) reactivity.Atom {
	return reactivity.Atom{
		ID:        e.ID,
		Type:      reactivity.IntentAtom,
		Source:    reactivity.Received,
		Taxonomy:  "intent." + e.Kind,
		Content:   e.Payload,
		CreatedAt: e.CreatedAt,
	}
}

func (cb *Cerebrum) dispatch(ctx context.Context, m *reactivity.Molecule) {
	for _, e := range m.DrainEmissions() {
		event := Event{
			ID:        fmt.Sprintf("emission-%d", time.Now().UnixNano()),
			Kind:      e.Kind,
			Source:    e.Target,
			Payload:   e.Payload,
			CreatedAt: time.Now(),
		}
		if cb.motor != nil {
			cb.motor.Send(ctx, event)
		}
	}
}
