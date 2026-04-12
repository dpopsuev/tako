package containertest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/troupe/signal"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))
	os.Exit(m.Run())
}

// TestFramework_E2E_InProcess exercises the full MCP circuit lifecycle
// (start -> step -> submit -> report) in-process using only origami
// packages. No containers, no consumer imports, no external services.
//
//nolint:gocyclo // E2E test with sequential MCP call choreography
func TestFramework_E2E_InProcess(t *testing.T) {
	const (
		totalCases = 1
		totalSteps = 1
	)

	// Build a framework-only CircuitConfig with a stub CreateSession
	// that dispatches a single case through a single step (echo circuit).
	stepSchemas := []mcp.StepSchema{
		{
			Name: "ECHO",
			Defs: []mcp.FieldDef{
				{Name: "output", Type: "string", Required: true},
			},
		},
	}

	cfg := &mcp.CircuitConfig{
		Name:                      "framework-e2e",
		Version:                   "dev",
		StepSchemas:               stepSchemas,
		DefaultGetNextStepTimeout: 5000,
		DefaultSessionTTL:         30000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus signal.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			runFn := echoRunFunc(disp, totalCases, totalSteps)
			meta := mcp.SessionMeta{
				TotalCases: totalCases,
				Scenario:   "echo-e2e",
			}
			return runFn, meta, nil
		},
		FormatReport: func(result any) (string, any, error) {
			r, ok := result.(*echoReport)
			if !ok {
				return "", nil, fmt.Errorf("unexpected result type: %T", result)
			}
			return fmt.Sprintf("Echo E2E: %d cases, %d steps processed", r.Cases, r.Steps), r, nil
		},
	}

	// Create the CircuitServer (registers circuit + signal tools).
	srv := mcp.NewCircuitServer(cfg)
	t.Cleanup(srv.Shutdown)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect an in-memory MCP client to the server.
	session := connectInMemory(ctx, t, srv)
	defer session.Close()

	// Verify tool discovery.
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	foundCircuit := false
	for _, tool := range tools.Tools {
		if tool.Name == "circuit" {
			foundCircuit = true
			break
		}
	}
	if !foundCircuit {
		t.Fatal("circuit tool not found in ListTools")
	}

	// ACTION: start
	startResult := callCircuitTool(ctx, t, session, map[string]any{
		"action": "start",
	})
	sessionID, ok := startResult["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("expected non-empty session_id, got %v", startResult["session_id"])
	}
	tc, _ := startResult["total_cases"].(float64)
	if int(tc) != totalCases {
		t.Fatalf("expected total_cases=%d, got %v", totalCases, tc)
	}
	status, _ := startResult["status"].(string)
	if status != "running" {
		t.Fatalf("expected status=running, got %s", status)
	}
	t.Logf("started session %s (total_cases=%d, scenario=%s)",
		sessionID, int(tc), startResult["scenario"])

	// ACTION: step + submit loop
	stepsProcessed := 0
	for i := 0; i < 20; i++ {
		stepResult := callCircuitTool(ctx, t, session, map[string]any{
			"action":     "step",
			"session_id": sessionID,
			"timeout_ms": 5000,
		})

		if done, _ := stepResult["done"].(bool); done {
			if errMsg, _ := stepResult["error"].(string); errMsg != "" {
				t.Fatalf("circuit done with error: %s", errMsg)
			}
			t.Log("circuit reported done=true")
			break
		}

		if avail, _ := stepResult["available"].(bool); !avail {
			continue
		}

		caseID, _ := stepResult["case_id"].(string)
		step, _ := stepResult["step"].(string)
		dispatchID, _ := stepResult["dispatch_id"].(float64)

		t.Logf("step %d: case=%s step=%s dispatch_id=%d",
			i, caseID, step, int64(dispatchID))

		// ACTION: submit
		submitResult := callCircuitTool(ctx, t, session, map[string]any{
			"action":      "submit",
			"session_id":  sessionID,
			"dispatch_id": int64(dispatchID),
			"step":        step,
			"fields": map[string]any{
				"output": fmt.Sprintf("echo from %s/%s", caseID, step),
			},
		})
		okMsg, _ := submitResult["ok"].(string)
		if okMsg != "step accepted" {
			t.Fatalf("expected 'step accepted', got %q", okMsg)
		}
		stepsProcessed++
	}

	if stepsProcessed != totalCases*totalSteps {
		t.Fatalf("processed %d steps, expected %d", stepsProcessed, totalCases*totalSteps)
	}

	// Wait for circuit to finish after all artifacts submitted.
	// Poll step until done=true.
	for i := 0; i < 10; i++ {
		stepResult := callCircuitTool(ctx, t, session, map[string]any{
			"action":     "step",
			"session_id": sessionID,
			"timeout_ms": 1000,
		})
		if done, _ := stepResult["done"].(bool); done {
			break
		}
	}

	// ACTION: report
	reportResult := callCircuitTool(ctx, t, session, map[string]any{
		"action":     "report",
		"session_id": sessionID,
	})
	reportStatus, _ := reportResult["status"].(string)
	if reportStatus != "done" {
		t.Fatalf("expected report status=done, got %s (error: %v)",
			reportStatus, reportResult["error"])
	}
	reportText, _ := reportResult["report"].(string)
	if reportText == "" {
		t.Fatal("expected non-empty report text")
	}
	t.Logf("report: %s", reportText)

	// Verify structured report.
	structured, ok := reportResult["structured"].(map[string]any)
	if !ok {
		t.Fatal("expected structured report in response")
	}
	cases, _ := structured["Cases"].(float64)
	steps, _ := structured["Steps"].(float64)
	if int(cases) != totalCases || int(steps) != totalCases*totalSteps {
		t.Errorf("structured report: Cases=%d Steps=%d, want Cases=%d Steps=%d",
			int(cases), int(steps), totalCases, totalCases*totalSteps)
	}

	t.Log("framework E2E lifecycle completed successfully")
}

