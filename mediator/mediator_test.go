package mediator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/mediator"
)

func newTestBackend(t *testing.T, tools map[string]func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error)) *httptest.Server {
	t.Helper()
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "test-backend", Version: "v0.1.0"},
		nil,
	)
	for name, handler := range tools {
		server.AddTool(
			&sdkmcp.Tool{
				Name:        name,
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
			handler,
		)
	}
	h := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func echoHandler(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	var args struct{ Message string }
	json.Unmarshal(req.Params.Arguments, &args)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "echo: " + args.Message}},
	}, nil
}

func addHandler(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	var args struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	json.Unmarshal(req.Params.Arguments, &args)
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("%d", args.A+args.B)}},
	}, nil
}

func connectMediator(t *testing.T, ts *httptest.Server) *sdkmcp.ClientSession {
	t.Helper()
	ctx := t.Context()
	transport := &sdkmcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to mediator: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func extractText(t *testing.T, result *sdkmcp.CallToolResult) string {
	t.Helper()
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatal("no text content in result")
	return ""
}

func TestMediator_RoutesToCorrectBackend(t *testing.T) {
	backend1 := newTestBackend(t, map[string]func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error){
		"echo": echoHandler,
	})
	backend2 := newTestBackend(t, map[string]func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error){
		"add": addHandler,
	})

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "svc1", Endpoint: backend1.URL + "/mcp"},
		{Name: "svc2", Endpoint: backend2.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: mustJSON(map[string]any{"message": "hello"}),
	})
	if err != nil {
		t.Fatalf("CallTool echo: %v", err)
	}
	if got := extractText(t, result); got != "echo: hello" {
		t.Errorf("echo = %q, want %q", got, "echo: hello")
	}

	result, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "add",
		Arguments: mustJSON(map[string]any{"a": 3, "b": 7}),
	})
	if err != nil {
		t.Fatalf("CallTool add: %v", err)
	}
	if got := extractText(t, result); got != "10" {
		t.Errorf("add = %q, want %q", got, "10")
	}
}

func TestMediator_ListToolsAggregation(t *testing.T) {
	backend1 := newTestBackend(t, map[string]func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error){
		"tool_a": echoHandler,
		"tool_b": echoHandler,
	})
	backend2 := newTestBackend(t, map[string]func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error){
		"tool_c": echoHandler,
	})

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "svc1", Endpoint: backend1.URL + "/mcp"},
		{Name: "svc2", Endpoint: backend2.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	found := make(map[string]bool)
	for _, tool := range tools.Tools {
		found[tool.Name] = true
	}

	for _, name := range []string{"tool_a", "tool_b", "tool_c"} {
		if !found[name] {
			t.Errorf("tool %q not found in aggregated list", name)
		}
	}
}

func TestMediator_Healthz(t *testing.T) {
	backend := newTestBackend(t, map[string]func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error){
		"echo": echoHandler,
	})

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "svc1", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz = %d, want 200", resp.StatusCode)
	}
}

func TestMediator_Readyz_AllHealthy(t *testing.T) {
	backend := newTestBackend(t, map[string]func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error){
		"echo": echoHandler,
	})

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "svc1", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("readyz = %d, want 200", resp.StatusCode)
	}
}

func TestMediator_UnknownTool_ReturnsError(t *testing.T) {
	backend := newTestBackend(t, map[string]func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error){
		"echo": echoHandler,
	})

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "svc1", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	result, err := gw.CallTool(ctx, "nonexistent", nil)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError for unknown tool")
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// --- TSK-185: Routing signal emission ---

