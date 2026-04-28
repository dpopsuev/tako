package contracts

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tangle/visual"
)

const testNodeName = "test"

// RunWalkerContract runs the walker compliance suite.
func RunWalkerContract(t *testing.T, factory func() circuit.Walker) {
	t.Helper()

	t.Run("Identity_NonEmpty", func(t *testing.T) {
		w := factory()
		id := w.Identity()
		if id.Name == "" {
			t.Error("Identity().Name must be non-empty")
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
		s.CurrentNode = testNodeName
		if s.CurrentNode != testNodeName {
			t.Error("State().CurrentNode not persisted")
		}
		// WalkerState is intentionally NOT thread-safe — walkers
		// are single-goroutine. No concurrent access test.
	})

	t.Run("Handle_ProducesArtifact", func(t *testing.T) {
		w := factory()
		node := &contractNode{name: testNodeName}
		nc := circuit.NodeContext{
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

func (n *contractNode) Name() string               { return n.name }
func (n *contractNode) Approach() visual.Element { return "" }
func (n *contractNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return &contractArt{typ: n.name}, nil
}

type contractArt struct{ typ string }

func (a *contractArt) Type() string        { return a.typ }
func (a *contractArt) Confidence() float64 { return 1.0 }
func (a *contractArt) Raw() any            { return nil }
