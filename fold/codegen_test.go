package fold

import (
	"strings"
	"testing"
)

func TestGenerateDomainServe_Assets(t *testing.T) {
	m := &Manifest{
		Name:    "asterisk",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{
				Circuits: map[string]string{
					"rca":         "circuits/rca.yaml",
					"calibration": "circuits/calibration.yaml",
				},
				Prompts: map[string]string{
					"recall": "prompts/recall/judge-similarity.md",
				},
				Files: map[string]string{
					"vocabulary": "vocabulary.yaml",
				},
			},
		},
	}

	src, err := GenerateDomainServe(m)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	for _, want := range []string{
		"//go:embed circuits/calibration.yaml",
		"//go:embed circuits/rca.yaml",
		"//go:embed prompts/recall/judge-similarity.md",
		"//go:embed vocabulary.yaml",
		"var domainData embed.FS",
		"AssetIndex",
		`"circuits"`,
		`"rca"`,
		`"vocabulary"`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in generated code:\n%s", want, code)
		}
	}

	if strings.Contains(code, "//go:embed internal") {
		t.Errorf("assets mode should not use directory embed")
	}
}

func TestGenerateDomainServe_DefaultPort(t *testing.T) {
	m := &Manifest{
		Name:    "myapp",
		Version: "2.0",
		DomainServe: &DomainServeConfig{
			Assets: &AssetMap{
				Files: map[string]string{"data": "data.yaml"},
			},
		},
	}

	src, err := GenerateDomainServe(m)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	if !strings.Contains(code, "9300") {
		t.Errorf("expected default port 9300 in:\n%s", code)
	}
}

func TestGenerateDomainServe_NilConfig(t *testing.T) {
	m := &Manifest{Name: "test"}
	_, err := GenerateDomainServe(m)
	if err == nil {
		t.Fatal("expected error for nil domain_serve config")
	}
	if !strings.Contains(err.Error(), "domain_serve") {
		t.Errorf("error should mention domain_serve, got: %v", err)
	}
}

func TestGenerateDomainServe_NoAssets(t *testing.T) {
	m := &Manifest{
		Name:        "test",
		DomainServe: &DomainServeConfig{Port: 9300},
	}
	_, err := GenerateDomainServe(m)
	if err == nil {
		t.Fatal("expected error for missing assets")
	}
	if !strings.Contains(err.Error(), "assets is required") {
		t.Errorf("error = %q, want mention of assets", err.Error())
	}
}

func TestGenerateDomainServe_DataDirFlag(t *testing.T) {
	m := &Manifest{
		Name:    "asterisk",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{
				Circuits: map[string]string{"rca": "circuits/rca.yaml"},
			},
		},
	}

	src, err := GenerateDomainServe(m)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	for _, want := range []string{
		`"flag"`,
		`"io/fs"`,
		`flag.String("data-dir"`,
		`var domainFS fs.FS = domainData`,
		`os.DirFS(*dataDir)`,
		`domainserve.New(domainFS,`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in generated code:\n%s", want, code)
		}
	}
}

func TestGenerateDomainServe_HealthzFlag(t *testing.T) {
	m := &Manifest{
		Name:    "asterisk",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{
				Circuits: map[string]string{"rca": "circuits/rca.yaml"},
			},
		},
	}

	src, err := GenerateDomainServe(m)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	for _, want := range []string{
		`flag.Bool("healthz"`,
		`if *healthz {`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in generated code:\n%s", want, code)
		}
	}

	// Old-style --healthz parsing (os.Args loop) should be gone.
	if strings.Contains(code, `os.Args[1:]`) {
		t.Errorf("should use flag-based --healthz, not os.Args loop:\n%s", code)
	}
}

func TestGenerateWiredBinary_DataDirFlag(t *testing.T) {
	root := origamiRoot(t)
	m := &Manifest{
		Name:    "asterisk",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{
				Circuits: map[string]string{"rca": "circuits/rca.yaml"},
				Files:    map[string]string{"vocabulary": "vocabulary.yaml"},
			},
		},
		Schematics: map[string]SchematicRef{
			"rca": {
				Path:     "github.com/dpopsuev/origami-rca",
				Bindings: map[string]string{"source": "reportportal"},
			},
		},
		Connectors: map[string]ConnectorRef{
			"reportportal": {Path: "github.com/dpopsuev/origami-rca/connectors/rp"},
		},
	}

	g, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatal(err)
	}

	src, err := GenerateWiredBinary(m, g)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	for _, want := range []string{
		`"flag"`,
		`"io/fs"`,
		`flag.String("data-dir"`,
		`var domainFS fs.FS = domainData`,
		`os.DirFS(*dataDir)`,
		`domainserve.New(domainFS,`,
		`bridgedCfg.DomainFS = domainFS`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in generated code:\n%s", want, code)
		}
	}
}

func TestGenerateWiredBinary(t *testing.T) {
	root := origamiRoot(t)

	m := &Manifest{
		Name:    "asterisk",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port: 9300,
			Assets: &AssetMap{
				Circuits: map[string]string{"rca": "circuits/rca.yaml"},
				Files:    map[string]string{"vocabulary": "vocabulary.yaml"},
			},
		},
		Schematics: map[string]SchematicRef{
			"rca": {
				Path: "github.com/dpopsuev/origami-rca",
				Bindings: map[string]string{
					"source": "reportportal",
				},
			},
		},
		Connectors: map[string]ConnectorRef{
			"reportportal": {Path: "github.com/dpopsuev/origami-rca/connectors/rp"},
		},
	}

	g, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatal(err)
	}

	src, err := GenerateWiredBinary(m, g)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	for _, want := range []string{
		"DO NOT EDIT",
		"package main",
		`"github.com/dpopsuev/origami-rca"`,
		"origamirca.Hooks()",
		"fwmcp.SessionFactoryToConfig(factory)",
		"bridgedCfg.DomainFS = domainFS",
		"domainserve.New(domainFS",
		"NewStreamableHTTPHandler",
		"fwmcp.NewCircuitServer(bridgedCfg)",
		"/mcp",
		"/domain/",
		"/healthz",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in generated code:\n%s", want, code)
		}
	}

	if t.Failed() {
		t.Logf("Full generated code:\n%s", code)
	}
}
