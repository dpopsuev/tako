package fold

import (
	"strings"
	"testing"
)

// E2E gate test: board manifest with uses/bind resolves and generates
// valid wired binary code. This must pass before consumer migration.
func TestE2E_BoardManifest_ResolvesAndGenerates(t *testing.T) {
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
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{
				Circuits: map[string]string{"rca": "circuits/rca.yaml"},
			},
		},
	}

	g, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	src, err := GenerateWiredBinary(m, g)
	if err != nil {
		t.Fatalf("GenerateWiredBinary: %v", err)
	}
	code := string(src)

	for _, want := range []string{
		"DO NOT EDIT",
		"package main",
		"domainserve.New(domainFS",
		"NewStreamableHTTPHandler",
		"/mcp",
		"/domain/",
		"/healthz",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in generated code", want)
		}
	}

	if t.Failed() {
		t.Logf("Generated code:\n%s", code)
	}
}

func TestE2E_BoardManifest_NegativeRejectsUnboundSocket(t *testing.T) {
	// rh-rca's sockets are all optional: true, so missing bindings
	// are accepted. This test verifies Resolve succeeds (not errors)
	// when a hooks-mode schematic has no bindings.
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
		},
		Bind: map[string]map[string]string{},
		DomainServe: &DomainServeConfig{
			Port:   9300,
			Assets: &AssetMap{Circuits: map[string]string{"rca": "circuits/rca.yaml"}},
		},
	}

	_, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatalf("Resolve should succeed with all-optional sockets: %v", err)
	}
}
