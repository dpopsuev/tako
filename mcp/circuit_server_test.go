package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/mcp"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))
	os.Exit(m.Run())
}

// testStepSchemas defines a simple 3-step circuit for generic tests.
var testStepSchemas = []mcp.StepSchema{
	{
		Name: "STEP_A",
		Defs: []mcp.FieldDef{
			{Name: "value", Type: "string", Required: true},
			{Name: "score", Type: "float", Required: true},
		},
	},
	{
		Name: "STEP_B",
		Defs: []mcp.FieldDef{
			{Name: "result", Type: "bool", Required: true},
		},
	},
	{
		Name: "STEP_C",
		Defs: []mcp.FieldDef{
			{Name: "summary", Type: "string", Required: true},
		},
	},
}

// testReport is the domain result type for test circuits.
type testReport struct {
	CasesProcessed int
	StepsProcessed int
}

// stubRunFunc creates a RunFunc that dispatches nCases in parallel (up to
// the MuxDispatcher's capacity), each with nSteps sequential steps. This
// mirrors how real circuit runners operate: cases run concurrently but
// steps within a case are sequential.
func stubRunFunc(disp *dispatch.MuxDispatcher, nCases, nSteps, parallel int, steps []string, promptDir string) mcp.RunFunc {
	return func(ctx context.Context) (any, error) {
		sem := make(chan struct{}, parallel)
		var mu sync.Mutex
		total := 0
		errCh := make(chan error, nCases)

		var wg sync.WaitGroup
		for c := 0; c < nCases; c++ {
			wg.Add(1)
			go func(caseIdx int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				caseID := fmt.Sprintf("C%02d", caseIdx+1)
				for s := 0; s < nSteps; s++ {
					step := steps[s%len(steps)]
					promptPath := ""
					if promptDir != "" {
						promptPath = fmt.Sprintf("%s/%s_%s.md", promptDir, caseID, step)
					}
					dc := dispatch.DispatchContext{
						CaseID:       caseID,
						Step:         step,
						PromptPath:   promptPath,
						ArtifactPath: fmt.Sprintf("/tmp/test_%s_%s.json", caseID, step),
					}
					if _, err := disp.Dispatch(ctx, dc); err != nil {
						errCh <- err
						return
					}
					mu.Lock()
					total++
					mu.Unlock()
				}
			}(c)
		}
		wg.Wait()
		close(errCh)
		if err := <-errCh; err != nil {
			return nil, err
		}

		mu.Lock()
		t := total
		mu.Unlock()
		return &testReport{CasesProcessed: nCases, StepsProcessed: t}, nil
	}
}

// stubRunFuncInstant creates a RunFunc that completes instantly (like a stub backend).
func stubRunFuncInstant(nCases int) mcp.RunFunc {
	return func(ctx context.Context) (any, error) {
		return &testReport{CasesProcessed: nCases, StepsProcessed: 0}, nil
	}
}

// newTestConfig creates a CircuitConfig for testing.
func newTestConfig(nCases, nSteps int, promptDir string) mcp.CircuitConfig {
	steps := []string{"STEP_A", "STEP_B", "STEP_C"}
	return mcp.CircuitConfig{
		Name:        "test-circuit",
		Version:     "dev",
		StepSchemas: testStepSchemas,
		WorkerPreamble: "You are a test circuit worker.",
		DefaultGetNextStepTimeout: 1000,
		DefaultSessionTTL:         300000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (mcp.RunFunc, mcp.SessionMeta, error) {
			parallel := params.Parallel
			if parallel < 1 {
				parallel = 1
			}
			return stubRunFunc(disp, nCases, nSteps, parallel, steps, promptDir),
				mcp.SessionMeta{TotalCases: nCases, Scenario: "test-scenario"},
				nil
		},
		FormatReport: func(result any) (string, any, error) {
			r, ok := result.(*testReport)
			if !ok {
				return "", nil, fmt.Errorf("unexpected result type")
			}
			return fmt.Sprintf("Processed %d cases, %d steps", r.CasesProcessed, r.StepsProcessed), r, nil
		},
	}
}

func newTestConfigStub(nCases int) mcp.CircuitConfig {
	return mcp.CircuitConfig{
		Name:        "test-circuit",
		Version:     "dev",
		StepSchemas: testStepSchemas,
		DefaultGetNextStepTimeout: 1000,
		DefaultSessionTTL:         300000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return stubRunFuncInstant(nCases),
				mcp.SessionMeta{TotalCases: nCases, Scenario: "test-stub"},
				nil
		},
		FormatReport: func(result any) (string, any, error) {
			r, ok := result.(*testReport)
			if !ok {
				return "", nil, fmt.Errorf("unexpected result type")
			}
			return fmt.Sprintf("Stub: %d cases", r.CasesProcessed), r, nil
		},
	}
}

func newTestServer(t *testing.T, cfg mcp.CircuitConfig) *mcp.CircuitServer {
	t.Helper()
	srv := mcp.NewCircuitServer(cfg)
	t.Cleanup(srv.Shutdown)
	return srv
}

func connectInMemory(t *testing.T, ctx context.Context, srv *mcp.CircuitServer) *sdkmcp.ClientSession {
	t.Helper()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	serverSession, err := srv.MCPServer.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	return session
}