func TestMediator_EmitsRouteSignal(t *testing.T) {
	backend := newNamedCircuitBackend(t, "rca")

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "rca", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	// Start a circuit — should emit "route" and "session_start" signals.
	startRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustJSON(map[string]any{"action": "start", "force": true}),
	})
	if err != nil {
		t.Fatalf("circuit/start: %v", err)
	}
	if startRes.IsError {
		t.Fatalf("circuit/start error: %s", extractText(t, startRes))
	}

	// Check that the SignalBus has the expected signals.
	signals := gw.Bus.Since(0)
	if len(signals) < 2 {
		t.Fatalf("expected at least 2 signals (route + session_start), got %d", len(signals))
	}

	// First signal should be "route".
	if signals[0].Event != "route" {
		t.Errorf("signals[0].Event = %q, want %q", signals[0].Event, "route")
	}
	if signals[0].Agent != "mediator" {
		t.Errorf("signals[0].Agent = %q, want %q", signals[0].Agent, "mediator")
	}
	if signals[0].Meta["backend"] != "rca" {
		t.Errorf("signals[0].Meta[backend] = %q, want %q", signals[0].Meta["backend"], "rca")
	}

	// Second signal should be "session_start".
	if signals[1].Event != "session_start" {
		t.Errorf("signals[1].Event = %q, want %q", signals[1].Event, "session_start")
	}
	if signals[1].Meta["backend"] != "rca" {
		t.Errorf("signals[1].Meta[backend] = %q, want %q", signals[1].Meta["backend"], "rca")
	}
	if signals[1].Meta["session_id"] == "" {
		t.Error("signals[1].Meta[session_id] is empty, expected a session ID")
	}
}

func TestMediator_EmitsRouteSignalWithCircuitType(t *testing.T) {
	rcaBackend := newNamedCircuitBackend(t, "rca")
	gndBackend := newNamedCircuitBackend(t, "gnd")

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "rca", Endpoint: rcaBackend.URL + "/mcp"},
		{Name: "gnd", Endpoint: gndBackend.URL + "/mcp", CircuitType: "gnd"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	// Route to gnd backend via circuit_type.
	_, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "circuit",
		Arguments: mustJSON(map[string]any{
			"action": "start",
			"force":  true,
			"extra":  map[string]any{"circuit_type": "gnd"},
		}),
	})
	if err != nil {
		t.Fatalf("circuit/start: %v", err)
	}

	signals := gw.Bus.Since(0)
	if len(signals) < 2 {
		t.Fatalf("expected at least 2 signals, got %d", len(signals))
	}

	// Route signal should reference gnd backend and circuit_type.
	if signals[0].Meta["backend"] != "gnd" {
		t.Errorf("route signal backend = %q, want %q", signals[0].Meta["backend"], "gnd")
	}
	if signals[0].Meta["circuit_type"] != "gnd" {
		t.Errorf("route signal circuit_type = %q, want %q", signals[0].Meta["circuit_type"], "gnd")
	}
}

func TestMediator_NotifySessionDone(t *testing.T) {
	gw := mediator.New(nil)
	gw.NotifySessionDone("s-abc-1", "rca")

	signals := gw.Bus.Since(0)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].Event != "session_done" {
		t.Errorf("event = %q, want %q", signals[0].Event, "session_done")
	}
	if signals[0].Meta["session_id"] != "s-abc-1" {
		t.Errorf("session_id = %q, want %q", signals[0].Meta["session_id"], "s-abc-1")
	}
	if signals[0].Meta["backend"] != "rca" {
		t.Errorf("backend = %q, want %q", signals[0].Meta["backend"], "rca")
	}
}

func TestMediator_TraceRecorderWritesFile(t *testing.T) {
	backend := newNamedCircuitBackend(t, "traced")

	stateDir := t.TempDir()
	gw := mediator.New(
		[]mediator.BackendConfig{
			{Name: "traced", Endpoint: backend.URL + "/mcp"},
		},
		mediator.WithStateDir(stateDir),
	)
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	_, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustJSON(map[string]any{"action": "start", "force": true}),
	})
	if err != nil {
		t.Fatalf("circuit/start: %v", err)
	}

	gw.Stop(context.Background())

	// Verify trace file was created and contains the routing events.
	tracePath := stateDir + "/mediator-trace.jsonl"
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("trace file is empty")
	}

	content := string(data)
	if !strings.Contains(content, `"route"`) {
		t.Error("trace file missing 'route' event")
	}
	if !strings.Contains(content, `"session_start"`) {
		t.Error("trace file missing 'session_start' event")
	}
}

// --- TSK-187: Signals() accessor ---

func TestMediator_Signals_ReturnsBus(t *testing.T) {
	gw := mediator.New(nil)
	bus := gw.Signals()
	if bus == nil {
		t.Fatal("Signals() returned nil")
	}
	// Verify it's the same bus that receives emitted signals.
	gw.Bus.Emit("test_event", "test_agent", "", "", nil)
	signals := bus.Since(0)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal via Signals(), got %d", len(signals))
	}
	if signals[0].Event != "test_event" {
		t.Errorf("signal event = %q, want %q", signals[0].Event, "test_event")
	}
}

