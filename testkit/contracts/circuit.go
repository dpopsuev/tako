package contracts

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/circuit/def"
)

// RunCircuitDefContract verifies that LoadCircuit produces a well-formed
// CircuitDef. The factory must return valid circuit YAML bytes.
func RunCircuitDefContract(t *testing.T, factory func() []byte) {
	t.Helper()

	t.Run("LoadCircuit_Parses", func(t *testing.T) {
		data := factory()
		d, err := def.LoadCircuit(data)
		if err != nil {
			t.Fatalf("LoadCircuit: %v", err)
		}
		if d.Circuit == "" {
			t.Error("circuit name is empty")
		}
		if d.Start == "" {
			t.Error("start node is empty")
		}
		if d.Done == "" {
			t.Error("done node is empty")
		}
	})

	t.Run("LoadCircuit_HasNodes", func(t *testing.T) {
		data := factory()
		d, err := def.LoadCircuit(data)
		if err != nil {
			t.Fatalf("LoadCircuit: %v", err)
		}
		if len(d.Nodes) == 0 {
			t.Error("circuit has no nodes")
		}
	})

	t.Run("LoadCircuit_HasEdges", func(t *testing.T) {
		data := factory()
		d, err := def.LoadCircuit(data)
		if err != nil {
			t.Fatalf("LoadCircuit: %v", err)
		}
		if len(d.Edges) == 0 {
			t.Error("circuit has no edges")
		}
	})

	t.Run("LoadCircuit_StartNodeExists", func(t *testing.T) {
		data := factory()
		d, err := def.LoadCircuit(data)
		if err != nil {
			t.Fatalf("LoadCircuit: %v", err)
		}
		found := false
		for i := range d.Nodes {
			if d.Nodes[i].Name == d.Start {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("start node %q not found in nodes", d.Start)
		}
	})
}

// RunArtifactValidationContract verifies that ValidateArtifact correctly
// accepts and rejects artifacts against a schema.
func RunArtifactValidationContract(t *testing.T) {
	t.Helper()

	schema := &circuit.ArtifactSchema{
		Type:     "object",
		Required: []string{"score"},
		Fields: map[string]circuit.FieldSchema{
			"score": {Type: "number"},
		},
	}

	t.Run("ValidArtifact_Passes", func(t *testing.T) {
		art := &mapArtifact{data: map[string]any{"score": 0.95}}
		if err := circuit.ValidateArtifact(schema, art); err != nil {
			t.Errorf("valid artifact rejected: %v", err)
		}
	})

	t.Run("MissingRequired_Fails", func(t *testing.T) {
		art := &mapArtifact{data: map[string]any{"other": "value"}}
		if err := circuit.ValidateArtifact(schema, art); err == nil {
			t.Error("missing required field should fail validation")
		}
	})

	t.Run("NilSchema_Passes", func(t *testing.T) {
		art := &mapArtifact{data: map[string]any{"anything": true}}
		if err := circuit.ValidateArtifact(nil, art); err != nil {
			t.Errorf("nil schema should accept anything: %v", err)
		}
	})
}

// mapArtifact wraps a map as a circuit.Artifact for testing.
type mapArtifact struct {
	data map[string]any
}

func (a *mapArtifact) Type() string        { return "map" }
func (a *mapArtifact) Confidence() float64 { return 1.0 }
func (a *mapArtifact) Raw() any            { return a.data }