func callTool(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	actualName := toolName(name)
	actualArgs := toolArgs(name, args)
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

func callToolE(ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) (map[string]any, error) {
	actualName := toolName(name)
	actualArgs := toolArgs(name, args)
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      actualName,
		Arguments: actualArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("CallTool(%s/%s): %w", actualName, name, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				return nil, fmt.Errorf("CallTool(%s/%s) error: %s", actualName, name, tc.Text)
			}
		}
		return nil, fmt.Errorf("CallTool(%s/%s) returned error", actualName, name)
	}
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			result := make(map[string]any)
			if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
				return nil, fmt.Errorf("unmarshal %s result: %w", name, err)
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("no text content in %s result", name)
}

// toolName and toolArgs map legacy tool names to consolidated tool names with action.
// This reduces test churn during the 11→3 tool consolidation.
func toolName(legacy string) string {
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

func toolArgs(legacy string, args map[string]any) map[string]any {
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

func containsCI(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !containsCI(s, sub) {
			return false
		}
	}
	return true
}

// --- Tests ---

func TestCircuitServer_ToolDiscovery(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(3))
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := map[string]bool{
		"circuit": false,
		"signal":  false,
	}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q not found in ListTools", name)
		}
	}
}

func TestCircuitServer_StubFullLoop(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(3))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sessionID, ok := startResult["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("expected non-empty session_id, got %v", startResult["session_id"])
	}
	totalCases, _ := startResult["total_cases"].(float64)
	if int(totalCases) != 3 {
		t.Fatalf("expected total_cases=3, got %v", totalCases)
	}

	time.Sleep(300 * time.Millisecond)
	stepResult := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	done, _ := stepResult["done"].(bool)
	if !done {
		t.Fatalf("expected done=true for stub RunFunc, got %v", stepResult)
	}

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
	report, _ := reportResult["report"].(string)
	if report == "" {
		t.Fatal("expected non-empty report string")
	}
	t.Logf("report: %s", report)
}

func TestCircuitServer_GetNextStep_NoSession(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(1))
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: map[string]any{"action": "step", "session_id": "nonexistent"},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for missing session")
	}
}

func TestCircuitServer_DoubleStart_WhileRunning(t *testing.T) {
	cfg := newTestConfig(3, 3, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	callTool(t, ctx, session, "start_circuit", map[string]any{})

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: map[string]any{"action": "start"},
	})
	if err != nil {
		t.Fatalf("expected tool error, got transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for double start while running")
	}
}

func TestCircuitServer_ForceStart_ReplacesRunning(t *testing.T) {
	cfg := newTestConfig(3, 3, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start1 := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sid1 := start1["session_id"].(string)

	start2 := callTool(t, ctx, session, "start_circuit", map[string]any{"force": true})
	sid2 := start2["session_id"].(string)

	if sid2 == sid1 {
		t.Fatal("force-started session should have a different ID")
	}
	t.Logf("force-started session %s replaced %s", sid2, sid1)
}

// TestStartCircuit_WorkerPrompt verifies parallel>1 returns worker_prompt.
func TestStartCircuit_WorkerPrompt(t *testing.T) {
	cfg := newTestConfig(3, 3, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	workerPrompt, _ := startResult["worker_prompt"].(string)
	if workerPrompt == "" {
		t.Fatal("expected non-empty worker_prompt for parallel>1")
	}
	if !containsAll(workerPrompt, sessionID, `action="step"`, `action="submit"`,
		"worker_started", "worker_stopped", "mode", "stream") {
		t.Errorf("worker_prompt missing required protocol keywords")
	}

	workerCount, _ := startResult["worker_count"].(float64)
	if int(workerCount) != 4 {
		t.Errorf("expected worker_count=4, got %v", workerCount)
	}

	if !containsCI(workerPrompt, "STEP_A") || !containsCI(workerPrompt, "STEP_B") || !containsCI(workerPrompt, "STEP_C") {
		t.Error("worker_prompt missing step schema names")
	}
	t.Logf("worker_prompt length: %d chars", len(workerPrompt))
}

// TestStartCircuit_WorkerPrompt_Serial verifies parallel=1 omits worker_prompt.
func TestStartCircuit_WorkerPrompt_Serial(t *testing.T) {
	cfg := newTestConfig(3, 3, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})

	workerPrompt, _ := startResult["worker_prompt"].(string)
	if workerPrompt != "" {
		t.Errorf("expected empty worker_prompt for parallel=1, got %d chars", len(workerPrompt))
	}
}

// TestCapacityWarning_ProtocolAgnostic verifies capacity warning text.
func TestCapacityWarning_ProtocolAgnostic(t *testing.T) {
	cfg := newTestConfig(3, 3, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	warning, _ := step["capacity_warning"].(string)

	if warning == "" {
		t.Fatal("expected capacity_warning when only 1/4 workers active")
	}

	forbidden := []string{"launch", "subagent", "pull more", "MUST"}
	for _, word := range forbidden {
		if containsCI(warning, word) {
			t.Errorf("capacity_warning contains v1 language %q: %s", word, warning)
		}
	}

	required := []string{"under capacity", "workers active"}
	for _, word := range required {
		if !containsCI(warning, word) {
			t.Errorf("capacity_warning missing %q: %s", word, warning)
		}
	}
	t.Logf("capacity_warning: %s", warning)
}

// TestCapacityGate_ProtocolAgnostic verifies gate error message.
func TestCapacityGate_ProtocolAgnostic(t *testing.T) {
	sess := &mcp.CircuitSession{DesiredCapacity: 4}
	sess.AgentPull()
	gateErr := sess.CheckCapacityGate()
	if gateErr == nil {
		t.Fatal("expected gate error with 1/4 capacity")
	}
	msg := gateErr.Error()

	forbidden := []string{"CAPACITY GATE ADVISORY", "Pull", "bring more workers", "TTL watchdog"}
	for _, word := range forbidden {
		if containsCI(msg, word) {
			t.Errorf("gate message contains v1 language %q: %s", word, msg)
		}
	}

	required := []string{"capacity gate", "workers observed", "expects"}
	for _, word := range required {
		if !containsCI(msg, word) {
			t.Errorf("gate message missing %q: %s", word, msg)
		}
	}
	t.Logf("gate message: %s", msg)
}

// TestWorkerMode_StreamRegistration verifies worker_started signal tracking.
func TestWorkerMode_StreamRegistration(t *testing.T) {
	cfg := newTestConfig(3, 3, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	for i := 0; i < 4; i++ {
		callTool(t, ctx, session, "emit_signal", map[string]any{
			"session_id": sessionID,
			"event":      "worker_started",
			"agent":      "worker",
			"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", i), "mode": "stream"},
		})
	}

	signals := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	signalList, _ := signals["signals"].([]any)

	var workerStarted int
	for _, s := range signalList {
		sig, _ := s.(map[string]any)
		if sig["event"] == "worker_started" {
			workerStarted++
			meta, _ := sig["meta"].(map[string]any)
			if meta["mode"] != "stream" {
				t.Errorf("worker_started signal missing mode=stream: %v", meta)
			}
		}
	}
	if workerStarted != 4 {
		t.Errorf("expected 4 worker_started signals, got %d", workerStarted)
	}
}

// TestWorkerMode_NoWorkerID_Ignored verifies graceful handling.
func TestWorkerMode_NoWorkerID_Ignored(t *testing.T) {
	cfg := newTestConfig(3, 3, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 2,
	})
	sessionID := startResult["session_id"].(string)

	callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "worker_started",
		"agent":      "worker",
		"meta":       map[string]any{"mode": "stream"},
	})

	callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "worker_started",
		"agent":      "worker",
	})

	t.Log("worker_started without worker_id accepted without panic")
}