// --- helpers ---

// echoReport is the domain result for the echo circuit.
type echoReport struct {
	Cases int `json:"Cases"`
	Steps int `json:"Steps"`
}

// echoRunFunc creates a RunFunc that dispatches nCases, each with nSteps
// sequential ECHO steps via the MuxDispatcher. The artifact content is
// ignored (echo behavior).
func echoRunFunc(disp *dispatch.MuxDispatcher, nCases, nSteps int) mcp.RunFunc {
	return func(ctx context.Context) (any, error) {
		var mu sync.Mutex
		totalSteps := 0
		errCh := make(chan error, nCases)

		var wg sync.WaitGroup
		for c := 0; c < nCases; c++ {
			wg.Add(1)
			go func(caseIdx int) {
				defer wg.Done()
				caseID := fmt.Sprintf("echo-%02d", caseIdx+1)
				for s := 0; s < nSteps; s++ {
					dc := dispatch.Context{
						CaseID:        caseID,
						Step:          "ECHO",
						PromptContent: fmt.Sprintf("Echo step %d for case %s", s+1, caseID),
						ArtifactPath:  fmt.Sprintf("/tmp/echo_%s_%d.json", caseID, s),
					}
					if _, err := disp.Dispatch(ctx, dc); err != nil {
						errCh <- fmt.Errorf("dispatch %s step %d: %w", caseID, s, err)
						return
					}
					mu.Lock()
					totalSteps++
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
		t := totalSteps
		mu.Unlock()
		return &echoReport{Cases: nCases, Steps: t}, nil
	}
}

// connectInMemory creates an in-memory MCP client-server connection.
func connectInMemory(ctx context.Context, t *testing.T, srv *mcp.CircuitServer) *sdkmcp.ClientSession {
	t.Helper()
	t1, t2 := sdkmcp.NewInMemoryTransports()
	serverSession, err := srv.MCPServer.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "e2e-test-client", Version: "v0.0.1"},
		nil,
	)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	return session
}

// callCircuitTool invokes the consolidated "circuit" tool and returns
// the parsed JSON response. Fails the test on transport or tool error.
func callCircuitTool(ctx context.Context, t *testing.T, session *sdkmcp.ClientSession, args map[string]any) map[string]any {
	t.Helper()
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(circuit, action=%v): %v", args["action"], err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				t.Fatalf("CallTool(circuit, action=%v) error: %s",
					args["action"], tc.Text)
			}
		}
		t.Fatalf("CallTool(circuit, action=%v) returned error", args["action"])
	}

	result := make(map[string]any)
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
				t.Fatalf("unmarshal circuit result: %v (text: %s)", err, tc.Text)
			}
			return result
		}
	}
	t.Fatal("no text content in circuit tool result")
	return nil
}
