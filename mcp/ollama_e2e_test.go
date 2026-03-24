package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/mcp"
)

// --- log capture infrastructure ---

type logRecord struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
}

type logBuffer struct {
	mu      sync.Mutex
	records []logRecord
}

func (b *logBuffer) Records() []logRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]logRecord, len(b.records))
	copy(out, b.records)
	return out
}

func (b *logBuffer) HasMessage(msg string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.records {
		if r.Message == msg {
			return true
		}
	}
	return false
}

func (b *logBuffer) CountMessage(msg string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, r := range b.records {
		if r.Message == msg {
			count++
		}
	}
	return count
}

func (b *logBuffer) MessagesWithAttr(msg, key string, value any) []logRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []logRecord
	for _, r := range b.records {
		if r.Message != msg {
			continue
		}
		if v, ok := r.Attrs[key]; ok && fmt.Sprint(v) == fmt.Sprint(value) {
			out = append(out, r)
		}
	}
	return out
}

type captureHandler struct {
	buf   *logBuffer
	attrs []slog.Attr
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := logRecord{
		Level:   r.Level,
		Message: r.Message,
		Attrs:   make(map[string]any),
	}
	for _, a := range h.attrs {
		rec.Attrs[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value.Any()
		return true
	})
	h.buf.mu.Lock()
	h.buf.records = append(h.buf.records, rec)
	h.buf.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &captureHandler{buf: h.buf, attrs: newAttrs}
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	return h
}

func captureLogs(t *testing.T) *logBuffer {
	t.Helper()
	buf := &logBuffer{}
	handler := &captureHandler{buf: buf}
	logger := slog.New(handler)
	old := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(old) })
	return buf
}

// --- Ollama helpers ---

