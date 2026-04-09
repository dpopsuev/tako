package fold

import (
	"testing"
)

func TestResolve_BoardUsesAndBind(t *testing.T) {
	root := origamiRoot(t)

	m := &Manifest{
		Kind:    "Board",
		Name:    "test-board",
		Version: "1.0",
		Uses: map[string]UsesRef{
			"rca": {
				Kind:   "Schematic",
				Module: "github.com/dpopsuev/origami-rca",
			},
			"reportportal": {
				Kind:   "Component",
				Module: "github.com/dpopsuev/origami-rca/connectors/rp",
			},
		},
		Bind: map[string]map[string]string{
			"rca": {
				"source": "reportportal",
			},
		},
	}

	g, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatal(err)
	}

	if g.Root.Name != "rca" {
		t.Errorf("root = %q, want rca", g.Root.Name)
	}

	// RCA sockets have no option: fields — no resolved options expected.
	if len(g.Root.Options) != 0 {
		t.Errorf("root options = %v, want none (sockets have no option: field)", g.Root.Options)
	}
}
