package builders

import (
	"testing"

	"github.com/dpopsuev/tako/circuit/def"
)

func TestInstrumentManifestBuilder_Defaults(t *testing.T) {
	m := NewInstrumentManifest("test-scan").Build()
	if m.Name != "test-scan" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Kind != def.KindInstrument {
		t.Errorf("Kind = %q, want %q", m.Kind, def.KindInstrument)
	}
	if m.Dispatch != def.DispatchCLI {
		t.Errorf("Dispatch = %q, want exec", m.Dispatch)
	}
	if m.Tune == "" {
		t.Error("Tune should have default")
	}
	if len(m.Actions) == 0 {
		t.Error("Actions should have default")
	}
}

func TestInstrumentManifestBuilder_Fluent(t *testing.T) {
	m := NewInstrumentManifest("oculus").
		WithNamespace("instruments").
		WithDispatch(def.DispatchMCP).
		WithEndpoint("http://localhost:8080/mcp").
		WithTune("curl -sf http://localhost:8080/health").
		WithVersion("2.0").
		WithDescription("Oculus scan via MCP").
		WithAction("scan", def.ActionDef{
			Command:     "scan",
			InputSchema: `{"type":"object"}`,
		}).
		Build()

	if m.Dispatch != def.DispatchMCP {
		t.Errorf("Dispatch = %q", m.Dispatch)
	}
	if m.Endpoint != "http://localhost:8080/mcp" {
		t.Errorf("Endpoint = %q", m.Endpoint)
	}
	if _, ok := m.Actions["scan"]; !ok {
		t.Error("missing scan action")
	}
}

func TestInstrumentManifestBuilder_Docker(t *testing.T) {
	m := NewInstrumentManifest("vulncheck").
		WithDispatch(def.DispatchContainer).
		WithImage("osv-scanner:latest").
		WithTune("docker inspect osv-scanner:latest").
		WithAction("scan", def.ActionDef{Command: "osv-scanner scan"}).
		Build()

	if m.Dispatch != def.DispatchContainer {
		t.Errorf("Dispatch = %q", m.Dispatch)
	}
	if m.Image != "osv-scanner:latest" {
		t.Errorf("Image = %q", m.Image)
	}
}

func TestInstrumentManifestBuilder_GoDispatch(t *testing.T) {
	m := NewInstrumentManifest("core-transformers").
		WithDispatch(def.DispatchInproc).
		WithAction("llm", def.ActionDef{GoFunc: "transformers.LLMTransformer"}).
		WithAction("jq", def.ActionDef{GoFunc: "transformers.JQTransformer"}).
		Build()

	if m.Dispatch != def.DispatchInproc {
		t.Errorf("Dispatch = %q", m.Dispatch)
	}
	if len(m.Actions) < 2 {
		t.Errorf("Actions count = %d, want >= 2", len(m.Actions))
	}
	if m.Actions["llm"].GoFunc != "transformers.LLMTransformer" {
		t.Errorf("llm.GoFunc = %q", m.Actions["llm"].GoFunc)
	}
}
