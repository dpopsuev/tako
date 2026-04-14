package fold

import "testing"

func TestParseManifest_Board_ValidUsesAndBind(t *testing.T) {
	m, err := ParseManifest(loadFixtureManifest(t, "board-uses-bind"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Kind != "Board" {
		t.Errorf("Kind = %q, want Board", m.Kind)
	}
	if len(m.Uses) != 3 {
		t.Errorf("Uses = %d, want 3", len(m.Uses))
	}
	if m.Uses["alpha"].Kind != "Schematic" {
		t.Errorf("Uses[alpha].Kind = %q, want Schematic", m.Uses["alpha"].Kind)
	}
	if m.Uses["datasource"].Module != "github.com/dpopsuev/origami-components/rp" {
		t.Errorf("Uses[datasource].Module = %q", m.Uses["datasource"].Module)
	}
	if m.Bind["alpha"]["data"] != "datasource" {
		t.Errorf("Bind[alpha][data] = %q, want datasource", m.Bind["alpha"]["data"])
	}
}

func TestParseManifest_Board_RejectsUnknownBindTarget(t *testing.T) {
	_, err := ParseManifest(loadFixtureManifest(t, "bind-unknown-target"))
	if err == nil {
		t.Fatal("expected error for bind referencing nonexistent component")
	}
}

func TestParseManifest_Board_RejectsBindForUnknownSchematic(t *testing.T) {
	_, err := ParseManifest(loadFixtureManifest(t, "bind-unknown-schematic"))
	if err == nil {
		t.Fatal("expected error for bind referencing nonexistent schematic")
	}
}

func TestParseManifest_Board_RejectsMissingModule(t *testing.T) {
	_, err := ParseManifest(loadFixtureManifest(t, "missing-module"))
	if err == nil {
		t.Fatal("expected error for uses entry without module")
	}
}

func TestParseManifest_Board_HasBindings(t *testing.T) {
	m, err := ParseManifest(loadFixtureManifest(t, "has-bindings"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if !m.HasBindings() {
		t.Error("HasBindings() = false, want true")
	}
}
