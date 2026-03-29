package domainfs_test

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/domainfs"
	"github.com/dpopsuev/origami/domainserve"
)

// sessionCaller adapts *sdkmcp.ClientSession to subprocess.ToolCaller.
type sessionCaller struct {
	session *sdkmcp.ClientSession
}

func (s *sessionCaller) CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	return s.session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
}

//nolint:gocyclo // integration test exercises multiple fs operations sequentially
func TestIntegration_RoundTrip(t *testing.T) {
	sourceFS := fstest.MapFS{
		"circuits/rca.yaml": &fstest.MapFile{
			Data: []byte("circuit: rca\ntopology: cascade\ndescription: Root-cause analysis\n"),
		},
		"prompts/recall/judge-similarity.md": &fstest.MapFile{
			Data: []byte("You are a recall judge. Compare the failure with known symptoms."),
		},
		"prompts/triage/classify-symptoms.md": &fstest.MapFile{
			Data: []byte("Classify the symptom category."),
		},
		"heuristics.yaml": &fstest.MapFile{
			Data: []byte("rules: []"),
		},
		"vocabulary.yaml": &fstest.MapFile{
			Data: []byte("defect_types: {}"),
		},
	}

	handler := domainserve.New(sourceFS, domainserve.Config{
		Name:    "asterisk",
		Version: "v0.1.0",
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-engine", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	rfs := domainfs.New(&sessionCaller{session: session})

	// Read a file
	f, err := rfs.Open("prompts/recall/judge-similarity.md")
	if err != nil {
		t.Fatalf("Open file: %v", err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := "You are a recall judge. Compare the failure with known symptoms."
	if string(data) != want {
		t.Errorf("file content = %q, want %q", string(data), want)
	}

	// Read a directory
	d, err := rfs.Open("prompts")
	if err != nil {
		t.Fatalf("Open dir: %v", err)
	}
	dirFile, ok := d.(fs.ReadDirFile)
	if !ok {
		t.Fatal("expected ReadDirFile for directory")
	}
	entries, err := dirFile.ReadDir(-1)
	d.Close()
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("dir entries = %d, want 2 (recall, triage)", len(entries))
	}

	// Missing file
	_, err = rfs.Open("nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}

	// List root
	root, err := rfs.Open(".")
	if err != nil {
		t.Fatalf("Open root: %v", err)
	}
	rootDir := root.(fs.ReadDirFile)
	rootEntries, err := rootDir.ReadDir(-1)
	root.Close()
	if err != nil {
		t.Fatalf("ReadDir root: %v", err)
	}
	if len(rootEntries) < 3 {
		t.Errorf("root entries = %d, want >= 3 (circuits, prompts, heuristics.yaml, vocabulary.yaml)", len(rootEntries))
	}

	// Verify domain_info via raw MCP call
	infoResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "domain_info",
	})
	if err != nil {
		t.Fatalf("domain_info: %v", err)
	}
	var info domainserve.DomainInfo
	for _, c := range infoResult.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			json.Unmarshal([]byte(tc.Text), &info)
		}
	}
	if info.Name != "asterisk" {
		t.Errorf("info.Name = %q, want asterisk", info.Name)
	}
	if len(info.Circuits) != 1 || info.Circuits[0].Topology != "cascade" {
		t.Errorf("circuits = %+v, want 1 cascade circuit", info.Circuits)
	}

	// Health probes
	for _, path := range []string{"/healthz", "/readyz"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("%s = %d, want 200", path, resp.StatusCode)
		}
	}
}