// --- Schema preservation through mediator proxy ---

type typedEchoInput struct {
	Message string `json:"message" jsonschema:"the message to echo"`
}

type typedAddInput struct {
	A int `json:"a" jsonschema:"first operand"`
	B int `json:"b" jsonschema:"second operand"`
}

func newTypedBackend(t *testing.T) *httptest.Server {
	t.Helper()
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "typed-backend", Version: "v0.1.0"},
		nil,
	)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "echo",
		Description: "Echoes a message",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, input typedEchoInput) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "echo: " + input.Message}},
		}, nil, nil
	})
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add",
		Description: "Adds two numbers",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, input typedAddInput) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("%d", input.A+input.B)}},
		}, nil, nil
	})
	h := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func TestMediator_PreservesSchemaFromBackend(t *testing.T) {
	backend := newTypedBackend(t)

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "svc1", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	for _, tool := range tools.Tools {
		if tool.Name != "echo" && tool.Name != "add" {
			continue
		}
		schema := tool.InputSchema
		if schema == nil {
			t.Errorf("tool %q: InputSchema is nil", tool.Name)
			continue
		}
		raw, _ := json.Marshal(schema)
		var parsed map[string]any
		json.Unmarshal(raw, &parsed)

		props, ok := parsed["properties"]
		if !ok {
			t.Errorf("tool %q: schema has no 'properties' key; got %s", tool.Name, raw)
			continue
		}
		propMap, _ := props.(map[string]any)
		switch tool.Name {
		case "echo":
			if _, ok := propMap["message"]; !ok {
				t.Errorf("echo schema missing 'message' property; got %s", raw)
			}
		case "add":
			if _, ok := propMap["a"]; !ok {
				t.Errorf("add schema missing 'a' property; got %s", raw)
			}
			if _, ok := propMap["b"]; !ok {
				t.Errorf("add schema missing 'b' property; got %s", raw)
			}
		}
	}
}

func TestMediator_PreservesDescriptionFromBackend(t *testing.T) {
	backend := newTypedBackend(t)

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "svc1", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	for _, tool := range tools.Tools {
		if tool.Name == "echo" && tool.Description == "" {
			t.Error("echo tool description not preserved through mediator")
		}
		if tool.Name == "add" && tool.Description == "" {
			t.Error("add tool description not preserved through mediator")
		}
	}
}

func TestMediator_TypedBackendParamsReachBackend(t *testing.T) {
	backend := newTypedBackend(t)

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "svc1", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: mustJSON(map[string]any{"message": "schema-test"}),
	})
	if err != nil {
		t.Fatalf("CallTool echo: %v", err)
	}
	if got := extractText(t, result); got != "echo: schema-test" {
		t.Errorf("echo = %q, want %q", got, "echo: schema-test")
	}

	result, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "add",
		Arguments: mustJSON(map[string]any{"a": 11, "b": 22}),
	})
	if err != nil {
		t.Fatalf("CallTool add: %v", err)
	}
	if got := extractText(t, result); got != "33" {
		t.Errorf("add = %q, want %q", got, "33")
	}
}

// --- Session-affinity routing (Origami Mediator) ---

