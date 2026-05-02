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

	sensory chan reactivity.Atom
	motor   MotorBus

	classifier    Classifier
	promptBuilder PromptBuilder
	parser        ResponseParser

	store    *MoleculeStore
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
		store:         NewMoleculeStore(),
		sensory:       make(chan reactivity.Atom, 64),
	}
	for _, opt := range opts {
		opt(cb)
	}
	return cb
}

var _ organ.Organ = (*Cerebrum)(nil)

func (cb *Cerebrum) Name() organ.OrganName { return organ.CerebrumOrgan }

func (cb *Cerebrum) Receive(wire artifact.Wire) error {
	cb.sensory <- reactivity.Atom{
		ID:        fmt.Sprintf("wire-%d", time.Now().UnixNano()),
		Type:      reactivity.IntentAtom,
		Source:    reactivity.Received,
		Taxonomy:  "intent.wire." + wire.Kind,
		Content:   wire.Payload,
		CreatedAt: time.Now(),
	}
	return nil
}

func (cb *Cerebrum) Run(ctx context.Context) {
	for {
		select {
		case atom := <-cb.sensory:
			molecule := cb.cognize(atom)
			cb.reactor.React(molecule, atom)
			cb.dispatch(ctx, molecule)

			if molecule.Sealed() {
				cb.molecule = molecule
				cb.store.Park()
			}
		case <-ctx.Done():
			return
		}
	}
}

func (cb *Cerebrum) Think(ctx context.Context, need []byte) error {
	molecule := cb.cognize(reactivity.Atom{
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
	cb.store.Park()
	return nil
}

func (cb *Cerebrum) Result() *reactivity.Molecule {
	return cb.molecule
}

func (cb *Cerebrum) Store() *MoleculeStore {
	return cb.store
}

func (cb *Cerebrum) Sensory() chan<- reactivity.Atom {
	return cb.sensory
}

func (cb *Cerebrum) cognize(seed reactivity.Atom) *reactivity.Molecule {
	for _, id := range cb.store.Molecules() {
		m, ok := cb.store.Molecule(id)
		if !ok || m.Sealed() {
			continue
		}
		if matchesDomain(m, seed) {
			return cb.store.Focus(id)
		}
	}
	molID := fmt.Sprintf("mol-%d", time.Now().UnixNano())
	m := cb.store.Focus(molID)
	cb.reactor.React(m, seed)
	cb.dispatch(context.Background(), m)
	return m
}

func taxonomyDomain(taxonomy string) string {
	for i := len(taxonomy) - 1; i >= 0; i-- {
		if taxonomy[i] == '.' {
			return taxonomy[i+1:]
		}
	}
	return ""
}

func (cb *Cerebrum) dispatch(ctx context.Context, m *reactivity.Molecule) {
	if cb.motor == nil {
		m.DrainEmissions()
		return
	}
	for _, e := range m.DrainEmissions() {
		cb.motor.Send(ctx, Command{Kind: e.Kind, Target: e.Target, Payload: e.Payload})
	}
}
