package fold

import (
	"strings"
	"testing"
)

func TestLoadInstruments_Valid(t *testing.T) {
	instruments := map[string]string{
		"dummy-echo": "testkit/instruments/dummy-echo/instrument.yaml",
	}
	// Paths are relative to repo root.
	loaded, err := LoadInstruments(instruments, "..")
	if err != nil {
		t.Fatalf("LoadInstruments: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d, want 1", len(loaded))
	}
	if loaded[0].Name != "dummy-echo" {
		t.Errorf("name = %q, want dummy-echo", loaded[0].Name)
	}
	if loaded[0].Manifest.Tune == "" {
		t.Error("manifest.Tune is empty")
	}
}

func TestLoadInstruments_MissingFile(t *testing.T) {
	instruments := map[string]string{
		"nonexistent": "does/not/exist/instrument.yaml",
	}
	_, err := LoadInstruments(instruments, "..")
	if err == nil {
		t.Fatal("expected error for missing instrument file")
	}
}

func TestLoadInstruments_NameMismatch(t *testing.T) {
	instruments := map[string]string{
		"wrong-name": "testkit/instruments/dummy-echo/instrument.yaml",
	}
	_, err := LoadInstruments(instruments, "..")
	if err == nil {
		t.Fatal("expected error for name mismatch")
	}
}

func TestLoadInstruments_Empty(t *testing.T) {
	loaded, err := LoadInstruments(nil, "..")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected nil, got %v", loaded)
	}
}

func TestRenderInstrumentSetup_WithInstruments(t *testing.T) {
	m := &Manifest{
		LoadedInstruments: []LoadedInstrument{
			{Name: "oculus", Path: "instruments/oculus/instrument.yaml"},
		},
	}
	code := renderInstrumentSetup(m)
	if code == "" {
		t.Fatal("expected non-empty instrument setup code")
	}
	if !strings.Contains(code, "fwdef.LoadInstrumentManifest") {
		t.Error("missing LoadInstrumentManifest call")
	}
	if !strings.Contains(code, "fwengine.TuneAll") {
		t.Error("missing TuneAll call")
	}
	if !strings.Contains(code, `instruments["oculus"]`) {
		t.Error("missing instrument registry assignment")
	}
}

func TestRenderInstrumentSetup_NoInstruments(t *testing.T) {
	m := &Manifest{}
	code := renderInstrumentSetup(m)
	if code != "" {
		t.Errorf("expected empty, got %q", code)
	}
}

func TestBoardManifest_WithInstruments(t *testing.T) {
	yaml := `kind: Board
name: test-board
instruments:
  dummy-echo: testkit/instruments/dummy-echo/instrument.yaml
`
	bm, err := ParseBoardManifest([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseBoardManifest: %v", err)
	}
	if len(bm.Instruments) != 1 {
		t.Fatalf("instruments = %d, want 1", len(bm.Instruments))
	}
	if bm.Instruments["dummy-echo"] != "testkit/instruments/dummy-echo/instrument.yaml" {
		t.Errorf("path = %q", bm.Instruments["dummy-echo"])
	}
}

func TestGenerateWiredBinary_WithInstruments(t *testing.T) {
	m := &Manifest{
		Name:    "test",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port:   9300,
			Assets: &AssetMap{},
		},
		LoadedInstruments: []LoadedInstrument{
			{Name: "oculus", Path: "instruments/oculus/instrument.yaml"},
		},
	}
	g := &ResolvedGraph{
		Root: ResolvedSchematic{
			Name:           "test",
			Alias:          "test",
			SessionFactory: "Factory",
		},
	}

	src, err := GenerateWiredBinary(m, g)
	if err != nil {
		t.Fatalf("GenerateWiredBinary: %v", err)
	}
	code := string(src)

	if !strings.Contains(code, "fwdef") {
		t.Error("missing fwdef import for instrument loading")
	}
	if !strings.Contains(code, "fwengine") {
		t.Error("missing fwengine import for TuneAll")
	}
	if !strings.Contains(code, "TuneAll") {
		t.Error("missing TuneAll call in generated code")
	}
	if !strings.Contains(code, "bridgedCfg.Manifests = instruments") {
		t.Error("missing instrument registry wiring")
	}
}