// newNamedCircuitBackend creates a circuit backend that tags its report with the given label.
func newNamedCircuitBackend(t *testing.T, label string) *httptest.Server {
	t.Helper()
	cfg := mcp.CircuitConfig{
		Name:        label + "-circuit",
		Version:     "dev",
		StepSchemas: []mcp.StepSchema{
			{
				Name: "STEP",
				Defs: []mcp.FieldDef{{Name: "value", Type: "string", Required: true}},
			},
		},
		DefaultGetNextStepTimeout: 5000,
		DefaultSessionTTL:         300000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(ctx context.Context) (any, error) {
				if _, err := disp.Dispatch(ctx, dispatch.DispatchContext{
					CaseID: "C01", Step: "STEP",
				}); err != nil {
					return nil, err
				}
				return map[string]any{"backend": label}, nil
			}, mcp.SessionMeta{TotalCases: 1, Scenario: label + "-scenario"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return fmt.Sprintf("report from %s", label), result, nil
		},
	}

	srv := mcp.NewCircuitServer(cfg)
	t.Cleanup(srv.Shutdown)

	h := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return srv.MCPServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: false},
	)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func TestMediator_SessionAffinityRouting(t *testing.T) {
	rcaBackend := newNamedCircuitBackend(t, "rca")
	dsrBackend := newNamedCircuitBackend(t, "dsr")

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "rca", Endpoint: rcaBackend.URL + "/mcp"},
		{Name: "dsr", Endpoint: dsrBackend.URL + "/mcp", CircuitType: "gnd"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	// Start circuit on default backend (rca — no circuit_type).
	rcaStart, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustJSON(map[string]any{"action": "start", "force": true}),
	})
	if err != nil {
		t.Fatalf("circuit/start (rca): %v", err)
	}
	rcaText := extractText(t, rcaStart)
	var rcaOut map[string]any
	json.Unmarshal([]byte(rcaText), &rcaOut)
	rcaSessionID, _ := rcaOut["session_id"].(string)
	if rcaSessionID == "" {
		t.Fatalf("no session_id from rca start: %s", rcaText)
	}

	// Small delay to ensure different session IDs (timestamp-based).
	time.Sleep(2 * time.Millisecond)

	// Start circuit on dsr backend (circuit_type=gnd).
	dsrStart, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "circuit",
		Arguments: mustJSON(map[string]any{
			"action": "start",
			"force":  true,
			"extra":  map[string]any{"circuit_type": "gnd"},
		}),
	})
	if err != nil {
		t.Fatalf("circuit/start (dsr): %v", err)
	}
	dsrText := extractText(t, dsrStart)
	var dsrOut map[string]any
	json.Unmarshal([]byte(dsrText), &dsrOut)
	dsrSessionID, _ := dsrOut["session_id"].(string)
	if dsrSessionID == "" {
		t.Fatalf("no session_id from dsr start: %s", dsrText)
	}

	if rcaSessionID == dsrSessionID {
		t.Fatal("rca and dsr should have different session IDs")
	}

	// Drive both sessions to completion.
	drainSession := func(sessionID, label string) {
		t.Helper()
		for i := 0; i < 10; i++ {
			res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
				Name:      "circuit",
				Arguments: mustJSON(map[string]any{"action": "step", "session_id": sessionID, "timeout_ms": 3000}),
			})
			if err != nil {
				t.Fatalf("circuit/step (%s): %v", label, err)
			}
			var out map[string]any
			json.Unmarshal([]byte(extractText(t, res)), &out)
			if done, _ := out["done"].(bool); done {
				return
			}
			if avail, _ := out["available"].(bool); !avail {
				continue
			}
			dispatchID := int64(out["dispatch_id"].(float64))
			step, _ := out["step"].(string)
			_, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
				Name: "circuit",
				Arguments: mustJSON(map[string]any{
					"action": "submit",
					"session_id": sessionID, "dispatch_id": dispatchID,
					"step": step, "fields": map[string]any{"value": "test"},
				}),
			})
			if err != nil {
				t.Fatalf("circuit/submit (%s): %v", label, err)
			}
		}
	}

	drainSession(rcaSessionID, "rca")
	drainSession(dsrSessionID, "dsr")

	// Verify reports come from correct backends.
	rcaReport, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustJSON(map[string]any{"action": "report", "session_id": rcaSessionID}),
	})
	if err != nil {
		t.Fatalf("circuit/report (rca): %v", err)
	}
	var rcaReportOut map[string]any
	json.Unmarshal([]byte(extractText(t, rcaReport)), &rcaReportOut)
	if structured, ok := rcaReportOut["structured"].(map[string]any); ok {
		if structured["backend"] != "rca" {
			t.Errorf("rca report backend = %v, want rca", structured["backend"])
		}
	} else {
		t.Errorf("rca report missing structured field: %s", extractText(t, rcaReport))
	}

	dsrReport, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustJSON(map[string]any{"action": "report", "session_id": dsrSessionID}),
	})
	if err != nil {
		t.Fatalf("circuit/report (dsr): %v", err)
	}
	var dsrReportOut map[string]any
	json.Unmarshal([]byte(extractText(t, dsrReport)), &dsrReportOut)
	if structured, ok := dsrReportOut["structured"].(map[string]any); ok {
		if structured["backend"] != "dsr" {
			t.Errorf("dsr report backend = %v, want dsr", structured["backend"])
		}
	} else {
		t.Errorf("dsr report missing structured field: %s", extractText(t, dsrReport))
	}
}