func requireOllama(t *testing.T) {
	t.Helper()
	if os.Getenv("ORIGAMI_WET_E2E") != "1" {
		t.Skip("ORIGAMI_WET_E2E=1 required")
	}
	if testing.Short() {
		t.Skip("-short flag set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:11434/api/tags", nil)
	if err != nil {
		t.Skipf("Ollama not available: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("Ollama not available: %v", err)
	}
	resp.Body.Close()
}

func ollamaModel() string {
	if m := os.Getenv("OLLAMA_MODEL"); m != "" {
		return m
	}
	return "llama3.2:3b"
}

func ollamaChat(ctx context.Context, model, prompt string) (string, error) {
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:11434/api/chat", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Message.Content), nil
}

// --- step schema definitions ---

var ollamaStepSchemas = []mcp.StepSchema{
	{Name: "ASK", Defs: []mcp.FieldDef{{Name: "response", Type: "string", Required: true}}},
	{Name: "STEP_A", Defs: []mcp.FieldDef{{Name: "response", Type: "string", Required: true}}},
	{Name: "STEP_B", Defs: []mcp.FieldDef{{Name: "response", Type: "string", Required: true}}},
	{Name: "STEP_C", Defs: []mcp.FieldDef{{Name: "response", Type: "string", Required: true}}},
	{Name: "classify", Defs: []mcp.FieldDef{{Name: "response", Type: "string", Required: true}}},
	{Name: "analyze", Defs: []mcp.FieldDef{{Name: "response", Type: "string", Required: true}}},
	{Name: "conclude", Defs: []mcp.FieldDef{{Name: "response", Type: "string", Required: true}}},
	{Name: "summarize", Defs: []mcp.FieldDef{{Name: "response", Type: "string", Required: true}}},
}

func findSchemas(names ...string) []mcp.StepSchema {
	var out []mcp.StepSchema
	for _, n := range names {
		for _, s := range ollamaStepSchemas {
			if s.Name == n {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// --- dispatchTransformer for delegation tests ---

type dispatchTransformer struct {
	disp *dispatch.MuxDispatcher
}

func (t *dispatchTransformer) Name() string { return "dispatch" }

func (t *dispatchTransformer) Transform(ctx context.Context, tc *engine.TransformerContext) (any, error) {
	dc := agentport.Context{
		CaseID:        "C01",
		Step:          tc.NodeName,
		PromptContent: tc.Prompt,
	}
	artifact, err := t.disp.Dispatch(ctx, dc)
	if err != nil {
		return nil, err
	}
	return string(artifact), nil
}

// --- client loop helper ---

// runOllamaLoop runs the standard MCP client loop: get_next_step -> Ollama -> submit_step -> repeat.
func runOllamaLoop(t *testing.T, ctx context.Context, sessionID, model string, callTool func(name string, args map[string]any) map[string]any, callToolErr func(name string, args map[string]any) (map[string]any, error)) int {
	t.Helper()
	steps := 0
	for {
		res := callTool("get_next_step", map[string]any{
			"session_id": sessionID,
			"timeout_ms": 30000,
		})
		if done, _ := res["done"].(bool); done {
			break
		}
		if avail, _ := res["available"].(bool); !avail {
			continue
		}

		prompt, _ := res["prompt_content"].(string)
		step, _ := res["step"].(string)
		dispatchID, _ := res["dispatch_id"].(float64)

		if prompt == "" {
			t.Fatalf("step %s: empty prompt_content", step)
		}

		t.Logf("step %s: sending prompt (%d chars) to Ollama", step, len(prompt))
		llmResp, err := ollamaChat(ctx, model, prompt)
		if err != nil {
			t.Fatalf("step %s: ollamaChat: %v", step, err)
		}
		t.Logf("step %s: Ollama response: %q", step, llmResp)

		callTool("submit_step", map[string]any{
			"session_id":  sessionID,
			"dispatch_id": int64(dispatchID),
			"step":        step,
			"fields":      map[string]any{"response": llmResp},
		})
		steps++
	}
	return steps
}

// ===================================================================
// Test 1: Simple Circuit + Happy-Path Logs
// ===================================================================

func TestOllamaE2E_SimpleCircuit(t *testing.T) {
	requireOllama(t)
	logs := captureLogs(t)

	cfg := mcp.CircuitConfig{
		Name:        "ollama-simple",
		Version:     "test",
		StepSchemas: findSchemas("ASK"),
		DefaultGetNextStepTimeout: 60000,
		DefaultSessionTTL:         120000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(ctx context.Context) (any, error) {
				dc := agentport.Context{
					CaseID:        "C01",
					Step:          "ASK",
					PromptContent: "What is 2+2? Reply with only the number.",
				}
				artifact, err := disp.Dispatch(ctx, dc)
				if err != nil {
					return nil, err
				}
				return map[string]any{"answer": string(artifact)}, nil
			}, mcp.SessionMeta{TotalCases: 1, Scenario: "simple-math"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return "done", result, nil
		},
	}

	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	model := ollamaModel()
	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sessionID := startResult["session_id"].(string)
	t.Logf("session: %s, model: %s", sessionID, model)

	ct := func(name string, args map[string]any) map[string]any {
		return callTool(t, ctx, session, name, args)
	}
	cte := func(name string, args map[string]any) (map[string]any, error) {
		return callToolE(ctx, session, name, args)
	}

	steps := runOllamaLoop(t, ctx, sessionID, model, ct, cte)
	if steps != 1 {
		t.Fatalf("expected 1 step, got %d", steps)
	}

	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if status, _ := report["status"].(string); status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}

	// Assert happy-path logs
	assertLog(t, logs, "circuit session started", "total_cases", int64(1))
	assertLogExists(t, logs, "step dispatched to worker")
	assertLogExists(t, logs, "dispatch round-trip")
	assertLogExists(t, logs, "step artifact accepted")
	assertLogExists(t, logs, "circuit complete")
	assertLogExists(t, logs, "report generated")

	dumpLogSummary(t, logs)
}

// ===================================================================
// Test 2: Multi-Step Cascade + Step-by-Step Logs
// ===================================================================

func TestOllamaE2E_MultiStepCascade(t *testing.T) {
	requireOllama(t)
	logs := captureLogs(t)

	prompts := map[string]string{
		"STEP_A": "List exactly 3 primary colors, one per line. Reply with only the colors.",
		"STEP_B": "Which primary color is most associated with the sky? Reply with only the color name.",
		"STEP_C": "Write one short sentence about the color blue.",
	}
	stepOrder := []string{"STEP_A", "STEP_B", "STEP_C"}

	cfg := mcp.CircuitConfig{
		Name:        "ollama-cascade",
		Version:     "test",
		StepSchemas: findSchemas("STEP_A", "STEP_B", "STEP_C"),
		DefaultGetNextStepTimeout: 60000,
		DefaultSessionTTL:         120000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(ctx context.Context) (any, error) {
				results := make(map[string]string)
				for _, step := range stepOrder {
					dc := agentport.Context{
						CaseID:        "C01",
						Step:          step,
						PromptContent: prompts[step],
					}
					artifact, err := disp.Dispatch(ctx, dc)
					if err != nil {
						return nil, err
					}
					results[step] = string(artifact)
				}
				return results, nil
			}, mcp.SessionMeta{TotalCases: 1, Scenario: "cascade-test"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return "cascade complete", result, nil
		},
	}

	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	model := ollamaModel()
	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sessionID := startResult["session_id"].(string)
	t.Logf("session: %s, model: %s", sessionID, model)

	ct := func(name string, args map[string]any) map[string]any {
		return callTool(t, ctx, session, name, args)
	}
	cte := func(name string, args map[string]any) (map[string]any, error) {
		return callToolE(ctx, session, name, args)
	}

	steps := runOllamaLoop(t, ctx, sessionID, model, ct, cte)
	if steps != 3 {
		t.Fatalf("expected 3 steps, got %d", steps)
	}

	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if status, _ := report["status"].(string); status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}

	// Assert step-by-step logs
	roundTrips := logs.CountMessage("dispatch round-trip")
	if roundTrips != 3 {
		t.Errorf("expected 3 'dispatch round-trip' logs, got %d", roundTrips)
	}

	dispatched := logs.CountMessage("step dispatched to worker")
	if dispatched != 3 {
		t.Errorf("expected 3 'step dispatched to worker' logs, got %d", dispatched)
	}

	accepted := logs.CountMessage("step artifact accepted")
	if accepted != 3 {
		t.Errorf("expected 3 'step artifact accepted' logs, got %d", accepted)
	}

	dumpLogSummary(t, logs)
}

// ===================================================================
// Test 3: Delegated Circuit + Delegation Logs
// ===================================================================

func TestOllamaE2E_DelegatedCircuit(t *testing.T) {
	requireOllama(t)
	logs := captureLogs(t)

	innerDef := &circuit.CircuitDef{
		Circuit: "analysis",
		Nodes: []circuit.NodeDef{
			{Name: "analyze", HandlerType: "transformer", Handler: "dispatch",
				Prompt: "Write one word that rhymes with 'cat'. Reply with only the word."},
			{Name: "conclude", HandlerType: "transformer", Handler: "dispatch",
				Prompt: "Name one fruit. Reply with only the fruit name."},
		},
		Edges: []circuit.EdgeDef{
			{ID: "analyze-conclude", From: "analyze", To: "conclude"},
			{ID: "conclude-done", From: "conclude", To: "_done"},
		},
		Start: "analyze",
		Done:  "_done",
	}

	outerDef := &circuit.CircuitDef{
		Circuit: "outer",
		Nodes: []circuit.NodeDef{
			{Name: "classify", HandlerType: "transformer", Handler: "dispatch",
				Prompt: "Name one primary color. Reply with only the color name."},
			{Name: "delegate_analysis", HandlerType: "circuit", Handler: "analysis"},
			{Name: "summarize", HandlerType: "transformer", Handler: "dispatch",
				Prompt: "Say 'done'. Reply with only the word."},
		},
		Edges: []circuit.EdgeDef{
			{ID: "classify-delegate", From: "classify", To: "delegate_analysis"},
			{ID: "delegate-summarize", From: "delegate_analysis", To: "summarize"},
			{ID: "summarize-done", From: "summarize", To: "_done"},
		},
		Start: "classify",
		Done:  "_done",
	}

	cfg := mcp.CircuitConfig{
		Name:        "ollama-delegated",
		Version:     "test",
		StepSchemas: findSchemas("classify", "analyze", "conclude", "summarize"),
		DefaultGetNextStepTimeout: 60000,
		DefaultSessionTTL:         180000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(ctx context.Context) (any, error) {
				dt := &dispatchTransformer{disp: disp}
				reg := engine.GraphRegistries{
					Transformers: engine.TransformerRegistry{"dispatch": dt},
					Circuits:     map[string]*circuit.CircuitDef{"analysis": innerDef},
				}
				g, err := engine.BuildGraph(outerDef, reg)
				if err != nil {
					return nil, fmt.Errorf("build graph: %w", err)
				}
				if dg, ok := g.(*engine.DefaultGraph); ok {
					dg.SetObserver(engine.NewLogObserver(nil))
				}
				walker := circuit.NewProcessWalker("e2e-delegate")
				if err := g.Walk(ctx, walker, outerDef.Start); err != nil {
					return nil, err
				}
				return walker.State().Outputs, nil
			}, mcp.SessionMeta{TotalCases: 1, Scenario: "delegation-test"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return "delegation complete", result, nil
		},
	}

	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	model := ollamaModel()
	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sessionID := startResult["session_id"].(string)
	t.Logf("session: %s, model: %s", sessionID, model)

	ct := func(name string, args map[string]any) map[string]any {
		return callTool(t, ctx, session, name, args)
	}
	cte := func(name string, args map[string]any) (map[string]any, error) {
		return callToolE(ctx, session, name, args)
	}

	steps := runOllamaLoop(t, ctx, sessionID, model, ct, cte)
	// 4 steps: classify + analyze + conclude + summarize
	if steps != 4 {
		t.Fatalf("expected 4 steps (2 outer + 2 inner), got %d", steps)
	}

	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if status, _ := report["status"].(string); status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}

	// Assert delegation logs
	roundTrips := logs.CountMessage("dispatch round-trip")
	if roundTrips != 4 {
		t.Errorf("expected 4 'dispatch round-trip' logs, got %d", roundTrips)
	}

	// Transformer execution logging
	transformerExec := logs.CountMessage("transformer executing")
	if transformerExec < 4 {
		t.Errorf("expected at least 4 'transformer executing' logs, got %d", transformerExec)
	}

	transformerDone := logs.CountMessage("transformer completed")
	if transformerDone < 4 {
		t.Errorf("expected at least 4 'transformer completed' logs, got %d", transformerDone)
	}

	// Walk observer events: delegate_start and delegate_end
	walkLogs := logs.CountMessage("walk")
	if walkLogs == 0 {
		t.Error("expected walk observer logs")
	}

	dumpLogSummary(t, logs)
}

// ===================================================================
// Test 4: Unhappy Paths
// ===================================================================

func TestOllamaE2E_UnhappyPaths(t *testing.T) {
	t.Run("MalformedResponse", func(t *testing.T) {
		if os.Getenv("ORIGAMI_WET_E2E") != "1" {
			t.Skip("ORIGAMI_WET_E2E=1 required")
		}
		logs := captureLogs(t)

		cfg := mcp.CircuitConfig{
			Name:    "ollama-unhappy",
			Version: "test",
			StepSchemas: []mcp.StepSchema{
				{Name: "VALIDATE", Defs: []mcp.FieldDef{
					{Name: "response", Type: "string", Required: true},
				}},
			},
			DefaultGetNextStepTimeout: 5000,
			DefaultSessionTTL:         30000,
			CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
				return func(ctx context.Context) (any, error) {
					dc := agentport.Context{
						CaseID:        "C01",
						Step:          "VALIDATE",
						PromptContent: "test",
					}
					_, err := disp.Dispatch(ctx, dc)
					return nil, err
				}, mcp.SessionMeta{TotalCases: 1, Scenario: "unhappy"}, nil
			},
			FormatReport: func(result any) (string, any, error) {
				return "done", result, nil
			},
		}

		srv := newTestServer(t, cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		session := connectInMemory(t, ctx, srv)
		defer session.Close()

		startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
		sessionID := startResult["session_id"].(string)

		res := callTool(t, ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
			"timeout_ms": 5000,
		})
		dispatchID, _ := res["dispatch_id"].(float64)

		// Submit with empty required field
		_, err := callToolE(ctx, session, "submit_step", map[string]any{
			"session_id":  sessionID,
			"dispatch_id": int64(dispatchID),
			"step":        "VALIDATE",
			"fields":      map[string]any{},
		})
		if err == nil {
			t.Fatal("expected error for empty required field")
		}
		if !strings.Contains(err.Error(), "missing required field") {
			t.Errorf("error should mention 'missing required field': %v", err)
		}

		if !logs.HasMessage("step schema validation failed") {
			t.Error("expected 'step schema validation failed' log")
		}
	})

	t.Run("UnknownStep", func(t *testing.T) {
		if os.Getenv("ORIGAMI_WET_E2E") != "1" {
			t.Skip("ORIGAMI_WET_E2E=1 required")
		}
		logs := captureLogs(t)

		cfg := mcp.CircuitConfig{
			Name:    "ollama-unknown-step",
			Version: "test",
			StepSchemas: []mcp.StepSchema{
				{Name: "KNOWN", Defs: []mcp.FieldDef{
					{Name: "response", Type: "string", Required: true},
				}},
			},
			DefaultGetNextStepTimeout: 5000,
			DefaultSessionTTL:         30000,
			CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
				return func(ctx context.Context) (any, error) {
					dc := agentport.Context{
						CaseID:        "C01",
						Step:          "KNOWN",
						PromptContent: "test",
					}
					_, err := disp.Dispatch(ctx, dc)
					return nil, err
				}, mcp.SessionMeta{TotalCases: 1, Scenario: "unhappy"}, nil
			},
			FormatReport: func(result any) (string, any, error) {
				return "done", result, nil
			},
		}

		srv := newTestServer(t, cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		session := connectInMemory(t, ctx, srv)
		defer session.Close()

		startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
		sessionID := startResult["session_id"].(string)

		res := callTool(t, ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
			"timeout_ms": 5000,
		})
		dispatchID, _ := res["dispatch_id"].(float64)

		_, err := callToolE(ctx, session, "submit_step", map[string]any{
			"session_id":  sessionID,
			"dispatch_id": int64(dispatchID),
			"step":        "NONEXISTENT",
			"fields":      map[string]any{"response": "test"},
		})
		if err == nil {
			t.Fatal("expected error for unknown step")
		}
		if !strings.Contains(err.Error(), "unknown step") {
			t.Errorf("error should mention 'unknown step': %v", err)
		}

		if !logs.HasMessage("step schema validation failed") {
			t.Error("expected 'step schema validation failed' log")
		}
	})

	t.Run("LLMTimeout", func(t *testing.T) {
		requireOllama(t)
		logs := captureLogs(t)

		cfg := mcp.CircuitConfig{
			Name:    "ollama-timeout",
			Version: "test",
			StepSchemas: []mcp.StepSchema{
				{Name: "SLOW", Defs: []mcp.FieldDef{
					{Name: "response", Type: "string", Required: true},
				}},
			},
			DefaultGetNextStepTimeout: 60000,
			DefaultSessionTTL:         60000,
			CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
				return func(ctx context.Context) (any, error) {
					dc := agentport.Context{
						CaseID:        "C01",
						Step:          "SLOW",
						PromptContent: "Write a very long essay about the history of mathematics.",
					}
					_, err := disp.Dispatch(ctx, dc)
					return nil, err
				}, mcp.SessionMeta{TotalCases: 1, Scenario: "timeout"}, nil
			},
			FormatReport: func(result any) (string, any, error) {
				return "done", result, nil
			},
		}

		srv := newTestServer(t, cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		session := connectInMemory(t, ctx, srv)
		defer session.Close()

		startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
		sessionID := startResult["session_id"].(string)

		res := callTool(t, ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
			"timeout_ms": 30000,
		})
		prompt, _ := res["prompt_content"].(string)
		dispatchID, _ := res["dispatch_id"].(float64)

		// Call Ollama with a very short timeout to simulate timeout
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer timeoutCancel()
		_, err := ollamaChat(timeoutCtx, ollamaModel(), prompt)
		if err == nil {
			t.Log("Ollama responded before timeout; timeout test is non-deterministic, submitting anyway")
			callTool(t, ctx, session, "submit_step", map[string]any{
				"session_id":  sessionID,
				"dispatch_id": int64(dispatchID),
				"step":        "SLOW",
				"fields":      map[string]any{"response": "timeout-fallback"},
			})
		} else {
			t.Logf("Ollama timed out as expected: %v", err)
			// Submit a fallback so the circuit can complete
			callTool(t, ctx, session, "submit_step", map[string]any{
				"session_id":  sessionID,
				"dispatch_id": int64(dispatchID),
				"step":        "SLOW",
				"fields":      map[string]any{"response": "timeout-fallback"},
			})
		}

		assertLogExists(t, logs, "step dispatched to worker")
		assertLogExists(t, logs, "step artifact accepted")
		_ = logs
	})
}

// --- log assertion helpers ---

func assertLogExists(t *testing.T, logs *logBuffer, msg string) {
	t.Helper()
	if !logs.HasMessage(msg) {
		t.Errorf("expected log message %q not found", msg)
	}
}

func assertLog(t *testing.T, logs *logBuffer, msg string, key string, value any) {
	t.Helper()
	matches := logs.MessagesWithAttr(msg, key, value)
	if len(matches) == 0 {
		t.Errorf("expected log message %q with %s=%v not found", msg, key, value)
	}
}

func dumpLogSummary(t *testing.T, logs *logBuffer) {
	t.Helper()
	records := logs.Records()
	counts := make(map[string]int)
	for _, r := range records {
		counts[r.Message]++
	}
	t.Log("--- log summary ---")
	for msg, count := range counts {
		t.Logf("  %s: %d", msg, count)
	}
	t.Logf("  total records: %d", len(records))
}
