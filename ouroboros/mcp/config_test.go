package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	fwmcp "github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/ouroboros"
	"github.com/dpopsuev/origami/ouroboros/mcp"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestServer(t *testing.T) *fwmcp.CircuitServer {
	t.Helper()
	runsDir := t.TempDir()
	cfg := mcp.NewOuroborosConfig(runsDir)
	srv := fwmcp.NewCircuitServer(cfg)
	mcp.RegisterExtraTools(srv, runsDir)
	t.Cleanup(srv.Shutdown)
	return srv
}

func newTestServerWithDir(t *testing.T, runsDir string) *fwmcp.CircuitServer {
	t.Helper()
	cfg := mcp.NewOuroborosConfig(runsDir)
	srv := fwmcp.NewCircuitServer(cfg)
	mcp.RegisterExtraTools(srv, runsDir)
	t.Cleanup(srv.Shutdown)
	return srv
}

func connectInMemory(t *testing.T, ctx context.Context, srv *fwmcp.CircuitServer) *sdkmcp.ClientSession {
	t.Helper()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := srv.MCPServer.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	return session
}

// ouroborosToolName maps legacy tool names to consolidated tool names.
func ouroborosToolName(legacy string) string {
	switch legacy {
	case "start_circuit", "get_next_step", "submit_step", "get_report":
		return "circuit"
	case "emit_signal", "get_signals", "get_worker_health":
		return "signal"
	case "get_trace", "get_run_report", "diff_runs":
		return "trace"
	default:
		return legacy
	}
}

// ouroborosToolArgs adds the action field for consolidated tools.
func ouroborosToolArgs(legacy string, args map[string]any) map[string]any {
	action := ""
	switch legacy {
	case "start_circuit":
		action = "start"
	case "get_next_step":
		action = "step"
	case "submit_step":
		action = "submit"
	case "get_report":
		action = "report"
	case "emit_signal":
		action = "emit"
	case "get_signals":
		action = "list"
	case "get_worker_health":
		action = "health"
	case "get_trace":
		action = "events"
	case "get_run_report":
		action = "report"
	case "diff_runs":
		action = "diff"
	default:
		return args
	}
	merged := map[string]any{"action": action}
	for k, v := range args {
		merged[k] = v
	}
	return merged
}

func callTool(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	actualName := ouroborosToolName(name)
	actualArgs := ouroborosToolArgs(name, args)
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      actualName,
		Arguments: actualArgs,
	})
	if err != nil {
		t.Fatalf("CallTool(%s/%s): %v", actualName, name, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				t.Fatalf("CallTool(%s/%s) returned error: %s", actualName, name, tc.Text)
			}
		}
		t.Fatalf("CallTool(%s/%s) returned error", actualName, name)
	}
	result := make(map[string]any)
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
				t.Fatalf("unmarshal tool result: %v (text: %s)", err, tc.Text)
			}
			return result
		}
	}
	t.Fatalf("no text content in tool result")
	return nil
}

func callToolExpectError(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	actualName := ouroborosToolName(name)
	actualArgs := ouroborosToolArgs(name, args)
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      actualName,
		Arguments: actualArgs,
	})
	if err != nil {
		return err.Error()
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				return tc.Text
			}
		}
		return "unknown error"
	}
	t.Fatal("expected error but got success")
	return ""
}

func startDiscovery(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, extra map[string]any) string {
	t.Helper()
	result := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
		"extra":    extra,
	})
	sessionID, ok := result["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatal("start_circuit did not return session_id")
	}
	return sessionID
}

func mockResponse(modelName, provider, version string) string {
	return `{"model_name": "` + modelName + `", "provider": "` + provider + `", "version": "` + version + `", "wrapper": "Cursor"}

` + "```go\n" + `func calculateSum(numbers []int, label string, verbose bool) (int, string, error) {
	total := 0
	var description string
	for _, num := range numbers {
		if num > 0 {
			total += num
			if verbose {
				description += fmt.Sprintf("%d,", num)
			}
		} else if num < 0 {
			total -= num
			if verbose {
				description += fmt.Sprintf("(%d),", num)
			}
		}
	}
	if total == 0 {
		return 0, "", fmt.Errorf("empty result for %s", label)
	}
	if label != "" {
		description = label + ": " + description
	}
	return total, description, nil
}` + "\n```"
}