func TestMediator_SingleBackend_NoPapercupCollision(t *testing.T) {
	backend := newNamedCircuitBackend(t, "only")

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "svc", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	// Verify Papercup tools are listed exactly once.
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	counts := make(map[string]int)
	for _, tool := range tools.Tools {
		counts[tool.Name]++
	}
	for name := range mediator.PapercupTools {
		if counts[name] != 1 {
			t.Errorf("tool %q listed %d times, want 1", name, counts[name])
		}
	}
}

// --- Gap 2: MCP routing gap — tools reachable via HTTP ---

func newCircuitBackend(t *testing.T) (*httptest.Server, *mcp.CircuitServer) {
	t.Helper()
	cfg := mcp.CircuitConfig{
		Name:        "test-circuit",
		Version:     "dev",
		StepSchemas: []mcp.StepSchema{
			{
				Name: "STEP_A",
				Defs: []mcp.FieldDef{
					{Name: "value", Type: "string", Required: true},
					{Name: "score", Type: "float", Required: true},
				},
			},
		},
		DefaultGetNextStepTimeout: 5000,
		DefaultSessionTTL:         300000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (mcp.RunFunc, mcp.SessionMeta, error) {
			nCases := 2
			return func(ctx context.Context) (any, error) {
				for i := 0; i < nCases; i++ {
					caseID := fmt.Sprintf("C%02d", i+1)
					if _, err := disp.Dispatch(ctx, dispatch.DispatchContext{
						CaseID: caseID, Step: "STEP_A",
					}); err != nil {
						return nil, err
					}
				}
				return map[string]any{"cases": nCases}, nil
			}, mcp.SessionMeta{TotalCases: nCases, Scenario: "http-test"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return "Processed 2 cases", result, nil
		},
	}

	srv := mcp.NewCircuitServer(cfg)
	t.Cleanup(srv.Shutdown)

	h := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return srv.MCPServer },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts, srv
}

func TestMediator_MCPToolsReachableViaHTTP(t *testing.T) {
	backend, _ := newCircuitBackend(t)

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "rca", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	wantTools := map[string]bool{
		"circuit": false,
		"signal":  false,
	}
	for _, tool := range tools.Tools {
		if _, ok := wantTools[tool.Name]; ok {
			wantTools[tool.Name] = true
		}
	}
	for name, found := range wantTools {
		if !found {
			t.Errorf("circuit tool %q not found via HTTP mediator", name)
		}
	}
}

func TestWorker_CanCallToolsViaHTTPTransport(t *testing.T) {
	backend, _ := newCircuitBackend(t)

	gw := mediator.New([]mediator.BackendConfig{
		{Name: "rca", Endpoint: backend.URL + "/mcp"},
	})
	ctx := t.Context()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer gw.Stop(context.Background())

	ts := httptest.NewServer(gw.Handler())
	defer ts.Close()

	session := connectMediator(t, ts)

	// Start circuit
	startRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustJSON(map[string]any{"action": "start", "parallel": 1}),
	})
	if err != nil {
		t.Fatalf("circuit/start: %v", err)
	}
	var startOut map[string]any
	for _, c := range startRes.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			json.Unmarshal([]byte(tc.Text), &startOut)
		}
	}
	sessionID, _ := startOut["session_id"].(string)
	if sessionID == "" {
		t.Fatal("no session_id in circuit/start response")
	}

	// Worker loop via HTTP
	stepsProcessed := 0
	for {
		res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      "circuit",
			Arguments: mustJSON(map[string]any{"action": "step", "session_id": sessionID, "timeout_ms": 5000}),
		})
		if err != nil {
			t.Fatalf("circuit/step: %v", err)
		}
		var out map[string]any
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				json.Unmarshal([]byte(tc.Text), &out)
			}
		}

		if done, _ := out["done"].(bool); done {
			break
		}
		if avail, _ := out["available"].(bool); !avail {
			continue
		}

		dispatchID := int64(out["dispatch_id"].(float64))
		step, _ := out["step"].(string)

		_, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: mustJSON(map[string]any{
				"action":      "submit",
				"session_id":  sessionID,
				"dispatch_id": dispatchID,
				"step":        step,
				"fields":      map[string]any{"value": "http-worker", "score": 0.95},
			}),
		})
		if err != nil {
			t.Fatalf("circuit/submit: %v", err)
		}
		stepsProcessed++
	}

	if stepsProcessed != 2 {
		t.Fatalf("processed %d steps via HTTP, want 2", stepsProcessed)
	}
}
