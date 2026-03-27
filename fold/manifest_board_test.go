package fold

import "testing"

func TestParseManifest_Board_ValidUsesAndBind(t *testing.T) {
	yaml := `
kind: board
name: asterisk
version: "1.0"

uses:
  rca:
    kind: schematic
    module: github.com/dpopsuev/origami-rca
  reportportal:
    kind: component
    module: github.com/dpopsuev/origami-components/rp
  mcp:
    kind: component
    module: github.com/dpopsuev/origami/connectors/mcp

bind:
  rca:
    data: reportportal
    transport: mcp

domains: [ocp/ptp]

domain_serve:
  port: 9300
  assets:
    circuits:
      rca: circuits/rca.yaml
`
	m, err := ParseManifest([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Kind != "board" {
		t.Errorf("Kind = %q, want board", m.Kind)
	}
	if len(m.Uses) != 3 {
		t.Errorf("Uses = %d, want 3", len(m.Uses))
	}
	if m.Uses["rca"].Kind != "schematic" {
		t.Errorf("Uses[rca].Kind = %q, want schematic", m.Uses["rca"].Kind)
	}
	if m.Uses["reportportal"].Module != "github.com/dpopsuev/origami-components/rp" {
		t.Errorf("Uses[reportportal].Module = %q", m.Uses["reportportal"].Module)
	}
	if m.Bind["rca"]["data"] != "reportportal" {
		t.Errorf("Bind[rca][data] = %q, want reportportal", m.Bind["rca"]["data"])
	}
}

func TestParseManifest_Board_RejectsUnknownBindTarget(t *testing.T) {
	yaml := `
kind: board
name: test
version: "1.0"

uses:
  rca:
    kind: schematic
    module: github.com/dpopsuev/origami-rca

bind:
  rca:
    data: nonexistent

domain_serve:
  port: 9300
  assets:
    circuits:
      rca: circuits/rca.yaml
`
	_, err := ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for bind referencing nonexistent component, got nil")
	}
}

func TestParseManifest_Board_RejectsBindForUnknownSchematic(t *testing.T) {
	yaml := `
kind: board
name: test
version: "1.0"

uses:
  mcp:
    kind: component
    module: github.com/dpopsuev/origami/connectors/mcp

bind:
  nonexistent:
    transport: mcp

domain_serve:
  port: 9300
  assets:
    circuits:
      rca: circuits/rca.yaml
`
	_, err := ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for bind referencing nonexistent schematic, got nil")
	}
}

func TestParseManifest_Board_RejectsMissingModule(t *testing.T) {
	yaml := `
kind: board
name: test
version: "1.0"

uses:
  rca:
    kind: schematic

domain_serve:
  port: 9300
  assets:
    circuits:
      rca: circuits/rca.yaml
`
	_, err := ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for uses entry without module, got nil")
	}
}

func TestParseManifest_Board_HasBindings(t *testing.T) {
	yaml := `
kind: board
name: test
version: "1.0"

uses:
  rca:
    kind: schematic
    module: github.com/dpopsuev/origami-rca

domain_serve:
  port: 9300
  assets:
    circuits:
      rca: circuits/rca.yaml
`
	m, err := ParseManifest([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if !m.HasBindings() {
		t.Error("HasBindings() = false, want true for manifest with uses")
	}
}