func submitResponse(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, sessionID string, raw string) {
	t.Helper()
	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if step["done"] == true {
		t.Fatal("expected step available, got done=true")
	}

	stepName := step["step"].(string)
	dispatchID := step["dispatch_id"].(float64)

	callTool(t, ctx, session, "submit_step", map[string]any{
		"session_id":  sessionID,
		"dispatch_id": int64(dispatchID),
		"step":        stepName,
		"fields":      map[string]any{"response": raw},
	})
}

func waitDone(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, sessionID string) {
	t.Helper()
	for i := 0; i < 50; i++ {
		step := callTool(t, ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
			"timeout_ms": 200,
		})
		if step["done"] == true {
			return
		}
		if step["available"] == true {
			t.Fatal("unexpected available step after expected termination")
		}
	}
	t.Fatal("timed out waiting for done")
}

// --- Tool discovery ---

func TestOuroboros_ToolDiscovery(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	expected := map[string]bool{
		"circuit":           false,
		"signal":            false,
		"assemble_profiles": false,
	}

	for _, tool := range tools.Tools {
		if _, ok := expected[tool.Name]; ok {
			expected[tool.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

// --- Full discovery loop ---

func TestOuroboros_FullDiscoveryLoop(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	sessionID := startDiscovery(t, ctx, session, map[string]any{
		"max_iterations":      5.0,
		"terminate_on_repeat": true,
	})

	submitResponse(t, ctx, session, sessionID, mockResponse("claude-sonnet-4-20250514", "Anthropic", "20250514"))
	submitResponse(t, ctx, session, sessionID, mockResponse("gpt-4o", "OpenAI", "4o"))
	submitResponse(t, ctx, session, sessionID, mockResponse("claude-sonnet-4-20250514", "Anthropic", "20250514"))

	waitDone(t, ctx, session, sessionID)

	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if report["status"] != "done" {
		t.Fatalf("expected status=done, got %v", report["status"])
	}

	structured := report["structured"].(map[string]any)
	uniqueModels := structured["unique_models"].([]any)
	if len(uniqueModels) != 2 {
		t.Fatalf("expected 2 unique models, got %d", len(uniqueModels))
	}
}

// --- Max iterations ---

func TestOuroboros_MaxIterations(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	sessionID := startDiscovery(t, ctx, session, map[string]any{
		"max_iterations": 2.0,
	})

	submitResponse(t, ctx, session, sessionID, mockResponse("model-a", "ProviderA", "1.0"))
	submitResponse(t, ctx, session, sessionID, mockResponse("model-b", "ProviderB", "2.0"))

	waitDone(t, ctx, session, sessionID)

	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if report["status"] != "done" {
		t.Fatalf("expected status=done, got %v", report["status"])
	}
}

// --- Signal bus ---

func TestOuroboros_SignalBus(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	sessionID := startDiscovery(t, ctx, session, map[string]any{})

	callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "test_event",
		"agent":      "test",
		"meta":       map[string]any{"key": "value"},
	})

	signalsResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	total := signalsResult["total"].(float64)
	if total < 2 {
		t.Fatalf("expected at least 2 signals, got %v", total)
	}
}

// --- No session ---

func TestOuroboros_GetNextStep_NoSession(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	errMsg := callToolExpectError(t, ctx, session, "get_next_step", map[string]any{
		"session_id": "nonexistent",
	})
	if errMsg == "" {
		t.Fatal("expected error for nonexistent session")
	}
}

// --- Wrapper identity rejection ---

func TestOuroboros_WrapperRejection(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	sessionID := startDiscovery(t, ctx, session, map[string]any{
		"max_iterations":      3.0,
		"terminate_on_repeat": false,
	})

	submitResponse(t, ctx, session, sessionID, mockResponse("Auto", "unknown", ""))
	submitResponse(t, ctx, session, sessionID, mockResponse("gpt-4o", "OpenAI", "4o"))
	submitResponse(t, ctx, session, sessionID, mockResponse("Cursor", "Cursor", "1.0"))

	waitDone(t, ctx, session, sessionID)

	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})

	structured := report["structured"].(map[string]any)
	uniqueModels := structured["unique_models"].([]any)
	if len(uniqueModels) != 1 {
		t.Fatalf("expected 1 unique model (wrappers rejected), got %d", len(uniqueModels))
	}
}

