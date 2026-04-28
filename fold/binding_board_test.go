package fold

import (
	"testing"
)

func TestResolve_BoardUsesAndBind(t *testing.T) {
	root := takoRoot(t)

	m := &Manifest{
		Kind:    "Board",
		Name:    "test-board",
		Version: "1.0",
		Uses: map[string]UsesRef{
			"alpha": {
				Kind:   "Schematic",
				Module: "github.com/example/schematic-a",
			},
			"datasource": {
				Kind:   "Component",
				Module: "github.com/example/schematic-a/connectors/rp",
			},
		},
		Bind: map[string]map[string]string{
			"alpha": {
				"source": "datasource",
			},
		},
	}

	g, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatal(err)
	}

	if g.Root.Name != "alpha" {
		t.Errorf("root = %q, want alpha", g.Root.Name)
	}

	// Sockets have no option: fields — no resolved options expected.
	if len(g.Root.Options) != 0 {
		t.Errorf("root options = %v, want none (sockets have no option: field)", g.Root.Options)
	}
}