// TestV2Workers_FullDrain_Deterministic is the definitive v2 choreography test.
func TestV2Workers_FullDrain_Deterministic(t *testing.T) {
	cfg := newTestConfig(4, 2, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	if wp, _ := startResult["worker_prompt"].(string); wp == "" {
		t.Fatal("expected worker_prompt in start_circuit response")
	}
	if wc, _ := startResult["worker_count"].(float64); int(wc) != 4 {
		t.Fatalf("expected worker_count=4, got %v", wc)
	}

	type stepRecord struct {
		CaseID     string
		Step       string
		DispatchID int64
	}

	var mu sync.Mutex
	workLog := make(map[int][]stepRecord)
	seenDispatchIDs := make(map[int64]bool)

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			_, err := callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_started",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID), "mode": "stream"},
			})
			if err != nil {
				errCh <- fmt.Errorf("w%d emit worker_started: %w", workerID, err)
				return
			}

			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 300,
				})
				if err != nil {
					errCh <- fmt.Errorf("w%d get_next_step: %w", workerID, err)
					return
				}

				if done, _ := res["done"].(bool); done {
					break
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				caseID, _ := res["case_id"].(string)
				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				_, err = callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(dispatchID),
					"step":        step,
					"fields":      testFieldsForStepWithWorker(step, workerID),
				})
				if err != nil {
					errCh <- fmt.Errorf("w%d submit(%s/%s): %w", workerID, caseID, step, err)
					return
				}

				mu.Lock()
				workLog[workerID] = append(workLog[workerID], stepRecord{
					CaseID: caseID, Step: step, DispatchID: int64(dispatchID),
				})
				if seenDispatchIDs[int64(dispatchID)] {
					errCh <- fmt.Errorf("w%d: duplicate dispatch_id %d", workerID, int64(dispatchID))
				}
				seenDispatchIDs[int64(dispatchID)] = true
				mu.Unlock()
			}

			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_stopped",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID)},
			})
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("worker error: %v", err)
	}

	for i := 0; i < 4; i++ {
		if len(workLog[i]) == 0 {
			t.Errorf("worker-%d got zero steps (starvation)", i)
		} else {
			t.Logf("worker-%d processed %d steps", i, len(workLog[i]))
		}
	}

	var totalSteps int
	for _, records := range workLog {
		totalSteps += len(records)
	}
	if totalSteps == 0 {
		t.Fatal("circuit produced zero steps")
	}
	t.Logf("total steps: %d across 4 workers", totalSteps)

	signals := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	signalList, _ := signals["signals"].([]any)

	startedWorkers := make(map[string]bool)
	stoppedWorkers := make(map[string]bool)
	for _, s := range signalList {
		sig, _ := s.(map[string]any)
		event, _ := sig["event"].(string)
		meta, _ := sig["meta"].(map[string]any)
		wid, _ := meta["worker_id"].(string)
		switch event {
		case "worker_started":
			startedWorkers[wid] = true
		case "worker_stopped":
			stoppedWorkers[wid] = true
		}
	}
	if len(startedWorkers) != 4 {
		t.Errorf("expected 4 worker_started signals, got %d", len(startedWorkers))
	}
	if len(stoppedWorkers) != 4 {
		t.Errorf("expected 4 worker_stopped signals, got %d", len(stoppedWorkers))
	}

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("expected status=done, got %s", status)
	}
}

