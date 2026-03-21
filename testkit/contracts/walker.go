package contracts

import (
	"context"
	"testing"

	framework "github.com/dpopsuev/origami"
)

// RunWalkerContract runs the walker compliance suite.
func RunWalkerContract(t *testing.T, factory func() framework.Walker) {
	t.Helper()

	t.Run("Identity_NonEmpty", func(t *testing.T) {
		w := factory()
		id := w.Identity()
		if id.PersonaName == "" {
			t.Error("Identity().PersonaName must be non-empty")
		}
	})

	t.Run("State_NonNil", func(t *testing.T) {
		w := factory()
		if w.State() == nil {
			t.Error("State() must return non-nil WalkerState")
		}
	})

	t.Run("State_Consistent", func(t *testing.T) {
		w := factory()
		s := w.State()
		s.CurrentNode = "test"
		if s.CurrentNode != "test" {
			t.Error("State().CurrentNode not persisted")
		}
		// WalkerState is intentionally NOT thread-safe — walkers
		// are single-goroutine. No concurrent access test.
	})

	t.Run("Handle_ProducesArtifact", func(t *testing.T) {
		w := factory()
		node := &contractNode{name: "test"}
		nc := framework.NodeContext{
			WalkerState: w.State(),
			Meta:        make(map[string]any),
		}
		art, err := w.Handle(context.Background(), node, nc)
		if err != nil {
			t.Fatalf("Handle returned error: %v", err)
		}
		if art == nil {
			t.Error("Handle returned nil artifact")
		}
	})
}

type contractNode struct {
	name string
}

func (n *contractNode) Name() string                     { return n.name }
func (n *contractNode) ElementAffinity() framework.Element { return "" }
func (n *contractNode) Process(_ context.Context, _ framework.NodeContext) (framework.Artifact, error) {
	return &contractArt{typ: n.name}, nil
}

type contractArt struct{ typ string }

func (a *contractArt) Type() string       { return a.typ }
func (a *contractArt) Confidence() float64 { return 1.0 }
func (a *contractArt) Raw() any           { return nil }