// --- Report persistence ---

func TestOuroboros_ReportPersistence(t *testing.T) {
	runsDir := t.TempDir()
	srv := newTestServerWithDir(t, runsDir)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	sessionID := startDiscovery(t, ctx, session, map[string]any{
		"max_iterations": 1.0,
	})

	submitResponse(t, ctx, session, sessionID, mockResponse("model-a", "ProvA", "1.0"))
	waitDone(t, ctx, session, sessionID)

	callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})

	store, err := ouroboros.NewFileRunStore(runsDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	runs, err := store.ListRuns()
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("expected at least one persisted run")
	}
}

// --- Assemble profiles ---

func TestOuroboros_AssembleProfiles(t *testing.T) {
	runsDir := t.TempDir()
	store, err := ouroboros.NewFileRunStore(runsDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	gpt4o := circuit.ModelIdentity{ModelName: "gpt-4o", Provider: "OpenAI", Version: "2025-01"}
	run := ouroboros.RunReport{
		RunID:     "run-test",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Config:    ouroboros.DiscoveryConfig{ProbeID: "refactor-v1"},
		Results: []ouroboros.DiscoveryResult{
			{
				Model: gpt4o,
				Probe: ouroboros.ProbeResult{
					ProbeID:         "refactor-v1",
					DimensionScores: map[ouroboros.Dimension]float64{ouroboros.DimSpeed: 0.8},
				},
			},
		},
		UniqueModels: []circuit.ModelIdentity{gpt4o},
		TermReason:   "max_iterations_reached",
	}
	if err := store.SaveRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	srv := newTestServerWithDir(t, runsDir)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	result := callTool(t, ctx, session, "assemble_profiles", map[string]any{})
	if result["model_count"].(float64) != 1 {
		t.Fatalf("expected 1 model, got %v", result["model_count"])
	}
}

// --- Double start ---

func TestOuroboros_DoubleStart_Error(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	startDiscovery(t, ctx, session, map[string]any{})

	errMsg := callToolExpectError(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
		"extra":    map[string]any{},
	})
	if errMsg == "" {
		t.Fatal("expected error for double start")
	}
}

// --- Force start ---

func TestOuroboros_ForceStart(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)

	id1 := startDiscovery(t, ctx, session, map[string]any{})

	result := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
		"force":    true,
		"extra":    map[string]any{},
	})
	id2, ok := result["session_id"].(string)
	if !ok || id2 == "" {
		t.Fatal("force start did not return session_id")
	}
	if id1 == id2 {
		t.Error("force start should create a new session with different ID")
	}
}

// --- Probe registry (unchanged) ---

func TestProbeRegistry_AllFiveProbes(t *testing.T) {
	r := mcp.NewProbeRegistry()
	expected := []string{"refactor-v1", "debug-v1", "summarize-v1", "ambiguity-v1", "persistence-v1"}

	for _, id := range expected {
		h, err := r.Get(id)
		if err != nil {
			t.Errorf("Get(%q): %v", id, err)
			continue
		}
		if h.ID != id {
			t.Errorf("handler ID = %q, want %q", h.ID, id)
		}
		if h.BuildPrompt == nil || h.Score == nil {
			t.Errorf("handler %q has nil BuildPrompt or Score", id)
		}
		if prompt := h.Prompt(); prompt == "" {
			t.Errorf("handler %q returned empty prompt", id)
		}
	}
}