// TestV2Workers_ConcurrencyTiming_Deterministic measures concurrent throughput.
func TestV2Workers_ConcurrencyTiming_Deterministic(t *testing.T) {
	cfg := newTestConfig(8, 3, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 4,
	})
	sessionID := startResult["session_id"].(string)

	const perStepDelay = 20 * time.Millisecond
	var mu sync.Mutex
	var totalSteps int64

	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	start := time.Now()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_started",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID), "mode": "stream"},
			})

			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 300,
				})
				if err != nil {
					errCh <- err
					return
				}
				if done, _ := res["done"].(bool); done {
					break
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				time.Sleep(perStepDelay)

				step, _ := res["step"].(string)
				dispatchID, _ := res["dispatch_id"].(float64)

				_, err = callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(dispatchID),
					"step":        step,
					"fields":      testFieldsForStepWithWorker(step, workerID),
				})
				if err != nil {
					errCh <- err
					return
				}
				mu.Lock()
				totalSteps++
				mu.Unlock()
			}

			_, _ = callToolE(ctx, session, "emit_signal", map[string]any{
				"session_id": sessionID,
				"event":      "worker_stopped",
				"agent":      "worker",
				"meta":       map[string]any{"worker_id": fmt.Sprintf("w%d", workerID)},
			})
		}(i)
	}

	wg.Wait()
	close(errCh)
	elapsed := time.Since(start)

	for err := range errCh {
		t.Fatalf("worker error: %v", err)
	}

	serialEstimate := time.Duration(totalSteps) * perStepDelay
	speedup := float64(serialEstimate) / float64(elapsed)

	t.Logf("timing: steps=%d, elapsed=%v, serial=%v, speedup=%.2fx",
		totalSteps, elapsed, serialEstimate, speedup)

	if elapsed > time.Duration(float64(serialEstimate)*0.75) {
		t.Errorf("concurrent execution too slow: elapsed=%v > 75%% of serial=%v (speedup=%.2fx)",
			elapsed, serialEstimate, speedup)
	}
}

// TestWorkerPrompt_StepSchemas verifies the generated prompt mentions all steps.
func TestWorkerPrompt_StepSchemas(t *testing.T) {
	cfg := newTestConfig(1, 1, "")
	sess := &mcp.CircuitSession{
		ID:              "test-session",
		DesiredCapacity: 4,
	}

	prompt := sess.WorkerPrompt(&cfg)

	for _, schema := range testStepSchemas {
		if !containsCI(prompt, schema.Name) {
			t.Errorf("worker prompt missing step %s", schema.Name)
		}
	}

	if !containsCI(prompt, "test-session") {
		t.Error("worker prompt missing session_id")
	}

	keywords := []string{`action="step"`, `action="submit"`, "worker_started", "worker_stopped", "mode", "stream"}
	for _, kw := range keywords {
		if !containsCI(prompt, kw) {
			t.Errorf("worker prompt missing keyword %q", kw)
		}
	}
}

// TestWorkerPrompt_SessionIDEmbedded verifies session ID is concrete, not a placeholder.
func TestWorkerPrompt_SessionIDEmbedded(t *testing.T) {
	cfg := newTestConfig(1, 1, "")
	sess := &mcp.CircuitSession{
		ID:              "s-1234567890",
		DesiredCapacity: 2,
	}

	prompt := sess.WorkerPrompt(&cfg)

	if !containsCI(prompt, "s-1234567890") {
		t.Error("worker prompt does not contain the actual session ID")
	}

	if containsCI(prompt, "%s") || containsCI(prompt, "{session_id}") || containsCI(prompt, "%[1]s") {
		t.Error("worker prompt contains unresolved template placeholders")
	}
}

// TestSignalBus_EmitAndGet tests basic signal bus flow.
func TestSignalBus_EmitAndGet(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(3))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sessionID := startResult["session_id"].(string)
	time.Sleep(300 * time.Millisecond)

	emitResult := callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "dispatch",
		"agent":      "main",
		"case_id":    "C01",
		"step":       "STEP_A",
		"meta":       map[string]any{"detail": "test"},
	})
	if emitResult["ok"] != "signal emitted" {
		t.Fatalf("expected ok='signal emitted', got %v", emitResult)
	}

	getResult := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sessionID,
	})
	total, _ := getResult["total"].(float64)
	if total < 2 {
		t.Fatalf("expected at least 2 signals, got %v", total)
	}

	signals, ok := getResult["signals"].([]any)
	if !ok || len(signals) == 0 {
		t.Fatal("expected signals array")
	}

	found := false
	for _, s := range signals {
		sig, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if sig["event"] == "dispatch" && sig["agent"] == "main" && sig["case_id"] == "C01" {
			found = true
			break
		}
	}
	if !found {
		t.Error("agent-emitted dispatch signal not found in bus")
	}
}

// TestSignalBus_EmitRejectsEmpty verifies validation.
func TestSignalBus_EmitRejectsEmpty(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sessionID := startResult["session_id"].(string)
	time.Sleep(300 * time.Millisecond)

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "signal",
		Arguments: map[string]any{
			"action": "emit", "session_id": sessionID, "event": "", "agent": "main",
		},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for empty event")
	}

	res, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "signal",
		Arguments: map[string]any{
			"action": "emit", "session_id": sessionID, "event": "test", "agent": "",
		},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for empty agent")
	}
}

