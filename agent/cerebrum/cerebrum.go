package cerebrum

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	troupe "github.com/dpopsuev/tangle"
)

// Cerebrum is the agent's mind. Circuit + Monolog + Completer.
// Think(need) runs the ReActivity loop: Need → Molecule → Wish.
type Cerebrum struct {
	circuit   *reactivity.Circuit
	completer troupe.Completer
	maxTurns  int
}

// New creates a Cerebrum.
func New(circuit *reactivity.Circuit, completer troupe.Completer) *Cerebrum {
	return &Cerebrum{
		circuit:   circuit,
		completer: completer,
		maxTurns:  100,
	}
}

// Think processes a Need through the ReActivity Circuit.
// Returns the sealed Molecule or an error.
func (cb *Cerebrum) Think(ctx context.Context, need []byte) (*reactivity.Molecule, error) {
	m := reactivity.NewMolecule(fmt.Sprintf("mol-%d", time.Now().UnixNano()))

	for turn := 0; turn < cb.maxTurns && !m.Sealed(); turn++ {
		prompt := cb.buildPrompt(m, need)

		response, err := cb.completer.Complete(ctx, string(prompt))
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

		atom := cb.parseAtom(m, response, turn)

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

	if !m.Sealed() {
		cb.circuit.Seal(m, reactivity.Atom{
			ID:        "wish-max-turns",
			Type:      reactivity.RetrospectionAtom,
			Taxonomy:  "retrospection.wish.max-turns-exceeded",
			Content:   []byte("exceeded max turns"),
			CreatedAt: time.Now(),
		})
	}

	return m, nil
}

func (cb *Cerebrum) buildPrompt(m *reactivity.Molecule, need []byte) string {
	phase := m.Phase()
	mass := m.Mass(phase)

	prompt := fmt.Sprintf("phase:%s mass:%d need:%s", phase, mass, string(need))
	return prompt
}

func (cb *Cerebrum) parseAtom(m *reactivity.Molecule, response string, turn int) reactivity.Atom {
	phase := m.Phase()
	return reactivity.Atom{
		ID:        fmt.Sprintf("atom-%s-%d", phase, turn),
		Type:      phase,
		Source:    reactivity.Fresh,
		Taxonomy:  fmt.Sprintf("%s.response.turn-%d", phase, turn),
		Content:   []byte(response),
		CreatedAt: time.Now(),
	}
}
