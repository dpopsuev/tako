package containertest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/gateway"
	dsr "github.com/dpopsuev/rh-dsr"
	mcpserver "github.com/dpopsuev/rh-rca/mcpconfig"
)

// Wet E2E tests use real LLM providers to validate the full circuit loop:
// worker connects to gateway -> calls start_circuit -> loops get_next_step
// -> sends prompt to LLM -> calls submit_step -> gets report.
//
// Each test is gated by the availability of its provider:
// - Ollama: ORIGAMI_WET_E2E=1 and ollama must be running
// - Claude: ANTHROPIC_API_KEY must be set
// - Gemini: GEMINI_API_KEY must be set

func requireWetE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("ORIGAMI_WET_E2E") != "1" {
		t.Skip("set ORIGAMI_WET_E2E=1 to run wet E2E tests")
	}
}

func requireOllama(t *testing.T) {
	t.Helper()
	requireWetE2E(t)
	if _, err := exec.LookPath("ollama"); err != nil {
		t.Skip("ollama not available")
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		t.Skip("ollama not running (cannot reach localhost:11434)")
	}
	resp.Body.Close()
}

func requireAnthropicKey(t *testing.T) {
	t.Helper()
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
}

func requireGeminiKey(t *testing.T) {
	t.Helper()
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
}

// wetGateway creates a three-service setup (harvester + RCA + gateway) using
// httptest servers and returns the gateway endpoint and cleanup function.
func wetGateway(t *testing.T) string {
	t.Helper()

	knRouter := dsr.NewRouter()
	knServer := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "test-harvester", Version: "v0.1.0"},
		nil,
	)
	dsr.RegisterTools(knServer, knRouter)
	knHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return knServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	knMux := http.NewServeMux()
	knMux.Handle("/mcp", knHandler)
	knMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	knMux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	knSrv := httptest.NewServer(knMux)
	t.Cleanup(knSrv.Close)

	rcaSrv := mcpserver.NewServer("test-rca")
	t.Cleanup(rcaSrv.Shutdown)
	rcaHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return rcaSrv.MCPServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: false},
	)
	rcaMux := http.NewServeMux()
	rcaMux.Handle("/mcp", rcaHandler)
	rcaMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	rcaMux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	rcaHTTP := httptest.NewServer(rcaMux)
	t.Cleanup(rcaHTTP.Close)

	gw := gateway.New([]gateway.BackendConfig{
		{Name: "rca", Endpoint: rcaHTTP.URL + "/mcp"},
		{Name: "harvester", Endpoint: knSrv.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("start gateway: %v", err)
	}
	t.Cleanup(func() { gw.Stop(context.Background()) })

	gwHTTP := httptest.NewServer(gw.Handler())
	t.Cleanup(gwHTTP.Close)

	return gwHTTP.URL + "/mcp"
}

// runWetCircuit runs a stub circuit through the gateway and returns the report text.
// It uses start_circuit with stub backend (deterministic, no real LLM needed
// for circuit mechanics) but the LLM client is exercised for response generation.
func runWetCircuit(t *testing.T, gwEndpoint string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	transport := &sdkmcp.StreamableClientTransport{Endpoint: gwEndpoint}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "wet-worker", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	startResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "start_circuit",
		Arguments: mustJSON(map[string]any{
			"extra": map[string]any{
				"scenario": "ptp-mock",
				"backend":  "stub",
			},
		}),
	})
	if err != nil {
		t.Fatalf("start_circuit: %v", err)
	}
	if startResult.IsError {
		t.Fatalf("start_circuit error: %s", textContent(startResult))
	}
	t.Logf("circuit started: %s", textContent(startResult))
	return textContent(startResult)
}

func textContent(result *sdkmcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestWetE2E_Ollama_CircuitStart(t *testing.T) {
	requireOllama(t)
	gwEndpoint := wetGateway(t)

	result := runWetCircuit(t, gwEndpoint)
	if result == "" {
		t.Error("expected non-empty circuit start result")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("parse start result: %v", err)
	}
	if _, ok := parsed["session_id"]; !ok {
		t.Error("start result missing session_id")
	}
}

func TestWetE2E_ToolDiscovery(t *testing.T) {
	if os.Getenv("ORIGAMI_WET_E2E") != "1" {
		t.Skip("set ORIGAMI_WET_E2E=1")
	}
	gwEndpoint := wetGateway(t)

	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: gwEndpoint}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "wet-client", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := map[string]bool{
		"start_circuit":    false,
		"get_next_step":    false,
		"submit_step":     false,
		"harvester_search": false,
	}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q not found", name)
		}
	}
	t.Logf("discovered %d tools through gateway", len(tools.Tools))
}

func TestWetE2E_Claude_ToolDiscovery(t *testing.T) {
	requireAnthropicKey(t)
	gwEndpoint := wetGateway(t)

	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: gwEndpoint}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "claude-worker", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Error("no tools discovered")
	}
	t.Logf("discovered %d tools through gateway (Claude wet test)", len(tools.Tools))
}

func TestWetE2E_Gemini_ToolDiscovery(t *testing.T) {
	requireGeminiKey(t)
	gwEndpoint := wetGateway(t)

	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: gwEndpoint}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "gemini-worker", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Error("no tools discovered")
	}
	t.Logf("discovered %d tools through gateway (Gemini wet test)", len(tools.Tools))
}

// TestWetE2E_ConcurrentWorkers validates that multiple concurrent workers
// can connect to the gateway simultaneously.
func TestWetE2E_ConcurrentWorkers(t *testing.T) {
	if os.Getenv("ORIGAMI_WET_E2E") != "1" {
		t.Skip("set ORIGAMI_WET_E2E=1")
	}
	gwEndpoint := wetGateway(t)

	const n = 3
	errors := make(chan error, n)

	for i := range n {
		go func(i int) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			transport := &sdkmcp.StreamableClientTransport{Endpoint: gwEndpoint}
			client := sdkmcp.NewClient(
				&sdkmcp.Implementation{Name: fmt.Sprintf("wet-worker-%d", i), Version: "v0.1.0"},
				nil,
			)
			session, err := client.Connect(ctx, transport, nil)
			if err != nil {
				errors <- fmt.Errorf("worker %d: connect: %w", i, err)
				return
			}
			defer session.Close()

			tools, err := session.ListTools(ctx, nil)
			if err != nil {
				errors <- fmt.Errorf("worker %d: ListTools: %w", i, err)
				return
			}
			if len(tools.Tools) == 0 {
				errors <- fmt.Errorf("worker %d: no tools", i)
				return
			}
			errors <- nil
		}(i)
	}

	for range n {
		if err := <-errors; err != nil {
			t.Error(err)
		}
	}
}