// TestSession_TTL_Abort verifies the TTL watchdog aborts stale sessions.
func TestSession_TTL_Abort(t *testing.T) {
	cfg := newTestConfig(3, 3, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}

	srv.SetSessionTTL(200 * time.Millisecond)
	time.Sleep(500 * time.Millisecond)

	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	done, _ := res["done"].(bool)
	if !done {
		t.Fatalf("expected done=true after TTL abort, got %v", res)
	}
	t.Log("TTL abort verified")
}

// TestCleanArtifactJSON verifies markdown fence stripping.
func TestCleanArtifactJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain JSON", `{"key":"value"}`, `{"key":"value"}`},
		{"json fenced", "```json\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
		{"generic fenced", "```\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
		{"whitespace", "  \n{\"key\":\"value\"}\n  ", `{"key":"value"}`},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(mcp.CleanArtifactJSON([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("CleanArtifactJSON(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCircuitServer_GetWorkerHealth(t *testing.T) {
	srv := newTestServer(t, newTestConfig(2, 1, ""))
	ctx := context.Background()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	// Emit worker signals manually
	callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "worker_started",
		"agent":      "worker",
		"meta": map[string]any{
			"worker_id": "test-w1",
		},
	})
	callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sessionID,
		"event":      "error",
		"agent":      "worker",
		"case_id":    "C01",
		"step":       "STEP_A",
		"meta": map[string]any{
			"worker_id": "test-w1",
			"error":     "test error",
		},
	})

	health := callTool(t, ctx, session, "get_worker_health", map[string]any{
		"session_id": sessionID,
	})

	workers, ok := health["workers"]
	if !ok {
		t.Fatal("health response missing 'workers' field")
	}
	workerList, ok := workers.([]any)
	if !ok || len(workerList) == 0 {
		t.Fatal("expected at least one worker in health summary")
	}

	w := workerList[0].(map[string]any)
	if w["worker_id"] != "test-w1" {
		t.Errorf("expected worker_id=test-w1, got %v", w["worker_id"])
	}
	if w["error_count"].(float64) != 1 {
		t.Errorf("expected error_count=1, got %v", w["error_count"])
	}
	if w["last_error"] != "test error" {
		t.Errorf("expected last_error='test error', got %v", w["last_error"])
	}

	srv.Shutdown()
}

// --- submit_step tests ---

func TestStepSchema_ValidateFields(t *testing.T) {
	schema := mcp.StepSchema{
		Name: "TEST_STEP",
		Defs: []mcp.FieldDef{
			{Name: "name", Type: "string", Required: true},
			{Name: "score", Type: "float", Required: true},
			{Name: "notes", Type: "string", Required: false},
		},
	}

	t.Run("valid with all fields", func(t *testing.T) {
		err := schema.ValidateFields(map[string]any{
			"name": "foo", "score": 0.9, "notes": "ok",
		})
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("valid with optional missing", func(t *testing.T) {
		err := schema.ValidateFields(map[string]any{
			"name": "foo", "score": 0.9,
		})
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		err := schema.ValidateFields(map[string]any{"name": "foo"})
		if err == nil {
			t.Fatal("expected error for missing required field")
		}
		if !strings.Contains(err.Error(), "score") {
			t.Errorf("error should mention 'score': %v", err)
		}
	})

	t.Run("null required field", func(t *testing.T) {
		err := schema.ValidateFields(map[string]any{
			"name": "foo", "score": nil,
		})
		if err == nil {
			t.Fatal("expected error for null required field")
		}
	})

	t.Run("no defs passes anything", func(t *testing.T) {
		empty := mcp.StepSchema{Name: "EMPTY"}
		err := empty.ValidateFields(map[string]any{"whatever": 42})
		if err != nil {
			t.Errorf("schema with no defs should pass: %v", err)
		}
	})
}

func TestCircuitConfig_FindSchema(t *testing.T) {
	cfg := mcp.CircuitConfig{
		StepSchemas: testStepSchemas,
	}

	t.Run("found", func(t *testing.T) {
		s, err := cfg.FindSchema("STEP_B")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Name != "STEP_B" {
			t.Errorf("expected STEP_B, got %s", s.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := cfg.FindSchema("NO_SUCH_STEP")
		if err == nil {
			t.Fatal("expected error for unknown step")
		}
		if !strings.Contains(err.Error(), "STEP_A") {
			t.Errorf("error should list valid steps: %v", err)
		}
	})
}

func TestSubmitStep_FullLoop(t *testing.T) {
	srv := newTestServer(t, newTestConfig(1, 1, ""))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 2000,
	})

	if done, _ := res["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}

	step, _ := res["step"].(string)
	dispatchID, _ := res["dispatch_id"].(float64)

	result := callTool(t, ctx, session, "submit_step", map[string]any{
		"session_id":  sessionID,
		"dispatch_id": int64(dispatchID),
		"step":        step,
		"fields":      testFieldsForStep(step),
	})

	if ok, _ := result["ok"].(string); ok != "step accepted" {
		t.Errorf("expected 'step accepted', got %q", ok)
	}
}

func TestSubmitStep_UnknownStep(t *testing.T) {
	srv := newTestServer(t, newTestConfig(1, 1, ""))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 2000,
	})
	dispatchID, _ := res["dispatch_id"].(float64)

	_, err := callToolE(ctx, session, "submit_step", map[string]any{
		"session_id":  sessionID,
		"dispatch_id": int64(dispatchID),
		"step":        "NONEXISTENT",
		"fields":      map[string]any{"x": 1},
	})
	if err == nil {
		t.Fatal("expected error for unknown step")
	}
	if !strings.Contains(err.Error(), "unknown step") {
		t.Errorf("error should mention 'unknown step': %v", err)
	}
}

func TestSubmitStep_MissingRequiredField(t *testing.T) {
	srv := newTestServer(t, newTestConfig(1, 1, ""))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 2000,
	})
	dispatchID, _ := res["dispatch_id"].(float64)
	step, _ := res["step"].(string)

	_, err := callToolE(ctx, session, "submit_step", map[string]any{
		"session_id":  sessionID,
		"dispatch_id": int64(dispatchID),
		"step":        step,
		"fields":      map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
	if !strings.Contains(err.Error(), "missing required field") {
		t.Errorf("error should mention 'missing required field': %v", err)
	}
}

func TestSubmitStep_ZeroDispatchID(t *testing.T) {
	srv := newTestServer(t, newTestConfig(1, 1, ""))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	_, err := callToolE(ctx, session, "submit_step", map[string]any{
		"session_id":  sessionID,
		"dispatch_id": 0,
		"step":        "STEP_A",
		"fields":      map[string]any{"value": "x", "score": 1.0},
	})
	if err == nil {
		t.Fatal("expected error for dispatch_id=0")
	}
}

// --- Timeout and fail-fast tests ---

func TestSession_MaxDuration_AbortsCircuit(t *testing.T) {
	cfg := mcp.CircuitConfig{
		Name:               "test-circuit",
		Version:            "dev",
		StepSchemas:        testStepSchemas,
		MaxSessionDuration: 200, // 200ms max
		DefaultGetNextStepTimeout: 1000,
		DefaultSessionTTL:         300000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(ctx context.Context) (any, error) {
				// RunFunc that sleeps for 2s — should be aborted by max duration
				select {
				case <-time.After(2 * time.Second):
					return &testReport{CasesProcessed: 1}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}, mcp.SessionMeta{TotalCases: 1, Scenario: "max-dur-test"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return "report", result, nil
		},
	}
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sessionID := startResult["session_id"].(string)

	// Poll get_next_step until done=true (session should abort within ~200ms)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("session did not abort within expected time")
		default:
		}

		res, err := callToolE(ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
			"timeout_ms": 100,
		})
		if err != nil {
			// Might get an error if the session is already torn down
			break
		}
		if done, _ := res["done"].(bool); done {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// get_report should show error status
	time.Sleep(100 * time.Millisecond)
	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "error" {
		t.Errorf("expected status=error after max duration abort, got %s", status)
	}
}

func TestSession_MaxDuration_ZeroIsNoLimit(t *testing.T) {
	cfg := mcp.CircuitConfig{
		Name:               "test-circuit",
		Version:            "dev",
		StepSchemas:        testStepSchemas,
		MaxSessionDuration: 0, // no limit
		DefaultGetNextStepTimeout: 1000,
		DefaultSessionTTL:         300000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return func(ctx context.Context) (any, error) {
				return &testReport{CasesProcessed: 1}, nil
			}, mcp.SessionMeta{TotalCases: 1, Scenario: "no-limit-test"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return "report", result, nil
		},
	}
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sessionID := startResult["session_id"].(string)

	time.Sleep(300 * time.Millisecond)

	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	done, _ := res["done"].(bool)
	if !done {
		t.Fatal("expected done=true for instant RunFunc with MaxSessionDuration=0")
	}

	reportResult := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Errorf("expected status=done, got %s (session should run normally without max duration)", status)
	}
}

func TestSession_MaxDuration_InteractionWithTTL(t *testing.T) {
	cfg := mcp.CircuitConfig{
		Name:               "test-circuit",
		Version:            "dev",
		StepSchemas:        testStepSchemas,
		MaxSessionDuration: 5000, // 5s max (long)
		DefaultGetNextStepTimeout: 1000,
		DefaultSessionTTL:         200, // 200ms TTL (short — should win)
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (mcp.RunFunc, mcp.SessionMeta, error) {
			return stubRunFunc(disp, 3, 3, 1, []string{"STEP_A", "STEP_B", "STEP_C"}, ""),
				mcp.SessionMeta{TotalCases: 3, Scenario: "ttl-vs-maxdur"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return "report", result, nil
		},
	}
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	// Pull one step to prove the session is running
	step := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	if done, _ := step["done"].(bool); done {
		t.Fatal("expected step, got done=true")
	}

	// Don't submit — let TTL expire (200ms)
	time.Sleep(500 * time.Millisecond)

	res := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sessionID,
	})
	done, _ := res["done"].(bool)
	if !done {
		t.Fatal("expected done=true after TTL abort (TTL should win over max duration)")
	}
}

