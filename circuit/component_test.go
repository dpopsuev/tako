package circuit

import (
	"os"
	"path/filepath"
	"testing"
)

func loadComponentFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", "component", name+".yaml")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "component.yaml")
	os.WriteFile(path, data, 0644)
	return path
}

func TestLoadComponentManifest_ValidComponent(t *testing.T) {
	m, err := LoadComponentManifest(loadComponentFixture(t, "valid-component"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != "Component" {
		t.Errorf("Kind = %q, want Component", m.Kind)
	}
}

func TestLoadComponentManifest_ValidSchematic(t *testing.T) {
	m, err := LoadComponentManifest(loadComponentFixture(t, "valid-schematic"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != "Schematic" {
		t.Errorf("Kind = %q, want Schematic", m.Kind)
	}
}

func TestLoadComponentManifest_RejectsMissingAPIVersion(t *testing.T) {
	_, err := LoadComponentManifest(loadComponentFixture(t, "missing-apiversion"))
	if err == nil {
		t.Fatal("expected error for missing apiVersion")
	}
}

func TestLoadComponentManifest_RejectsWrongKind(t *testing.T) {
	_, err := LoadComponentManifest(loadComponentFixture(t, "wrong-kind-board"))
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestLoadComponentManifest_RejectsTransportInSources(t *testing.T) {
	_, err := LoadComponentManifest(loadComponentFixture(t, "transport-in-sources"))
	if err == nil {
		t.Fatal("expected error for Transport in sources")
	}
}

func TestLoadComponentManifest_RejectsSourceReaderInTransports(t *testing.T) {
	_, err := LoadComponentManifest(loadComponentFixture(t, "sourcereader-in-transports"))
	if err == nil {
		t.Fatal("expected error for SourceReader in transports")
	}
}

func TestLoadComponentManifest_RejectsDriverInTransports(t *testing.T) {
	_, err := LoadComponentManifest(loadComponentFixture(t, "driver-in-transports"))
	if err == nil {
		t.Fatal("expected error for Driver in transports")
	}
}

func TestLoadComponentManifest_RejectsTriggerInStorage(t *testing.T) {
	_, err := LoadComponentManifest(loadComponentFixture(t, "trigger-in-storage"))
	if err == nil {
		t.Fatal("expected error for Trigger in storage")
	}
}

func TestLoadComponentManifest_NeedsGivesParse(t *testing.T) {
	m, err := LoadComponentManifest(loadComponentFixture(t, "needs-gives"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Needs.Sources) != 1 {
		t.Errorf("needs.sources = %d, want 1", len(m.Needs.Sources))
	}
	if len(m.Gives) != 1 {
		t.Errorf("gives = %d, want 1", len(m.Gives))
	}
	if m.Gives[0].Factory != "NewSourceReader" {
		t.Errorf("gives[0].factory = %q, want NewSourceReader", m.Gives[0].Factory)
	}
}
