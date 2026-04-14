package fold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit/def"
)

func writeGoFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "exports.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateExports_MissingOptionFunc(t *testing.T) {
	// RED: component.yaml declares option: WithSourceReader, module doesn't export it.
	modDir := t.TempDir()
	writeGoFile(t, modDir, "package alpha\n\nfunc Factory() {}\n")

	cm := &def.ComponentManifest{
		Module: "github.com/test/alpha",
		Needs: struct {
			Origami    string          `yaml:"origami,omitempty"`
			Transports []def.SocketDef `yaml:"transports,omitempty"`
			Sources    []def.SocketDef `yaml:"sources,omitempty"`
			Storage    []def.SocketDef `yaml:"storage,omitempty"`
		}{
			Sources: []def.SocketDef{
				{Name: "source", Option: "WithSourceReader"},
			},
		},
	}

	err := ValidateExports(cm, modDir)
	if err == nil {
		t.Fatal("expected error for missing WithSourceReader")
	}
	if !strings.Contains(err.Error(), "WithSourceReader") {
		t.Errorf("error should mention WithSourceReader, got: %v", err)
	}
}

func TestValidateExports_MissingSessionFactory(t *testing.T) {
	// RED: component.yaml declares session_factory: "Factory()", module doesn't export it.
	modDir := t.TempDir()
	writeGoFile(t, modDir, "package alpha\n\nfunc OtherFunc() {}\n")

	cm := &def.ComponentManifest{
		Module:         "github.com/test/alpha",
		SessionFactory: "Factory()",
	}

	err := ValidateExports(cm, modDir)
	if err == nil {
		t.Fatal("expected error for missing Factory")
	}
	if !strings.Contains(err.Error(), "Factory") {
		t.Errorf("error should mention Factory, got: %v", err)
	}
}

func TestValidateExports_MissingResolver(t *testing.T) {
	// RED: component.yaml declares resolver: SchematicResolver, module doesn't export it.
	modDir := t.TempDir()
	writeGoFile(t, modDir, "package beta\n\nfunc Factory() {}\n")

	cm := &def.ComponentManifest{
		Module:   "github.com/test/beta",
		Resolver: "SchematicResolver",
	}

	err := ValidateExports(cm, modDir)
	if err == nil {
		t.Fatal("expected error for missing SchematicResolver")
	}
	if !strings.Contains(err.Error(), "SchematicResolver") {
		t.Errorf("error should mention SchematicResolver, got: %v", err)
	}
}

func TestValidateExports_AllPresent(t *testing.T) {
	// GREEN path: all declared symbols exist.
	modDir := t.TempDir()
	writeGoFile(t, modDir, `package alpha

func Factory() {}
func SchematicResolver() {}
func WithSourceReader() {}
`)

	cm := &def.ComponentManifest{
		Module:         "github.com/test/alpha",
		SessionFactory: "Factory()",
		Resolver:       "SchematicResolver",
		Needs: struct {
			Origami    string          `yaml:"origami,omitempty"`
			Transports []def.SocketDef `yaml:"transports,omitempty"`
			Sources    []def.SocketDef `yaml:"sources,omitempty"`
			Storage    []def.SocketDef `yaml:"storage,omitempty"`
		}{
			Sources: []def.SocketDef{
				{Name: "source", Option: "WithSourceReader"},
			},
		},
	}

	if err := ValidateExports(cm, modDir); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateExports_OptionalSocketSkipped(t *testing.T) {
	// Optional sockets without option field are not checked.
	modDir := t.TempDir()
	writeGoFile(t, modDir, "package alpha\n")

	cm := &def.ComponentManifest{
		Module: "github.com/test/alpha",
		Needs: struct {
			Origami    string          `yaml:"origami,omitempty"`
			Transports []def.SocketDef `yaml:"transports,omitempty"`
			Sources    []def.SocketDef `yaml:"sources,omitempty"`
			Storage    []def.SocketDef `yaml:"storage,omitempty"`
		}{
			Sources: []def.SocketDef{
				{Name: "source", Optional: true}, // no option field → nothing to check
			},
		},
	}

	if err := ValidateExports(cm, modDir); err != nil {
		t.Fatalf("expected no error for optional socket without option, got: %v", err)
	}
}

func TestValidateExports_ComponentSkipped(t *testing.T) {
	// Components (kind: Component) are not schematics — no session_factory/resolver to check.
	modDir := t.TempDir()
	writeGoFile(t, modDir, "package rp\n")

	cm := &def.ComponentManifest{
		Kind:   "Component",
		Module: "github.com/test/rp",
	}

	if err := ValidateExports(cm, modDir); err != nil {
		t.Fatalf("expected no error for component, got: %v", err)
	}
}