// TestGetReport_ContextCancellation verifies that get_report returns promptly when
// the MCP handler context is cancelled.
func TestGetReport_ContextCancellation(t *testing.T) {
	cfg := newTestConfig(3, 3, "")
	srv := newTestServer(t, cfg)
	mainCtx, mainCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer mainCancel()

	session := connectInMemory(t, mainCtx, srv)
	defer session.Close()

	startResult := callTool(t, mainCtx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	// Call get_report with a short context — session is still running so it would block
	shortCtx, shortCancel := context.WithTimeout(mainCtx, 100*time.Millisecond)
	defer shortCancel()

	start := time.Now()
	_, err := callToolE(shortCtx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context on get_report")
	}
	if elapsed > 1*time.Second {
		t.Errorf("get_report took %v to fail, expected ~100ms abort", elapsed)
	}
	t.Logf("get_report cancelled in %v", elapsed)
}

func testFieldsForStep(step string) map[string]any {
	switch step {
	case "STEP_A":
		return map[string]any{"value": "test", "score": 0.95}
	case "STEP_B":
		return map[string]any{"result": true}
	case "STEP_C":
		return map[string]any{"summary": "done"}
	default:
		return map[string]any{"data": step}
	}
}

func testFieldsForStepWithWorker(step string, workerID int) map[string]any {
	switch step {
	case "STEP_A":
		return map[string]any{"value": fmt.Sprintf("worker-%d", workerID), "score": 0.9}
	case "STEP_B":
		return map[string]any{"result": true}
	case "STEP_C":
		return map[string]any{"summary": fmt.Sprintf("done by worker-%d", workerID)}
	default:
		return map[string]any{"step": step, "worker": workerID}
	}
}

// --- Gap 1: Dispatch gap — late worker tests ---

func TestCircuitSession_LateWorker_StillGetsSteps(t *testing.T) {
	ctx := context.Background()
	runCtx, runCancel := context.WithCancel(ctx)

	disp := dispatch.NewMuxDispatcher(runCtx, dispatch.WithMuxSignalBus(dispatch.NewSignalBus()))
	bus := dispatch.NewSignalBus()

	nCases, nSteps := 3, 1
	steps := []string{"STEP_A"}
	runFn := stubRunFunc(disp, nCases, nSteps, 1, steps, "")

	sess := mcp.NewCircuitSession(runCtx, "test-late-worker",
		mcp.SessionMeta{TotalCases: nCases, Scenario: "late-worker"},
		1, disp, bus, runFn, runCancel)
	sess.SetTTL(300 * time.Second)
	sess.Start()

	// Simulate worker startup latency.
	time.Sleep(2 * time.Second)

	if state := sess.GetState(); state != mcp.StateRunning {
		t.Fatalf("session state after 2s = %s; want running "+
			"(session completed before workers could connect)", state)
	}

	for i := 0; i < nCases; i++ {
		dc, done, avail, err := sess.GetNextStep(ctx, 5*time.Second)
		if err != nil {
			t.Fatalf("step %d: GetNextStep error: %v", i, err)
		}
		if done {
			t.Fatalf("step %d: session done prematurely (dispatch gap)", i)
		}
		if !avail {
			t.Fatalf("step %d: no step available", i)
		}
		fields := testFieldsForStep(dc.Step)
		data, _ := json.Marshal(fields)
		if err := sess.SubmitArtifact(ctx, dc.DispatchID, data); err != nil {
			t.Fatalf("step %d: SubmitArtifact: %v", i, err)
		}
	}

	select {
	case <-sess.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("session did not complete after all steps submitted")
	}

	if sess.Err() != nil {
		t.Fatalf("session error: %v", sess.Err())
	}
	if sess.Result() == nil {
		t.Fatal("session result is nil")
	}
}

func TestCircuitServer_MCP_LateWorkerEndToEnd(t *testing.T) {
	cfg := newTestConfig(2, 1, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	// Simulate worker startup latency.
	time.Sleep(2 * time.Second)

	stepsProcessed := 0
	for {
		res, err := callToolE(ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
			"timeout_ms": 5000,
		})
		if err != nil {
			t.Fatalf("get_next_step: %v", err)
		}

		if done, _ := res["done"].(bool); done {
			break
		}
		if avail, _ := res["available"].(bool); !avail {
			continue
		}

		dispatchID := int64(res["dispatch_id"].(float64))
		step, _ := res["step"].(string)

		_, err = callToolE(ctx, session, "submit_step", map[string]any{
			"session_id":  sessionID,
			"dispatch_id": dispatchID,
			"step":        step,
			"fields":      testFieldsForStep(step),
		})
		if err != nil {
			t.Fatalf("submit_step: %v", err)
		}
		stepsProcessed++
	}

	if stepsProcessed != 2 {
		t.Fatalf("processed %d steps, want 2 "+
			"(0 means the dispatch gap caused instant completion)", stepsProcessed)
	}
}

func TestCircuitServer_MCP_LateWorker_ReportProduced(t *testing.T) {
	cfg := newTestConfig(3, 1, "")
	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	startResult := callTool(t, ctx, session, "start_circuit", map[string]any{
		"parallel": 1,
	})
	sessionID := startResult["session_id"].(string)

	time.Sleep(1 * time.Second)

	for {
		res, err := callToolE(ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
			"timeout_ms": 5000,
		})
		if err != nil {
			t.Fatalf("get_next_step: %v", err)
		}

		if done, _ := res["done"].(bool); done {
			break
		}
		if avail, _ := res["available"].(bool); !avail {
			continue
		}

		dispatchID := int64(res["dispatch_id"].(float64))
		step, _ := res["step"].(string)

		_, err = callToolE(ctx, session, "submit_step", map[string]any{
			"session_id":  sessionID,
			"dispatch_id": dispatchID,
			"step":        step,
			"fields":      testFieldsForStep(step),
		})
		if err != nil {
			t.Fatalf("submit_step: %v", err)
		}
	}

	report, err := callToolE(ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if err != nil {
		t.Fatalf("get_report: %v", err)
	}

	reportText, _ := report["report"].(string)
	if reportText == "" {
		t.Fatal("report text is empty — late worker produced no results")
	}
	if !containsCI(reportText, "3 cases") {
		t.Errorf("report should mention 3 cases, got: %s", reportText)
	}
}

// --- Gap 2: MCP routing gap — worker prompt endpoint ---

func TestWorkerPrompt_ContainsEndpointURL(t *testing.T) {
	endpoint := "http://localhost:9000/mcp"
	cfg := &mcp.CircuitConfig{
		Name:            "test-circuit",
		Version:         "dev",
		StepSchemas:     testStepSchemas,
		WorkerPreamble:  "You are a test worker.",
		GatewayEndpoint: endpoint,
	}

	ctx := context.Background()
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	disp := dispatch.NewMuxDispatcher(runCtx)
	bus := dispatch.NewSignalBus()
	runFn := stubRunFuncInstant(1)

	sess := mcp.NewCircuitSession(runCtx, "test-endpoint",
		mcp.SessionMeta{TotalCases: 1, Scenario: "endpoint-test"},
		1, disp, bus, runFn, runCancel)
	sess.Start()

	prompt := sess.WorkerPrompt(cfg)

	if !strings.Contains(prompt, endpoint) {
		t.Errorf("worker prompt does not contain gateway endpoint %q;\n"+
			"Task subagents cannot inherit MCP config and need explicit URLs.\n"+
			"Prompt:\n%s", endpoint, prompt)
	}
	if !strings.Contains(prompt, "## Connection") {
		t.Error("worker prompt missing ## Connection section")
	}
}

func TestWorkerPrompt_NoEndpoint_OmitsConnectionSection(t *testing.T) {
	cfg := &mcp.CircuitConfig{
		Name:        "test-circuit",
		Version:     "dev",
		StepSchemas: testStepSchemas,
	}

	ctx := context.Background()
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	disp := dispatch.NewMuxDispatcher(runCtx)
	bus := dispatch.NewSignalBus()
	runFn := stubRunFuncInstant(1)

	sess := mcp.NewCircuitSession(runCtx, "test-no-endpoint",
		mcp.SessionMeta{TotalCases: 1, Scenario: "no-endpoint-test"},
		1, disp, bus, runFn, runCancel)
	sess.Start()

	prompt := sess.WorkerPrompt(cfg)

	if strings.Contains(prompt, "## Connection") {
		t.Error("worker prompt should not have Connection section when GatewayEndpoint is empty")
	}
}

// --- TSK-193: MCP tool contract tests ---
//
// Verify each Papercup tool accepts valid input and returns
// well-formed output with expected fields present.

func TestToolContract_StartCircuit(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(3))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	out := callTool(t, ctx, session, "start_circuit", map[string]any{})
	requireField(t, out, "session_id", "start_circuit")
	requireField(t, out, "total_cases", "start_circuit")
	requireField(t, out, "status", "start_circuit")
}

