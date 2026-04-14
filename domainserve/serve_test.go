package domainserve_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/domainserve"
)

func setup(t *testing.T, fsys fstest.MapFS) *sdkmcp.ClientSession {
	t.Helper()
	handler := domainserve.New(fsys, domainserve.Config{
		Name:    "test-domain",
		Version: "v0.1.0",
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func callTool(t *testing.T, session *sdkmcp.ClientSession, name string, args map[string]any) *sdkmcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return result
}

func resultText(result *sdkmcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestDomainInfo(t *testing.T) {
	fs := fstest.MapFS{
		"circuits/alpha.yaml": &fstest.MapFile{
			Data: []byte("circuit: alpha\ntopology: cascade\ndescription: Primary analysis\n"),
		},
	}

	session := setup(t, fs)
	result := callTool(t, session, "domain_info", nil)

	if result.IsError {
		t.Fatalf("domain_info error: %s", resultText(result))
	}

	var info domainserve.DomainInfo
	if err := json.Unmarshal([]byte(resultText(result)), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if info.Name != "test-domain" {
		t.Errorf("name = %q, want test-domain", info.Name)
	}
	if info.Version != "v0.1.0" {
		t.Errorf("version = %q, want v0.1.0", info.Version)
	}
	if len(info.Circuits) != 1 {
		t.Fatalf("circuits = %d, want 1", len(info.Circuits))
	}
	if info.Circuits[0].Name != "alpha" {
		t.Errorf("circuit name = %q, want alpha", info.Circuits[0].Name)
	}
	if info.Circuits[0].Topology != "cascade" {
		t.Errorf("topology = %q, want cascade", info.Circuits[0].Topology)
	}
	if info.Circuits[0].Description != "Primary analysis" {
		t.Errorf("description = %q, want Primary analysis", info.Circuits[0].Description)
	}
}

func TestDomainInfo_MultipleCircuits(t *testing.T) {
	fs := fstest.MapFS{
		"circuits/alpha.yaml": &fstest.MapFile{
			Data: []byte("circuit: alpha\ntopology: cascade\ndescription: Alpha\n"),
		},
		"circuits/trend.yaml": &fstest.MapFile{
			Data: []byte("circuit: trend\ntopology: cascade\ndescription: Trend analysis\n"),
		},
	}

	session := setup(t, fs)
	result := callTool(t, session, "domain_info", nil)

	var info domainserve.DomainInfo
	if err := json.Unmarshal([]byte(resultText(result)), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(info.Circuits) != 2 {
		t.Fatalf("circuits = %d, want 2", len(info.Circuits))
	}
}

func TestDomainInfo_NoCircuits(t *testing.T) {
	fs := fstest.MapFS{
		"prompts/hello.md": &fstest.MapFile{Data: []byte("hello")},
	}

	session := setup(t, fs)
	result := callTool(t, session, "domain_info", nil)

	var info domainserve.DomainInfo
	if err := json.Unmarshal([]byte(resultText(result)), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if info.Circuits != nil {
		t.Errorf("circuits should be nil when circuits/ missing, got %v", info.Circuits)
	}
}

func TestDomainRead(t *testing.T) {
	fs := fstest.MapFS{
		"prompts/recall.md": &fstest.MapFile{Data: []byte("You are a recall judge.")},
	}

	session := setup(t, fs)
	result := callTool(t, session, "domain_read", map[string]any{"path": "prompts/recall.md"})

	if result.IsError {
		t.Fatalf("domain_read error: %s", resultText(result))
	}
	if resultText(result) != "You are a recall judge." {
		t.Errorf("content = %q, want %q", resultText(result), "You are a recall judge.")
	}
}

func TestDomainRead_Missing(t *testing.T) {
	session := setup(t, fstest.MapFS{})
	result := callTool(t, session, "domain_read", map[string]any{"path": "nope.txt"})

	if !result.IsError {
		t.Fatal("expected error for missing file")
	}
}

func TestDomainRead_PathTraversal(t *testing.T) {
	session := setup(t, fstest.MapFS{})

	for _, path := range []string{"../etc/passwd", "../../go.mod", "/absolute/path"} {
		result := callTool(t, session, "domain_read", map[string]any{"path": path})
		if !result.IsError {
			t.Errorf("expected error for path %q", path)
		}
	}
}

func TestDomainList(t *testing.T) {
	fs := fstest.MapFS{
		"prompts/recall.md": &fstest.MapFile{Data: []byte("a")},
		"prompts/triage.md": &fstest.MapFile{Data: []byte("b")},
	}

	session := setup(t, fs)
	result := callTool(t, session, "domain_list", map[string]any{"path": "prompts"})

	if result.IsError {
		t.Fatalf("domain_list error: %s", resultText(result))
	}

	var entries []domainserve.DirEntry
	if err := json.Unmarshal([]byte(resultText(result)), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
}

func TestDomainList_Root(t *testing.T) {
	fs := fstest.MapFS{
		"circuits/alpha.yaml": &fstest.MapFile{Data: []byte("a")},
		"prompts/x.md":        &fstest.MapFile{Data: []byte("b")},
		"vocabulary.yaml":     &fstest.MapFile{Data: []byte("c")},
	}

	session := setup(t, fs)
	result := callTool(t, session, "domain_list", map[string]any{"path": "."})

	if result.IsError {
		t.Fatalf("domain_list error: %s", resultText(result))
	}

	var entries []domainserve.DirEntry
	if err := json.Unmarshal([]byte(resultText(result)), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hasDir := false
	hasFile := false
	for _, e := range entries {
		if e.Name == "circuits" && e.IsDir {
			hasDir = true
		}
		if e.Name == "vocabulary.yaml" && !e.IsDir {
			hasFile = true
		}
	}
	if !hasDir {
		t.Error("missing circuits/ directory in root listing")
	}
	if !hasFile {
		t.Error("missing vocabulary.yaml in root listing")
	}
}

func TestDomainList_PathTraversal(t *testing.T) {
	session := setup(t, fstest.MapFS{})
	result := callTool(t, session, "domain_list", map[string]any{"path": "../etc"})
	if !result.IsError {
		t.Error("expected error for traversal path")
	}
}

func TestHealth(t *testing.T) {
	handler := domainserve.New(fstest.MapFS{}, domainserve.Config{
		Name: "test", Version: "v0",
	})

	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s returned %d, want 200", path, w.Code)
		}
	}
}

func setupWithAssets(t *testing.T, fsys fstest.MapFS, assets *domainserve.AssetIndex) *sdkmcp.ClientSession {
	t.Helper()
	handler := domainserve.New(fsys, domainserve.Config{
		Name:    "test-domain",
		Version: "v0.1.0",
		Assets:  assets,
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func TestDomainResolve_Section(t *testing.T) {
	fsys := fstest.MapFS{
		"prompts/recall/judge-similarity.md": &fstest.MapFile{Data: []byte("You are a recall judge.")},
	}
	assets := &domainserve.AssetIndex{
		Sections: map[string]map[string]string{
			"prompts": {"recall": "prompts/recall/judge-similarity.md"},
		},
	}

	session := setupWithAssets(t, fsys, assets)
	result := callTool(t, session, "domain_resolve", map[string]any{
		"section": "prompts",
		"key":     "recall",
	})

	if result.IsError {
		t.Fatalf("domain_resolve error: %s", resultText(result))
	}
	if got := resultText(result); got != "You are a recall judge." {
		t.Errorf("content = %q, want %q", got, "You are a recall judge.")
	}
}

func TestDomainResolve_File(t *testing.T) {
	fsys := fstest.MapFS{
		"vocabulary.yaml": &fstest.MapFile{Data: []byte("defects:\n  pb001: product bug\n")},
	}
	assets := &domainserve.AssetIndex{
		Files: map[string]string{"vocabulary": "vocabulary.yaml"},
	}

	session := setupWithAssets(t, fsys, assets)
	result := callTool(t, session, "domain_resolve", map[string]any{
		"section": "vocabulary",
	})

	if result.IsError {
		t.Fatalf("domain_resolve error: %s", resultText(result))
	}
	if got := resultText(result); got != "defects:\n  pb001: product bug\n" {
		t.Errorf("content = %q", got)
	}
}

func TestDomainResolve_MissingSection(t *testing.T) {
	assets := &domainserve.AssetIndex{
		Sections: map[string]map[string]string{},
	}
	session := setupWithAssets(t, fstest.MapFS{}, assets)
	result := callTool(t, session, "domain_resolve", map[string]any{
		"section": "nonexistent",
		"key":     "foo",
	})
	if !result.IsError {
		t.Fatal("expected error for missing section")
	}
}

func TestDomainResolve_MissingKey(t *testing.T) {
	assets := &domainserve.AssetIndex{
		Sections: map[string]map[string]string{
			"prompts": {"recall": "prompts/recall.md"},
		},
	}
	session := setupWithAssets(t, fstest.MapFS{}, assets)
	result := callTool(t, session, "domain_resolve", map[string]any{
		"section": "prompts",
		"key":     "nonexistent",
	})
	if !result.IsError {
		t.Fatal("expected error for missing key")
	}
}

// setupFS creates a domain-serve from any fs.FS (not just fstest.MapFS).
func setupFS(t *testing.T, fsys fs.FS, cfg domainserve.Config) *sdkmcp.ClientSession {
	t.Helper()
	handler := domainserve.New(fsys, cfg)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	transport := &sdkmcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(t.Context(), transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// TestEmbedVsMounted_FunctionalEquivalence proves that domainserve.New
// produces identical MCP responses whether backed by an in-memory FS
// (simulating embed.FS) or an os.DirFS (simulating --data-dir).
func TestEmbedVsMounted_FunctionalEquivalence(t *testing.T) {
	// --- shared test data ---
	files := map[string]string{
		"circuits/alpha.yaml":     "circuit: alpha\ntopology: cascade\ndescription: Primary analysis\n",
		"prompts/recall/judge.md": "You are a recall judge.",
		"vocabulary.yaml":         "defects:\n  pb001: product bug\n",
		"scenarios/ptp.yaml":      "scenario: ptp\ncases: [C01]\n",
	}
	assets := &domainserve.AssetIndex{
		Sections: map[string]map[string]string{
			"circuits": {"alpha": "circuits/alpha.yaml"},
			"prompts":  {"recall": "prompts/recall/judge.md"},
		},
		Files: map[string]string{
			"vocabulary": "vocabulary.yaml",
		},
	}
	cfg := domainserve.Config{
		Name:    "test-domain",
		Version: "v0.1.0",
		Assets:  assets,
	}

	// --- build in-memory FS (simulates embed.FS) ---
	memFS := fstest.MapFS{}
	for path, content := range files {
		memFS[path] = &fstest.MapFile{Data: []byte(content)}
	}

	// --- build os.DirFS (simulates --data-dir) ---
	diskDir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(diskDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	diskFS := os.DirFS(diskDir)

	embedSession := setupFS(t, memFS, cfg)
	mountSession := setupFS(t, diskFS, cfg)

	// --- tool calls to compare ---
	type toolCall struct {
		name string
		args map[string]any
	}
	calls := []toolCall{
		{"domain_info", nil},
		{"domain_read", map[string]any{"path": "circuits/alpha.yaml"}},
		{"domain_read", map[string]any{"path": "prompts/recall/judge.md"}},
		{"domain_read", map[string]any{"path": "vocabulary.yaml"}},
		{"domain_read", map[string]any{"path": "scenarios/ptp.yaml"}},
		{"domain_list", map[string]any{"path": "."}},
		{"domain_list", map[string]any{"path": "circuits"}},
		{"domain_list", map[string]any{"path": "prompts/recall"}},
		{"domain_resolve", map[string]any{"section": "circuits", "key": "alpha"}},
		{"domain_resolve", map[string]any{"section": "prompts", "key": "recall"}},
		{"domain_resolve", map[string]any{"section": "vocabulary"}},
	}

	for _, tc := range calls {
		t.Run(tc.name+"_"+argsKey(tc.args), func(t *testing.T) {
			embedResult := callTool(t, embedSession, tc.name, tc.args)
			mountResult := callTool(t, mountSession, tc.name, tc.args)

			embedText := resultText(embedResult)
			mountText := resultText(mountResult)

			if embedResult.IsError != mountResult.IsError {
				t.Fatalf("error mismatch: embed=%v mount=%v\nembed: %s\nmount: %s",
					embedResult.IsError, mountResult.IsError, embedText, mountText)
			}

			if embedText != mountText {
				t.Errorf("response mismatch:\nembed: %s\nmount: %s", embedText, mountText)
			}
		})
	}
}

func argsKey(args map[string]any) string {
	if args == nil {
		return "nil"
	}
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ",")
}

func TestDomainInfo_WithAssets(t *testing.T) {
	fsys := fstest.MapFS{
		"circuits/alpha.yaml": &fstest.MapFile{
			Data: []byte("topology: cascade\ndescription: Alpha circuit\n"),
		},
	}
	assets := &domainserve.AssetIndex{
		Sections: map[string]map[string]string{
			"circuits": {"alpha": "circuits/alpha.yaml"},
		},
	}

	session := setupWithAssets(t, fsys, assets)
	result := callTool(t, session, "domain_info", nil)

	var info domainserve.DomainInfo
	if err := json.Unmarshal([]byte(resultText(result)), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(info.Circuits) != 1 {
		t.Fatalf("circuits = %d, want 1", len(info.Circuits))
	}
	if info.Circuits[0].Name != "alpha" {
		t.Errorf("circuit name = %q, want alpha", info.Circuits[0].Name)
	}
	if info.Circuits[0].Topology != "cascade" {
		t.Errorf("topology = %q, want cascade", info.Circuits[0].Topology)
	}
}
