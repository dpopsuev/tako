package circuit

import (
	"os"
	"path/filepath"
	"testing"
)

func writeComponentYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "component.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadComponentManifest_ValidComponent(t *testing.T) {
	path := writeComponentYAML(t, `
kind: component
component: test
namespace: test
version: "1.0"
`)
	m, err := LoadComponentManifest(path)
	if err != nil {
		t.Fatalf("LoadComponentManifest: %v", err)
	}
	if m.Kind != "component" {
		t.Errorf("Kind = %q, want %q", m.Kind, "component")
	}
}

func TestLoadComponentManifest_RejectsMissingKind(t *testing.T) {
	path := writeComponentYAML(t, `
component: test
namespace: test
version: "1.0"
`)
	_, err := LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for missing kind, got nil")
	}
}

func TestLoadComponentManifest_RejectsWrongKind(t *testing.T) {
	for _, kind := range []string{"schematic", "board", "circuit", "scenario"} {
		t.Run(kind, func(t *testing.T) {
			path := writeComponentYAML(t, `
kind: `+kind+`
component: test
namespace: test
version: "1.0"
`)
			_, err := LoadComponentManifest(path)
			if err == nil {
				t.Fatalf("expected error for kind: %s in component.yaml, got nil", kind)
			}
		})
	}
}

// S40: Transport/Trigger only in transports: section
func TestLoadComponentManifest_RejectsTransportInSources(t *testing.T) {
	path := writeComponentYAML(t, `
kind: component
component: test
namespace: test
version: "1.0"
needs:
  sources:
    - name: mcp
      type: Transport
`)
	_, err := LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for Transport in sources: section, got nil")
	}
}

// S41: SourceReader/SourceCatalog only in sources: section
func TestLoadComponentManifest_RejectsSourceReaderInTransports(t *testing.T) {
	path := writeComponentYAML(t, `
kind: component
component: test
namespace: test
version: "1.0"
needs:
  transports:
    - name: data
      type: SourceReader
`)
	_, err := LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for SourceReader in transports: section, got nil")
	}
}

// S42: Driver only in storage: section
func TestLoadComponentManifest_RejectsDriverInTransports(t *testing.T) {
	path := writeComponentYAML(t, `
kind: component
component: test
namespace: test
version: "1.0"
needs:
  transports:
    - name: db
      type: Driver
`)
	_, err := LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for Driver in transports: section, got nil")
	}
}

func TestLoadComponentManifest_RejectsTriggerInStorage(t *testing.T) {
	path := writeComponentYAML(t, `
kind: component
component: test
namespace: test
version: "1.0"
needs:
  storage:
    - name: events
      type: Trigger
`)
	_, err := LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for Trigger in storage: section, got nil")
	}
}

func TestLoadComponentManifest_NeedsGivesParse(t *testing.T) {
	path := writeComponentYAML(t, `
kind: component
component: test-rca
namespace: rca
version: "1.0"
needs:
  sources:
    - name: data
      type: SourceReader
  storage:
    - name: db
      type: Driver
gives:
  - socket: data
    factory: NewSourceReader
`)
	m, err := LoadComponentManifest(path)
	if err != nil {
		t.Fatalf("LoadComponentManifest: %v", err)
	}
	if len(m.Needs.Sources) != 1 {
		t.Errorf("Needs.Sources = %d, want 1", len(m.Needs.Sources))
	}
	if len(m.Needs.Storage) != 1 {
		t.Errorf("Needs.Storage = %d, want 1", len(m.Needs.Storage))
	}
	if len(m.Gives) != 1 {
		t.Errorf("Gives = %d, want 1", len(m.Gives))
	}
}
