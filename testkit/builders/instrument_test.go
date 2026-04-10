package builders

import (
	"testing"

	"github.com/dpopsuev/origami/circuit/def"
)

func TestInstrumentManifestBuilder_Defaults(t *testing.T) {
	m := NewInstrumentManifest("test-scan").Build()
	if m.Name != "test-scan" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Kind != def.KindInstrument {
		t.Errorf("Kind = %q, want %q", m.Kind, def.KindInstrument)
	}
	if m.Dispatch != def.DispatchExec {
		t.Errorf("Dispatch = %q, want exec", m.Dispatch)
	}
	if m.Tune == "" {
		t.Error("Tune should have default")
	}
	if m.Command == "" {
		t.Error("Command should have default")
	}
}

func TestInstrumentManifestBuilder_Fluent(t *testing.T) {
	m := NewInstrumentManifest("oculus-scan").
		WithNamespace("instruments").
		WithDispatch(def.DispatchMCP).
		WithEndpoint("http://localhost:8080/mcp").
		WithTune("curl -sf http://localhost:8080/health").
		WithVersion("2.0").
		WithDescription("Oculus scan via MCP").
		WithInputSchema(`{"type":"object"}`).
		WithOutputSchema(`{"type":"object"}`).
		Build()

	if m.Dispatch != def.DispatchMCP {
		t.Errorf("Dispatch = %q", m.Dispatch)
	}
	if m.Endpoint != "http://localhost:8080/mcp" {
		t.Errorf("Endpoint = %q", m.Endpoint)
	}
	if m.Version != "2.0" {
		t.Errorf("Version = %q", m.Version)
	}
}

func TestInstrumentManifestBuilder_Docker(t *testing.T) {
	m := NewInstrumentManifest("vulncheck").
		WithDispatch(def.DispatchDocker).
		WithImage("osv-scanner:latest").
		WithTune("docker inspect osv-scanner:latest").
		Build()

	if m.Dispatch != def.DispatchDocker {
		t.Errorf("Dispatch = %q", m.Dispatch)
	}
	if m.Image != "osv-scanner:latest" {
		t.Errorf("Image = %q", m.Image)
	}
}