func TestToolContract_GetNextStep(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sid := start["session_id"].(string)
	time.Sleep(300 * time.Millisecond)

	out := callTool(t, ctx, session, "get_next_step", map[string]any{
		"session_id": sid,
	})
	// Stub completes immediately — should return done=true.
	if _, ok := out["done"]; !ok {
		t.Error("get_next_step missing 'done' field")
	}
}

func TestToolContract_EmitSignal(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sid := start["session_id"].(string)

	out := callTool(t, ctx, session, "emit_signal", map[string]any{
		"session_id": sid,
		"event":      "test_event",
		"agent":      "test_agent",
	})
	requireField(t, out, "ok", "emit_signal")
	requireField(t, out, "index", "emit_signal")
}

func TestToolContract_GetSignals(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sid := start["session_id"].(string)

	out := callTool(t, ctx, session, "get_signals", map[string]any{
		"session_id": sid,
	})
	requireField(t, out, "signals", "get_signals")
	requireField(t, out, "total", "get_signals")
}

func TestToolContract_GetWorkerHealth(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sid := start["session_id"].(string)

	out := callTool(t, ctx, session, "get_worker_health", map[string]any{
		"session_id": sid,
	})
	// Worker health returns summary even with no workers.
	if out == nil {
		t.Error("get_worker_health returned nil")
	}
}

func TestToolContract_GetReport(t *testing.T) {
	srv := newTestServer(t, newTestConfigStub(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sid := start["session_id"].(string)
	time.Sleep(300 * time.Millisecond)

	out := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sid,
	})
	requireField(t, out, "status", "get_report")
}

func TestToolContract_SubmitStep_RequiresDispatchID(t *testing.T) {
	srv := newTestServer(t, newTestConfig(1, 1, ""))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := connectInMemory(t, ctx, srv)
	defer session.Close()

	start := callTool(t, ctx, session, "start_circuit", map[string]any{})
	sid := start["session_id"].(string)

	// dispatch_id=0 should fail (zero value treated as missing).
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "circuit",
		Arguments: map[string]any{
			"action":      "submit",
			"session_id":  sid,
			"dispatch_id": 0,
			"step":        "STEP_A",
			"fields":      map[string]any{"x": 1},
		},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !res.IsError {
		t.Error("submit_step with dispatch_id=0 should return error")
	}
}

func requireField(t *testing.T, out map[string]any, field, tool string) {
	t.Helper()
	if _, ok := out[field]; !ok {
		t.Errorf("%s response missing required field %q", tool, field)
	}
}
