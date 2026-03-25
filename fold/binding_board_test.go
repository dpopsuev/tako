package fold

import (
	"strings"
	"testing"
)

func TestResolve_BoardUsesAndBind(t *testing.T) {
	root := origamiRoot(t)

	m := &Manifest{
		Kind:    "board",
		Name:    "test-board",
		Version: "1.0",
		Uses: map[string]UsesRef{
			"rca": {
				Kind:   "schematic",
				Module: "github.com/dpopsuev/rh-rca",
			},
			"reportportal": {
				Kind:   "component",
				Module: "github.com/dpopsuev/rh-rca/connectors/rp",
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

	var optNames []string
	for _, o := range g.Root.Options {
		optNames = append(optNames, o.OptionFunc)
	}
	if !contains(optNames, "WithSourceReader") {
		t.Errorf("root options %v missing WithSourceReader", optNames)
	}

	for _, o := range g.Root.Options {
		if o.OptionFunc == "WithSourceReader" {
			if !strings.Contains(o.Provider, "NewSourceReader") {
				t.Errorf("WithSourceReader provider = %q, want *NewSourceReader*", o.Provider)
			}
		}
	}
}
